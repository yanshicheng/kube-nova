package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	corev1 "k8s.io/api/core/v1"

	"github.com/zeromicro/go-zero/core/logx"
)

type PVCGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 PVC 详情
func NewPVCGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PVCGetLogic {
	return &PVCGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PVCGetLogic) PVCGet(req *types.ClusterNamespaceResourceRequest) (resp *types.PVCDetail, err error) {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	pvcClient := client.PVC()

	pvc, err := pvcClient.Get(req.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取 PVC 失败: %v", err)
		return nil, fmt.Errorf("获取 PVC 失败")
	}

	// 获取容量
	capacity := ""
	if pvc.Status.Capacity != nil {
		if storage, ok := pvc.Status.Capacity[corev1.ResourceStorage]; ok {
			capacity = storage.String()
		}
	}

	// 获取存储类
	storageClass := ""
	if pvc.Spec.StorageClassName != nil {
		storageClass = *pvc.Spec.StorageClassName
	}

	// 获取绑定的 PV
	volume := ""
	if pvc.Spec.VolumeName != "" {
		volume = pvc.Spec.VolumeName
	}

	// 获取请求的存储大小
	requestStorage := ""
	if pvc.Spec.Resources.Requests != nil {
		if storage, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
			requestStorage = storage.String()
		}
	}

	// 转换 AccessModes
	accessModes := make([]string, len(pvc.Spec.AccessModes))
	for i, mode := range pvc.Spec.AccessModes {
		accessModes[i] = string(mode)
	}

	// 获取 VolumeMode
	volumeMode := ""
	if pvc.Spec.VolumeMode != nil {
		volumeMode = string(*pvc.Spec.VolumeMode)
	}

	detail := &types.PVCDetail{
		Name:              pvc.Name,
		Namespace:         pvc.Namespace,
		Status:            string(pvc.Status.Phase),
		Volume:            volume,
		Capacity:          capacity,
		AccessModes:       accessModes,
		StorageClass:      storageClass,
		VolumeMode:        volumeMode,
		RequestStorage:    requestStorage,
		Labels:            pvc.Labels,
		Annotations:       pvc.Annotations,
		Age:               calculateAge(pvc.CreationTimestamp.Time),
		CreationTimestamp: pvc.CreationTimestamp.UnixMilli(),
	}

	l.Infof("成功获取 PVC 详情: %s/%s", req.Namespace, req.Name)
	return detail, nil
}
