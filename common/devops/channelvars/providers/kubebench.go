package providers

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
)

// KubeBenchProvider KubeBench Provider实现
type KubeBenchProvider struct {
	channelManager channelvars.ChannelManager
}

// NewKubeBenchProvider 创建KubeBench Provider
func NewKubeBenchProvider(channelManager channelvars.ChannelManager) *KubeBenchProvider {
	return &KubeBenchProvider{
		channelManager: channelManager,
	}
}

// Capabilities 声明能力
func (p *KubeBenchProvider) Capabilities() channelvars.ProviderCapabilities {
	return channelvars.ProviderCapabilities{
		SupportedQueries:        []string{},
		SupportedAddressFormats: []channelvars.AddressFormat{},
		SupportsAddressResolve:  false,
		RequiresCredential:      false,
	}
}

// QueryOptions 查询选项
func (p *KubeBenchProvider) QueryOptions(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	return &channelvars.QueryOptionsResponse{
		Options: []channelvars.Option{},
	}, nil
}

// ResolveAddress 解析地址
func (p *KubeBenchProvider) ResolveAddress(ctx context.Context, req *channelvars.ResolveAddressRequest) (*channelvars.ResolveAddressResponse, error) {
	return &channelvars.ResolveAddressResponse{
		Resolved: false,
		Reason:   "kubebench provider does not support address resolve",
	}, nil
}

// RenderOutput 渲染输出
func (p *KubeBenchProvider) RenderOutput(ctx context.Context, req *channelvars.RenderOutputRequest) (*channelvars.RenderOutputResponse, error) {
	return nil, fmt.Errorf("kubebench provider does not support render output")
}

// ValidateValue 校验值
func (p *KubeBenchProvider) ValidateValue(ctx context.Context, req *channelvars.ValidateValueRequest) error {
	return nil
}
