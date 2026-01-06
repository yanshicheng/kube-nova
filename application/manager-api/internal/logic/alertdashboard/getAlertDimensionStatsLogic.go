package alertdashboard

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetAlertDimensionStatsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// NewGetAlertDimensionStatsLogic 获取告警维度统计
func NewGetAlertDimensionStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAlertDimensionStatsLogic {
	return &GetAlertDimensionStatsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// GetAlertDimensionStats 获取告警维度统计
// 按集群、项目或工作空间维度聚合告警数据
func (l *GetAlertDimensionStatsLogic) GetAlertDimensionStats(req *types.GetAlertDimensionStatsRequest) (resp *types.GetAlertDimensionStatsResponse, err error) {
	// 构建过滤条件
	filter := &pb.AlertDashboardFilter{
		ClusterUuid: req.ClusterUuid,
		ProjectId:   req.ProjectId,
		WorkspaceId: req.WorkspaceId,
	}

	// 设置默认分页参数
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	// 调用 RPC 服务获取告警维度统计
	rpcResp, err := l.svcCtx.ManagerRpc.AlertDashboardDimensionStats(l.ctx, &pb.GetAlertDimensionStatsReq{
		Filter:    filter,
		Dimension: req.Dimension,
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		l.Errorf("获取告警维度统计失败: %v", err)
		return nil, fmt.Errorf("获取告警维度统计失败")
	}

	// 转换响应
	var items []types.DimensionStatItem
	for _, item := range rpcResp.Items {
		items = append(items, types.DimensionStatItem{
			DimensionId:   item.DimensionId,
			DimensionName: item.DimensionName,
			TotalCount:    item.TotalCount,
			FiringCount:   item.FiringCount,
			ResolvedCount: item.ResolvedCount,
			CriticalCount: item.CriticalCount,
			WarningCount:  item.WarningCount,
			InfoCount:     item.InfoCount,
		})
	}

	resp = &types.GetAlertDimensionStatsResponse{
		Items: items,
		Total: rpcResp.Total,
	}

	return resp, nil
}
