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

type GetWorkloadPodContainersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取pod的所有containers通过workload
func NewGetWorkloadPodContainersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetWorkloadPodContainersLogic {
	return &GetWorkloadPodContainersLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetWorkloadPodContainersLogic) GetWorkloadPodContainers(req *types.GetDefaultPodNameRequest) (resp *types.ContainerInfoList, err error) {
	workload, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("获取工作空间失败: %v", err)
		return nil, fmt.Errorf("获取工作空间失败")
	}
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workload.Data.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}
	containers, err := client.Pods().GetAllContainers(workload.Data.Namespace, req.PodName)
	if err != nil {
		l.Errorf("获取pod containers失败: %v", err)
		return nil, fmt.Errorf("获取pod containers失败")
	}
	resp = l.convertToContainerInfoList(containers)
	return
}

func (l *GetWorkloadPodContainersLogic) convertToContainerInfoList(container *types2.ContainerInfoList) *types.ContainerInfoList {
	// 转换 InitContainers
	initContainers := make([]types.ContainerInfo, 0, len(container.InitContainers))
	for _, c := range container.InitContainers {
		initContainers = append(initContainers, types.ContainerInfo{
			Name:  c.Name,
			Image: c.Image,
		})
	}

	// 转换 Containers
	containersList := make([]types.ContainerInfo, 0, len(container.Containers))
	for _, c := range container.Containers {
		containersList = append(containersList, types.ContainerInfo{
			Name:  c.Name,
			Image: c.Image,
		})
	}
	// 转换 EphemeralContainers
	ephemeralContainers := make([]types.ContainerInfo, 0, len(container.EphemeralContainers))
	for _, c := range container.EphemeralContainers {
		ephemeralContainers = append(ephemeralContainers, types.ContainerInfo{
			Name:  c.Name,
			Image: c.Image,
		})
	}
	// 返回单个对象
	return &types.ContainerInfoList{
		InitContainers:      initContainers,
		Containers:          containersList,
		EphemeralContainers: ephemeralContainers,
	}
}
