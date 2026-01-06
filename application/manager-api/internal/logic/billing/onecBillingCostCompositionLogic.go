package billing

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type OnecBillingCostCompositionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOnecBillingCostCompositionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingCostCompositionLogic {
	return &OnecBillingCostCompositionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OnecBillingCostCompositionLogic) OnecBillingCostComposition(req *types.OnecBillingCostCompositionRequest) (resp *types.OnecBillingCostCompositionResponse, err error) {
	// 调用RPC服务获取费用构成
	result, err := l.svcCtx.ManagerRpc.OnecBillingCostComposition(l.ctx, &pb.OnecBillingCostCompositionReq{
		Month:       req.Month,
		ClusterUuid: req.ClusterUuid,
		ProjectId:   req.ProjectId,
		StartTime:   req.StartTime,
		EndTime:     req.EndTime,
	})
	if err != nil {
		l.Errorf("获取费用构成失败: %v", err)
		return nil, fmt.Errorf("获取费用构成失败: %v", err)
	}

	// 转换响应数据
	var items []types.CostCompositionItem
	for _, item := range result.Items {
		items = append(items, types.CostCompositionItem{
			CostName:   item.CostName,
			CostAmount: item.CostAmount,
			Percentage: item.Percentage,
		})
	}

	return &types.OnecBillingCostCompositionResponse{
		Items:     items,
		TotalCost: result.TotalCost,
	}, nil
}
