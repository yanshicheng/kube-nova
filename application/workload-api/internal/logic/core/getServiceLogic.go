package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetServiceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Service 详情
func NewGetServiceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetServiceLogic {
	return &GetServiceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetServiceLogic) GetService(req *types.DefaultNameRequest) (resp *types.ServiceDetail, err error) {
	// 1. 获取集群以及命名空间
	workloadInfo, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{Id: req.WorkloadId})
	if err != nil {
		l.Errorf("获取项目工作空间详情失败: %v", err)
		return nil, fmt.Errorf("获取项目工作空间详情失败")
	}

	// 2. 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workloadInfo.Data.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	// 3. 初始化 Service 客户端
	serviceClient := client.Services()

	// 4. 获取 Service 详细信息
	serviceDetail, err := serviceClient.GetDetail(workloadInfo.Data.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取 Service 详情失败: %v", err)
		return nil, fmt.Errorf("获取 Service 详情失败")
	}

	// 5. 转换端口信息
	ports := make([]types.ServicePort, 0, len(serviceDetail.Ports))
	for _, port := range serviceDetail.Ports {
		ports = append(ports, types.ServicePort{
			Name:       port.Name,
			Protocol:   port.Protocol,
			Port:       port.Port,
			TargetPort: port.TargetPort,
			NodePort:   port.NodePort,
		})
	}

	// 6. 构造响应
	resp = &types.ServiceDetail{
		Name:               serviceDetail.Name,
		Namespace:          serviceDetail.Namespace,
		Type:               serviceDetail.Type,
		ClusterIP:          serviceDetail.ClusterIP,
		ExternalIPs:        serviceDetail.ExternalIPs,
		LoadBalancerIP:     serviceDetail.LoadBalancerIP,
		LoadBalancerStatus: serviceDetail.LoadBalancerStatus,
		Ports:              ports,
		Selector:           serviceDetail.Selector,
		SessionAffinity:    serviceDetail.SessionAffinity,
		EndpointCount:      serviceDetail.EndpointCount,
		Labels:             serviceDetail.Labels,
		Annotations:        serviceDetail.Annotations,
		Age:                serviceDetail.Age,
		CreationTimestamp:  serviceDetail.CreationTimestamp,
	}

	l.Infof("成功获取 Service 详情: %s", req.Name)
	return resp, nil
}
