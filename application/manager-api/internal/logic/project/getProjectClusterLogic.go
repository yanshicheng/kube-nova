package project

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetProjectClusterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetProjectClusterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProjectClusterLogic {
	return &GetProjectClusterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetProjectClusterLogic) GetProjectCluster(req *types.DefaultIdRequest) (resp *types.ProjectCluster, err error) {

	// 调用RPC服务获取项目集群配额详情
	rpcResp, err := l.svcCtx.ManagerRpc.ProjectClusterGetById(l.ctx, &pb.GetOnecProjectClusterByIdReq{
		Id: req.Id,
	})

	if err != nil {
		l.Errorf("查询项目集群配额详情失败: %v", err)
		return nil, fmt.Errorf("查询项目集群配额详情失败: %v", err)
	}

	if rpcResp.Data == nil {
		l.Errorf("项目集群配额不存在, ID: %d", req.Id)
		return nil, fmt.Errorf("项目集群配额不存在")
	}

	// 转换响应数据
	cluster := &types.ProjectCluster{
		Id:                        rpcResp.Data.Id,
		ClusterUuid:               rpcResp.Data.ClusterUuid,
		ClusterName:               rpcResp.Data.ClusterName,
		ProjectId:                 rpcResp.Data.ProjectId,
		CpuLimit:                  rpcResp.Data.CpuLimit,
		CpuOvercommitRatio:        rpcResp.Data.CpuOvercommitRatio,
		CpuCapacity:               rpcResp.Data.CpuCapacity,
		CpuAllocated:              rpcResp.Data.CpuAllocated,
		MemLimit:                  rpcResp.Data.MemLimit,
		MemOvercommitRatio:        rpcResp.Data.MemOvercommitRatio,
		MemCapacity:               rpcResp.Data.MemCapacity,
		MemAllocated:              rpcResp.Data.MemAllocated,
		StorageLimit:              rpcResp.Data.StorageLimit,
		StorageAllocated:          rpcResp.Data.StorageAllocated,
		GpuLimit:                  rpcResp.Data.GpuLimit,
		GpuOvercommitRatio:        rpcResp.Data.GpuOvercommitRatio,
		GpuCapacity:               rpcResp.Data.GpuCapacity,
		GpuAllocated:              rpcResp.Data.GpuAllocated,
		PodsLimit:                 rpcResp.Data.PodsLimit,
		PodsAllocated:             rpcResp.Data.PodsAllocated,
		ConfigmapLimit:            rpcResp.Data.ConfigmapLimit,
		ConfigmapAllocated:        rpcResp.Data.ConfigmapAllocated,
		SecretLimit:               rpcResp.Data.SecretLimit,
		SecretAllocated:           rpcResp.Data.SecretAllocated,
		PvcLimit:                  rpcResp.Data.PvcLimit,
		PvcAllocated:              rpcResp.Data.PvcAllocated,
		EphemeralStorageLimit:     rpcResp.Data.EphemeralStorageLimit,
		EphemeralStorageAllocated: rpcResp.Data.EphemeralStorageAllocated,
		ServiceLimit:              rpcResp.Data.ServiceLimit,
		ServiceAllocated:          rpcResp.Data.ServiceAllocated,
		LoadbalancersLimit:        rpcResp.Data.LoadbalancersLimit,
		LoadbalancersAllocated:    rpcResp.Data.LoadbalancersAllocated,
		NodeportsLimit:            rpcResp.Data.NodeportsLimit,
		NodeportsAllocated:        rpcResp.Data.NodeportsAllocated,
		DeploymentsLimit:          rpcResp.Data.DeploymentsLimit,
		DeploymentsAllocated:      rpcResp.Data.DeploymentsAllocated,
		JobsLimit:                 rpcResp.Data.JobsLimit,
		JobsAllocated:             rpcResp.Data.JobsAllocated,
		CronjobsLimit:             rpcResp.Data.CronjobsLimit,
		CronjobsAllocated:         rpcResp.Data.CronjobsAllocated,
		DaemonsetsLimit:           rpcResp.Data.DaemonsetsLimit,
		DaemonsetsAllocated:       rpcResp.Data.DaemonsetsAllocated,
		StatefulsetsLimit:         rpcResp.Data.StatefulsetsLimit,
		StatefulsetsAllocated:     rpcResp.Data.StatefulsetsAllocated,
		IngressesLimit:            rpcResp.Data.IngressesLimit,
		IngressesAllocated:        rpcResp.Data.IngressesAllocated,
		CreatedBy:                 rpcResp.Data.CreatedBy,
		UpdatedBy:                 rpcResp.Data.UpdatedBy,
		CreatedAt:                 rpcResp.Data.CreatedAt,
		UpdatedAt:                 rpcResp.Data.UpdatedAt,
	}

	return cluster, nil
}
