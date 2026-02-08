package operator

import (
	"fmt"

	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ============================================================================
// 存储配置转换函数
// 适用于：Deployment, DaemonSet, CronJob
// 注意：StatefulSet 需要单独处理 VolumeClaimTemplates，请使用
//       ConvertStatefulSetToStorageConfig 和 ApplyStorageConfigToStatefulSet
// ============================================================================

// ConvertPodSpecToStorageConfig 从 PodSpec 提取存储配置
// 适用于：Deployment, DaemonSet, CronJob
// 注意：不包含 VolumeClaimTemplates（仅 StatefulSet 支持）
func ConvertPodSpecToStorageConfig(spec *corev1.PodSpec) *types.StorageConfig {
	if spec == nil {
		return &types.StorageConfig{
			Volumes:              []types.VolumeConfig{},
			VolumeMounts:         []types.VolumeMountConfig{},
			VolumeClaimTemplates: []types.PersistentVolumeClaimConfig{},
		}
	}

	config := &types.StorageConfig{
		Volumes:              make([]types.VolumeConfig, 0, len(spec.Volumes)),
		VolumeMounts:         make([]types.VolumeMountConfig, 0),
		VolumeClaimTemplates: []types.PersistentVolumeClaimConfig{}, // 非 StatefulSet 资源返回空
	}

	// 转换 Volumes
	for _, vol := range spec.Volumes {
		config.Volumes = append(config.Volumes, convertK8sVolumeToConfig(vol))
	}

	// 转换 Init 容器的 VolumeMounts
	for _, c := range spec.InitContainers {
		config.VolumeMounts = append(config.VolumeMounts, types.VolumeMountConfig{
			ContainerName: c.Name,
			ContainerType: types.ContainerTypeInit,
			Mounts:        convertK8sVolumeMountsToConfig(c.VolumeMounts),
		})
	}

	// 转换主容器的 VolumeMounts
	for _, c := range spec.Containers {
		config.VolumeMounts = append(config.VolumeMounts, types.VolumeMountConfig{
			ContainerName: c.Name,
			ContainerType: types.ContainerTypeMain,
			Mounts:        convertK8sVolumeMountsToConfig(c.VolumeMounts),
		})
	}

	// 转换 Ephemeral 容器的 VolumeMounts
	for _, c := range spec.EphemeralContainers {
		config.VolumeMounts = append(config.VolumeMounts, types.VolumeMountConfig{
			ContainerName: c.Name,
			ContainerType: types.ContainerTypeEphemeral,
			Mounts:        convertK8sVolumeMountsToConfig(c.VolumeMounts),
		})
	}

	return config
}

// ApplyStorageConfigToPodSpec 应用存储配置到 PodSpec
// 适用于：Deployment, DaemonSet, CronJob
// 注意：不处理 VolumeClaimTemplates（仅 StatefulSet 支持）
func ApplyStorageConfigToPodSpec(spec *corev1.PodSpec, config *types.UpdateStorageConfigRequest) error {
	if spec == nil {
		return fmt.Errorf("PodSpec 不能为空")
	}
	if config == nil {
		return nil
	}

	// 更新 Volumes（全量替换）
	if config.Volumes != nil {
		spec.Volumes = convertConfigToK8sVolumes(config.Volumes)
	}

	// 构建容器名到挂载的映射
	mountsMap := make(map[string][]types.VolumeMount)
	containerTypeMap := make(map[string]types.ContainerType)
	for _, vmConfig := range config.VolumeMounts {
		mountsMap[vmConfig.ContainerName] = vmConfig.Mounts
		containerTypeMap[vmConfig.ContainerName] = vmConfig.ContainerType
	}

	// 更新 Init 容器
	for i := range spec.InitContainers {
		name := spec.InitContainers[i].Name
		if mounts, exists := mountsMap[name]; exists {
			if containerTypeMap[name] != "" && containerTypeMap[name] != types.ContainerTypeInit {
				return fmt.Errorf("容器 '%s' 类型不匹配，期望 init，实际 %s", name, containerTypeMap[name])
			}
			spec.InitContainers[i].VolumeMounts = convertConfigToK8sVolumeMounts(mounts)
		}
	}

	// 更新主容器
	for i := range spec.Containers {
		name := spec.Containers[i].Name
		if mounts, exists := mountsMap[name]; exists {
			if containerTypeMap[name] != "" && containerTypeMap[name] != types.ContainerTypeMain {
				return fmt.Errorf("容器 '%s' 类型不匹配，期望 main，实际 %s", name, containerTypeMap[name])
			}
			spec.Containers[i].VolumeMounts = convertConfigToK8sVolumeMounts(mounts)
		}
	}

	return nil
}

// ============================================================================
// StatefulSet 专用存储配置转换
// 注意：StatefulSet 的 VolumeClaimTemplates 不能直接更新，只能在创建时指定
//       更新 VolumeClaimTemplates 需要重建 StatefulSet
// ============================================================================

// ConvertStatefulSetStorageConfig 从 StatefulSet 提取完整存储配置（包含 VolumeClaimTemplates）
// 仅适用于：StatefulSet
func ConvertStatefulSetStorageConfig(spec *corev1.PodSpec, vcts []corev1.PersistentVolumeClaim) *types.StorageConfig {
	config := ConvertPodSpecToStorageConfig(spec)

	// 转换 VolumeClaimTemplates
	if len(vcts) > 0 {
		config.VolumeClaimTemplates = make([]types.PersistentVolumeClaimConfig, 0, len(vcts))
		for _, vct := range vcts {
			config.VolumeClaimTemplates = append(config.VolumeClaimTemplates, convertK8sPVCToConfig(vct))
		}
	}

	return config
}

// ConvertConfigToPVCTemplates 将配置转换为 K8s PVC 模板
// 仅适用于：StatefulSet
// 注意：此函数用于创建 StatefulSet，现有 StatefulSet 的 VolumeClaimTemplates 不可更新
func ConvertConfigToPVCTemplates(configs []types.PersistentVolumeClaimConfig) []corev1.PersistentVolumeClaim {
	if len(configs) == 0 {
		return nil
	}

	pvcs := make([]corev1.PersistentVolumeClaim, 0, len(configs))
	for _, cfg := range configs {
		pvcs = append(pvcs, convertConfigToK8sPVC(cfg))
	}
	return pvcs
}

// ============================================================================
// Volume 转换
// ============================================================================

func convertK8sVolumeToConfig(vol corev1.Volume) types.VolumeConfig {
	config := types.VolumeConfig{Name: vol.Name}

	switch {
	case vol.EmptyDir != nil:
		config.Type = "emptyDir"
		config.EmptyDir = &types.EmptyDirVolumeConfig{
			Medium: string(vol.EmptyDir.Medium),
		}
		if vol.EmptyDir.SizeLimit != nil {
			config.EmptyDir.SizeLimit = vol.EmptyDir.SizeLimit.String()
		}

	case vol.HostPath != nil:
		config.Type = "hostPath"
		config.HostPath = &types.HostPathVolumeConfig{Path: vol.HostPath.Path}
		if vol.HostPath.Type != nil {
			t := string(*vol.HostPath.Type)
			config.HostPath.Type = &t
		}

	case vol.ConfigMap != nil:
		config.Type = "configMap"
		config.ConfigMap = &types.ConfigMapVolumeConfig{
			Name:        vol.ConfigMap.Name,
			Items:       convertK8sKeyToPathToConfig(vol.ConfigMap.Items),
			DefaultMode: vol.ConfigMap.DefaultMode,
			Optional:    vol.ConfigMap.Optional,
		}

	case vol.Secret != nil:
		config.Type = "secret"
		config.Secret = &types.SecretVolumeConfig{
			SecretName:  vol.Secret.SecretName,
			Items:       convertK8sKeyToPathToConfig(vol.Secret.Items),
			DefaultMode: vol.Secret.DefaultMode,
			Optional:    vol.Secret.Optional,
		}

	case vol.PersistentVolumeClaim != nil:
		config.Type = "persistentVolumeClaim"
		config.PersistentVolumeClaim = &types.PVCVolumeConfig{
			ClaimName: vol.PersistentVolumeClaim.ClaimName,
			ReadOnly:  vol.PersistentVolumeClaim.ReadOnly,
		}

	case vol.NFS != nil:
		config.Type = "nfs"
		config.NFS = &types.NFSVolumeConfig{
			Server:   vol.NFS.Server,
			Path:     vol.NFS.Path,
			ReadOnly: vol.NFS.ReadOnly,
		}

	case vol.DownwardAPI != nil:
		config.Type = "downwardAPI"
		config.DownwardAPI = convertK8sDownwardAPIToConfig(vol.DownwardAPI)

	case vol.Projected != nil:
		config.Type = "projected"
		config.Projected = convertK8sProjectedToConfig(vol.Projected)

	case vol.CSI != nil:
		config.Type = "csi"
		config.CSI = convertK8sCSIToConfig(vol.CSI)

	default:
		config.Type = "unknown"
	}

	return config
}

func convertConfigToK8sVolumes(configs []types.VolumeConfig) []corev1.Volume {
	volumes := make([]corev1.Volume, 0, len(configs))

	for _, cfg := range configs {
		vol := corev1.Volume{Name: cfg.Name}

		switch cfg.Type {
		case "emptyDir":
			if cfg.EmptyDir != nil {
				vol.EmptyDir = &corev1.EmptyDirVolumeSource{
					Medium: corev1.StorageMedium(cfg.EmptyDir.Medium),
				}
				if cfg.EmptyDir.SizeLimit != "" {
					if size, err := resource.ParseQuantity(cfg.EmptyDir.SizeLimit); err == nil {
						vol.EmptyDir.SizeLimit = &size
					}
				}
			}

		case "hostPath":
			if cfg.HostPath != nil {
				vol.HostPath = &corev1.HostPathVolumeSource{Path: cfg.HostPath.Path}
				if cfg.HostPath.Type != nil {
					t := corev1.HostPathType(*cfg.HostPath.Type)
					vol.HostPath.Type = &t
				}
			}

		case "configMap":
			if cfg.ConfigMap != nil {
				vol.ConfigMap = &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: cfg.ConfigMap.Name},
					Items:                convertConfigToK8sKeyToPath(cfg.ConfigMap.Items),
					DefaultMode:          cfg.ConfigMap.DefaultMode,
					Optional:             cfg.ConfigMap.Optional,
				}
			}

		case "secret":
			if cfg.Secret != nil {
				vol.Secret = &corev1.SecretVolumeSource{
					SecretName:  cfg.Secret.SecretName,
					Items:       convertConfigToK8sKeyToPath(cfg.Secret.Items),
					DefaultMode: cfg.Secret.DefaultMode,
					Optional:    cfg.Secret.Optional,
				}
			}

		case "persistentVolumeClaim":
			if cfg.PersistentVolumeClaim != nil {
				vol.PersistentVolumeClaim = &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: cfg.PersistentVolumeClaim.ClaimName,
					ReadOnly:  cfg.PersistentVolumeClaim.ReadOnly,
				}
			}

		case "nfs":
			if cfg.NFS != nil {
				vol.NFS = &corev1.NFSVolumeSource{
					Server:   cfg.NFS.Server,
					Path:     cfg.NFS.Path,
					ReadOnly: cfg.NFS.ReadOnly,
				}
			}

		case "downwardAPI":
			if cfg.DownwardAPI != nil {
				vol.DownwardAPI = convertConfigToK8sDownwardAPI(cfg.DownwardAPI)
			}

		case "projected":
			if cfg.Projected != nil {
				vol.Projected = convertConfigToK8sProjected(cfg.Projected)
			}

		case "csi":
			if cfg.CSI != nil {
				vol.CSI = convertConfigToK8sCSI(cfg.CSI)
			}
		}

		volumes = append(volumes, vol)
	}

	return volumes
}

// ============================================================================
// VolumeMount 转换
// ============================================================================

func convertK8sVolumeMountsToConfig(mounts []corev1.VolumeMount) []types.VolumeMount {
	result := make([]types.VolumeMount, 0, len(mounts))
	for _, m := range mounts {
		vm := types.VolumeMount{
			Name:        m.Name,
			MountPath:   m.MountPath,
			SubPath:     m.SubPath,
			SubPathExpr: m.SubPathExpr,
			ReadOnly:    m.ReadOnly,
		}
		if m.MountPropagation != nil {
			vm.MountPropagation = string(*m.MountPropagation)
		}
		result = append(result, vm)
	}
	return result
}

func convertConfigToK8sVolumeMounts(mounts []types.VolumeMount) []corev1.VolumeMount {
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
			mp := corev1.MountPropagationMode(m.MountPropagation)
			vm.MountPropagation = &mp
		}
		result = append(result, vm)
	}
	return result
}

// ============================================================================
// KeyToPath 转换
// ============================================================================

func convertK8sKeyToPathToConfig(items []corev1.KeyToPath) []types.KeyToPathConfig {
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

func convertConfigToK8sKeyToPath(items []types.KeyToPathConfig) []corev1.KeyToPath {
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

// ============================================================================
// DownwardAPI 转换
// ============================================================================

func convertK8sDownwardAPIToConfig(da *corev1.DownwardAPIVolumeSource) *types.DownwardAPIVolumeConfig {
	if da == nil {
		return nil
	}
	config := &types.DownwardAPIVolumeConfig{
		Items:       make([]types.DownwardAPIVolumeFileConfig, 0, len(da.Items)),
		DefaultMode: da.DefaultMode,
	}
	for _, item := range da.Items {
		fileConfig := types.DownwardAPIVolumeFileConfig{Path: item.Path, Mode: item.Mode}
		if item.FieldRef != nil {
			fileConfig.FieldRef = &types.ObjectFieldSelector{FieldPath: item.FieldRef.FieldPath}
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
		config.Items = append(config.Items, fileConfig)
	}
	return config
}

func convertConfigToK8sDownwardAPI(cfg *types.DownwardAPIVolumeConfig) *corev1.DownwardAPIVolumeSource {
	if cfg == nil {
		return nil
	}
	source := &corev1.DownwardAPIVolumeSource{
		Items:       make([]corev1.DownwardAPIVolumeFile, 0, len(cfg.Items)),
		DefaultMode: cfg.DefaultMode,
	}
	for _, item := range cfg.Items {
		fileItem := corev1.DownwardAPIVolumeFile{Path: item.Path, Mode: item.Mode}
		if item.FieldRef != nil {
			fileItem.FieldRef = &corev1.ObjectFieldSelector{FieldPath: item.FieldRef.FieldPath}
		}
		if item.ResourceFieldRef != nil {
			fileItem.ResourceFieldRef = &corev1.ResourceFieldSelector{
				ContainerName: item.ResourceFieldRef.ContainerName,
				Resource:      item.ResourceFieldRef.Resource,
			}
			if item.ResourceFieldRef.Divisor != "" {
				if divisor, err := resource.ParseQuantity(item.ResourceFieldRef.Divisor); err == nil {
					fileItem.ResourceFieldRef.Divisor = divisor
				}
			}
		}
		source.Items = append(source.Items, fileItem)
	}
	return source
}

// ============================================================================
// Projected 转换
// ============================================================================

func convertK8sProjectedToConfig(proj *corev1.ProjectedVolumeSource) *types.ProjectedVolumeConfig {
	if proj == nil {
		return nil
	}
	config := &types.ProjectedVolumeConfig{
		Sources:     make([]types.VolumeProjectionConfig, 0, len(proj.Sources)),
		DefaultMode: proj.DefaultMode,
	}
	for _, src := range proj.Sources {
		projCfg := types.VolumeProjectionConfig{}
		if src.Secret != nil {
			projCfg.Secret = &types.SecretProjectionConfig{
				Name:     src.Secret.Name,
				Items:    convertK8sKeyToPathToConfig(src.Secret.Items),
				Optional: src.Secret.Optional,
			}
		}
		if src.ConfigMap != nil {
			projCfg.ConfigMap = &types.ConfigMapProjectionConfig{
				Name:     src.ConfigMap.Name,
				Items:    convertK8sKeyToPathToConfig(src.ConfigMap.Items),
				Optional: src.ConfigMap.Optional,
			}
		}
		if src.DownwardAPI != nil {
			projCfg.DownwardAPI = &types.DownwardAPIProjectionConfig{
				Items: make([]types.DownwardAPIVolumeFileConfig, 0, len(src.DownwardAPI.Items)),
			}
			for _, item := range src.DownwardAPI.Items {
				fileConfig := types.DownwardAPIVolumeFileConfig{Path: item.Path, Mode: item.Mode}
				if item.FieldRef != nil {
					fileConfig.FieldRef = &types.ObjectFieldSelector{FieldPath: item.FieldRef.FieldPath}
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
				projCfg.DownwardAPI.Items = append(projCfg.DownwardAPI.Items, fileConfig)
			}
		}
		if src.ServiceAccountToken != nil {
			projCfg.ServiceAccountToken = &types.ServiceAccountTokenProjectionConfig{
				Audience:          src.ServiceAccountToken.Audience,
				ExpirationSeconds: src.ServiceAccountToken.ExpirationSeconds,
				Path:              src.ServiceAccountToken.Path,
			}
		}
		config.Sources = append(config.Sources, projCfg)
	}
	return config
}

func convertConfigToK8sProjected(cfg *types.ProjectedVolumeConfig) *corev1.ProjectedVolumeSource {
	if cfg == nil {
		return nil
	}
	source := &corev1.ProjectedVolumeSource{
		Sources:     make([]corev1.VolumeProjection, 0, len(cfg.Sources)),
		DefaultMode: cfg.DefaultMode,
	}
	for _, srcCfg := range cfg.Sources {
		proj := corev1.VolumeProjection{}
		if srcCfg.Secret != nil {
			proj.Secret = &corev1.SecretProjection{
				LocalObjectReference: corev1.LocalObjectReference{Name: srcCfg.Secret.Name},
				Items:                convertConfigToK8sKeyToPath(srcCfg.Secret.Items),
				Optional:             srcCfg.Secret.Optional,
			}
		}
		if srcCfg.ConfigMap != nil {
			proj.ConfigMap = &corev1.ConfigMapProjection{
				LocalObjectReference: corev1.LocalObjectReference{Name: srcCfg.ConfigMap.Name},
				Items:                convertConfigToK8sKeyToPath(srcCfg.ConfigMap.Items),
				Optional:             srcCfg.ConfigMap.Optional,
			}
		}
		if srcCfg.DownwardAPI != nil {
			proj.DownwardAPI = &corev1.DownwardAPIProjection{
				Items: make([]corev1.DownwardAPIVolumeFile, 0, len(srcCfg.DownwardAPI.Items)),
			}
			for _, item := range srcCfg.DownwardAPI.Items {
				fileItem := corev1.DownwardAPIVolumeFile{Path: item.Path, Mode: item.Mode}
				if item.FieldRef != nil {
					fileItem.FieldRef = &corev1.ObjectFieldSelector{FieldPath: item.FieldRef.FieldPath}
				}
				if item.ResourceFieldRef != nil {
					fileItem.ResourceFieldRef = &corev1.ResourceFieldSelector{
						ContainerName: item.ResourceFieldRef.ContainerName,
						Resource:      item.ResourceFieldRef.Resource,
					}
					if item.ResourceFieldRef.Divisor != "" {
						if divisor, err := resource.ParseQuantity(item.ResourceFieldRef.Divisor); err == nil {
							fileItem.ResourceFieldRef.Divisor = divisor
						}
					}
				}
				proj.DownwardAPI.Items = append(proj.DownwardAPI.Items, fileItem)
			}
		}
		if srcCfg.ServiceAccountToken != nil {
			proj.ServiceAccountToken = &corev1.ServiceAccountTokenProjection{
				Audience:          srcCfg.ServiceAccountToken.Audience,
				ExpirationSeconds: srcCfg.ServiceAccountToken.ExpirationSeconds,
				Path:              srcCfg.ServiceAccountToken.Path,
			}
		}
		source.Sources = append(source.Sources, proj)
	}
	return source
}

// ============================================================================
// CSI 转换
// ============================================================================

func convertK8sCSIToConfig(csi *corev1.CSIVolumeSource) *types.CSIVolumeConfig {
	if csi == nil {
		return nil
	}
	config := &types.CSIVolumeConfig{
		Driver:           csi.Driver,
		ReadOnly:         csi.ReadOnly,
		FSType:           csi.FSType,
		VolumeAttributes: csi.VolumeAttributes,
	}
	if csi.NodePublishSecretRef != nil {
		config.NodePublishSecretRef = &types.SecretReference{Name: csi.NodePublishSecretRef.Name}
	}
	return config
}

func convertConfigToK8sCSI(cfg *types.CSIVolumeConfig) *corev1.CSIVolumeSource {
	if cfg == nil {
		return nil
	}
	source := &corev1.CSIVolumeSource{
		Driver:           cfg.Driver,
		ReadOnly:         cfg.ReadOnly,
		FSType:           cfg.FSType,
		VolumeAttributes: cfg.VolumeAttributes,
	}
	if cfg.NodePublishSecretRef != nil {
		source.NodePublishSecretRef = &corev1.LocalObjectReference{Name: cfg.NodePublishSecretRef.Name}
	}
	return source
}

// ============================================================================
// PVC (VolumeClaimTemplate) 转换 - 仅 StatefulSet 使用
// ============================================================================

func convertK8sPVCToConfig(pvc corev1.PersistentVolumeClaim) types.PersistentVolumeClaimConfig {
	config := types.PersistentVolumeClaimConfig{
		Name:             pvc.Name,
		StorageClassName: pvc.Spec.StorageClassName,
	}

	if len(pvc.Spec.AccessModes) > 0 {
		config.AccessModes = make([]string, 0, len(pvc.Spec.AccessModes))
		for _, am := range pvc.Spec.AccessModes {
			config.AccessModes = append(config.AccessModes, string(am))
		}
	}

	if pvc.Spec.VolumeMode != nil {
		vm := string(*pvc.Spec.VolumeMode)
		config.VolumeMode = &vm
	}

	config.Resources = types.PersistentVolumeClaimResources{}
	if pvc.Spec.Resources.Requests != nil {
		if storage := pvc.Spec.Resources.Requests.Storage(); storage != nil {
			config.Resources.Requests = types.StorageResourceList{Storage: storage.String()}
		}
	}
	if pvc.Spec.Resources.Limits != nil {
		if storage := pvc.Spec.Resources.Limits.Storage(); storage != nil {
			config.Resources.Limits = types.StorageResourceList{Storage: storage.String()}
		}
	}

	if pvc.Spec.Selector != nil {
		config.Selector = convertK8sLabelSelectorToConfig(pvc.Spec.Selector)
	}

	return config
}

func convertConfigToK8sPVC(cfg types.PersistentVolumeClaimConfig) corev1.PersistentVolumeClaim {
	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: cfg.Name},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: cfg.StorageClassName,
		},
	}

	if len(cfg.AccessModes) > 0 {
		pvc.Spec.AccessModes = make([]corev1.PersistentVolumeAccessMode, 0, len(cfg.AccessModes))
		for _, am := range cfg.AccessModes {
			pvc.Spec.AccessModes = append(pvc.Spec.AccessModes, corev1.PersistentVolumeAccessMode(am))
		}
	}

	if cfg.VolumeMode != nil {
		vm := corev1.PersistentVolumeMode(*cfg.VolumeMode)
		pvc.Spec.VolumeMode = &vm
	}

	pvc.Spec.Resources = corev1.VolumeResourceRequirements{}
	if cfg.Resources.Requests.Storage != "" {
		if storage, err := resource.ParseQuantity(cfg.Resources.Requests.Storage); err == nil {
			pvc.Spec.Resources.Requests = corev1.ResourceList{corev1.ResourceStorage: storage}
		}
	}
	if cfg.Resources.Limits.Storage != "" {
		if storage, err := resource.ParseQuantity(cfg.Resources.Limits.Storage); err == nil {
			pvc.Spec.Resources.Limits = corev1.ResourceList{corev1.ResourceStorage: storage}
		}
	}

	if cfg.Selector != nil {
		pvc.Spec.Selector = convertConfigToK8sLabelSelector(cfg.Selector)
	}

	return pvc
}

// ============================================================================
// LabelSelector 转换辅助
// ============================================================================

func convertK8sLabelSelectorToConfig(sel *metav1.LabelSelector) *types.CommLabelSelectorConfig {
	if sel == nil {
		return nil
	}
	config := &types.CommLabelSelectorConfig{MatchLabels: sel.MatchLabels}
	if len(sel.MatchExpressions) > 0 {
		config.MatchExpressions = make([]types.LabelSelectorRequirement, 0, len(sel.MatchExpressions))
		for _, expr := range sel.MatchExpressions {
			config.MatchExpressions = append(config.MatchExpressions, types.LabelSelectorRequirement{
				Key:      expr.Key,
				Operator: string(expr.Operator),
				Values:   expr.Values,
			})
		}
	}
	return config
}

func convertConfigToK8sLabelSelector(cfg *types.CommLabelSelectorConfig) *metav1.LabelSelector {
	if cfg == nil {
		return nil
	}
	sel := &metav1.LabelSelector{MatchLabels: cfg.MatchLabels}
	if len(cfg.MatchExpressions) > 0 {
		sel.MatchExpressions = make([]metav1.LabelSelectorRequirement, 0, len(cfg.MatchExpressions))
		for _, expr := range cfg.MatchExpressions {
			sel.MatchExpressions = append(sel.MatchExpressions, metav1.LabelSelectorRequirement{
				Key:      expr.Key,
				Operator: metav1.LabelSelectorOperator(expr.Operator),
				Values:   expr.Values,
			})
		}
	}
	return sel
}
