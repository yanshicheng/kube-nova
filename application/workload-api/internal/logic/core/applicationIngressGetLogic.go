package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ApplicationIngressGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 服务级别 Ingress 获取
func NewApplicationIngressGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApplicationIngressGetLogic {
	return &ApplicationIngressGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ApplicationIngressGetLogic) ApplicationIngressGet(req *types.GetAppIngressRequest) (resp []types.IngressListItem, err error) {
	serviceNames, namespace, workspace, err := l.getVersionServices(req.ApplicationId, req.WorkloadId)
	if err != nil {
		return nil, err
	}

	if len(serviceNames) == 0 {
		l.Logger.Infof("应用 %d 没有找到关联的 Services", req.ApplicationId)
		return []types.IngressListItem{}, nil
	}

	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workspace.Data.ClusterUuid)
	if err != nil {
		return nil, fmt.Errorf("获取集群客户端失败: %w", err)
	}

	ingressOperator := client.Ingresses()

	ingressList, err := ingressOperator.ListByServiceNames(namespace, serviceNames)
	if err != nil {
		return nil, fmt.Errorf("查询 Ingress 失败: %w", err)
	}

	l.Logger.Infof("应用 %d 找到 %d 个关联的 Ingress", req.ApplicationId, len(ingressList.Items))

	result := make([]types.IngressListItem, 0, len(ingressList.Items))
	for _, ingressInfo := range ingressList.Items {
		result = append(result, types.IngressListItem{
			Name:              ingressInfo.Name,
			Namespace:         ingressInfo.Namespace,
			IngressClass:      ingressInfo.IngressClass,
			Hosts:             ingressInfo.Hosts,
			Address:           ingressInfo.Address,
			Ports:             ingressInfo.Ports,
			Age:               ingressInfo.Age,
			CreationTimestamp: ingressInfo.CreationTimestamp,
			Labels:            ingressInfo.Labels,
		})
	}

	return result, nil
}

// getVersionServices 获取所有版本关联的 Service Names
func (l *ApplicationIngressGetLogic) getVersionServices(appId, workloadId uint64) ([]string, string, *managerservice.GetOnecProjectWorkspaceByIdResp, error) {
	app, err := l.svcCtx.ManagerRpc.ApplicationGetById(l.ctx, &managerservice.GetOnecProjectApplicationByIdReq{
		Id: appId,
	})
	if err != nil {
		return nil, "", nil, fmt.Errorf("获取应用信息失败: %w", err)
	}

	workspace, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{
		Id: workloadId,
	})
	if err != nil {
		return nil, "", nil, fmt.Errorf("获取工作空间失败: %w", err)
	}

	allVersions, err := l.svcCtx.ManagerRpc.VersionSearch(l.ctx, &managerservice.SearchOnecProjectVersionReq{
		ApplicationId: appId,
	})
	if err != nil {
		return nil, "", nil, fmt.Errorf("获取版本列表失败: %w", err)
	}

	if len(allVersions.Data) == 0 {
		return []string{}, "", workspace, nil
	}

	firstVersion, err := l.svcCtx.ManagerRpc.VersionDetail(l.ctx, &managerservice.GetOnecProjectVersionDetailReq{
		Id: allVersions.Data[0].Id,
	})
	if err != nil {
		return nil, "", nil, fmt.Errorf("获取版本详情失败: %w", err)
	}

	namespace := firstVersion.Namespace

	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, firstVersion.ClusterUuid)
	if err != nil {
		return nil, "", nil, fmt.Errorf("获取集群客户端失败: %w", err)
	}

	resourceType := app.Data.ResourceType
	serviceNameMap := make(map[string]bool) // 用于去重

	for _, version := range allVersions.Data {
		var podLabels map[string]string
		var err error

		switch resourceType {
		case "Deployment", "deployment":
			podLabels, err = client.Deployment().GetPodLabels(namespace, version.ResourceName)
		case "StatefulSet", "statefulset":
			podLabels, err = client.StatefulSet().GetPodLabels(namespace, version.ResourceName)
		case "DaemonSet", "daemonset":
			podLabels, err = client.DaemonSet().GetPodLabels(namespace, version.ResourceName)
		default:
			l.Logger.Errorf("不支持的资源类型: %s，跳过版本 %s", resourceType, version.ResourceName)
			continue
		}

		if err != nil {
			l.Logger.Errorf("获取版本 %s (ID: %d) 的 Pod Labels 失败: %v", version.ResourceName, version.Id, err)
			continue
		}

		if len(podLabels) == 0 {
			l.Logger.Errorf("版本 %s (ID: %d) 没有 Pod Labels", version.ResourceName, version.Id)
			continue
		}

		services, err := client.Services().GetServicesByLabels(namespace, podLabels)
		if err != nil {
			l.Logger.Errorf("查询版本 %s 的 Services 失败: %v", version.ResourceName, err)
			continue
		}

		for _, svc := range services {
			serviceNameMap[svc.Name] = true
		}

		l.Logger.Infof("版本 %s (Labels: %v) 找到 %d 个匹配的 Services",
			version.ResourceName, podLabels, len(services))
	}

	serviceNames := make([]string, 0, len(serviceNameMap))
	for name := range serviceNameMap {
		serviceNames = append(serviceNames, name)
	}

	l.Logger.Infof("应用 %d 共找到 %d 个关联的 Services: %v", appId, len(serviceNames), serviceNames)

	return serviceNames, namespace, workspace, nil
}
