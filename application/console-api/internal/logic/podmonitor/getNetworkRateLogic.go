package podmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNetworkRateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取网络速率
func NewGetNetworkRateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNetworkRateLogic {
	return &GetNetworkRateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNetworkRateLogic) GetNetworkRate(req *types.GetNetworkRateRequest) (resp *types.GetNetworkRateResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	pod := client.Pod()

	timeRange := parseRangeTime(req.Start, req.End, req.Step)

	metrics, err := pod.GetNetworkRate(req.Namespace, req.PodName, timeRange)
	if err != nil {
		l.Errorf("获取网络速率失败: Error=%v", err)
		return nil, err
	}

	resp = &types.GetNetworkRateResponse{
		Data: types.NetworkRateMetrics{
			Namespace: metrics.Namespace,
			PodName:   metrics.PodName,
			Current: types.NetworkRateSnapshot{
				Timestamp:           metrics.Current.Timestamp.Unix(),
				ReceiveBytesPerSec:  metrics.Current.ReceiveBytesPerSec,
				TransmitBytesPerSec: metrics.Current.TransmitBytesPerSec,
			},
			Summary: types.NetworkRateSummary{
				AvgReceiveBytesPerSec:  metrics.Summary.AvgReceiveBytesPerSec,
				MaxReceiveBytesPerSec:  metrics.Summary.MaxReceiveBytesPerSec,
				AvgTransmitBytesPerSec: metrics.Summary.AvgTransmitBytesPerSec,
				MaxTransmitBytesPerSec: metrics.Summary.MaxTransmitBytesPerSec,
			},
		},
	}

	for _, point := range metrics.Trend {
		resp.Data.Trend = append(resp.Data.Trend, types.NetworkRateDataPoint{
			Timestamp:           point.Timestamp.Unix(),
			ReceiveBytesPerSec:  point.ReceiveBytesPerSec,
			TransmitBytesPerSec: point.TransmitBytesPerSec,
		})
	}

	return resp, nil
}
