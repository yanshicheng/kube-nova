package providers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
)

const (
	hostOutputJSON          = "json"
	hostOutputJSONBase64    = "jsonBase64"
	hostOutputAnsible       = "ansible"
	hostOutputAnsibleBase64 = "ansibleBase64"
)

type hostAccessConfig struct {
	Host     string `json:"host"`
	Port     int64  `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func normalizeHostOutputFormat(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "normal", "json":
		return hostOutputJSON
	case "jsonbase64":
		return hostOutputJSONBase64
	case "inventory", "ansible":
		return hostOutputAnsible
	case "ansiblebase64":
		return hostOutputAnsibleBase64
	default:
		return strings.TrimSpace(value)
	}
}

func renderHostConfigs(hosts []hostAccessConfig, outputFormat string) (string, error) {
	outputFormat = normalizeHostOutputFormat(outputFormat)
	switch outputFormat {
	case hostOutputJSON:
		data, err := json.MarshalIndent(hosts, "", "  ")
		if err != nil {
			return "", err
		}
		return string(data), nil
	case hostOutputJSONBase64:
		value, err := renderHostConfigs(hosts, hostOutputJSON)
		if err != nil {
			return "", err
		}
		return base64.StdEncoding.EncodeToString([]byte(value)), nil
	case hostOutputAnsible:
		return renderHostConfigsAnsible(hosts), nil
	case hostOutputAnsibleBase64:
		value, err := renderHostConfigs(hosts, hostOutputAnsible)
		if err != nil {
			return "", err
		}
		return base64.StdEncoding.EncodeToString([]byte(value)), nil
	default:
		return "", fmt.Errorf("unsupported output format: %s", outputFormat)
	}
}

func renderSingleHostConfig(host hostAccessConfig, outputFormat string) (string, error) {
	outputFormat = normalizeHostOutputFormat(outputFormat)
	if outputFormat == hostOutputJSON || outputFormat == hostOutputJSONBase64 {
		data, err := json.MarshalIndent(host, "", "  ")
		if err != nil {
			return "", err
		}
		if outputFormat == hostOutputJSONBase64 {
			return base64.StdEncoding.EncodeToString(data), nil
		}
		return string(data), nil
	}
	return renderHostConfigs([]hostAccessConfig{host}, outputFormat)
}

func renderHostConfigsAnsible(hosts []hostAccessConfig) string {
	var builder strings.Builder
	builder.WriteString("[all]\n")
	for _, host := range hosts {
		name := strings.TrimSpace(host.Host)
		fmt.Fprintf(&builder, "%s ansible_ssh_host=%s ansible_ssh_port=%d ansible_ssh_user=%s ansible_ssh_pass=%s\n",
			name, host.Host, normalizeHostPort(host.Port), host.Username, host.Password)
	}
	return strings.TrimRight(builder.String(), "\n")
}

func parseHostConfigs(value string) ([]hostAccessConfig, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if decoded, ok := decodeBase64Text(value); ok {
		value = decoded
	}
	if strings.HasPrefix(value, "[all]") {
		return parseAnsibleInventory(value), nil
	}
	if strings.HasPrefix(value, "[") {
		var hosts []hostAccessConfig
		if err := json.Unmarshal([]byte(value), &hosts); err != nil {
			return nil, err
		}
		normalizeHostConfigs(hosts)
		return hosts, nil
	}
	var host hostAccessConfig
	if err := json.Unmarshal([]byte(value), &host); err != nil {
		return nil, err
	}
	host.Host = strings.TrimSpace(host.Host)
	host.Username = strings.TrimSpace(host.Username)
	host.Password = strings.TrimSpace(host.Password)
	host.Port = normalizeHostPort(host.Port)
	return []hostAccessConfig{host}, nil
}

func normalizeHostConfigs(hosts []hostAccessConfig) {
	for i := range hosts {
		hosts[i].Host = strings.TrimSpace(hosts[i].Host)
		hosts[i].Username = strings.TrimSpace(hosts[i].Username)
		hosts[i].Password = strings.TrimSpace(hosts[i].Password)
		hosts[i].Port = normalizeHostPort(hosts[i].Port)
	}
}

func normalizeHostPort(port int64) int64 {
	if port <= 0 {
		return 22
	}
	return port
}

func decodeBase64Text(value string) (string, bool) {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return "", false
	}
	text := strings.TrimSpace(string(decoded))
	return text, text != ""
}

func parseAnsibleInventory(value string) []hostAccessConfig {
	lines := strings.Split(value, "\n")
	result := make([]hostAccessConfig, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "[") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		host := hostAccessConfig{Host: fields[0], Port: 22}
		for _, field := range fields[1:] {
			key, val, ok := strings.Cut(field, "=")
			if !ok {
				continue
			}
			switch strings.TrimSpace(key) {
			case "ansible_ssh_host", "ansible_host":
				host.Host = strings.TrimSpace(val)
			case "ansible_ssh_port", "ansible_port":
				if port, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64); err == nil {
					host.Port = port
				}
			case "ansible_ssh_user", "ansible_user":
				host.Username = strings.TrimSpace(val)
			case "ansible_ssh_pass", "ansible_password":
				host.Password = strings.TrimSpace(val)
			}
		}
		host.Port = normalizeHostPort(host.Port)
		if strings.TrimSpace(host.Host) != "" {
			result = append(result, host)
		}
	}
	return result
}

func hostConfigEndpointCandidates(value string) []string {
	hosts, _ := parseHostConfigs(value)
	result := make([]string, 0, len(hosts)+2)
	for _, host := range hosts {
		if strings.TrimSpace(host.Host) != "" {
			result = append(result, host.Host)
			if host.Port > 0 {
				result = append(result, net.JoinHostPort(host.Host, strconv.FormatInt(host.Port, 10)))
			}
		}
	}
	return uniqueStrings(result)
}

func resolveHostAddressByCandidates(ctx context.Context, channelManager channelvars.ChannelManager, req *channelvars.ResolveAddressRequest, fields map[string]string) (*channelvars.ResolveAddressResponse, error) {
	for _, candidate := range hostConfigEndpointCandidates(req.Address) {
		instances, err := channelManager.FindInstancesByEndpoint(ctx, candidate, req.ProjectID)
		if err != nil {
			return nil, fmt.Errorf("find instances by endpoint: %w", err)
		}
		if len(instances) > 0 {
			return &channelvars.ResolveAddressResponse{
				Resolved:          true,
				ChannelInstanceID: instances[0].ID,
				Fields:            fields,
			}, nil
		}
	}
	return &channelvars.ResolveAddressResponse{
		Resolved: false,
		Reason:   "no channel instance found for host config",
	}, nil
}

func uniqueStrings(items []string) []string {
	result := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return result
}
