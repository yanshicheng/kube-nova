package providers

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
)

// ArgoCDProvider ArgoCD Provider实现
type ArgoCDProvider struct {
	channelManager channelvars.ChannelManager
}

// NewArgoCDProvider 创建ArgoCD Provider
func NewArgoCDProvider(channelManager channelvars.ChannelManager) *ArgoCDProvider {
	return &ArgoCDProvider{
		channelManager: channelManager,
	}
}

// Capabilities 声明能力
func (p *ArgoCDProvider) Capabilities() channelvars.ProviderCapabilities {
	return channelvars.ProviderCapabilities{
		SupportedQueries: []string{
			"argocd.project",
			"argocd.application",
		},
		SupportedAddressFormats: []channelvars.AddressFormat{
			{
				Field:   "address.projectUrl",
				Pattern: `^https?://[^/]+/settings/projects/.+$`,
				Example: "https://argocd.example.com/settings/projects/default",
			},
		},
		SupportsAddressResolve: true,
		RequiresCredential:     true,
	}
}

// QueryOptions 查询选项
func (p *ArgoCDProvider) QueryOptions(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	switch req.ProviderKey {
	case "argocd.project":
		return p.queryProjects(ctx, req)
	case "argocd.application":
		return p.queryApplications(ctx, req)
	default:
		return &channelvars.QueryOptionsResponse{
			Options: []channelvars.Option{},
		}, nil
	}
}

// queryProjects 查询项目列表
func (p *ArgoCDProvider) queryProjects(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	options := []channelvars.Option{
		{Label: "default", Value: "default"},
		{Label: "production", Value: "production"},
		{Label: "staging", Value: "staging"},
	}

	return &channelvars.QueryOptionsResponse{
		Options: options,
	}, nil
}

// queryApplications 查询应用列表
func (p *ArgoCDProvider) queryApplications(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	options := []channelvars.Option{
		{Label: "my-app", Value: "my-app"},
		{Label: "web-service", Value: "web-service"},
		{Label: "api-gateway", Value: "api-gateway"},
	}

	return &channelvars.QueryOptionsResponse{
		Options: options,
	}, nil
}

// ResolveAddress 解析地址
func (p *ArgoCDProvider) ResolveAddress(ctx context.Context, req *channelvars.ResolveAddressRequest) (*channelvars.ResolveAddressResponse, error) {
	return &channelvars.ResolveAddressResponse{
		Resolved: false,
		Reason:   "argocd address resolve not implemented",
	}, nil
}

// RenderOutput 渲染输出
func (p *ArgoCDProvider) RenderOutput(ctx context.Context, req *channelvars.RenderOutputRequest) (*channelvars.RenderOutputResponse, error) {
	return nil, fmt.Errorf("argocd provider does not support render output")
}

// ValidateValue 校验值
func (p *ArgoCDProvider) ValidateValue(ctx context.Context, req *channelvars.ValidateValueRequest) error {
	return nil
}
