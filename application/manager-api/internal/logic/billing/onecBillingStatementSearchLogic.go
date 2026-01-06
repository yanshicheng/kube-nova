package billing

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type OnecBillingStatementSearchLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOnecBillingStatementSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingStatementSearchLogic {
	return &OnecBillingStatementSearchLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OnecBillingStatementSearchLogic) OnecBillingStatementSearch(req *types.OnecBillingStatementSearchRequest) (resp *types.OnecBillingStatementSearchResponse, err error) {
	// 调用RPC服务搜索账单
	result, err := l.svcCtx.ManagerRpc.OnecBillingStatementSearch(l.ctx, &pb.OnecBillingStatementSearchReq{
		StartTime:     req.StartTime,
		EndTime:       req.EndTime,
		ClusterUuid:   req.ClusterUuid,
		ProjectId:     req.ProjectId,
		StatementType: req.StatementType,
		Page:          req.Page,
		PageSize:      req.PageSize,
		OrderField:    req.OrderField,
		IsAsc:         req.IsAsc,
	})
	if err != nil {
		l.Errorf("搜索账单失败: %v", err)
		return nil, fmt.Errorf("搜索账单失败: %v", err)
	}

	// 转换账单列表
	var list []types.OnecBillingStatement
	for _, item := range result.List {
		list = append(list, types.OnecBillingStatement{
			Id:                 item.Id,
			StatementNo:        item.StatementNo,
			StatementType:      item.StatementType,
			BillingStartTime:   item.BillingStartTime,
			BillingEndTime:     item.BillingEndTime,
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
			CreatedAt:          item.CreatedAt,
			UpdatedAt:          item.UpdatedAt,
		})
	}

	return &types.OnecBillingStatementSearchResponse{
		Summary: types.BillingStatementSummary{
			TotalCount:    result.Summary.TotalCount,
			TotalAmount:   result.Summary.TotalAmount,
			CpuCost:       result.Summary.CpuCost,
			MemoryCost:    result.Summary.MemoryCost,
			StorageCost:   result.Summary.StorageCost,
			GpuCost:       result.Summary.GpuCost,
			PodCost:       result.Summary.PodCost,
			ManagementFee: result.Summary.ManagementFee,
		},
		List:  list,
		Total: result.Total,
	}, nil
}
