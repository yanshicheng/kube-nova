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

type GetProbesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetProbesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProbesLogic {
	return &GetProbesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetProbesLogic) GetProbes(req *types.DefaultIdRequest) (resp *types.CommProbesResponse, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	resourceType := strings.ToUpper(versionDetail.ResourceType)

	var probes *k8sTypes.ProbesResponse

	switch resourceType {
	case "DEPLOYMENT":
		probes, err = client.Deployment().GetProbes(versionDetail.Namespace, versionDetail.ResourceName)
	case "STATEFULSET":
		probes, err = client.StatefulSet().GetProbes(versionDetail.Namespace, versionDetail.ResourceName)
	case "DAEMONSET":
		probes, err = client.DaemonSet().GetProbes(versionDetail.Namespace, versionDetail.ResourceName)
	case "CRONJOB":
		probes, err = client.CronJob().GetProbes(versionDetail.Namespace, versionDetail.ResourceName)
	default:
		return nil, fmt.Errorf("资源类型 %s 不支持查询健康检查", resourceType)
	}

	if err != nil {
		l.Errorf("获取健康检查配置失败: %v", err)
		return nil, fmt.Errorf("获取健康检查配置失败")
	}

	resp = convertToProbesResponse(probes)
	return resp, nil
}

// convertToProbesResponse 将 k8sTypes.ProbesResponse 转换为 API 响应类型
func convertToProbesResponse(probes *k8sTypes.ProbesResponse) *types.CommProbesResponse {
	containers := make([]types.CommContainerProbes, 0, len(probes.Containers))

	for _, container := range probes.Containers {
		containerProbe := types.CommContainerProbes{
			ContainerName: container.ContainerName,
			ContainerType: string(container.ContainerType), // 添加容器类型转换
		}

		if container.LivenessProbe != nil {
			containerProbe.LivenessProbe = convertToCommProbe(container.LivenessProbe)
		}

		if container.ReadinessProbe != nil {
			containerProbe.ReadinessProbe = convertToCommProbe(container.ReadinessProbe)
		}

		if container.StartupProbe != nil {
			containerProbe.StartupProbe = convertToCommProbe(container.StartupProbe)
		}

		containers = append(containers, containerProbe)
	}

	return &types.CommProbesResponse{
		Containers: containers,
	}
}

// convertToCommProbe 将 k8sTypes.Probe 转换为 API 类型
func convertToCommProbe(probe *k8sTypes.Probe) *types.CommProbe {
	result := &types.CommProbe{
		Type:                probe.Type,
		InitialDelaySeconds: probe.InitialDelaySeconds,
		TimeoutSeconds:      probe.TimeoutSeconds,
		PeriodSeconds:       probe.PeriodSeconds,
		SuccessThreshold:    probe.SuccessThreshold,
		FailureThreshold:    probe.FailureThreshold,
	}

	if probe.HttpGet != nil {
		headers := make([]types.CommHTTPHeader, 0, len(probe.HttpGet.HTTPHeaders))
		for _, h := range probe.HttpGet.HTTPHeaders {
			headers = append(headers, types.CommHTTPHeader{
				Name:  h.Name,
				Value: h.Value,
			})
		}

		result.HttpGet = &types.CommHTTPGetAction{
			Path:        probe.HttpGet.Path,
			Port:        probe.HttpGet.Port,
			Host:        probe.HttpGet.Host,
			Scheme:      probe.HttpGet.Scheme,
			HTTPHeaders: headers,
		}
	}

	if probe.TcpSocket != nil {
		result.TcpSocket = &types.CommTCPSocketAction{
			Port: probe.TcpSocket.Port,
			Host: probe.TcpSocket.Host,
		}
	}

	if probe.Exec != nil {
		result.Exec = &types.CommExecAction{
			Command: probe.Exec.Command,
		}
	}

	if probe.Grpc != nil {
		result.Grpc = &types.CommGRPCAction{
			Port:    probe.Grpc.Port,
			Service: probe.Grpc.Service,
		}
	}

	return result
}
