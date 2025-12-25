package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ServicesListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Service 列表
func NewServicesListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServicesListLogic {
	return &ServicesListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServicesListLogic) ServicesList(req *types.ListRequest) (resp *types.ServiceListResponse, err error) {
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

	serviceList, err := serviceClient.List(workloadInfo.Data.Namespace, req.Search, req.LabelSelector)
	if err != nil {
		l.Errorf("获取 Service 列表失败: %v", err)
		return nil, fmt.Errorf("获取 Service 列表失败")
	}

	items := make([]types.ServiceListItem, 0, len(serviceList.Items))
	for _, svc := range serviceList.Items {
		items = append(items, types.ServiceListItem{
			Name:              svc.Name,
			Namespace:         svc.Namespace,
			Type:              svc.Type,
			ClusterIP:         svc.ClusterIP,
			ExternalIP:        svc.ExternalIP,
			Ports:             svc.Ports,
			Age:               svc.Age,
			CreationTimestamp: svc.CreationTimestamp,
			Labels:            svc.Labels,
		})
	}

	l.Infof("成功获取 Service 列表，共 %d 个", len(items))
	return &types.ServiceListResponse{
		Total: serviceList.Total,
		Items: items,
	}, nil
}
