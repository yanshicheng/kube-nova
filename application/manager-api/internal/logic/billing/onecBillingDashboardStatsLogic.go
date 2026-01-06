package billing

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type OnecBillingDashboardStatsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOnecBillingDashboardStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingDashboardStatsLogic {
	return &OnecBillingDashboardStatsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OnecBillingDashboardStatsLogic) OnecBillingDashboardStats(req *types.OnecBillingDashboardStatsRequest) (resp *types.OnecBillingDashboardStatsResponse, err error) {
	// 调用RPC服务获取仪表盘统计数据
	result, err := l.svcCtx.ManagerRpc.OnecBillingDashboardStats(l.ctx, &pb.OnecBillingDashboardStatsReq{
		Month:       req.Month,
		ClusterUuid: req.ClusterUuid,
		ProjectId:   req.ProjectId,
		StartTime:   req.StartTime,
		EndTime:     req.EndTime,
	})
	if err != nil {
		l.Errorf("获取仪表盘统计数据失败: %v", err)
		return nil, fmt.Errorf("获取仪表盘统计数据失败: %v", err)
	}

	// 转换响应数据
	return &types.OnecBillingDashboardStatsResponse{
		TotalCost:         result.TotalCost,
		TotalCostMom:      result.TotalCostMom,
		ResourceCost:      result.ResourceCost,
		ResourceCostMom:   result.ResourceCostMom,
		ManagementCost:    result.ManagementCost,
		ManagementCostMom: result.ManagementCostMom,
		StatementCount:    result.StatementCount,
		ProjectCount:      result.ProjectCount,
	}, nil
}
