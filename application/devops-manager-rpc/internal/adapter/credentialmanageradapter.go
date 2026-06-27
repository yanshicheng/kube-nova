package adapter

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
)

// CredentialManagerAdapter 凭证管理器适配器
type CredentialManagerAdapter struct {
	credentialModel *model.DevopsCredentialModel
}

// NewCredentialManagerAdapter 创建凭证管理器适配器
func NewCredentialManagerAdapter(credentialModel *model.DevopsCredentialModel) *CredentialManagerAdapter {
	return &CredentialManagerAdapter{
		credentialModel: credentialModel,
	}
}

// GetCredential 获取凭证
func (a *CredentialManagerAdapter) GetCredential(ctx context.Context, id int64) (*channelvars.Credential, error) {
	// MongoDB使用string ID
	idStr := fmt.Sprintf("%d", id)
	credential, err := a.credentialModel.FindOne(ctx, idStr)
	if err != nil {
		return nil, fmt.Errorf("查询凭证失败: %w", err)
	}

	return &channelvars.Credential{
		ID:       id,
		Name:     credential.Name,
		Type:     credential.CredentialType,
		Username: credential.Username,
		Password: credential.Password,
		Token:    credential.Token,
		SSHKey:   credential.PrivateKey,
		Content:  credential.SecretText,
	}, nil
}
