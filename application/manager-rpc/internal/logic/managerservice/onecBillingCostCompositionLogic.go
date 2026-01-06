package managerservicelogic

import (
	"context"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type OnecBillingCostCompositionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewOnecBillingCostCompositionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingCostCompositionLogic {
	return &OnecBillingCostCompositionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// OnecBillingCostComposition 获取费用构成数据
func (l *OnecBillingCostCompositionLogic) OnecBillingCostComposition(in *pb.OnecBillingCostCompositionReq) (*pb.OnecBillingCostCompositionResp, error) {
	l.Logger.Infof("开始获取费用构成数据，startTime: %d, endTime: %d, month: %s, 集群UUID: %s, 项目ID: %d",
		in.StartTime, in.EndTime, in.Month, in.ClusterUuid, in.ProjectId)

	var composition []*model.CostCompositionItem
	var err error

	// 优先使用时间区间查询
	if in.StartTime > 0 && in.EndTime > 0 {
		composition, err = l.svcCtx.OnecBillingStatementModel.GetCostCompositionByTimeRange(
			l.ctx, in.StartTime, in.EndTime, in.ClusterUuid, in.ProjectId)
	} else {
		// 按月份查询（向后兼容）
		month := in.Month
		if month == "" {
			month = time.Now().Format("2006-01")
		}
		composition, err = l.svcCtx.OnecBillingStatementModel.GetCostComposition(
			l.ctx, month, in.ClusterUuid, in.ProjectId)
	}

	if err != nil {
		l.Logger.Errorf("获取费用构成数据失败，错误: %v", err)
		return nil, errorx.Msg("获取费用构成数据失败")
	}

	// 计算总费用
	var totalCost float64
	for _, item := range composition {
		totalCost += item.CostAmount
	}

	// 转换响应数据并计算百分比
	var items []*pb.CostCompositionItem
	for _, item := range composition {
		var percentage float64
		if totalCost > 0 {
			percentage = item.CostAmount / totalCost * 100
		}
		items = append(items, &pb.CostCompositionItem{
			CostName:   item.CostName,
			CostAmount: item.CostAmount,
			Percentage: percentage,
		})
	}

	l.Logger.Infof("获取费用构成数据成功，总费用: %.2f", totalCost)
	return &pb.OnecBillingCostCompositionResp{
		Items:     items,
		TotalCost: totalCost,
	}, nil
}
