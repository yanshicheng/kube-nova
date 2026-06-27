// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package pipeline

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/client/pipelineconfigservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type DevopsPipelineEnvironmentListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询流水线环境
func NewDevopsPipelineEnvironmentListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsPipelineEnvironmentListLogic {
	return &DevopsPipelineEnvironmentListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsPipelineEnvironmentListLogic) DevopsPipelineEnvironmentList(req *types.ListDevopsPipelineEnvironmentRequest) (resp *types.ListDevopsPipelineEnvironmentResponse, err error) {
	result, err := l.svcCtx.PipelineRpc.PipelineEnvironmentList(l.ctx, &pipelineconfigservice.ListPipelineEnvironmentReq{
		Page:          req.Page,
		PageSize:      req.PageSize,
		Name:          req.Name,
		Code:          req.Code,
		Status:        req.Status,
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.DevopsPipelineEnvironment, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, environmentToType(item))
	}

	return &types.ListDevopsPipelineEnvironmentResponse{Items: items, Total: result.Total}, nil
}
