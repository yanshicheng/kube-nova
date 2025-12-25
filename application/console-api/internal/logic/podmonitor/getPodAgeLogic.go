package podmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetPodAgeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Pod 运行时间
func NewGetPodAgeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPodAgeLogic {
	return &GetPodAgeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPodAgeLogic) GetPodAge(req *types.GetPodAgeRequest) (resp *types.GetPodAgeResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	pod := client.Pod()

	metrics, err := pod.GetPodAge(req.Namespace, req.PodName)
	if err != nil {
		l.Errorf("获取 Pod 运行时间失败: Error=%v", err)
		return nil, err
	}

	resp = &types.GetPodAgeResponse{
		Data: types.PodAgeMetrics{
			Namespace:     metrics.Namespace,
			PodName:       metrics.PodName,
			CreationTime:  metrics.CreationTime.Unix(),
			Age:           metrics.Age,
			AgeSeconds:    metrics.AgeSeconds,
			Uptime:        metrics.Uptime,
			UptimeSeconds: metrics.UptimeSeconds,
		},
	}

	l.Infof("获取 Pod 运行时间成功: Pod=%s, Age=%s", req.PodName, metrics.Age)
	return resp, nil
}
