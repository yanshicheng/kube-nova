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

type DevopsCredentialUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新凭据
func NewDevopsCredentialUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsCredentialUpdateLogic {
	return &DevopsCredentialUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsCredentialUpdateLogic) DevopsCredentialUpdate(req *types.UpdateDevopsCredentialRequest) error {
	_, err := l.svcCtx.ChannelRpc.CredentialUpdate(l.ctx, &channelservice.UpdateCredentialReq{
		Id:               req.Id,
		Name:             req.Name,
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
		Scope:            req.Scope,
		ProjectId:        req.ProjectId,
		ChannelGroupCode: req.ChannelGroupCode,
		ChannelType:      req.ChannelType,
		CurrentUserId:    currentUserID(l.ctx),
		CurrentRoles:     currentRoles(l.ctx),
		UpdatedBy:        currentUsername(l.ctx),
	})
	return err
}
