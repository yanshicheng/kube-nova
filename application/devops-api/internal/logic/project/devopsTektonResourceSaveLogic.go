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

type DevopsTektonResourceSaveLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 保存项目 Tekton 资源
func NewDevopsTektonResourceSaveLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsTektonResourceSaveLogic {
	return &DevopsTektonResourceSaveLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsTektonResourceSaveLogic) DevopsTektonResourceSave(req *types.SaveDevopsTektonResourceRequest) error {
	_, err := l.svcCtx.ProjectRpc.TektonResourceSave(l.ctx, &projectservice.SaveTektonResourceReq{
		ProjectId:     req.ProjectId,
		BindingId:     req.BindingId,
		ResourceType:  req.ResourceType,
		Name:          req.Name,
		Yaml:          req.Yaml,
		Operator:      currentUsername(l.ctx),
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	return err
}
