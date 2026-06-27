package providers

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
)

// TrivyProvider Trivy Provider实现
type TrivyProvider struct {
	channelManager channelvars.ChannelManager
}

// NewTrivyProvider 创建Trivy Provider
func NewTrivyProvider(channelManager channelvars.ChannelManager) *TrivyProvider {
	return &TrivyProvider{
		channelManager: channelManager,
	}
}

// Capabilities 声明能力
func (p *TrivyProvider) Capabilities() channelvars.ProviderCapabilities {
	return channelvars.ProviderCapabilities{
		SupportedQueries:        []string{},
		SupportedAddressFormats: []channelvars.AddressFormat{},
		SupportsAddressResolve:  false,
		RequiresCredential:      false,
	}
}

// QueryOptions 查询选项
func (p *TrivyProvider) QueryOptions(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	return &channelvars.QueryOptionsResponse{
		Options: []channelvars.Option{},
	}, nil
}

// ResolveAddress 解析地址
func (p *TrivyProvider) ResolveAddress(ctx context.Context, req *channelvars.ResolveAddressRequest) (*channelvars.ResolveAddressResponse, error) {
	return &channelvars.ResolveAddressResponse{
		Resolved: false,
		Reason:   "trivy provider does not support address resolve",
	}, nil
}

// RenderOutput 渲染输出
func (p *TrivyProvider) RenderOutput(ctx context.Context, req *channelvars.RenderOutputRequest) (*channelvars.RenderOutputResponse, error) {
	return nil, fmt.Errorf("trivy provider does not support render output")
}

// ValidateValue 校验值
func (p *TrivyProvider) ValidateValue(ctx context.Context, req *channelvars.ValidateValueRequest) error {
	return nil
}
