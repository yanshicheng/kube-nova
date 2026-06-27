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

type DevopsTektonSecretListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询项目 Tekton Secret
func NewDevopsTektonSecretListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsTektonSecretListLogic {
	return &DevopsTektonSecretListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsTektonSecretListLogic) DevopsTektonSecretList(req *types.ListDevopsTektonSecretRequest) (resp *types.ListDevopsTektonSecretResponse, err error) {
	result, err := l.svcCtx.ProjectRpc.TektonSecretList(l.ctx, &projectservice.ListTektonSecretReq{
		ProjectId:     req.ProjectId,
		BindingId:     req.BindingId,
		Keyword:       req.Keyword,
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.TektonSecret, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, tektonSecretToType(item))
	}

	return &types.ListDevopsTektonSecretResponse{Items: items, Total: result.Total}, nil
}
