package resourcedashboard

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetResourceDashboardSummaryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetResourceDashboardSummaryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetResourceDashboardSummaryLogic {
	return &GetResourceDashboardSummaryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetResourceDashboardSummaryLogic) GetResourceDashboardSummary(req *types.GetResourceDashboardSummaryReq) (resp *types.GetResourceDashboardSummaryResp, err error) {
	// 调用 RPC
	rpcResp, err := l.svcCtx.ManagerRpc.GetResourceDashboardSummary(l.ctx, &pb.ResourceDashboardSummaryReq{
		ClusterUuid: req.ClusterUuid,
		ProjectId:   req.ProjectId,
	})
	if err != nil {
		return nil, err
	}

	// 转换响应
	resp = &types.GetResourceDashboardSummaryResp{
		CurrentFilter: types.FilterCondition{
			ClusterUuid:            rpcResp.CurrentFilter.ClusterUuid,
			ClusterName:            rpcResp.CurrentFilter.ClusterName,
			ProjectId:              rpcResp.CurrentFilter.ProjectId,
			ProjectName:            rpcResp.CurrentFilter.ProjectName,
			FilteredClusterCount:   rpcResp.CurrentFilter.FilteredClusterCount,
			FilteredProjectCount:   rpcResp.CurrentFilter.FilteredProjectCount,
			FilteredWorkspaceCount: rpcResp.CurrentFilter.FilteredWorkspaceCount,
		},
		SummaryCards: types.DashboardSummaryCards{
			ClusterTotalCount:   rpcResp.SummaryCards.ClusterTotalCount,
			ProjectTotalCount:   rpcResp.SummaryCards.ProjectTotalCount,
			WorkspaceTotalCount: rpcResp.SummaryCards.WorkspaceTotalCount,
			CpuPhysical:         rpcResp.SummaryCards.CpuPhysical,
			CpuLimit:            rpcResp.SummaryCards.CpuLimit,
			CpuPhysicalRate:     rpcResp.SummaryCards.CpuPhysicalRate,
			MemPhysical:         rpcResp.SummaryCards.MemPhysical,
			MemLimit:            rpcResp.SummaryCards.MemLimit,
			MemPhysicalRate:     rpcResp.SummaryCards.MemPhysicalRate,
			GpuPhysical:         rpcResp.SummaryCards.GpuPhysical,
			GpuLimit:            rpcResp.SummaryCards.GpuLimit,
			GpuPhysicalRate:     rpcResp.SummaryCards.GpuPhysicalRate,
			StoragePhysical:     rpcResp.SummaryCards.StoragePhysical,
			StorageLimit:        rpcResp.SummaryCards.StorageLimit,
			StoragePhysicalRate: rpcResp.SummaryCards.StoragePhysicalRate,
			PodPhysical:         rpcResp.SummaryCards.PodPhysical,
			PodLimit:            rpcResp.SummaryCards.PodLimit,
			PodPhysicalRate:     rpcResp.SummaryCards.PodPhysicalRate,
		},
		AllocationOverview: types.ResourceAllocationOverview{
			Cpu:     convertOverviewItem(rpcResp.AllocationOverview.Cpu),
			Mem:     convertOverviewItem(rpcResp.AllocationOverview.Mem),
			Gpu:     convertOverviewItem(rpcResp.AllocationOverview.Gpu),
			Storage: convertOverviewItem(rpcResp.AllocationOverview.Storage),
			Pod:     convertOverviewItem(rpcResp.AllocationOverview.Pod),
		},
		OversellSavings: types.ResourceOversellSavings{
			Cpu: convertOversellItem(rpcResp.OversellSavings.Cpu),
			Mem: convertOversellItem(rpcResp.OversellSavings.Mem),
			Gpu: convertOversellItem(rpcResp.OversellSavings.Gpu),
		},
	}

	return resp, nil
}

// 转换资源概览项
func convertOverviewItem(item *pb.ResourceOverviewItem) types.ResourceOverviewItem {
	if item == nil {
		return types.ResourceOverviewItem{}
	}
	return types.ResourceOverviewItem{
		Physical: item.Physical,
		Limit:    item.Limit,
		Capacity: item.Capacity,
		Unit:     item.Unit,
	}
}

// 转换超分收益项
func convertOversellItem(item *pb.OversellSavingItem) types.OversellSavingItem {
	if item == nil {
		return types.OversellSavingItem{}
	}
	return types.OversellSavingItem{
		LimitTotal:    item.LimitTotal,
		CapacityTotal: item.CapacityTotal,
		SavingAmount:  item.SavingAmount,
		SavingRate:    item.SavingRate,
		Unit:          item.Unit,
	}
}
