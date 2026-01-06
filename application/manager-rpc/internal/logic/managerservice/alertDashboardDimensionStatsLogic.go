package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertDashboardDimensionStatsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertDashboardDimensionStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertDashboardDimensionStatsLogic {
	return &AlertDashboardDimensionStatsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AlertDashboardDimensionStats 获取告警维度统计
func (l *AlertDashboardDimensionStatsLogic) AlertDashboardDimensionStats(in *pb.GetAlertDimensionStatsReq) (*pb.GetAlertDimensionStatsResp, error) {
	// 解析过滤条件
	var clusterUuid string
	var projectId, workspaceId uint64

	if in.Filter != nil {
		clusterUuid = in.Filter.ClusterUuid
		projectId = in.Filter.ProjectId
		workspaceId = in.Filter.WorkspaceId
	}

	// 校验维度参数
	dimension := in.Dimension
	if dimension == "" {
		dimension = "cluster"
	}
	if dimension != "cluster" && dimension != "project" && dimension != "workspace" {
		return nil, errorx.Msg("不支持的维度类型，请使用 cluster、project 或 workspace")
	}

	// 设置默认分页参数
	page := in.Page
	if page < 1 {
		page = 1
	}
	pageSize := in.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	// 调用 Model 层获取统计数据
	stats, total, err := l.svcCtx.AlertInstancesModel.GetDimensionStats(l.ctx, dimension, clusterUuid, projectId, workspaceId, page, pageSize)
	if err != nil {
		l.Errorf("获取告警维度统计失败: %v", err)
		return nil, errorx.Msg("获取告警维度统计失败")
	}

	// 转换为 pb 格式
	var items []*pb.DimensionStatItem
	for _, stat := range stats {
		items = append(items, &pb.DimensionStatItem{
			DimensionId:   stat.DimensionId,
			DimensionName: stat.DimensionName,
			TotalCount:    stat.TotalCount,
			FiringCount:   stat.FiringCount,
			ResolvedCount: stat.ResolvedCount,
			CriticalCount: stat.CriticalCount,
			WarningCount:  stat.WarningCount,
			InfoCount:     stat.InfoCount,
		})
	}

	return &pb.GetAlertDimensionStatsResp{
		Items: items,
		Total: uint64(total),
	}, nil
}
