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

// GetResourceImages 查询资源使用的镜像（返回新格式 CommContainerInfoList）
func (l *GetResourceImagesLogic) GetResourceImages(req *types.DefaultIdRequest) (resp *types.CommContainerInfoList, err error) {
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

	// 转换为新的扁平化格式
	resp = l.convertToCommContainerInfoList(containerInfoList)
	return resp, nil
}

// convertToCommContainerInfoList 将旧格式转换为新的扁平化格式
func (l *GetResourceImagesLogic) convertToCommContainerInfoList(container *types2.ContainerInfoList) *types.CommContainerInfoList {
	result := &types.CommContainerInfoList{
		Containers: make([]types.CommContainerImageInfo, 0),
	}

	// 添加 init 容器
	for _, c := range container.InitContainers {
		result.Containers = append(result.Containers, types.CommContainerImageInfo{
			ContainerName: c.Name,
			ContainerType: "init",
			Image:         c.Image,
		})
	}

	// 添加主容器
	for _, c := range container.Containers {
		result.Containers = append(result.Containers, types.CommContainerImageInfo{
			ContainerName: c.Name,
			ContainerType: "main",
			Image:         c.Image,
		})
	}

	// 添加临时容器
	for _, c := range container.EphemeralContainers {
		result.Containers = append(result.Containers, types.CommContainerImageInfo{
			ContainerName: c.Name,
			ContainerType: "ephemeral",
			Image:         c.Image,
		})
	}

	return result
}
