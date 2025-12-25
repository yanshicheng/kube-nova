package operator

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	networkingv1lister "k8s.io/client-go/listers/networking/v1"
	"sigs.k8s.io/yaml"
)

const (
	// DefaultIngressClassAnnotation 默认 IngressClass 注解
	DefaultIngressClassAnnotation = "ingressclass.kubernetes.io/is-default-class"
)

type ingressClassOperator struct {
	BaseOperator
	client             kubernetes.Interface
	informerFactory    informers.SharedInformerFactory
	ingressClassLister networkingv1lister.IngressClassLister
	ingressLister      networkingv1lister.IngressLister
}

// NewIngressClassOperator 创建 IngressClass 操作器（不使用 informer）
func NewIngressClassOperator(ctx context.Context, client kubernetes.Interface) types.IngressClassOperator {
	return &ingressClassOperator{
		BaseOperator: NewBaseOperator(ctx, false),
		client:       client,
	}
}

// NewIngressClassOperatorWithInformer 创建 IngressClass 操作器（使用 informer）
func NewIngressClassOperatorWithInformer(
	ctx context.Context,
	client kubernetes.Interface,
	informerFactory informers.SharedInformerFactory,
) types.IngressClassOperator {
	var ingressClassLister networkingv1lister.IngressClassLister
	var ingressLister networkingv1lister.IngressLister

	if informerFactory != nil {
		ingressClassLister = informerFactory.Networking().V1().IngressClasses().Lister()
		ingressLister = informerFactory.Networking().V1().Ingresses().Lister()
	}

	return &ingressClassOperator{
		BaseOperator:       NewBaseOperator(ctx, informerFactory != nil),
		client:             client,
		informerFactory:    informerFactory,
		ingressClassLister: ingressClassLister,
		ingressLister:      ingressLister,
	}
}

// Create 创建 IngressClass
func (i *ingressClassOperator) Create(ic *networkingv1.IngressClass) (*networkingv1.IngressClass, error) {
	if ic == nil || ic.Name == "" {
		return nil, fmt.Errorf("IngressClass对象和名称不能为空")
	}

	created, err := i.client.NetworkingV1().IngressClasses().Create(i.ctx, ic, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("创建IngressClass失败: %v", err)
	}

	return created, nil
}

// Get 获取 IngressClass
func (i *ingressClassOperator) Get(name string) (*networkingv1.IngressClass, error) {
	if name == "" {
		return nil, fmt.Errorf("名称不能为空")
	}

	// 优先使用 informer
	if i.useInformer && i.ingressClassLister != nil {
		ic, err := i.ingressClassLister.Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("IngressClass %s 不存在", name)
			}
			ic, apiErr := i.client.NetworkingV1().IngressClasses().Get(i.ctx, name, metav1.GetOptions{})
			if apiErr != nil {
				return nil, fmt.Errorf("获取IngressClass失败: %v", apiErr)
			}
			i.injectTypeMeta(ic)
			return ic, nil
		}
		result := ic.DeepCopy()
		i.injectTypeMeta(result)
		return result, nil
	}

	ic, err := i.client.NetworkingV1().IngressClasses().Get(i.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("IngressClass %s 不存在", name)
		}
		return nil, fmt.Errorf("获取IngressClass失败: %v", err)
	}

	i.injectTypeMeta(ic)
	return ic, nil
}

// injectTypeMeta 注入 TypeMeta
func (i *ingressClassOperator) injectTypeMeta(ic *networkingv1.IngressClass) {
	ic.TypeMeta = metav1.TypeMeta{
		Kind:       "IngressClass",
		APIVersion: "networking.k8s.io/v1",
	}
}

// Update 更新 IngressClass
func (i *ingressClassOperator) Update(ic *networkingv1.IngressClass) (*networkingv1.IngressClass, error) {
	if ic == nil || ic.Name == "" {
		return nil, fmt.Errorf("IngressClass对象和名称不能为空")
	}

	updated, err := i.client.NetworkingV1().IngressClasses().Update(i.ctx, ic, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("更新IngressClass失败: %v", err)
	}

	return updated, nil
}

// Delete 删除 IngressClass
func (i *ingressClassOperator) Delete(name string) error {
	if name == "" {
		return fmt.Errorf("名称不能为空")
	}

	err := i.client.NetworkingV1().IngressClasses().Delete(i.ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("删除IngressClass失败: %v", err)
	}

	return nil
}

// List 获取 IngressClass 列表
func (i *ingressClassOperator) List(search string, labelSelector string) (*types.ListIngressClassResponse, error) {
	var selector labels.Selector = labels.Everything()
	if labelSelector != "" {
		parsedSelector, err := labels.Parse(labelSelector)
		if err != nil {
			return nil, fmt.Errorf("解析标签选择器失败: %v", err)
		}
		selector = parsedSelector
	}

	var ingressClasses []*networkingv1.IngressClass
	var err error

	if i.useInformer && i.ingressClassLister != nil {
		ingressClasses, err = i.ingressClassLister.List(selector)
		if err != nil {
			return nil, fmt.Errorf("获取IngressClass列表失败: %v", err)
		}
	} else {
		listOpts := metav1.ListOptions{LabelSelector: selector.String()}
		icList, err := i.client.NetworkingV1().IngressClasses().List(i.ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("获取IngressClass列表失败: %v", err)
		}
		ingressClasses = make([]*networkingv1.IngressClass, len(icList.Items))
		for idx := range icList.Items {
			ingressClasses[idx] = &icList.Items[idx]
		}
	}

	// 搜索过滤
	if search != "" {
		filtered := make([]*networkingv1.IngressClass, 0)
		searchLower := strings.ToLower(search)
		for _, ic := range ingressClasses {
			if strings.Contains(strings.ToLower(ic.Name), searchLower) ||
				strings.Contains(strings.ToLower(ic.Spec.Controller), searchLower) {
				filtered = append(filtered, ic)
			}
		}
		ingressClasses = filtered
	}

	// 转换为响应格式
	items := make([]types.IngressClassInfo, len(ingressClasses))
	for idx, ic := range ingressClasses {
		items[idx] = i.toIngressClassInfo(ic)
	}

	// 按名称排序
	sort.Slice(items, func(a, b int) bool {
		return items[a].Name < items[b].Name
	})

	return &types.ListIngressClassResponse{
		Total: len(items),
		Items: items,
	}, nil
}

// toIngressClassInfo 转换为 IngressClassInfo
func (i *ingressClassOperator) toIngressClassInfo(ic *networkingv1.IngressClass) types.IngressClassInfo {
	parameters := "<none>"
	if ic.Spec.Parameters != nil {
		if ic.Spec.Parameters.Namespace != nil && *ic.Spec.Parameters.Namespace != "" {
			parameters = fmt.Sprintf("%s/%s", ic.Spec.Parameters.Kind, ic.Spec.Parameters.Name)
		} else {
			parameters = fmt.Sprintf("%s/%s", ic.Spec.Parameters.Kind, ic.Spec.Parameters.Name)
		}
	}

	return types.IngressClassInfo{
		Name:              ic.Name,
		Controller:        ic.Spec.Controller,
		Parameters:        parameters,
		IsDefault:         i.isDefaultIngressClass(ic),
		Labels:            ic.Labels,
		Annotations:       ic.Annotations,
		Age:               i.formatAge(ic.CreationTimestamp.Time),
		CreationTimestamp: ic.CreationTimestamp.UnixMilli(),
	}
}

// isDefaultIngressClass 判断是否为默认 IngressClass
func (i *ingressClassOperator) isDefaultIngressClass(ic *networkingv1.IngressClass) bool {
	if ic.Annotations == nil {
		return false
	}
	if val, ok := ic.Annotations[DefaultIngressClassAnnotation]; ok && val == "true" {
		return true
	}
	return false
}

// formatAge 格式化时间
func (i *ingressClassOperator) formatAge(t time.Time) string {
	duration := time.Since(t)
	if duration.Hours() >= 24 {
		days := int(duration.Hours() / 24)
		return fmt.Sprintf("%dd", days)
	}
	if duration.Hours() >= 1 {
		return fmt.Sprintf("%dh", int(duration.Hours()))
	}
	if duration.Minutes() >= 1 {
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	}
	return fmt.Sprintf("%ds", int(duration.Seconds()))
}

// GetYaml 获取 IngressClass 的 YAML
func (i *ingressClassOperator) GetYaml(name string) (string, error) {
	ic, err := i.Get(name)
	if err != nil {
		return "", err
	}

	ic.ManagedFields = nil

	yamlBytes, err := yaml.Marshal(ic)
	if err != nil {
		return "", fmt.Errorf("转换为YAML失败: %v", err)
	}

	return string(yamlBytes), nil
}

// Describe 获取 IngressClass 详情
func (i *ingressClassOperator) Describe(name string) (string, error) {
	ic, err := i.Get(name)
	if err != nil {
		return "", err
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Name:         %s\n", ic.Name))
	sb.WriteString(fmt.Sprintf("Labels:       %s\n", i.formatLabels(ic.Labels)))
	sb.WriteString(fmt.Sprintf("Annotations:  %s\n", i.formatAnnotations(ic.Annotations)))
	sb.WriteString(fmt.Sprintf("Controller:   %s\n", ic.Spec.Controller))

	// Parameters
	if ic.Spec.Parameters != nil {
		sb.WriteString("Parameters:\n")
		if ic.Spec.Parameters.APIGroup != nil {
			sb.WriteString(fmt.Sprintf("  APIGroup:  %s\n", *ic.Spec.Parameters.APIGroup))
		}
		sb.WriteString(fmt.Sprintf("  Kind:      %s\n", ic.Spec.Parameters.Kind))
		sb.WriteString(fmt.Sprintf("  Name:      %s\n", ic.Spec.Parameters.Name))
		if ic.Spec.Parameters.Namespace != nil {
			sb.WriteString(fmt.Sprintf("  Namespace: %s\n", *ic.Spec.Parameters.Namespace))
		}
		if ic.Spec.Parameters.Scope != nil {
			sb.WriteString(fmt.Sprintf("  Scope:     %s\n", *ic.Spec.Parameters.Scope))
		}
	} else {
		sb.WriteString("Parameters:   <none>\n")
	}

	sb.WriteString(fmt.Sprintf("IsDefault:    %v\n", i.isDefaultIngressClass(ic)))

	// Events
	events, err := i.getEvents(ic.Name)
	if err == nil && len(events) > 0 {
		sb.WriteString("Events:\n")
		sb.WriteString("  Type    Reason  Age  From  Message\n")
		sb.WriteString("  ----    ------  ---  ----  -------\n")
		for _, event := range events {
			age := i.formatAge(time.UnixMilli(event.LastTimestamp))
			sb.WriteString(fmt.Sprintf("  %s  %s  %s  %s  %s\n",
				event.Type, event.Reason, age, event.Source, event.Message))
		}
	} else {
		sb.WriteString("Events:       <none>\n")
	}

	return sb.String(), nil
}

// formatLabels 格式化标签
func (i *ingressClassOperator) formatLabels(labelMap map[string]string) string {
	if len(labelMap) == 0 {
		return "<none>"
	}
	parts := make([]string, 0, len(labelMap))
	for k, v := range labelMap {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(parts, ", ")
}

// formatAnnotations 格式化注解
func (i *ingressClassOperator) formatAnnotations(annotations map[string]string) string {
	if len(annotations) == 0 {
		return "<none>"
	}
	parts := make([]string, 0, len(annotations))
	for k, v := range annotations {
		if len(v) > 50 {
			v = v[:50] + "..."
		}
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(parts, ", ")
}

// getEvents 获取事件
func (i *ingressClassOperator) getEvents(name string) ([]types.EventInfo, error) {
	fieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=IngressClass", name)
	events, err := i.client.CoreV1().Events("").List(i.ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return nil, err
	}

	result := make([]types.EventInfo, 0, len(events.Items))
	for _, event := range events.Items {
		eventInfo := types.EventInfo{
			Type:               event.Type,
			Reason:             event.Reason,
			Message:            event.Message,
			Count:              event.Count,
			FirstTimestamp:     event.FirstTimestamp.UnixMilli(),
			LastTimestamp:      event.LastTimestamp.UnixMilli(),
			Source:             event.Source.Component,
			InvolvedObjectKind: event.InvolvedObject.Kind,
			InvolvedObjectName: event.InvolvedObject.Name,
			InvolvedObjectUID:  string(event.InvolvedObject.UID),
			ReportingComponent: event.ReportingController,
			ReportingInstance:  event.ReportingInstance,
			Action:             event.Action,
			Annotations:        event.Annotations,
		}

		// 处理 EventTime
		if !event.EventTime.IsZero() {
			eventInfo.EventTime = event.EventTime.UnixMilli()
		}

		// 处理 Series
		if event.Series != nil {
			eventInfo.Series = &types.EventSeries{
				Count:            event.Series.Count,
				LastObservedTime: event.Series.LastObservedTime.UnixMilli(),
			}
		}

		// 处理 Related
		if event.Related != nil {
			eventInfo.Related = &types.ObjectReference{
				Kind:            event.Related.Kind,
				Namespace:       event.Related.Namespace,
				Name:            event.Related.Name,
				UID:             string(event.Related.UID),
				APIVersion:      event.Related.APIVersion,
				ResourceVersion: event.Related.ResourceVersion,
				FieldPath:       event.Related.FieldPath,
			}
		}

		result = append(result, eventInfo)
	}

	return result, nil
}

// Watch 监听 IngressClass 变化
func (i *ingressClassOperator) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return i.client.NetworkingV1().IngressClasses().Watch(i.ctx, opts)
}

// UpdateLabels 更新标签
func (i *ingressClassOperator) UpdateLabels(name string, labelMap map[string]string) error {
	ic, err := i.Get(name)
	if err != nil {
		return err
	}

	if ic.Labels == nil {
		ic.Labels = make(map[string]string)
	}
	for k, v := range labelMap {
		ic.Labels[k] = v
	}

	_, err = i.Update(ic)
	return err
}

// UpdateAnnotations 更新注解
func (i *ingressClassOperator) UpdateAnnotations(name string, annotations map[string]string) error {
	ic, err := i.Get(name)
	if err != nil {
		return err
	}

	if ic.Annotations == nil {
		ic.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		ic.Annotations[k] = v
	}

	_, err = i.Update(ic)
	return err
}

// SetDefault 设置为默认 IngressClass
func (i *ingressClassOperator) SetDefault(name string) error {
	// 首先取消其他默认 IngressClass
	icList, err := i.client.NetworkingV1().IngressClasses().List(i.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("获取IngressClass列表失败: %v", err)
	}

	for _, ic := range icList.Items {
		if i.isDefaultIngressClass(&ic) && ic.Name != name {
			if err := i.UnsetDefault(ic.Name); err != nil {
				return fmt.Errorf("取消默认IngressClass %s 失败: %v", ic.Name, err)
			}
		}
	}

	// 设置新的默认 IngressClass
	ic, err := i.Get(name)
	if err != nil {
		return err
	}

	if ic.Annotations == nil {
		ic.Annotations = make(map[string]string)
	}
	ic.Annotations[DefaultIngressClassAnnotation] = "true"

	_, err = i.Update(ic)
	return err
}

// UnsetDefault 取消默认 IngressClass
func (i *ingressClassOperator) UnsetDefault(name string) error {
	ic, err := i.Get(name)
	if err != nil {
		return err
	}

	if ic.Annotations == nil {
		return nil
	}

	delete(ic.Annotations, DefaultIngressClassAnnotation)

	_, err = i.Update(ic)
	return err
}

// GetUsage 获取使用该 IngressClass 的 Ingress 列表
func (i *ingressClassOperator) GetUsage(name string) (*types.IngressClassUsageResponse, error) {
	// 验证 IngressClass 存在
	ic, err := i.Get(name)
	if err != nil {
		return nil, err
	}

	response := &types.IngressClassUsageResponse{
		IngressClassName: name,
		IsDefault:        i.isDefaultIngressClass(ic),
		Ingresses:        make([]types.IngressRefInfo, 0),
		NamespaceStats:   make(map[string]int),
	}

	// 获取所有 Ingress
	var ingresses []*networkingv1.Ingress
	if i.useInformer && i.ingressLister != nil {
		ingresses, err = i.ingressLister.List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("获取Ingress列表失败: %v", err)
		}
	} else {
		ingressList, err := i.client.NetworkingV1().Ingresses("").List(i.ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("获取Ingress列表失败: %v", err)
		}
		ingresses = make([]*networkingv1.Ingress, len(ingressList.Items))
		for idx := range ingressList.Items {
			ingresses[idx] = &ingressList.Items[idx]
		}
	}

	// 过滤使用该 IngressClass 的 Ingress
	namespaceSet := make(map[string]bool)
	for _, ing := range ingresses {
		ingressClassName := ""
		if ing.Spec.IngressClassName != nil {
			ingressClassName = *ing.Spec.IngressClassName
		} else if val, ok := ing.Annotations["kubernetes.io/ingress.class"]; ok {
			// 兼容旧版注解
			ingressClassName = val
		}

		// 如果 Ingress 没有指定 IngressClass 且当前是默认的，也算匹配
		if ingressClassName == name || (ingressClassName == "" && response.IsDefault) {
			ingInfo := i.toIngressRefInfo(ing)
			response.Ingresses = append(response.Ingresses, ingInfo)

			// 统计命名空间
			response.NamespaceStats[ing.Namespace]++
			namespaceSet[ing.Namespace] = true
		}
	}

	response.IngressCount = len(response.Ingresses)
	response.NamespaceCount = len(namespaceSet)

	response.CanDelete = response.IngressCount == 0
	if !response.CanDelete {
		response.DeleteWarning = fmt.Sprintf("IngressClass被%d个Ingress使用，分布在%d个命名空间中，删除可能导致这些Ingress无法正常工作",
			response.IngressCount, response.NamespaceCount)
	}

	// 按命名空间和名称排序
	sort.Slice(response.Ingresses, func(a, b int) bool {
		if response.Ingresses[a].Namespace != response.Ingresses[b].Namespace {
			return response.Ingresses[a].Namespace < response.Ingresses[b].Namespace
		}
		return response.Ingresses[a].Name < response.Ingresses[b].Name
	})

	return response, nil
}

// toIngressRefInfo 转换为 IngressRefInfo
func (i *ingressClassOperator) toIngressRefInfo(ing *networkingv1.Ingress) types.IngressRefInfo {
	// 收集主机名
	hosts := make([]string, 0)
	for _, rule := range ing.Spec.Rules {
		if rule.Host != "" {
			hosts = append(hosts, rule.Host)
		}
	}

	// 收集地址
	addresses := make([]string, 0)
	for _, lb := range ing.Status.LoadBalancer.Ingress {
		if lb.IP != "" {
			addresses = append(addresses, lb.IP)
		}
		if lb.Hostname != "" {
			addresses = append(addresses, lb.Hostname)
		}
	}
	address := strings.Join(addresses, ", ")
	if address == "" {
		address = "<pending>"
	}

	// 计算后端数量
	backendCount := 0
	for _, rule := range ing.Spec.Rules {
		if rule.HTTP != nil {
			backendCount += len(rule.HTTP.Paths)
		}
	}
	if ing.Spec.DefaultBackend != nil {
		backendCount++
	}

	return types.IngressRefInfo{
		Name:         ing.Name,
		Namespace:    ing.Namespace,
		Hosts:        hosts,
		Address:      address,
		TLSEnabled:   len(ing.Spec.TLS) > 0,
		RulesCount:   len(ing.Spec.Rules),
		BackendCount: backendCount,
		Age:          i.formatAge(ing.CreationTimestamp.Time),
	}
}

// GetIngressesByClass 根据 IngressClass 获取所有 Ingress
func (i *ingressClassOperator) GetIngressesByClass(name string, namespace string) ([]types.IngressRefInfo, error) {
	usage, err := i.GetUsage(name)
	if err != nil {
		return nil, err
	}

	// 如果指定了命名空间，进行过滤
	if namespace != "" {
		filtered := make([]types.IngressRefInfo, 0)
		for _, ing := range usage.Ingresses {
			if ing.Namespace == namespace {
				filtered = append(filtered, ing)
			}
		}
		return filtered, nil
	}

	return usage.Ingresses, nil
}

// CanDelete 检查是否可以安全删除
func (i *ingressClassOperator) CanDelete(name string) (bool, string, error) {
	usage, err := i.GetUsage(name)
	if err != nil {
		return false, "", err
	}

	return usage.CanDelete, usage.DeleteWarning, nil
}

// GetControllerStatus 获取控制器状态
func (i *ingressClassOperator) GetControllerStatus(name string) (*types.IngressClassControllerStatus, error) {
	ic, err := i.Get(name)
	if err != nil {
		return nil, err
	}

	status := &types.IngressClassControllerStatus{
		IngressClassName: name,
		ControllerName:   ic.Spec.Controller,
		ControllerPods:   make([]string, 0),
	}

	// 尝试根据 controller 名称查找对应的 Pod
	// 常见的 ingress controller: nginx, traefik, haproxy 等
	controllerName := ic.Spec.Controller

	// 根据 controller 名称推断可能的命名空间和标签
	var namespace string
	var labelSelector string

	switch {
	case strings.Contains(controllerName, "nginx"):
		namespace = "ingress-nginx"
		labelSelector = "app.kubernetes.io/name=ingress-nginx"
	case strings.Contains(controllerName, "traefik"):
		namespace = "traefik"
		labelSelector = "app.kubernetes.io/name=traefik"
	case strings.Contains(controllerName, "haproxy"):
		namespace = "haproxy-controller"
		labelSelector = "app.kubernetes.io/name=haproxy-ingress"
	default:
		// 无法确定，返回基本信息
		return status, nil
	}

	status.Namespace = namespace

	// 查找 controller Pod
	pods, err := i.client.CoreV1().Pods(namespace).List(i.ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		// 尝试在所有命名空间查找
		pods, err = i.client.CoreV1().Pods("").List(i.ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			return status, nil
		}
	}

	readyCount := 0
	for _, pod := range pods.Items {
		status.ControllerPods = append(status.ControllerPods, fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))
		if pod.Namespace != "" {
			status.Namespace = pod.Namespace
		}

		// 检查 Pod 是否 Ready
		for _, cond := range pod.Status.Conditions {
			if cond.Type == "Ready" && cond.Status == "True" {
				readyCount++
				break
			}
		}
	}

	status.ControllerReplicas = len(pods.Items)
	status.ControllerReady = readyCount > 0 && readyCount == len(pods.Items)

	return status, nil
}

// GetDefaultIngressClass 获取默认 IngressClass
func (i *ingressClassOperator) GetDefaultIngressClass() (*networkingv1.IngressClass, error) {
	var ingressClasses []*networkingv1.IngressClass
	var err error

	if i.useInformer && i.ingressClassLister != nil {
		ingressClasses, err = i.ingressClassLister.List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("获取IngressClass列表失败: %v", err)
		}
	} else {
		icList, err := i.client.NetworkingV1().IngressClasses().List(i.ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("获取IngressClass列表失败: %v", err)
		}
		ingressClasses = make([]*networkingv1.IngressClass, len(icList.Items))
		for idx := range icList.Items {
			ingressClasses[idx] = &icList.Items[idx]
		}
	}

	for _, ic := range ingressClasses {
		if i.isDefaultIngressClass(ic) {
			result := ic.DeepCopy()
			i.injectTypeMeta(result)
			return result, nil
		}
	}

	return nil, fmt.Errorf("未找到默认IngressClass")
}
