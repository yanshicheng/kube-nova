package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type OnecBillingCostTrendLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewOnecBillingCostTrendLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingCostTrendLogic {
	return &OnecBillingCostTrendLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// OnecBillingCostTrend 获取费用趋势数据
func (l *OnecBillingCostTrendLogic) OnecBillingCostTrend(in *pb.OnecBillingCostTrendReq) (*pb.OnecBillingCostTrendResp, error) {
	l.Logger.Infof("开始获取费用趋势数据，月数: %d, 集群UUID: %s, 项目ID: %d",
		in.Months, in.ClusterUuid, in.ProjectId)

	// 参数校验，如果未指定月数则默认6个月
	months := int(in.Months)
	if months <= 0 {
		months = 6
	}

	// 获取趋势数据
	trend, err := l.svcCtx.OnecBillingStatementModel.GetMonthlyTrend(l.ctx, months, in.ClusterUuid, in.ProjectId)
	if err != nil {
		l.Logger.Errorf("获取费用趋势数据失败，错误: %v", err)
		return nil, errorx.Msg("获取费用趋势数据失败")
	}

	// 转换响应数据
	var items []*pb.CostTrendItem
	for _, item := range trend {
		items = append(items, &pb.CostTrendItem{
			Month:          item.Month,
			TotalCost:      item.TotalCost,
			ResourceCost:   item.ResourceCost,
			ManagementCost: item.ManagementCost,
		})
	}

	l.Logger.Infof("获取费用趋势数据成功，返回月数: %d", len(items))
	return &pb.OnecBillingCostTrendResp{
		Items: items,
	}, nil
}
