package podmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetVolumeIOPSLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Volume IOPS
func NewGetVolumeIOPSLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetVolumeIOPSLogic {
	return &GetVolumeIOPSLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetVolumeIOPSLogic) GetVolumeIOPS(req *types.GetVolumeIOPSRequest) (resp *types.GetVolumeIOPSResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	pod := client.Pod()

	timeRange := parseRangeTime(req.Start, req.End, req.Step)

	metrics, err := pod.GetVolumeIOPS(req.Namespace, req.PodName, timeRange)
	if err != nil {
		l.Errorf("获取 Volume IOPS 失败: Error=%v", err)
		return nil, err
	}

	resp = &types.GetVolumeIOPSResponse{
		Data: types.VolumeIOPSMetrics{
			Namespace: metrics.Namespace,
			PodName:   metrics.PodName,
			Current: types.IOPSSnapshot{
				Timestamp: metrics.Current.Timestamp.Unix(),
				ReadIOPS:  metrics.Current.ReadIOPS,
				WriteIOPS: metrics.Current.WriteIOPS,
				TotalIOPS: metrics.Current.TotalIOPS,
			},
			Summary: types.IOPSSummary{
				AvgReadIOPS:  metrics.Summary.AvgReadIOPS,
				MaxReadIOPS:  metrics.Summary.MaxReadIOPS,
				AvgWriteIOPS: metrics.Summary.AvgWriteIOPS,
				MaxWriteIOPS: metrics.Summary.MaxWriteIOPS,
			},
		},
	}

	for _, point := range metrics.Trend {
		resp.Data.Trend = append(resp.Data.Trend, types.IOPSDataPoint{
			Timestamp: point.Timestamp.Unix(),
			ReadIOPS:  point.ReadIOPS,
			WriteIOPS: point.WriteIOPS,
			TotalIOPS: point.TotalIOPS,
		})
	}

	l.Infof("获取 Volume IOPS 成功: Read=%.2f, Write=%.2f",
		metrics.Current.ReadIOPS, metrics.Current.WriteIOPS)
	return resp, nil
}
