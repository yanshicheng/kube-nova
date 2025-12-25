package podmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNetworkIOByContainerLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取容器网络 I/O
func NewGetNetworkIOByContainerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNetworkIOByContainerLogic {
	return &GetNetworkIOByContainerLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNetworkIOByContainerLogic) GetNetworkIOByContainer(req *types.GetNetworkIOByContainerRequest) (resp *types.GetNetworkIOByContainerResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	pod := client.Pod()

	timeRange := parseRangeTime(req.Start, req.End, req.Step)

	metrics, err := pod.GetNetworkIOByContainer(req.Namespace, req.PodName, req.ContainerName, timeRange)
	if err != nil {
		l.Errorf("获取容器网络 I/O 失败: Container=%s, Error=%v", req.ContainerName, err)
		return nil, err
	}

	resp = &types.GetNetworkIOByContainerResponse{
		Data: types.ContainerNetworkMetrics{
			Namespace:     metrics.Namespace,
			PodName:       metrics.PodName,
			ContainerName: metrics.ContainerName,
			Current: types.NetworkSnapshot{
				Timestamp:       metrics.Current.Timestamp.Unix(),
				ReceiveBytes:    metrics.Current.ReceiveBytes,
				TransmitBytes:   metrics.Current.TransmitBytes,
				ReceivePackets:  metrics.Current.ReceivePackets,
				TransmitPackets: metrics.Current.TransmitPackets,
				ReceiveErrors:   metrics.Current.ReceiveErrors,
				TransmitErrors:  metrics.Current.TransmitErrors,
			},
			Summary: types.NetworkSummary{
				TotalReceiveBytes:    metrics.Summary.TotalReceiveBytes,
				TotalTransmitBytes:   metrics.Summary.TotalTransmitBytes,
				TotalReceivePackets:  metrics.Summary.TotalReceivePackets,
				TotalTransmitPackets: metrics.Summary.TotalTransmitPackets,
				TotalErrors:          metrics.Summary.TotalErrors,
			},
		},
	}

	for _, point := range metrics.Trend {
		resp.Data.Trend = append(resp.Data.Trend, types.NetworkDataPoint{
			Timestamp:     point.Timestamp.Unix(),
			ReceiveBytes:  point.ReceiveBytes,
			TransmitBytes: point.TransmitBytes,
		})
	}

	return resp, nil
}
