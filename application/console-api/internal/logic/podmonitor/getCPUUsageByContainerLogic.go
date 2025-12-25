package podmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetCPUUsageByContainerLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取容器 CPU 使用情况
func NewGetCPUUsageByContainerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCPUUsageByContainerLogic {
	return &GetCPUUsageByContainerLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetCPUUsageByContainerLogic) GetCPUUsageByContainer(req *types.GetCPUUsageByContainerRequest) (resp *types.GetCPUUsageByContainerResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	pod := client.Pod()

	timeRange := parseRangeTime(req.Start, req.End, req.Step)

	metrics, err := pod.GetCPUUsageByContainer(req.Namespace, req.PodName, req.ContainerName, timeRange)
	if err != nil {
		l.Errorf("获取容器 CPU 使用失败: Namespace=%s, Pod=%s, Container=%s, Error=%v",
			req.Namespace, req.PodName, req.ContainerName, err)
		return nil, err
	}

	resp = &types.GetCPUUsageByContainerResponse{
		Data: types.ContainerCPUMetrics{
			Namespace:     metrics.Namespace,
			PodName:       metrics.PodName,
			ContainerName: metrics.ContainerName,
			Current: types.CPUUsageSnapshot{
				Timestamp:     metrics.Current.Timestamp.Unix(),
				UsageCores:    metrics.Current.UsageCores,
				UsagePercent:  metrics.Current.UsagePercent,
				RequestCores:  metrics.Current.RequestCores,
				LimitCores:    metrics.Current.LimitCores,
				ThrottledTime: metrics.Current.ThrottledTime,
			},
			Limits: types.CPULimits{
				RequestCores: metrics.Limits.RequestCores,
				LimitCores:   metrics.Limits.LimitCores,
				HasLimit:     metrics.Limits.HasLimit,
			},
			Summary: types.CPUSummary{
				AvgUsageCores:    metrics.Summary.AvgUsageCores,
				MaxUsageCores:    metrics.Summary.MaxUsageCores,
				MinUsageCores:    metrics.Summary.MinUsageCores,
				AvgUsagePercent:  metrics.Summary.AvgUsagePercent,
				MaxUsagePercent:  metrics.Summary.MaxUsagePercent,
				TotalThrottled:   metrics.Summary.TotalThrottled,
				ThrottledPercent: metrics.Summary.ThrottledPercent,
			},
		},
	}

	for _, point := range metrics.Trend {
		resp.Data.Trend = append(resp.Data.Trend, types.CPUUsageDataPoint{
			Timestamp:    point.Timestamp.Unix(),
			UsageCores:   point.UsageCores,
			UsagePercent: point.UsagePercent,
		})
	}

	l.Infof("获取容器 CPU 使用成功: Container=%s, Usage=%.2f cores", req.ContainerName, metrics.Current.UsageCores)
	return resp, nil
}
