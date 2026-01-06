package resourcedashboard

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetProjectResourceRankingLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// NewGetProjectResourceRankingLogic 获取项目资源排行
func NewGetProjectResourceRankingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProjectResourceRankingLogic {
	return &GetProjectResourceRankingLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetProjectResourceRankingLogic) GetProjectResourceRanking(req *types.GetProjectResourceRankingReq) (resp *types.GetProjectResourceRankingResp, err error) {
	// 调用 RPC 服务获取项目排行数据
	rpcResp, err := l.svcCtx.ManagerRpc.GetProjectResourceRanking(l.ctx, &pb.ProjectResourceRankingReq{
		ClusterUuid: req.ClusterUuid,
		SortBy:      req.SortBy,
		TopN:        req.TopN,
	})
	if err != nil {
		l.Errorf("获取项目资源排行失败: %v", err)
		return nil, fmt.Errorf("获取项目资源排行失败")
	}

	// 转换响应
	resp = &types.GetProjectResourceRankingResp{
		Items: make([]types.ProjectRankingItem, 0, len(rpcResp.Items)),
		Total: rpcResp.Total,
	}

	for _, item := range rpcResp.Items {
		resp.Items = append(resp.Items, types.ProjectRankingItem{
			Rank:                item.Rank,
			ProjectId:           item.ProjectId,
			ProjectName:         item.ProjectName,
			ProjectUuid:         item.ProjectUuid,
			IsSystem:            item.IsSystem,
			ClusterCount:        item.ClusterCount,
			WorkspaceCount:      item.WorkspaceCount,
			PrimaryClusterUuid:  item.PrimaryClusterUuid,
			PrimaryClusterName:  item.PrimaryClusterName,
			CpuLimit:            item.CpuLimit,
			CpuCapacity:         item.CpuCapacity,
			CpuAllocated:        item.CpuAllocated,
			CpuUsageRate:        item.CpuUsageRate,
			MemLimitGib:         item.MemLimitGib,
			MemCapacityGib:      item.MemCapacityGib,
			MemAllocatedGib:     item.MemAllocatedGib,
			MemUsageRate:        item.MemUsageRate,
			GpuLimit:            item.GpuLimit,
			GpuCapacity:         item.GpuCapacity,
			GpuAllocated:        item.GpuAllocated,
			GpuUsageRate:        item.GpuUsageRate,
			StorageLimitGib:     item.StorageLimitGib,
			StorageAllocatedGib: item.StorageAllocatedGib,
			StorageUsageRate:    item.StorageUsageRate,
			PodsLimit:           item.PodsLimit,
			PodsAllocated:       item.PodsAllocated,
			PodsUsageRate:       item.PodsUsageRate,
			OverallUsageRate:    item.OverallUsageRate,
		})
	}

	return resp, nil
}
