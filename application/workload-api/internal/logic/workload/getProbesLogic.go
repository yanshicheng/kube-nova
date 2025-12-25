package workload

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	types2 "github.com/yanshicheng/kube-nova/common/k8smanager/types"

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

func (l *GetProbesLogic) GetProbes(req *types.DefaultIdRequest) (resp *types.ProbesResponse, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	resourceType := strings.ToUpper(versionDetail.ResourceType)

	var probes *types2.ProbesResponse

	switch resourceType {
	case "DEPLOYMENT":
		probes, err = client.Deployment().GetProbes(versionDetail.Namespace, versionDetail.ResourceName)
	case "STATEFULSET":
		probes, err = client.StatefulSet().GetProbes(versionDetail.Namespace, versionDetail.ResourceName)
	case "DAEMONSET":
		probes, err = client.DaemonSet().GetProbes(versionDetail.Namespace, versionDetail.ResourceName)
	case "JOB":
		probes, err = client.Job().GetProbes(versionDetail.Namespace, versionDetail.ResourceName)
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

func convertToProbesResponse(probes *types2.ProbesResponse) *types.ProbesResponse {
	containers := make([]types.ContainerProbes, 0, len(probes.Containers))

	for _, container := range probes.Containers {
		containerProbe := types.ContainerProbes{
			ContainerName: container.ContainerName,
		}

		if container.LivenessProbe != nil {
			containerProbe.LivenessProbe = convertToProbe(container.LivenessProbe)
		}

		if container.ReadinessProbe != nil {
			containerProbe.ReadinessProbe = convertToProbe(container.ReadinessProbe)
		}

		if container.StartupProbe != nil {
			containerProbe.StartupProbe = convertToProbe(container.StartupProbe)
		}

		containers = append(containers, containerProbe)
	}

	return &types.ProbesResponse{
		Containers: containers,
	}
}

func convertToProbe(probe *types2.Probe) *types.Probe {
	result := &types.Probe{
		Type:                probe.Type,
		InitialDelaySeconds: probe.InitialDelaySeconds,
		TimeoutSeconds:      probe.TimeoutSeconds,
		PeriodSeconds:       probe.PeriodSeconds,
		SuccessThreshold:    probe.SuccessThreshold,
		FailureThreshold:    probe.FailureThreshold,
	}

	if probe.HttpGet != nil {
		headers := make([]types.HTTPHeader, 0, len(probe.HttpGet.HttpHeaders))
		for _, h := range probe.HttpGet.HttpHeaders {
			headers = append(headers, types.HTTPHeader{
				Name:  h.Name,
				Value: h.Value,
			})
		}

		result.HttpGet = &types.HTTPGetAction{
			Path:        probe.HttpGet.Path,
			Port:        probe.HttpGet.Port,
			Host:        probe.HttpGet.Host,
			Scheme:      probe.HttpGet.Scheme,
			HttpHeaders: headers,
		}
	}

	if probe.TcpSocket != nil {
		result.TcpSocket = &types.TCPSocketAction{
			Port: probe.TcpSocket.Port,
			Host: probe.TcpSocket.Host,
		}
	}

	if probe.Exec != nil {
		result.Exec = &types.ExecAction{
			Command: probe.Exec.Command,
		}
	}

	if probe.Grpc != nil {
		result.Grpc = &types.GRPCAction{
			Port:    probe.Grpc.Port,
			Service: probe.Grpc.Service,
		}
	}

	return result
}
