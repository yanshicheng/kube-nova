package operator

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1lister "k8s.io/client-go/listers/core/v1"
	"sigs.k8s.io/yaml"
)

type serviceOperator struct {
	BaseOperator
	client          kubernetes.Interface
	informerFactory informers.SharedInformerFactory
	serviceLister   corev1lister.ServiceLister
}

func NewServiceOperator(ctx context.Context, client kubernetes.Interface) types.ServiceOperator {
	return &serviceOperator{
		BaseOperator: NewBaseOperator(ctx, false),
		client:       client,
	}
}

func NewServiceOperatorWithInformer(
	ctx context.Context,
	client kubernetes.Interface,
	informerFactory informers.SharedInformerFactory,
) types.ServiceOperator {
	var serviceLister corev1lister.ServiceLister

	if informerFactory != nil {
		serviceLister = informerFactory.Core().V1().Services().Lister()
	}

	return &serviceOperator{
		BaseOperator:    NewBaseOperator(ctx, informerFactory != nil),
		client:          client,
		informerFactory: informerFactory,
		serviceLister:   serviceLister,
	}
}

func (s *serviceOperator) Create(service *corev1.Service) (*corev1.Service, error) {
	if service == nil || service.Name == "" || service.Namespace == "" {
		return nil, fmt.Errorf("Service对象、名称和命名空间不能为空")
	}
	injectCommonAnnotations(service)
	created, err := s.client.CoreV1().Services(service.Namespace).Create(s.ctx, service, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("创建Service失败: %v", err)
	}

	return created, nil
}

func (s *serviceOperator) Get(namespace, name string) (*corev1.Service, error) {
	if namespace == "" || name == "" {
		return nil, fmt.Errorf("命名空间和名称不能为空")
	}

	if s.serviceLister != nil {
		service, err := s.serviceLister.Services(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("Service %s/%s 不存在", namespace, name)
			}
			service, apiErr := s.client.CoreV1().Services(namespace).Get(s.ctx, name, metav1.GetOptions{})
			if apiErr != nil {
				return nil, fmt.Errorf("获取Service失败")
			}
			service.TypeMeta = metav1.TypeMeta{
				Kind:       "Service",
				APIVersion: "v1",
			}
			service.Name = name
			return service, nil
		}
		service.TypeMeta = metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		}
		service.Name = name
		return service, nil
	}

	service, err := s.client.CoreV1().Services(namespace).Get(s.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("Service %s/%s 不存在", namespace, name)
		}
		return nil, fmt.Errorf("获取Service失败")
	}

	service.TypeMeta = metav1.TypeMeta{
		Kind:       "Service",
		APIVersion: "v1",
	}
	service.Name = name

	return service, nil
}

func (s *serviceOperator) Update(service *corev1.Service) (*corev1.Service, error) {
	if service == nil || service.Name == "" || service.Namespace == "" {
		return nil, fmt.Errorf("Service对象、名称和命名空间不能为空")
	}

	updated, err := s.client.CoreV1().Services(service.Namespace).Update(s.ctx, service, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("更新Service失败")
	}

	return updated, nil
}

func (s *serviceOperator) Delete(namespace, name string) error {
	if namespace == "" || name == "" {
		return fmt.Errorf("命名空间和名称不能为空")
	}

	err := s.client.CoreV1().Services(namespace).Delete(s.ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("删除Service失败")
	}

	return nil
}

// Service Operator 修改后的 List 方法
func (s *serviceOperator) List(namespace string, search string, labelSelector string) (*types.ListServiceResponse, error) {
	var selector labels.Selector = labels.Everything()
	if labelSelector != "" {
		parsedSelector, err := labels.Parse(labelSelector)
		if err != nil {
			return nil, fmt.Errorf("解析标签选择器失败")
		}
		selector = parsedSelector
	}

	var services []*corev1.Service
	var err error

	if s.useInformer && s.serviceLister != nil {
		services, err = s.serviceLister.Services(namespace).List(selector)
		if err != nil {
			return nil, fmt.Errorf("获取Service列表失败")
		}
	} else {
		listOpts := metav1.ListOptions{LabelSelector: selector.String()}
		svcList, err := s.client.CoreV1().Services(namespace).List(s.ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("获取Service列表失败")
		}
		services = make([]*corev1.Service, len(svcList.Items))
		for i := range svcList.Items {
			services[i] = &svcList.Items[i]
		}
	}

	// 搜索过滤
	if search != "" {
		filtered := make([]*corev1.Service, 0)
		searchLower := strings.ToLower(search)
		for _, svc := range services {
			if strings.Contains(strings.ToLower(svc.Name), searchLower) {
				filtered = append(filtered, svc)
			}
		}
		services = filtered
	}

	// 转换为响应格式
	items := make([]types.ServiceInfo, len(services))
	for i, svc := range services {
		items[i] = s.convertToServiceInfo(svc)
	}

	return &types.ListServiceResponse{
		Total: len(items),
		Items: items,
	}, nil
}

// convertToServiceInfo 转换 Service 对象为 ServiceInfo（更新版）
func (s *serviceOperator) convertToServiceInfo(svc *corev1.Service) types.ServiceInfo {
	// 处理端口信息
	ports := make([]string, 0)
	for _, port := range svc.Spec.Ports {
		portStr := fmt.Sprintf("%d", port.Port)
		if port.NodePort != 0 {
			portStr = fmt.Sprintf("%d:%d", port.Port, port.NodePort)
		}
		portStr += "/" + string(port.Protocol)
		ports = append(ports, portStr)
	}

	// 处理外部 IP
	externalIP := "<none>"
	if len(svc.Spec.ExternalIPs) > 0 {
		externalIP = strings.Join(svc.Spec.ExternalIPs, ",")
	} else if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
		if len(svc.Status.LoadBalancer.Ingress) > 0 {
			ips := make([]string, 0)
			for _, ing := range svc.Status.LoadBalancer.Ingress {
				if ing.IP != "" {
					ips = append(ips, ing.IP)
				} else if ing.Hostname != "" {
					ips = append(ips, ing.Hostname)
				}
			}
			if len(ips) > 0 {
				externalIP = strings.Join(ips, ",")
			}
		}
	}

	// 处理 IP 协议族
	ipFamilies := make([]string, 0)
	if svc.Spec.IPFamilies != nil {
		for _, family := range svc.Spec.IPFamilies {
			ipFamilies = append(ipFamilies, string(family))
		}
	}

	// IP 协议族策略
	ipFamilyPolicy := ""
	if svc.Spec.IPFamilyPolicy != nil {
		ipFamilyPolicy = string(*svc.Spec.IPFamilyPolicy)
	}

	// 外部流量策略
	externalTrafficPolicy := ""
	if svc.Spec.ExternalTrafficPolicy != "" {
		externalTrafficPolicy = string(svc.Spec.ExternalTrafficPolicy)
	}

	// 负载均衡器类
	loadBalancerClass := ""
	if svc.Spec.LoadBalancerClass != nil {
		loadBalancerClass = *svc.Spec.LoadBalancerClass
	}

	return types.ServiceInfo{
		Name:                  svc.Name,
		Namespace:             svc.Namespace,
		Type:                  string(svc.Spec.Type),
		ClusterIP:             svc.Spec.ClusterIP,
		ExternalIP:            externalIP,
		Ports:                 strings.Join(ports, ","),
		Age:                   time.Since(svc.CreationTimestamp.Time).String(),
		CreationTimestamp:     svc.CreationTimestamp.UnixMilli(),
		Labels:                svc.Labels,
		ClusterIPs:            svc.Spec.ClusterIPs,
		IpFamilies:            ipFamilies,
		IpFamilyPolicy:        ipFamilyPolicy,
		ExternalTrafficPolicy: externalTrafficPolicy,
		SessionAffinity:       string(svc.Spec.SessionAffinity),
		LoadBalancerClass:     loadBalancerClass,
		Selector:              svc.Spec.Selector,
		Annotations:           svc.Annotations,
	}
}
func (s *serviceOperator) Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return s.client.CoreV1().Services(namespace).Watch(s.ctx, opts)
}

func (s *serviceOperator) UpdateLabels(namespace, name string, labels map[string]string) error {
	service, err := s.Get(namespace, name)
	if err != nil {
		return err
	}

	if service.Labels == nil {
		service.Labels = make(map[string]string)
	}
	for k, v := range labels {
		service.Labels[k] = v
	}

	_, err = s.Update(service)
	return err
}

func (s *serviceOperator) UpdateAnnotations(namespace, name string, annotations map[string]string) error {
	service, err := s.Get(namespace, name)
	if err != nil {
		return err
	}

	if service.Annotations == nil {
		service.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		service.Annotations[k] = v
	}

	_, err = s.Update(service)
	return err
}

func (s *serviceOperator) GetYaml(namespace, name string) (string, error) {
	service, err := s.Get(namespace, name)
	if err != nil {
		return "", err
	}

	service.TypeMeta = metav1.TypeMeta{
		Kind:       "Service",
		APIVersion: "v1",
	}
	service.Name = name

	service.ManagedFields = nil
	yamlBytes, err := yaml.Marshal(service)
	if err != nil {
		return "", fmt.Errorf("转换为YAML失败: %v", err)
	}

	return string(yamlBytes), nil
}

func (s *serviceOperator) GetDetail(namespace, name string) (*types.ServiceDetailInfo, error) {
	service, err := s.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	detail := &types.ServiceDetailInfo{
		ServiceInfo: s.convertToServiceInfo(service),
		Ports:       make([]types.ServicePortInfo, 0),
		ExternalIPs: service.Spec.ExternalIPs,
	}

	for _, port := range service.Spec.Ports {
		detail.Ports = append(detail.Ports, types.ServicePortInfo{
			Name:       port.Name,
			Protocol:   string(port.Protocol),
			Port:       port.Port,
			TargetPort: port.TargetPort.String(),
			NodePort:   port.NodePort,
		})
	}

	if service.Spec.Type == corev1.ServiceTypeLoadBalancer {
		detail.LoadBalancerIP = service.Spec.LoadBalancerIP
		if len(service.Status.LoadBalancer.Ingress) > 0 {
			detail.LoadBalancerStatus = "Available"
		} else {
			detail.LoadBalancerStatus = "Pending"
		}
	}

	// 获取 Endpoints 数量
	endpoints, err := s.client.CoreV1().Endpoints(namespace).Get(s.ctx, name, metav1.GetOptions{})
	if err == nil {
		count := 0
		for _, subset := range endpoints.Subsets {
			count += len(subset.Addresses)
		}
		detail.EndpointCount = count
	}

	return detail, nil
}

func (s *serviceOperator) GetEndpoints(namespace, name string) (*types.GetServiceEndpointsResponse, error) {
	endpoints, err := s.client.CoreV1().Endpoints(namespace).Get(s.ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取Endpoints失败: %v", err)
	}

	response := &types.GetServiceEndpointsResponse{
		ServiceName: name,
		Namespace:   namespace,
		Endpoints:   make([]types.ServiceEndpointInfo, 0),
	}

	for _, subset := range endpoints.Subsets {
		ports := make([]int32, 0)
		for _, port := range subset.Ports {
			ports = append(ports, port.Port)
		}

		// 处理就绪的地址
		for _, addr := range subset.Addresses {
			podName := ""
			nodeName := ""
			if addr.TargetRef != nil {
				podName = addr.TargetRef.Name
			}
			if addr.NodeName != nil {
				nodeName = *addr.NodeName
			}

			response.Endpoints = append(response.Endpoints, types.ServiceEndpointInfo{
				PodName:   podName,
				PodIP:     addr.IP,
				NodeName:  nodeName,
				Ready:     true,
				Ports:     ports,
				Addresses: []string{addr.IP},
			})
		}

		// 处理未就绪的地址
		for _, addr := range subset.NotReadyAddresses {
			podName := ""
			nodeName := ""
			if addr.TargetRef != nil {
				podName = addr.TargetRef.Name
			}
			if addr.NodeName != nil {
				nodeName = *addr.NodeName
			}

			response.Endpoints = append(response.Endpoints, types.ServiceEndpointInfo{
				PodName:   podName,
				PodIP:     addr.IP,
				NodeName:  nodeName,
				Ready:     false,
				Ports:     ports,
				Addresses: []string{addr.IP},
			})
		}
	}

	response.TotalCount = len(response.Endpoints)

	return response, nil
}

func (s *serviceOperator) UpdateSelector(namespace, name string, selector map[string]string) error {
	service, err := s.Get(namespace, name)
	if err != nil {
		return err
	}

	service.Spec.Selector = selector

	_, err = s.Update(service)
	return err
}

func (s *serviceOperator) GetMatchingPods(namespace, name string) ([]types.PodInfo, error) {
	service, err := s.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	if len(service.Spec.Selector) == 0 {
		return []types.PodInfo{}, nil
	}

	selector := labels.SelectorFromSet(service.Spec.Selector)
	podList, err := s.client.CoreV1().Pods(namespace).List(s.ctx, metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("获取Pod列表失败: %v", err)
	}

	pods := make([]types.PodInfo, 0, len(podList.Items))
	for _, pod := range podList.Items {
		restarts := int32(0)
		for _, cs := range pod.Status.ContainerStatuses {
			restarts += cs.RestartCount
		}

		ready := 0
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Ready {
				ready++
			}
		}

		pods = append(pods, types.PodInfo{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Status:    string(pod.Status.Phase),
			Ready:     fmt.Sprintf("%d/%d", ready, len(pod.Spec.Containers)),
			Restarts:  restarts,
			Age:       time.Since(pod.CreationTimestamp.Time).String(),
			Node:      pod.Spec.NodeName,
			PodIP:     pod.Status.PodIP,
		})
	}

	return pods, nil
}

func (s *serviceOperator) AddPort(namespace, name string, port types.ServicePortInfo) error {
	service, err := s.Get(namespace, name)
	if err != nil {
		return err
	}

	targetPort := intstr.FromString(port.TargetPort)
	if val, err := strconv.Atoi(port.TargetPort); err == nil {
		targetPort = intstr.FromInt(val)
	}

	newPort := corev1.ServicePort{
		Name:       port.Name,
		Protocol:   corev1.Protocol(port.Protocol),
		Port:       port.Port,
		TargetPort: targetPort,
	}

	if port.NodePort != 0 {
		newPort.NodePort = port.NodePort
	}

	service.Spec.Ports = append(service.Spec.Ports, newPort)

	_, err = s.Update(service)
	return err
}

func (s *serviceOperator) DeletePort(namespace, name, portName string) error {
	service, err := s.Get(namespace, name)
	if err != nil {
		return err
	}

	newPorts := make([]corev1.ServicePort, 0)
	for _, p := range service.Spec.Ports {
		if p.Name != portName {
			newPorts = append(newPorts, p)
		}
	}

	service.Spec.Ports = newPorts

	_, err = s.Update(service)
	return err
}

func (s *serviceOperator) UpdatePort(namespace, name string, port types.ServicePortInfo) error {
	service, err := s.Get(namespace, name)
	if err != nil {
		return err
	}

	targetPort := intstr.FromString(port.TargetPort)
	if val, err := strconv.Atoi(port.TargetPort); err == nil {
		targetPort = intstr.FromInt(val)
	}

	for i := range service.Spec.Ports {
		if service.Spec.Ports[i].Name == port.Name {
			service.Spec.Ports[i].Protocol = corev1.Protocol(port.Protocol)
			service.Spec.Ports[i].Port = port.Port
			service.Spec.Ports[i].TargetPort = targetPort
			if port.NodePort != 0 {
				service.Spec.Ports[i].NodePort = port.NodePort
			}
			break
		}
	}

	_, err = s.Update(service)
	return err
}

func (s *serviceOperator) GetResourceServices(req *types.GetResourceServicesRequest) ([]types.ServiceInfo, error) {
	if req == nil || req.Namespace == "" || req.ResourceType == "" || req.ResourceName == "" {
		return nil, fmt.Errorf("请求参数不完整")
	}

	var labelSelector map[string]string

	switch req.ResourceType {
	case "deployment":
		deploy, err := s.client.AppsV1().Deployments(req.Namespace).Get(s.ctx, req.ResourceName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("获取Deployment失败: %v", err)
		}
		labelSelector = deploy.Spec.Selector.MatchLabels

	case "statefulset":
		sts, err := s.client.AppsV1().StatefulSets(req.Namespace).Get(s.ctx, req.ResourceName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("获取StatefulSet失败: %v", err)
		}
		labelSelector = sts.Spec.Selector.MatchLabels

	case "daemonset":
		ds, err := s.client.AppsV1().DaemonSets(req.Namespace).Get(s.ctx, req.ResourceName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("获取DaemonSet失败: %v", err)
		}
		labelSelector = ds.Spec.Selector.MatchLabels

	case "pod":
		pod, err := s.client.CoreV1().Pods(req.Namespace).Get(s.ctx, req.ResourceName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("获取Pod失败: %v", err)
		}
		labelSelector = pod.Labels

	default:
		return nil, fmt.Errorf("不支持的资源类型: %s", req.ResourceType)
	}

	return s.GetServicesByLabels(req.Namespace, labelSelector)
}

func (s *serviceOperator) GetServicesByLabels(namespace string, labels map[string]string) ([]types.ServiceInfo, error) {
	serviceList, err := s.client.CoreV1().Services(namespace).List(s.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取Service列表失败: %v", err)
	}

	matchingServices := make([]types.ServiceInfo, 0)

	for _, svc := range serviceList.Items {
		if svc.Spec.Selector == nil {
			continue
		}

		matches := true
		for k, v := range svc.Spec.Selector {
			if labelValue, ok := labels[k]; !ok || labelValue != v {
				matches = false
				break
			}
		}

		if matches {
			matchingServices = append(matchingServices, s.convertToServiceInfo(&svc))
		}
	}

	return matchingServices, nil
}
