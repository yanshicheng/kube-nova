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

type DevopsTektonPVCListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询项目 Tekton PVC
func NewDevopsTektonPVCListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsTektonPVCListLogic {
	return &DevopsTektonPVCListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsTektonPVCListLogic) DevopsTektonPVCList(req *types.ListDevopsTektonPVCRequest) (resp *types.ListDevopsTektonPVCResponse, err error) {
	result, err := l.svcCtx.ProjectRpc.TektonPVCList(l.ctx, &projectservice.ListTektonPVCReq{
		ProjectId:     req.ProjectId,
		BindingId:     req.BindingId,
		Keyword:       req.Keyword,
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.TektonPVC, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, tektonPVCToType(item))
	}

	return &types.ListDevopsTektonPVCResponse{Items: items, Total: result.Total}, nil
}
