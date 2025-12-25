package workload

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	types2 "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetResourceImagesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetResourceImagesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetResourceImagesLogic {
	return &GetResourceImagesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetResourceImagesLogic) GetResourceImages(req *types.DefaultIdRequest) (resp *types.ContainerInfoList, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	var containerInfoList *types2.ContainerInfoList

	switch strings.ToUpper(versionDetail.ResourceType) {
	case "DEPLOYMENT":
		containerInfoList, err = client.Deployment().GetContainerImages(versionDetail.Namespace, versionDetail.ResourceName)
	case "STATEFULSET":
		containerInfoList, err = client.StatefulSet().GetContainerImages(versionDetail.Namespace, versionDetail.ResourceName)
	case "DAEMONSET":
		containerInfoList, err = client.DaemonSet().GetContainerImages(versionDetail.Namespace, versionDetail.ResourceName)
	case "JOB":
		containerInfoList, err = client.Job().GetContainerImages(versionDetail.Namespace, versionDetail.ResourceName)
	case "CRONJOB":
		containerInfoList, err = client.CronJob().GetContainerImages(versionDetail.Namespace, versionDetail.ResourceName)
	default:
		return nil, fmt.Errorf("不支持的资源类型")
	}

	if err != nil {
		l.Errorf("获取容器镜像失败: %v", err)
		return nil, fmt.Errorf("获取容器镜像失败")
	}

	resp = l.convertToContainerInfoList(containerInfoList)
	return resp, nil
}

func (l *GetResourceImagesLogic) convertToContainerInfoList(container *types2.ContainerInfoList) *types.ContainerInfoList {
	initContainers := make([]types.ContainerInfo, 0, len(container.InitContainers))
	for _, c := range container.InitContainers {
		initContainers = append(initContainers, types.ContainerInfo{
			Name:  c.Name,
			Image: c.Image,
		})
	}

	containersList := make([]types.ContainerInfo, 0, len(container.Containers))
	for _, c := range container.Containers {
		containersList = append(containersList, types.ContainerInfo{
			Name:  c.Name,
			Image: c.Image,
		})
	}

	ephemeralContainers := make([]types.ContainerInfo, 0, len(container.EphemeralContainers))
	for _, c := range container.EphemeralContainers {
		ephemeralContainers = append(ephemeralContainers, types.ContainerInfo{
			Name:  c.Name,
			Image: c.Image,
		})
	}

	return &types.ContainerInfoList{
		InitContainers:      initContainers,
		Containers:          containersList,
		EphemeralContainers: ephemeralContainers,
	}
}
