package podmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetMemoryOOMLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 OOM 情况
func NewGetMemoryOOMLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMemoryOOMLogic {
	return &GetMemoryOOMLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetMemoryOOMLogic) GetMemoryOOM(req *types.GetMemoryOOMRequest) (resp *types.GetMemoryOOMResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	pod := client.Pod()

	timeRange := parseRangeTime(req.Start, req.End, req.Step)

	metrics, err := pod.GetMemoryOOM(req.Namespace, req.PodName, timeRange)
	if err != nil {
		l.Errorf("获取 OOM 情况失败: Error=%v", err)
		return nil, err
	}

	resp = &types.GetMemoryOOMResponse{
		Data: types.OOMMetrics{
			Namespace:     metrics.Namespace,
			PodName:       metrics.PodName,
			TotalOOMKills: metrics.TotalOOMKills,
		},
	}

	for _, container := range metrics.ByContainer {
		resp.Data.ByContainer = append(resp.Data.ByContainer, types.ContainerOOM{
			ContainerName: container.ContainerName,
			OOMKills:      container.OOMKills,
		})
	}

	for _, event := range metrics.Events {
		resp.Data.Events = append(resp.Data.Events, types.OOMEvent{
			Timestamp:     event.Timestamp.Unix(),
			ContainerName: event.ContainerName,
			Reason:        event.Reason,
		})
	}

	l.Infof("获取 OOM 情况成功: Pod=%s, TotalOOMs=%d", req.PodName, metrics.TotalOOMKills)
	return resp, nil
}
