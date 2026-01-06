package billing

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type OnecBillingClusterTopLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOnecBillingClusterTopLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingClusterTopLogic {
	return &OnecBillingClusterTopLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OnecBillingClusterTopLogic) OnecBillingClusterTop(req *types.OnecBillingClusterTopRequest) (resp *types.OnecBillingClusterTopResponse, err error) {
	// 调用RPC服务获取集群费用排行
	result, err := l.svcCtx.ManagerRpc.OnecBillingClusterTop(l.ctx, &pb.OnecBillingClusterTopReq{
		Month:     req.Month,
		TopN:      req.TopN,
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
	})
	if err != nil {
		l.Errorf("获取集群费用排行失败: %v", err)
		return nil, fmt.Errorf("获取集群费用排行失败: %v", err)
	}

	// 转换响应数据
	var items []types.ClusterCostItem
	for _, item := range result.Items {
		items = append(items, types.ClusterCostItem{
			ClusterUuid:  item.ClusterUuid,
			ClusterName:  item.ClusterName,
			ProjectCount: item.ProjectCount,
			TotalCost:    item.TotalCost,
		})
	}

	return &types.OnecBillingClusterTopResponse{
		Items: items,
	}, nil
}
