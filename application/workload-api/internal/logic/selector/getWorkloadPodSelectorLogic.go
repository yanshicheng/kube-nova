package selector

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetWorkloadPodSelectorLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取工作负载 Pod 选择器
func NewGetWorkloadPodSelectorLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetWorkloadPodSelectorLogic {
	return &GetWorkloadPodSelectorLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetWorkloadPodSelectorLogic) GetWorkloadPodSelector(req *types.WorkloadResourceSelectorRequest) (resp *types.PodSelectorResponse, err error) {
	// 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	var matchLabels map[string]string
	var labels map[string]string

	// 根据 type 调用对应的 operator
	switch req.Type {
	case "deployment":
		deploymentOp := client.Deployment()
		labels, err = deploymentOp.GetPodLabels(req.Namespace, req.Name)
		if err != nil {
			return nil, fmt.Errorf("获取 Deployment 失败: %v", err)
		}
		matchLabels, err = deploymentOp.GetPodSelectorLabels(req.Namespace, req.Name)
		if err != nil {
			return nil, fmt.Errorf("获取 Deployment Pod 选择器失败: %v", err)
		}

	case "statefulset":
		statefulSetOp := client.StatefulSet()
		labels, err = statefulSetOp.GetPodLabels(req.Namespace, req.Name)
		if err != nil {
			return nil, fmt.Errorf("获取 StatefulSet 失败: %v", err)
		}
		matchLabels, err = statefulSetOp.GetPodSelectorLabels(req.Namespace, req.Name)
		if err != nil {
			return nil, fmt.Errorf("获取 StatefulSet Pod 选择器失败: %v", err)
		}

	case "daemonset":
		daemonSetOp := client.DaemonSet()
		labels, err = daemonSetOp.GetPodLabels(req.Namespace, req.Name)
		if err != nil {
			return nil, fmt.Errorf("获取 DaemonSet 失败: %v", err)
		}
		matchLabels, err = daemonSetOp.GetPodSelectorLabels(req.Namespace, req.Name)
		if err != nil {
			return nil, fmt.Errorf("获取 DaemonSet Pod 选择器失败: %v", err)
		}

	case "cronjob":
		cronJobOp := client.CronJob()
		labels, err = cronJobOp.GetPodLabels(req.Namespace, req.Name)
		if err != nil {
			return nil, fmt.Errorf("获取 CronJob 失败: %v", err)
		}
		matchLabels, err = cronJobOp.GetPodSelectorLabels(req.Namespace, req.Name)
		if err != nil {
			return nil, fmt.Errorf("获取 CronJob Pod 选择器失败: %v", err)
		}

	default:
		return nil, fmt.Errorf("不支持的资源类型: %s", req.Type)
	}

	return &types.PodSelectorResponse{
		MatchLabels: matchLabels,
		Labels:      labels,
	}, nil
}
