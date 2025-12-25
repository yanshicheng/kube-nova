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

// 修改存储配置
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

	updateReq := convertToK8sUpdateStorageConfigRequest(req)
	resourceType := strings.ToUpper(versionDetail.ResourceType)

	switch resourceType {
	case "DEPLOYMENT":
		err = client.Deployment().UpdateStorageConfig(versionDetail.Namespace, versionDetail.ResourceName, updateReq)
	case "STATEFULSET":
		err = client.StatefulSet().UpdateStorageConfig(versionDetail.Namespace, versionDetail.ResourceName, updateReq)
	case "DAEMONSET":
		err = client.DaemonSet().UpdateStorageConfig(versionDetail.Namespace, versionDetail.ResourceName, updateReq)
	case "JOB":
		err = client.Job().UpdateStorageConfig(versionDetail.Namespace, versionDetail.ResourceName, updateReq)
	case "CRONJOB":
		err = client.CronJob().UpdateStorageConfig(versionDetail.Namespace, versionDetail.ResourceName, updateReq)
	default:
		return "", fmt.Errorf("资源类型 %s 不支持修改存储配置", resourceType)
	}

	// 构建存储配置详情
	volumeCount := len(req.Volumes)
	mountCount := len(req.VolumeMounts)
	pvcCount := len(req.VolumeClaimTemplates)

	if err != nil {
		l.Errorf("修改存储配置失败: %v", err)
		recordAuditLog(l.ctx, l.svcCtx, versionDetail, "修改存储配置",
			fmt.Sprintf("%s %s/%s 修改存储配置失败, Volumes数量: %d, VolumeMounts数量: %d, PVC模板数量: %d, 错误: %v", resourceType, versionDetail.Namespace, versionDetail.ResourceName, volumeCount, mountCount, pvcCount, err), 2)
		return "", fmt.Errorf("修改存储配置失败")
	}

	recordAuditLog(l.ctx, l.svcCtx, versionDetail, "修改存储配置",
		fmt.Sprintf("%s %s/%s 修改存储配置成功, Volumes数量: %d, VolumeMounts数量: %d, PVC模板数量: %d", resourceType, versionDetail.Namespace, versionDetail.ResourceName, volumeCount, mountCount, pvcCount), 1)
	return "修改存储配置成功", nil
}

func convertToK8sUpdateStorageConfigRequest(req *types.UpdateStorageConfigRequest) *k8sTypes.UpdateStorageConfigRequest {
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
				Items:       convertToK8sKeyToPathConfigs(v.ConfigMap.Items),
				DefaultMode: &defaultMode,
				Optional:    &optional,
			}
		}

		if v.Secret != nil {
			defaultMode := v.Secret.DefaultMode
			optional := v.Secret.Optional
			volume.Secret = &k8sTypes.SecretVolumeConfig{
				SecretName:  v.Secret.SecretName,
				Items:       convertToK8sKeyToPathConfigs(v.Secret.Items),
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
			template.Selector = convertToK8sLabelSelectorConfig(vct.Selector)
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

func convertToK8sKeyToPathConfigs(items []types.KeyToPathConfig) []k8sTypes.KeyToPathConfig {
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
