package providers

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
)

// BuildKitProvider BuildKit Provider实现
type BuildKitProvider struct {
	channelManager channelvars.ChannelManager
}

// NewBuildKitProvider 创建BuildKit Provider
func NewBuildKitProvider(channelManager channelvars.ChannelManager) *BuildKitProvider {
	return &BuildKitProvider{
		channelManager: channelManager,
	}
}

// Capabilities 声明能力
func (p *BuildKitProvider) Capabilities() channelvars.ProviderCapabilities {
	return channelvars.ProviderCapabilities{
		SupportedQueries:        []string{},
		SupportedAddressFormats: []channelvars.AddressFormat{},
		SupportsAddressResolve:  false,
		RequiresCredential:      false,
	}
}

// QueryOptions 查询选项
func (p *BuildKitProvider) QueryOptions(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	return &channelvars.QueryOptionsResponse{
		Options: []channelvars.Option{},
	}, nil
}

// ResolveAddress 解析地址
func (p *BuildKitProvider) ResolveAddress(ctx context.Context, req *channelvars.ResolveAddressRequest) (*channelvars.ResolveAddressResponse, error) {
	return &channelvars.ResolveAddressResponse{
		Resolved: false,
		Reason:   "buildkit provider does not support address resolve",
	}, nil
}

// RenderOutput 渲染输出
func (p *BuildKitProvider) RenderOutput(ctx context.Context, req *channelvars.RenderOutputRequest) (*channelvars.RenderOutputResponse, error) {
	return nil, fmt.Errorf("buildkit provider does not support render output")
}

// ValidateValue 校验值
func (p *BuildKitProvider) ValidateValue(ctx context.Context, req *channelvars.ValidateValueRequest) error {
	return nil
}
