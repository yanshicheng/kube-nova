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

type DevopsTektonResourceGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取项目 Tekton 资源
func NewDevopsTektonResourceGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsTektonResourceGetLogic {
	return &DevopsTektonResourceGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsTektonResourceGetLogic) DevopsTektonResourceGet(req *types.GetDevopsTektonResourceRequest) (resp *types.TektonResource, err error) {
	result, err := l.svcCtx.ProjectRpc.TektonResourceGet(l.ctx, &projectservice.GetTektonResourceReq{
		ProjectId:     req.ProjectId,
		BindingId:     req.BindingId,
		ResourceType:  req.ResourceType,
		Name:          req.Name,
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	data := tektonResourceToType(result.Data)
	return &data, nil
}
