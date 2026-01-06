package managerservicelogic

import (
	"context"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type OnecBillingDashboardStatsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewOnecBillingDashboardStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingDashboardStatsLogic {
	return &OnecBillingDashboardStatsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// OnecBillingDashboardStats 获取仪表盘统计数据
func (l *OnecBillingDashboardStatsLogic) OnecBillingDashboardStats(in *pb.OnecBillingDashboardStatsReq) (*pb.OnecBillingDashboardStatsResp, error) {
	l.Logger.Infof("开始获取仪表盘统计数据，startTime: %d, endTime: %d, month: %s, 集群UUID: %s, 项目ID: %d",
		in.StartTime, in.EndTime, in.Month, in.ClusterUuid, in.ProjectId)

	var currentStats, lastStats *model.MonthlyStats
	var err error

	// 优先使用时间区间查询
	if in.StartTime > 0 && in.EndTime > 0 {
		// 按时间区间查询
		currentStats, err = l.svcCtx.OnecBillingStatementModel.GetStatsByTimeRange(
			l.ctx, in.StartTime, in.EndTime, in.ClusterUuid, in.ProjectId)
		if err != nil {
			l.Logger.Errorf("获取当期统计数据失败，错误: %v", err)
			return nil, errorx.Msg("获取统计数据失败")
		}

		// 计算上一个周期（相同时长）
		duration := in.EndTime - in.StartTime
		lastStartTime := in.StartTime - duration
		lastEndTime := in.StartTime

		lastStats, err = l.svcCtx.OnecBillingStatementModel.GetStatsByTimeRange(
			l.ctx, lastStartTime, lastEndTime, in.ClusterUuid, in.ProjectId)
		if err != nil {
			l.Logger.Errorf("获取上期统计数据失败，错误: %v", err)
			lastStats = &model.MonthlyStats{}
		}
	} else {
		// 按月份查询（向后兼容）
		month := in.Month
		if month == "" {
			month = time.Now().Format("2006-01")
		}

		currentStats, err = l.svcCtx.OnecBillingStatementModel.GetMonthlyStats(
			l.ctx, month, in.ClusterUuid, in.ProjectId)
		if err != nil {
			l.Logger.Errorf("获取当月统计数据失败，月份: %s, 错误: %v", month, err)
			return nil, errorx.Msg("获取统计数据失败")
		}

		// 计算上月月份
		currentTime, _ := time.Parse("2006-01", month)
		lastMonth := currentTime.AddDate(0, -1, 0).Format("2006-01")

		lastStats, err = l.svcCtx.OnecBillingStatementModel.GetMonthlyStats(
			l.ctx, lastMonth, in.ClusterUuid, in.ProjectId)
		if err != nil {
			l.Logger.Errorf("获取上月统计数据失败，月份: %s, 错误: %v", lastMonth, err)
			lastStats = &model.MonthlyStats{}
		}
	}

	// 计算环比
	totalCostMom := calculateMom(currentStats.TotalCost, lastStats.TotalCost)
	resourceCostMom := calculateMom(currentStats.ResourceCost, lastStats.ResourceCost)
	managementCostMom := calculateMom(currentStats.ManagementCost, lastStats.ManagementCost)

	l.Logger.Infof("获取仪表盘统计数据成功，总费用: %.2f, 账单数: %d", currentStats.TotalCost, currentStats.StatementCount)
	return &pb.OnecBillingDashboardStatsResp{
		TotalCost:         currentStats.TotalCost,
		TotalCostMom:      totalCostMom,
		ResourceCost:      currentStats.ResourceCost,
		ResourceCostMom:   resourceCostMom,
		ManagementCost:    currentStats.ManagementCost,
		ManagementCostMom: managementCostMom,
		StatementCount:    currentStats.StatementCount,
		ProjectCount:      currentStats.ProjectCount,
	}, nil
}

// calculateMom 计算环比变化率
func calculateMom(current, last float64) float64 {
	if last == 0 {
		if current > 0 {
			return 100.0
		}
		return 0
	}
	return (current - last) / last * 100
}
