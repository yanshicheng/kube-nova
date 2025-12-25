package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type StorageClassGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 StorageClass 详情
func NewStorageClassGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StorageClassGetLogic {
	return &StorageClassGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *StorageClassGetLogic) StorageClassGet(req *types.ClusterResourceNameRequest) (resp *types.StorageClassDetail, err error) {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	scOp := client.StorageClasses()
	sc, err := scOp.Get(req.Name)
	if err != nil {
		l.Errorf("获取 StorageClass 详情失败: %v", err)
		return nil, fmt.Errorf("获取 StorageClass 详情失败")
	}

	// 获取关联信息
	scInfo, _ := scOp.List("", "")
	var isDefault bool
	for _, item := range scInfo.Items {
		if item.Name == req.Name {
			isDefault = item.IsDefault
			break
		}
	}

	reclaimPolicy := "Delete"
	if sc.ReclaimPolicy != nil {
		reclaimPolicy = string(*sc.ReclaimPolicy)
	}

	volumeBindingMode := "Immediate"
	if sc.VolumeBindingMode != nil {
		volumeBindingMode = string(*sc.VolumeBindingMode)
	}

	allowExpansion := false
	if sc.AllowVolumeExpansion != nil {
		allowExpansion = *sc.AllowVolumeExpansion
	}

	resp = &types.StorageClassDetail{
		Name:                 sc.Name,
		Provisioner:          sc.Provisioner,
		ReclaimPolicy:        reclaimPolicy,
		VolumeBindingMode:    volumeBindingMode,
		AllowVolumeExpansion: allowExpansion,
		IsDefault:            isDefault,
		Parameters:           sc.Parameters,
		MountOptions:         sc.MountOptions,
		Labels:               sc.Labels,
		Annotations:          sc.Annotations,
		Age:                  formatAge(sc.CreationTimestamp.Time),
		CreationTimestamp:    sc.CreationTimestamp.UnixMilli(),
	}

	// 处理 AllowedTopologies
	if len(sc.AllowedTopologies) > 0 {
		resp.AllowedTopologies = make([]types.TopologyInfo, 0)
		for _, topology := range sc.AllowedTopologies {
			for _, expr := range topology.MatchLabelExpressions {
				resp.AllowedTopologies = append(resp.AllowedTopologies, types.TopologyInfo{
					Key:    expr.Key,
					Values: expr.Values,
				})
			}
		}
	}

	return resp, nil
}
