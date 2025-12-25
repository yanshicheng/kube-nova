package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type SearchClusterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSearchClusterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchClusterLogic {
	return &SearchClusterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchClusterLogic) SearchCluster(req *types.SearchClusterRequest) (resp *types.SearchClusterResponse, err error) {

	// 调用RPC搜索集群
	rpcResp, err := l.svcCtx.ManagerRpc.ClusterSearch(l.ctx, &pb.SearchClusterReq{
		Page:        req.Page,
		PageSize:    req.PageSize,
		OrderField:  req.OrderStr,
		IsAsc:       req.IsAsc,
		Name:        req.Name,
		Environment: req.Environment,
		Uuid:        req.Uuid,
	})
	if err != nil {
		l.Errorf("RPC调用搜索集群失败: %v", err)
		return nil, fmt.Errorf("搜索集群失败: %w", err)
	}

	// 转换响应
	items := make([]types.Cluster, 0, len(rpcResp.Data))
	for _, item := range rpcResp.Data {

		items = append(items, types.Cluster{
			Id:           item.Id,
			Name:         item.Name,
			Avatar:       item.Avatar,
			Environment:  item.Environment,
			ClusterType:  item.ClusterType,
			Version:      item.Version,
			Status:       item.Status,
			HealthStatus: item.HealthStatus,
			Uuid:         item.Uuid,
			StorageUsage: item.StorageUsage,
			MemoryUsage:  item.MemoryUsage,
			CpuUsage:     item.CpuUsage,
			PodUsage:     item.PodUsage,
			CreatedAt:    item.CreatedAt,
		})
	}

	resp = &types.SearchClusterResponse{
		Items: items,
		Total: rpcResp.Total,
	}

	return resp, nil
}
