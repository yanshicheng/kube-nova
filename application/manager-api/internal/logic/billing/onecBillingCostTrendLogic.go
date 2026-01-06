package billing

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type OnecBillingCostTrendLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOnecBillingCostTrendLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingCostTrendLogic {
	return &OnecBillingCostTrendLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OnecBillingCostTrendLogic) OnecBillingCostTrend(req *types.OnecBillingCostTrendRequest) (resp *types.OnecBillingCostTrendResponse, err error) {
	// 调用RPC服务获取费用趋势
	result, err := l.svcCtx.ManagerRpc.OnecBillingCostTrend(l.ctx, &pb.OnecBillingCostTrendReq{
		Months:      req.Months,
		ClusterUuid: req.ClusterUuid,
		ProjectId:   req.ProjectId,
	})
	if err != nil {
		l.Errorf("获取费用趋势失败: %v", err)
		return nil, fmt.Errorf("获取费用趋势失败: %v", err)
	}

	// 转换响应数据
	var items []types.CostTrendItem
	for _, item := range result.Items {
		items = append(items, types.CostTrendItem{
			Month:          item.Month,
			TotalCost:      item.TotalCost,
			ResourceCost:   item.ResourceCost,
			ManagementCost: item.ManagementCost,
		})
	}

	return &types.OnecBillingCostTrendResponse{
		Items: items,
	}, nil
}
