package podmonitor

import (
	"context"
	"time"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetProbeStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取探针状态
func NewGetProbeStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProbeStatusLogic {
	return &GetProbeStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetProbeStatusLogic) GetProbeStatus(req *types.GetProbeStatusRequest) (resp *types.GetProbeStatusResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	pod := client.Pod()

	metrics, err := pod.GetProbeStatus(req.Namespace, req.PodName)
	if err != nil {
		l.Errorf("获取探针状态失败: Namespace=%s, Pod=%s, Error=%v",
			req.Namespace, req.PodName, err)
		return nil, err
	}

	resp = &types.GetProbeStatusResponse{
		Data: types.PodProbeMetrics{
			Namespace:       metrics.Namespace,
			PodName:         metrics.PodName,
			LivenessProbes:  make([]types.ProbeStatus, 0),
			ReadinessProbes: make([]types.ProbeStatus, 0),
			StartupProbes:   make([]types.ProbeStatus, 0),
		},
	}

	// 转换 Liveness 探针
	for _, probe := range metrics.LivenessProbes {
		ps := types.ProbeStatus{
			ContainerName: probe.ContainerName,
			ProbeType:     probe.ProbeType,
			Status:        probe.Status,
			SuccessCount:  probe.SuccessCount,
			FailureCount:  probe.FailureCount,
			TotalProbes:   probe.TotalProbes,
			LastProbeTime: probe.LastProbeTime.Format(time.RFC3339),
			FailureReason: probe.FailureReason,
		}
		if probe.LastSuccessTime != nil {
			successTime := probe.LastSuccessTime.Format(time.RFC3339)
			ps.LastSuccessTime = successTime
		}
		if probe.LastFailureTime != nil {
			failureTime := probe.LastFailureTime.Format(time.RFC3339)
			ps.LastFailureTime = failureTime
		}
		resp.Data.LivenessProbes = append(resp.Data.LivenessProbes, ps)
	}

	// 转换 Readiness 探针
	for _, probe := range metrics.ReadinessProbes {
		ps := types.ProbeStatus{
			ContainerName: probe.ContainerName,
			ProbeType:     probe.ProbeType,
			Status:        probe.Status,
			SuccessCount:  probe.SuccessCount,
			FailureCount:  probe.FailureCount,
			TotalProbes:   probe.TotalProbes,
			LastProbeTime: probe.LastProbeTime.Format(time.RFC3339),
			FailureReason: probe.FailureReason,
		}
		if probe.LastSuccessTime != nil {
			successTime := probe.LastSuccessTime.Format(time.RFC3339)
			ps.LastSuccessTime = successTime
		}
		if probe.LastFailureTime != nil {
			failureTime := probe.LastFailureTime.Format(time.RFC3339)
			ps.LastFailureTime = failureTime
		}
		resp.Data.ReadinessProbes = append(resp.Data.ReadinessProbes, ps)
	}

	// 转换 Startup 探针
	for _, probe := range metrics.StartupProbes {
		ps := types.ProbeStatus{
			ContainerName: probe.ContainerName,
			ProbeType:     probe.ProbeType,
			Status:        probe.Status,
			SuccessCount:  probe.SuccessCount,
			FailureCount:  probe.FailureCount,
			TotalProbes:   probe.TotalProbes,
			LastProbeTime: probe.LastProbeTime.Format(time.RFC3339),
			FailureReason: probe.FailureReason,
		}
		if probe.LastSuccessTime != nil {
			successTime := probe.LastSuccessTime.Format(time.RFC3339)
			ps.LastSuccessTime = successTime
		}
		if probe.LastFailureTime != nil {
			failureTime := probe.LastFailureTime.Format(time.RFC3339)
			ps.LastFailureTime = failureTime
		}
		resp.Data.StartupProbes = append(resp.Data.StartupProbes, ps)
	}

	l.Infof("获取探针状态成功: Pod=%s", req.PodName)
	return resp, nil
}
