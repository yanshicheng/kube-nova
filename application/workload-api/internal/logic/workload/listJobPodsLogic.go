package workload

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	types2 "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListJobPodsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取指定job pod列表
func NewListJobPodsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListJobPodsLogic {
	return &ListJobPodsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListJobPodsLogic) ListJobPods(req *types.GetJobPodListRequest) (resp []types.PodResourceList, err error) {
	workspace, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{Id: req.Id})
	if err != nil {
		l.Errorf("获取项目工作空间详情失败: %v", err)
		return nil, fmt.Errorf("获取项目工作空间详情失败: %v", err)
	}
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workspace.Data.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败: %v", err)
	}
	pods, err := client.Pods().GetResourcePods(&types2.GetPodsRequest{
		Namespace:    workspace.Data.Namespace,
		ResourceName: req.JobName,
		ResourceType: "Job",
	})
	if err != nil {
		l.Errorf("获取job pod列表失败: %v", err)
		return nil, fmt.Errorf("获取job pod列表失败: %v", err)
	}
	resp = l.convertToResourceList(pods)
	return
}

// 格式转换函数
func (l *ListJobPodsLogic) convertToResourceList(pods []types2.PodDetailInfo) []types.PodResourceList {
	tpod := make([]types.PodResourceList, 0, len(pods))
	for _, pod := range pods {
		tpod = append(tpod, types.PodResourceList{
			Name:         pod.Name,
			Status:       pod.Status,
			CreationTime: pod.CreationTime,
			Namespace:    pod.Namespace,
			Ready:        pod.Ready,
			Restarts:     pod.Restarts,
			PodIP:        pod.PodIP,
			Age:          pod.Age,
			Node:         pod.Node,
			Labels:       pod.Labels,
		})
	}
	return tpod
}
