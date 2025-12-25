package workload

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	types2 "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPodContainersWithClusterNamespaceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 通过集群namespace获取所有containers
func NewGetPodContainersWithClusterNamespaceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPodContainersWithClusterNamespaceLogic {
	return &GetPodContainersWithClusterNamespaceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPodContainersWithClusterNamespaceLogic) GetPodContainersWithClusterNamespace(req *types.GetPodContainersWithClusterNamespaceRequest) (resp *types.ContainerInfoList, err error) {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		logx.Errorf("集群管理器获取失败: %s", err.Error())
		return nil, err
	}
	containers, err := client.Pods().GetAllContainers(req.Namespace, req.PodName)
	if err != nil {
		l.Errorf("获取pod containers失败: %v", err)
		return nil, fmt.Errorf("获取pod containers失败")
	}
	resp = l.convertToContainerInfoList(containers)
	return
}

func (l *GetPodContainersWithClusterNamespaceLogic) convertToContainerInfoList(container *types2.ContainerInfoList) *types.ContainerInfoList {
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
