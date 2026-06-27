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

type DevopsTektonResourceLogLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取项目 Tekton Pod 或 PipelineRun 日志
func NewDevopsTektonResourceLogLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsTektonResourceLogLogic {
	return &DevopsTektonResourceLogLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsTektonResourceLogLogic) DevopsTektonResourceLog(req *types.LogDevopsTektonResourceRequest) (resp *types.LogDevopsTektonResourceResponse, err error) {
	result, err := l.svcCtx.ProjectRpc.TektonResourceLog(l.ctx, &projectservice.LogTektonResourceReq{
		ProjectId:     req.ProjectId,
		BindingId:     req.BindingId,
		ResourceType:  req.ResourceType,
		Name:          req.Name,
		Container:     req.Container,
		TailLines:     req.TailLines,
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	return &types.LogDevopsTektonResourceResponse{
		Content:    result.Content,
		Containers: result.Containers,
	}, nil
}
