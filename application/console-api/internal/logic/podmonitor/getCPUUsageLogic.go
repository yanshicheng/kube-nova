package podmonitor

import (
	"context"
	"fmt"
	"time"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetCPUUsageLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetCPUUsageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCPUUsageLogic {
	return &GetCPUUsageLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// parseTime 解析时间字符串
func (l *GetCPUUsageLogic) parseTime(timeStr string, fieldName string) (time.Time, error) {
	timeFormats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999Z07:00",
		"2006-01-02T15:04:05.999Z",
		"2006-01-02T15:04:05Z",
	}

	for _, format := range timeFormats {
		t, err := time.Parse(format, timeStr)
		if err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("解析%s时间失败: %s", fieldName, timeStr)
}

func (l *GetCPUUsageLogic) GetCPUUsage(req *types.GetCPUUsageRequest) (resp *types.GetCPUUsageResponse, err error) {

	// 1. 获取 Prometheus 客户端
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	// 2. 构建时间范围

	timeRange := parseRangeTime(req.Start, req.End, req.Step)

	// 3. 查询 CPU 数据
	pod := client.Pod()
	metrics, err := pod.GetCPUUsage(req.Namespace, req.PodName, timeRange)
	if err != nil {
		l.Errorf("查询 CPU 失败: %v", err)
		return nil, err
	}

	l.Infof("查询结果: current=%.4f cores (%.2f%%), trend=%d 数据点",
		metrics.Current.UsageCores, metrics.Current.UsagePercent, len(metrics.Trend))

	// 4. 转换响应格式
	resp = &types.GetCPUUsageResponse{
		Data: types.PodCPUMetrics{
			Namespace: metrics.Namespace,
			PodName:   metrics.PodName,
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

	// 5. 转换趋势数据
	if len(metrics.Trend) > 0 {
		resp.Data.Trend = make([]types.CPUUsageDataPoint, len(metrics.Trend))
		for i, point := range metrics.Trend {
			resp.Data.Trend[i] = types.CPUUsageDataPoint{
				Timestamp:    point.Timestamp.Unix(),
				UsageCores:   point.UsageCores,
				UsagePercent: point.UsagePercent,
			}
		}
	} else {
		resp.Data.Trend = []types.CPUUsageDataPoint{}
	}

	return resp, nil
}
