package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type StorageClassListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 StorageClass 列表
func NewStorageClassListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StorageClassListLogic {
	return &StorageClassListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *StorageClassListLogic) StorageClassList(req *types.ClusterResourceListRequest) (resp *types.StorageClassListResponse, err error) {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	scOp := client.StorageClasses()
	result, err := scOp.List(req.Search, req.LabelSelector)
	if err != nil {
		l.Errorf("获取 StorageClass 列表失败: %v", err)
		return nil, fmt.Errorf("获取 StorageClass 列表失败")
	}

	resp = &types.StorageClassListResponse{
		Total: result.Total,
		Items: make([]types.StorageClassListItem, 0, len(result.Items)),
	}

	for _, item := range result.Items {
		resp.Items = append(resp.Items, types.StorageClassListItem{
			Name:                 item.Name,
			Provisioner:          item.Provisioner,
			ReclaimPolicy:        item.ReclaimPolicy,
			VolumeBindingMode:    item.VolumeBindingMode,
			AllowVolumeExpansion: item.AllowVolumeExpansion,
			IsDefault:            item.IsDefault,
			Age:                  item.Age,
			CreationTimestamp:    item.CreationTimestamp,
			Labels:               item.Labels,
			Annotations:          item.Annotations,
		})
	}

	return resp, nil
}
