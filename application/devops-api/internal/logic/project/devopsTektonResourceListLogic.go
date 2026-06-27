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

type DevopsTektonResourceListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询项目 Tekton 资源
func NewDevopsTektonResourceListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsTektonResourceListLogic {
	return &DevopsTektonResourceListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsTektonResourceListLogic) DevopsTektonResourceList(req *types.ListDevopsTektonResourceRequest) (resp *types.ListDevopsTektonResourceResponse, err error) {
	result, err := l.svcCtx.ProjectRpc.TektonResourceList(l.ctx, &projectservice.ListTektonResourceReq{
		ProjectId:     req.ProjectId,
		BindingId:     req.BindingId,
		ResourceType:  req.ResourceType,
		Keyword:       req.Keyword,
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.TektonResource, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, tektonResourceToType(item))
	}
	return &types.ListDevopsTektonResourceResponse{Items: items, Total: result.Total}, nil
}
