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

type UpdateEnvVarsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateEnvVarsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateEnvVarsLogic {
	return &UpdateEnvVarsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateEnvVarsLogic) UpdateEnvVars(req *types.UpdateEnvVarsRequest) (resp string, err error) {
	versionDetail, controller, err := getResourceController(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取资源控制器失败: %v", err)
		return "", err
	}

	env := make([]k8sTypes.EnvVar, 0, len(req.Env))
	for _, e := range req.Env {
		env = append(env, k8sTypes.EnvVar{
			Name:   e.Name,
			Source: convertToK8sEnvVarSource(e.Source),
		})
	}

	updateReq := &k8sTypes.UpdateEnvVarsRequest{
		Name:          versionDetail.ResourceName,
		Namespace:     versionDetail.Namespace,
		ContainerName: req.ContainerName,
		Env:           env,
	}

	resourceType := strings.ToUpper(versionDetail.ResourceType)

	switch resourceType {
	case "DEPLOYMENT":
		err = controller.Deployment.UpdateEnvVars(updateReq)
	case "STATEFULSET":
		err = controller.StatefulSet.UpdateEnvVars(updateReq)
	case "DAEMONSET":
		err = controller.DaemonSet.UpdateEnvVars(updateReq)
	case "JOB":
		err = controller.Job.UpdateEnvVars(updateReq)
	case "CRONJOB":
		err = controller.CronJob.UpdateEnvVars(updateReq)
	default:
		return "", fmt.Errorf("资源类型 %s 不支持修改环境变量", resourceType)
	}

	envCount := len(req.Env)

	if err != nil {
		l.Errorf("修改环境变量失败: %v", err)
		recordAuditLog(l.ctx, l.svcCtx, versionDetail, "修改环境变量",
			fmt.Sprintf("%s %s/%s 容器 %s 修改环境变量失败, 环境变量数量: %d, 错误: %v", resourceType, versionDetail.Namespace, versionDetail.ResourceName, req.ContainerName, envCount, err), 2)
		return "", fmt.Errorf("修改环境变量失败")
	}

	recordAuditLog(l.ctx, l.svcCtx, versionDetail, "修改环境变量",
		fmt.Sprintf("%s %s/%s 容器 %s 修改环境变量成功, 环境变量数量: %d", resourceType, versionDetail.Namespace, versionDetail.ResourceName, req.ContainerName, envCount), 1)
	return "修改环境变量成功", nil
}

func convertToK8sEnvVarSource(source types.EnvVarSource) k8sTypes.EnvVarSource {
	result := k8sTypes.EnvVarSource{
		Type:  source.Type,
		Value: source.Value,
	}

	if source.ConfigMapKeyRef != nil {
		result.ConfigMapKeyRef = &k8sTypes.ConfigMapKeySelector{
			Name:     source.ConfigMapKeyRef.Name,
			Key:      source.ConfigMapKeyRef.Key,
			Optional: source.ConfigMapKeyRef.Optional,
		}
	}

	if source.SecretKeyRef != nil {
		result.SecretKeyRef = &k8sTypes.SecretKeySelector{
			Name:     source.SecretKeyRef.Name,
			Key:      source.SecretKeyRef.Key,
			Optional: source.SecretKeyRef.Optional,
		}
	}

	if source.FieldRef != nil {
		result.FieldRef = &k8sTypes.ObjectFieldSelector{
			FieldPath: source.FieldRef.FieldPath,
		}
	}

	if source.ResourceFieldRef != nil {
		result.ResourceFieldRef = &k8sTypes.ResourceFieldSelector{
			ContainerName: source.ResourceFieldRef.ContainerName,
			Resource:      source.ResourceFieldRef.Resource,
			Divisor:       source.ResourceFieldRef.Divisor,
		}
	}

	return result
}
