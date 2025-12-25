package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	corev1 "k8s.io/api/core/v1"

	"github.com/zeromicro/go-zero/core/logx"
)

type PVGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 PV 详情
func NewPVGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PVGetLogic {
	return &PVGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PVGetLogic) PVGet(req *types.ClusterResourceNameRequest) (resp *types.PVDetail, err error) {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	pvOp := client.PersistentVolumes()
	pv, err := pvOp.Get(req.Name)
	if err != nil {
		l.Errorf("获取 PV 详情失败: %v", err)
		return nil, fmt.Errorf("获取 PV 详情失败")
	}

	capacity := ""
	if qty, ok := pv.Spec.Capacity[corev1.ResourceStorage]; ok {
		capacity = qty.String()
	}

	claim := ""
	if pv.Spec.ClaimRef != nil {
		claim = fmt.Sprintf("%s/%s", pv.Spec.ClaimRef.Namespace, pv.Spec.ClaimRef.Name)
	}

	reclaimPolicy := "Delete"
	if pv.Spec.PersistentVolumeReclaimPolicy != "" {
		reclaimPolicy = string(pv.Spec.PersistentVolumeReclaimPolicy)
	}

	volumeMode := "Filesystem"
	if pv.Spec.VolumeMode != nil {
		volumeMode = string(*pv.Spec.VolumeMode)
	}

	volumeAttributesClass := ""
	if pv.Spec.VolumeAttributesClassName != nil {
		volumeAttributesClass = *pv.Spec.VolumeAttributesClassName
	}

	accessModes := make([]string, 0, len(pv.Spec.AccessModes))
	for _, mode := range pv.Spec.AccessModes {
		accessModes = append(accessModes, string(mode))
	}

	resp = &types.PVDetail{
		Name:                  pv.Name,
		Capacity:              capacity,
		AccessModes:           accessModes,
		ReclaimPolicy:         reclaimPolicy,
		Status:                string(pv.Status.Phase),
		Claim:                 claim,
		StorageClass:          pv.Spec.StorageClassName,
		VolumeAttributesClass: volumeAttributesClass,
		VolumeMode:            volumeMode,
		Reason:                pv.Status.Reason,
		Message:               pv.Status.Message,
		Finalizers:            pv.Finalizers,
		Labels:                pv.Labels,
		Annotations:           pv.Annotations,
		Age:                   formatAge(pv.CreationTimestamp.Time),
		CreationTimestamp:     pv.CreationTimestamp.UnixMilli(),
	}

	// 处理 Source
	resp.Source = convertPVSource(pv)

	// 处理 NodeAffinity
	if pv.Spec.NodeAffinity != nil && pv.Spec.NodeAffinity.Required != nil {
		resp.NodeAffinity = fmt.Sprintf("%v", pv.Spec.NodeAffinity.Required.NodeSelectorTerms)
	}

	return resp, nil
}

// convertPVSource 转换 PV 存储源信息
func convertPVSource(pv *corev1.PersistentVolume) types.PVSourceInfo {
	source := pv.Spec.PersistentVolumeSource
	result := types.PVSourceInfo{
		Details: make(map[string]string),
	}

	switch {
	case source.NFS != nil:
		result.Type = "NFS"
		result.Details["server"] = source.NFS.Server
		result.Details["path"] = source.NFS.Path
		result.Details["readOnly"] = fmt.Sprintf("%v", source.NFS.ReadOnly)
	case source.HostPath != nil:
		result.Type = "HostPath"
		result.Details["path"] = source.HostPath.Path
		if source.HostPath.Type != nil {
			result.Details["type"] = string(*source.HostPath.Type)
		}
	case source.CSI != nil:
		result.Type = "CSI"
		result.Details["driver"] = source.CSI.Driver
		result.Details["volumeHandle"] = source.CSI.VolumeHandle
		result.Details["readOnly"] = fmt.Sprintf("%v", source.CSI.ReadOnly)
	case source.Local != nil:
		result.Type = "Local"
		result.Details["path"] = source.Local.Path
	case source.ISCSI != nil:
		result.Type = "ISCSI"
		result.Details["targetPortal"] = source.ISCSI.TargetPortal
		result.Details["iqn"] = source.ISCSI.IQN
		result.Details["lun"] = fmt.Sprintf("%d", source.ISCSI.Lun)
	case source.FC != nil:
		result.Type = "FC"
		result.Details["targetWWNs"] = fmt.Sprintf("%v", source.FC.TargetWWNs)
	case source.RBD != nil:
		result.Type = "RBD"
		result.Details["monitors"] = fmt.Sprintf("%v", source.RBD.CephMonitors)
		result.Details["image"] = source.RBD.RBDImage
		result.Details["pool"] = source.RBD.RBDPool
	default:
		result.Type = "Unknown"
	}

	return result
}
