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
	// 获取版本详情和资源控制器
	versionDetail, controller, err := getResourceController(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取资源控制器失败: %v", err)
		return "", err
	}

	// 构建更新镜像请求
	updateReq := &k8sTypes.UpdateImageRequest{
		Name:          versionDetail.ResourceName,
		Namespace:     versionDetail.Namespace,
		ContainerName: req.ContainerName,
		Image:         req.Image,
	}

	// 根据资源类型执行更新
	resourceType := strings.ToUpper(versionDetail.ResourceType)

	switch resourceType {
	case "DEPLOYMENT":
		err = controller.Deployment.UpdateImage(updateReq)
	case "STATEFULSET":
		err = controller.StatefulSet.UpdateImage(updateReq)
	case "DAEMONSET":
		err = controller.DaemonSet.UpdateImage(updateReq)
	default:
		return "", fmt.Errorf("资源类型 %s 不支持更新镜像操作", resourceType)
	}

	// 处理容器名称显示
	containerInfo := req.ContainerName
	if containerInfo == "" {
		containerInfo = "默认容器"
	}

	if err != nil {
		l.Errorf("更新镜像失败: %v", err)
		recordAuditLog(l.ctx, l.svcCtx, versionDetail, "镜像更新",
			fmt.Sprintf("%s %s/%s 更新镜像失败, 容器: %s, 目标镜像: %s, 错误: %v", resourceType, versionDetail.Namespace, versionDetail.ResourceName, containerInfo, req.Image, err), 2)
		return "", err
	}

	recordAuditLog(l.ctx, l.svcCtx, versionDetail, "镜像更新",
		fmt.Sprintf("%s %s/%s 更新镜像成功, 容器: %s, 新镜像: %s", resourceType, versionDetail.Namespace, versionDetail.ResourceName, containerInfo, req.Image), 1)

	l.Infof("成功更新镜像: %s/%s, 容器: %s, 新镜像: %s",
		versionDetail.Namespace, versionDetail.ResourceName, req.ContainerName, req.Image)
	return "更新镜像成功", nil
}
