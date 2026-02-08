package core

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/yanshicheng/kube-nova/common/utils"

	"github.com/zeromicro/go-zero/core/logx"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type ApplicationServiceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 服务级 service 创建
func NewApplicationServiceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApplicationServiceLogic {
	return &ApplicationServiceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ApplicationServiceLogic) ApplicationService(req *types.ApplicationServiceRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}

	// 参数校验
	if req.IsAllSvc && req.IsAppSvc {
		return "", fmt.Errorf("IsAllSvc 和 IsAppSvc 不能同时为 true")
	}

	var selector map[string]string
	var serviceLevel string

	// 根据不同情况确定 selector
	switch {
	case req.IsAllSvc:
		// 全部版本
		selector, err = l.getAppLabels(req.ApplicationId)
		if err != nil {
			return "", fmt.Errorf("获取应用共同 labels 失败: %w", err)
		}
		serviceLevel = "全部版本"

	case req.IsAppSvc:
		// 应用级别
		if len(req.Labels) == 0 {
			return "", fmt.Errorf("应用级 Service 必须提供 labels")
		}
		selector = req.Labels
		serviceLevel = "应用级别"

	default:
		// 版本级别
		if len(req.Selector) == 0 {
			return "", fmt.Errorf("版本级 Service 必须提供 selector")
		}
		selector = req.Selector
		serviceLevel = "版本级别"
	}

	// 将确定的 selector 赋值回 req
	req.Selector = selector

	// 获取工作空间信息
	workspace, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{
		Id: req.WorkloadId,
	})
	if err != nil {
		return "", fmt.Errorf("获取工作空间失败: %w", err)
	}

	// 获取 K8s 客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workspace.Data.ClusterUuid)
	if err != nil {
		return "", fmt.Errorf("获取集群客户端失败: %w", err)
	}

	// 构建 K8s Service 对象
	k8sService := l.buildK8sService(req, workspace.Data.Namespace)

	// 获取 app
	app, err := l.svcCtx.ManagerRpc.ApplicationGetById(l.ctx, &managerservice.GetOnecProjectApplicationByIdReq{
		Id: req.ApplicationId,
	})
	if err != nil {
		return "", fmt.Errorf("获取应用失败: %w", err)
	}

	// 获取 cluster
	projectCluster, err := l.svcCtx.ManagerRpc.ProjectClusterGetById(l.ctx, &managerservice.GetOnecProjectClusterByIdReq{
		Id: workspace.Data.ProjectClusterId,
	})
	if err != nil {
		return "", fmt.Errorf("获取项目集群失败: %w", err)
	}

	// 获取 project
	project, err := l.svcCtx.ManagerRpc.ProjectGetById(l.ctx, &managerservice.GetOnecProjectByIdReq{
		Id: projectCluster.Data.ProjectId,
	})
	if err != nil {
		return "", fmt.Errorf("获取项目失败: %w", err)
	}

	// 注入注解
	utils.AddAnnotations(&k8sService.ObjectMeta, &utils.AnnotationsInfo{
		ServiceName:   k8sService.Name,
		ProjectName:   project.Data.Name,
		WorkspaceName: workspace.Data.Name,
		ProjectUuid:   project.Data.Uuid,
		OwnedByApp:    app.Data.NameCn,
	})

	// 构建端口信息用于审计
	portInfo := l.buildPortInfo(req.Ports)

	// 构建选择器信息用于审计
	selectorInfo := l.buildSelectorInfo(req.Selector)

	// 创建 Service
	serviceOperator := client.Services()
	createdService, createErr := serviceOperator.Create(k8sService)
	if createErr != nil {
		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ApplicationId: req.ApplicationId,
			Title:         "创建应用 Service",
			ActionDetail:  fmt.Sprintf("用户 %s 为应用 %s 创建 Service %s 失败, 级别: %s, 类型: %s, 端口: %s, 选择器: %s, 错误原因: %v", username, app.Data.NameCn, req.Name, serviceLevel, req.Type, portInfo, selectorInfo, createErr),
			Status:        0,
		})
		return "", fmt.Errorf("创建 Service 失败: %w", createErr)
	}

	// 记录成功的审计日志
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ApplicationId: req.ApplicationId,
		Title:         "创建应用 Service",
		ActionDetail:  fmt.Sprintf("用户 %s 为应用 %s 成功创建 Service %s, 级别: %s, 类型: %s, 端口: %s, 选择器: %s", username, app.Data.NameCn, createdService.Name, serviceLevel, req.Type, portInfo, selectorInfo),
		Status:        1,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	return fmt.Sprintf("Service %s 创建成功", createdService.Name), nil
}

// buildPortInfo 构建端口信息字符串用于审计日志
func (l *ApplicationServiceLogic) buildPortInfo(ports []types.ServicePort) string {
	if len(ports) == 0 {
		return "无"
	}
	var portStrs []string
	for _, p := range ports {
		portStr := fmt.Sprintf("%d→%s", p.Port, p.TargetPort)
		if p.Name != "" {
			portStr = fmt.Sprintf("%s:%s", p.Name, portStr)
		}
		if p.NodePort > 0 {
			portStr = fmt.Sprintf("%s(NodePort:%d)", portStr, p.NodePort)
		}
		portStrs = append(portStrs, portStr)
	}
	return strings.Join(portStrs, ", ")
}

// buildSelectorInfo 构建选择器信息字符串用于审计日志
func (l *ApplicationServiceLogic) buildSelectorInfo(selector map[string]string) string {
	if len(selector) == 0 {
		return "无"
	}
	var selectorStrs []string
	for k, v := range selector {
		selectorStrs = append(selectorStrs, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(selectorStrs, ", ")
}

// buildK8sService 构建 K8s Service 对象
func (l *ApplicationServiceLogic) buildK8sService(req *types.ApplicationServiceRequest, namespace string) *corev1.Service {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        req.Name,
			Namespace:   namespace,
			Labels:      req.Labels,
			Annotations: req.Annotations,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceType(req.Type),
			Selector: req.Selector,
		},
	}

	// 端口配置
	if len(req.Ports) > 0 {
		ports := make([]corev1.ServicePort, 0, len(req.Ports))
		for _, p := range req.Ports {
			port := corev1.ServicePort{
				Name:     p.Name,
				Protocol: corev1.Protocol(p.Protocol),
				Port:     p.Port,
			}

			// 处理 TargetPort
			if val, err := strconv.Atoi(p.TargetPort); err == nil {
				port.TargetPort = intstr.FromInt(val)
			} else {
				port.TargetPort = intstr.FromString(p.TargetPort)
			}

			// NodePort
			if p.NodePort > 0 && (req.Type == "NodePort" || req.Type == "LoadBalancer") {
				port.NodePort = p.NodePort
			}

			// AppProtocol
			if p.AppProtocol != "" {
				port.AppProtocol = &p.AppProtocol
			}

			ports = append(ports, port)
		}
		service.Spec.Ports = ports
	}

	// IP 配置
	if req.ClusterIP != "" {
		service.Spec.ClusterIP = req.ClusterIP
	}

	if len(req.ClusterIPs) > 0 {
		service.Spec.ClusterIPs = req.ClusterIPs
	}

	if len(req.IpFamilies) > 0 {
		ipFamilies := make([]corev1.IPFamily, 0, len(req.IpFamilies))
		for _, family := range req.IpFamilies {
			ipFamilies = append(ipFamilies, corev1.IPFamily(family))
		}
		service.Spec.IPFamilies = ipFamilies
	}

	if req.IpFamilyPolicy != "" {
		policy := corev1.IPFamilyPolicyType(req.IpFamilyPolicy)
		service.Spec.IPFamilyPolicy = &policy
	}

	if len(req.ExternalIPs) > 0 {
		service.Spec.ExternalIPs = req.ExternalIPs
	}

	if req.Type == "ExternalName" && req.ExternalName != "" {
		service.Spec.ExternalName = req.ExternalName
	}

	// 负载均衡器配置
	if req.Type == "LoadBalancer" {
		if req.LoadBalancerIP != "" {
			service.Spec.LoadBalancerIP = req.LoadBalancerIP
		}

		if req.LoadBalancerClass != "" {
			service.Spec.LoadBalancerClass = &req.LoadBalancerClass
		}

		if len(req.LoadBalancerSourceRanges) > 0 {
			service.Spec.LoadBalancerSourceRanges = req.LoadBalancerSourceRanges
		}

		if !req.AllocateLoadBalancerNodePorts {
			allocate := false
			service.Spec.AllocateLoadBalancerNodePorts = &allocate
		}

		if req.HealthCheckNodePort > 0 {
			service.Spec.HealthCheckNodePort = req.HealthCheckNodePort
		}
	}

	// 流量策略
	if req.ExternalTrafficPolicy != "" && (req.Type == "NodePort" || req.Type == "LoadBalancer") {
		policy := corev1.ServiceExternalTrafficPolicyType(req.ExternalTrafficPolicy)
		service.Spec.ExternalTrafficPolicy = policy
	}

	if req.InternalTrafficPolicy != "" && req.Type != "ExternalName" {
		policy := corev1.ServiceInternalTrafficPolicyType(req.InternalTrafficPolicy)
		service.Spec.InternalTrafficPolicy = &policy
	}

	// 会话保持
	if req.SessionAffinity != "" {
		service.Spec.SessionAffinity = corev1.ServiceAffinity(req.SessionAffinity)
	} else {
		service.Spec.SessionAffinity = corev1.ServiceAffinityNone
	}

	if req.SessionAffinity == "ClientIP" && req.SessionAffinityConfig.ClientIP.TimeoutSeconds > 0 {
		service.Spec.SessionAffinityConfig = &corev1.SessionAffinityConfig{
			ClientIP: &corev1.ClientIPConfig{
				TimeoutSeconds: &req.SessionAffinityConfig.ClientIP.TimeoutSeconds,
			},
		}
	}

	// 发布未就绪地址
	if req.PublishNotReadyAddresses {
		service.Spec.PublishNotReadyAddresses = true
	}

	return service
}

func (l *ApplicationServiceLogic) getAppLabels(appId uint64) (map[string]string, error) {
	// 获取应用信息
	app, err := l.svcCtx.ManagerRpc.ApplicationGetById(l.ctx, &managerservice.GetOnecProjectApplicationByIdReq{
		Id: appId,
	})
	if err != nil {
		return nil, fmt.Errorf("获取应用信息失败: %w", err)
	}

	// 获取所有版本
	allVersions, err := l.svcCtx.ManagerRpc.VersionSearch(l.ctx, &managerservice.SearchOnecProjectVersionReq{
		ApplicationId: appId,
	})
	if err != nil {
		return nil, fmt.Errorf("获取版本列表失败: %w", err)
	}

	if len(allVersions.Data) == 0 {
		return nil, fmt.Errorf("应用没有任何版本")
	}

	// 获取第一个版本的详细信息
	firstVersion, err := l.svcCtx.ManagerRpc.VersionDetail(l.ctx, &managerservice.GetOnecProjectVersionDetailReq{
		Id: allVersions.Data[0].Id,
	})
	if err != nil {
		return nil, fmt.Errorf("获取版本详情失败: %w", err)
	}

	// 获取 K8s 客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, firstVersion.ClusterUuid)
	if err != nil {
		return nil, fmt.Errorf("获取集群客户端失败: %w", err)
	}

	// 检查所有版本的 app label 是否一致
	var appLabel string
	resourceType := strings.ToLower(app.Data.ResourceType)

	for i, version := range allVersions.Data {
		var labels map[string]string
		var err error

		// 根据资源类型获取 Pod Labels
		switch resourceType {
		case "deployment":
			labels, err = client.Deployment().GetPodLabels(firstVersion.Namespace, version.ResourceName)
		case "statefulset":
			labels, err = client.StatefulSet().GetPodLabels(firstVersion.Namespace, version.ResourceName)
		case "daemonset":
			labels, err = client.DaemonSet().GetPodLabels(firstVersion.Namespace, version.ResourceName)
		default:
			return nil, fmt.Errorf("不支持的资源类型: %s", resourceType)
		}

		if err != nil {
			return nil, fmt.Errorf("获取版本 %s 的 Pod Labels 失败: %w", version.ResourceName, err)
		}

		// 检查是否有 app label
		currentAppLabel, exists := labels["app"]
		if !exists {
			return nil, fmt.Errorf("版本 %s 没有 app label", version.ResourceName)
		}

		// 第一个版本记录 app label 值
		if i == 0 {
			appLabel = currentAppLabel
		} else {
			// 后续版本检查 app label 是否一致
			if currentAppLabel != appLabel {
				return nil, fmt.Errorf("版本 app label 不一致: %s vs %s", appLabel, currentAppLabel)
			}
		}
	}

	// 返回 app label
	return map[string]string{
		"app": appLabel,
	}, nil
}
