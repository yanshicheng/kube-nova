package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	types2 "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ServicesGetResourceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取资源关联的 Service 列表
func NewServicesGetResourceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServicesGetResourceLogic {
	return &ServicesGetResourceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServicesGetResourceLogic) ServicesGetResource(req *types.DefaultNameRequest) (resp []types.ServiceListItem, err error) {
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

	getResourceReq := &types2.GetResourceServicesRequest{
		Namespace:    workloadInfo.Data.Namespace,
		ResourceName: req.Name,
	}

	services, err := serviceClient.GetResourceServices(getResourceReq)
	if err != nil {
		l.Errorf("获取资源关联的 Services 失败: %v", err)
		return nil, fmt.Errorf("获取资源关联的 Services 失败")
	}

	serviceList := make([]types.ServiceListItem, 0, len(services))
	for _, svc := range services {
		serviceList = append(serviceList, types.ServiceListItem{
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

	l.Infof("成功获取资源关联的 Services: %s, 共 %d 个", req.Name, len(serviceList))
	return serviceList, nil
}
