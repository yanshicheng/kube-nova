package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PVListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 PV 列表
func NewPVListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PVListLogic {
	return &PVListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PVListLogic) PVList(req *types.PVListRequest) (resp *types.PVListResponse, err error) {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	pvOp := client.PersistentVolumes()
	result, err := pvOp.List(req.Search, req.LabelSelector, req.Status, req.StorageClass)
	if err != nil {
		l.Errorf("获取 PV 列表失败: %v", err)
		return nil, fmt.Errorf("获取 PV 列表失败")
	}

	resp = &types.PVListResponse{
		Total: result.Total,
		Items: make([]types.PVListItem, 0, len(result.Items)),
	}

	for _, item := range result.Items {
		resp.Items = append(resp.Items, types.PVListItem{
			Name:                  item.Name,
			Capacity:              item.Capacity,
			AccessModes:           item.AccessModes,
			ReclaimPolicy:         item.ReclaimPolicy,
			Status:                item.Status,
			Claim:                 item.Claim,
			StorageClass:          item.StorageClass,
			VolumeAttributesClass: item.VolumeAttributesClass,
			Reason:                item.Reason,
			VolumeMode:            item.VolumeMode,
			Age:                   item.Age,
			CreationTimestamp:     item.CreationTimestamp,
			Labels:                item.Labels,
		})
	}

	return resp, nil
}
