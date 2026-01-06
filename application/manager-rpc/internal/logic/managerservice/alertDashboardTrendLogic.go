package managerservicelogic

import (
	"context"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertDashboardTrendLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertDashboardTrendLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertDashboardTrendLogic {
	return &AlertDashboardTrendLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AlertDashboardTrend 获取告警趋势分析
func (l *AlertDashboardTrendLogic) AlertDashboardTrend(in *pb.GetAlertTrendReq) (*pb.GetAlertTrendResp, error) {
	// 解析过滤条件
	var clusterUuid string
	var projectId, workspaceId uint64

	if in.Filter != nil {
		clusterUuid = in.Filter.ClusterUuid
		projectId = in.Filter.ProjectId
		workspaceId = in.Filter.WorkspaceId
	}

	// 设置默认天数
	days := int(in.Days)
	if days <= 0 {
		days = 7
	}
	// 限制最大天数为30天
	if days > 30 {
		days = 30
	}

	// 调用 Model 层获取统计数据
	stats, err := l.svcCtx.AlertInstancesModel.GetTrendStats(l.ctx, clusterUuid, projectId, workspaceId, days)
	if err != nil {
		l.Errorf("获取告警趋势统计失败: %v", err)
		return nil, errorx.Msg("获取告警趋势统计失败")
	}

	// 构建完整的日期序列，填充没有数据的日期
	dateMap := make(map[string]*pb.TrendDataPoint)
	for _, stat := range stats {
		dateMap[stat.Date] = &pb.TrendDataPoint{
			Date:          stat.Date,
			NewCount:      stat.NewCount,
			ResolvedCount: stat.ResolvedCount,
			FiringCount:   stat.FiringCount,
		}
	}

	// 生成完整的日期序列
	var dataPoints []*pb.TrendDataPoint
	var totalNew, totalResolved int64

	startDate := time.Now().AddDate(0, 0, -days+1)
	for i := 0; i < days; i++ {
		date := startDate.AddDate(0, 0, i).Format("2006-01-02")
		if point, ok := dateMap[date]; ok {
			dataPoints = append(dataPoints, point)
			totalNew += point.NewCount
			totalResolved += point.ResolvedCount
		} else {
			dataPoints = append(dataPoints, &pb.TrendDataPoint{
				Date:          date,
				NewCount:      0,
				ResolvedCount: 0,
				FiringCount:   0,
			})
		}
	}

	return &pb.GetAlertTrendResp{
		DataPoints:    dataPoints,
		TotalNew:      totalNew,
		TotalResolved: totalResolved,
	}, nil
}
