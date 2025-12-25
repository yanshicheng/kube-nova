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

type ApplicationServiceGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// versionLabelInfo 版本标签信息
type versionLabelInfo struct {
	VersionId     uint64
	VersionName   string
	VersionNumber string
	Labels        map[string]string
	IsAppOnly     bool
}

func NewApplicationServiceGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApplicationServiceGetLogic {
	return &ApplicationServiceGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ApplicationServiceGetLogic) ApplicationServiceGet(req *types.GetAppServicesRequest) (resp []types.ApplicationServiceListResponse, err error) {
	//  获取所有版本的 labels 信息
	versionLabels, workspace, err := l.getVersionLabels(req.ApplicationId, req.WorkloadId)
	if err != nil {
		return nil, err
	}

	if len(versionLabels) == 0 {
		return []types.ApplicationServiceListResponse{}, nil
	}

	// 获取 K8s 客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workspace.Data.ClusterUuid)
	if err != nil {
		return nil, fmt.Errorf("获取集群客户端失败: %w", err)
	}

	// 创建版本信息映射（用于后续查找）
	versionMap := make(map[uint64]versionLabelInfo)
	var appOnlyVersion *versionLabelInfo

	for _, vl := range versionLabels {
		if vl.IsAppOnly {
			appOnlyVersion = &vl
		} else {
			versionMap[vl.VersionId] = vl
		}
	}

	// 查询所有可能的 Service
	serviceMap := make(map[string]*types.ApplicationServiceListResponse)

	// 先用 app label 查询服务级别的 Service（匹配所有版本）
	if appOnlyVersion != nil {
		services, err := client.Services().GetServicesByLabels(workspace.Data.Namespace, appOnlyVersion.Labels)
		if err != nil {
			return nil, fmt.Errorf("查询 Service 失败: %w", err)
		}

		for _, svc := range services {
			// 检查是否只有 app label（没有版本标识）
			if l.isServiceLevelService(&svc) {
				serviceMap[svc.Name] = l.convertToResponse(&svc, appOnlyVersion)
			}
		}
	}

	// 为每个具体版本查询版本级别的 Service
	for _, vl := range versionMap {
		services, err := client.Services().GetServicesByLabels(workspace.Data.Namespace, vl.Labels)
		if err != nil {
			l.Logger.Errorf("查询版本 %s (ID: %d) 的 Service 失败: %v", vl.VersionName, vl.VersionId, err)
			continue
		}

		for _, svc := range services {
			// 检查 Service 的 selector 是否完全匹配这个版本的 labels
			if l.selectorMatchesLabels(svc.Selector, vl.Labels) {
				// 如果已存在（被服务级别查询到），跳过
				if _, exists := serviceMap[svc.Name]; !exists {
					serviceMap[svc.Name] = l.convertToResponse(&svc, &vl)
				}
			}
		}
	}

	// 换为数组返回
	resp = make([]types.ApplicationServiceListResponse, 0, len(serviceMap))
	for _, svcResp := range serviceMap {
		resp = append(resp, *svcResp)
	}

	return resp, nil
}

// isServiceLevelService 判断 Service 是否是服务级别（只有 app label，没有版本标识）
func (l *ApplicationServiceGetLogic) isServiceLevelService(svc *types2.ServiceInfo) bool {
	// 如果 Service 没有 selector，认为是服务级别
	if len(svc.Selector) == 0 {
		return true
	}

	// 检查是否包含版本相关的 label
	for key := range svc.Selector {
		if key == "version" || strings.Contains(key, "version") {
			return false
		}
	}

	// 没有版本标识，是服务级别
	return true
}

// selectorMatchesLabels 检查 selector 是否完全匹配 labels
func (l *ApplicationServiceGetLogic) selectorMatchesLabels(selector, labels map[string]string) bool {
	// selector 的每个键值对都必须在 labels 中存在且值相同
	for key, selectorValue := range selector {
		labelValue, exists := labels[key]
		if !exists || labelValue != selectorValue {
			return false
		}
	}
	return true
}

// getVersionLabels 获取所有版本的 labels 信息
func (l *ApplicationServiceGetLogic) getVersionLabels(appId, workloadId uint64) ([]versionLabelInfo, *managerservice.GetOnecProjectWorkspaceByIdResp, error) {
	// 获取应用信息
	app, err := l.svcCtx.ManagerRpc.ApplicationGetById(l.ctx, &managerservice.GetOnecProjectApplicationByIdReq{
		Id: appId,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("获取应用信息失败: %w", err)
	}

	// 获取工作空间信息
	workspace, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{
		Id: workloadId,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("获取工作空间失败: %w", err)
	}

	// 获取所有版本
	allVersions, err := l.svcCtx.ManagerRpc.VersionSearch(l.ctx, &managerservice.SearchOnecProjectVersionReq{
		ApplicationId: appId,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("获取版本列表失败: %w", err)
	}

	if len(allVersions.Data) == 0 {
		return []versionLabelInfo{}, workspace, nil
	}

	// 获取第一个版本的详细信息（用于获取 cluster 和 namespace）
	firstVersion, err := l.svcCtx.ManagerRpc.VersionDetail(l.ctx, &managerservice.GetOnecProjectVersionDetailReq{
		Id: allVersions.Data[0].Id,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("获取版本详情失败: %w", err)
	}

	// 获取 K8s 客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, firstVersion.ClusterUuid)
	if err != nil {
		return nil, nil, fmt.Errorf("获取集群客户端失败: %w", err)
	}

	// 收集所有版本的 labels
	resourceType := strings.ToLower(app.Data.ResourceType)
	versionLabels := make([]versionLabelInfo, 0)

	var commonAppLabel string
	appLabelConsistent := true
	hasAppLabel := false

	for i, version := range allVersions.Data {
		var podLabels map[string]string
		var err error

		// 根据资源类型获取 Pod Labels
		switch resourceType {
		case "deployment":
			podLabels, err = client.Deployment().GetPodLabels(firstVersion.Namespace, version.ResourceName)
		case "statefulset":
			podLabels, err = client.StatefulSet().GetPodLabels(firstVersion.Namespace, version.ResourceName)
		case "daemonset":
			podLabels, err = client.DaemonSet().GetPodLabels(firstVersion.Namespace, version.ResourceName)
		default:
			return nil, nil, fmt.Errorf("不支持的资源类型: %s", resourceType)
		}

		// 如果获取失败，记录日志但继续处理
		if err != nil {
			l.Logger.Errorf("获取版本 %s (ID: %d) 的 Pod Labels 失败: %v", version.ResourceName, version.Id, err)
			appLabelConsistent = false
			continue
		}

		if len(podLabels) == 0 {
			l.Logger.Errorf("版本 %s (ID: %d) 没有 Pod Labels", version.ResourceName, version.Id)
			appLabelConsistent = false
			continue
		}

		versionLabels = append(versionLabels, versionLabelInfo{
			VersionId:     version.Id,
			VersionName:   version.Version,
			VersionNumber: fmt.Sprintf("%d", version.Version),
			Labels:        podLabels,
			IsAppOnly:     false,
		})

		// 检查 app label 的一致性
		if appLabel, exists := podLabels["app"]; exists {
			hasAppLabel = true
			if i == 0 {
				commonAppLabel = appLabel
			} else if appLabel != commonAppLabel {
				appLabelConsistent = false
			}
		} else {
			appLabelConsistent = false
		}
	}

	// 如果所有版本的 app label 一致，添加通用的 app label（version = 0 表示 "all"）
	if appLabelConsistent && hasAppLabel && commonAppLabel != "" {
		versionLabels = append(versionLabels, versionLabelInfo{
			VersionId:     0,
			VersionName:   "全部版本",
			VersionNumber: "0",
			Labels: map[string]string{
				"app": commonAppLabel,
			},
			IsAppOnly: true,
		})
	}

	return versionLabels, workspace, nil
}

// convertToResponse 转换 ServiceInfo 为 ApplicationServiceListResponse
func (l *ApplicationServiceGetLogic) convertToResponse(svc *types2.ServiceInfo, versionInfo *versionLabelInfo) *types.ApplicationServiceListResponse {
	return &types.ApplicationServiceListResponse{
		Name:                  svc.Name,
		Version:               versionInfo.VersionId,
		VersionName:           versionInfo.VersionName,
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
	}
}
