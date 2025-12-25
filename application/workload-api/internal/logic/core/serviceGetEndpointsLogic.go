package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceGetEndpointsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Service Endpoints
func NewServiceGetEndpointsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceGetEndpointsLogic {
	return &ServiceGetEndpointsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceGetEndpointsLogic) ServiceGetEndpoints(req *types.DefaultNameRequest) (resp *types.ServiceEndpointsResponse, err error) {
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

	endpoints, err := serviceClient.GetEndpoints(workloadInfo.Data.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取 Service Endpoints 失败: %v", err)
		return nil, fmt.Errorf("获取 Service Endpoints 失败")
	}

	endpointList := make([]types.ServiceEndpoint, 0, len(endpoints.Endpoints))
	for _, ep := range endpoints.Endpoints {
		endpointList = append(endpointList, types.ServiceEndpoint{
			PodName:   ep.PodName,
			PodIP:     ep.PodIP,
			NodeName:  ep.NodeName,
			Ready:     ep.Ready,
			Ports:     ep.Ports,
			Addresses: ep.Addresses,
		})
	}

	l.Infof("成功获取 Service Endpoints: %s, 共 %d 个", req.Name, len(endpointList))
	return &types.ServiceEndpointsResponse{
		ServiceName: endpoints.ServiceName,
		Namespace:   endpoints.Namespace,
		Endpoints:   endpointList,
		TotalCount:  endpoints.TotalCount,
	}, nil
}
