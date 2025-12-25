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

type UpdateProbesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateProbesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateProbesLogic {
	return &UpdateProbesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateProbesLogic) UpdateProbes(req *types.UpdateProbesRequest) (resp string, err error) {
	versionDetail, controller, err := getResourceController(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取资源控制器失败: %v", err)
		return "", err
	}

	updateReq := &k8sTypes.UpdateProbesRequest{
		Name:          versionDetail.ResourceName,
		Namespace:     versionDetail.Namespace,
		ContainerName: req.ContainerName,
	}

	if req.LivenessProbe != nil {
		updateReq.LivenessProbe = convertToK8sProbe(req.LivenessProbe)
	}

	if req.ReadinessProbe != nil {
		updateReq.ReadinessProbe = convertToK8sProbe(req.ReadinessProbe)
	}

	if req.StartupProbe != nil {
		updateReq.StartupProbe = convertToK8sProbe(req.StartupProbe)
	}

	resourceType := strings.ToUpper(versionDetail.ResourceType)

	switch resourceType {
	case "DEPLOYMENT":
		err = controller.Deployment.UpdateProbes(updateReq)
	case "STATEFULSET":
		err = controller.StatefulSet.UpdateProbes(updateReq)
	case "DAEMONSET":
		err = controller.DaemonSet.UpdateProbes(updateReq)
	case "JOB":
		err = controller.Job.UpdateProbes(updateReq)
	case "CRONJOB":
		err = controller.CronJob.UpdateProbes(updateReq)
	default:
		return "", fmt.Errorf("资源类型 %s 不支持修改健康检查", resourceType)
	}

	// 构建健康检查配置详情
	probeDetail := ""
	if req.LivenessProbe != nil {
		probeDetail += fmt.Sprintf("存活探针类型: %s", req.LivenessProbe.Type)
	}
	if req.ReadinessProbe != nil {
		if probeDetail != "" {
			probeDetail += ", "
		}
		probeDetail += fmt.Sprintf("就绪探针类型: %s", req.ReadinessProbe.Type)
	}
	if req.StartupProbe != nil {
		if probeDetail != "" {
			probeDetail += ", "
		}
		probeDetail += fmt.Sprintf("启动探针类型: %s", req.StartupProbe.Type)
	}

	if err != nil {
		l.Errorf("修改健康检查失败: %v", err)
		recordAuditLog(l.ctx, l.svcCtx, versionDetail, "修改健康检查",
			fmt.Sprintf("%s %s/%s 容器 %s 修改健康检查失败, %s, 错误: %v", resourceType, versionDetail.Namespace, versionDetail.ResourceName, req.ContainerName, probeDetail, err), 2)
		return "", fmt.Errorf("修改健康检查失败")
	}

	recordAuditLog(l.ctx, l.svcCtx, versionDetail, "修改健康检查",
		fmt.Sprintf("%s %s/%s 容器 %s 修改健康检查成功, %s", resourceType, versionDetail.Namespace, versionDetail.ResourceName, req.ContainerName, probeDetail), 1)
	return "修改健康检查成功", nil
}

func convertToK8sProbe(probe *types.Probe) *k8sTypes.Probe {
	result := &k8sTypes.Probe{
		Type:                probe.Type,
		InitialDelaySeconds: probe.InitialDelaySeconds,
		TimeoutSeconds:      probe.TimeoutSeconds,
		PeriodSeconds:       probe.PeriodSeconds,
		SuccessThreshold:    probe.SuccessThreshold,
		FailureThreshold:    probe.FailureThreshold,
	}

	if probe.HttpGet != nil {
		headers := make([]k8sTypes.HTTPHeader, 0, len(probe.HttpGet.HttpHeaders))
		for _, h := range probe.HttpGet.HttpHeaders {
			headers = append(headers, k8sTypes.HTTPHeader{
				Name:  h.Name,
				Value: h.Value,
			})
		}

		result.HttpGet = &k8sTypes.HTTPGetAction{
			Path:        probe.HttpGet.Path,
			Port:        probe.HttpGet.Port,
			Host:        probe.HttpGet.Host,
			Scheme:      probe.HttpGet.Scheme,
			HttpHeaders: headers,
		}
	}

	if probe.TcpSocket != nil {
		result.TcpSocket = &k8sTypes.TCPSocketAction{
			Port: probe.TcpSocket.Port,
			Host: probe.TcpSocket.Host,
		}
	}

	if probe.Exec != nil {
		result.Exec = &k8sTypes.ExecAction{
			Command: probe.Exec.Command,
		}
	}

	if probe.Grpc != nil {
		result.Grpc = &k8sTypes.GRPCAction{
			Port:    probe.Grpc.Port,
			Service: probe.Grpc.Service,
		}
	}

	return result
}
