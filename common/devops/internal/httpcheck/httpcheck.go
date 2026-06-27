package httpcheck

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/common/devops/authutil"
	devopstypes "github.com/yanshicheng/kube-nova/common/devops/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type Probe struct {
	Path         string
	PlainVersion bool
	APIVersion   string
	Capability   string
}

type ServiceProvider struct {
	ProductName string
	Probes      []Probe
}

func (p ServiceProvider) TestConnection(ctx context.Context, req devopstypes.Request) devopstypes.Result {
	endpoint := strings.TrimRight(strings.TrimSpace(req.Channel.Endpoint), "/")
	metadata := devopstypes.Metadata{
		ProductName:  p.ProductName,
		Capabilities: map[string]any{},
	}
	if endpoint == "" || strings.HasPrefix(endpoint, "system://") {
		return devopstypes.Unhealthy("访问地址为空", metadata)
	}
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		return devopstypes.Unhealthy("访问地址必须是 HTTP(S)", metadata)
	}

	probes := p.Probes
	if len(probes) == 0 {
		probes = []Probe{{Path: "/"}}
	}

	client := &http.Client{
		Timeout: 6 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: req.Channel.InsecureSkipTLS}, //nolint:gosec
		},
	}
	var lastMessage string
	var successMessage string
	for _, probe := range probes {
		result := p.probe(ctx, client, endpoint, probe, req, metadata)
		metadata = result.Metadata
		if result.Success {
			if successMessage == "" {
				successMessage = result.Message
			}
			continue
		}
		lastMessage = result.Message
	}
	if successMessage != "" {
		return devopstypes.Healthy(successMessage, metadata)
	}
	if lastMessage == "" {
		lastMessage = "连接失败"
	}
	return devopstypes.Unhealthy(lastMessage, metadata)
}

func (p ServiceProvider) probe(ctx context.Context, client *http.Client, endpoint string, probe Probe, req devopstypes.Request, metadata devopstypes.Metadata) devopstypes.Result {
	target := endpoint + "/" + strings.TrimLeft(probe.Path, "/")
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/internal/httpcheck/httpcheck.go", err)
		return devopstypes.Unhealthy(err.Error(), metadata)
	}
	ApplyAuth(request, req.Channel, req.Credential)
	resp, err := client.Do(request)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/internal/httpcheck/httpcheck.go", err)
		return devopstypes.Unhealthy(fmt.Sprintf("连接失败: %s", devopstypes.TrimMessage(err.Error())), metadata)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		setProbeStatus(metadata, probe, resp.StatusCode)
		return devopstypes.Unhealthy(fmt.Sprintf("HTTP %d", resp.StatusCode), metadata)
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if version := versionFromHeader(resp); version != "" {
		metadata.ProductVersion = version
	}
	if version := versionFromBody(body, probe.PlainVersion); version != "" {
		metadata.ProductVersion = version
	}
	if probe.APIVersion != "" {
		metadata.APIVersion = probe.APIVersion
	}
	if probe.Capability != "" {
		metadata.Capabilities[probe.Capability] = true
	}
	metadata.Capabilities["httpStatus"] = resp.StatusCode
	setProbeStatus(metadata, probe, resp.StatusCode)
	return devopstypes.Healthy(fmt.Sprintf("%s HTTP %d", p.ProductName, resp.StatusCode), metadata)
}

func setProbeStatus(metadata devopstypes.Metadata, probe Probe, status int) {
	if metadata.Capabilities == nil || probe.Capability == "" {
		return
	}
	metadata.Capabilities[probe.Capability+"Status"] = status
}

func ApplyAuth(req *http.Request, channel devopstypes.Channel, credential *devopstypes.Credential) {
	if credential != nil {
		switch credential.Type {
		case "username_password":
			req.SetBasicAuth(credential.Username, credential.Password)
		case "token":
			applyToken(req, channel.Type, credential.Token)
		case "secret_text":
			applyToken(req, channel.Type, credential.SecretText)
		}
		return
	}
	switch channel.AuthType {
	case "username_password":
		req.SetBasicAuth(channel.Username, channel.Password)
	case "token":
		applyToken(req, channel.Type, channel.Token)
	}
}

func applyToken(req *http.Request, channelType, token string) {
	token = authutil.NormalizeBearerToken(channelType, token)
	if token == "" {
		return
	}
	if channelType == "gitlab" {
		req.Header.Set("PRIVATE-TOKEN", token)
	}
	req.Header.Set("Authorization", "Bearer "+token)
}

func versionFromHeader(resp *http.Response) string {
	for _, key := range []string{"X-Jenkins", "X-Harbor-Version", "X-Artifactory-Version", "X-Sonar-Version"} {
		if value := strings.TrimSpace(resp.Header.Get(key)); value != "" {
			return value
		}
	}
	if version := versionFromServerHeader(resp.Header.Get("Server")); version != "" {
		return version
	}
	return ""
}

func versionFromBody(body []byte, plain bool) string {
	text := strings.TrimSpace(string(body))
	if text == "" {
		return ""
	}
	var data any
	if err := json.Unmarshal(body, &data); err == nil {
		if version := findStringValue(data, "version", "Version", "gitVersion", "harbor_version"); version != "" {
			return version
		}
	}
	if plain && len(text) < 80 {
		return text
	}
	return ""
}

func versionFromServerHeader(server string) string {
	for _, part := range strings.Fields(strings.TrimSpace(server)) {
		if version := strings.TrimPrefix(part, "Nexus/"); version != part {
			return strings.Trim(version, " ;,()")
		}
	}
	return ""
}

func findStringValue(data any, keys ...string) string {
	switch value := data.(type) {
	case map[string]any:
		for _, key := range keys {
			if item, ok := value[key].(string); ok && strings.TrimSpace(item) != "" {
				return strings.TrimSpace(item)
			}
		}
		for _, item := range value {
			if found := findStringValue(item, keys...); found != "" {
				return found
			}
		}
	case []any:
		for _, item := range value {
			if found := findStringValue(item, keys...); found != "" {
				return found
			}
		}
	}
	return ""
}
