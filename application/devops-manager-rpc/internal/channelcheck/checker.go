package channelcheck

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	devopsargocd "github.com/yanshicheng/kube-nova/common/devops/argocd"
	"github.com/yanshicheng/kube-nova/common/devops/authutil"
	devopsgitlab "github.com/yanshicheng/kube-nova/common/devops/gitlab"
	devopsharbor "github.com/yanshicheng/kube-nova/common/devops/harbor"
	devopshost "github.com/yanshicheng/kube-nova/common/devops/host"
	devopsjenkins "github.com/yanshicheng/kube-nova/common/devops/jenkins"
	devopsjfrog "github.com/yanshicheng/kube-nova/common/devops/jfrog"
	devopskubeNova "github.com/yanshicheng/kube-nova/common/devops/kubeNova"
	devopskubebench "github.com/yanshicheng/kube-nova/common/devops/kube_bench"
	devopskubernetes "github.com/yanshicheng/kube-nova/common/devops/kubernetes"
	devopsnexus "github.com/yanshicheng/kube-nova/common/devops/nexus"
	devopsregistry "github.com/yanshicheng/kube-nova/common/devops/registry"
	devopssonar "github.com/yanshicheng/kube-nova/common/devops/sonar"
	devopstekton "github.com/yanshicheng/kube-nova/common/devops/tekton"
	devopstrivy "github.com/yanshicheng/kube-nova/common/devops/trivy"
	devopstypes "github.com/yanshicheng/kube-nova/common/devops/types"
	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/crypto/ssh"
)

type Result struct {
	Success      bool
	HealthStatus string
	Message      string
	CheckedAt    int64
	Metadata     string
}

type Checker struct {
	channelModel    *model.DevopsChannelModel
	credentialModel *model.DevopsCredentialModel
	hostModel       *model.DevopsHostModel
	cron            *cron.Cron
}

type resolvedCredential struct {
	data   *model.DevopsCredential
	secret model.CredentialSecret
}

type hostGroupConfig struct {
	HostID  string   `json:"hostId"`
	HostIDs []string `json:"hostIds"`
}

func NewChecker(channelModel *model.DevopsChannelModel, credentialModel *model.DevopsCredentialModel, hostModel *model.DevopsHostModel) *Checker {
	return &Checker{channelModel: channelModel, credentialModel: credentialModel, hostModel: hostModel}
}

func (c *Checker) StartDaily() {
	c.cron = cron.New(cron.WithLocation(time.Local), cron.WithChain(cron.Recover(cron.DefaultLogger)))
	if _, err := c.cron.AddFunc("0 0 * * *", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		if err := c.CheckAll(ctx); err != nil {
			logx.Errorf("DevOps渠道定时连通性检测失败: %v", err)
		}
	}); err != nil {
		logx.Errorf("DevOps渠道定时任务注册失败: %v", err)
		return
	}
	c.cron.Start()
}

func (c *Checker) CheckAll(ctx context.Context) error {
	channels, err := c.channelModel.ListAll(ctx)
	if err != nil {
		return err
	}
	for _, item := range channels {
		if item == nil || item.Status != 1 {
			continue
		}
		result := c.Test(ctx, item)
		if err := c.channelModel.UpdateHealth(ctx, item.ID.Hex(), result.HealthStatus, result.Message, result.Metadata, "system"); err != nil {
			logx.Errorf("更新渠道连通性失败: channel=%s err=%v", item.Code, err)
		}
	}
	return nil
}

func (c *Checker) Test(ctx context.Context, channel *model.DevopsChannel) Result {
	checkedAt := time.Now().Unix()
	if channel == nil {
		return unhealthy("渠道配置为空", checkedAt)
	}
	endpoint := strings.TrimSpace(channel.Endpoint)
	if strings.HasPrefix(endpoint, "system://") && channel.IsSystem && !shouldUseAssetCheck(channel) {
		return healthyWithMetadata("系统内置渠道", checkedAt, channelMetadata(channel, map[string]any{"system": true}))
	}
	credential, err := c.resolveCredential(ctx, channel.CredentialID)
	if err != nil {
		return unhealthy(fmt.Sprintf("凭据读取失败: %s", trimMessage(err.Error())), checkedAt)
	}
	if provider := providerFor(channel.ChannelType); provider != nil {
		req, err := c.buildProviderRequest(ctx, channel, credential)
		if err != nil {
			return unhealthy(fmt.Sprintf("检测请求构建失败: %s", trimMessage(err.Error())), checkedAt)
		}
		return fromDevopsResult(provider.TestConnection(ctx, req))
	}
	switch channel.ChannelType {
	case "kubernetes", "tekton", "kube_bench":
		return testKubernetes(ctx, channel, credential, checkedAt)
	case "host":
		return c.testHostChannel(ctx, channel, credential, checkedAt)
	case "host_group":
		return c.testHostGroup(ctx, channel, checkedAt)
	case "spotbugs", "trivy":
		return healthyWithMetadata("本地执行型渠道，无需在线检测", checkedAt, channelMetadata(channel, map[string]any{"config": true}))
	default:
		if endpoint == "" {
			return unhealthy("访问地址为空", checkedAt)
		}
		if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
			return testHTTP(ctx, channel, credential, checkedAt)
		}
		return testChannelTCP(ctx, channel, endpoint, checkedAt)
	}
}

func shouldUseAssetCheck(channel *model.DevopsChannel) bool {
	if channel.ChannelType != "host" && channel.ChannelType != "host_group" {
		return false
	}
	config := strings.TrimSpace(channel.Config)
	return config != "" && config != "{}"
}

func (c *Checker) TestHost(ctx context.Context, host *model.DevopsHost) Result {
	checkedAt := time.Now().Unix()
	return c.testHost(ctx, host, checkedAt)
}

func providerFor(channelType string) devopstypes.Provider {
	switch channelType {
	case "jenkins":
		return devopsjenkins.New()
	case "gitlab":
		return devopsgitlab.New()
	case "github":
		return devopsgitlab.NewGitHub()
	case "gitee":
		return devopsgitlab.NewGitee()
	case "nexus":
		return devopsnexus.New()
	case "jfrog":
		return devopsjfrog.New()
	case "harbor":
		return devopsharbor.New()
	case "registry", "aliyun_registry":
		return devopsregistry.New()
	case "sonarqube":
		return devopssonar.New()
	case "trivy":
		return devopstrivy.New()
	case "kube_bench":
		return devopskubebench.New()
	case "kubernetes":
		return devopskubernetes.New()
	case "tekton":
		return devopstekton.New()
	case "host":
		return devopshost.New()
	case "kube-nova", "kubenova":
		return devopskubeNova.New()
	case "argocd":
		return devopsargocd.New()
	default:
		return nil
	}
}

func fromDevopsResult(result devopstypes.Result) Result {
	return Result{
		Success:      result.Success,
		HealthStatus: result.HealthStatus,
		Message:      result.Message,
		CheckedAt:    result.CheckedAt,
		Metadata:     devopstypes.MetadataJSON(result.Metadata),
	}
}

func (c *Checker) buildProviderRequest(ctx context.Context, channel *model.DevopsChannel, credential *resolvedCredential) (devopstypes.Request, error) {
	req := devopstypes.Request{
		Channel: devopstypes.Channel{
			ID:              channel.ID.Hex(),
			Name:            channel.Name,
			Code:            channel.Code,
			Type:            channel.ChannelType,
			Endpoint:        channel.Endpoint,
			Config:          channel.Config,
			AuthType:        channel.AuthType,
			Username:        channel.Username,
			Password:        channel.Password,
			Token:           channel.Token,
			InsecureSkipTLS: channel.InsecureSkipTLS,
		},
		Credential: credentialToDevops(credential),
	}
	if channel.ChannelType != "host" {
		return req, nil
	}
	var cfg hostGroupConfig
	_ = json.Unmarshal([]byte(channel.Config), &cfg)
	if cfg.HostID == "" {
		return req, nil
	}
	host, err := c.hostModel.FindOne(ctx, cfg.HostID)
	if err != nil {
		return req, err
	}
	devopsHost, err := c.hostToDevops(ctx, host)
	if err != nil {
		return req, err
	}
	req.Hosts = []devopstypes.Host{devopsHost}
	return req, nil
}

func credentialToDevops(credential *resolvedCredential) *devopstypes.Credential {
	if credential == nil || credential.data == nil {
		return nil
	}
	return &devopstypes.Credential{
		Type:        credential.data.CredentialType,
		Username:    credential.secret.Username,
		Password:    credential.secret.Password,
		Token:       credential.secret.Token,
		PrivateKey:  credential.secret.PrivateKey,
		Passphrase:  credential.secret.Passphrase,
		Kubeconfig:  credential.secret.Kubeconfig,
		SecretText:  credential.secret.SecretText,
		Certificate: credential.secret.Certificate,
		JsonData:    credential.secret.JsonData,
	}
}

func (c *Checker) hostToDevops(ctx context.Context, host *model.DevopsHost) (devopstypes.Host, error) {
	devopsHost := devopstypes.Host{
		ID:   host.ID.Hex(),
		Name: host.Name,
		IP:   host.IP,
		Port: host.Port,
	}
	credential, err := c.resolveCredential(ctx, host.CredentialID)
	if err != nil {
		return devopsHost, err
	}
	devopsHost.Credential = credentialToDevops(credential)
	return devopsHost, nil
}

func (c *Checker) resolveCredential(ctx context.Context, credentialID string) (*resolvedCredential, error) {
	if strings.TrimSpace(credentialID) == "" {
		return nil, nil
	}
	credential, err := c.credentialModel.FindOne(ctx, credentialID)
	if err != nil {
		return nil, err
	}
	secret, err := credential.Secret()
	if err != nil {
		return nil, err
	}
	return &resolvedCredential{data: credential, secret: secret}, nil
}

func testHTTP(ctx context.Context, channel *model.DevopsChannel, credential *resolvedCredential, checkedAt int64) Result {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: channel.InsecureSkipTLS}, //nolint:gosec
	}
	client := &http.Client{Timeout: 5 * time.Second, Transport: transport}
	resp, err := doHTTP(ctx, client, http.MethodHead, channel, credential)
	if err != nil || resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusNotImplemented {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		resp, err = doHTTP(ctx, client, http.MethodGet, channel, credential)
	}
	if err != nil {
		return unhealthy(fmt.Sprintf("连接失败: %s", trimMessage(err.Error())), checkedAt)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		metadata := channelMetadata(channel, map[string]any{"httpStatus": resp.StatusCode})
		return healthyWithMetadata(fmt.Sprintf("HTTP %d", resp.StatusCode), checkedAt, metadata)
	}
	return unhealthy(fmt.Sprintf("HTTP %d", resp.StatusCode), checkedAt)
}

func doHTTP(ctx context.Context, client *http.Client, method string, channel *model.DevopsChannel, credential *resolvedCredential) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, channel.Endpoint, http.NoBody)
	if err != nil {
		return nil, err
	}
	applyHTTPAuth(req, channel, credential)
	return client.Do(req)
}

func applyHTTPAuth(req *http.Request, channel *model.DevopsChannel, credential *resolvedCredential) {
	if credential != nil {
		switch credential.data.CredentialType {
		case "username_password":
			req.SetBasicAuth(credential.secret.Username, credential.secret.Password)
		case "token":
			if token := authutil.NormalizeBearerToken(channel.ChannelType, credential.secret.Token); token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
		case "secret_text":
			if token := authutil.NormalizeBearerToken(channel.ChannelType, credential.secret.SecretText); token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
		}
		return
	}
	switch channel.AuthType {
	case "username_password":
		req.SetBasicAuth(channel.Username, channel.Password)
	case "token":
		if token := authutil.NormalizeBearerToken(channel.ChannelType, channel.Token); token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}
}

func testKubernetes(ctx context.Context, channel *model.DevopsChannel, credential *resolvedCredential, checkedAt int64) Result {
	endpoint := strings.TrimSpace(channel.Endpoint)
	token := strings.TrimSpace(channel.Token)
	if credential != nil {
		switch credential.data.CredentialType {
		case "token":
			token = strings.TrimSpace(credential.secret.Token)
		case "kubeconfig":
			if parsed := kubeconfigValue(credential.secret.Kubeconfig, "server"); parsed != "" && (endpoint == "" || strings.HasPrefix(endpoint, "system://")) {
				endpoint = parsed
			}
			if token == "" {
				token = kubeconfigValue(credential.secret.Kubeconfig, "token")
			}
		}
	}
	if endpoint == "" || strings.HasPrefix(endpoint, "system://") {
		return unhealthy("Kubernetes API 地址为空", checkedAt)
	}
	versionURL := strings.TrimRight(endpoint, "/") + "/version"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, versionURL, http.NoBody)
	if err != nil {
		return unhealthy(err.Error(), checkedAt)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+authutil.NormalizeBearerToken(channel.ChannelType, token))
	}
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: channel.InsecureSkipTLS}, //nolint:gosec
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return unhealthy(fmt.Sprintf("连接失败: %s", trimMessage(err.Error())), checkedAt)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return Result{Success: true, HealthStatus: "healthy", Message: fmt.Sprintf("Kubernetes API %d", resp.StatusCode), CheckedAt: checkedAt}
	}
	return unhealthy(fmt.Sprintf("Kubernetes API %d", resp.StatusCode), checkedAt)
}

func kubeconfigValue(content, key string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, key+":") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, key+":"))
		return strings.Trim(value, `"'`)
	}
	return ""
}

func (c *Checker) testHostChannel(ctx context.Context, channel *model.DevopsChannel, credential *resolvedCredential, checkedAt int64) Result {
	var cfg hostGroupConfig
	_ = json.Unmarshal([]byte(channel.Config), &cfg)
	if cfg.HostID != "" {
		host, err := c.hostModel.FindOne(ctx, cfg.HostID)
		if err != nil {
			return unhealthy(fmt.Sprintf("主机资产不存在: %s", trimMessage(err.Error())), checkedAt)
		}
		return c.testHost(ctx, host, checkedAt)
	}
	if strings.TrimSpace(channel.Endpoint) == "" {
		return unhealthy("主机地址为空", checkedAt)
	}
	if credential != nil {
		return testHostAddress(ctx, channel.Endpoint, credential, checkedAt)
	}
	return testTCP(ctx, channel.Endpoint, checkedAt)
}

func (c *Checker) testHostGroup(ctx context.Context, channel *model.DevopsChannel, checkedAt int64) Result {
	var cfg hostGroupConfig
	if err := json.Unmarshal([]byte(channel.Config), &cfg); err != nil {
		return unhealthy("主机组配置不是有效 JSON", checkedAt)
	}
	if len(cfg.HostIDs) == 0 && cfg.HostID != "" {
		cfg.HostIDs = []string{cfg.HostID}
	}
	if len(cfg.HostIDs) == 0 {
		return unhealthy("主机组未选择主机资产", checkedAt)
	}
	failures := make([]string, 0)
	success := 0
	for _, hostID := range cfg.HostIDs {
		host, err := c.hostModel.FindOne(ctx, hostID)
		if err != nil {
			failures = append(failures, hostID+": 主机不存在")
			continue
		}
		result := c.testHost(ctx, host, checkedAt)
		name := host.Name
		if name == "" {
			name = host.IP
		}
		if result.Success {
			success++
			continue
		}
		failures = append(failures, fmt.Sprintf("%s: %s", name, result.Message))
	}
	if len(failures) == 0 {
		metadata := channelMetadata(channel, map[string]any{
			"hostGroup": true,
			"success":   success,
			"total":     len(cfg.HostIDs),
		})
		return healthyWithMetadata(fmt.Sprintf("主机组检测通过 %d/%d", success, len(cfg.HostIDs)), checkedAt, metadata)
	}
	return unhealthy(fmt.Sprintf("主机组检测通过 %d/%d，失败: %s", success, len(cfg.HostIDs), strings.Join(failures, "；")), checkedAt)
}

func (c *Checker) testHost(ctx context.Context, host *model.DevopsHost, checkedAt int64) Result {
	if host == nil {
		return unhealthy("主机资产为空", checkedAt)
	}
	if strings.TrimSpace(host.IP) == "" {
		return unhealthy("主机 IP 为空", checkedAt)
	}
	devopsHost, err := c.hostToDevops(ctx, host)
	if err != nil {
		return unhealthy(fmt.Sprintf("凭据读取失败: %s", trimMessage(err.Error())), checkedAt)
	}
	return fromDevopsResult(devopshost.New().TestConnection(ctx, devopstypes.Request{Hosts: []devopstypes.Host{devopsHost}}))
}

func testHostAddress(ctx context.Context, address string, credential *resolvedCredential, checkedAt int64) Result {
	if credential == nil {
		return testTCP(ctx, address, checkedAt)
	}
	switch credential.data.CredentialType {
	case "username_password", "ssh_key":
		return testSSH(ctx, address, credential, checkedAt)
	default:
		return testTCP(ctx, address, checkedAt)
	}
}

func testSSH(ctx context.Context, address string, credential *resolvedCredential, checkedAt int64) Result {
	authMethods := make([]ssh.AuthMethod, 0, 2)
	if credential.data.CredentialType == "username_password" && credential.secret.Password != "" {
		authMethods = append(authMethods, ssh.Password(credential.secret.Password))
	}
	if credential.data.CredentialType == "ssh_key" && credential.secret.PrivateKey != "" {
		signer, err := parseSSHSigner(credential.secret.PrivateKey, credential.secret.Passphrase)
		if err != nil {
			return unhealthy(fmt.Sprintf("SSH 私钥解析失败: %s", trimMessage(err.Error())), checkedAt)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}
	if credential.secret.Username == "" || len(authMethods) == 0 {
		return testTCP(ctx, address, checkedAt)
	}
	conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", address)
	if err != nil {
		return unhealthy(fmt.Sprintf("连接失败: %s", trimMessage(err.Error())), checkedAt)
	}
	defer conn.Close()
	config := &ssh.ClientConfig{
		User:            credential.secret.Username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec
		Timeout:         5 * time.Second,
	}
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, address, config)
	if err != nil {
		return unhealthy(fmt.Sprintf("SSH 认证失败: %s", trimMessage(err.Error())), checkedAt)
	}
	client := ssh.NewClient(sshConn, chans, reqs)
	_ = client.Close()
	return Result{Success: true, HealthStatus: "healthy", Message: "SSH 连接正常", CheckedAt: checkedAt}
}

func parseSSHSigner(privateKey, passphrase string) (ssh.Signer, error) {
	if passphrase != "" {
		return ssh.ParsePrivateKeyWithPassphrase([]byte(privateKey), []byte(passphrase))
	}
	return ssh.ParsePrivateKey([]byte(privateKey))
}

func testTCP(ctx context.Context, endpoint string, checkedAt int64) Result {
	address := endpoint
	if parsed, err := url.Parse(endpoint); err == nil && parsed.Host != "" {
		address = parsed.Host
	}
	if _, _, err := net.SplitHostPort(address); err != nil {
		return unhealthy("非 HTTP(S) 地址需要填写 host:port", checkedAt)
	}
	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return unhealthy(fmt.Sprintf("连接失败: %s", trimMessage(err.Error())), checkedAt)
	}
	_ = conn.Close()
	return Result{Success: true, HealthStatus: "healthy", Message: "TCP 连接正常", CheckedAt: checkedAt}
}

func testChannelTCP(ctx context.Context, channel *model.DevopsChannel, endpoint string, checkedAt int64) Result {
	result := testTCP(ctx, endpoint, checkedAt)
	if result.Success {
		metadata := channelMetadata(channel, map[string]any{"tcp": true})
		metadata.HealthMessage = result.Message
		metadata.LastCheckedAt = checkedAt
		result.Metadata = devopstypes.MetadataJSON(metadata)
	}
	return result
}

func healthyWithMetadata(message string, checkedAt int64, metadata devopstypes.Metadata) Result {
	message = trimMessage(message)
	metadata.HealthMessage = message
	metadata.LastCheckedAt = checkedAt
	return Result{
		Success:      true,
		HealthStatus: "healthy",
		Message:      message,
		CheckedAt:    checkedAt,
		Metadata:     devopstypes.MetadataJSON(metadata),
	}
}

func channelMetadata(channel *model.DevopsChannel, capabilities map[string]any) devopstypes.Metadata {
	if capabilities == nil {
		capabilities = map[string]any{}
	}
	channelType := ""
	config := ""
	if channel != nil {
		channelType = channel.ChannelType
		config = channel.Config
	}
	metadata := devopstypes.Metadata{
		ProductName:  channelProductName(channelType),
		Capabilities: capabilities,
	}
	cfg := parseChannelConfig(config)
	switch strings.TrimSpace(channelType) {
	case "trivy":
		metadata.EngineVersion = firstConfigString(cfg, "engineVersion", "version", "imageVersion")
		metadata.SchemaVersion = firstConfigString(cfg, "dbVersion", "vulnerabilityDbVersion", "schemaVersion")
		if metadata.EngineVersion == "" {
			metadata.EngineVersion = "待配置版本"
		}
	case "kube_bench":
		metadata.EngineVersion = firstConfigString(cfg, "engineVersion", "version", "imageVersion")
		metadata.BenchmarkProfile = firstConfigString(cfg, "profile", "benchmarkProfile")
		if metadata.EngineVersion == "" {
			metadata.EngineVersion = "待配置版本"
		}
		if metadata.BenchmarkProfile == "" {
			metadata.BenchmarkProfile = "cis"
		}
	case "spotbugs", "buildkit":
		metadata.EngineVersion = firstConfigString(cfg, "engineVersion", "version", "imageVersion", "toolVersion")
		if metadata.EngineVersion == "" {
			metadata.EngineVersion = "待配置版本"
		}
	case "registry", "aliyun_registry":
		metadata.APIVersion = "v2"
	case "github":
		metadata.APIVersion = "v3"
	case "gitee":
		metadata.APIVersion = "v5"
	case "tekton":
		metadata.APIVersion = "tekton.dev/v1"
	case "kubernetes":
		metadata.APIVersion = "v1"
	}
	return metadata
}

func parseChannelConfig(raw string) map[string]any {
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return map[string]any{}
	}
	return data
}

func firstConfigString(data map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := data[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func channelProductName(channelType string) string {
	switch strings.TrimSpace(channelType) {
	case "jenkins":
		return "Jenkins"
	case "tekton":
		return "Tekton"
	case "buildkit":
		return "BuildKit"
	case "gitlab":
		return "GitLab"
	case "github":
		return "GitHub"
	case "gitee":
		return "Gitee"
	case "svn":
		return "SVN"
	case "nexus":
		return "Nexus"
	case "jfrog":
		return "JFrog"
	case "harbor":
		return "Harbor"
	case "registry", "aliyun_registry":
		return "Docker Registry"
	case "sonarqube":
		return "SonarQube"
	case "spotbugs":
		return "SpotBugs"
	case "trivy":
		return "Trivy"
	case "kube_bench":
		return "kube-bench"
	case "kubernetes":
		return "Kubernetes"
	case "host":
		return "Host"
	case "host_group":
		return "Host Group"
	case "kube-nova", "kubenova":
		return "kube-nova"
	case "argocd":
		return "Argo CD"
	default:
		return strings.TrimSpace(channelType)
	}
}

func hostAddress(host *model.DevopsHost) string {
	port := host.Port
	if port == 0 {
		port = 22
	}
	return net.JoinHostPort(host.IP, strconv.FormatInt(port, 10))
}

func unhealthy(message string, checkedAt int64) Result {
	return Result{Success: false, HealthStatus: "unhealthy", Message: trimMessage(message), CheckedAt: checkedAt}
}

func trimMessage(message string) string {
	message = strings.TrimSpace(message)
	if len(message) > 240 {
		return message[:240]
	}
	return message
}

func closeBody(body io.Closer) {
	if body != nil {
		_ = body.Close()
	}
}
