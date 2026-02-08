package workload

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	k8sTypes "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetStorageConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

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

	var config *k8sTypes.StorageConfig

	switch resourceType {
	case "DEPLOYMENT":
		config, err = client.Deployment().GetStorageConfig(versionDetail.Namespace, versionDetail.ResourceName)
	case "STATEFULSET":
		config, err = client.StatefulSet().GetStorageConfig(versionDetail.Namespace, versionDetail.ResourceName)
	case "DAEMONSET":
		config, err = client.DaemonSet().GetStorageConfig(versionDetail.Namespace, versionDetail.ResourceName)
	case "CRONJOB":
		config, err = client.CronJob().GetStorageConfig(versionDetail.Namespace, versionDetail.ResourceName)
	default:
		return nil, fmt.Errorf("资源类型 %s 不支持查询存储配置", resourceType)
	}

	if err != nil {
		l.Errorf("获取存储配置失败: %v", err)
		return nil, fmt.Errorf("获取存储配置失败")
	}

	resp = convertK8sStorageConfigToAPIStorageConfig(config)
	return resp, nil
}

// convertK8sStorageConfigToAPIStorageConfig 将 k8sTypes.StorageConfig 转换为 types.StorageConfig
func convertK8sStorageConfigToAPIStorageConfig(config *k8sTypes.StorageConfig) *types.StorageConfig {
	if config == nil {
		return &types.StorageConfig{
			Volumes:              []types.VolumeConfig{},
			VolumeMounts:         []types.VolumeMountConfig{},
			VolumeClaimTemplates: []types.PersistentVolumeClaimConfig{},
		}
	}

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
			if v.HostPath.Type != nil {
				volume.HostPath.Type = *v.HostPath.Type
			}
		}

		if v.ConfigMap != nil {
			volume.ConfigMap = &types.ConfigMapVolumeConfig{
				Name:  v.ConfigMap.Name,
				Items: convertK8sKeyToPathConfigsToAPI(v.ConfigMap.Items),
			}
			if v.ConfigMap.DefaultMode != nil {
				volume.ConfigMap.DefaultMode = *v.ConfigMap.DefaultMode
			}
			if v.ConfigMap.Optional != nil {
				volume.ConfigMap.Optional = *v.ConfigMap.Optional
			}
		}

		if v.Secret != nil {
			volume.Secret = &types.SecretVolumeConfig{
				SecretName: v.Secret.SecretName,
				Items:      convertK8sKeyToPathConfigsToAPI(v.Secret.Items),
			}
			if v.Secret.DefaultMode != nil {
				volume.Secret.DefaultMode = *v.Secret.DefaultMode
			}
			if v.Secret.Optional != nil {
				volume.Secret.Optional = *v.Secret.Optional
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

		if v.DownwardAPI != nil {
			volume.DownwardAPI = convertK8sDownwardAPIVolumeConfigToAPI(v.DownwardAPI)
		}

		if v.Projected != nil {
			volume.Projected = convertK8sProjectedVolumeConfigToAPI(v.Projected)
		}

		if v.CSI != nil {
			volume.CSI = convertK8sCSIVolumeConfigToAPI(v.CSI)
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
			ContainerType: string(vm.ContainerType),
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

		if vct.StorageClassName != nil {
			template.StorageClassName = *vct.StorageClassName
		}

		if vct.VolumeMode != nil {
			template.VolumeMode = *vct.VolumeMode
		}

		if vct.Selector != nil {
			template.Selector = convertK8sLabelSelectorConfigToAPILabelSelectorConfig(vct.Selector)
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

// convertK8sKeyToPathConfigsToAPI 转换 KeyToPath 配置
func convertK8sKeyToPathConfigsToAPI(items []k8sTypes.KeyToPathConfig) []types.KeyToPathConfig {
	result := make([]types.KeyToPathConfig, 0, len(items))
	for _, item := range items {
		converted := types.KeyToPathConfig{
			Key:  item.Key,
			Path: item.Path,
		}
		if item.Mode != nil {
			converted.Mode = *item.Mode
		}
		result = append(result, converted)
	}
	return result
}

// convertK8sLabelSelectorConfigToAPILabelSelectorConfig 转换标签选择器
func convertK8sLabelSelectorConfigToAPILabelSelectorConfig(selector *k8sTypes.CommLabelSelectorConfig) *types.LabelSelectorConfig {
	if selector == nil {
		return nil
	}

	result := &types.LabelSelectorConfig{
		MatchLabels: selector.MatchLabels,
	}

	if len(selector.MatchExpressions) > 0 {
		result.MatchExpressions = make([]types.LabelSelectorRequirementConfig, 0, len(selector.MatchExpressions))
		for _, expr := range selector.MatchExpressions {
			result.MatchExpressions = append(result.MatchExpressions, types.LabelSelectorRequirementConfig{
				Key:      expr.Key,
				Operator: expr.Operator,
				Values:   expr.Values,
			})
		}
	}

	return result
}

// convertK8sDownwardAPIVolumeConfigToAPI 转换 DownwardAPI 卷配置
func convertK8sDownwardAPIVolumeConfigToAPI(config *k8sTypes.DownwardAPIVolumeConfig) *types.DownwardAPIVolumeConfig {
	if config == nil {
		return nil
	}

	result := &types.DownwardAPIVolumeConfig{}

	if config.DefaultMode != nil {
		result.DefaultMode = *config.DefaultMode
	}

	if len(config.Items) > 0 {
		result.Items = make([]types.DownwardAPIVolumeFileConfig, 0, len(config.Items))
		for _, item := range config.Items {
			fileConfig := types.DownwardAPIVolumeFileConfig{
				Path: item.Path,
			}
			if item.Mode != nil {
				fileConfig.Mode = *item.Mode
			}
			if item.FieldRef != nil {
				fileConfig.FieldRef = &types.ObjectFieldSelector{
					FieldPath: item.FieldRef.FieldPath,
				}
			}
			if item.ResourceFieldRef != nil {
				fileConfig.ResourceFieldRef = &types.ResourceFieldSelector{
					ContainerName: item.ResourceFieldRef.ContainerName,
					Resource:      item.ResourceFieldRef.Resource,
					Divisor:       item.ResourceFieldRef.Divisor,
				}
			}
			result.Items = append(result.Items, fileConfig)
		}
	}

	return result
}

// convertK8sProjectedVolumeConfigToAPI 转换 Projected 卷配置
func convertK8sProjectedVolumeConfigToAPI(config *k8sTypes.ProjectedVolumeConfig) *types.ProjectedVolumeConfig {
	if config == nil {
		return nil
	}

	result := &types.ProjectedVolumeConfig{}

	if config.DefaultMode != nil {
		result.DefaultMode = *config.DefaultMode
	}

	if len(config.Sources) > 0 {
		result.Sources = make([]types.VolumeProjectionConfig, 0, len(config.Sources))
		for _, source := range config.Sources {
			projection := types.VolumeProjectionConfig{}

			if source.Secret != nil {
				projection.Secret = &types.SecretProjectionConfig{
					Name:  source.Secret.Name,
					Items: convertK8sKeyToPathConfigsToAPI(source.Secret.Items),
				}
				if source.Secret.Optional != nil {
					projection.Secret.Optional = *source.Secret.Optional
				}
			}

			if source.ConfigMap != nil {
				projection.ConfigMap = &types.ConfigMapProjectionConfig{
					Name:  source.ConfigMap.Name,
					Items: convertK8sKeyToPathConfigsToAPI(source.ConfigMap.Items),
				}
				if source.ConfigMap.Optional != nil {
					projection.ConfigMap.Optional = *source.ConfigMap.Optional
				}
			}

			if source.DownwardAPI != nil {
				projection.DownwardAPI = &types.DownwardAPIProjectionConfig{
					Items: make([]types.DownwardAPIVolumeFileConfig, 0, len(source.DownwardAPI.Items)),
				}
				for _, item := range source.DownwardAPI.Items {
					fileConfig := types.DownwardAPIVolumeFileConfig{
						Path: item.Path,
					}
					if item.Mode != nil {
						fileConfig.Mode = *item.Mode
					}
					if item.FieldRef != nil {
						fileConfig.FieldRef = &types.ObjectFieldSelector{
							FieldPath: item.FieldRef.FieldPath,
						}
					}
					if item.ResourceFieldRef != nil {
						fileConfig.ResourceFieldRef = &types.ResourceFieldSelector{
							ContainerName: item.ResourceFieldRef.ContainerName,
							Resource:      item.ResourceFieldRef.Resource,
							Divisor:       item.ResourceFieldRef.Divisor,
						}
					}
					projection.DownwardAPI.Items = append(projection.DownwardAPI.Items, fileConfig)
				}
			}

			if source.ServiceAccountToken != nil {
				projection.ServiceAccountToken = &types.ServiceAccountTokenProjectionConfig{
					Audience: source.ServiceAccountToken.Audience,
					Path:     source.ServiceAccountToken.Path,
				}
				if source.ServiceAccountToken.ExpirationSeconds != nil {
					projection.ServiceAccountToken.ExpirationSeconds = *source.ServiceAccountToken.ExpirationSeconds
				}
			}

			result.Sources = append(result.Sources, projection)
		}
	}

	return result
}

// convertK8sCSIVolumeConfigToAPI 转换 CSI 卷配置
func convertK8sCSIVolumeConfigToAPI(config *k8sTypes.CSIVolumeConfig) *types.CSIVolumeConfig {
	if config == nil {
		return nil
	}

	result := &types.CSIVolumeConfig{
		Driver:           config.Driver,
		VolumeAttributes: config.VolumeAttributes,
	}

	if config.ReadOnly != nil {
		result.ReadOnly = *config.ReadOnly
	}

	if config.FSType != nil {
		result.FSType = *config.FSType
	}

	if config.NodePublishSecretRef != nil {
		result.NodePublishSecretRef = &types.SecretReference{
			Name: config.NodePublishSecretRef.Name,
		}
	}

	return result
}
