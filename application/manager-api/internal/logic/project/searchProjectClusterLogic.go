package project

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type SearchProjectClusterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// NewSearchProjectClusterLogic 搜索项目的集群配额列表，projectId为必传参数
func NewSearchProjectClusterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchProjectClusterLogic {
	return &SearchProjectClusterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchProjectClusterLogic) SearchProjectCluster(req *types.SearchProjectClusterRequest) (resp []types.ProjectCluster, err error) {

	// 调用RPC服务搜索项目集群配额
	rpcResp, err := l.svcCtx.ManagerRpc.ProjectClusterSearch(l.ctx, &pb.SearchOnecProjectClusterReq{
		ProjectId:   req.ProjectId,
		ClusterUuid: req.ClusterUuid,
	})

	if err != nil {
		l.Errorf("搜索项目集群配额失败: %v", err)
		return nil, fmt.Errorf("搜索项目集群配额失败: %v", err)
	}

	// 转换响应数据
	clusters := make([]types.ProjectCluster, 0, len(rpcResp.Data))
	for _, c := range rpcResp.Data {
		clusters = append(clusters, types.ProjectCluster{
			Id:                        c.Id,
			ClusterUuid:               c.ClusterUuid,
			ClusterName:               c.ClusterName,
			ProjectId:                 c.ProjectId,
			CpuLimit:                  c.CpuLimit,
			CpuOvercommitRatio:        c.CpuOvercommitRatio,
			CpuCapacity:               c.CpuCapacity,
			CpuAllocated:              c.CpuAllocated,
			MemLimit:                  c.MemLimit,
			MemOvercommitRatio:        c.MemOvercommitRatio,
			MemCapacity:               c.MemCapacity,
			MemAllocated:              c.MemAllocated,
			StorageLimit:              c.StorageLimit,
			StorageAllocated:          c.StorageAllocated,
			GpuLimit:                  c.GpuLimit,
			GpuOvercommitRatio:        c.GpuOvercommitRatio,
			GpuCapacity:               c.GpuCapacity,
			GpuAllocated:              c.GpuAllocated,
			PodsLimit:                 c.PodsLimit,
			PodsAllocated:             c.PodsAllocated,
			ConfigmapLimit:            c.ConfigmapLimit,
			ConfigmapAllocated:        c.ConfigmapAllocated,
			SecretLimit:               c.SecretLimit,
			SecretAllocated:           c.SecretAllocated,
			PvcLimit:                  c.PvcLimit,
			PvcAllocated:              c.PvcAllocated,
			EphemeralStorageLimit:     c.EphemeralStorageLimit,
			EphemeralStorageAllocated: c.EphemeralStorageAllocated,
			ServiceLimit:              c.ServiceLimit,
			ServiceAllocated:          c.ServiceAllocated,
			LoadbalancersLimit:        c.LoadbalancersLimit,
			LoadbalancersAllocated:    c.LoadbalancersAllocated,
			NodeportsLimit:            c.NodeportsLimit,
			NodeportsAllocated:        c.NodeportsAllocated,
			DeploymentsLimit:          c.DeploymentsLimit,
			DeploymentsAllocated:      c.DeploymentsAllocated,
			JobsLimit:                 c.JobsLimit,
			JobsAllocated:             c.JobsAllocated,
			CronjobsLimit:             c.CronjobsLimit,
			CronjobsAllocated:         c.CronjobsAllocated,
			DaemonsetsLimit:           c.DaemonsetsLimit,
			DaemonsetsAllocated:       c.DaemonsetsAllocated,
			StatefulsetsLimit:         c.StatefulsetsLimit,
			StatefulsetsAllocated:     c.StatefulsetsAllocated,
			IngressesLimit:            c.IngressesLimit,
			IngressesAllocated:        c.IngressesAllocated,
			CreatedBy:                 c.CreatedBy,
			UpdatedBy:                 c.UpdatedBy,
			CreatedAt:                 c.CreatedAt,
			UpdatedAt:                 c.UpdatedAt,
		})
	}

	return clusters, nil
}
