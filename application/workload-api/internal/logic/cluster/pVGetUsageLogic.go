package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PVGetUsageLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 PV 使用情况
func NewPVGetUsageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PVGetUsageLogic {
	return &PVGetUsageLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PVGetUsageLogic) PVGetUsage(req *types.ClusterResourceNameRequest) (resp *types.PVUsageResponse, err error) {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	pvOp := client.PersistentVolumes()
	result, err := pvOp.GetUsage(req.Name)
	if err != nil {
		l.Errorf("获取 PV 使用情况失败: %v", err)
		return nil, fmt.Errorf("获取 PV 使用情况失败")
	}

	resp = &types.PVUsageResponse{
		PVName:       result.PVName,
		Status:       result.Status,
		UsedByPods:   make([]types.PodRefInfo, 0, len(result.UsedByPods)),
		CanDelete:    result.CanDelete,
		DeleteReason: result.DeleteReason,
	}

	// 转换 ClaimInfo
	if result.ClaimInfo != nil {
		resp.ClaimInfo = types.PVClaimInfo{
			Name:         result.ClaimInfo.Name,
			Namespace:    result.ClaimInfo.Namespace,
			Status:       result.ClaimInfo.Status,
			Capacity:     result.ClaimInfo.Capacity,
			AccessModes:  result.ClaimInfo.AccessModes,
			StorageClass: result.ClaimInfo.StorageClass,
			Labels:       result.ClaimInfo.Labels,
			Age:          result.ClaimInfo.Age,
		}
	}

	// 转换 UsedByPods
	for _, pod := range result.UsedByPods {
		resp.UsedByPods = append(resp.UsedByPods, types.PodRefInfo{
			Name:       pod.Name,
			Namespace:  pod.Namespace,
			NodeName:   pod.NodeName,
			Status:     pod.Status,
			VolumeName: pod.VolumeName,
		})
	}

	return resp, nil
}
