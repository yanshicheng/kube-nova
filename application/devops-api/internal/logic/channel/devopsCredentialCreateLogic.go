// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package channel

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/client/channelservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type DevopsCredentialCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建凭据
func NewDevopsCredentialCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsCredentialCreateLogic {
	return &DevopsCredentialCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsCredentialCreateLogic) DevopsCredentialCreate(req *types.CreateDevopsCredentialRequest) (resp *types.IdResponse, err error) {
	result, err := l.svcCtx.ChannelRpc.CredentialCreate(l.ctx, &channelservice.CreateCredentialReq{
		Name:             req.Name,
		Code:             req.Code,
		CredentialType:   req.CredentialType,
		Username:         req.Username,
		Password:         req.Password,
		Token:            req.Token,
		PrivateKey:       req.PrivateKey,
		Passphrase:       req.Passphrase,
		Kubeconfig:       req.Kubeconfig,
		SecretText:       req.SecretText,
		Certificate:      req.Certificate,
		JsonData:         req.JsonData,
		Description:      req.Description,
		Status:           req.Status,
		IsSystem:         req.IsSystem,
		Scope:            req.Scope,
		ProjectId:        req.ProjectId,
		ChannelGroupCode: req.ChannelGroupCode,
		ChannelType:      req.ChannelType,
		CurrentUserId:    currentUserID(l.ctx),
		CurrentRoles:     currentRoles(l.ctx),
		CreatedBy:        currentUsername(l.ctx),
	})
	if err != nil {
		return nil, err
	}

	return &types.IdResponse{Id: result.Id}, nil
}
