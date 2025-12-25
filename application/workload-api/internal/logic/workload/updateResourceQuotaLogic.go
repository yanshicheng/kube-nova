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

func (l *UpdateResourceQuotaLogic) UpdateResourceQuota(req *types.UpdateResourcesRequest) (resp string, err error) {
	versionDetail, controller, err := getResourceController(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取资源控制器失败: %v", err)
		return "", err
	}

	updateReq := &k8sTypes.UpdateResourcesRequest{
		Name:          versionDetail.ResourceName,
		Namespace:     versionDetail.Namespace,
		ContainerName: req.ContainerName,
		Resources: k8sTypes.ResourceRequirements{
			Limits: k8sTypes.ResourceList{
				Cpu:    req.Resources.Limits.Cpu,
				Memory: req.Resources.Limits.Memory,
			},
			Requests: k8sTypes.ResourceList{
				Cpu:    req.Resources.Requests.Cpu,
				Memory: req.Resources.Requests.Memory,
			},
		},
	}

	resourceType := strings.ToUpper(versionDetail.ResourceType)

	switch resourceType {
	case "DEPLOYMENT":
		err = controller.Deployment.UpdateResources(updateReq)
	case "STATEFULSET":
		err = controller.StatefulSet.UpdateResources(updateReq)
	case "DAEMONSET":
		err = controller.DaemonSet.UpdateResources(updateReq)
	case "JOB":
		err = controller.Job.UpdateResources(updateReq)
	case "CRONJOB":
		err = controller.CronJob.UpdateResources(updateReq)
	default:
		return "", fmt.Errorf("资源类型 %s 不支持修改资源配额", resourceType)
	}

	// 构建资源配额详情
	quotaDetail := fmt.Sprintf("Limits(CPU: %s, Memory: %s), Requests(CPU: %s, Memory: %s)",
		req.Resources.Limits.Cpu, req.Resources.Limits.Memory,
		req.Resources.Requests.Cpu, req.Resources.Requests.Memory)

	if err != nil {
		l.Errorf("修改资源配额失败: %v", err)
		recordAuditLog(l.ctx, l.svcCtx, versionDetail, "修改资源配额",
			fmt.Sprintf("%s %s/%s 容器 %s 修改资源配额失败, 目标配置: %s, 错误: %v", resourceType, versionDetail.Namespace, versionDetail.ResourceName, req.ContainerName, quotaDetail, err), 2)
		return "", fmt.Errorf("修改资源配额失败")
	}

	recordAuditLog(l.ctx, l.svcCtx, versionDetail, "修改资源配额",
		fmt.Sprintf("%s %s/%s 容器 %s 修改资源配额成功, 新配置: %s", resourceType, versionDetail.Namespace, versionDetail.ResourceName, req.ContainerName, quotaDetail), 1)
	return "修改资源配额成功", nil
}
