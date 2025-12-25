package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type StorageClassGetPVsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 StorageClass 关联的 PV
func NewStorageClassGetPVsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StorageClassGetPVsLogic {
	return &StorageClassGetPVsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *StorageClassGetPVsLogic) StorageClassGetPVs(req *types.ClusterResourceNameRequest) (resp *types.StorageClassPVResponse, err error) {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	scOp := client.StorageClasses()
	result, err := scOp.GetAssociatedPVs(req.Name)
	if err != nil {
		l.Errorf("获取 StorageClass 关联的 PV 失败: %v", err)
		return nil, fmt.Errorf("获取 StorageClass 关联的 PV 失败")
	}

	resp = &types.StorageClassPVResponse{
		StorageClassName: result.StorageClassName,
		PVCount:          result.PVCount,
		BoundPVCount:     result.BoundPVCount,
		AvailablePVCount: result.AvailablePVCount,
		TotalCapacity:    result.TotalCapacity,
		PVs:              make([]types.StorageClassPVItem, 0, len(result.PVs)),
	}

	for _, pv := range result.PVs {
		resp.PVs = append(resp.PVs, types.StorageClassPVItem{
			Name:        pv.Name,
			Capacity:    pv.Capacity,
			AccessModes: pv.AccessModes,
			Status:      pv.Status,
			Claim:       pv.Claim,
			Age:         pv.Age,
		})
	}

	return resp, nil
}
