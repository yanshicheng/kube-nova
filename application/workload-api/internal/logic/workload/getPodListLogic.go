package workload

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	types2 "github.com/yanshicheng/kube-nova/common/k8smanager/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetPodListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取pod列表
func NewGetPodListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPodListLogic {
	return &GetPodListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPodListLogic) GetPodList(req *types.DefaultIdRequest) (resp []types.PodResourceList, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}
	podList, err := client.Pods().GetResourcePods(&types2.GetPodsRequest{
		Namespace:    versionDetail.Namespace,
		ResourceName: versionDetail.ResourceName,
		ResourceType: versionDetail.ResourceType,
	})
	if err != nil {
		l.Errorf("获取 Pod 列表失败: %v", err)
		return nil, err
	}

	resp = l.convertToResourceList(podList)
	return
}

// 格式转换函数
func (l *GetPodListLogic) convertToResourceList(pods []types2.PodDetailInfo) []types.PodResourceList {
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
