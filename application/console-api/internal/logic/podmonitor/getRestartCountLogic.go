package podmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetRestartCountLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取重启次数
func NewGetRestartCountLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRestartCountLogic {
	return &GetRestartCountLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetRestartCountLogic) GetRestartCount(req *types.GetRestartCountRequest) (resp *types.GetRestartCountResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	pod := client.Pod()

	timeRange := parseRangeTime(req.Start, req.End, req.Step)

	metrics, err := pod.GetRestartCount(req.Namespace, req.PodName, timeRange)
	if err != nil {
		l.Errorf("获取重启次数失败: Error=%v", err)
		return nil, err
	}

	resp = &types.GetRestartCountResponse{
		Data: types.RestartMetrics{
			Namespace:     metrics.Namespace,
			PodName:       metrics.PodName,
			TotalRestarts: metrics.TotalRestarts,
		},
	}

	for _, container := range metrics.ByContainer {
		cr := types.ContainerRestart{
			ContainerName: container.ContainerName,
			RestartCount:  container.RestartCount,
		}
		if container.LastRestart != nil {
			lastRestart := container.LastRestart.Unix()
			cr.LastRestart = lastRestart
		}
		resp.Data.ByContainer = append(resp.Data.ByContainer, cr)
	}

	for _, event := range metrics.RecentRestarts {
		resp.Data.RecentRestarts = append(resp.Data.RecentRestarts, types.RestartEvent{
			Timestamp:     event.Timestamp.Unix(),
			ContainerName: event.ContainerName,
			Reason:        event.Reason,
			ExitCode:      event.ExitCode,
		})
	}

	for _, point := range metrics.Trend {
		resp.Data.Trend = append(resp.Data.Trend, types.RestartDataPoint{
			Timestamp:    point.Timestamp.Unix(),
			RestartCount: point.RestartCount,
		})
	}

	l.Infof("获取重启次数成功: Pod=%s, TotalRestarts=%d", req.PodName, metrics.TotalRestarts)
	return resp, nil
}
