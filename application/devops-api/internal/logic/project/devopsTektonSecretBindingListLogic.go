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

type DevopsTektonSecretBindingListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询项目 Tekton Secret 可用渠道
func NewDevopsTektonSecretBindingListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsTektonSecretBindingListLogic {
	return &DevopsTektonSecretBindingListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsTektonSecretBindingListLogic) DevopsTektonSecretBindingList(req *types.ListDevopsTektonSecretBindingRequest) (resp *types.ListDevopsTektonSecretBindingResponse, err error) {
	result, err := l.svcCtx.ProjectRpc.TektonSecretBindingList(l.ctx, &projectservice.ListTektonSecretBindingReq{
		ProjectId:     req.ProjectId,
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.TektonSecretBindingOption, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, tektonSecretBindingToType(item))
	}

	return &types.ListDevopsTektonSecretBindingResponse{Items: items, Total: result.Total}, nil
}
