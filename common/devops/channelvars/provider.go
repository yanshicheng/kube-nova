package channelvars

import (
	"context"
)

// Provider 渠道变量Provider接口
type Provider interface {
	// Capabilities 声明能力
	Capabilities() ProviderCapabilities

	// QueryOptions 查询选项
	QueryOptions(ctx context.Context, req *QueryOptionsRequest) (*QueryOptionsResponse, error)

	// ResolveAddress 解析地址
	ResolveAddress(ctx context.Context, req *ResolveAddressRequest) (*ResolveAddressResponse, error)

	// RenderOutput 渲染输出
	RenderOutput(ctx context.Context, req *RenderOutputRequest) (*RenderOutputResponse, error)

	// ValidateValue 校验值
	ValidateValue(ctx context.Context, req *ValidateValueRequest) error
}

// ProviderCapabilities Provider能力声明
type ProviderCapabilities struct {
	SupportedQueries        []string        // 支持的查询类型
	SupportedAddressFormats []AddressFormat // 支持的地址格式
	SupportsAddressResolve  bool            // 是否支持地址解析
	RequiresCredential      bool            // 是否需要凭证
}

// AddressFormat 地址格式
type AddressFormat struct {
	Field   string // 字段key
	Pattern string // 正则表达式
	Example string // 示例
}

// QueryOptionsRequest 查询选项请求
type QueryOptionsRequest struct {
	ChannelInstanceID int64             // 渠道实例ID
	ProviderKey       string            // Provider key
	ParentValues      map[string]string // 父级依赖值
	ProjectID         int64             // 项目ID（用于权限过滤）
	Search            *string           // 搜索关键词（可选）
}

// QueryOptionsResponse 查询选项响应
type QueryOptionsResponse struct {
	Options []Option // 选项列表
}

// ResolveAddressRequest 解析地址请求
type ResolveAddressRequest struct {
	Address         string // 地址
	ChannelTypeCode string // 渠道类型编码
	ProjectID       int64  // 项目ID（用于权限过滤）
}

// ResolveAddressResponse 解析地址响应
type ResolveAddressResponse struct {
	Resolved          bool              // 是否解析成功
	ChannelInstanceID int64             // 渠道实例ID
	Fields            map[string]string // 解析出的字段
	Reason            string            // 失败原因
}

// RenderOutputRequest 渲染输出请求
type RenderOutputRequest struct {
	ChannelInstanceID int64             // 渠道实例ID
	ProviderKey       string            // Provider key
	Values            map[string]string // 所有字段值
	ProjectID         int64             // 项目ID
}

// RenderOutputResponse 渲染输出响应
type RenderOutputResponse struct {
	Output string // 输出内容
}

// ValidateValueRequest 校验值请求
type ValidateValueRequest struct {
	ChannelInstanceID int64  // 渠道实例ID
	FieldKey          string // 字段key
	Value             string // 值
	ProjectID         int64  // 项目ID
}

// ProviderRegistry Provider注册表
type ProviderRegistry struct {
	providers map[string]Provider // key: providerType
}

var globalProviderRegistry = &ProviderRegistry{
	providers: make(map[string]Provider),
}

// GetProviderRegistry 获取全局Provider注册表
func GetProviderRegistry() *ProviderRegistry {
	return globalProviderRegistry
}

// Register 注册Provider
func (r *ProviderRegistry) Register(providerType string, provider Provider) {
	r.providers[providerType] = provider
}

// Get 获取Provider
func (r *ProviderRegistry) Get(providerType string) Provider {
	return r.providers[providerType]
}

// Has 判断Provider是否存在
func (r *ProviderRegistry) Has(providerType string) bool {
	_, ok := r.providers[providerType]
	return ok
}

// ChannelInstance 渠道实例（简化版，实际使用时从数据库查询）
type ChannelInstance struct {
	ID           int64
	Name         string
	Code         string
	Endpoint     string
	ChannelType  string
	ProviderType string
	CredentialID int64
	Config       map[string]interface{} // 渠道配置
}
