package providers

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
)

// HostProvider Host Provider实现
type HostProvider struct {
	channelManager channelvars.ChannelManager
}

// NewHostProvider 创建Host Provider
func NewHostProvider(channelManager channelvars.ChannelManager) *HostProvider {
	return &HostProvider{
		channelManager: channelManager,
	}
}

// Capabilities 声明能力
func (p *HostProvider) Capabilities() channelvars.ProviderCapabilities {
	return channelvars.ProviderCapabilities{
		SupportedQueries: []string{"host.hostConfig"},
		SupportedAddressFormats: []channelvars.AddressFormat{
			{Field: channelvars.FieldAddressHostConfig, Pattern: `^\{.*\}$`, Example: `{"host":"192.168.1.1","port":22,"username":"root","password":"password"}`},
		},
		SupportsAddressResolve: true,
		RequiresCredential:     true,
	}
}

// QueryOptions 查询选项
func (p *HostProvider) QueryOptions(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	return &channelvars.QueryOptionsResponse{
		Options: []channelvars.Option{},
	}, nil
}

// ResolveAddress 解析地址
func (p *HostProvider) ResolveAddress(ctx context.Context, req *channelvars.ResolveAddressRequest) (*channelvars.ResolveAddressResponse, error) {
	fields := map[string]string{channelvars.FieldAddressHostConfig: strings.TrimSpace(req.Address)}
	return resolveHostAddressByCandidates(ctx, p.channelManager, req, fields)
}

// RenderOutput 渲染输出
func (p *HostProvider) RenderOutput(ctx context.Context, req *channelvars.RenderOutputRequest) (*channelvars.RenderOutputResponse, error) {
	value := strings.TrimSpace(firstValue(req.Values, channelvars.FieldAddressHostConfig, "hostConfig"))
	if value == "" {
		return nil, fmt.Errorf("%s is required", channelvars.FieldAddressHostConfig)
	}
	hosts, err := parseHostConfigs(value)
	if err != nil {
		return nil, err
	}
	if len(hosts) == 0 {
		return nil, fmt.Errorf("%s is required", channelvars.FieldAddressHostConfig)
	}
	output, err := renderSingleHostConfig(hosts[0], firstValue(req.Values, channelvars.FieldOptionOutputFormat, "outputFormat"))
	if err != nil {
		return nil, err
	}
	return &channelvars.RenderOutputResponse{Output: output}, nil
}

// ValidateValue 校验值
func (p *HostProvider) ValidateValue(ctx context.Context, req *channelvars.ValidateValueRequest) error {
	return nil
}

func firstValue(values map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(values[key]); value != "" {
			return value
		}
	}
	return ""
}
