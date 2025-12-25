package podmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetPodStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Pod 状态
func NewGetPodStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPodStatusLogic {
	return &GetPodStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPodStatusLogic) GetPodStatus(req *types.GetPodStatusRequest) (resp *types.GetPodStatusResponse, err error) {
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

	// 4. 调用 Pod 操作器获取状态
	metrics, err := pod.GetPodStatus(req.Namespace, req.PodName, timeRange)
	if err != nil {
		l.Errorf("获取 Pod 状态失败: Namespace=%s, Pod=%s, Error=%v",
			req.Namespace, req.PodName, err)
		return nil, err
	}

	// 5. 转换为 API 响应格式
	resp = &types.GetPodStatusResponse{
		Data: types.PodStatusMetrics{
			Namespace: metrics.Namespace,
			PodName:   metrics.PodName,
			Current: types.PodStatusSnapshot{
				Timestamp: metrics.Current.Timestamp.Unix(),
				Phase:     metrics.Current.Phase,
				Ready:     metrics.Current.Ready,
				Reason:    metrics.Current.Reason,
				Message:   metrics.Current.Message,
			},
		},
	}

	// 转换容器状态
	for _, cs := range metrics.Current.ContainerStates {
		resp.Data.Current.ContainerStates = append(resp.Data.Current.ContainerStates, types.ContainerState{
			ContainerName: cs.ContainerName,
			Ready:         cs.Ready,
			RestartCount:  cs.RestartCount,
			State:         cs.State,
			Reason:        cs.Reason,
		})
	}

	// 转换历史数据
	for _, point := range metrics.History {
		resp.Data.History = append(resp.Data.History, types.PodStatusDataPoint{
			Timestamp: point.Timestamp.Unix(),
			Phase:     point.Phase,
			Ready:     point.Ready,
		})
	}

	// 转换状态转换记录
	for _, trans := range metrics.Transitions {
		resp.Data.Transitions = append(resp.Data.Transitions, types.StatusTransition{
			Timestamp: trans.Timestamp.Unix(),
			FromPhase: trans.FromPhase,
			ToPhase:   trans.ToPhase,
			Reason:    trans.Reason,
		})
	}

	l.Infof("获取 Pod 状态成功: Namespace=%s, Pod=%s, Phase=%s, Ready=%v",
		req.Namespace, req.PodName, metrics.Current.Phase, metrics.Current.Ready)

	return resp, nil
}
