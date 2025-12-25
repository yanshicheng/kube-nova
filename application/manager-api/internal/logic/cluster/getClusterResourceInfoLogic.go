package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetClusterResourceInfoLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetClusterResourceInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetClusterResourceInfoLogic {
	return &GetClusterResourceInfoLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetClusterResourceInfoLogic) GetClusterResourceInfo(req *types.DefaultIdRequest) (resp *types.ClusterResourceInfo, err error) {

	// 先获取集群详情，获取UUID
	clusterResp, err := l.svcCtx.ManagerRpc.ClusterDetail(l.ctx, &pb.ClusterDetailReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("RPC调用获取集群详情失败: %v", err)
		return nil, fmt.Errorf("获取集群详情失败: %w", err)
	}

	l.Infof("获取到集群UUID: %s", clusterResp.Uuid)

	// 调用RPC获取集群资源信息
	rpcResp, err := l.svcCtx.ManagerRpc.ClusterResourceInfo(l.ctx, &pb.ClusterResourceInfoReq{
		ClusterUuid: clusterResp.Uuid,
	})
	if err != nil {
		l.Errorf("RPC调用获取集群资源信息失败: %v", err)
		return nil, fmt.Errorf("获取集群资源信息失败: %w", err)
	}

	// 转换响应 - 根据新的protobuf字段映射
	resp = &types.ClusterResourceInfo{
		Id:                      rpcResp.Id,
		ClusterUuid:             rpcResp.ClusterUuid,
		CpuPhysicalCapacity:     rpcResp.CpuPhysicalCapacity,
		CpuAllocatedTotal:       rpcResp.CpuAllocatedTotal,
		CpuCapacityTotal:        rpcResp.CpuCapacityTotal,
		MemPhysicalCapacity:     rpcResp.MemPhysicalCapacity,
		MemAllocatedTotal:       rpcResp.MemAllocatedTotal,
		MemCapacityTotal:        rpcResp.MemCapacityTotal,
		StoragePhysicalCapacity: rpcResp.StoragePhysicalCapacity,
		StorageAllocatedTotal:   rpcResp.StorageAllocatedTotal,
		GpuPhysicalCapacity:     rpcResp.GpuPhysicalCapacity,
		GpuAllocatedTotal:       rpcResp.GpuAllocatedTotal,
		GpuCapacityTotal:        rpcResp.GpuCapacityTotal,
		GpuUsedTotal:            rpcResp.GpuUsedTotal,
		PodsPhysicalCapacity:    rpcResp.PodsPhysicalCapacity,
		PodsAllocatedTotal:      rpcResp.PodsAllocatedTotal,
	}

	return resp, nil
}
