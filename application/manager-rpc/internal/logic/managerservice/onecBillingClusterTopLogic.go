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

type OnecBillingClusterTopLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewOnecBillingClusterTopLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingClusterTopLogic {
	return &OnecBillingClusterTopLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// OnecBillingClusterTop 获取集群费用排行
func (l *OnecBillingClusterTopLogic) OnecBillingClusterTop(in *pb.OnecBillingClusterTopReq) (*pb.OnecBillingClusterTopResp, error) {
	l.Logger.Infof("开始获取集群费用排行，startTime: %d, endTime: %d, month: %s, TOP数量: %d",
		in.StartTime, in.EndTime, in.Month, in.TopN)

	// 参数处理
	topN := int(in.TopN)
	if topN <= 0 {
		topN = 10
	}

	var clusters []*model.ClusterCostStats
	var err error

	// 优先使用时间区间查询
	if in.StartTime > 0 && in.EndTime > 0 {
		clusters, err = l.svcCtx.OnecBillingStatementModel.GetClusterTopByTimeRange(
			l.ctx, in.StartTime, in.EndTime, topN)
	} else {
		// 按月份查询（向后兼容）
		month := in.Month
		if month == "" {
			month = time.Now().Format("2006-01")
		}
		clusters, err = l.svcCtx.OnecBillingStatementModel.GetClusterTop(l.ctx, month, topN)
	}

	if err != nil {
		l.Logger.Errorf("获取集群费用排行失败，错误: %v", err)
		return nil, errorx.Msg("获取集群费用排行失败")
	}

	// 转换响应数据
	var items []*pb.ClusterCostItem
	for _, cluster := range clusters {
		items = append(items, &pb.ClusterCostItem{
			ClusterUuid:  cluster.ClusterUuid,
			ClusterName:  cluster.ClusterName,
			ProjectCount: cluster.ProjectCount,
			TotalCost:    cluster.TotalCost,
		})
	}

	l.Logger.Infof("获取集群费用排行成功，返回数量: %d", len(items))
	return &pb.OnecBillingClusterTopResp{
		Items: items,
	}, nil
}
