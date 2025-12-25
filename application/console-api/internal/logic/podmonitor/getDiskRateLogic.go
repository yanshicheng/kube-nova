package podmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetDiskRateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取磁盘速率
func NewGetDiskRateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDiskRateLogic {
	return &GetDiskRateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetDiskRateLogic) GetDiskRate(req *types.GetDiskRateRequest) (resp *types.GetDiskRateResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	pod := client.Pod()

	timeRange := parseRangeTime(req.Start, req.End, req.Step)

	metrics, err := pod.GetDiskRate(req.Namespace, req.PodName, timeRange)
	if err != nil {
		l.Errorf("获取磁盘速率失败: Error=%v", err)
		return nil, err
	}

	resp = &types.GetDiskRateResponse{
		Data: types.DiskRateMetrics{
			Namespace: metrics.Namespace,
			PodName:   metrics.PodName,
			Current: types.DiskRateSnapshot{
				Timestamp:        metrics.Current.Timestamp.Unix(),
				ReadBytesPerSec:  metrics.Current.ReadBytesPerSec,
				WriteBytesPerSec: metrics.Current.WriteBytesPerSec,
				ReadOpsPerSec:    metrics.Current.ReadOpsPerSec,
				WriteOpsPerSec:   metrics.Current.WriteOpsPerSec,
			},
			Summary: types.DiskRateSummary{
				AvgReadBytesPerSec:  metrics.Summary.AvgReadBytesPerSec,
				MaxReadBytesPerSec:  metrics.Summary.MaxReadBytesPerSec,
				AvgWriteBytesPerSec: metrics.Summary.AvgWriteBytesPerSec,
				MaxWriteBytesPerSec: metrics.Summary.MaxWriteBytesPerSec,
			},
		},
	}

	for _, point := range metrics.Trend {
		resp.Data.Trend = append(resp.Data.Trend, types.DiskRateDataPoint{
			Timestamp:        point.Timestamp.Unix(),
			ReadBytesPerSec:  point.ReadBytesPerSec,
			WriteBytesPerSec: point.WriteBytesPerSec,
		})
	}

	return resp, nil
}
