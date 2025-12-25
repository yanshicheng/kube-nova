package podmonitor

import (
	"context"
	"time"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	types2 "github.com/yanshicheng/kube-nova/common/prometheusmanager/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetCPUThrottlingLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 CPU 节流情况
func NewGetCPUThrottlingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCPUThrottlingLogic {
	return &GetCPUThrottlingLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetCPUThrottlingLogic) GetCPUThrottling(req *types.GetCPUThrottlingRequest) (resp *types.GetCPUThrottlingResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	pod := client.Pod()

	timeRange := parseRangeTime(req.Start, req.End, req.Step)

	metrics, err := pod.GetCPUThrottling(req.Namespace, req.PodName, timeRange)
	if err != nil {
		l.Errorf("获取 CPU 节流失败: Namespace=%s, Pod=%s, Error=%v", req.Namespace, req.PodName, err)
		return nil, err
	}

	resp = &types.GetCPUThrottlingResponse{
		Data: types.CPUThrottlingMetrics{
			Namespace:        metrics.Namespace,
			PodName:          metrics.PodName,
			TotalThrottled:   metrics.TotalThrottled,
			ThrottledPercent: metrics.ThrottledPercent,
		},
	}

	for _, point := range metrics.Trend {
		resp.Data.Trend = append(resp.Data.Trend, types.CPUThrottlingDataPoint{
			Timestamp:        point.Timestamp.Unix(),
			ThrottledSeconds: point.ThrottledSeconds,
			ThrottledPercent: point.ThrottledPercent,
		})
	}

	for _, container := range metrics.ByContainer {
		resp.Data.ByContainer = append(resp.Data.ByContainer, types.ContainerThrottling{
			ContainerName:    container.ContainerName,
			ThrottledSeconds: container.ThrottledSeconds,
			ThrottledPercent: container.ThrottledPercent,
		})
	}

	l.Infof("获取 CPU 节流成功: Pod=%s, Throttled=%.2f%%", req.PodName, metrics.ThrottledPercent)
	return resp, nil
}

func parseRangeTime(startTime, endTime, step string) *types2.TimeRange {
	timeTange := &types2.TimeRange{}
	if startTime == "" {
		timeTange.Start = time.Now().Add(-time.Hour)
	} else {
		start, err := time.Parse(time.RFC3339, startTime)
		if err != nil {
			timeTange.Start = time.Now().Add(-time.Hour)
		}
		timeTange.Start = start
	}
	if endTime == "" {
		timeTange.End = time.Now()
	} else {
		end, err := time.Parse(time.RFC3339, endTime)
		if err != nil {
			timeTange.End = time.Now()
		}
		timeTange.End = end
	}
	if step == "" {
		timeTange.Step = "1m"
	} else {
		timeTange.Step = step
	}
	return timeTange
}
