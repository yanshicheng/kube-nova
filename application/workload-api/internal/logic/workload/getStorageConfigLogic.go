package workload

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	types2 "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetStorageConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询存储配置
func NewGetStorageConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetStorageConfigLogic {
	return &GetStorageConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetStorageConfigLogic) GetStorageConfig(req *types.DefaultIdRequest) (resp *types.StorageConfig, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	resourceType := strings.ToUpper(versionDetail.ResourceType)

	var config *types2.StorageConfig

	switch resourceType {
	case "DEPLOYMENT":
		config, err = client.Deployment().GetStorageConfig(versionDetail.Namespace, versionDetail.ResourceName)
	case "STATEFULSET":
		config, err = client.StatefulSet().GetStorageConfig(versionDetail.Namespace, versionDetail.ResourceName)
	case "DAEMONSET":
		config, err = client.DaemonSet().GetStorageConfig(versionDetail.Namespace, versionDetail.ResourceName)
	case "JOB":
		config, err = client.Job().GetStorageConfig(versionDetail.Namespace, versionDetail.ResourceName)
	case "CRONJOB":
		config, err = client.CronJob().GetStorageConfig(versionDetail.Namespace, versionDetail.ResourceName)
	default:
		return nil, fmt.Errorf("资源类型 %s 不支持查询存储配置", resourceType)
	}

	if err != nil {
		l.Errorf("获取存储配置失败: %v", err)
		return nil, fmt.Errorf("获取存储配置失败")
	}

	resp = convertToStorageConfig(config)
	return resp, nil
}

func convertToStorageConfig(config *types2.StorageConfig) *types.StorageConfig {
	result := &types.StorageConfig{
		Volumes:              make([]types.VolumeConfig, 0, len(config.Volumes)),
		VolumeMounts:         make([]types.VolumeMountConfig, 0, len(config.VolumeMounts)),
		VolumeClaimTemplates: make([]types.PersistentVolumeClaimConfig, 0, len(config.VolumeClaimTemplates)),
	}

	// 转换 Volumes
	for _, v := range config.Volumes {
		volume := types.VolumeConfig{
			Name: v.Name,
			Type: v.Type,
		}

		if v.EmptyDir != nil {
			volume.EmptyDir = &types.EmptyDirVolumeConfig{
				Medium:    v.EmptyDir.Medium,
				SizeLimit: v.EmptyDir.SizeLimit,
			}
		}

		if v.HostPath != nil {
			volume.HostPath = &types.HostPathVolumeConfig{
				Path: v.HostPath.Path,
			}
			// 安全处理 Type 指针
			if v.HostPath.Type != nil {
				hostPathType := *v.HostPath.Type
				volume.HostPath.Type = hostPathType
			}
		}

		if v.ConfigMap != nil {
			volume.ConfigMap = &types.ConfigMapVolumeConfig{
				Name:  v.ConfigMap.Name,
				Items: convertToKeyToPathConfigs(v.ConfigMap.Items),
			}
			// 安全处理指针字段
			if v.ConfigMap.DefaultMode != nil {
				defaultMode := *v.ConfigMap.DefaultMode
				volume.ConfigMap.DefaultMode = defaultMode
			}
			if v.ConfigMap.Optional != nil {
				optional := *v.ConfigMap.Optional
				volume.ConfigMap.Optional = optional
			}
		}

		if v.Secret != nil {
			volume.Secret = &types.SecretVolumeConfig{
				SecretName: v.Secret.SecretName,
				Items:      convertToKeyToPathConfigs(v.Secret.Items),
			}
			// 安全处理指针字段
			if v.Secret.DefaultMode != nil {
				defaultMode := *v.Secret.DefaultMode
				volume.Secret.DefaultMode = defaultMode
			}
			if v.Secret.Optional != nil {
				optional := *v.Secret.Optional
				volume.Secret.Optional = optional
			}
		}

		if v.PersistentVolumeClaim != nil {
			volume.PersistentVolumeClaim = &types.PVCVolumeConfig{
				ClaimName: v.PersistentVolumeClaim.ClaimName,
				ReadOnly:  v.PersistentVolumeClaim.ReadOnly,
			}
		}

		if v.NFS != nil {
			volume.NFS = &types.NFSVolumeConfig{
				Server:   v.NFS.Server,
				Path:     v.NFS.Path,
				ReadOnly: v.NFS.ReadOnly,
			}
		}

		result.Volumes = append(result.Volumes, volume)
	}

	// 转换 VolumeMounts
	for _, vm := range config.VolumeMounts {
		mounts := make([]types.VolumeMount, 0, len(vm.Mounts))
		for _, m := range vm.Mounts {
			mounts = append(mounts, types.VolumeMount{
				Name:             m.Name,
				MountPath:        m.MountPath,
				SubPath:          m.SubPath,
				SubPathExpr:      m.SubPathExpr,
				ReadOnly:         m.ReadOnly,
				MountPropagation: m.MountPropagation,
			})
		}

		result.VolumeMounts = append(result.VolumeMounts, types.VolumeMountConfig{
			ContainerName: vm.ContainerName,
			Mounts:        mounts,
		})
	}

	// 转换 VolumeClaimTemplates (StatefulSet 专用)
	for _, vct := range config.VolumeClaimTemplates {
		template := types.PersistentVolumeClaimConfig{
			Name:        vct.Name,
			AccessModes: vct.AccessModes,
			Resources: types.PersistentVolumeClaimResources{
				Requests: types.StorageResourceList{
					Storage: vct.Resources.Requests.Storage,
				},
			},
		}

		// 安全处理指针字段
		if vct.StorageClassName != nil {
			storageClassName := *vct.StorageClassName
			template.StorageClassName = storageClassName
		}

		if vct.VolumeMode != nil {
			volumeMode := *vct.VolumeMode
			template.VolumeMode = volumeMode
		}

		if vct.Selector != nil {
			template.Selector = convertToLabelSelectorConfig(vct.Selector)
		}

		if vct.Resources.Limits.Storage != "" {
			template.Resources.Limits = types.StorageResourceList{
				Storage: vct.Resources.Limits.Storage,
			}
		}

		result.VolumeClaimTemplates = append(result.VolumeClaimTemplates, template)
	}

	return result
}

func convertToKeyToPathConfigs(items []types2.KeyToPathConfig) []types.KeyToPathConfig {
	result := make([]types.KeyToPathConfig, 0, len(items))
	for _, item := range items {
		converted := types.KeyToPathConfig{
			Key:  item.Key,
			Path: item.Path,
		}
		// 安全处理指针字段
		if item.Mode != nil {
			mode := *item.Mode
			converted.Mode = mode
		}
		result = append(result, converted)
	}
	return result
}
