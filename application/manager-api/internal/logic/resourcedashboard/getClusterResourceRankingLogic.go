package resourcedashboard

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetClusterResourceRankingLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// NewGetClusterResourceRankingLogic 获取集群资源排行
func NewGetClusterResourceRankingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetClusterResourceRankingLogic {
	return &GetClusterResourceRankingLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetClusterResourceRankingLogic) GetClusterResourceRanking(req *types.GetClusterResourceRankingReq) (resp *types.GetClusterResourceRankingResp, err error) {
	// 调用 RPC 服务获取集群排行数据
	rpcResp, err := l.svcCtx.ManagerRpc.GetClusterResourceRanking(l.ctx, &pb.ClusterResourceRankingReq{
		ProjectId: req.ProjectId,
		SortBy:    req.SortBy,
		TopN:      req.TopN,
	})
	if err != nil {
		l.Errorf("获取集群资源排行失败: %v", err)
		return nil, fmt.Errorf("获取集群资源排行失败")
	}

	// 转换响应
	resp = &types.GetClusterResourceRankingResp{
		Items: make([]types.ClusterRankingItem, 0, len(rpcResp.Items)),
		Total: rpcResp.Total,
	}

	for _, item := range rpcResp.Items {
		resp.Items = append(resp.Items, types.ClusterRankingItem{
			Rank:                  item.Rank,
			ClusterUuid:           item.ClusterUuid,
			ClusterName:           item.ClusterName,
			Environment:           item.Environment,
			Region:                item.Region,
			Provider:              item.Provider,
			ProjectCount:          item.ProjectCount,
			WorkspaceCount:        item.WorkspaceCount,
			CpuCapacity:           item.CpuCapacity,
			CpuLimit:              item.CpuLimit,
			CpuAllocated:          item.CpuAllocated,
			CpuAllocationRate:     item.CpuAllocationRate,
			CpuOversellRate:       item.CpuOversellRate,
			MemCapacityGib:        item.MemCapacityGib,
			MemLimitGib:           item.MemLimitGib,
			MemAllocatedGib:       item.MemAllocatedGib,
			MemAllocationRate:     item.MemAllocationRate,
			MemOversellRate:       item.MemOversellRate,
			GpuCapacity:           item.GpuCapacity,
			GpuLimit:              item.GpuLimit,
			GpuAllocated:          item.GpuAllocated,
			GpuAllocationRate:     item.GpuAllocationRate,
			GpuOversellRate:       item.GpuOversellRate,
			StorageLimitGib:       item.StorageLimitGib,
			StorageAllocatedGib:   item.StorageAllocatedGib,
			StorageAllocationRate: item.StorageAllocationRate,
			PodsLimit:             item.PodsLimit,
			PodsAllocated:         item.PodsAllocated,
			PodsAllocationRate:    item.PodsAllocationRate,
			OverallUsageRate:      item.OverallUsageRate,
		})
	}

	return resp, nil
}
