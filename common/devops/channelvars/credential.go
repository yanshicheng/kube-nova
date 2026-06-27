package channelvars

import (
	"context"
	"fmt"
)

// CredentialResolver 凭证解析器
type CredentialResolver struct {
	channelManager    ChannelManager
	credentialManager CredentialManager
	addressResolver   *AddressResolver
}

// NewCredentialResolver 创建凭证解析器
func NewCredentialResolver(channelManager ChannelManager, credentialManager CredentialManager) *CredentialResolver {
	return &CredentialResolver{
		channelManager:    channelManager,
		credentialManager: credentialManager,
		addressResolver:   NewAddressResolver(channelManager),
	}
}

// ResolveCredentialRequest 解析凭证请求
type ResolveCredentialRequest struct {
	EndpointID      int64  // 渠道实例ID
	Address         string // 地址
	ChannelTypeCode string // 渠道类型编码
	ProjectID       int64  // 项目ID
}

// Resolve 解析凭证
func (r *CredentialResolver) Resolve(ctx context.Context, req *ResolveCredentialRequest) (*Credential, error) {
	// 策略1：如果有endpoint，直接查询关联的凭证
	if req.EndpointID > 0 {
		instance, err := r.channelManager.GetInstance(ctx, req.EndpointID)
		if err != nil {
			return nil, fmt.Errorf("get channel instance: %w", err)
		}

		if instance.CredentialID > 0 {
			return r.credentialManager.GetCredential(ctx, instance.CredentialID)
		}

		return nil, fmt.Errorf("channel instance %d has no credential", req.EndpointID)
	}

	// 策略2：如果有address，尝试反查endpoint
	if req.Address != "" {
		resolveResp, err := r.addressResolver.Resolve(ctx, &ResolveAddressRequest{
			Address:         req.Address,
			ChannelTypeCode: req.ChannelTypeCode,
			ProjectID:       req.ProjectID,
		})
		if err != nil {
			return nil, fmt.Errorf("resolve address: %w", err)
		}

		if !resolveResp.Resolved {
			return nil, fmt.Errorf("cannot resolve address to endpoint: %s", resolveResp.Reason)
		}

		// 递归调用，使用解析出的endpoint
		return r.Resolve(ctx, &ResolveCredentialRequest{
			EndpointID: resolveResp.ChannelInstanceID,
			ProjectID:  req.ProjectID,
		})
	}

	return nil, fmt.Errorf("no endpoint or address provided")
}

// CredentialManager 凭证管理器接口
type CredentialManager interface {
	// GetCredential 获取凭证
	GetCredential(ctx context.Context, id int64) (*Credential, error)
}

// Credential 凭证
type Credential struct {
	ID       int64
	Name     string
	Type     string // username_password/token/ssh_key/kubeconfig
	Username string
	Password string
	Token    string
	SSHKey   string
	Content  string // 通用内容字段
}
