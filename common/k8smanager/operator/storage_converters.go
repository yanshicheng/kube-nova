package operator

import (
	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ==================== 存储配置转换 ====================

// convertPodSpecToStorageConfig 将 PodSpec 转换为存储配置
func convertPodSpecToStorageConfig(spec *corev1.PodSpec) *types.StorageConfig {
	config := &types.StorageConfig{
		Volumes:      make([]types.VolumeConfig, 0),
		VolumeMounts: make([]types.VolumeMountConfig, 0),
	}

	// 转换 Volumes
	for _, v := range spec.Volumes {
		config.Volumes = append(config.Volumes, convertK8sVolumeToConfig(&v))
	}

	// 转换 VolumeMounts（按容器组织）
	for _, container := range spec.Containers {
		if len(container.VolumeMounts) > 0 {
			vmConfig := types.VolumeMountConfig{
				ContainerName: container.Name,
				Mounts:        make([]types.VolumeMount, 0),
			}
			for _, vm := range container.VolumeMounts {
				mount := types.VolumeMount{
					Name:        vm.Name,
					MountPath:   vm.MountPath,
					SubPath:     vm.SubPath,
					SubPathExpr: vm.SubPathExpr,
					ReadOnly:    vm.ReadOnly,
				}
				// 安全处理 MountPropagation 指针
				if vm.MountPropagation != nil {
					mount.MountPropagation = string(*vm.MountPropagation)
				}
				vmConfig.Mounts = append(vmConfig.Mounts, mount)
			}
			config.VolumeMounts = append(config.VolumeMounts, vmConfig)
		}
	}

	return config
}

// convertK8sVolumeToConfig 将 K8s Volume 转换为配置
func convertK8sVolumeToConfig(volume *corev1.Volume) types.VolumeConfig {
	config := types.VolumeConfig{
		Name: volume.Name,
	}

	// 判断卷类型并转换
	if volume.EmptyDir != nil {
		config.Type = "emptyDir"
		config.EmptyDir = &types.EmptyDirVolumeConfig{
			Medium: string(volume.EmptyDir.Medium),
		}
		if volume.EmptyDir.SizeLimit != nil {
			config.EmptyDir.SizeLimit = volume.EmptyDir.SizeLimit.String()
		}
	} else if volume.HostPath != nil {
		config.Type = "hostPath"
		config.HostPath = &types.HostPathVolumeConfig{
			Path: volume.HostPath.Path,
		}
		// 安全处理 Type 指针
		if volume.HostPath.Type != nil {
			hostPathType := string(*volume.HostPath.Type)
			config.HostPath.Type = &hostPathType
		}
	} else if volume.ConfigMap != nil {
		config.Type = "configMap"
		config.ConfigMap = &types.ConfigMapVolumeConfig{
			Name:  volume.ConfigMap.Name,
			Items: convertK8sKeyToPathItems(volume.ConfigMap.Items),
		}
		// 安全处理指针字段
		if volume.ConfigMap.DefaultMode != nil {
			config.ConfigMap.DefaultMode = volume.ConfigMap.DefaultMode
		}
		if volume.ConfigMap.Optional != nil {
			config.ConfigMap.Optional = volume.ConfigMap.Optional
		}
	} else if volume.Secret != nil {
		config.Type = "secret"
		config.Secret = &types.SecretVolumeConfig{
			SecretName: volume.Secret.SecretName,
			Items:      convertK8sKeyToPathItems(volume.Secret.Items),
		}
		// 安全处理指针字段
		if volume.Secret.DefaultMode != nil {
			config.Secret.DefaultMode = volume.Secret.DefaultMode
		}
		if volume.Secret.Optional != nil {
			config.Secret.Optional = volume.Secret.Optional
		}
	} else if volume.PersistentVolumeClaim != nil {
		config.Type = "persistentVolumeClaim"
		config.PersistentVolumeClaim = &types.PVCVolumeConfig{
			ClaimName: volume.PersistentVolumeClaim.ClaimName,
			ReadOnly:  volume.PersistentVolumeClaim.ReadOnly,
		}
	} else if volume.NFS != nil {
		config.Type = "nfs"
		config.NFS = &types.NFSVolumeConfig{
			Server:   volume.NFS.Server,
			Path:     volume.NFS.Path,
			ReadOnly: volume.NFS.ReadOnly,
		}
	} else if volume.DownwardAPI != nil {
		config.Type = "downwardAPI"
		config.DownwardAPI = &types.DownwardAPIVolumeConfig{
			Items: convertK8sDownwardAPIItems(volume.DownwardAPI.Items),
		}
		// 安全处理 DefaultMode 指针
		if volume.DownwardAPI.DefaultMode != nil {
			config.DownwardAPI.DefaultMode = volume.DownwardAPI.DefaultMode
		}
	} else if volume.Projected != nil {
		config.Type = "projected"
		config.Projected = &types.ProjectedVolumeConfig{
			Sources: convertK8sProjectedSources(volume.Projected.Sources),
		}
		// 安全处理 DefaultMode 指针
		if volume.Projected.DefaultMode != nil {
			config.Projected.DefaultMode = volume.Projected.DefaultMode
		}
	} else if volume.CSI != nil {
		config.Type = "csi"
		config.CSI = &types.CSIVolumeConfig{
			Driver:           volume.CSI.Driver,
			VolumeAttributes: volume.CSI.VolumeAttributes,
		}
		// 安全处理指针字段
		if volume.CSI.ReadOnly != nil {
			config.CSI.ReadOnly = volume.CSI.ReadOnly
		}
		if volume.CSI.FSType != nil {
			config.CSI.FSType = volume.CSI.FSType
		}
		if volume.CSI.NodePublishSecretRef != nil {
			config.CSI.NodePublishSecretRef = &types.SecretReference{
				Name: volume.CSI.NodePublishSecretRef.Name,
			}
		}
	}

	return config
}

func convertK8sKeyToPathItems(items []corev1.KeyToPath) []types.KeyToPathConfig {
	result := make([]types.KeyToPathConfig, 0, len(items))
	for _, item := range items {
		result = append(result, types.KeyToPathConfig{
			Key:  item.Key,
			Path: item.Path,
			Mode: item.Mode,
		})
	}
	return result
}

func convertK8sDownwardAPIItems(items []corev1.DownwardAPIVolumeFile) []types.DownwardAPIVolumeFileConfig {
	result := make([]types.DownwardAPIVolumeFileConfig, 0, len(items))
	for _, item := range items {
		fileConfig := types.DownwardAPIVolumeFileConfig{
			Path: item.Path,
			Mode: item.Mode,
		}
		if item.FieldRef != nil {
			fileConfig.FieldRef = &types.ObjectFieldSelector{
				FieldPath: item.FieldRef.FieldPath,
			}
		}
		if item.ResourceFieldRef != nil {
			divisor := ""
			if !item.ResourceFieldRef.Divisor.IsZero() {
				divisor = item.ResourceFieldRef.Divisor.String()
			}
			fileConfig.ResourceFieldRef = &types.ResourceFieldSelector{
				ContainerName: item.ResourceFieldRef.ContainerName,
				Resource:      item.ResourceFieldRef.Resource,
				Divisor:       divisor,
			}
		}
		result = append(result, fileConfig)
	}
	return result
}

func convertK8sProjectedSources(sources []corev1.VolumeProjection) []types.VolumeProjectionConfig {
	result := make([]types.VolumeProjectionConfig, 0, len(sources))
	for _, source := range sources {
		projection := types.VolumeProjectionConfig{}

		if source.Secret != nil {
			projection.Secret = &types.SecretProjectionConfig{
				Name:  source.Secret.Name,
				Items: convertK8sKeyToPathItems(source.Secret.Items),
			}
			// 安全处理 Optional 指针
			if source.Secret.Optional != nil {
				projection.Secret.Optional = source.Secret.Optional
			}
		}
		if source.ConfigMap != nil {
			projection.ConfigMap = &types.ConfigMapProjectionConfig{
				Name:  source.ConfigMap.Name,
				Items: convertK8sKeyToPathItems(source.ConfigMap.Items),
			}
			// 安全处理 Optional 指针
			if source.ConfigMap.Optional != nil {
				projection.ConfigMap.Optional = source.ConfigMap.Optional
			}
		}
		if source.DownwardAPI != nil {
			projection.DownwardAPI = &types.DownwardAPIProjectionConfig{
				Items: convertK8sDownwardAPIItems(source.DownwardAPI.Items),
			}
		}
		if source.ServiceAccountToken != nil {
			projection.ServiceAccountToken = &types.ServiceAccountTokenProjectionConfig{
				Audience: source.ServiceAccountToken.Audience,
				Path:     source.ServiceAccountToken.Path,
			}
			// 安全处理 ExpirationSeconds 指针
			if source.ServiceAccountToken.ExpirationSeconds != nil {
				projection.ServiceAccountToken.ExpirationSeconds = source.ServiceAccountToken.ExpirationSeconds
			}
		}

		result = append(result, projection)
	}
	return result
}

// convertVolumesConfigToK8s 转换卷配置到 K8s 格式
func convertVolumesConfigToK8s(volumes []types.VolumeConfig) []corev1.Volume {
	result := make([]corev1.Volume, 0, len(volumes))
	for _, v := range volumes {
		k8sVolume := corev1.Volume{
			Name: v.Name,
		}

		switch v.Type {
		case "emptyDir":
			if v.EmptyDir != nil {
				k8sVolume.EmptyDir = &corev1.EmptyDirVolumeSource{
					Medium: corev1.StorageMedium(v.EmptyDir.Medium),
				}
				if v.EmptyDir.SizeLimit != "" {
					quantity, _ := resource.ParseQuantity(v.EmptyDir.SizeLimit)
					k8sVolume.EmptyDir.SizeLimit = &quantity
				}
			}
		case "hostPath":
			if v.HostPath != nil {
				k8sVolume.HostPath = &corev1.HostPathVolumeSource{
					Path: v.HostPath.Path,
				}
				// 安全处理 Type 指针
				if v.HostPath.Type != nil {
					hostPathType := corev1.HostPathType(*v.HostPath.Type)
					k8sVolume.HostPath.Type = &hostPathType
				}
			}
		case "configMap":
			if v.ConfigMap != nil {
				k8sVolume.ConfigMap = &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: v.ConfigMap.Name,
					},
					DefaultMode: v.ConfigMap.DefaultMode,
					Optional:    v.ConfigMap.Optional,
					Items:       convertKeyToPathConfigToK8s(v.ConfigMap.Items),
				}
			}
		case "secret":
			if v.Secret != nil {
				k8sVolume.Secret = &corev1.SecretVolumeSource{
					SecretName:  v.Secret.SecretName,
					DefaultMode: v.Secret.DefaultMode,
					Optional:    v.Secret.Optional,
					Items:       convertKeyToPathConfigToK8s(v.Secret.Items),
				}
			}
		case "persistentVolumeClaim":
			if v.PersistentVolumeClaim != nil {
				k8sVolume.PersistentVolumeClaim = &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: v.PersistentVolumeClaim.ClaimName,
					ReadOnly:  v.PersistentVolumeClaim.ReadOnly,
				}
			}
		case "nfs":
			if v.NFS != nil {
				k8sVolume.NFS = &corev1.NFSVolumeSource{
					Server:   v.NFS.Server,
					Path:     v.NFS.Path,
					ReadOnly: v.NFS.ReadOnly,
				}
			}
		case "downwardAPI":
			if v.DownwardAPI != nil {
				k8sVolume.DownwardAPI = &corev1.DownwardAPIVolumeSource{
					DefaultMode: v.DownwardAPI.DefaultMode,
					Items:       convertDownwardAPIItemsToK8s(v.DownwardAPI.Items),
				}
			}
		case "projected":
			if v.Projected != nil {
				k8sVolume.Projected = &corev1.ProjectedVolumeSource{
					DefaultMode: v.Projected.DefaultMode,
					Sources:     convertProjectedSourcesToK8s(v.Projected.Sources),
				}
			}
		case "csi":
			if v.CSI != nil {
				k8sVolume.CSI = &corev1.CSIVolumeSource{
					Driver:           v.CSI.Driver,
					ReadOnly:         v.CSI.ReadOnly,
					FSType:           v.CSI.FSType,
					VolumeAttributes: v.CSI.VolumeAttributes,
				}
				if v.CSI.NodePublishSecretRef != nil {
					k8sVolume.CSI.NodePublishSecretRef = &corev1.LocalObjectReference{
						Name: v.CSI.NodePublishSecretRef.Name,
					}
				}
			}
		}

		result = append(result, k8sVolume)
	}
	return result
}

func convertKeyToPathConfigToK8s(items []types.KeyToPathConfig) []corev1.KeyToPath {
	result := make([]corev1.KeyToPath, 0, len(items))
	for _, item := range items {
		result = append(result, corev1.KeyToPath{
			Key:  item.Key,
			Path: item.Path,
			Mode: item.Mode,
		})
	}
	return result
}

func convertDownwardAPIItemsToK8s(items []types.DownwardAPIVolumeFileConfig) []corev1.DownwardAPIVolumeFile {
	result := make([]corev1.DownwardAPIVolumeFile, 0, len(items))
	for _, item := range items {
		k8sItem := corev1.DownwardAPIVolumeFile{
			Path: item.Path,
			Mode: item.Mode,
		}
		if item.FieldRef != nil {
			k8sItem.FieldRef = &corev1.ObjectFieldSelector{
				FieldPath: item.FieldRef.FieldPath,
			}
		}
		if item.ResourceFieldRef != nil {
			k8sItem.ResourceFieldRef = &corev1.ResourceFieldSelector{
				ContainerName: item.ResourceFieldRef.ContainerName,
				Resource:      item.ResourceFieldRef.Resource,
			}
			if item.ResourceFieldRef.Divisor != "" {
				quantity, _ := resource.ParseQuantity(item.ResourceFieldRef.Divisor)
				k8sItem.ResourceFieldRef.Divisor = quantity
			}
		}
		result = append(result, k8sItem)
	}
	return result
}

func convertProjectedSourcesToK8s(sources []types.VolumeProjectionConfig) []corev1.VolumeProjection {
	result := make([]corev1.VolumeProjection, 0, len(sources))
	for _, source := range sources {
		projection := corev1.VolumeProjection{}

		if source.Secret != nil {
			projection.Secret = &corev1.SecretProjection{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: source.Secret.Name,
				},
				Optional: source.Secret.Optional,
				Items:    convertKeyToPathConfigToK8s(source.Secret.Items),
			}
		}
		if source.ConfigMap != nil {
			projection.ConfigMap = &corev1.ConfigMapProjection{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: source.ConfigMap.Name,
				},
				Optional: source.ConfigMap.Optional,
				Items:    convertKeyToPathConfigToK8s(source.ConfigMap.Items),
			}
		}
		if source.DownwardAPI != nil {
			projection.DownwardAPI = &corev1.DownwardAPIProjection{
				Items: convertDownwardAPIItemsToK8s(source.DownwardAPI.Items),
			}
		}
		if source.ServiceAccountToken != nil {
			projection.ServiceAccountToken = &corev1.ServiceAccountTokenProjection{
				Audience:          source.ServiceAccountToken.Audience,
				ExpirationSeconds: source.ServiceAccountToken.ExpirationSeconds,
				Path:              source.ServiceAccountToken.Path,
			}
		}

		result = append(result, projection)
	}
	return result
}

// convertVolumeMountsToK8s 转换卷挂载到 K8s 格式
func convertVolumeMountsToK8s(mounts []types.VolumeMount) []corev1.VolumeMount {
	result := make([]corev1.VolumeMount, 0, len(mounts))
	for _, m := range mounts {
		vm := corev1.VolumeMount{
			Name:        m.Name,
			MountPath:   m.MountPath,
			SubPath:     m.SubPath,
			SubPathExpr: m.SubPathExpr,
			ReadOnly:    m.ReadOnly,
		}
		if m.MountPropagation != "" {
			prop := corev1.MountPropagationMode(m.MountPropagation)
			vm.MountPropagation = &prop
		}
		result = append(result, vm)
	}
	return result
}

// convertPVCToConfig 转换 PVC 到配置（StatefulSet 用）
func convertPVCToConfig(pvc *corev1.PersistentVolumeClaim) types.PersistentVolumeClaimConfig {
	config := types.PersistentVolumeClaimConfig{
		Name:        pvc.Name,
		AccessModes: make([]string, 0),
	}

	// 安全处理指针字段
	if pvc.Spec.StorageClassName != nil {
		config.StorageClassName = pvc.Spec.StorageClassName
	}

	if pvc.Spec.VolumeMode != nil {
		volumeMode := string(*pvc.Spec.VolumeMode)
		config.VolumeMode = &volumeMode
	}

	for _, mode := range pvc.Spec.AccessModes {
		config.AccessModes = append(config.AccessModes, string(mode))
	}

	config.Resources = types.PersistentVolumeClaimResources{
		Requests: types.StorageResourceList{
			Storage: pvc.Spec.Resources.Requests.Storage().String(),
		},
	}

	if pvc.Spec.Resources.Limits != nil && pvc.Spec.Resources.Limits.Storage() != nil {
		config.Resources.Limits = types.StorageResourceList{
			Storage: pvc.Spec.Resources.Limits.Storage().String(),
		}
	}

	if pvc.Spec.Selector != nil {
		config.Selector = &types.LabelSelectorConfig{
			MatchLabels: pvc.Spec.Selector.MatchLabels,
		}
		// MatchExpressions 转换省略
	}

	return config
}

// convertPVCConfigsToK8s 转换 PVC 配置列表到 K8s 格式
func convertPVCConfigsToK8s(configs []types.PersistentVolumeClaimConfig) []corev1.PersistentVolumeClaim {
	result := make([]corev1.PersistentVolumeClaim, 0, len(configs))
	for _, config := range configs {
		pvc := corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name: config.Name,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				StorageClassName: config.StorageClassName,
				AccessModes:      make([]corev1.PersistentVolumeAccessMode, 0),
			},
		}

		// 安全处理 VolumeMode 指针
		if config.VolumeMode != nil {
			volumeMode := corev1.PersistentVolumeMode(*config.VolumeMode)
			pvc.Spec.VolumeMode = &volumeMode
		}

		for _, mode := range config.AccessModes {
			pvc.Spec.AccessModes = append(pvc.Spec.AccessModes, corev1.PersistentVolumeAccessMode(mode))
		}

		storage, _ := resource.ParseQuantity(config.Resources.Requests.Storage)
		pvc.Spec.Resources.Requests = corev1.ResourceList{
			corev1.ResourceStorage: storage,
		}

		if config.Resources.Limits.Storage != "" {
			limitStorage, _ := resource.ParseQuantity(config.Resources.Limits.Storage)
			pvc.Spec.Resources.Limits = corev1.ResourceList{
				corev1.ResourceStorage: limitStorage,
			}
		}

		if config.Selector != nil {
			pvc.Spec.Selector = convertLabelSelectorToK8s(config.Selector)
		}

		result = append(result, pvc)
	}
	return result
}
