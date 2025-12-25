package podmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetContainerStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取容器状态
func NewGetContainerStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetContainerStatusLogic {
	return &GetContainerStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetContainerStatusLogic) GetContainerStatus(req *types.GetContainerStatusRequest) (resp *types.GetContainerStatusResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	pod := client.Pod()

	metrics, err := pod.GetContainerStatus(req.Namespace, req.PodName, req.ContainerName)
	if err != nil {
		l.Errorf("获取容器状态失败: Container=%s, Error=%v", req.ContainerName, err)
		return nil, err
	}

	resp = &types.GetContainerStatusResponse{
		Data: types.ContainerStatusMetrics{
			Namespace:     metrics.Namespace,
			PodName:       metrics.PodName,
			ContainerName: metrics.ContainerName,
			State:         metrics.State,
			Ready:         metrics.Ready,
			RestartCount:  metrics.RestartCount,
			ExitCode:      metrics.ExitCode,
			Reason:        metrics.Reason,
			Message:       metrics.Message,
			Image:         metrics.Image,
			ImageID:       metrics.ImageID,
			ContainerID:   metrics.ContainerID,
		},
	}

	if metrics.StartTime != nil {
		startTime := metrics.StartTime.Unix()
		resp.Data.StartTime = startTime
	}

	if metrics.LastRestart != nil {
		lastRestart := metrics.LastRestart.Unix()
		resp.Data.LastRestart = lastRestart
	}

	l.Infof("获取容器状态成功: Container=%s, State=%s, Ready=%v",
		req.ContainerName, metrics.State, metrics.Ready)
	return resp, nil
}
