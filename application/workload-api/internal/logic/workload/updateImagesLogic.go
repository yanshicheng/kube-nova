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

type UpdateImagesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateImagesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateImagesLogic {
	return &UpdateImagesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ImageChangeDetail 镜像变更详情
type ImageChangeDetail struct {
	ContainerType string // 容器类型：init/main/ephemeral
	ContainerName string // 容器名称
	OldImage      string // 旧镜像
	NewImage      string // 新镜像
}

// UpdateImages 批量更新镜像
func (l *UpdateImagesLogic) UpdateImages(req *types.CommUpdateImagesRequest) (resp string, err error) {
	versionDetail, controller, err := getResourceController(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取资源控制器失败: %v", err)
		return "", err
	}

	resourceType := strings.ToUpper(versionDetail.ResourceType)

	// 获取原镜像配置（用于对比变更）
	var oldImages *k8sTypes.ContainerInfoList
	switch resourceType {
	case "DEPLOYMENT":
		oldImages, err = controller.Deployment.GetContainerImages(versionDetail.Namespace, versionDetail.ResourceName)
	case "STATEFULSET":
		oldImages, err = controller.StatefulSet.GetContainerImages(versionDetail.Namespace, versionDetail.ResourceName)
	case "DAEMONSET":
		oldImages, err = controller.DaemonSet.GetContainerImages(versionDetail.Namespace, versionDetail.ResourceName)
	case "JOB":
		oldImages, err = controller.Job.GetContainerImages(versionDetail.Namespace, versionDetail.ResourceName)
	case "CRONJOB":
		oldImages, err = controller.CronJob.GetContainerImages(versionDetail.Namespace, versionDetail.ResourceName)
	default:
		return "", fmt.Errorf("资源类型 %s 不支持批量更新镜像操作", resourceType)
	}

	if err != nil {
		l.Errorf("获取原镜像配置失败: %v", err)
		return "", fmt.Errorf("获取原镜像配置失败: %v", err)
	}

	// 将 API 类型转换为 k8s 类型
	newImages := convertToK8sCommContainerInfoList(req.Containers)

	// 比较新旧镜像，获取实际变更列表
	changes := compareAndGetChanges(oldImages, newImages)

	// 如果没有实际变更，直接返回
	if len(changes) == 0 {
		l.Infof("镜像配置无变更: %s/%s", versionDetail.Namespace, versionDetail.ResourceName)
		return "镜像配置无变更", nil
	}

	// 构建更新镜像请求
	updateReq := &k8sTypes.UpdateImagesRequest{
		Name:       versionDetail.ResourceName,
		Namespace:  versionDetail.Namespace,
		Containers: *newImages,
		Reason:     req.Reason,
	}

	// 执行更新
	switch resourceType {
	case "DEPLOYMENT":
		err = controller.Deployment.UpdateImages(updateReq)
	case "STATEFULSET":
		err = controller.StatefulSet.UpdateImages(updateReq)
	case "DAEMONSET":
		err = controller.DaemonSet.UpdateImages(updateReq)
	case "CRONJOB":
		err = controller.CronJob.UpdateImages(updateReq)
	}

	// 生成变更详情字符串
	changeDetail := formatChangeDetails(changes)

	if err != nil {
		l.Errorf("批量更新镜像失败: %v", err)
		recordAuditLog(l.ctx, l.svcCtx, versionDetail, "批量镜像更新",
			fmt.Sprintf("%s %s/%s 批量更新镜像失败, %s, 错误: %v",
				resourceType, versionDetail.Namespace, versionDetail.ResourceName, changeDetail, err), 2)
		return "", err
	}

	// 记录成功的审计日志
	recordAuditLog(l.ctx, l.svcCtx, versionDetail, "批量镜像更新",
		fmt.Sprintf("%s %s/%s 批量更新镜像成功, %s",
			resourceType, versionDetail.Namespace, versionDetail.ResourceName, changeDetail), 1)
	l.Infof("成功批量更新镜像: %s/%s, %s", versionDetail.Namespace, versionDetail.ResourceName, changeDetail)

	return "批量更新镜像成功", nil
}

// convertToK8sCommContainerInfoList 将 API 类型转换为 k8s 类型
func convertToK8sCommContainerInfoList(containers types.CommContainerInfoList) *k8sTypes.CommContainerInfoList {
	result := &k8sTypes.CommContainerInfoList{
		Containers: make([]k8sTypes.ContainerImageInfo, 0, len(containers.Containers)),
	}

	for _, c := range containers.Containers {
		result.Containers = append(result.Containers, k8sTypes.ContainerImageInfo{
			ContainerName: c.ContainerName,
			ContainerType: k8sTypes.ContainerType(c.ContainerType),
			Image:         c.Image,
		})
	}

	return result
}

// compareAndGetChanges 比较新旧镜像配置，返回实际变更的列表
func compareAndGetChanges(oldImages *k8sTypes.ContainerInfoList, newImages *k8sTypes.CommContainerInfoList) []ImageChangeDetail {
	if oldImages == nil || newImages == nil {
		return nil
	}

	// 构建旧镜像的 map：containerType:containerName -> image
	oldImageMap := make(map[string]string)

	for _, c := range oldImages.InitContainers {
		oldImageMap["init:"+c.Name] = c.Image
	}
	for _, c := range oldImages.Containers {
		oldImageMap["main:"+c.Name] = c.Image
	}
	for _, c := range oldImages.EphemeralContainers {
		oldImageMap["ephemeral:"+c.Name] = c.Image
	}

	// 对比并收集变更
	var changes []ImageChangeDetail

	for _, newC := range newImages.Containers {
		key := string(newC.ContainerType) + ":" + newC.ContainerName
		oldImage, exists := oldImageMap[key]

		// 只有当旧镜像存在且与新镜像不同时才记录变更
		if exists && oldImage != newC.Image {
			changes = append(changes, ImageChangeDetail{
				ContainerType: string(newC.ContainerType),
				ContainerName: newC.ContainerName,
				OldImage:      oldImage,
				NewImage:      newC.Image,
			})
		}
	}

	return changes
}

// formatChangeDetails 格式化变更详情为可读字符串
func formatChangeDetails(changes []ImageChangeDetail) string {
	if len(changes) == 0 {
		return "无镜像变更"
	}

	var details []string
	for _, change := range changes {
		// 格式：[容器类型]容器名称: 旧镜像 -> 新镜像
		detail := fmt.Sprintf("[%s]%s: %s -> %s",
			containerTypeToChineseName(change.ContainerType),
			change.ContainerName,
			change.OldImage,
			change.NewImage,
		)
		details = append(details, detail)
	}

	return fmt.Sprintf("共变更 %d 个容器镜像: %s", len(changes), strings.Join(details, "; "))
}

// containerTypeToChineseName 将容器类型转换为中文名称
func containerTypeToChineseName(containerType string) string {
	switch containerType {
	case "init":
		return "初始化容器"
	case "main":
		return "主容器"
	case "ephemeral":
		return "临时容器"
	default:
		return containerType
	}
}
