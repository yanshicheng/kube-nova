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

type UpdateStorageConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateStorageConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateStorageConfigLogic {
	return &UpdateStorageConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateStorageConfigLogic) UpdateStorageConfig(req *types.UpdateStorageConfigRequest) (resp string, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	resourceType := strings.ToUpper(versionDetail.ResourceType)

	// 获取原存储配置
	var oldConfig *k8sTypes.StorageConfig
	switch resourceType {
	case "DEPLOYMENT":
		oldConfig, err = client.Deployment().GetStorageConfig(versionDetail.Namespace, versionDetail.ResourceName)
	case "STATEFULSET":
		oldConfig, err = client.StatefulSet().GetStorageConfig(versionDetail.Namespace, versionDetail.ResourceName)
	case "DAEMONSET":
		oldConfig, err = client.DaemonSet().GetStorageConfig(versionDetail.Namespace, versionDetail.ResourceName)
	case "CRONJOB":
		oldConfig, err = client.CronJob().GetStorageConfig(versionDetail.Namespace, versionDetail.ResourceName)
	default:
		return "", fmt.Errorf("资源类型 %s 不支持修改存储配置", resourceType)
	}

	if err != nil {
		l.Errorf("获取原存储配置失败: %v", err)
		// 继续执行，oldConfig 可能为 nil
	}

	// 转换请求
	updateReq := convertAPIStorageConfigToK8sUpdateRequest(req)

	// 执行更新
	switch resourceType {
	case "DEPLOYMENT":
		err = client.Deployment().UpdateStorageConfig(versionDetail.Namespace, versionDetail.ResourceName, updateReq)
	case "STATEFULSET":
		err = client.StatefulSet().UpdateStorageConfig(versionDetail.Namespace, versionDetail.ResourceName, updateReq)
	case "DAEMONSET":
		err = client.DaemonSet().UpdateStorageConfig(versionDetail.Namespace, versionDetail.ResourceName, updateReq)
	case "CRONJOB":
		err = client.CronJob().UpdateStorageConfig(versionDetail.Namespace, versionDetail.ResourceName, updateReq)
	}

	// 比较并生成变更详情
	changeDetail := compareStorageConfigChanges(oldConfig, req)

	// 如果没有变更，不记录日志
	if changeDetail == "" {
		if err != nil {
			l.Errorf("修改存储配置失败: %v", err)
			return "", fmt.Errorf("修改存储配置失败")
		}
		return "修改存储配置成功(无变更)", nil
	}

	if err != nil {
		l.Errorf("修改存储配置失败: %v", err)
		recordAuditLog(l.ctx, l.svcCtx, versionDetail, "修改存储配置",
			fmt.Sprintf("%s %s/%s 修改存储配置失败, %s, 错误: %v", resourceType, versionDetail.Namespace, versionDetail.ResourceName, changeDetail, err), 2)
		return "", fmt.Errorf("修改存储配置失败")
	}

	recordAuditLog(l.ctx, l.svcCtx, versionDetail, "修改存储配置",
		fmt.Sprintf("%s %s/%s 修改存储配置成功, %s", resourceType, versionDetail.Namespace, versionDetail.ResourceName, changeDetail), 1)
	return "修改存储配置成功", nil
}

// convertAPIStorageConfigToK8sUpdateRequest 将 API 请求转换为 k8s 更新请求
func convertAPIStorageConfigToK8sUpdateRequest(req *types.UpdateStorageConfigRequest) *k8sTypes.UpdateStorageConfigRequest {
	if req == nil {
		return nil
	}

	result := &k8sTypes.UpdateStorageConfigRequest{
		Volumes:              make([]k8sTypes.VolumeConfig, 0, len(req.Volumes)),
		VolumeMounts:         make([]k8sTypes.VolumeMountConfig, 0, len(req.VolumeMounts)),
		VolumeClaimTemplates: make([]k8sTypes.PersistentVolumeClaimConfig, 0, len(req.VolumeClaimTemplates)),
	}

	// 转换 Volumes
	for _, v := range req.Volumes {
		volume := k8sTypes.VolumeConfig{
			Name: v.Name,
			Type: v.Type,
		}

		if v.EmptyDir != nil {
			volume.EmptyDir = &k8sTypes.EmptyDirVolumeConfig{
				Medium:    v.EmptyDir.Medium,
				SizeLimit: v.EmptyDir.SizeLimit,
			}
		}

		if v.HostPath != nil {
			hostPathType := v.HostPath.Type
			volume.HostPath = &k8sTypes.HostPathVolumeConfig{
				Path: v.HostPath.Path,
				Type: &hostPathType,
			}
		}

		if v.ConfigMap != nil {
			defaultMode := v.ConfigMap.DefaultMode
			optional := v.ConfigMap.Optional
			volume.ConfigMap = &k8sTypes.ConfigMapVolumeConfig{
				Name:        v.ConfigMap.Name,
				Items:       convertAPIKeyToPathConfigsToK8s(v.ConfigMap.Items),
				DefaultMode: &defaultMode,
				Optional:    &optional,
			}
		}

		if v.Secret != nil {
			defaultMode := v.Secret.DefaultMode
			optional := v.Secret.Optional
			volume.Secret = &k8sTypes.SecretVolumeConfig{
				SecretName:  v.Secret.SecretName,
				Items:       convertAPIKeyToPathConfigsToK8s(v.Secret.Items),
				DefaultMode: &defaultMode,
				Optional:    &optional,
			}
		}

		if v.PersistentVolumeClaim != nil {
			volume.PersistentVolumeClaim = &k8sTypes.PVCVolumeConfig{
				ClaimName: v.PersistentVolumeClaim.ClaimName,
				ReadOnly:  v.PersistentVolumeClaim.ReadOnly,
			}
		}

		if v.NFS != nil {
			volume.NFS = &k8sTypes.NFSVolumeConfig{
				Server:   v.NFS.Server,
				Path:     v.NFS.Path,
				ReadOnly: v.NFS.ReadOnly,
			}
		}

		if v.DownwardAPI != nil {
			volume.DownwardAPI = convertAPIDownwardAPIVolumeConfigToK8s(v.DownwardAPI)
		}

		if v.Projected != nil {
			volume.Projected = convertAPIProjectedVolumeConfigToK8s(v.Projected)
		}

		if v.CSI != nil {
			volume.CSI = convertAPICSIVolumeConfigToK8s(v.CSI)
		}

		result.Volumes = append(result.Volumes, volume)
	}

	// 转换 VolumeMounts
	for _, vm := range req.VolumeMounts {
		mounts := make([]k8sTypes.VolumeMount, 0, len(vm.Mounts))
		for _, m := range vm.Mounts {
			mounts = append(mounts, k8sTypes.VolumeMount{
				Name:             m.Name,
				MountPath:        m.MountPath,
				SubPath:          m.SubPath,
				SubPathExpr:      m.SubPathExpr,
				ReadOnly:         m.ReadOnly,
				MountPropagation: m.MountPropagation,
			})
		}

		result.VolumeMounts = append(result.VolumeMounts, k8sTypes.VolumeMountConfig{
			ContainerName: vm.ContainerName,
			ContainerType: k8sTypes.ContainerType(vm.ContainerType),
			Mounts:        mounts,
		})
	}

	// 转换 VolumeClaimTemplates
	for _, vct := range req.VolumeClaimTemplates {
		storageClassName := vct.StorageClassName
		volumeMode := vct.VolumeMode

		template := k8sTypes.PersistentVolumeClaimConfig{
			Name:             vct.Name,
			StorageClassName: &storageClassName,
			AccessModes:      vct.AccessModes,
			Resources: k8sTypes.PersistentVolumeClaimResources{
				Requests: k8sTypes.StorageResourceList{
					Storage: vct.Resources.Requests.Storage,
				},
			},
			VolumeMode: &volumeMode,
		}

		if vct.Selector != nil {
			template.Selector = convertAPILabelSelectorConfigToK8s(vct.Selector)
		}

		if vct.Resources.Limits.Storage != "" {
			template.Resources.Limits = k8sTypes.StorageResourceList{
				Storage: vct.Resources.Limits.Storage,
			}
		}

		result.VolumeClaimTemplates = append(result.VolumeClaimTemplates, template)
	}

	return result
}

// convertAPIKeyToPathConfigsToK8s 转换 KeyToPath 配置
func convertAPIKeyToPathConfigsToK8s(items []types.KeyToPathConfig) []k8sTypes.KeyToPathConfig {
	result := make([]k8sTypes.KeyToPathConfig, 0, len(items))
	for _, item := range items {
		mode := item.Mode
		result = append(result, k8sTypes.KeyToPathConfig{
			Key:  item.Key,
			Path: item.Path,
			Mode: &mode,
		})
	}
	return result
}

// convertAPILabelSelectorConfigToK8s 转换标签选择器
func convertAPILabelSelectorConfigToK8s(selector *types.LabelSelectorConfig) *k8sTypes.CommLabelSelectorConfig {
	if selector == nil {
		return nil
	}

	result := &k8sTypes.CommLabelSelectorConfig{
		MatchLabels: selector.MatchLabels,
	}

	if len(selector.MatchExpressions) > 0 {
		result.MatchExpressions = make([]k8sTypes.LabelSelectorRequirement, 0, len(selector.MatchExpressions))
		for _, expr := range selector.MatchExpressions {
			result.MatchExpressions = append(result.MatchExpressions, k8sTypes.LabelSelectorRequirement{
				Key:      expr.Key,
				Operator: expr.Operator,
				Values:   expr.Values,
			})
		}
	}

	return result
}

// convertAPIDownwardAPIVolumeConfigToK8s 转换 DownwardAPI 卷配置
func convertAPIDownwardAPIVolumeConfigToK8s(config *types.DownwardAPIVolumeConfig) *k8sTypes.DownwardAPIVolumeConfig {
	if config == nil {
		return nil
	}

	defaultMode := config.DefaultMode
	result := &k8sTypes.DownwardAPIVolumeConfig{
		DefaultMode: &defaultMode,
	}

	if len(config.Items) > 0 {
		result.Items = make([]k8sTypes.DownwardAPIVolumeFileConfig, 0, len(config.Items))
		for _, item := range config.Items {
			mode := item.Mode
			fileConfig := k8sTypes.DownwardAPIVolumeFileConfig{
				Path: item.Path,
				Mode: &mode,
			}
			if item.FieldRef != nil {
				fileConfig.FieldRef = &k8sTypes.ObjectFieldSelector{
					FieldPath: item.FieldRef.FieldPath,
				}
			}
			if item.ResourceFieldRef != nil {
				fileConfig.ResourceFieldRef = &k8sTypes.ResourceFieldSelector{
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

// convertAPIProjectedVolumeConfigToK8s 转换 Projected 卷配置
func convertAPIProjectedVolumeConfigToK8s(config *types.ProjectedVolumeConfig) *k8sTypes.ProjectedVolumeConfig {
	if config == nil {
		return nil
	}

	defaultMode := config.DefaultMode
	result := &k8sTypes.ProjectedVolumeConfig{
		DefaultMode: &defaultMode,
	}

	if len(config.Sources) > 0 {
		result.Sources = make([]k8sTypes.VolumeProjectionConfig, 0, len(config.Sources))
		for _, source := range config.Sources {
			projection := k8sTypes.VolumeProjectionConfig{}

			if source.Secret != nil {
				optional := source.Secret.Optional
				projection.Secret = &k8sTypes.SecretProjectionConfig{
					Name:     source.Secret.Name,
					Items:    convertAPIKeyToPathConfigsToK8s(source.Secret.Items),
					Optional: &optional,
				}
			}

			if source.ConfigMap != nil {
				optional := source.ConfigMap.Optional
				projection.ConfigMap = &k8sTypes.ConfigMapProjectionConfig{
					Name:     source.ConfigMap.Name,
					Items:    convertAPIKeyToPathConfigsToK8s(source.ConfigMap.Items),
					Optional: &optional,
				}
			}

			if source.DownwardAPI != nil {
				projection.DownwardAPI = &k8sTypes.DownwardAPIProjectionConfig{
					Items: make([]k8sTypes.DownwardAPIVolumeFileConfig, 0, len(source.DownwardAPI.Items)),
				}
				for _, item := range source.DownwardAPI.Items {
					mode := item.Mode
					fileConfig := k8sTypes.DownwardAPIVolumeFileConfig{
						Path: item.Path,
						Mode: &mode,
					}
					if item.FieldRef != nil {
						fileConfig.FieldRef = &k8sTypes.ObjectFieldSelector{
							FieldPath: item.FieldRef.FieldPath,
						}
					}
					if item.ResourceFieldRef != nil {
						fileConfig.ResourceFieldRef = &k8sTypes.ResourceFieldSelector{
							ContainerName: item.ResourceFieldRef.ContainerName,
							Resource:      item.ResourceFieldRef.Resource,
							Divisor:       item.ResourceFieldRef.Divisor,
						}
					}
					projection.DownwardAPI.Items = append(projection.DownwardAPI.Items, fileConfig)
				}
			}

			if source.ServiceAccountToken != nil {
				expirationSeconds := source.ServiceAccountToken.ExpirationSeconds
				projection.ServiceAccountToken = &k8sTypes.ServiceAccountTokenProjectionConfig{
					Audience:          source.ServiceAccountToken.Audience,
					Path:              source.ServiceAccountToken.Path,
					ExpirationSeconds: &expirationSeconds,
				}
			}

			result.Sources = append(result.Sources, projection)
		}
	}

	return result
}

// convertAPICSIVolumeConfigToK8s 转换 CSI 卷配置
func convertAPICSIVolumeConfigToK8s(config *types.CSIVolumeConfig) *k8sTypes.CSIVolumeConfig {
	if config == nil {
		return nil
	}

	readOnly := config.ReadOnly
	fsType := config.FSType
	result := &k8sTypes.CSIVolumeConfig{
		Driver:           config.Driver,
		ReadOnly:         &readOnly,
		FSType:           &fsType,
		VolumeAttributes: config.VolumeAttributes,
	}

	if config.NodePublishSecretRef != nil {
		result.NodePublishSecretRef = &k8sTypes.SecretReference{
			Name: config.NodePublishSecretRef.Name,
		}
	}

	return result
}

// ==================== 变更比较函数 ====================

// compareStorageConfigChanges 比较存储配置变更，返回变更详情
// 如果没有变更返回空字符串
func compareStorageConfigChanges(oldConfig *k8sTypes.StorageConfig, newReq *types.UpdateStorageConfigRequest) string {
	if newReq == nil {
		return ""
	}

	var changes []string

	// 比较 Volumes
	oldVolumeCount := 0
	if oldConfig != nil {
		oldVolumeCount = len(oldConfig.Volumes)
	}
	newVolumeCount := len(newReq.Volumes)

	if oldVolumeCount != newVolumeCount {
		changes = append(changes, fmt.Sprintf("Volumes数量: %d -> %d", oldVolumeCount, newVolumeCount))
	} else if oldConfig != nil {
		volumeChanges := compareVolumes(oldConfig.Volumes, newReq.Volumes)
		if volumeChanges != "" {
			changes = append(changes, volumeChanges)
		}
	}

	// 比较 VolumeMounts
	oldMountCount := 0
	if oldConfig != nil {
		oldMountCount = len(oldConfig.VolumeMounts)
	}
	newMountCount := len(newReq.VolumeMounts)

	if oldMountCount != newMountCount {
		changes = append(changes, fmt.Sprintf("VolumeMounts数量: %d -> %d", oldMountCount, newMountCount))
	} else if oldConfig != nil {
		mountChanges := compareVolumeMounts(oldConfig.VolumeMounts, newReq.VolumeMounts)
		if mountChanges != "" {
			changes = append(changes, mountChanges)
		}
	}

	// 比较 VolumeClaimTemplates (StatefulSet)
	oldPVCCount := 0
	if oldConfig != nil {
		oldPVCCount = len(oldConfig.VolumeClaimTemplates)
	}
	newPVCCount := len(newReq.VolumeClaimTemplates)

	if oldPVCCount != newPVCCount {
		changes = append(changes, fmt.Sprintf("VolumeClaimTemplates数量: %d -> %d", oldPVCCount, newPVCCount))
	}

	if len(changes) == 0 {
		return ""
	}

	return "存储配置变更: " + strings.Join(changes, "; ")
}

// compareVolumes 比较卷配置变更
func compareVolumes(oldVolumes []k8sTypes.VolumeConfig, newVolumes []types.VolumeConfig) string {
	if len(oldVolumes) != len(newVolumes) {
		return fmt.Sprintf("Volumes数量变化: %d -> %d", len(oldVolumes), len(newVolumes))
	}

	// 构建旧配置的 map
	oldMap := make(map[string]k8sTypes.VolumeConfig)
	for _, v := range oldVolumes {
		oldMap[v.Name] = v
	}

	var changes []string
	for _, newV := range newVolumes {
		oldV, exists := oldMap[newV.Name]
		if !exists {
			changes = append(changes, fmt.Sprintf("新增卷[%s](%s)", newV.Name, newV.Type))
			continue
		}

		if oldV.Type != newV.Type {
			changes = append(changes, fmt.Sprintf("卷[%s]类型: %s -> %s", newV.Name, oldV.Type, newV.Type))
		}
	}

	// 检查删除的卷
	newMap := make(map[string]bool)
	for _, v := range newVolumes {
		newMap[v.Name] = true
	}
	for _, oldV := range oldVolumes {
		if !newMap[oldV.Name] {
			changes = append(changes, fmt.Sprintf("删除卷[%s]", oldV.Name))
		}
	}

	if len(changes) == 0 {
		return ""
	}
	return strings.Join(changes, ", ")
}

// compareVolumeMounts 比较卷挂载变更
func compareVolumeMounts(oldMounts []k8sTypes.VolumeMountConfig, newMounts []types.VolumeMountConfig) string {
	// 构建旧配置的 map
	oldMap := make(map[string]k8sTypes.VolumeMountConfig)
	for _, vm := range oldMounts {
		oldMap[vm.ContainerName] = vm
	}

	var changes []string
	for _, newVM := range newMounts {
		containerName := newVM.ContainerName
		containerType := newVM.ContainerType
		if containerType == "" {
			containerType = "main"
		}

		oldVM, exists := oldMap[containerName]
		if !exists {
			changes = append(changes, fmt.Sprintf("容器[%s](%s)新增%d个挂载点", containerName, containerType, len(newVM.Mounts)))
			continue
		}

		if len(oldVM.Mounts) != len(newVM.Mounts) {
			changes = append(changes, fmt.Sprintf("容器[%s](%s)挂载点数量: %d -> %d",
				containerName, containerType, len(oldVM.Mounts), len(newVM.Mounts)))
		}
	}

	if len(changes) == 0 {
		return ""
	}
	return strings.Join(changes, ", ")
}
