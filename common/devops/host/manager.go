package host

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	devopstypes "github.com/yanshicheng/kube-nova/common/devops/types"
	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/crypto/ssh"
)

type Manager struct{}

func New() devopstypes.Provider {
	return Manager{}
}

func (Manager) TestConnection(ctx context.Context, req devopstypes.Request) devopstypes.Result {
	host := hostFromRequest(req)
	if host.IP == "" {
		return devopstypes.Unhealthy("主机 IP 为空", devopstypes.Metadata{ProductName: "Host"})
	}
	address := net.JoinHostPort(host.IP, strconv.FormatInt(defaultPort(host.Port), 10))
	credential := host.Credential
	if credential == nil {
		credential = req.Credential
	}
	if credential == nil || credential.Username == "" {
		return testTCP(ctx, address)
	}
	return testSSH(ctx, address, credential)
}

func hostFromRequest(req devopstypes.Request) devopstypes.Host {
	if len(req.Hosts) > 0 {
		return req.Hosts[0]
	}
	if parsed, err := url.Parse(req.Channel.Endpoint); err == nil && parsed.Host != "" {
		host, port := splitAddress(parsed.Host)
		return devopstypes.Host{IP: host, Port: port}
	}
	host, port := splitAddress(req.Channel.Endpoint)
	return devopstypes.Host{
		IP:   host,
		Port: port,
	}
}

func splitAddress(address string) (string, int64) {
	address = strings.TrimSpace(address)
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return address, 22
	}
	parsedPort, err := strconv.ParseInt(port, 10, 64)
	if err != nil || parsedPort == 0 {
		parsedPort = 22
	}
	return host, parsedPort
}

func defaultPort(port int64) int64 {
	if port == 0 {
		return 22
	}
	return port
}

func testTCP(ctx context.Context, address string) devopstypes.Result {
	conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", address)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/host/manager.go", err)
		return devopstypes.Unhealthy(fmt.Sprintf("连接失败: %s", devopstypes.TrimMessage(err.Error())), devopstypes.Metadata{ProductName: "Host"})
	}
	_ = conn.Close()
	return devopstypes.Healthy("TCP 连接正常", devopstypes.Metadata{
		ProductName:  "Host",
		Capabilities: map[string]any{"tcp": true},
	})
}

func testSSH(ctx context.Context, address string, credential *devopstypes.Credential) devopstypes.Result {
	authMethods := make([]ssh.AuthMethod, 0, 2)
	if credential.Type == "username_password" && credential.Password != "" {
		authMethods = append(authMethods, ssh.Password(credential.Password))
	}
	if credential.Type == "ssh_key" && credential.PrivateKey != "" {
		signer, err := parseSSHSigner(credential.PrivateKey, credential.Passphrase)
		if err != nil {
			logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/host/manager.go", err)
			return devopstypes.Unhealthy(fmt.Sprintf("SSH 私钥解析失败: %s", devopstypes.TrimMessage(err.Error())), devopstypes.Metadata{ProductName: "Host"})
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}
	if len(authMethods) == 0 {
		return testTCP(ctx, address)
	}
	conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", address)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/host/manager.go", err)
		return devopstypes.Unhealthy(fmt.Sprintf("连接失败: %s", devopstypes.TrimMessage(err.Error())), devopstypes.Metadata{ProductName: "Host"})
	}
	defer conn.Close()

	config := &ssh.ClientConfig{
		User:            credential.Username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec
		Timeout:         5 * time.Second,
	}
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, address, config)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/host/manager.go", err)
		return devopstypes.Unhealthy(fmt.Sprintf("SSH 认证失败: %s", devopstypes.TrimMessage(err.Error())), devopstypes.Metadata{ProductName: "Host"})
	}
	client := ssh.NewClient(sshConn, chans, reqs)
	defer client.Close()

	metadata := devopstypes.Metadata{
		ProductName:  "Host",
		AuthUser:     credential.Username,
		Capabilities: map[string]any{"ssh": true},
	}
	if output := runSSHCommand(client, "uname -srm"); output != "" {
		metadata.Capabilities["system"] = output
	}
	return devopstypes.Healthy("SSH 连接正常", metadata)
}

func parseSSHSigner(privateKey, passphrase string) (ssh.Signer, error) {
	if passphrase != "" {
		return ssh.ParsePrivateKeyWithPassphrase([]byte(privateKey), []byte(passphrase))
	}
	return ssh.ParsePrivateKey([]byte(privateKey))
}

func runSSHCommand(client *ssh.Client, command string) string {
	session, err := client.NewSession()
	if err != nil {
		return ""
	}
	defer session.Close()
	output, err := session.CombinedOutput(command)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}
