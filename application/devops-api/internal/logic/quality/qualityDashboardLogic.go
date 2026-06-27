// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package quality

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/devops-quality-rpc/client/qualityservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type QualityDashboardLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 质量看板
func NewQualityDashboardLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QualityDashboardLogic {
	return &QualityDashboardLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *QualityDashboardLogic) QualityDashboard(req *types.QualityDashboardRequest) (resp *types.QualityDashboardResponse, err error) {
	projectIDs, restricted, err := resolveProjectScope(l.ctx, l.svcCtx, req.ProjectId)
	if err != nil {
		return nil, err
	}
	result, err := l.svcCtx.QualityRpc.QualityDashboard(l.ctx, &qualityservice.QualityDashboardReq{
		ProjectId:         req.ProjectId,
		SystemId:          req.SystemId,
		TargetType:        req.TargetType,
		StartDate:         req.StartDate,
		EndDate:           req.EndDate,
		CurrentUserId:     currentUserID(l.ctx),
		CurrentRoles:      currentRoles(l.ctx),
		ProjectIds:        projectIDs,
		ProjectRestricted: restricted,
	})
	if err != nil {
		return nil, err
	}
	return &types.QualityDashboardResponse{
		Overview:     dashboardOverviewToType(result.Overview),
		ProjectRanks: dashboardMetricsToType(result.ProjectRanks),
		SystemRanks:  dashboardMetricsToType(result.SystemRanks),
		ToolRanks:    dashboardMetricsToType(result.ToolRanks),
	}, nil
}

func dashboardMetricsToType(items []*qualityservice.QualityDashboardMetric) []types.QualityDashboardMetric {
	metrics := make([]types.QualityDashboardMetric, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		metrics = append(metrics, types.QualityDashboardMetric{
			Id:            item.Id,
			Name:          item.Name,
			Code:          item.Code,
			ReportCount:   item.ReportCount,
			IssueCount:    item.IssueCount,
			CriticalCount: item.CriticalCount,
			HighCount:     item.HighCount,
			GatePassRate:  item.GatePassRate,
		})
	}
	return metrics
}
