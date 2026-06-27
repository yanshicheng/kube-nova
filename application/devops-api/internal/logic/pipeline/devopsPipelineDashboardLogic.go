// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package pipeline

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/client/executionservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type DevopsPipelineDashboardLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 流水线驾驶舱
func NewDevopsPipelineDashboardLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsPipelineDashboardLogic {
	return &DevopsPipelineDashboardLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsPipelineDashboardLogic) DevopsPipelineDashboard(req *types.DevopsPipelineDashboardRequest) (resp *types.DevopsPipelineDashboardResponse, err error) {
	result, err := l.svcCtx.PipelineExecRpc.PipelineDashboard(l.ctx, &executionservice.PipelineDashboardReq{
		ProjectId:     req.ProjectId,
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}

	return &types.DevopsPipelineDashboardResponse{
		Overview:         pipelineDashboardOverviewToType(result.Overview),
		Trends:           pipelineDashboardTrendsToType(result.Trends),
		ProjectRanks:     pipelineDashboardMetricsToType(result.ProjectRanks),
		SystemRanks:      pipelineDashboardMetricsToType(result.SystemRanks),
		PipelineRanks:    pipelineDashboardMetricsToType(result.PipelineRanks),
		EnvironmentRanks: pipelineDashboardMetricsToType(result.EnvironmentRanks),
		ChannelRanks:     pipelineDashboardMetricsToType(result.ChannelRanks),
	}, nil
}
