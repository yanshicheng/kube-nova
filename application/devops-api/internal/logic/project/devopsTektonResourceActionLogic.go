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

type DevopsTektonResourceActionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 执行项目 Tekton 资源动作
func NewDevopsTektonResourceActionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsTektonResourceActionLogic {
	return &DevopsTektonResourceActionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsTektonResourceActionLogic) DevopsTektonResourceAction(req *types.ActionDevopsTektonResourceRequest) error {
	_, err := l.svcCtx.ProjectRpc.TektonResourceAction(l.ctx, &projectservice.ActionTektonResourceReq{
		ProjectId:     req.ProjectId,
		BindingId:     req.BindingId,
		ResourceType:  req.ResourceType,
		Name:          req.Name,
		Action:        req.Action,
		Operator:      currentUsername(l.ctx),
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	return err
}
