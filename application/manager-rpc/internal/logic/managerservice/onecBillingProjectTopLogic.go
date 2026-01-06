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

type OnecBillingProjectTopLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewOnecBillingProjectTopLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingProjectTopLogic {
	return &OnecBillingProjectTopLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// OnecBillingProjectTop 获取项目费用排行
func (l *OnecBillingProjectTopLogic) OnecBillingProjectTop(in *pb.OnecBillingProjectTopReq) (*pb.OnecBillingProjectTopResp, error) {
	l.Logger.Infof("开始获取项目费用排行，startTime: %d, endTime: %d, month: %s, 集群UUID: %s, TOP数量: %d",
		in.StartTime, in.EndTime, in.Month, in.ClusterUuid, in.TopN)

	// 参数处理
	topN := int(in.TopN)
	if topN <= 0 {
		topN = 10
	}

	var projects []*model.ProjectCostStats
	var err error

	// 优先使用时间区间查询
	if in.StartTime > 0 && in.EndTime > 0 {
		projects, err = l.svcCtx.OnecBillingStatementModel.GetProjectTopByTimeRange(
			l.ctx, in.StartTime, in.EndTime, in.ClusterUuid, topN)
	} else {
		// 按月份查询（向后兼容）
		month := in.Month
		if month == "" {
			month = time.Now().Format("2006-01")
		}
		projects, err = l.svcCtx.OnecBillingStatementModel.GetProjectTop(
			l.ctx, month, in.ClusterUuid, topN)
	}

	if err != nil {
		l.Logger.Errorf("获取项目费用排行失败，错误: %v", err)
		return nil, errorx.Msg("获取项目费用排行失败")
	}

	// 转换响应数据
	var items []*pb.ProjectCostItem
	for _, project := range projects {
		items = append(items, &pb.ProjectCostItem{
			ProjectId:   project.ProjectId,
			ProjectName: project.ProjectName,
			ProjectUuid: project.ProjectUuid,
			TotalCost:   project.TotalCost,
		})
	}

	l.Logger.Infof("获取项目费用排行成功，返回数量: %d", len(items))
	return &pb.OnecBillingProjectTopResp{
		Items: items,
	}, nil
}
