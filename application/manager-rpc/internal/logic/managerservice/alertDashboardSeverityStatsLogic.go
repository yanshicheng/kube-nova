package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertDashboardSeverityStatsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertDashboardSeverityStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertDashboardSeverityStatsLogic {
	return &AlertDashboardSeverityStatsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// severityCnMap 级别中文名映射
var severityCnMap = map[string]string{
	"critical": "严重",
	"warning":  "警告",
	"info":     "信息",
	"none":     "无级别",
}

// AlertDashboardSeverityStats 获取告警级别统计
func (l *AlertDashboardSeverityStatsLogic) AlertDashboardSeverityStats(in *pb.GetAlertSeverityStatsReq) (*pb.GetAlertSeverityStatsResp, error) {
	// 解析过滤条件
	var clusterUuid string
	var projectId, workspaceId uint64

	if in.Filter != nil {
		clusterUuid = in.Filter.ClusterUuid
		projectId = in.Filter.ProjectId
		workspaceId = in.Filter.WorkspaceId
	}

	// 调用 Model 层获取统计数据
	stats, err := l.svcCtx.AlertInstancesModel.GetSeverityStats(l.ctx, clusterUuid, projectId, workspaceId)
	if err != nil {
		l.Errorf("获取告警级别统计失败: %v", err)
		return nil, errorx.Msg("获取告警级别统计失败")
	}

	// 计算总数用于百分比计算
	var totalCount int64
	for _, stat := range stats {
		totalCount += stat.TotalCount
	}

	// 转换为 pb 格式
	var items []*pb.SeverityStatItem
	for _, stat := range stats {
		// 计算百分比
		var percentage float64
		if totalCount > 0 {
			percentage = float64(stat.TotalCount) / float64(totalCount) * 100
		}

		// 获取中文名称
		severityCn, ok := severityCnMap[stat.Severity]
		if !ok {
			severityCn = stat.Severity
		}

		items = append(items, &pb.SeverityStatItem{
			Severity:      stat.Severity,
			SeverityCn:    severityCn,
			TotalCount:    stat.TotalCount,
			FiringCount:   stat.FiringCount,
			ResolvedCount: stat.ResolvedCount,
			Percentage:    percentage,
		})
	}

	// 确保所有级别都有数据，即使数量为0
	existingSeverities := make(map[string]bool)
	for _, item := range items {
		existingSeverities[item.Severity] = true
	}

	// 补充缺失的级别
	severityOrder := []string{"critical", "warning", "info", "none"}
	for _, severity := range severityOrder {
		if !existingSeverities[severity] {
			items = append(items, &pb.SeverityStatItem{
				Severity:      severity,
				SeverityCn:    severityCnMap[severity],
				TotalCount:    0,
				FiringCount:   0,
				ResolvedCount: 0,
				Percentage:    0,
			})
		}
	}

	return &pb.GetAlertSeverityStatsResp{
		Items:      items,
		TotalCount: totalCount,
	}, nil
}
