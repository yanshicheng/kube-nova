package alertdashboard

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetAlertSummaryReportLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// NewGetAlertSummaryReportLogic 获取告警汇总报告
func NewGetAlertSummaryReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAlertSummaryReportLogic {
	return &GetAlertSummaryReportLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// GetAlertSummaryReport 获取告警汇总报告
// 聚合所有仪表盘数据，用于导出或生成报告
func (l *GetAlertSummaryReportLogic) GetAlertSummaryReport(req *types.GetAlertSummaryReportRequest) (resp *types.GetAlertSummaryReportResponse, err error) {
	// 构建过滤条件
	filter := &pb.AlertDashboardFilter{
		ClusterUuid: req.ClusterUuid,
		ProjectId:   req.ProjectId,
		WorkspaceId: req.WorkspaceId,
	}

	// 调用 RPC 服务获取告警汇总报告
	rpcResp, err := l.svcCtx.ManagerRpc.AlertDashboardSummaryReport(l.ctx, &pb.GetAlertSummaryReportReq{
		Filter:    filter,
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
	})
	if err != nil {
		l.Errorf("获取告警汇总报告失败: %v", err)
		return nil, fmt.Errorf("获取告警汇总报告失败")
	}

	// 转换总览统计
	overview := types.GetAlertOverviewResponse{
		TotalCount:         rpcResp.Overview.TotalCount,
		FiringCount:        rpcResp.Overview.FiringCount,
		ResolvedCount:      rpcResp.Overview.ResolvedCount,
		TodayNewCount:      rpcResp.Overview.TodayNewCount,
		TodayResolvedCount: rpcResp.Overview.TodayResolvedCount,
		AvgDuration:        rpcResp.Overview.AvgDuration,
		ResolvedRate:       rpcResp.Overview.ResolvedRate,
		CompareYesterday:   rpcResp.Overview.CompareYesterday,
	}

	// 转换级别统计
	var severityItems []types.SeverityStatItem
	for _, item := range rpcResp.SeverityStats.Items {
		severityItems = append(severityItems, types.SeverityStatItem{
			Severity:      item.Severity,
			SeverityCn:    item.SeverityCn,
			TotalCount:    item.TotalCount,
			FiringCount:   item.FiringCount,
			ResolvedCount: item.ResolvedCount,
			Percentage:    item.Percentage,
		})
	}
	severityStats := types.GetAlertSeverityStatsResponse{
		Items:      severityItems,
		TotalCount: rpcResp.SeverityStats.TotalCount,
	}

	// 转换趋势数据
	var trendDataPoints []types.TrendDataPoint
	for _, point := range rpcResp.Trend.DataPoints {
		trendDataPoints = append(trendDataPoints, types.TrendDataPoint{
			Date:          point.Date,
			NewCount:      point.NewCount,
			ResolvedCount: point.ResolvedCount,
			FiringCount:   point.FiringCount,
		})
	}
	trend := types.GetAlertTrendResponse{
		DataPoints:    trendDataPoints,
		TotalNew:      rpcResp.Trend.TotalNew,
		TotalResolved: rpcResp.Trend.TotalResolved,
	}

	// 转换排行榜数据的辅助函数
	convertRankingItems := func(items []*pb.RankingItem) []types.RankingItem {
		var result []types.RankingItem
		for _, item := range items {
			result = append(result, types.RankingItem{
				Rank:          item.Rank,
				ItemId:        item.ItemId,
				ItemName:      item.ItemName,
				AlertCount:    item.AlertCount,
				FiringCount:   item.FiringCount,
				CriticalCount: item.CriticalCount,
				WarningCount:  item.WarningCount,
				Percentage:    item.Percentage,
				ExtraInfo:     item.ExtraInfo,
			})
		}
		return result
	}

	resp = &types.GetAlertSummaryReportResponse{
		Overview:      overview,
		SeverityStats: severityStats,
		Trend:         trend,
		TopClusters:   convertRankingItems(rpcResp.TopClusters),
		TopProjects:   convertRankingItems(rpcResp.TopProjects),
		TopRules:      convertRankingItems(rpcResp.TopRules),
	}

	return resp, nil
}
