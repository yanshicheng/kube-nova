package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type IngressListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Ingress 列表
func NewIngressListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *IngressListLogic {
	return &IngressListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}
func (l *IngressListLogic) IngressList(req *types.ListRequest) (resp *types.IngressListResponse, err error) {
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

	ingressClient := client.Ingresses()

	ingressList, err := ingressClient.List(workloadInfo.Data.Namespace, req.Search, req.LabelSelector)
	if err != nil {
		l.Errorf("获取 Ingress 列表失败: %v", err)
		return nil, fmt.Errorf("获取 Ingress 列表失败")
	}

	items := make([]types.IngressListItem, 0, len(ingressList.Items))
	for _, ing := range ingressList.Items {
		items = append(items, types.IngressListItem{
			Name:              ing.Name,
			Namespace:         ing.Namespace,
			IngressClass:      ing.IngressClass,
			Hosts:             ing.Hosts,
			Address:           ing.Address,
			Ports:             ing.Ports,
			Age:               ing.Age,
			CreationTimestamp: ing.CreationTimestamp,
			Labels:            ing.Labels,
		})
	}

	l.Infof("成功获取 Ingress 列表，共 %d 个", len(items))
	return &types.IngressListResponse{
		Total: ingressList.Total,
		Items: items,
	}, nil
}
