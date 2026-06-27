// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package project

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/client/projectservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type DevopsTektonSecretUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新项目 Tekton Secret
func NewDevopsTektonSecretUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsTektonSecretUpdateLogic {
	return &DevopsTektonSecretUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsTektonSecretUpdateLogic) DevopsTektonSecretUpdate(req *types.UpdateDevopsTektonSecretRequest) error {
	_, err := l.svcCtx.ProjectRpc.TektonSecretUpdate(l.ctx, &projectservice.SaveTektonSecretReq{
		ProjectId:     req.ProjectId,
		BindingId:     req.BindingId,
		Name:          req.Name,
		Type:          req.Type,
		Data:          req.Data,
		Operator:      currentUsername(l.ctx),
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})

	return err
}
