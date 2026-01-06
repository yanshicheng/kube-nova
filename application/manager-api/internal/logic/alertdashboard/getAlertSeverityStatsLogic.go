package alertdashboard

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetAlertSeverityStatsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// NewGetAlertSeverityStatsLogic 获取告警级别统计
func NewGetAlertSeverityStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAlertSeverityStatsLogic {
	return &GetAlertSeverityStatsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// GetAlertSeverityStats 获取告警级别统计
// 返回各级别的告警数量、触发中数量、已恢复数量和占比
func (l *GetAlertSeverityStatsLogic) GetAlertSeverityStats(req *types.GetAlertSeverityStatsRequest) (resp *types.GetAlertSeverityStatsResponse, err error) {
	// 构建过滤条件
	filter := &pb.AlertDashboardFilter{
		ClusterUuid: req.ClusterUuid,
		ProjectId:   req.ProjectId,
		WorkspaceId: req.WorkspaceId,
	}

	// 调用 RPC 服务获取告警级别统计
	rpcResp, err := l.svcCtx.ManagerRpc.AlertDashboardSeverityStats(l.ctx, &pb.GetAlertSeverityStatsReq{
		Filter: filter,
	})
	if err != nil {
		l.Errorf("获取告警级别统计失败: %v", err)
		return nil, fmt.Errorf("获取告警级别统计失败")
	}

	// 转换响应
	var items []types.SeverityStatItem
	for _, item := range rpcResp.Items {
		items = append(items, types.SeverityStatItem{
			Severity:      item.Severity,
			SeverityCn:    item.SeverityCn,
			TotalCount:    item.TotalCount,
			FiringCount:   item.FiringCount,
			ResolvedCount: item.ResolvedCount,
			Percentage:    item.Percentage,
		})
	}

	resp = &types.GetAlertSeverityStatsResponse{
		Items:      items,
		TotalCount: rpcResp.TotalCount,
	}

	return resp, nil
}
