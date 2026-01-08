package operator

import (
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	corev1 "k8s.io/api/core/v1"
)

// getWorkloadResourceSummary 获取工作负载资源摘要的通用实现
// 适用于 Deployment、StatefulSet、DaemonSet、CronJob
func getWorkloadResourceSummary(
	namespace string,
	selectorLabels map[string]string,
	domainSuffix string,
	nodeLb []string, // 新增：节点负载均衡器地址，多个地址用逗号分隔
	podOp types.PodOperator,
	svcOp types.ServiceOperator,
	ingressOp types.IngressOperator,
) (*types.WorkloadResourceSummary, error) {
	summary := &types.WorkloadResourceSummary{
		IngressDomains: make([]string, 0),
		Service: types.ServiceAccessSummary{
			InternalAccessList: make([]string, 0),
			ExternalAccessList: make([]string, 0),
			NodePortList:       make([]string, 0),
		},
		IsAppSelector: false, // 默认为 false
	}

	// 1. 获取关联的 Pod
	pods, err := getRelatedPodsForWorkload(namespace, selectorLabels, podOp)
	if err != nil {
		return nil, fmt.Errorf("获取关联 Pod 失败: %v", err)
	}

	summary.PodCount = len(pods)
	summary.AbnormalPodCount = countAbnormalPodsForWorkload(pods)

	// 2. 优化：智能查询关联的 Service
	services, isAppSelector, err := getRelatedServicesForWorkloadOptimized(namespace, selectorLabels, svcOp)
	if err != nil {
		return nil, fmt.Errorf("获取关联 Service 失败: %v", err)
	}

	summary.ServiceCount = len(services)
	summary.IsAppSelector = isAppSelector // 设置标志

	// 3. 处理 Service 信息
	processServiceInfoForWorkload(services, domainSuffix, nodeLb, summary)

	// 4. 优化：只查询与 Service 相关的 Ingress
	ingresses, err := getRelatedIngressesForWorkloadOptimized(namespace, services, ingressOp)
	if err != nil {
		// 记录警告但不中断流程
		fmt.Printf("警告: 获取关联 Ingress 失败: %v, 跳过 Ingress 信息\n", err)
		summary.IngressCount = 0
		summary.IngressDomains = []string{}
	} else {
		summary.IngressCount = len(ingresses)
		summary.IngressDomains = extractIngressDomainsForWorkload(ingresses)
	}

	return summary, nil
}

// getRelatedServicesForWorkloadOptimized 智能查询关联的 Service
// 同时使用 app 标签和所有标签查询，合并结果去重
// 返回值：services, isAppSelector, error
// isAppSelector 表示是否通过 app 标签查询到了 Service
func getRelatedServicesForWorkloadOptimized(
	namespace string,
	selectorLabels map[string]string,
	svcOp types.ServiceOperator,
) ([]types.ServiceInfo, bool, error) {
	// 用于去重的 map
	svcMap := make(map[string]types.ServiceInfo)
	isAppSelector := false

	// 需求1: 优先用 app 标签查询（如果存在）
	if appLabel, hasAppLabel := selectorLabels["app"]; hasAppLabel && appLabel != "" {
		appLabels := map[string]string{"app": appLabel}
		appServices, err := svcOp.GetServicesByLabels(namespace, appLabels)

		if err != nil {
			fmt.Printf("警告: 使用 app 标签查询 Service 失败: %v\n", err)
		} else if len(appServices) > 0 {
			// 将结果加入 map（去重）
			for _, svc := range appServices {
				svcMap[svc.Name] = svc
			}
			isAppSelector = true // 标记为通过 app 标签查询到了 Service
			fmt.Printf("通过 app 标签 (app=%s) 查询到 %d 个 Service\n", appLabel, len(appServices))
		}
	}

	// 需求2: 使用所有标签查询
	allServices, err := svcOp.GetServicesByLabels(namespace, selectorLabels)
	if err != nil {
		// 如果用所有标签查询失败，但之前 app 标签查到了结果，仍然返回 app 标签的结果
		if len(svcMap) > 0 {
			fmt.Printf("警告: 使用所有标签查询 Service 失败: %v, 返回 app 标签查询结果\n", err)
			result := make([]types.ServiceInfo, 0, len(svcMap))
			for _, svc := range svcMap {
				result = append(result, svc)
			}
			return result, isAppSelector, nil
		}
		return nil, false, fmt.Errorf("查询 Service 失败: %v", err)
	}

	// 将所有标签查询的结果也加入 map（去重）
	for _, svc := range allServices {
		svcMap[svc.Name] = svc
	}

	fmt.Printf("通过所有标签查询到 %d 个 Service，合并后共 %d 个 Service\n",
		len(allServices), len(svcMap))

	// 转换为切片返回
	result := make([]types.ServiceInfo, 0, len(svcMap))
	for _, svc := range svcMap {
		result = append(result, svc)
	}

	return result, isAppSelector, nil
}

// getRelatedIngressesForWorkloadOptimized 获取与 Service 相关联的 Ingress
func getRelatedIngressesForWorkloadOptimized(
	namespace string,
	services []types.ServiceInfo,
	ingressOp types.IngressOperator,
) ([]types.IngressInfo, error) {
	// 如果没有 Service，直接返回空列表
	if len(services) == 0 {
		return []types.IngressInfo{}, nil
	}

	// 构建 Service 名称集合
	svcNames := make(map[string]bool)
	for _, svc := range services {
		svcNames[svc.Name] = true
	}

	// 获取命名空间下的所有 Ingress（不再逐个调用 GetDetail）
	ingressList, err := ingressOp.List(namespace, "", "")
	if err != nil {
		return nil, fmt.Errorf("获取 Ingress 列表失败: %v", err)
	}

	// 过滤出引用了这些 Service 的 Ingress
	relatedIngresses := make([]types.IngressInfo, 0)

	for _, ingress := range ingressList.Items {
		// 由于 IngressInfo 中没有详细的 Rules 信息，我们需要调用 GetDetail
		// 但是我们可以添加超时保护和错误处理
		detail, err := ingressOp.GetDetail(namespace, ingress.Name)
		if err != nil {
			// 如果单个 Ingress 获取失败，记录警告但继续处理其他 Ingress
			fmt.Printf("警告: 获取 Ingress %s/%s 详情失败: %v, 跳过该 Ingress\n",
				namespace, ingress.Name, err)
			continue
		}

		// 检查规则中是否引用了这些 Service
		isRelated := checkIngressRelatedToServices(detail, svcNames)

		if isRelated {
			relatedIngresses = append(relatedIngresses, ingress)
		}
	}

	return relatedIngresses, nil
}

// checkIngressRelatedToServices 检查 Ingress 是否关联到指定的 Service
func checkIngressRelatedToServices(detail *types.IngressDetail, svcNames map[string]bool) bool {
	// 检查规则中的后端 Service
	for _, rule := range detail.Rules {
		for _, path := range rule.Paths {
			if svcNames[path.Backend.ServiceName] {
				return true
			}
		}
	}

	// 检查默认后端
	if detail.DefaultBackend != nil && svcNames[detail.DefaultBackend.ServiceName] {
		return true
	}

	return false
}

// getRelatedPodsForWorkload 获取与工作负载相关联的 Pod（通用实现）
func getRelatedPodsForWorkload(
	namespace string,
	selectorLabels map[string]string,
	podOp types.PodOperator,
) ([]*corev1.Pod, error) {
	// 构建标签选择器
	labelSelector := buildLabelSelectorForWorkload(selectorLabels)

	// 使用 Pod 操作器获取 Pod 列表
	req := types.ListRequest{
		Labels:   labelSelector,
		Page:     1,
		PageSize: 1000, // 设置一个较大的值获取所有 Pod
	}

	podList, err := podOp.List(namespace, req)
	if err != nil {
		return nil, err
	}

	// 将 PodDetailInfo 转换为 corev1.Pod
	pods := make([]*corev1.Pod, 0, len(podList.Items))
	for _, podDetail := range podList.Items {
		pod := &corev1.Pod{
			Status: corev1.PodStatus{
				Phase: corev1.PodPhase(podDetail.Status),
			},
		}
		pods = append(pods, pod)
	}

	return pods, nil
}

// countAbnormalPodsForWorkload 统计异常的 Pod 数量（通用实现）
func countAbnormalPodsForWorkload(pods []*corev1.Pod) int {
	abnormalCount := 0
	for _, pod := range pods {
		// 只有 Running 和 Succeeded 被视为正常状态
		if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodSucceeded {
			abnormalCount++
		}
	}
	return abnormalCount
}

// processServiceInfoForWorkload 处理 Service 信息（通用实现）
func processServiceInfoForWorkload(
	services []types.ServiceInfo,
	domainSuffix string,
	nodeLb []string, // 节点负载均衡器地址，多个地址用逗号分隔，如: "192.168.1.100,192.168.1.101"
	summary *types.WorkloadResourceSummary,
) {
	internalAccessList := make([]string, 0)
	externalAccessList := make([]string, 0)
	nodePortList := make([]string, 0)

	// 解析 nodeLb 地址列表
	nodeLbAddresses := nodeLb

	for _, svc := range services {
		// 解析端口信息
		ports := parseServicePortsForWorkload(svc.Ports)

		if svc.ClusterIP != "" && svc.ClusterIP != "None" {
			for _, port := range ports {
				if domainSuffix != "" {
					protocol := "http"
					if port.Port == 443 || strings.Contains(strings.ToLower(port.Protocol), "https") {
						protocol = "https"
					}
					domain := fmt.Sprintf("%s.%s.%s.%s", svc.Name, svc.Namespace, "svc", domainSuffix)
					internalAccessList = append(internalAccessList,
						fmt.Sprintf("%s://%s:%d", protocol, domain, port.Port))
				} else {
					protocol := "http"
					if port.Port == 443 {
						protocol = "https"
					}
					internalAccessList = append(internalAccessList,
						fmt.Sprintf("%s://%s:%d", protocol, svc.ClusterIP, port.Port))
				}
			}
		}

		// 再处理不同类型的特定信息
		switch svc.Type {
		case "NodePort":
			// 生成 NodePort 访问地址列表
			for _, port := range ports {
				if port.NodePort > 0 {
					// 如果提供了 nodeLb 地址，为每个地址生成访问地址
					if len(nodeLbAddresses) > 0 {
						for _, lbAddr := range nodeLbAddresses {
							nodePortList = append(nodePortList,
								fmt.Sprintf("%s:%d", lbAddr, port.NodePort))
						}
					} else {
						// 如果没有提供 nodeLb，使用占位符
						nodePortList = append(nodePortList,
							fmt.Sprintf("<NodeIP>:%d", port.NodePort))
					}
				}
			}

		case "LoadBalancer":
			// 生成 LoadBalancer 外部访问地址
			if svc.ExternalIP != "" && svc.ExternalIP != "<pending>" && svc.ExternalIP != "<none>" {
				for _, port := range ports {
					protocol := "http"
					if port.Port == 443 || strings.Contains(strings.ToLower(port.Protocol), "https") {
						protocol = "https"
					}
					externalAccessList = append(externalAccessList,
						fmt.Sprintf("%s://%s:%d", protocol, svc.ExternalIP, port.Port))
				}
			}
		}
	}

	summary.Service.InternalAccessList = internalAccessList
	summary.Service.ExternalAccessList = externalAccessList
	summary.Service.NodePortList = nodePortList
}

// parseNodeLbAddresses 解析节点负载均衡器地址列表
// 输入格式: "192.168.1.100,192.168.1.101,192.168.1.102"
// 输出: []string{"192.168.1.100", "192.168.1.101", "192.168.1.102"}
func parseNodeLbAddresses(nodeLb string) []string {
	if nodeLb == "" {
		return []string{}
	}

	addresses := make([]string, 0)
	parts := strings.Split(nodeLb, ",")

	for _, part := range parts {
		addr := strings.TrimSpace(part)
		if addr != "" {
			addresses = append(addresses, addr)
		}
	}

	return addresses
}

// ServicePortForWorkload Service 端口信息（内部使用）
type ServicePortForWorkload struct {
	Port     int32
	NodePort int32
	Protocol string
}

// parseServicePortsForWorkload 解析 Service 端口字符串（通用实现）
func parseServicePortsForWorkload(portsStr string) []ServicePortForWorkload {
	if portsStr == "" {
		return []ServicePortForWorkload{}
	}

	ports := make([]ServicePortForWorkload, 0)
	portPairs := strings.Split(portsStr, ",")

	for _, pair := range portPairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		// 移除协议部分 (e.g., /TCP, /UDP)
		portPart := pair
		protocol := "TCP"
		if idx := strings.Index(pair, "/"); idx > 0 {
			portPart = pair[:idx]
			protocol = pair[idx+1:]
		}

		// 解析端口: "80:30080" 或 "80"
		parts := strings.Split(portPart, ":")
		if len(parts) == 2 {
			// NodePort 格式: "port:nodePort"
			port := parsePortForWorkload(parts[0])
			nodePort := parsePortForWorkload(parts[1])
			ports = append(ports, ServicePortForWorkload{
				Port:     port,
				NodePort: nodePort,
				Protocol: protocol,
			})
		} else if len(parts) == 1 {
			// ClusterIP 格式: "port"
			port := parsePortForWorkload(parts[0])
			ports = append(ports, ServicePortForWorkload{
				Port:     port,
				Protocol: protocol,
			})
		}
	}

	return ports
}

// parsePortForWorkload 解析端口号（通用实现）
func parsePortForWorkload(portStr string) int32 {
	portStr = strings.TrimSpace(portStr)
	var port int32
	fmt.Sscanf(portStr, "%d", &port)
	return port
}

// extractIngressDomainsForWorkload 从 Ingress 列表中提取所有域名（通用实现）
func extractIngressDomainsForWorkload(ingresses []types.IngressInfo) []string {
	domains := make([]string, 0)
	domainSet := make(map[string]bool) // 用于去重

	for _, ingress := range ingresses {
		for _, host := range ingress.Hosts {
			if host != "" && !domainSet[host] {
				domains = append(domains, host)
				domainSet[host] = true
			}
		}
	}

	return domains
}

// buildLabelSelectorForWorkload 构建标签选择器字符串（通用实现）
func buildLabelSelectorForWorkload(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}

	selectors := make([]string, 0, len(labels))
	for k, v := range labels {
		selectors = append(selectors, fmt.Sprintf("%s=%s", k, v))
	}

	return strings.Join(selectors, ",")
}
