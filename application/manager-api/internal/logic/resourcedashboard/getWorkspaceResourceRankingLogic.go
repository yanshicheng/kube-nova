package resourcedashboard

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetWorkspaceResourceRankingLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// NewGetWorkspaceResourceRankingLogic 获取工作空间资源排行
func NewGetWorkspaceResourceRankingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetWorkspaceResourceRankingLogic {
	return &GetWorkspaceResourceRankingLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetWorkspaceResourceRankingLogic) GetWorkspaceResourceRanking(req *types.GetWorkspaceResourceRankingReq) (resp *types.GetWorkspaceResourceRankingResp, err error) {
	// 调用 RPC 服务获取工作空间排行数据
	rpcResp, err := l.svcCtx.ManagerRpc.GetWorkspaceResourceRanking(l.ctx, &pb.WorkspaceResourceRankingReq{
		ClusterUuid: req.ClusterUuid,
		ProjectId:   req.ProjectId,
		SortBy:      req.SortBy,
		TopN:        req.TopN,
	})
	if err != nil {
		l.Errorf("获取工作空间资源排行失败: %v", err)
		return nil, fmt.Errorf("获取工作空间资源排行失败")
	}

	// 转换响应
	resp = &types.GetWorkspaceResourceRankingResp{
		Items: make([]types.WorkspaceRankingItem, 0, len(rpcResp.Items)),
		Total: rpcResp.Total,
	}

	for _, item := range rpcResp.Items {
		resp.Items = append(resp.Items, types.WorkspaceRankingItem{
			Rank:                item.Rank,
			WorkspaceId:         item.WorkspaceId,
			WorkspaceName:       item.WorkspaceName,
			WorkspaceUuid:       item.WorkspaceUuid,
			Namespace:           item.Namespace,
			ProjectId:           item.ProjectId,
			ProjectName:         item.ProjectName,
			ClusterUuid:         item.ClusterUuid,
			ClusterName:         item.ClusterName,
			CpuLimit:            item.CpuLimit,
			CpuAllocated:        item.CpuAllocated,
			CpuUsageRate:        item.CpuUsageRate,
			MemLimitGib:         item.MemLimitGib,
			MemAllocatedGib:     item.MemAllocatedGib,
			MemUsageRate:        item.MemUsageRate,
			GpuLimit:            item.GpuLimit,
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
