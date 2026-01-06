package billing

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type OnecBillingProjectTopLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOnecBillingProjectTopLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingProjectTopLogic {
	return &OnecBillingProjectTopLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OnecBillingProjectTopLogic) OnecBillingProjectTop(req *types.OnecBillingProjectTopRequest) (resp *types.OnecBillingProjectTopResponse, err error) {
	// 调用RPC服务获取项目费用排行
	result, err := l.svcCtx.ManagerRpc.OnecBillingProjectTop(l.ctx, &pb.OnecBillingProjectTopReq{
		Month:       req.Month,
		ClusterUuid: req.ClusterUuid,
		TopN:        req.TopN,
		StartTime:   req.StartTime,
		EndTime:     req.EndTime,
	})
	if err != nil {
		l.Errorf("获取项目费用排行失败: %v", err)
		return nil, fmt.Errorf("获取项目费用排行失败: %v", err)
	}

	// 转换响应数据
	var items []types.ProjectCostItem
	for _, item := range result.Items {
		items = append(items, types.ProjectCostItem{
			ProjectId:   item.ProjectId,
			ProjectName: item.ProjectName,
			ProjectUuid: item.ProjectUuid,
			TotalCost:   item.TotalCost,
		})
	}

	return &types.OnecBillingProjectTopResponse{
		Items: items,
	}, nil
}
