package podmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetMemoryUsageLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Pod 内存使用情况
func NewGetMemoryUsageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMemoryUsageLogic {
	return &GetMemoryUsageLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetMemoryUsageLogic) GetMemoryUsage(req *types.GetMemoryUsageRequest) (resp *types.GetMemoryUsageResponse, err error) {
	// 1. 获取 Prometheus 客户端
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	// 2. 获取 Pod 操作器
	pod := client.Pod()

	// 3. 构建时间范围
	timeRange := parseRangeTime(req.Start, req.End, req.Step)

	// 4. 调用 Pod 操作器获取内存使用情况
	metrics, err := pod.GetMemoryUsage(req.Namespace, req.PodName, timeRange)
	if err != nil {
		l.Errorf("获取内存使用情况失败: Namespace=%s, Pod=%s, Error=%v",
			req.Namespace, req.PodName, err)
		return nil, err
	}

	// 5. 转换为 API 响应格式
	resp = &types.GetMemoryUsageResponse{
		Data: types.PodMemoryMetrics{
			Namespace: metrics.Namespace,
			PodName:   metrics.PodName,
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

	// 转换趋势数据
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
