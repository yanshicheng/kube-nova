package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertDashboardTopRankingLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertDashboardTopRankingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertDashboardTopRankingLogic {
	return &AlertDashboardTopRankingLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AlertDashboardTopRanking 获取告警排行榜
func (l *AlertDashboardTopRankingLogic) AlertDashboardTopRanking(in *pb.GetAlertTopRankingReq) (*pb.GetAlertTopRankingResp, error) {
	// 解析过滤条件
	var clusterUuid string
	var projectId, workspaceId uint64

	if in.Filter != nil {
		clusterUuid = in.Filter.ClusterUuid
		projectId = in.Filter.ProjectId
		workspaceId = in.Filter.WorkspaceId
	}

	// 校验排行类型
	rankingType := in.RankingType
	if rankingType == "" {
		rankingType = "cluster"
	}
	validTypes := map[string]bool{
		"cluster":   true,
		"project":   true,
		"workspace": true,
		"rule":      true,
		"instance":  true,
	}
	if !validTypes[rankingType] {
		return nil, errorx.Msg("不支持的排行类型，请使用 cluster、project、workspace、rule 或 instance")
	}

	// 设置默认排行数量
	topN := int(in.TopN)
	if topN <= 0 {
		topN = 10
	}
	// 限制最大数量
	if topN > 100 {
		topN = 100
	}

	stats, totalAlerts, err := l.svcCtx.AlertInstancesModel.GetTopRanking(
		l.ctx,
		rankingType,
		clusterUuid,
		projectId,
		workspaceId,
		topN,
		in.Severity,
		in.Status,
	)
	if err != nil {
		l.Errorf("获取告警排行榜失败: %v", err)
		return nil, errorx.Msg("获取告警排行榜失败")
	}

	// 转换为 pb 格式
	var items []*pb.RankingItem
	for i, stat := range stats {
		// 计算占比
		var percentage float64
		if totalAlerts > 0 {
			percentage = float64(stat.AlertCount) / float64(totalAlerts) * 100
		}

		items = append(items, &pb.RankingItem{
			Rank:          int32(i + 1),
			ItemId:        stat.ItemId,
			ItemName:      stat.ItemName,
			AlertCount:    stat.AlertCount,
			FiringCount:   stat.FiringCount,
			CriticalCount: stat.CriticalCount,
			WarningCount:  stat.WarningCount,
			Percentage:    percentage,
			ExtraInfo:     stat.ExtraInfo,
		})
	}

	return &pb.GetAlertTopRankingResp{
		Items:       items,
		TotalAlerts: totalAlerts,
	}, nil
}
