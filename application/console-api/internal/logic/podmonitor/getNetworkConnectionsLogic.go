package podmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNetworkConnectionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取网络连接情况
func NewGetNetworkConnectionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNetworkConnectionsLogic {
	return &GetNetworkConnectionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNetworkConnectionsLogic) GetNetworkConnections(req *types.GetNetworkConnectionsRequest) (resp *types.GetNetworkConnectionsResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	pod := client.Pod()

	timeRange := parseRangeTime(req.Start, req.End, req.Step)

	metrics, err := pod.GetNetworkConnections(req.Namespace, req.PodName, timeRange)
	if err != nil {
		l.Errorf("获取网络连接情况失败: Error=%v", err)
		return nil, err
	}

	resp = &types.GetNetworkConnectionsResponse{
		Data: types.NetworkConnectionMetrics{
			Namespace: metrics.Namespace,
			PodName:   metrics.PodName,
			Current: types.ConnectionSnapshot{
				Timestamp:      metrics.Current.Timestamp.Unix(),
				TCPConnections: metrics.Current.TCPConnections,
				UDPConnections: metrics.Current.UDPConnections,
				TCPListening:   metrics.Current.TCPListening,
				TCPEstablished: metrics.Current.TCPEstablished,
				TCPTimeWait:    metrics.Current.TCPTimeWait,
				TCPCloseWait:   metrics.Current.TCPCloseWait,
			},
			Summary: types.ConnectionSummary{
				AvgTCPEstablished: metrics.Summary.AvgTCPEstablished,
				MaxTCPEstablished: metrics.Summary.MaxTCPEstablished,
				AvgTCPTimeWait:    metrics.Summary.AvgTCPTimeWait,
				MaxTCPTimeWait:    metrics.Summary.MaxTCPTimeWait,
			},
		},
	}

	for _, point := range metrics.Trend {
		resp.Data.Trend = append(resp.Data.Trend, types.ConnectionDataPoint{
			Timestamp:      point.Timestamp.Unix(),
			TCPEstablished: point.TCPEstablished,
			TCPTimeWait:    point.TCPTimeWait,
			TotalActive:    point.TotalActive,
		})
	}

	l.Infof("获取网络连接情况成功: Established=%d, TimeWait=%d",
		metrics.Current.TCPEstablished, metrics.Current.TCPTimeWait)
	return resp, nil
}
