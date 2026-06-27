package providers

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
)

// KubeNovaProvider KubeNova Provider实现
type KubeNovaProvider struct {
	channelManager channelvars.ChannelManager
}

// NewKubeNovaProvider 创建KubeNova Provider
func NewKubeNovaProvider(channelManager channelvars.ChannelManager) *KubeNovaProvider {
	return &KubeNovaProvider{
		channelManager: channelManager,
	}
}

// Capabilities 声明能力
func (p *KubeNovaProvider) Capabilities() channelvars.ProviderCapabilities {
	return channelvars.ProviderCapabilities{
		SupportedQueries:        []string{},
		SupportedAddressFormats: []channelvars.AddressFormat{},
		SupportsAddressResolve:  false,
		RequiresCredential:      true,
	}
}

// QueryOptions 查询选项
func (p *KubeNovaProvider) QueryOptions(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	return &channelvars.QueryOptionsResponse{
		Options: []channelvars.Option{},
	}, nil
}

// ResolveAddress 解析地址
func (p *KubeNovaProvider) ResolveAddress(ctx context.Context, req *channelvars.ResolveAddressRequest) (*channelvars.ResolveAddressResponse, error) {
	return &channelvars.ResolveAddressResponse{
		Resolved: false,
		Reason:   "kubenova provider does not support address resolve",
	}, nil
}

// RenderOutput 渲染输出
func (p *KubeNovaProvider) RenderOutput(ctx context.Context, req *channelvars.RenderOutputRequest) (*channelvars.RenderOutputResponse, error) {
	return nil, fmt.Errorf("kubenova provider does not support render output")
}

// ValidateValue 校验值
func (p *KubeNovaProvider) ValidateValue(ctx context.Context, req *channelvars.ValidateValueRequest) error {
	return nil
}
