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

type UpdateResourceQuotaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateResourceQuotaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateResourceQuotaLogic {
	return &UpdateResourceQuotaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateResourceQuotaLogic) UpdateResourceQuota(req *types.CommUpdateResourcesRequest) (resp string, err error) {
	versionDetail, controller, err := getResourceController(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取资源控制器失败: %v", err)
		return "", err
	}

	resourceType := strings.ToUpper(versionDetail.ResourceType)

	// 获取原资源配额配置
	var oldResources *k8sTypes.ResourcesResponse
	switch resourceType {
	case "DEPLOYMENT":
		oldResources, err = controller.Deployment.GetResources(versionDetail.Namespace, versionDetail.ResourceName)
	case "STATEFULSET":
		oldResources, err = controller.StatefulSet.GetResources(versionDetail.Namespace, versionDetail.ResourceName)
	case "DAEMONSET":
		oldResources, err = controller.DaemonSet.GetResources(versionDetail.Namespace, versionDetail.ResourceName)
	case "CRONJOB":
		oldResources, err = controller.CronJob.GetResources(versionDetail.Namespace, versionDetail.ResourceName)
	default:
		return "", fmt.Errorf("资源类型 %s 不支持修改资源配额", resourceType)
	}

	if err != nil {
		l.Errorf("获取原资源配额配置失败: %v", err)
		// 继续执行，oldResources 可能为 nil
	}

	// 构建更新请求
	updateReq := convertToK8sUpdateResourcesRequest(versionDetail.ResourceName, versionDetail.Namespace, req)

	// 执行更新
	switch resourceType {
	case "DEPLOYMENT":
		err = controller.Deployment.UpdateResources(updateReq)
	case "STATEFULSET":
		err = controller.StatefulSet.UpdateResources(updateReq)
	case "DAEMONSET":
		err = controller.DaemonSet.UpdateResources(updateReq)
	case "CRONJOB":
		err = controller.CronJob.UpdateResources(updateReq)
	}

	// 比较并生成变更详情
	changeDetail := compareResourcesQuota(oldResources, req)

	// 如果没有变更，不记录日志
	if changeDetail == "" {
		if err != nil {
			l.Errorf("修改资源配额失败: %v", err)
			return "", fmt.Errorf("修改资源配额失败")
		}
		return "修改资源配额成功(无变更)", nil
	}

	if err != nil {
		l.Errorf("修改资源配额失败: %v", err)
		recordAuditLog(l.ctx, l.svcCtx, versionDetail, "修改资源配额",
			fmt.Sprintf("%s %s/%s 修改资源配额失败, %s, 错误: %v", resourceType, versionDetail.Namespace, versionDetail.ResourceName, changeDetail, err), 2)
		return "", fmt.Errorf("修改资源配额失败")
	}

	recordAuditLog(l.ctx, l.svcCtx, versionDetail, "修改资源配额",
		fmt.Sprintf("%s %s/%s 修改资源配额成功, %s", resourceType, versionDetail.Namespace, versionDetail.ResourceName, changeDetail), 1)
	return "修改资源配额成功", nil
}

// convertToK8sUpdateResourcesRequest 将 API 请求转换为 k8s 更新请求
func convertToK8sUpdateResourcesRequest(name, namespace string, req *types.CommUpdateResourcesRequest) *k8sTypes.UpdateResourcesRequest {
	if req == nil {
		return nil
	}

	containers := make([]k8sTypes.ContainerResources, 0, len(req.Containers))
	for _, c := range req.Containers {
		containers = append(containers, k8sTypes.ContainerResources{
			ContainerName: c.ContainerName,
			ContainerType: k8sTypes.ContainerType(c.ContainerType),
			Resources: k8sTypes.ResourceRequirements{
				Limits: k8sTypes.ResourceList{
					Cpu:    c.Resources.Limits.Cpu,
					Memory: c.Resources.Limits.Memory,
				},
				Requests: k8sTypes.ResourceList{
					Cpu:    c.Resources.Requests.Cpu,
					Memory: c.Resources.Requests.Memory,
				},
			},
		})
	}

	return &k8sTypes.UpdateResourcesRequest{
		Name:       name,
		Namespace:  namespace,
		Containers: containers,
	}
}

// compareResourcesQuota 比较资源配额变更，返回变更详情
// 如果没有变更返回空字符串
func compareResourcesQuota(oldResources *k8sTypes.ResourcesResponse, newReq *types.CommUpdateResourcesRequest) string {
	if newReq == nil || len(newReq.Containers) == 0 {
		return ""
	}

	// 构建旧配置的 map，方便查找
	oldMap := make(map[string]k8sTypes.ContainerResources)
	if oldResources != nil {
		for _, c := range oldResources.Containers {
			oldMap[c.ContainerName] = c
		}
	}

	var changes []string

	for _, newContainer := range newReq.Containers {
		containerName := newContainer.ContainerName
		containerType := newContainer.ContainerType
		if containerType == "" {
			containerType = "main"
		}

		oldContainer, exists := oldMap[containerName]

		var containerChanges []string

		if !exists {
			// 新容器配置
			if newContainer.Resources.Limits.Cpu != "" || newContainer.Resources.Limits.Memory != "" ||
				newContainer.Resources.Requests.Cpu != "" || newContainer.Resources.Requests.Memory != "" {
				containerChanges = append(containerChanges, fmt.Sprintf("新增配置 Limits(CPU: %s, Memory: %s), Requests(CPU: %s, Memory: %s)",
					defaultIfEmpty(newContainer.Resources.Limits.Cpu, "未设置"),
					defaultIfEmpty(newContainer.Resources.Limits.Memory, "未设置"),
					defaultIfEmpty(newContainer.Resources.Requests.Cpu, "未设置"),
					defaultIfEmpty(newContainer.Resources.Requests.Memory, "未设置")))
			}
		} else {
			// 比较 Limits CPU
			if newContainer.Resources.Limits.Cpu != oldContainer.Resources.Limits.Cpu {
				containerChanges = append(containerChanges, fmt.Sprintf("Limits.CPU: %s -> %s",
					defaultIfEmpty(oldContainer.Resources.Limits.Cpu, "未设置"),
					defaultIfEmpty(newContainer.Resources.Limits.Cpu, "未设置")))
			}

			// 比较 Limits Memory
			if newContainer.Resources.Limits.Memory != oldContainer.Resources.Limits.Memory {
				containerChanges = append(containerChanges, fmt.Sprintf("Limits.Memory: %s -> %s",
					defaultIfEmpty(oldContainer.Resources.Limits.Memory, "未设置"),
					defaultIfEmpty(newContainer.Resources.Limits.Memory, "未设置")))
			}

			// 比较 Requests CPU
			if newContainer.Resources.Requests.Cpu != oldContainer.Resources.Requests.Cpu {
				containerChanges = append(containerChanges, fmt.Sprintf("Requests.CPU: %s -> %s",
					defaultIfEmpty(oldContainer.Resources.Requests.Cpu, "未设置"),
					defaultIfEmpty(newContainer.Resources.Requests.Cpu, "未设置")))
			}

			// 比较 Requests Memory
			if newContainer.Resources.Requests.Memory != oldContainer.Resources.Requests.Memory {
				containerChanges = append(containerChanges, fmt.Sprintf("Requests.Memory: %s -> %s",
					defaultIfEmpty(oldContainer.Resources.Requests.Memory, "未设置"),
					defaultIfEmpty(newContainer.Resources.Requests.Memory, "未设置")))
			}
		}

		if len(containerChanges) > 0 {
			changes = append(changes, fmt.Sprintf("容器[%s](%s): %s", containerName, containerType, strings.Join(containerChanges, ", ")))
		}
	}

	if len(changes) == 0 {
		return ""
	}

	return "资源配额变更: " + strings.Join(changes, "; ")
}

// defaultIfEmpty 如果字符串为空返回默认值
func defaultIfEmpty(s, defaultVal string) string {
	if s == "" {
		return defaultVal
	}
	return s
}
