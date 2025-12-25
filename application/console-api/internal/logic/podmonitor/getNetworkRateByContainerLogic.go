package podmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNetworkRateByContainerLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取容器网络速率
func NewGetNetworkRateByContainerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNetworkRateByContainerLogic {
	return &GetNetworkRateByContainerLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNetworkRateByContainerLogic) GetNetworkRateByContainer(req *types.GetNetworkRateByContainerRequest) (resp *types.GetNetworkRateByContainerResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	pod := client.Pod()

	timeRange := parseRangeTime(req.Start, req.End, req.Step)

	metrics, err := pod.GetNetworkRateByContainer(req.Namespace, req.PodName, req.ContainerName, timeRange)
	if err != nil {
		l.Errorf("获取容器网络速率失败: Container=%s, Error=%v", req.ContainerName, err)
		return nil, err
	}

	resp = &types.GetNetworkRateByContainerResponse{
		Data: types.ContainerNetworkRateMetrics{
			Namespace:     metrics.Namespace,
			PodName:       metrics.PodName,
			ContainerName: metrics.ContainerName,
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
