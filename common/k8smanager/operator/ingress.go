package operator

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1lister "k8s.io/client-go/listers/core/v1"
	networkingv1lister "k8s.io/client-go/listers/networking/v1"
	"sigs.k8s.io/yaml"
)

type ingressOperator struct {
	BaseOperator
	client          kubernetes.Interface
	informerFactory informers.SharedInformerFactory
	ingressLister   networkingv1lister.IngressLister
	serviceLister   corev1lister.ServiceLister
	endpointsLister corev1lister.EndpointsLister
}

func NewIngressOperator(ctx context.Context, client kubernetes.Interface) types.IngressOperator {
	return &ingressOperator{
		BaseOperator: NewBaseOperator(ctx, false),
		client:       client,
	}
}

func NewIngressOperatorWithInformer(
	ctx context.Context,
	client kubernetes.Interface,
	informerFactory informers.SharedInformerFactory,
) types.IngressOperator {
	var (
		ingressLister   networkingv1lister.IngressLister
		serviceLister   corev1lister.ServiceLister
		endpointsLister corev1lister.EndpointsLister
	)

	if informerFactory != nil {
		ingressLister = informerFactory.Networking().V1().Ingresses().Lister()
		serviceLister = informerFactory.Core().V1().Services().Lister()
		endpointsLister = informerFactory.Core().V1().Endpoints().Lister()
	}

	return &ingressOperator{
		BaseOperator:    NewBaseOperator(ctx, informerFactory != nil),
		client:          client,
		informerFactory: informerFactory,
		ingressLister:   ingressLister,
		serviceLister:   serviceLister,
		endpointsLister: endpointsLister,
	}
}

// Create 创建 Ingress
func (i *ingressOperator) Create(ingress *networkingv1.Ingress) (*networkingv1.Ingress, error) {
	if ingress == nil || ingress.Name == "" || ingress.Namespace == "" {
		return nil, fmt.Errorf("Ingress对象、名称和命名空间不能为空")
	}
	injectCommonAnnotations(ingress)
	created, err := i.client.NetworkingV1().Ingresses(ingress.Namespace).Create(i.ctx, ingress, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("创建Ingress失败: %v", err)
	}

	return created, nil
}

// Get 获取指定的 Ingress
func (i *ingressOperator) Get(namespace, name string) (*networkingv1.Ingress, error) {
	if namespace == "" || name == "" {
		return nil, fmt.Errorf("命名空间和名称不能为空")
	}

	// 优先使用 informer
	if i.useInformer && i.ingressLister != nil {
		ingress, err := i.ingressLister.Ingresses(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("Ingress %s/%s 不存在", namespace, name)
			}
			// informer 出错时回退到 API 调用
			ingress, apiErr := i.client.NetworkingV1().Ingresses(namespace).Get(i.ctx, name, metav1.GetOptions{})
			if apiErr != nil {
				return nil, fmt.Errorf("获取Ingress失败: %v", apiErr)
			}
			return ingress, nil
		}
		return ingress, nil
	}

	// 使用 API 调用
	ingress, err := i.client.NetworkingV1().Ingresses(namespace).Get(i.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("Ingress %s/%s 不存在", namespace, name)
		}
		return nil, fmt.Errorf("获取Ingress失败: %v", err)
	}

	return ingress, nil
}

// Update 更新 Ingress
func (i *ingressOperator) Update(ingress *networkingv1.Ingress) (*networkingv1.Ingress, error) {
	if ingress == nil || ingress.Name == "" || ingress.Namespace == "" {
		return nil, fmt.Errorf("Ingress对象、名称和命名空间不能为空")
	}

	updated, err := i.client.NetworkingV1().Ingresses(ingress.Namespace).Update(i.ctx, ingress, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("更新Ingress失败: %v", err)
	}

	return updated, nil
}

// Delete 删除 Ingress
func (i *ingressOperator) Delete(namespace, name string) error {
	if namespace == "" || name == "" {
		return fmt.Errorf("命名空间和名称不能为空")
	}

	err := i.client.NetworkingV1().Ingresses(namespace).Delete(i.ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("删除Ingress失败: %v", err)
	}

	return nil
}

// List 获取 Ingress 列表，支持 search 和 labelSelector 过滤
func (i *ingressOperator) List(namespace string, search string, labelSelector string) (*types.ListIngressResponse, error) {
	var selector labels.Selector = labels.Everything()
	if labelSelector != "" {
		parsedSelector, err := labels.Parse(labelSelector)
		if err != nil {
			return nil, fmt.Errorf("解析标签选择器失败: %v", err)
		}
		selector = parsedSelector
	}

	var ingresses []*networkingv1.Ingress
	var err error

	// 优先使用 informer
	if i.useInformer && i.ingressLister != nil {
		ingresses, err = i.ingressLister.Ingresses(namespace).List(selector)
		if err != nil {
			return nil, fmt.Errorf("获取Ingress列表失败: %v", err)
		}
	} else {
		// 使用 API 调用
		listOpts := metav1.ListOptions{LabelSelector: selector.String()}
		ingressList, err := i.client.NetworkingV1().Ingresses(namespace).List(i.ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("获取Ingress列表失败: %v", err)
		}
		ingresses = make([]*networkingv1.Ingress, len(ingressList.Items))
		for idx := range ingressList.Items {
			ingresses[idx] = &ingressList.Items[idx]
		}
	}

	// 搜索过滤
	if search != "" {
		filtered := make([]*networkingv1.Ingress, 0)
		searchLower := strings.ToLower(search)
		for _, ing := range ingresses {
			if strings.Contains(strings.ToLower(ing.Name), searchLower) {
				filtered = append(filtered, ing)
			}
		}
		ingresses = filtered
	}

	// 转换为响应格式
	items := make([]types.IngressInfo, len(ingresses))
	for idx, ing := range ingresses {
		items[idx] = types.IngressInfo{
			Name:              ing.Name,
			Namespace:         ing.Namespace,
			IngressClass:      getIngressClassName(ing),
			Hosts:             extractHosts(ing),
			Address:           extractAddress(ing),
			Ports:             extractPorts(ing),
			Labels:            ing.Labels,
			Annotations:       ing.Annotations,
			Age:               time.Since(ing.CreationTimestamp.Time).String(),
			CreationTimestamp: ing.CreationTimestamp.UnixMilli(),
		}
	}

	return &types.ListIngressResponse{
		Total: len(items),
		Items: items,
	}, nil
}

// Watch 监听 Ingress 变化
func (i *ingressOperator) Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return i.client.NetworkingV1().Ingresses(namespace).Watch(i.ctx, opts)
}

// UpdateLabels 更新 Ingress 标签
func (i *ingressOperator) UpdateLabels(namespace, name string, labels map[string]string) error {
	ingress, err := i.Get(namespace, name)
	if err != nil {
		return err
	}

	if ingress.Labels == nil {
		ingress.Labels = make(map[string]string)
	}
	for k, v := range labels {
		ingress.Labels[k] = v
	}

	_, err = i.Update(ingress)
	return err
}

// UpdateAnnotations 更新 Ingress 注解
func (i *ingressOperator) UpdateAnnotations(namespace, name string, annotations map[string]string) error {
	ingress, err := i.Get(namespace, name)
	if err != nil {
		return err
	}

	if ingress.Annotations == nil {
		ingress.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		ingress.Annotations[k] = v
	}

	_, err = i.Update(ingress)
	return err
}

// GetYaml 获取 Ingress 的 YAML 表示
func (i *ingressOperator) GetYaml(namespace, name string) (string, error) {
	ingress, err := i.Get(namespace, name)
	if err != nil {
		return "", err
	}

	ingress.TypeMeta = metav1.TypeMeta{
		APIVersion: "networking.k8s.io/v1",
		Kind:       "Ingress",
	}
	ingress.ManagedFields = nil

	yamlBytes, err := yaml.Marshal(ingress)
	if err != nil {
		return "", fmt.Errorf("转换为YAML失败: %v", err)
	}

	return string(yamlBytes), nil
}

// GetDetail 获取 Ingress 详细信息
func (i *ingressOperator) GetDetail(namespace, name string) (*types.IngressDetail, error) {
	ingress, err := i.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	detail := &types.IngressDetail{
		Name:              ingress.Name,
		Namespace:         ingress.Namespace,
		IngressClass:      getIngressClassName(ingress),
		Labels:            ingress.Labels,
		Annotations:       ingress.Annotations,
		Rules:             convertRules(ingress.Spec.Rules),
		TLS:               convertTLS(ingress.Spec.TLS),
		DefaultBackend:    convertDefaultBackend(ingress.Spec.DefaultBackend),
		LoadBalancer:      convertLoadBalancerStatus(ingress.Status.LoadBalancer),
		Age:               time.Since(ingress.CreationTimestamp.Time).String(),
		CreationTimestamp: ingress.CreationTimestamp.UnixMilli(),
	}

	return detail, nil
}

// Describe 获取 Ingress 描述信息（类似 kubectl describe）
func (i *ingressOperator) Describe(namespace, name string) (*types.IngressDescribeInfo, error) {
	detail, err := i.GetDetail(namespace, name)
	if err != nil {
		return nil, err
	}

	describeInfo := &types.IngressDescribeInfo{
		IngressDetail: *detail,
		Events:        make([]string, 0),
		Backends:      make([]types.BackendStatus, 0),
	}

	// 获取相关事件
	events, err := i.getIngressEvents(namespace, name)
	if err == nil {
		describeInfo.Events = events
	}

	// 获取后端服务状态
	backends, err := i.getBackendStatus(namespace, detail.Rules)
	if err == nil {
		describeInfo.Backends = backends
	}

	return describeInfo, nil
}

// AddRule 添加 Ingress 规则
func (i *ingressOperator) AddRule(namespace, name string, rule types.IngressRuleInfo) error {
	ingress, err := i.Get(namespace, name)
	if err != nil {
		return err
	}

	// 转换规则
	newRule := convertToK8sRule(rule)
	ingress.Spec.Rules = append(ingress.Spec.Rules, newRule)

	_, err = i.Update(ingress)
	return err
}

// DeleteRule 删除 Ingress 规则
func (i *ingressOperator) DeleteRule(namespace, name, host string) error {
	ingress, err := i.Get(namespace, name)
	if err != nil {
		return err
	}

	// 过滤掉指定 host 的规则
	filteredRules := make([]networkingv1.IngressRule, 0)
	for _, rule := range ingress.Spec.Rules {
		if rule.Host != host {
			filteredRules = append(filteredRules, rule)
		}
	}

	ingress.Spec.Rules = filteredRules
	_, err = i.Update(ingress)
	return err
}

// UpdateRule 更新 Ingress 规则
func (i *ingressOperator) UpdateRule(namespace, name string, rule types.IngressRuleInfo) error {
	ingress, err := i.Get(namespace, name)
	if err != nil {
		return err
	}

	// 查找并更新指定 host 的规则
	found := false
	newRule := convertToK8sRule(rule)
	for idx, existingRule := range ingress.Spec.Rules {
		if existingRule.Host == rule.Host {
			ingress.Spec.Rules[idx] = newRule
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("未找到 host 为 %s 的规则", rule.Host)
	}

	_, err = i.Update(ingress)
	return err
}

// AddTLS 添加 TLS 配置
func (i *ingressOperator) AddTLS(namespace, name string, tls types.IngressTLSInfo) error {
	ingress, err := i.Get(namespace, name)
	if err != nil {
		return err
	}

	newTLS := networkingv1.IngressTLS{
		Hosts:      tls.Hosts,
		SecretName: tls.SecretName,
	}

	ingress.Spec.TLS = append(ingress.Spec.TLS, newTLS)
	_, err = i.Update(ingress)
	return err
}

// DeleteTLS 删除 TLS 配置
func (i *ingressOperator) DeleteTLS(namespace, name, secretName string) error {
	ingress, err := i.Get(namespace, name)
	if err != nil {
		return err
	}

	// 过滤掉指定 secretName 的 TLS 配置
	filteredTLS := make([]networkingv1.IngressTLS, 0)
	for _, tls := range ingress.Spec.TLS {
		if tls.SecretName != secretName {
			filteredTLS = append(filteredTLS, tls)
		}
	}

	ingress.Spec.TLS = filteredTLS
	_, err = i.Update(ingress)
	return err
}

// UpdateTLS 更新 TLS 配置
func (i *ingressOperator) UpdateTLS(namespace, name string, tls types.IngressTLSInfo) error {
	ingress, err := i.Get(namespace, name)
	if err != nil {
		return err
	}

	// 查找并更新指定 secretName 的 TLS 配置
	found := false
	newTLS := networkingv1.IngressTLS{
		Hosts:      tls.Hosts,
		SecretName: tls.SecretName,
	}

	for idx, existingTLS := range ingress.Spec.TLS {
		if existingTLS.SecretName == tls.SecretName {
			ingress.Spec.TLS[idx] = newTLS
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("未找到 secretName 为 %s 的 TLS 配置", tls.SecretName)
	}

	_, err = i.Update(ingress)
	return err
}

// ========== 辅助函数 ==========

// getIngressClassName 获取 IngressClass 名称
func getIngressClassName(ingress *networkingv1.Ingress) string {
	if ingress.Spec.IngressClassName != nil {
		return *ingress.Spec.IngressClassName
	}
	// 检查注解中的 IngressClass（旧版本兼容）
	if ingress.Annotations != nil {
		if class, ok := ingress.Annotations["kubernetes.io/ingress.class"]; ok {
			return class
		}
	}
	return ""
}

// extractHosts 提取所有主机名
func extractHosts(ingress *networkingv1.Ingress) []string {
	hosts := make([]string, 0)
	seenHosts := make(map[string]bool)

	for _, rule := range ingress.Spec.Rules {
		if rule.Host != "" && !seenHosts[rule.Host] {
			hosts = append(hosts, rule.Host)
			seenHosts[rule.Host] = true
		}
	}

	return hosts
}

// extractAddress 提取负载均衡器地址
func extractAddress(ingress *networkingv1.Ingress) string {
	addresses := make([]string, 0)
	for _, lb := range ingress.Status.LoadBalancer.Ingress {
		if lb.IP != "" {
			addresses = append(addresses, lb.IP)
		} else if lb.Hostname != "" {
			addresses = append(addresses, lb.Hostname)
		}
	}
	return strings.Join(addresses, ", ")
}

// extractPorts 提取端口信息
func extractPorts(ingress *networkingv1.Ingress) string {
	ports := make(map[string]bool)

	// 检查 TLS
	if len(ingress.Spec.TLS) > 0 {
		ports["443"] = true
	}

	// 默认添加 HTTP 端口
	if len(ingress.Spec.Rules) > 0 {
		ports["80"] = true
	}

	portList := make([]string, 0)
	if ports["80"] {
		portList = append(portList, "80")
	}
	if ports["443"] {
		portList = append(portList, "443")
	}

	return strings.Join(portList, ", ")
}

// convertRules 转换规则
func convertRules(rules []networkingv1.IngressRule) []types.IngressRuleInfo {
	result := make([]types.IngressRuleInfo, len(rules))
	for i, rule := range rules {
		result[i] = types.IngressRuleInfo{
			Host:  rule.Host,
			Paths: convertPaths(rule.HTTP),
		}
	}
	return result
}

// convertPaths 转换路径
func convertPaths(http *networkingv1.HTTPIngressRuleValue) []types.IngressPathInfo {
	if http == nil {
		return nil
	}

	result := make([]types.IngressPathInfo, len(http.Paths))
	for i, path := range http.Paths {
		pathType := "Prefix"
		if path.PathType != nil {
			pathType = string(*path.PathType)
		}

		result[i] = types.IngressPathInfo{
			Path:     path.Path,
			PathType: pathType,
			Backend:  convertBackend(&path.Backend),
		}
	}
	return result
}

// convertBackend 转换后端
func convertBackend(backend *networkingv1.IngressBackend) types.IngressBackendInfo {
	info := types.IngressBackendInfo{}

	if backend.Service != nil {
		info.ServiceName = backend.Service.Name
		if backend.Service.Port.Number != 0 {
			info.ServicePort = strconv.Itoa(int(backend.Service.Port.Number))
		} else if backend.Service.Port.Name != "" {
			info.ServicePort = backend.Service.Port.Name
		}
	}

	if backend.Resource != nil {
		apiGroup := ""
		if backend.Resource.APIGroup != nil {
			apiGroup = *backend.Resource.APIGroup
		}
		info.ResourceRef = &types.IngressResourceRef{
			APIGroup: apiGroup,
			Kind:     backend.Resource.Kind,
			Name:     backend.Resource.Name,
		}
	}

	return info
}

// convertDefaultBackend 转换默认后端
func convertDefaultBackend(backend *networkingv1.IngressBackend) *types.IngressBackendInfo {
	if backend == nil {
		return nil
	}
	result := convertBackend(backend)
	return &result
}

// convertTLS 转换 TLS 配置
func convertTLS(tls []networkingv1.IngressTLS) []types.IngressTLSInfo {
	result := make([]types.IngressTLSInfo, len(tls))
	for i, t := range tls {
		result[i] = types.IngressTLSInfo{
			Hosts:      t.Hosts,
			SecretName: t.SecretName,
		}
	}
	return result
}

// convertLoadBalancerStatus 转换负载均衡器状态
func convertLoadBalancerStatus(lb networkingv1.IngressLoadBalancerStatus) types.IngressLoadBalancerInfo {
	ingresses := make([]types.IngressLoadBalancerIngress, len(lb.Ingress))
	for i, ing := range lb.Ingress {
		ports := make([]types.IngressPortStatus, 0)
		if ing.Ports != nil {
			ports = make([]types.IngressPortStatus, len(ing.Ports))
			for j, port := range ing.Ports {
				errorMsg := ""
				if port.Error != nil {
					errorMsg = *port.Error
				}
				ports[j] = types.IngressPortStatus{
					Port:     port.Port,
					Protocol: string(port.Protocol),
					Error:    errorMsg,
				}
			}
		}

		ingresses[i] = types.IngressLoadBalancerIngress{
			IP:       ing.IP,
			Hostname: ing.Hostname,
			Ports:    ports,
		}
	}

	return types.IngressLoadBalancerInfo{
		Ingress: ingresses,
	}
}

// convertToK8sRule 转换为 K8s 规则
func convertToK8sRule(rule types.IngressRuleInfo) networkingv1.IngressRule {
	paths := make([]networkingv1.HTTPIngressPath, len(rule.Paths))
	for i, path := range rule.Paths {
		pathType := networkingv1.PathTypePrefix
		switch path.PathType {
		case "Exact":
			pathType = networkingv1.PathTypeExact
		case "ImplementationSpecific":
			pathType = networkingv1.PathTypeImplementationSpecific
		}

		backend := networkingv1.IngressBackend{}
		if path.Backend.ServiceName != "" {
			port := intstr.IntOrString{}
			if portNum, err := strconv.Atoi(path.Backend.ServicePort); err == nil {
				port = intstr.FromInt(portNum)
			} else {
				port = intstr.FromString(path.Backend.ServicePort)
			}

			backend.Service = &networkingv1.IngressServiceBackend{
				Name: path.Backend.ServiceName,
				Port: networkingv1.ServiceBackendPort{
					Number: int32(port.IntVal),
					Name:   port.StrVal,
				},
			}
		}

		paths[i] = networkingv1.HTTPIngressPath{
			Path:     path.Path,
			PathType: &pathType,
			Backend:  backend,
		}
	}

	return networkingv1.IngressRule{
		Host: rule.Host,
		IngressRuleValue: networkingv1.IngressRuleValue{
			HTTP: &networkingv1.HTTPIngressRuleValue{
				Paths: paths,
			},
		},
	}
}

// getIngressEvents 获取 Ingress 相关事件
func (i *ingressOperator) getIngressEvents(namespace, name string) ([]string, error) {
	events, err := i.client.CoreV1().Events(namespace).List(i.ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Ingress", name),
	})
	if err != nil {
		return nil, err
	}

	eventStrings := make([]string, 0)
	for _, event := range events.Items {
		eventStr := fmt.Sprintf("[%s] %s: %s",
			event.Type,
			event.Reason,
			event.Message,
		)
		eventStrings = append(eventStrings, eventStr)
	}

	return eventStrings, nil
}

// getBackendStatus 获取后端服务状态
func (i *ingressOperator) getBackendStatus(namespace string, rules []types.IngressRuleInfo) ([]types.BackendStatus, error) {
	backends := make([]types.BackendStatus, 0)
	seenServices := make(map[string]bool)

	for _, rule := range rules {
		for _, path := range rule.Paths {
			serviceName := path.Backend.ServiceName
			if serviceName == "" || seenServices[serviceName] {
				continue
			}
			seenServices[serviceName] = true

			status := types.BackendStatus{
				ServiceName: serviceName,
			}

			// 获取 Service 信息
			var err error

			if i.useInformer && i.serviceLister != nil {
				_, err = i.serviceLister.Services(namespace).Get(serviceName)
			} else {
				_, err = i.client.CoreV1().Services(namespace).Get(i.ctx, serviceName, metav1.GetOptions{})
			}

			if err != nil {
				status.Available = false
				status.Message = fmt.Sprintf("服务不存在: %v", err)
				backends = append(backends, status)
				continue
			}

			// 获取 Endpoints 信息
			var endpoints *corev1.Endpoints
			if i.useInformer && i.endpointsLister != nil {
				endpoints, err = i.endpointsLister.Endpoints(namespace).Get(serviceName)
			} else {
				endpoints, err = i.client.CoreV1().Endpoints(namespace).Get(i.ctx, serviceName, metav1.GetOptions{})
			}

			if err != nil || endpoints == nil {
				status.Available = false
				status.Message = "无可用端点"
			} else {
				endpointCount := 0
				for _, subset := range endpoints.Subsets {
					endpointCount += len(subset.Addresses)
				}
				status.Endpoints = endpointCount
				status.Available = endpointCount > 0
				if !status.Available {
					status.Message = "无可用端点"
				}
			}

			backends = append(backends, status)
		}
	}

	return backends, nil
}

// ListByServiceNames 通过 Service Names 查询 Ingress
func (i *ingressOperator) ListByServiceNames(namespace string, serviceNames []string) (*types.ListIngressResponse, error) {
	if len(serviceNames) == 0 {
		return &types.ListIngressResponse{Total: 0, Items: []types.IngressInfo{}}, nil
	}

	// 创建 service name 的 map 用于快速查找
	serviceNameMap := make(map[string]bool)
	for _, name := range serviceNames {
		serviceNameMap[name] = true
	}

	var ingresses []*networkingv1.Ingress
	var err error

	// 获取所有 Ingress
	if i.useInformer && i.ingressLister != nil {
		ingresses, err = i.ingressLister.Ingresses(namespace).List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("获取Ingress列表失败: %v", err)
		}
	} else {
		ingressList, err := i.client.NetworkingV1().Ingresses(namespace).List(i.ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("获取Ingress列表失败: %v", err)
		}
		ingresses = make([]*networkingv1.Ingress, len(ingressList.Items))
		for idx := range ingressList.Items {
			ingresses[idx] = &ingressList.Items[idx]
		}
	}

	// 过滤出引用指定 Service 的 Ingress
	matchedIngresses := make([]*networkingv1.Ingress, 0)
	for _, ing := range ingresses {
		if i.ingressReferencesServices(ing, serviceNameMap) {
			matchedIngresses = append(matchedIngresses, ing)
		}
	}

	// 转换为响应格式
	items := make([]types.IngressInfo, len(matchedIngresses))
	for idx, ing := range matchedIngresses {
		items[idx] = types.IngressInfo{
			Name:              ing.Name,
			Namespace:         ing.Namespace,
			IngressClass:      getIngressClassName(ing),
			Hosts:             extractHosts(ing),
			Address:           extractAddress(ing),
			Ports:             extractPorts(ing),
			Labels:            ing.Labels,
			Annotations:       ing.Annotations,
			Age:               time.Since(ing.CreationTimestamp.Time).String(),
			CreationTimestamp: ing.CreationTimestamp.UnixMilli(),
		}
	}

	return &types.ListIngressResponse{
		Total: len(items),
		Items: items,
	}, nil
}

// ingressReferencesServices 检查 Ingress 是否引用了指定的 Service
func (i *ingressOperator) ingressReferencesServices(ing *networkingv1.Ingress, serviceNames map[string]bool) bool {
	// 检查 DefaultBackend
	if ing.Spec.DefaultBackend != nil && ing.Spec.DefaultBackend.Service != nil {
		if serviceNames[ing.Spec.DefaultBackend.Service.Name] {
			return true
		}
	}

	// 检查 Rules 中的所有 Backend
	for _, rule := range ing.Spec.Rules {
		if rule.HTTP != nil {
			for _, path := range rule.HTTP.Paths {
				if path.Backend.Service != nil && serviceNames[path.Backend.Service.Name] {
					return true
				}
			}
		}
	}

	return false
}
