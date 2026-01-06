package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type OnecBillingStatementSearchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewOnecBillingStatementSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingStatementSearchLogic {
	return &OnecBillingStatementSearchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// OnecBillingStatementSearch 搜索账单
func (l *OnecBillingStatementSearchLogic) OnecBillingStatementSearch(in *pb.OnecBillingStatementSearchReq) (*pb.OnecBillingStatementSearchResp, error) {
	l.Logger.Infof("开始搜索账单，开始时间: %d, 结束时间: %d, 集群UUID: %s, 项目ID: %d, 账单类型: %s",
		in.StartTime, in.EndTime, in.ClusterUuid, in.ProjectId, in.StatementType)

	// 参数校验
	page := in.Page
	if page == 0 {
		page = 1
	}
	pageSize := in.PageSize
	if pageSize == 0 {
		pageSize = 10
	}

	// 执行搜索
	summary, list, total, err := l.svcCtx.OnecBillingStatementModel.SearchWithSummary(
		l.ctx,
		in.StartTime,
		in.EndTime,
		in.ClusterUuid,
		in.ProjectId,
		in.StatementType,
		page,
		pageSize,
		in.OrderField,
		in.IsAsc,
	)
	if err != nil {
		l.Logger.Errorf("搜索账单失败，错误: %v", err)
		return nil, errorx.Msg("搜索账单失败")
	}

	// 转换响应数据
	var statements []*pb.OnecBillingStatement
	for _, item := range list {
		statement := &pb.OnecBillingStatement{
			Id:                 item.Id,
			StatementNo:        item.StatementNo,
			StatementType:      item.StatementType,
			BillingHours:       item.BillingHours,
			ClusterUuid:        item.ClusterUuid,
			ClusterName:        item.ClusterName,
			ProjectId:          item.ProjectId,
			ProjectName:        item.ProjectName,
			ProjectUuid:        item.ProjectUuid,
			ProjectClusterId:   item.ProjectClusterId,
			BindingId:          item.BindingId,
			CpuCapacity:        item.CpuCapacity,
			MemCapacity:        item.MemCapacity,
			StorageLimit:       item.StorageLimit,
			GpuCapacity:        item.GpuCapacity,
			PodsLimit:          item.PodsLimit,
			WorkspaceCount:     item.WorkspaceCount,
			ApplicationCount:   item.ApplicationCount,
			PriceCpu:           item.PriceCpu,
			PriceMemory:        item.PriceMemory,
			PriceStorage:       item.PriceStorage,
			PriceGpu:           item.PriceGpu,
			PricePod:           item.PricePod,
			PriceManagementFee: item.PriceManagementFee,
			CpuCost:            item.CpuCost,
			MemoryCost:         item.MemoryCost,
			StorageCost:        item.StorageCost,
			GpuCost:            item.GpuCost,
			PodCost:            item.PodCost,
			ManagementFee:      item.ManagementFee,
			ResourceCostTotal:  item.ResourceCostTotal,
			TotalAmount:        item.TotalAmount,
			Remark:             item.Remark,
			CreatedBy:          item.CreatedBy,
			UpdatedBy:          item.UpdatedBy,
			CreatedAt:          item.CreatedAt.Unix(),
			UpdatedAt:          item.UpdatedAt.Unix(),
		}
		if item.BillingStartTime.Valid {
			statement.BillingStartTime = item.BillingStartTime.Time.Unix()
		}
		if item.BillingEndTime.Valid {
			statement.BillingEndTime = item.BillingEndTime.Time.Unix()
		}
		statements = append(statements, statement)
	}

	l.Logger.Infof("搜索账单成功，总数: %d, 返回数量: %d", total, len(statements))
	return &pb.OnecBillingStatementSearchResp{
		Summary: &pb.BillingStatementSummary{
			TotalCount:    summary.TotalCount,
			TotalAmount:   summary.TotalAmount,
			CpuCost:       summary.CpuCost,
			MemoryCost:    summary.MemoryCost,
			StorageCost:   summary.StorageCost,
			GpuCost:       summary.GpuCost,
			PodCost:       summary.PodCost,
			ManagementFee: summary.ManagementFee,
		},
		List:  statements,
		Total: total,
	}, nil
}
