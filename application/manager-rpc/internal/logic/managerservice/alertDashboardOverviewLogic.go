package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertDashboardOverviewLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertDashboardOverviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertDashboardOverviewLogic {
	return &AlertDashboardOverviewLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AlertDashboardOverview 获取告警总览统计
func (l *AlertDashboardOverviewLogic) AlertDashboardOverview(in *pb.GetAlertOverviewReq) (*pb.GetAlertOverviewResp, error) {
	// 解析过滤条件
	var clusterUuid string
	var projectId, workspaceId uint64

	if in.Filter != nil {
		clusterUuid = in.Filter.ClusterUuid
		projectId = in.Filter.ProjectId
		workspaceId = in.Filter.WorkspaceId
	}

	// 调用 Model 层获取统计数据
	stats, err := l.svcCtx.AlertInstancesModel.GetOverviewStats(l.ctx, clusterUuid, projectId, workspaceId)
	if err != nil {
		l.Errorf("获取告警总览统计失败: %v", err)
		return nil, errorx.Msg("获取告警总览统计失败")
	}

	// 计算恢复率
	var resolvedRate float64
	if stats.TotalCount > 0 {
		resolvedRate = float64(stats.ResolvedCount) / float64(stats.TotalCount) * 100
	}

	// 计算较昨日变化百分比
	var compareYesterday float64
	if stats.YesterdayCount > 0 {
		compareYesterday = float64(stats.TodayNewCount-stats.YesterdayCount) / float64(stats.YesterdayCount) * 100
	}

	return &pb.GetAlertOverviewResp{
		TotalCount:         stats.TotalCount,
		FiringCount:        stats.FiringCount,
		ResolvedCount:      stats.ResolvedCount,
		TodayNewCount:      stats.TodayNewCount,
		TodayResolvedCount: stats.TodayResolvedCount,
		AvgDuration:        stats.AvgDuration,
		ResolvedRate:       resolvedRate,
		CompareYesterday:   compareYesterday,
	}, nil
}
