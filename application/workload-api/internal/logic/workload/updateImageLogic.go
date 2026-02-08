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

type UpdateImageLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新资源镜像
func NewUpdateImageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateImageLogic {
	return &UpdateImageLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateImageLogic) UpdateImage(req *types.UpdateImageRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}

	// 获取版本详情和资源控制器
	versionDetail, controller, err := getResourceController(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取资源控制器失败: %v", err)
		return "", err
	}

	resourceType := strings.ToUpper(versionDetail.ResourceType)

	// 获取原镜像配置
	var oldImages *k8sTypes.ContainerInfoList
	switch resourceType {
	case "DEPLOYMENT":
		oldImages, err = controller.Deployment.GetContainerImages(versionDetail.Namespace, versionDetail.ResourceName)
	case "STATEFULSET":
		oldImages, err = controller.StatefulSet.GetContainerImages(versionDetail.Namespace, versionDetail.ResourceName)
	case "DAEMONSET":
		oldImages, err = controller.DaemonSet.GetContainerImages(versionDetail.Namespace, versionDetail.ResourceName)
	default:
		return "", fmt.Errorf("资源类型 %s 不支持更新镜像操作", resourceType)
	}

	// 查找原镜像
	oldImage := l.findContainerImage(oldImages, req.ContainerName)

	if req.Reason == "" {
		req.Reason = fmt.Sprintf("更新镜像: %s/%s, 容器: %s, 新镜像: %s, 操作人: %s",
			versionDetail.Namespace, versionDetail.ResourceName, req.ContainerName, req.Image, username)
	} else {
		req.Reason = fmt.Sprintf("%s, 操作人: %s", req.Reason, username)
	}

	// 构建更新镜像请求
	updateReq := &k8sTypes.UpdateImageRequest{
		Name:          versionDetail.ResourceName,
		Namespace:     versionDetail.Namespace,
		ContainerName: req.ContainerName,
		Image:         req.Image,
		Reason:        req.Reason,
	}

	// 执行更新
	switch resourceType {
	case "DEPLOYMENT":
		err = controller.Deployment.UpdateImage(updateReq)
	case "STATEFULSET":
		err = controller.StatefulSet.UpdateImage(updateReq)
	case "DAEMONSET":
		err = controller.DaemonSet.UpdateImage(updateReq)
	}

	// 处理容器名称显示
	containerInfo := req.ContainerName
	if containerInfo == "" {
		containerInfo = "默认容器"
	}

	// 生成变更详情
	changeDetail := CompareSingleImage(containerInfo, oldImage, req.Image)

	if err != nil {
		l.Errorf("更新镜像失败: %v", err)
		recordAuditLog(l.ctx, l.svcCtx, versionDetail, "镜像更新",
			fmt.Sprintf("%s %s/%s 更新镜像失败, %s, 错误: %v", resourceType, versionDetail.Namespace, versionDetail.ResourceName, changeDetail, err), 2)
		return "", err
	}

	recordAuditLog(l.ctx, l.svcCtx, versionDetail, "镜像更新",
		fmt.Sprintf("%s %s/%s 更新镜像成功, %s", resourceType, versionDetail.Namespace, versionDetail.ResourceName, changeDetail), 1)

	l.Infof("成功更新镜像: %s/%s, %s", versionDetail.Namespace, versionDetail.ResourceName, changeDetail)
	return "更新镜像成功", nil
}

// findContainerImage 查找指定容器的镜像
func (l *UpdateImageLogic) findContainerImage(images *k8sTypes.ContainerInfoList, containerName string) string {
	if images == nil {
		return "未知"
	}

	// 如果未指定容器名，返回第一个容器的镜像
	if containerName == "" {
		if len(images.Containers) > 0 {
			return images.Containers[0].Image
		}
		return "未知"
	}

	// 在主容器中查找
	for _, c := range images.Containers {
		if c.Name == containerName {
			return c.Image
		}
	}

	// 在初始化容器中查找
	for _, c := range images.InitContainers {
		if c.Name == containerName {
			return c.Image
		}
	}

	// 在临时容器中查找
	for _, c := range images.EphemeralContainers {
		if c.Name == containerName {
			return c.Image
		}
	}

	return "未知"
}
