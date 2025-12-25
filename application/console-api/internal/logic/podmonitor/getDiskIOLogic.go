package podmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetDiskIOLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取磁盘 I/O
func NewGetDiskIOLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDiskIOLogic {
	return &GetDiskIOLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetDiskIOLogic) GetDiskIO(req *types.GetDiskIORequest) (resp *types.GetDiskIOResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	pod := client.Pod()

	timeRange := parseRangeTime(req.Start, req.End, req.Step)

	metrics, err := pod.GetDiskIO(req.Namespace, req.PodName, timeRange)
	if err != nil {
		l.Errorf("获取磁盘 I/O 失败: Error=%v", err)
		return nil, err
	}

	resp = &types.GetDiskIOResponse{
		Data: types.DiskMetrics{
			Namespace: metrics.Namespace,
			PodName:   metrics.PodName,
			Current: types.DiskSnapshot{
				Timestamp:  metrics.Current.Timestamp.Unix(),
				ReadBytes:  metrics.Current.ReadBytes,
				WriteBytes: metrics.Current.WriteBytes,
				ReadOps:    metrics.Current.ReadOps,
				WriteOps:   metrics.Current.WriteOps,
			},
			Summary: types.DiskSummary{
				TotalReadBytes:  metrics.Summary.TotalReadBytes,
				TotalWriteBytes: metrics.Summary.TotalWriteBytes,
				TotalReadOps:    metrics.Summary.TotalReadOps,
				TotalWriteOps:   metrics.Summary.TotalWriteOps,
			},
		},
	}

	for _, point := range metrics.Trend {
		resp.Data.Trend = append(resp.Data.Trend, types.DiskDataPoint{
			Timestamp:  point.Timestamp.Unix(),
			ReadBytes:  point.ReadBytes,
			WriteBytes: point.WriteBytes,
		})
	}

	return resp, nil
}
