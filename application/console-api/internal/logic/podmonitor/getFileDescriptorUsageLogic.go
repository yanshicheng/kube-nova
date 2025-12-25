package podmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetFileDescriptorUsageLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取文件描述符使用情况
func NewGetFileDescriptorUsageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetFileDescriptorUsageLogic {
	return &GetFileDescriptorUsageLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetFileDescriptorUsageLogic) GetFileDescriptorUsage(req *types.GetFileDescriptorUsageRequest) (resp *types.GetFileDescriptorUsageResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	pod := client.Pod()

	timeRange := parseRangeTime(req.Start, req.End, req.Step)

	metrics, err := pod.GetFileDescriptorUsage(req.Namespace, req.PodName, timeRange)
	if err != nil {
		l.Errorf("获取文件描述符使用情况失败: Error=%v", err)
		return nil, err
	}

	resp = &types.GetFileDescriptorUsageResponse{
		Data: types.FileDescriptorMetrics{
			Namespace: metrics.Namespace,
			PodName:   metrics.PodName,
			Current: types.FDSnapshot{
				Timestamp:    metrics.Current.Timestamp.Unix(),
				OpenFDs:      metrics.Current.OpenFDs,
				MaxFDs:       metrics.Current.MaxFDs,
				UsagePercent: metrics.Current.UsagePercent,
			},
			Summary: types.FDSummary{
				AvgOpenFDs:      metrics.Summary.AvgOpenFDs,
				MaxOpenFDs:      metrics.Summary.MaxOpenFDs,
				AvgUsagePercent: metrics.Summary.AvgUsagePercent,
				MaxUsagePercent: metrics.Summary.MaxUsagePercent,
			},
		},
	}

	for _, point := range metrics.Trend {
		resp.Data.Trend = append(resp.Data.Trend, types.FDDataPoint{
			Timestamp:    point.Timestamp.Unix(),
			OpenFDs:      point.OpenFDs,
			UsagePercent: point.UsagePercent,
		})
	}

	l.Infof("获取文件描述符使用情况成功: OpenFDs=%d, Usage=%.2f%%",
		metrics.Current.OpenFDs, metrics.Current.UsagePercent)
	return resp, nil
}
