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

type GetEnvVarsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetEnvVarsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetEnvVarsLogic {
	return &GetEnvVarsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetEnvVarsLogic) GetEnvVars(req *types.DefaultIdRequest) (resp *types.EnvVarsResponse, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	resourceType := strings.ToUpper(versionDetail.ResourceType)

	var envVars *k8sTypes.EnvVarsResponse

	switch resourceType {
	case "DEPLOYMENT":
		envVars, err = client.Deployment().GetEnvVars(versionDetail.Namespace, versionDetail.ResourceName)
	case "STATEFULSET":
		envVars, err = client.StatefulSet().GetEnvVars(versionDetail.Namespace, versionDetail.ResourceName)
	case "DAEMONSET":
		envVars, err = client.DaemonSet().GetEnvVars(versionDetail.Namespace, versionDetail.ResourceName)
	case "JOB":
		envVars, err = client.Job().GetEnvVars(versionDetail.Namespace, versionDetail.ResourceName)
	case "CRONJOB":
		envVars, err = client.CronJob().GetEnvVars(versionDetail.Namespace, versionDetail.ResourceName)
	default:
		return nil, fmt.Errorf("资源类型 %s 不支持查询环境变量", resourceType)
	}

	if err != nil {
		l.Errorf("获取环境变量失败: %v", err)
		return nil, fmt.Errorf("获取环境变量失败")
	}

	resp = convertToEnvVarsResponse(envVars)
	return resp, nil
}

// convertToEnvVarsResponse 将 k8sTypes.EnvVarsResponse 转换为 API 响应类型
func convertToEnvVarsResponse(envVars *k8sTypes.EnvVarsResponse) *types.EnvVarsResponse {
	containers := make([]types.ContainerEnvVars, 0, len(envVars.Containers))

	for _, container := range envVars.Containers {
		env := make([]types.EnvVar, 0, len(container.Env))
		for _, e := range container.Env {
			env = append(env, types.EnvVar{
				Name:   e.Name,
				Source: convertToEnvVarSource(e.Source),
			})
		}

		containers = append(containers, types.ContainerEnvVars{
			ContainerName: container.ContainerName,
			ContainerType: string(container.ContainerType), // 添加容器类型转换
			Env:           env,
		})
	}

	return &types.EnvVarsResponse{
		Containers: containers,
	}
}

// convertToEnvVarSource 将 k8sTypes.EnvVarSource 转换为 API 类型
func convertToEnvVarSource(source k8sTypes.EnvVarSource) types.EnvVarSource {
	result := types.EnvVarSource{
		Type:  source.Type,
		Value: source.Value,
	}

	if source.ConfigMapKeyRef != nil {
		result.ConfigMapKeyRef = &types.ConfigMapKeySelector{
			Name:     source.ConfigMapKeyRef.Name,
			Key:      source.ConfigMapKeyRef.Key,
			Optional: source.ConfigMapKeyRef.Optional,
		}
	}

	if source.SecretKeyRef != nil {
		result.SecretKeyRef = &types.SecretKeySelector{
			Name:     source.SecretKeyRef.Name,
			Key:      source.SecretKeyRef.Key,
			Optional: source.SecretKeyRef.Optional,
		}
	}

	if source.FieldRef != nil {
		result.FieldRef = &types.ObjectFieldSelector{
			FieldPath: source.FieldRef.FieldPath,
		}
	}

	if source.ResourceFieldRef != nil {
		result.ResourceFieldRef = &types.ResourceFieldSelector{
			ContainerName: source.ResourceFieldRef.ContainerName,
			Resource:      source.ResourceFieldRef.Resource,
			Divisor:       source.ResourceFieldRef.Divisor,
		}
	}

	return result
}
