package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertDashboardSummaryReportLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertDashboardSummaryReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertDashboardSummaryReportLogic {
	return &AlertDashboardSummaryReportLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AlertDashboardSummaryReport 获取告警汇总报告
func (l *AlertDashboardSummaryReportLogic) AlertDashboardSummaryReport(in *pb.GetAlertSummaryReportReq) (*pb.GetAlertSummaryReportResp, error) {
	// 解析过滤条件
	var clusterUuid string
	var projectId, workspaceId uint64

	if in.Filter != nil {
		clusterUuid = in.Filter.ClusterUuid
		projectId = in.Filter.ProjectId
		workspaceId = in.Filter.WorkspaceId
	}

	// 获取总览统计
	overviewLogic := NewAlertDashboardOverviewLogic(l.ctx, l.svcCtx)
	overviewResp, err := overviewLogic.AlertDashboardOverview(&pb.GetAlertOverviewReq{
		Filter: in.Filter,
	})
	if err != nil {
		l.Errorf("获取告警总览统计失败: %v", err)
		return nil, errorx.Msg("获取告警汇总报告失败")
	}

	// 获取级别统计
	severityLogic := NewAlertDashboardSeverityStatsLogic(l.ctx, l.svcCtx)
	severityResp, err := severityLogic.AlertDashboardSeverityStats(&pb.GetAlertSeverityStatsReq{
		Filter: in.Filter,
	})
	if err != nil {
		l.Errorf("获取告警级别统计失败: %v", err)
		return nil, errorx.Msg("获取告警汇总报告失败")
	}

	// 获取趋势数据，默认7天
	trendLogic := NewAlertDashboardTrendLogic(l.ctx, l.svcCtx)
	trendResp, err := trendLogic.AlertDashboardTrend(&pb.GetAlertTrendReq{
		Filter: in.Filter,
		Days:   7,
	})
	if err != nil {
		l.Errorf("获取告警趋势统计失败: %v", err)
		return nil, errorx.Msg("获取告警汇总报告失败")
	}

	// 获取 Top 10 集群
	topClusters, _, err := l.svcCtx.AlertInstancesModel.GetTopRanking(
		l.ctx, "cluster", clusterUuid, projectId, workspaceId, 10, "", "",
	)
	if err != nil {
		l.Errorf("获取集群排行失败: %v", err)
		return nil, errorx.Msg("获取告警汇总报告失败")
	}

	// 获取 Top 10 项目
	topProjects, _, err := l.svcCtx.AlertInstancesModel.GetTopRanking(
		l.ctx, "project", clusterUuid, projectId, workspaceId, 10, "", "",
	)
	if err != nil {
		l.Errorf("获取项目排行失败: %v", err)
		return nil, errorx.Msg("获取告警汇总报告失败")
	}

	// 获取 Top 10 规则
	topRules, _, err := l.svcCtx.AlertInstancesModel.GetTopRanking(
		l.ctx, "rule", clusterUuid, projectId, workspaceId, 10, "", "",
	)
	if err != nil {
		l.Errorf("获取规则排行失败: %v", err)
		return nil, errorx.Msg("获取告警汇总报告失败")
	}

	// 转换排行数据为 pb 格式
	convertRankingItems := func(stats []*model.RankingStats) []*pb.RankingItem {
		var items []*pb.RankingItem
		for i, stat := range stats {
			items = append(items, &pb.RankingItem{
				Rank:          int32(i + 1),
				ItemId:        stat.ItemId,
				ItemName:      stat.ItemName,
				AlertCount:    stat.AlertCount,
				FiringCount:   stat.FiringCount,
				CriticalCount: stat.CriticalCount,
				WarningCount:  stat.WarningCount,
				ExtraInfo:     stat.ExtraInfo,
			})
		}
		return items
	}

	return &pb.GetAlertSummaryReportResp{
		Overview:      overviewResp,
		SeverityStats: severityResp,
		Trend:         trendResp,
		TopClusters:   convertRankingItems(topClusters),
		TopProjects:   convertRankingItems(topProjects),
		TopRules:      convertRankingItems(topRules),
	}, nil
}
