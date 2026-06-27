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

type DevopsTektonSecretCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建项目 Tekton Secret
func NewDevopsTektonSecretCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsTektonSecretCreateLogic {
	return &DevopsTektonSecretCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsTektonSecretCreateLogic) DevopsTektonSecretCreate(req *types.SaveDevopsTektonSecretRequest) error {
	_, err := l.svcCtx.ProjectRpc.TektonSecretCreate(l.ctx, &projectservice.SaveTektonSecretReq{
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
