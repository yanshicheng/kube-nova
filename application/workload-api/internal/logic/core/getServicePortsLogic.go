package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	types2 "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetServicePortsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 service 端口
func NewGetServicePortsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetServicePortsLogic {
	return &GetServicePortsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetServicePortsLogic) GetServicePorts(req *types.ServicePortsRequest) (resp []types.ServcePortsResponse, err error) {
	// 1. 获取工作空间信息
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

	// 4. 根据不同参数获取 Service 列表
	var services []types2.ServiceInfo

	switch {
	case req.VersionId > 0:
		// 情况1: 传递了 VersionId，返回该版本相关的 Service
		services, err = l.getServicesByVersion(req.VersionId, workloadInfo.Data.Namespace, workloadInfo.Data.ClusterUuid)
		if err != nil {
			return nil, err
		}

	case req.ApplicationId > 0:
		// 情况2: 传递了 ApplicationId，返回该应用所有版本的 Service
		services, err = l.getServicesByApplication(req.ApplicationId, req.WorkloadId)
		if err != nil {
			return nil, err
		}

	default:
		// 情况3: 只传递了 WorkloadId，返回该 namespace 下所有 Service
		serviceList, err := serviceClient.List(workloadInfo.Data.Namespace, "", "")
		if err != nil {
			l.Errorf("获取 namespace %s 所有 Service 失败: %v", workloadInfo.Data.Namespace, err)
			return nil, fmt.Errorf("获取 Service 列表失败")
		}
		services = serviceList.Items
	}

	// 5. 提取每个 Service 的端口信息
	resp = make([]types.ServcePortsResponse, 0, len(services))
	for _, svc := range services {
		// 获取 Service 详细信息
		serviceDetail, err := serviceClient.GetDetail(workloadInfo.Data.Namespace, svc.Name)
		if err != nil {
			l.Errorf("获取 Service %s 详情失败: %v", svc.Name, err)
			// 跳过错误的 Service，继续处理下一个
			continue
		}

		// 提取端口列表（Service 的 Port 字段）
		ports := make([]int32, 0, len(serviceDetail.Ports))
		for _, port := range serviceDetail.Ports {
			ports = append(ports, port.Port)
		}

		resp = append(resp, types.ServcePortsResponse{
			Nmae:  serviceDetail.Name,
			Ports: ports,
		})
	}

	l.Infof("成功获取 %d 个 Service 的端口信息", len(resp))
	return resp, nil
}

// getServicesByVersion 根据版本ID获取相关的 Service 列表
func (l *GetServicePortsLogic) getServicesByVersion(versionId uint64, namespace string, clusterUuid string) ([]types2.ServiceInfo, error) {
	// 1. 获取版本详情
	versionDetail, err := l.svcCtx.ManagerRpc.VersionDetail(l.ctx, &managerservice.GetOnecProjectVersionDetailReq{
		Id: versionId,
	})
	if err != nil {
		l.Errorf("获取版本详情失败: %v", err)
		return nil, fmt.Errorf("获取版本详情失败")
	}

	// 2. 获取应用信息
	app, err := l.svcCtx.ManagerRpc.ApplicationGetById(l.ctx, &managerservice.GetOnecProjectApplicationByIdReq{
		Id: versionDetail.ApplicationId,
	})
	if err != nil {
		l.Errorf("获取应用信息失败: %v", err)
		return nil, fmt.Errorf("获取应用信息失败")
	}

	// 3. 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, clusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	// 4. 获取该版本的 Pod Labels
	resourceType := strings.ToLower(app.Data.ResourceType)
	var podLabels map[string]string

	switch resourceType {
	case "deployment":
		podLabels, err = client.Deployment().GetPodLabels(namespace, versionDetail.ResourceName)
	case "statefulset":
		podLabels, err = client.StatefulSet().GetPodLabels(namespace, versionDetail.ResourceName)
	case "daemonset":
		podLabels, err = client.DaemonSet().GetPodLabels(namespace, versionDetail.ResourceName)
	default:
		return nil, fmt.Errorf("不支持的资源类型: %s", resourceType)
	}

	if err != nil {
		l.Errorf("获取 Pod Labels 失败: %v", err)
		return nil, fmt.Errorf("获取 Pod Labels 失败")
	}

	// 5. 根据 labels 查询 Service
	services, err := client.Services().GetServicesByLabels(namespace, podLabels)
	if err != nil {
		l.Errorf("根据 labels 查询 Service 失败: %v", err)
		return nil, fmt.Errorf("查询 Service 失败")
	}

	return services, nil
}

// getServicesByApplication 根据应用ID获取所有版本相关的 Service 列表
func (l *GetServicePortsLogic) getServicesByApplication(appId, workloadId uint64) ([]types2.ServiceInfo, error) {
	// 复用 ApplicationServiceGetLogic 的逻辑
	getLogic := NewApplicationServiceGetLogic(l.ctx, l.svcCtx)

	serviceList, err := getLogic.ApplicationServiceGet(&types.GetAppServicesRequest{
		ApplicationId: appId,
		WorkloadId:    workloadId,
	})
	if err != nil {
		return nil, err
	}

	// 转换为 ServiceInfo 类型
	services := make([]types2.ServiceInfo, 0, len(serviceList))
	for _, svc := range serviceList {
		services = append(services, types2.ServiceInfo{
			Name:                  svc.Name,
			Namespace:             svc.Namespace,
			Type:                  svc.Type,
			ClusterIP:             svc.ClusterIP,
			ExternalIP:            svc.ExternalIP,
			Ports:                 svc.Ports,
			Age:                   svc.Age,
			CreationTimestamp:     svc.CreationTimestamp,
			Labels:                svc.Labels,
			ClusterIPs:            svc.ClusterIPs,
			IpFamilies:            svc.IpFamilies,
			IpFamilyPolicy:        svc.IpFamilyPolicy,
			ExternalTrafficPolicy: svc.ExternalTrafficPolicy,
			SessionAffinity:       svc.SessionAffinity,
			LoadBalancerClass:     svc.LoadBalancerClass,
		})
	}

	return services, nil
}
