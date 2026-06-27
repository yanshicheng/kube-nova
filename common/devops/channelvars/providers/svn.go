package providers

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
)

// SVNProvider SVN Provider实现
type SVNProvider struct {
	channelManager channelvars.ChannelManager
}

// NewSVNProvider 创建SVN Provider
func NewSVNProvider(channelManager channelvars.ChannelManager) *SVNProvider {
	return &SVNProvider{
		channelManager: channelManager,
	}
}

// Capabilities 声明能力
func (p *SVNProvider) Capabilities() channelvars.ProviderCapabilities {
	return channelvars.ProviderCapabilities{
		SupportedQueries: []string{
			"svn.repository",
			"svn.path",
			"svn.revision",
		},
		SupportedAddressFormats: []channelvars.AddressFormat{
			{
				Field:   "address.projectUrl",
				Pattern: `^svn://[^/]+/.+$`,
				Example: "svn://svn.example.com/repo/trunk",
			},
		},
		SupportsAddressResolve: true,
		RequiresCredential:     true,
	}
}

// QueryOptions 查询选项
func (p *SVNProvider) QueryOptions(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	switch req.ProviderKey {
	case "svn.repository":
		return p.queryRepositories(ctx, req)
	case "svn.path":
		return p.queryPaths(ctx, req)
	case "svn.revision":
		return p.queryRevisions(ctx, req)
	default:
		return &channelvars.QueryOptionsResponse{
			Options: []channelvars.Option{},
		}, nil
	}
}

// queryRepositories 查询仓库列表
func (p *SVNProvider) queryRepositories(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	options := []channelvars.Option{
		{Label: "trunk", Value: "trunk"},
		{Label: "branches", Value: "branches"},
		{Label: "tags", Value: "tags"},
	}

	return &channelvars.QueryOptionsResponse{
		Options: options,
	}, nil
}

// queryPaths 查询路径列表
func (p *SVNProvider) queryPaths(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	options := []channelvars.Option{
		{Label: "/project1", Value: "/project1"},
		{Label: "/project2", Value: "/project2"},
	}

	return &channelvars.QueryOptionsResponse{
		Options: options,
	}, nil
}

// queryRevisions 查询版本列表
func (p *SVNProvider) queryRevisions(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	options := []channelvars.Option{
		{Label: "HEAD", Value: "HEAD"},
		{Label: "r1234", Value: "1234"},
		{Label: "r1233", Value: "1233"},
	}

	return &channelvars.QueryOptionsResponse{
		Options: options,
	}, nil
}

// ResolveAddress 解析地址
func (p *SVNProvider) ResolveAddress(ctx context.Context, req *channelvars.ResolveAddressRequest) (*channelvars.ResolveAddressResponse, error) {
	return &channelvars.ResolveAddressResponse{
		Resolved: false,
		Reason:   "svn address resolve not implemented",
	}, nil
}

// RenderOutput 渲染输出
func (p *SVNProvider) RenderOutput(ctx context.Context, req *channelvars.RenderOutputRequest) (*channelvars.RenderOutputResponse, error) {
	return nil, fmt.Errorf("svn provider does not support render output")
}

// ValidateValue 校验值
func (p *SVNProvider) ValidateValue(ctx context.Context, req *channelvars.ValidateValueRequest) error {
	return nil
}
