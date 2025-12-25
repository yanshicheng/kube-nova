package operator

import (
	"context"
	"fmt"
	"time"

	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	vpav1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	vpaversioned "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/client/clientset/versioned"
	vpainformers "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/client/informers/externalversions"
	vpav1lister "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/client/listers/autoscaling.k8s.io/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	autoscalingv2lister "k8s.io/client-go/listers/autoscaling/v2"
	"sigs.k8s.io/yaml"
)

// ==================== HPA Operator 实现 ====================

type hpaOperator struct {
	BaseOperator
	client          kubernetes.Interface
	informerFactory informers.SharedInformerFactory
	hpaLister       autoscalingv2lister.HorizontalPodAutoscalerLister
}

// NewHPAOperator 创建 HPA Operator（不使用 informer）
func NewHPAOperator(ctx context.Context, client kubernetes.Interface) types.HPAOperator {
	return &hpaOperator{
		BaseOperator: NewBaseOperator(ctx, false),
		client:       client,
	}
}

// NewHPAOperatorWithInformer 创建 HPA Operator（使用 informer）
func NewHPAOperatorWithInformer(
	ctx context.Context,
	client kubernetes.Interface,
	informerFactory informers.SharedInformerFactory,
) types.HPAOperator {
	var hpaLister autoscalingv2lister.HorizontalPodAutoscalerLister

	if informerFactory != nil {
		hpaLister = informerFactory.Autoscaling().V2().HorizontalPodAutoscalers().Lister()
	}

	return &hpaOperator{
		BaseOperator:    NewBaseOperator(ctx, informerFactory != nil),
		client:          client,
		informerFactory: informerFactory,
		hpaLister:       hpaLister,
	}
}

func (h *hpaOperator) Create(hpa *autoscalingv2.HorizontalPodAutoscaler) (*autoscalingv2.HorizontalPodAutoscaler, error) {
	if hpa == nil || hpa.Name == "" || hpa.Namespace == "" {
		return nil, fmt.Errorf("HPA对象、名称和命名空间不能为空")
	}

	created, err := h.client.AutoscalingV2().HorizontalPodAutoscalers(hpa.Namespace).Create(h.ctx, hpa, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("创建HPA失败: %v", err)
	}

	return created, nil
}

func (h *hpaOperator) Get(namespace, name string) (*autoscalingv2.HorizontalPodAutoscaler, error) {
	if namespace == "" || name == "" {
		return nil, fmt.Errorf("命名空间和名称不能为空")
	}

	// 优先使用 informer
	if h.useInformer && h.hpaLister != nil {
		hpa, err := h.hpaLister.HorizontalPodAutoscalers(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, nil
			}
			// informer 出错时回退到 API 调用
			hpa, apiErr := h.client.AutoscalingV2().HorizontalPodAutoscalers(namespace).Get(h.ctx, name, metav1.GetOptions{})
			if apiErr != nil {
				if errors.IsNotFound(apiErr) {
					return nil, nil
				}
				return nil, fmt.Errorf("获取HPA失败: %v", apiErr)
			}
			hpa.TypeMeta = metav1.TypeMeta{
				Kind:       "HorizontalPodAutoscaler",
				APIVersion: "autoscaling/v2",
			}
			return hpa, nil
		}
		hpa.TypeMeta = metav1.TypeMeta{
			Kind:       "HorizontalPodAutoscaler",
			APIVersion: "autoscaling/v2",
		}
		return hpa, nil
	}

	// 使用 API 调用
	hpa, err := h.client.AutoscalingV2().HorizontalPodAutoscalers(namespace).Get(h.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("获取HPA失败: %v", err)
	}

	hpa.TypeMeta = metav1.TypeMeta{
		Kind:       "HorizontalPodAutoscaler",
		APIVersion: "autoscaling/v2",
	}

	return hpa, nil
}

func (h *hpaOperator) Update(hpa *autoscalingv2.HorizontalPodAutoscaler) (*autoscalingv2.HorizontalPodAutoscaler, error) {
	if hpa == nil || hpa.Name == "" || hpa.Namespace == "" {
		return nil, fmt.Errorf("HPA对象、名称和命名空间不能为空")
	}

	updated, err := h.client.AutoscalingV2().HorizontalPodAutoscalers(hpa.Namespace).Update(h.ctx, hpa, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("更新HPA失败: %v", err)
	}

	return updated, nil
}

func (h *hpaOperator) Delete(namespace, name string) error {
	if namespace == "" || name == "" {
		return fmt.Errorf("命名空间和名称不能为空")
	}

	err := h.client.AutoscalingV2().HorizontalPodAutoscalers(namespace).Delete(h.ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("删除HPA失败: %v", err)
	}

	return nil
}

func (h *hpaOperator) GetDetail(namespace, name string) (*types.HPADetail, error) {
	hpa, err := h.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	return h.convertToHPADetail(hpa), nil
}

func (h *hpaOperator) GetYaml(namespace, name string) (string, error) {
	hpa, err := h.Get(namespace, name)
	if err != nil {
		return "", err
	}

	if hpa == nil {
		return "", fmt.Errorf("HPA %s/%s 不存在", namespace, name)
	}

	// ✅ 确保有 TypeMeta（Get 方法已经注入了，这里再确保一次）
	hpa.TypeMeta = metav1.TypeMeta{
		Kind:       "HorizontalPodAutoscaler",
		APIVersion: "autoscaling/v2",
	}

	// 清理不需要的字段
	hpa.ManagedFields = nil
	hpa.Status = autoscalingv2.HorizontalPodAutoscalerStatus{}

	yamlBytes, err := yaml.Marshal(hpa)
	if err != nil {
		return "", fmt.Errorf("转换为YAML失败: %v", err)
	}

	return string(yamlBytes), nil
}
func (h *hpaOperator) Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return h.client.AutoscalingV2().HorizontalPodAutoscalers(namespace).Watch(h.ctx, opts)
}

func (h *hpaOperator) UpdateLabels(namespace, name string, labels map[string]string) error {
	hpa, err := h.Get(namespace, name)
	if err != nil {
		return err
	}

	if hpa.Labels == nil {
		hpa.Labels = make(map[string]string)
	}
	for k, v := range labels {
		hpa.Labels[k] = v
	}

	_, err = h.Update(hpa)
	return err
}

func (h *hpaOperator) UpdateAnnotations(namespace, name string, annotations map[string]string) error {
	hpa, err := h.Get(namespace, name)
	if err != nil {
		return err
	}

	if hpa.Annotations == nil {
		hpa.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		hpa.Annotations[k] = v
	}

	_, err = h.Update(hpa)
	return err
}

func (h *hpaOperator) CreateFromYaml(namespace string, yamlStr string) (*autoscalingv2.HorizontalPodAutoscaler, error) {
	if yamlStr == "" {
		return nil, fmt.Errorf("YAML字符串不能为空")
	}

	hpa := &autoscalingv2.HorizontalPodAutoscaler{}
	err := yaml.Unmarshal([]byte(yamlStr), hpa)
	if err != nil {
		return nil, fmt.Errorf("解析YAML失败: %v", err)
	}

	// 如果 YAML 中没有指定 namespace，使用传入的 namespace
	if hpa.Namespace == "" {
		hpa.Namespace = namespace
	}

	return h.Create(hpa)
}

func (h *hpaOperator) UpdateFromYaml(namespace, name string, yamlStr string) (*autoscalingv2.HorizontalPodAutoscaler, error) {
	if yamlStr == "" {
		return nil, fmt.Errorf("YAML字符串不能为空")
	}

	hpa := &autoscalingv2.HorizontalPodAutoscaler{}
	err := yaml.Unmarshal([]byte(yamlStr), hpa)
	if err != nil {
		return nil, fmt.Errorf("解析YAML失败: %v", err)
	}

	// 确保 namespace 和 name 正确
	hpa.Namespace = namespace
	hpa.Name = name

	// 获取现有的 HPA 以保留 ResourceVersion
	existing, err := h.Get(namespace, name)
	if err != nil {
		return nil, err
	}
	hpa.ResourceVersion = existing.ResourceVersion

	return h.Update(hpa)
}

// convertToHPADetail 将 K8s HPA 对象转换为详情响应
func (h *hpaOperator) convertToHPADetail(hpa *autoscalingv2.HorizontalPodAutoscaler) *types.HPADetail {
	detail := &types.HPADetail{
		Name:              hpa.Name,
		Namespace:         hpa.Namespace,
		MinReplicas:       hpa.Spec.MinReplicas,
		MaxReplicas:       hpa.Spec.MaxReplicas,
		CurrentReplicas:   hpa.Status.CurrentReplicas,
		DesiredReplicas:   hpa.Status.DesiredReplicas,
		Labels:            hpa.Labels,
		Annotations:       hpa.Annotations,
		Age:               time.Since(hpa.CreationTimestamp.Time).String(),
		CreationTimestamp: hpa.CreationTimestamp.UnixMilli(),
	}

	// 转换 TargetRef
	detail.TargetRef = types.TargetRefInfo{
		APIVersion: hpa.Spec.ScaleTargetRef.APIVersion,
		Kind:       hpa.Spec.ScaleTargetRef.Kind,
		Name:       hpa.Spec.ScaleTargetRef.Name,
	}

	// 转换 Metrics
	detail.Metrics = make([]types.HPAMetric, len(hpa.Spec.Metrics))
	for i, metric := range hpa.Spec.Metrics {
		detail.Metrics[i] = h.convertMetricSpec(metric)
	}

	// 转换 Behavior
	if hpa.Spec.Behavior != nil {
		detail.Behavior = h.convertBehavior(hpa.Spec.Behavior)
	}

	// 转换 CurrentMetrics
	if len(hpa.Status.CurrentMetrics) > 0 {
		detail.CurrentMetrics = make([]types.HPACurrentMetric, len(hpa.Status.CurrentMetrics))
		for i, metric := range hpa.Status.CurrentMetrics {
			detail.CurrentMetrics[i] = h.convertCurrentMetric(metric)
		}
	}

	// 转换 Conditions
	if len(hpa.Status.Conditions) > 0 {
		detail.Conditions = make([]types.HPACondition, len(hpa.Status.Conditions))
		for i, cond := range hpa.Status.Conditions {
			detail.Conditions[i] = types.HPACondition{
				Type:               string(cond.Type),
				Status:             string(cond.Status),
				Reason:             cond.Reason,
				Message:            cond.Message,
				LastTransitionTime: cond.LastTransitionTime.UnixMilli(),
			}
		}
	}

	return detail
}

func (h *hpaOperator) convertMetricSpec(metric autoscalingv2.MetricSpec) types.HPAMetric {
	result := types.HPAMetric{
		Type: string(metric.Type),
	}

	switch metric.Type {
	case autoscalingv2.ResourceMetricSourceType:
		if metric.Resource != nil {
			result.Resource = &types.HPAResourceMetric{
				Name:   string(metric.Resource.Name),
				Target: h.convertMetricTarget(metric.Resource.Target),
			}
		}
	case autoscalingv2.PodsMetricSourceType:
		if metric.Pods != nil {
			result.Pods = &types.HPAPodsMetric{
				Metric: types.HPAMetricDef{
					Name:     metric.Pods.Metric.Name,
					Selector: metricSelectorToMap(metric.Pods.Metric.Selector),
				},
				Target: h.convertMetricTarget(metric.Pods.Target),
			}
		}
	case autoscalingv2.ObjectMetricSourceType:
		if metric.Object != nil {
			result.Object = &types.HPAObjectMetric{
				Metric: types.HPAMetricDef{
					Name:     metric.Object.Metric.Name,
					Selector: metricSelectorToMap(metric.Object.Metric.Selector),
				},
				DescribedObject: types.HPADescribedObject{
					APIVersion: metric.Object.DescribedObject.APIVersion,
					Kind:       metric.Object.DescribedObject.Kind,
					Name:       metric.Object.DescribedObject.Name,
				},
				Target: h.convertMetricTarget(metric.Object.Target),
			}
		}
	case autoscalingv2.ExternalMetricSourceType:
		if metric.External != nil {
			result.External = &types.HPAExternalMetric{
				Metric: types.HPAMetricDef{
					Name:     metric.External.Metric.Name,
					Selector: metricSelectorToMap(metric.External.Metric.Selector),
				},
				Target: h.convertMetricTarget(metric.External.Target),
			}
		}
	case autoscalingv2.ContainerResourceMetricSourceType:
		if metric.ContainerResource != nil {
			result.ContainerResource = &types.HPAContainerResourceMetric{
				Name:      string(metric.ContainerResource.Name),
				Container: metric.ContainerResource.Container,
				Target:    h.convertMetricTarget(metric.ContainerResource.Target),
			}
		}
	}

	return result
}

func (h *hpaOperator) convertMetricTarget(target autoscalingv2.MetricTarget) types.HPAMetricTarget {
	result := types.HPAMetricTarget{
		Type: string(target.Type),
	}

	if target.AverageUtilization != nil {
		result.AverageUtilization = target.AverageUtilization
	}
	if target.AverageValue != nil {
		result.AverageValue = target.AverageValue.String()
	}
	if target.Value != nil {
		result.Value = target.Value.String()
	}

	return result
}

func (h *hpaOperator) convertBehavior(behavior *autoscalingv2.HorizontalPodAutoscalerBehavior) *types.HPABehavior {
	result := &types.HPABehavior{}

	if behavior.ScaleUp != nil {
		result.ScaleUp = h.convertScalingRules(behavior.ScaleUp)
	}
	if behavior.ScaleDown != nil {
		result.ScaleDown = h.convertScalingRules(behavior.ScaleDown)
	}

	return result
}

func (h *hpaOperator) convertScalingRules(rules *autoscalingv2.HPAScalingRules) *types.HPAScalingRules {
	result := &types.HPAScalingRules{
		StabilizationWindowSeconds: rules.StabilizationWindowSeconds,
		SelectPolicy:               string(*rules.SelectPolicy),
	}

	if len(rules.Policies) > 0 {
		result.Policies = make([]types.HPAScalingPolicy, len(rules.Policies))
		for i, policy := range rules.Policies {
			result.Policies[i] = types.HPAScalingPolicy{
				Type:          string(policy.Type),
				Value:         policy.Value,
				PeriodSeconds: policy.PeriodSeconds,
			}
		}
	}

	return result
}

func (h *hpaOperator) convertCurrentMetric(metric autoscalingv2.MetricStatus) types.HPACurrentMetric {
	result := types.HPACurrentMetric{
		Type: string(metric.Type),
	}

	var current *autoscalingv2.MetricValueStatus

	switch metric.Type {
	case autoscalingv2.ResourceMetricSourceType:
		if metric.Resource != nil {
			current = &metric.Resource.Current
		}
	case autoscalingv2.PodsMetricSourceType:
		if metric.Pods != nil {
			current = &metric.Pods.Current
		}
	case autoscalingv2.ObjectMetricSourceType:
		if metric.Object != nil {
			current = &metric.Object.Current
		}
	case autoscalingv2.ExternalMetricSourceType:
		if metric.External != nil {
			current = &metric.External.Current
		}
	case autoscalingv2.ContainerResourceMetricSourceType:
		if metric.ContainerResource != nil {
			current = &metric.ContainerResource.Current
		}
	}

	if current != nil {
		if current.AverageUtilization != nil {
			result.Current.AverageUtilization = current.AverageUtilization
		}
		if current.AverageValue != nil {
			result.Current.AverageValue = current.AverageValue.String()
		}
		if current.Value != nil {
			result.Current.Value = current.Value.String()
		}
	}

	return result
}

// metricSelectorToMap 将 LabelSelector 转换为 map
func metricSelectorToMap(selector *metav1.LabelSelector) map[string]string {
	if selector == nil {
		return nil
	}
	return selector.MatchLabels
}

// GetByTargetRef 通过 TargetRef 查询 HPA
func (h *hpaOperator) GetByTargetRef(namespace string, targetRef types.TargetRefInfo) (*autoscalingv2.HorizontalPodAutoscaler, error) {
	if namespace == "" {
		return nil, fmt.Errorf("namespace cannot be empty")
	}
	if targetRef.Kind == "" || targetRef.Name == "" {
		return nil, fmt.Errorf("targetRef kind and name are required")
	}

	hpaList, err := h.client.AutoscalingV2().HorizontalPodAutoscalers(namespace).List(
		context.TODO(),
		metav1.ListOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list HPAs in namespace %s: %w", namespace, err)
	}

	for i := range hpaList.Items {
		hpa := &hpaList.Items[i]
		scaleTargetRef := hpa.Spec.ScaleTargetRef

		if scaleTargetRef.Kind == targetRef.Kind && scaleTargetRef.Name == targetRef.Name {
			if targetRef.APIVersion == "" || scaleTargetRef.APIVersion == targetRef.APIVersion {
				hpa.TypeMeta = metav1.TypeMeta{
					Kind:       "HorizontalPodAutoscaler",
					APIVersion: "autoscaling/v2",
				}
				return hpa, nil
			}
		}
	}

	return nil, nil
}

// GetDetailByTargetRef 通过 TargetRef 查询 HPA 详情
func (h *hpaOperator) GetDetailByTargetRef(namespace string, targetRef types.TargetRefInfo) (*types.HPADetail, error) {
	// 先通过 TargetRef 获取 HPA
	hpa, err := h.GetByTargetRef(namespace, targetRef)
	if err != nil {
		return nil, err
	}
	if hpa == nil {
		return nil, nil // 没有找到 HPA，返回 nil 而不是错误
	}
	// 转换为详情对象
	return h.GetDetail(namespace, hpa.Name)
}

// ==================== VPA Operator 实现 ====================

type vpaOperator struct {
	BaseOperator
	client          vpaversioned.Interface
	k8sClient       kubernetes.Interface
	informerFactory vpainformers.SharedInformerFactory
	vpaLister       vpav1lister.VerticalPodAutoscalerLister
}

// NewVPAOperator 创建 VPA Operator（不使用 informer）
func NewVPAOperator(ctx context.Context, vpaClient vpaversioned.Interface, k8sClient kubernetes.Interface) types.VPAOperator {
	return &vpaOperator{
		BaseOperator: NewBaseOperator(ctx, false),
		client:       vpaClient,
		k8sClient:    k8sClient,
	}
}

// NewVPAOperatorWithInformer 创建 VPA Operator（使用 informer）
func NewVPAOperatorWithInformer(
	ctx context.Context,
	vpaClient vpaversioned.Interface,
	k8sClient kubernetes.Interface,
	informerFactory vpainformers.SharedInformerFactory,
) types.VPAOperator {
	var vpaLister vpav1lister.VerticalPodAutoscalerLister

	if informerFactory != nil {
		vpaLister = informerFactory.Autoscaling().V1().VerticalPodAutoscalers().Lister()
	}

	return &vpaOperator{
		BaseOperator:    NewBaseOperator(ctx, informerFactory != nil),
		client:          vpaClient,
		k8sClient:       k8sClient,
		informerFactory: informerFactory,
		vpaLister:       vpaLister,
	}
}

func (v *vpaOperator) Create(vpa *vpav1.VerticalPodAutoscaler) (*vpav1.VerticalPodAutoscaler, error) {
	if vpa == nil || vpa.Name == "" || vpa.Namespace == "" {
		return nil, fmt.Errorf("VPA对象、名称和命名空间不能为空")
	}

	created, err := v.client.AutoscalingV1().VerticalPodAutoscalers(vpa.Namespace).Create(v.ctx, vpa, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("创建VPA失败: %v", err)
	}

	return created, nil
}

func (v *vpaOperator) Get(namespace, name string) (*vpav1.VerticalPodAutoscaler, error) {
	if namespace == "" || name == "" {
		return nil, fmt.Errorf("命名空间和名称不能为空")
	}

	// 优先使用 informer
	if v.useInformer && v.vpaLister != nil {
		vpa, err := v.vpaLister.VerticalPodAutoscalers(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, nil
			}
			// informer 出错时回退到 API 调用
			vpa, apiErr := v.client.AutoscalingV1().VerticalPodAutoscalers(namespace).Get(v.ctx, name, metav1.GetOptions{})
			if apiErr != nil {
				if errors.IsNotFound(apiErr) {
					return nil, nil
				}
				return nil, fmt.Errorf("获取VPA失败: %v", apiErr)
			}
			vpa.TypeMeta = metav1.TypeMeta{
				Kind:       "VerticalPodAutoscaler",
				APIVersion: "autoscaling.k8s.io/v1",
			}
			return vpa, nil
		}
		vpa.TypeMeta = metav1.TypeMeta{
			Kind:       "VerticalPodAutoscaler",
			APIVersion: "autoscaling.k8s.io/v1",
		}
		return vpa, nil
	}

	// 使用 API 调用
	vpa, err := v.client.AutoscalingV1().VerticalPodAutoscalers(namespace).Get(v.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("获取VPA失败: %v", err)
	}

	vpa.TypeMeta = metav1.TypeMeta{
		Kind:       "VerticalPodAutoscaler",
		APIVersion: "autoscaling.k8s.io/v1",
	}

	return vpa, nil
}

func (v *vpaOperator) Update(vpa *vpav1.VerticalPodAutoscaler) (*vpav1.VerticalPodAutoscaler, error) {
	if vpa == nil || vpa.Name == "" || vpa.Namespace == "" {
		return nil, fmt.Errorf("VPA对象、名称和命名空间不能为空")
	}

	updated, err := v.client.AutoscalingV1().VerticalPodAutoscalers(vpa.Namespace).Update(v.ctx, vpa, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("更新VPA失败: %v", err)
	}

	return updated, nil
}

func (v *vpaOperator) Delete(namespace, name string) error {
	if namespace == "" || name == "" {
		return fmt.Errorf("命名空间和名称不能为空")
	}

	err := v.client.AutoscalingV1().VerticalPodAutoscalers(namespace).Delete(v.ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("删除VPA失败: %v", err)
	}

	return nil
}

func (v *vpaOperator) GetDetail(namespace, name string) (*types.VPADetail, error) {
	vpa, err := v.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	return v.convertToVPADetail(vpa), nil
}

func (v *vpaOperator) GetYaml(namespace, name string) (string, error) {
	vpa, err := v.Get(namespace, name)
	if err != nil {
		return "", err
	}

	if vpa == nil {
		return "", fmt.Errorf("VPA %s/%s 不存在", namespace, name)
	}

	// ✅ 确保有 TypeMeta
	vpa.TypeMeta = metav1.TypeMeta{
		Kind:       "VerticalPodAutoscaler",
		APIVersion: "autoscaling.k8s.io/v1",
	}

	// 清理不需要的字段
	vpa.ManagedFields = nil
	vpa.Status = vpav1.VerticalPodAutoscalerStatus{}

	yamlBytes, err := yaml.Marshal(vpa)
	if err != nil {
		return "", fmt.Errorf("转换为YAML失败: %v", err)
	}

	return string(yamlBytes), nil
}

func (v *vpaOperator) Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return v.client.AutoscalingV1().VerticalPodAutoscalers(namespace).Watch(v.ctx, opts)
}

func (v *vpaOperator) UpdateLabels(namespace, name string, labels map[string]string) error {
	vpa, err := v.Get(namespace, name)
	if err != nil {
		return err
	}

	if vpa.Labels == nil {
		vpa.Labels = make(map[string]string)
	}
	for k, v := range labels {
		vpa.Labels[k] = v
	}

	_, err = v.Update(vpa)
	return err
}

func (v *vpaOperator) UpdateAnnotations(namespace, name string, annotations map[string]string) error {
	vpa, err := v.Get(namespace, name)
	if err != nil {
		return err
	}

	if vpa.Annotations == nil {
		vpa.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		vpa.Annotations[k] = v
	}

	_, err = v.Update(vpa)
	return err
}

func (v *vpaOperator) CreateFromYaml(namespace string, yamlStr string) (*vpav1.VerticalPodAutoscaler, error) {
	if yamlStr == "" {
		return nil, fmt.Errorf("YAML字符串不能为空")
	}

	vpa := &vpav1.VerticalPodAutoscaler{}
	err := yaml.Unmarshal([]byte(yamlStr), vpa)
	if err != nil {
		return nil, fmt.Errorf("解析YAML失败: %v", err)
	}

	// 如果 YAML 中没有指定 namespace，使用传入的 namespace
	if vpa.Namespace == "" {
		vpa.Namespace = namespace
	}

	return v.Create(vpa)
}

func (v *vpaOperator) UpdateFromYaml(namespace, name string, yamlStr string) (*vpav1.VerticalPodAutoscaler, error) {
	if yamlStr == "" {
		return nil, fmt.Errorf("YAML字符串不能为空")
	}

	vpa := &vpav1.VerticalPodAutoscaler{}
	err := yaml.Unmarshal([]byte(yamlStr), vpa)
	if err != nil {
		return nil, fmt.Errorf("解析YAML失败: %v", err)
	}

	// 确保 namespace 和 name 正确
	vpa.Namespace = namespace
	vpa.Name = name

	// 获取现有的 VPA 以保留 ResourceVersion
	existing, err := v.Get(namespace, name)
	if err != nil {
		return nil, err
	}
	vpa.ResourceVersion = existing.ResourceVersion

	return v.Update(vpa)
}

// convertToVPADetail 将 K8s VPA 对象转换为详情响应
func (v *vpaOperator) convertToVPADetail(vpa *vpav1.VerticalPodAutoscaler) *types.VPADetail {
	detail := &types.VPADetail{
		Name:              vpa.Name,
		Namespace:         vpa.Namespace,
		Labels:            vpa.Labels,
		Annotations:       vpa.Annotations,
		Age:               time.Since(vpa.CreationTimestamp.Time).String(),
		CreationTimestamp: vpa.CreationTimestamp.UnixMilli(),
	}

	// 转换 TargetRef
	detail.TargetRef = types.TargetRefInfo{
		APIVersion: vpa.Spec.TargetRef.APIVersion,
		Kind:       vpa.Spec.TargetRef.Kind,
		Name:       vpa.Spec.TargetRef.Name,
	}

	// 转换 UpdatePolicy
	if vpa.Spec.UpdatePolicy != nil {
		detail.UpdatePolicy = &types.VPAUpdatePolicy{}
		if vpa.Spec.UpdatePolicy.UpdateMode != nil {
			detail.UpdatePolicy.UpdateMode = string(*vpa.Spec.UpdatePolicy.UpdateMode)
		}
		if vpa.Spec.UpdatePolicy.MinReplicas != nil {
			detail.UpdatePolicy.MinReplicas = vpa.Spec.UpdatePolicy.MinReplicas
		}
	}

	// 转换 ResourcePolicy
	if vpa.Spec.ResourcePolicy != nil && len(vpa.Spec.ResourcePolicy.ContainerPolicies) > 0 {
		detail.ResourcePolicy.ContainerPolicies = make([]types.VPAResourcePolicy, len(vpa.Spec.ResourcePolicy.ContainerPolicies))
		for i, policy := range vpa.Spec.ResourcePolicy.ContainerPolicies {
			detail.ResourcePolicy.ContainerPolicies[i] = v.convertResourcePolicy(policy)
		}
	}

	// 转换 Recommendation
	if vpa.Status.Recommendation != nil && len(vpa.Status.Recommendation.ContainerRecommendations) > 0 {
		detail.Recommendation.ContainerRecommendations = make([]types.VPARecommendation, len(vpa.Status.Recommendation.ContainerRecommendations))
		for i, rec := range vpa.Status.Recommendation.ContainerRecommendations {
			detail.Recommendation.ContainerRecommendations[i] = v.convertRecommendation(rec)
		}
	}

	// 转换 Conditions
	if len(vpa.Status.Conditions) > 0 {
		detail.Conditions = make([]types.VPACondition, len(vpa.Status.Conditions))
		for i, cond := range vpa.Status.Conditions {
			detail.Conditions[i] = types.VPACondition{
				Type:               string(cond.Type),
				Status:             string(cond.Status),
				Reason:             cond.Reason,
				Message:            cond.Message,
				LastTransitionTime: cond.LastTransitionTime.UnixMilli(),
			}
		}
	}

	return detail
}

func (v *vpaOperator) convertResourcePolicy(policy vpav1.ContainerResourcePolicy) types.VPAResourcePolicy {
	result := types.VPAResourcePolicy{
		ContainerName: policy.ContainerName,
	}

	if policy.Mode != nil {
		result.Mode = string(*policy.Mode)
	}

	if policy.MinAllowed != nil {
		result.MinAllowed = v.convertResourceList(policy.MinAllowed)
	}

	if policy.MaxAllowed != nil {
		result.MaxAllowed = v.convertResourceList(policy.MaxAllowed)
	}

	if policy.ControlledResources != nil && len(*policy.ControlledResources) > 0 {
		result.ControlledResources = make([]string, len(*policy.ControlledResources))
		for i, res := range *policy.ControlledResources {
			result.ControlledResources[i] = string(res)
		}
	}

	if policy.ControlledValues != nil {
		result.ControlledValues = string(*policy.ControlledValues)
	}

	return result
}

func (v *vpaOperator) convertResourceList(resources corev1.ResourceList) types.VPAResourceConstraints {
	result := types.VPAResourceConstraints{}

	if cpu, ok := resources[corev1.ResourceCPU]; ok {
		result.CPU = cpu.String()
	}
	if memory, ok := resources[corev1.ResourceMemory]; ok {
		result.Memory = memory.String()
	}

	return result
}

func (v *vpaOperator) convertRecommendation(rec vpav1.RecommendedContainerResources) types.VPARecommendation {
	result := types.VPARecommendation{
		ContainerName: rec.ContainerName,
	}

	if rec.Target != nil {
		result.Target = v.convertResourceRecommendation(rec.Target)
	}

	if rec.LowerBound != nil {
		result.LowerBound = v.convertResourceRecommendation(rec.LowerBound)
	}

	if rec.UpperBound != nil {
		result.UpperBound = v.convertResourceRecommendation(rec.UpperBound)
	}

	if rec.UncappedTarget != nil {
		result.UncappedTarget = v.convertResourceRecommendation(rec.UncappedTarget)
	}

	return result
}

func (v *vpaOperator) convertResourceRecommendation(resources corev1.ResourceList) types.VPAResourceRecommendation {
	result := types.VPAResourceRecommendation{}

	if cpu, ok := resources[corev1.ResourceCPU]; ok {
		result.CPU = cpu.String()
	}
	if memory, ok := resources[corev1.ResourceMemory]; ok {
		result.Memory = memory.String()
	}

	return result
}

// GetByTargetRef 通过 TargetRef 查询 VPA
func (v *vpaOperator) GetByTargetRef(namespace string, targetRef types.TargetRefInfo) (*vpav1.VerticalPodAutoscaler, error) {
	if namespace == "" {
		return nil, fmt.Errorf("namespace cannot be empty")
	}
	if targetRef.Kind == "" || targetRef.Name == "" {
		return nil, fmt.Errorf("targetRef kind and name are required")
	}

	vpaList, err := v.client.AutoscalingV1().VerticalPodAutoscalers(namespace).List(
		context.TODO(),
		metav1.ListOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list VPAs in namespace %s: %w", namespace, err)
	}

	for i := range vpaList.Items {
		vpa := &vpaList.Items[i]

		if vpa.Spec.TargetRef == nil {
			continue
		}

		vpaTargetRef := vpa.Spec.TargetRef

		if vpaTargetRef.Kind == targetRef.Kind && vpaTargetRef.Name == targetRef.Name {
			if targetRef.APIVersion == "" || vpaTargetRef.APIVersion == targetRef.APIVersion {
				// ✅ 手动注入 TypeMeta
				vpa.TypeMeta = metav1.TypeMeta{
					Kind:       "VerticalPodAutoscaler",
					APIVersion: "autoscaling.k8s.io/v1",
				}
				return vpa, nil
			}
		}
	}

	return nil, nil
}

// GetDetailByTargetRef 通过 TargetRef 查询 VPA 详情
func (v *vpaOperator) GetDetailByTargetRef(namespace string, targetRef types.TargetRefInfo) (*types.VPADetail, error) {
	// 先通过 TargetRef 获取 VPA
	vpa, err := v.GetByTargetRef(namespace, targetRef)
	if err != nil {
		return nil, err
	}
	if vpa == nil {
		return nil, nil // 没有找到 HPA，返回 nil 而不是错误
	}
	// 转换为详情对象
	return v.GetDetail(namespace, vpa.Name)
}
