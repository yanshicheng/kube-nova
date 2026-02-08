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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建 Service
func NewServiceCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceCreateLogic {
	return &ServiceCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceCreateLogic) ServiceCreate(req *types.ServiceRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}

	// 获取集群以及命名空间
	workloadInfo, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{Id: req.WorkloadId})
	if err != nil {
		l.Errorf("获取项目工作空间详情失败: %v", err)
		return "", fmt.Errorf("获取项目工作空间详情失败")
	}

	// 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workloadInfo.Data.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	// 初始化 Service 客户端
	serviceClient := client.Services()

	// 构建 Service 对象
	service := l.buildServiceFromRequest(req, workloadInfo.Data.Namespace)

	// 构建端口信息用于审计
	portInfo := l.buildPortInfo(req.Ports)

	// 构建选择器信息用于审计
	selectorInfo := l.buildSelectorInfo(req.Selector)

	// 获取项目详情
	projectDetail, err := l.svcCtx.ManagerRpc.GetClusterNsDetail(l.ctx, &managerservice.GetClusterNsDetailReq{
		ClusterUuid: workloadInfo.Data.ClusterUuid,
		Namespace:   workloadInfo.Data.Namespace,
	})
	if err != nil {
		l.Errorf("获取项目详情失败: %v", err)
		return "", fmt.Errorf("获取项目详情失败")
	} else {
		// 注入注解
		utils.AddAnnotations(&service.ObjectMeta, &utils.AnnotationsInfo{
			ServiceName:   service.Name,
			ProjectName:   projectDetail.ProjectNameCn,
			WorkspaceName: projectDetail.WorkspaceNameCn,
			ProjectUuid:   projectDetail.ProjectUuid,
		})
	}

	// 创建 Service
	_, createErr := serviceClient.Create(service)
	if createErr != nil {
		l.Errorf("创建 Service 失败: %v", createErr)
		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			WorkspaceId:  req.WorkloadId,
			Title:        "创建 Service",
			ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 创建 Service %s 失败, 类型: %s, 端口: %s, 选择器: %s, 错误原因: %v", username, workloadInfo.Data.Namespace, req.Name, req.Type, portInfo, selectorInfo, createErr),
			Status:       0,
		})
		return "", fmt.Errorf("创建 Service 失败: %v", createErr)
	}

	// 记录成功的审计日志
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		WorkspaceId:  req.WorkloadId,
		Title:        "创建 Service",
		ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 成功创建 Service %s, 类型: %s, 端口: %s, 选择器: %s", username, workloadInfo.Data.Namespace, req.Name, req.Type, portInfo, selectorInfo),
		Status:       1,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	l.Infof("成功创建 Service: %s", req.Name)
	return "创建成功", nil
}

// buildPortInfo 构建端口信息字符串用于审计日志
func (l *ServiceCreateLogic) buildPortInfo(ports []types.ServicePort) string {
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
func (l *ServiceCreateLogic) buildSelectorInfo(selector map[string]string) string {
	if len(selector) == 0 {
		return "无"
	}
	var selectorStrs []string
	for k, v := range selector {
		selectorStrs = append(selectorStrs, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(selectorStrs, ", ")
}

// buildServiceFromRequest 从请求构建 K8s Service 对象
func (l *ServiceCreateLogic) buildServiceFromRequest(req *types.ServiceRequest, namespace string) *corev1.Service {
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
			if p.TargetPort != "" {
				if val, err := strconv.Atoi(p.TargetPort); err == nil {
					port.TargetPort = intstr.FromInt(val)
				} else {
					port.TargetPort = intstr.FromString(p.TargetPort)
				}
			}

			// NodePort
			if p.NodePort > 0 {
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
	if req.ExternalTrafficPolicy != "" {
		policy := corev1.ServiceExternalTrafficPolicyType(req.ExternalTrafficPolicy)
		service.Spec.ExternalTrafficPolicy = policy
	}

	if req.InternalTrafficPolicy != "" {
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
