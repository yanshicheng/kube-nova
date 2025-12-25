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

func (l *UpdateImagesLogic) UpdateImages(req *types.UpdateImagesRequest) (resp string, err error) {
	versionDetail, controller, err := getResourceController(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取资源控制器失败: %v", err)
		return "", err
	}

	// 构建更新镜像请求
	updateReq := &k8sTypes.UpdateImagesRequest{
		Name:      versionDetail.ResourceName,
		Namespace: versionDetail.Namespace,
		Containers: k8sTypes.ContainerInfoList{
			InitContainers:      convertToK8sContainerInfo(req.ContainerInfoList.InitContainers),
			Containers:          convertToK8sContainerInfo(req.ContainerInfoList.Containers),
			EphemeralContainers: convertToK8sContainerInfo(req.ContainerInfoList.EphemeralContainers),
		},
	}

	resourceType := strings.ToUpper(versionDetail.ResourceType)

	switch resourceType {
	case "DEPLOYMENT":
		err = controller.Deployment.UpdateImages(updateReq)
	case "STATEFULSET":
		err = controller.StatefulSet.UpdateImages(updateReq)
	case "DAEMONSET":
		err = controller.DaemonSet.UpdateImages(updateReq)
	case "JOB":
		err = controller.Job.UpdateImages(updateReq)
	case "CRONJOB":
		err = controller.CronJob.UpdateImages(updateReq)
	default:
		return "", fmt.Errorf("资源类型 %s 不支持批量更新镜像操作", resourceType)
	}

	// 构建镜像更新详情
	initCount := len(req.ContainerInfoList.InitContainers)
	containerCount := len(req.ContainerInfoList.Containers)
	ephemeralCount := len(req.ContainerInfoList.EphemeralContainers)

	if err != nil {
		l.Errorf("批量更新镜像失败: %v", err)
		recordAuditLog(l.ctx, l.svcCtx, versionDetail, "批量镜像更新",
			fmt.Sprintf("%s %s/%s 批量更新镜像失败, Init容器: %d个, 主容器: %d个, 临时容器: %d个, 错误: %v", resourceType, versionDetail.Namespace, versionDetail.ResourceName, initCount, containerCount, ephemeralCount, err), 2)
		return "", err
	}

	recordAuditLog(l.ctx, l.svcCtx, versionDetail, "批量镜像更新",
		fmt.Sprintf("%s %s/%s 批量更新镜像成功, Init容器: %d个, 主容器: %d个, 临时容器: %d个", resourceType, versionDetail.Namespace, versionDetail.ResourceName, initCount, containerCount, ephemeralCount), 1)
	l.Infof("成功批量更新镜像: %s/%s", versionDetail.Namespace, versionDetail.ResourceName)
	return "批量更新镜像成功", nil
}

func convertToK8sContainerInfo(containers []types.ContainerInfo) []k8sTypes.ContainerInfo {
	result := make([]k8sTypes.ContainerInfo, 0, len(containers))
	for _, c := range containers {
		result = append(result, k8sTypes.ContainerInfo{
			Name:  c.Name,
			Image: c.Image,
		})
	}
	return result
}
