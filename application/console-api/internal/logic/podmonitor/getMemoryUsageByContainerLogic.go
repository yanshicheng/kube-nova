package podmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetMemoryUsageByContainerLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取容器内存使用情况
func NewGetMemoryUsageByContainerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMemoryUsageByContainerLogic {
	return &GetMemoryUsageByContainerLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetMemoryUsageByContainerLogic) GetMemoryUsageByContainer(req *types.GetMemoryUsageByContainerRequest) (resp *types.GetMemoryUsageByContainerResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	pod := client.Pod()

	timeRange := parseRangeTime(req.Start, req.End, req.Step)

	metrics, err := pod.GetMemoryUsageByContainer(req.Namespace, req.PodName, req.ContainerName, timeRange)
	if err != nil {
		l.Errorf("获取容器内存使用失败: Container=%s, Error=%v", req.ContainerName, err)
		return nil, err
	}

	resp = &types.GetMemoryUsageByContainerResponse{
		Data: types.ContainerMemoryMetrics{
			Namespace:     metrics.Namespace,
			PodName:       metrics.PodName,
			ContainerName: metrics.ContainerName,
			Current: types.MemoryUsageSnapshot{
				Timestamp:       metrics.Current.Timestamp.Unix(),
				UsageBytes:      metrics.Current.UsageBytes,
				UsagePercent:    metrics.Current.UsagePercent,
				RequestBytes:    metrics.Current.RequestBytes,
				LimitBytes:      metrics.Current.LimitBytes,
				WorkingSetBytes: metrics.Current.WorkingSetBytes,
				RSSBytes:        metrics.Current.RSSBytes,
				CacheBytes:      metrics.Current.CacheBytes,
			},
			Limits: types.MemoryLimits{
				RequestBytes: metrics.Limits.RequestBytes,
				LimitBytes:   metrics.Limits.LimitBytes,
				HasLimit:     metrics.Limits.HasLimit,
			},
			Summary: types.MemorySummary{
				AvgUsageBytes:   metrics.Summary.AvgUsageBytes,
				MaxUsageBytes:   metrics.Summary.MaxUsageBytes,
				MinUsageBytes:   metrics.Summary.MinUsageBytes,
				AvgUsagePercent: metrics.Summary.AvgUsagePercent,
				MaxUsagePercent: metrics.Summary.MaxUsagePercent,
				OOMKills:        metrics.Summary.OOMKills,
			},
		},
	}

	for _, point := range metrics.Trend {
		resp.Data.Trend = append(resp.Data.Trend, types.MemoryUsageDataPoint{
			Timestamp:       point.Timestamp.Unix(),
			UsageBytes:      point.UsageBytes,
			UsagePercent:    point.UsagePercent,
			WorkingSetBytes: point.WorkingSetBytes,
		})
	}

	return resp, nil
}
