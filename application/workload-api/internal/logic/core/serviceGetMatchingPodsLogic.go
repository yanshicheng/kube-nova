package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceGetMatchingPodsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Service 选择器匹配的 Pods
func NewServiceGetMatchingPodsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceGetMatchingPodsLogic {
	return &ServiceGetMatchingPodsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceGetMatchingPodsLogic) ServiceGetMatchingPods(req *types.DefaultNameRequest) (resp []types.MatchedPodInfo, err error) {
	workloadInfo, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{Id: req.WorkloadId})
	if err != nil {
		l.Errorf("获取项目工作空间详情失败: %v", err)
		return nil, fmt.Errorf("获取项目工作空间详情失败")
	}

	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workloadInfo.Data.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	serviceClient := client.Services()

	pods, err := serviceClient.GetMatchingPods(workloadInfo.Data.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取匹配的 Pods 失败: %v", err)
		return nil, fmt.Errorf("获取匹配的 Pods 失败")
	}

	matchedPods := make([]types.MatchedPodInfo, 0, len(pods))
	for _, pod := range pods {
		matchedPods = append(matchedPods, types.MatchedPodInfo{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			PodIP:     pod.PodIP,
			Status:    pod.Status,
			NodeName:  pod.Node,
		})
	}

	l.Infof("成功获取匹配的 Pods: %s, 共 %d 个", req.Name, len(matchedPods))
	return matchedPods, nil
}
