package channelvars

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// AddressResolver 地址解析器
type AddressResolver struct {
	providerRegistry *ProviderRegistry
	channelManager   ChannelManager
}

// NewAddressResolver 创建地址解析器
func NewAddressResolver(channelManager ChannelManager) *AddressResolver {
	return &AddressResolver{
		providerRegistry: GetProviderRegistry(),
		channelManager:   channelManager,
	}
}

// Resolve 解析地址
func (r *AddressResolver) Resolve(ctx context.Context, req *ResolveAddressRequest) (*ResolveAddressResponse, error) {
	// 1. 根据渠道类型获取Provider
	channelType, err := r.channelManager.GetChannelTypeByCode(ctx, req.ChannelTypeCode)
	if err != nil {
		return nil, fmt.Errorf("get channel type: %w", err)
	}

	provider := r.providerRegistry.Get(channelType.ProviderType)
	if provider == nil {
		return &ResolveAddressResponse{
			Resolved: false,
			Reason:   fmt.Sprintf("provider not found: %s", channelType.ProviderType),
		}, nil
	}

	// 2. 检查Provider是否支持地址解析
	capabilities := provider.Capabilities()
	if !capabilities.SupportsAddressResolve {
		return &ResolveAddressResponse{
			Resolved: false,
			Reason:   fmt.Sprintf("provider %s does not support address resolve", channelType.ProviderType),
		}, nil
	}

	// 3. 调用Provider解析地址
	return provider.ResolveAddress(ctx, req)
}

// ChannelManager 渠道管理器接口
type ChannelManager interface {
	// GetChannelTypeByCode 根据编码获取渠道类型
	GetChannelTypeByCode(ctx context.Context, code string) (*ChannelType, error)

	// GetInstance 获取渠道实例
	GetInstance(ctx context.Context, id int64) (*ChannelInstance, error)

	// FindInstancesByEndpoint 根据endpoint查找渠道实例
	FindInstancesByEndpoint(ctx context.Context, endpoint string, projectID int64) ([]*ChannelInstance, error)
}

// ChannelType 渠道类型
type ChannelType struct {
	ID           string
	Code         string
	Name         string
	GroupCode    string
	SpecProfile  string
	ProviderType string
}

// ParseURL 解析URL
func ParseURL(address string) (*url.URL, error) {
	u, err := url.Parse(address)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	return u, nil
}

// ExtractHost 提取主机名
func ExtractHost(address string) (string, error) {
	u, err := ParseURL(address)
	if err != nil {
		return "", err
	}
	return u.Host, nil
}

// ExtractPath 提取路径
func ExtractPath(address string) (string, error) {
	u, err := ParseURL(address)
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(u.Path, "/"), nil
}

// MatchPattern 匹配地址格式
func MatchPattern(address, pattern string) bool {
	if pattern == "" {
		return false
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}

	return re.MatchString(address)
}

// NormalizeGitURL 规范化Git URL
func NormalizeGitURL(address string) string {
	// 移除 .git 后缀
	address = strings.TrimSuffix(address, ".git")

	// 移除尾部斜杠
	address = strings.TrimSuffix(address, "/")

	return address
}

// NormalizeImageURL 规范化镜像URL
func NormalizeImageURL(address string) string {
	// 移除协议前缀
	address = strings.TrimPrefix(address, "http://")
	address = strings.TrimPrefix(address, "https://")

	// 移除尾部斜杠
	address = strings.TrimSuffix(address, "/")

	return address
}
