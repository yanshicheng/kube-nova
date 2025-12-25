package workload

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	types2 "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPodContainersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取pod的所有containers
func NewGetPodContainersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPodContainersLogic {
	return &GetPodContainersLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPodContainersLogic) GetPodContainers(req *types.GetDefaultPodNameRequest) (resp *types.ContainerInfoList, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}
	containers, err := client.Pods().GetAllContainers(versionDetail.Namespace, req.PodName)
	if err != nil {
		l.Errorf("获取pod containers失败: %v", err)
		return nil, fmt.Errorf("获取pod containers失败")
	}
	resp = l.convertToContainerInfoList(containers)
	return
}

func (l *GetPodContainersLogic) convertToContainerInfoList(container *types2.ContainerInfoList) *types.ContainerInfoList {
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
