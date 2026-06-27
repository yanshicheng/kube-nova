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

type DevopsTektonResourceDescribeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取项目 Tekton 资源 Describe
func NewDevopsTektonResourceDescribeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsTektonResourceDescribeLogic {
	return &DevopsTektonResourceDescribeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsTektonResourceDescribeLogic) DevopsTektonResourceDescribe(req *types.GetDevopsTektonResourceRequest) (resp *types.DescribeDevopsTektonResourceResponse, err error) {
	result, err := l.svcCtx.ProjectRpc.TektonResourceDescribe(l.ctx, &projectservice.DescribeTektonResourceReq{
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
	return &types.DescribeDevopsTektonResourceResponse{Content: result.Content}, nil
}
