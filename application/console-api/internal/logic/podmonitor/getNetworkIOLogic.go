package podmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNetworkIOLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取网络 I/O
func NewGetNetworkIOLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNetworkIOLogic {
	return &GetNetworkIOLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNetworkIOLogic) GetNetworkIO(req *types.GetNetworkIORequest) (resp *types.GetNetworkIOResponse, err error) {
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

	// 4. 调用 Pod 操作器获取网络 I/O
	metrics, err := pod.GetNetworkIO(req.Namespace, req.PodName, timeRange)
	if err != nil {
		l.Errorf("获取网络 I/O 失败: Namespace=%s, Pod=%s, Error=%v",
			req.Namespace, req.PodName, err)
		return nil, err
	}

	// 5. 转换为 API 响应格式
	resp = &types.GetNetworkIOResponse{
		Data: types.NetworkMetrics{
			Namespace: metrics.Namespace,
			PodName:   metrics.PodName,
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

	// 转换趋势数据
	for _, point := range metrics.Trend {
		resp.Data.Trend = append(resp.Data.Trend, types.NetworkDataPoint{
			Timestamp:     point.Timestamp.Unix(),
			ReceiveBytes:  point.ReceiveBytes,
			TransmitBytes: point.TransmitBytes,
		})
	}

	return resp, nil
}
