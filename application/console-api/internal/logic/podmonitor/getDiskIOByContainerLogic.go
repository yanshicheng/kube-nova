package podmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetDiskIOByContainerLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取容器磁盘 I/O
func NewGetDiskIOByContainerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDiskIOByContainerLogic {
	return &GetDiskIOByContainerLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetDiskIOByContainerLogic) GetDiskIOByContainer(req *types.GetDiskIOByContainerRequest) (resp *types.GetDiskIOByContainerResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	pod := client.Pod()

	timeRange := parseRangeTime(req.Start, req.End, req.Step)

	metrics, err := pod.GetDiskIOByContainer(req.Namespace, req.PodName, req.ContainerName, timeRange)
	if err != nil {
		l.Errorf("获取容器磁盘 I/O 失败: Container=%s, Error=%v", req.ContainerName, err)
		return nil, err
	}

	resp = &types.GetDiskIOByContainerResponse{
		Data: types.ContainerDiskMetrics{
			Namespace:     metrics.Namespace,
			PodName:       metrics.PodName,
			ContainerName: metrics.ContainerName,
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
