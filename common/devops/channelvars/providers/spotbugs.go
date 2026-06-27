package providers

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
)

// SpotBugsProvider SpotBugs Provider实现
type SpotBugsProvider struct {
	channelManager channelvars.ChannelManager
}

// NewSpotBugsProvider 创建SpotBugs Provider
func NewSpotBugsProvider(channelManager channelvars.ChannelManager) *SpotBugsProvider {
	return &SpotBugsProvider{
		channelManager: channelManager,
	}
}

// Capabilities 声明能力
func (p *SpotBugsProvider) Capabilities() channelvars.ProviderCapabilities {
	return channelvars.ProviderCapabilities{
		SupportedQueries:        []string{},
		SupportedAddressFormats: []channelvars.AddressFormat{},
		SupportsAddressResolve:  false,
		RequiresCredential:      false,
	}
}

// QueryOptions 查询选项
func (p *SpotBugsProvider) QueryOptions(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	return &channelvars.QueryOptionsResponse{
		Options: []channelvars.Option{},
	}, nil
}

// ResolveAddress 解析地址
func (p *SpotBugsProvider) ResolveAddress(ctx context.Context, req *channelvars.ResolveAddressRequest) (*channelvars.ResolveAddressResponse, error) {
	return &channelvars.ResolveAddressResponse{
		Resolved: false,
		Reason:   "spotbugs provider does not support address resolve",
	}, nil
}

// RenderOutput 渲染输出
func (p *SpotBugsProvider) RenderOutput(ctx context.Context, req *channelvars.RenderOutputRequest) (*channelvars.RenderOutputResponse, error) {
	return nil, fmt.Errorf("spotbugs provider does not support render output")
}

// ValidateValue 校验值
func (p *SpotBugsProvider) ValidateValue(ctx context.Context, req *channelvars.ValidateValueRequest) error {
	return nil
}
