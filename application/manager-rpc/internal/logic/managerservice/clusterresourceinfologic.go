package managerservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ClusterResourceInfoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewClusterResourceInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterResourceInfoLogic {
	return &ClusterResourceInfoLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 集群资源
func (l *ClusterResourceInfoLogic) ClusterResourceInfo(in *pb.ClusterResourceInfoReq) (*pb.ClusterResourceInfoResp, error) {

	// 1. 查询资源信息
	resource, err := l.svcCtx.OnecClusterResourceModel.FindOneByClusterUuid(l.ctx, in.ClusterUuid)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("集群资源信息不存在 [clusterUuid=%s]", in.ClusterUuid)
			return nil, errorx.Msg("集群资源信息不存在")
		}
		l.Errorf("查询集群资源信息失败: %v", err)
		return nil, errorx.Msg("查询集群资源信息失败")
	}

	// 2. 构建响应
	resp := &pb.ClusterResourceInfoResp{
		Id:                      resource.Id,
		ClusterUuid:             resource.ClusterUuid,
		CpuPhysicalCapacity:     resource.CpuPhysicalCapacity,
		CpuAllocatedTotal:       resource.CpuAllocatedTotal,
		CpuCapacityTotal:        resource.CpuCapacityTotal,
		MemPhysicalCapacity:     resource.MemPhysicalCapacity,
		MemAllocatedTotal:       resource.MemAllocatedTotal,
		MemCapacityTotal:        resource.MemCapacityTotal,
		StoragePhysicalCapacity: resource.StoragePhysicalCapacity,
		StorageAllocatedTotal:   resource.StorageAllocatedTotal,
		GpuAllocatedTotal:       resource.GpuAllocatedTotal,
		GpuCapacityTotal:        resource.GpuCapacityTotal,
		GpuPhysicalCapacity:     resource.GpuPhysicalCapacity,
		PodsPhysicalCapacity:    resource.PodsPhysicalCapacity,
		PodsAllocatedTotal:      resource.PodsAllocatedTotal,
	}

	return resp, nil
}
