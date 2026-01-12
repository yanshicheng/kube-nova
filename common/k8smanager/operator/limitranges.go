package operator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1lister "k8s.io/client-go/listers/core/v1"
	"sigs.k8s.io/yaml"
)

type limitRangeOperator struct {
	BaseOperator
	client           kubernetes.Interface
	informerFactory  informers.SharedInformerFactory
	limitRangeLister corev1lister.LimitRangeLister
}

func NewLimitRangeOperator(ctx context.Context, client kubernetes.Interface) types.LimitRangeOperator {
	return &limitRangeOperator{
		BaseOperator: NewBaseOperator(ctx, false),
		client:       client,
	}
}

func NewLimitRangeOperatorWithInformer(
	ctx context.Context,
	client kubernetes.Interface,
	informerFactory informers.SharedInformerFactory,
) types.LimitRangeOperator {
	var limitRangeLister corev1lister.LimitRangeLister

	if informerFactory != nil {
		limitRangeLister = informerFactory.Core().V1().LimitRanges().Lister()
	}

	return &limitRangeOperator{
		BaseOperator:     NewBaseOperator(ctx, informerFactory != nil),
		client:           client,
		informerFactory:  informerFactory,
		limitRangeLister: limitRangeLister,
	}
}

func (l *limitRangeOperator) Create(limitRange *corev1.LimitRange) (*corev1.LimitRange, error) {
	if limitRange == nil || limitRange.Name == "" || limitRange.Namespace == "" {
		return nil, fmt.Errorf("LimitRange对象、名称和命名空间不能为空")
	}
	injectCommonAnnotations(limitRange)
	created, err := l.client.CoreV1().LimitRanges(limitRange.Namespace).Create(l.ctx, limitRange, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("创建LimitRange失败: %v", err)
	}

	return created, nil
}

func (l *limitRangeOperator) Get(namespace, name string) (*corev1.LimitRange, error) {
	if namespace == "" || name == "" {
		return nil, fmt.Errorf("命名空间和名称不能为空")
	}

	// 优先使用 informer
	if l.useInformer && l.limitRangeLister != nil {
		limitRange, err := l.limitRangeLister.LimitRanges(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("LimitRange %s/%s 不存在", namespace, name)
			}
			// informer 出错时回退到 API 调用
			limitRange, apiErr := l.client.CoreV1().LimitRanges(namespace).Get(l.ctx, name, metav1.GetOptions{})
			if apiErr != nil {
				return nil, fmt.Errorf("获取LimitRange失败: %v", apiErr)
			}
			return limitRange, nil
		}
		return limitRange, nil
	}

	// 使用 API 调用
	limitRange, err := l.client.CoreV1().LimitRanges(namespace).Get(l.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("LimitRange %s/%s 不存在", namespace, name)
		}
		return nil, fmt.Errorf("获取LimitRange失败: %v", err)
	}

	return limitRange, nil
}

func (l *limitRangeOperator) Update(limitRange *corev1.LimitRange) (*corev1.LimitRange, error) {
	if limitRange == nil || limitRange.Name == "" || limitRange.Namespace == "" {
		return nil, fmt.Errorf("LimitRange对象、名称和命名空间不能为空")
	}

	updated, err := l.client.CoreV1().LimitRanges(limitRange.Namespace).Update(l.ctx, limitRange, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("更新LimitRange失败: %v", err)
	}

	return updated, nil
}

func (l *limitRangeOperator) Delete(namespace, name string) error {
	if namespace == "" || name == "" {
		return fmt.Errorf("命名空间和名称不能为空")
	}

	err := l.client.CoreV1().LimitRanges(namespace).Delete(l.ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("删除LimitRange失败: %v", err)
	}

	return nil
}

func (l *limitRangeOperator) List(namespace string, search string, labelSelector string) (*types.ListLimitRangeResponse, error) {
	var selector labels.Selector = labels.Everything()
	if labelSelector != "" {
		parsedSelector, err := labels.Parse(labelSelector)
		if err != nil {
			return nil, fmt.Errorf("解析标签选择器失败: %v", err)
		}
		selector = parsedSelector
	}

	var limitRanges []*corev1.LimitRange
	var err error

	// 优先使用 informer
	if l.useInformer && l.limitRangeLister != nil {
		limitRanges, err = l.limitRangeLister.LimitRanges(namespace).List(selector)
		if err != nil {
			return nil, fmt.Errorf("获取LimitRange列表失败: %v", err)
		}
	} else {
		// 使用 API 调用
		listOpts := metav1.ListOptions{LabelSelector: selector.String()}
		limitRangeList, err := l.client.CoreV1().LimitRanges(namespace).List(l.ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("获取LimitRange列表失败: %v", err)
		}
		limitRanges = make([]*corev1.LimitRange, len(limitRangeList.Items))
		for i := range limitRangeList.Items {
			limitRanges[i] = &limitRangeList.Items[i]
		}
	}

	// 搜索过滤
	if search != "" {
		filtered := make([]*corev1.LimitRange, 0)
		searchLower := strings.ToLower(search)
		for _, lr := range limitRanges {
			if strings.Contains(strings.ToLower(lr.Name), searchLower) {
				filtered = append(filtered, lr)
			}
		}
		limitRanges = filtered
	}

	// 转换为响应格式
	items := make([]types.LimitRangeInfo, len(limitRanges))
	for i, lr := range limitRanges {
		items[i] = types.LimitRangeInfo{
			Name:              lr.Name,
			Namespace:         lr.Namespace,
			Labels:            lr.Labels,
			Annotations:       lr.Annotations,
			Age:               l.formatDuration(time.Since(lr.CreationTimestamp.Time)),
			CreationTimestamp: lr.CreationTimestamp.UnixMilli(),
			LimitCount:        len(lr.Spec.Limits),
		}
	}

	return &types.ListLimitRangeResponse{
		Total: len(items),
		Items: items,
	}, nil
}

func (l *limitRangeOperator) GetDetail(namespace, name string) (*types.LimitRangeDetail, error) {
	limitRange, err := l.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	detail := &types.LimitRangeDetail{
		LimitRangeInfo: types.LimitRangeInfo{
			Name:              limitRange.Name,
			Namespace:         limitRange.Namespace,
			Labels:            limitRange.Labels,
			Annotations:       limitRange.Annotations,
			Age:               l.formatDuration(time.Since(limitRange.CreationTimestamp.Time)),
			CreationTimestamp: limitRange.CreationTimestamp.UnixMilli(),
			LimitCount:        len(limitRange.Spec.Limits),
		},
	}

	// 解析限制范围
	l.parseLimitRange(limitRange, detail)

	return detail, nil
}

func (l *limitRangeOperator) GetLimits(namespace, name string) (*types.LimitRangeLimits, error) {
	limitRange, err := l.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	limits := &types.LimitRangeLimits{}

	for _, item := range limitRange.Spec.Limits {
		switch item.Type {
		case corev1.LimitTypePod:
			l.parsePodLimits(&item, limits)
		case corev1.LimitTypeContainer:
			l.parseContainerLimits(&item, limits)
		case corev1.LimitTypePersistentVolumeClaim:
			l.parsePVCLimits(&item, limits)
		}
	}

	return limits, nil
}

func (l *limitRangeOperator) Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return l.client.CoreV1().LimitRanges(namespace).Watch(l.ctx, opts)
}

func (l *limitRangeOperator) UpdateLabels(namespace, name string, labels map[string]string) error {
	limitRange, err := l.Get(namespace, name)
	if err != nil {
		return err
	}

	if limitRange.Labels == nil {
		limitRange.Labels = make(map[string]string)
	}
	for k, v := range labels {
		limitRange.Labels[k] = v
	}

	_, err = l.Update(limitRange)
	return err
}

func (l *limitRangeOperator) UpdateAnnotations(namespace, name string, annotations map[string]string) error {
	limitRange, err := l.Get(namespace, name)
	if err != nil {
		return err
	}

	if limitRange.Annotations == nil {
		limitRange.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		limitRange.Annotations[k] = v
	}

	_, err = l.Update(limitRange)
	return err
}

func (l *limitRangeOperator) GetYaml(namespace, name string) (string, error) {
	limitRange, err := l.Get(namespace, name)
	if err != nil {
		return "", err
	}

	limitRange.TypeMeta = metav1.TypeMeta{
		APIVersion: "v1",
		Kind:       "LimitRange",
	}
	limitRange.ManagedFields = nil

	yamlBytes, err := yaml.Marshal(limitRange)
	if err != nil {
		return "", fmt.Errorf("转换为YAML失败: %v", err)
	}

	return string(yamlBytes), nil
}

func (l *limitRangeOperator) UpdateLimitRangeSpec(namespace, name string, limits []corev1.LimitRangeItem) error {
	limitRange, err := l.Get(namespace, name)
	if err != nil {
		return err
	}

	limitRange.Spec.Limits = limits

	_, err = l.Update(limitRange)
	return err
}

func (l *limitRangeOperator) ValidateResourceRequest(namespace, name string, resourceType corev1.LimitType,
	requests, limits corev1.ResourceList) (bool, []string, error) {

	limitRange, err := l.Get(namespace, name)
	if err != nil {
		return false, nil, err
	}

	violations := make([]string, 0)

	for _, item := range limitRange.Spec.Limits {
		if item.Type != resourceType {
			continue
		}

		// 检查最大限制
		for resourceName, maxValue := range item.Max {
			if limitValue, exists := limits[resourceName]; exists {
				if limitValue.Cmp(maxValue) > 0 {
					violations = append(violations, fmt.Sprintf("%s limit %s 超过最大值 %s",
						resourceName, limitValue.String(), maxValue.String()))
				}
			}
			if requestValue, exists := requests[resourceName]; exists {
				if requestValue.Cmp(maxValue) > 0 {
					violations = append(violations, fmt.Sprintf("%s request %s 超过最大值 %s",
						resourceName, requestValue.String(), maxValue.String()))
				}
			}
		}

		// 检查最小限制
		for resourceName, minValue := range item.Min {
			if limitValue, exists := limits[resourceName]; exists {
				if limitValue.Cmp(minValue) < 0 {
					violations = append(violations, fmt.Sprintf("%s limit %s 低于最小值 %s",
						resourceName, limitValue.String(), minValue.String()))
				}
			}
			if requestValue, exists := requests[resourceName]; exists {
				if requestValue.Cmp(minValue) < 0 {
					violations = append(violations, fmt.Sprintf("%s request %s 低于最小值 %s",
						resourceName, requestValue.String(), minValue.String()))
				}
			}
		}
	}

	return len(violations) == 0, violations, nil
}

// parseLimitRange 解析 LimitRange 的详细信息
func (l *limitRangeOperator) parseLimitRange(limitRange *corev1.LimitRange, detail *types.LimitRangeDetail) {
	for _, item := range limitRange.Spec.Limits {
		switch item.Type {
		case corev1.LimitTypePod:
			l.parsePodLimitsToDetail(&item, detail)
		case corev1.LimitTypeContainer:
			l.parseContainerLimitsToDetail(&item, detail)
		case corev1.LimitTypePersistentVolumeClaim:
			l.parsePVCLimitsToDetail(&item, detail)
		}
	}
}

// parsePodLimitsToDetail 解析 Pod 级别限制到 Detail
func (l *limitRangeOperator) parsePodLimitsToDetail(item *corev1.LimitRangeItem, detail *types.LimitRangeDetail) {
	if max := item.Max[corev1.ResourceCPU]; !max.IsZero() {
		detail.PodMaxCPU = max.String()
	}
	if max := item.Max[corev1.ResourceMemory]; !max.IsZero() {
		detail.PodMaxMemory = max.String()
	}
	if max := item.Max[corev1.ResourceEphemeralStorage]; !max.IsZero() {
		detail.PodMaxEphemeralStorage = max.String()
	}

	if min := item.Min[corev1.ResourceCPU]; !min.IsZero() {
		detail.PodMinCPU = min.String()
	}
	if min := item.Min[corev1.ResourceMemory]; !min.IsZero() {
		detail.PodMinMemory = min.String()
	}
	if min := item.Min[corev1.ResourceEphemeralStorage]; !min.IsZero() {
		detail.PodMinEphemeralStorage = min.String()
	}
}

// parseContainerLimitsToDetail 解析 Container 级别限制到 Detail
func (l *limitRangeOperator) parseContainerLimitsToDetail(item *corev1.LimitRangeItem, detail *types.LimitRangeDetail) {
	// Max limits
	if max := item.Max[corev1.ResourceCPU]; !max.IsZero() {
		detail.ContainerMaxCPU = max.String()
	}
	if max := item.Max[corev1.ResourceMemory]; !max.IsZero() {
		detail.ContainerMaxMemory = max.String()
	}
	if max := item.Max[corev1.ResourceEphemeralStorage]; !max.IsZero() {
		detail.ContainerMaxEphemeralStorage = max.String()
	}

	// Min limits
	if min := item.Min[corev1.ResourceCPU]; !min.IsZero() {
		detail.ContainerMinCPU = min.String()
	}
	if min := item.Min[corev1.ResourceMemory]; !min.IsZero() {
		detail.ContainerMinMemory = min.String()
	}
	if min := item.Min[corev1.ResourceEphemeralStorage]; !min.IsZero() {
		detail.ContainerMinEphemeralStorage = min.String()
	}

	// Default limits
	if def := item.Default[corev1.ResourceCPU]; !def.IsZero() {
		detail.ContainerDefaultCPU = def.String()
	}
	if def := item.Default[corev1.ResourceMemory]; !def.IsZero() {
		detail.ContainerDefaultMemory = def.String()
	}
	if def := item.Default[corev1.ResourceEphemeralStorage]; !def.IsZero() {
		detail.ContainerDefaultEphemeralStorage = def.String()
	}

	// Default requests
	if defReq := item.DefaultRequest[corev1.ResourceCPU]; !defReq.IsZero() {
		detail.ContainerDefaultRequestCPU = defReq.String()
	}
	if defReq := item.DefaultRequest[corev1.ResourceMemory]; !defReq.IsZero() {
		detail.ContainerDefaultRequestMemory = defReq.String()
	}
	if defReq := item.DefaultRequest[corev1.ResourceEphemeralStorage]; !defReq.IsZero() {
		detail.ContainerDefaultRequestEphemeralStorage = defReq.String()
	}
}

// parsePVCLimitsToDetail 解析 PVC 级别限制到 Detail
func (l *limitRangeOperator) parsePVCLimitsToDetail(item *corev1.LimitRangeItem, detail *types.LimitRangeDetail) {
	if max := item.Max[corev1.ResourceStorage]; !max.IsZero() {
		detail.PVCMaxStorage = max.String()
	}
	if min := item.Min[corev1.ResourceStorage]; !min.IsZero() {
		detail.PVCMinStorage = min.String()
	}
}

// parsePodLimits 解析 Pod 级别限制到 LimitRangeLimits
func (l *limitRangeOperator) parsePodLimits(item *corev1.LimitRangeItem, limits *types.LimitRangeLimits) {
	if max := item.Max[corev1.ResourceCPU]; !max.IsZero() {
		limits.PodMaxCPU = max.String()
	}
	if max := item.Max[corev1.ResourceMemory]; !max.IsZero() {
		limits.PodMaxMemory = max.String()
	}
	if max := item.Max[corev1.ResourceEphemeralStorage]; !max.IsZero() {
		limits.PodMaxEphemeralStorage = max.String()
	}

	if min := item.Min[corev1.ResourceCPU]; !min.IsZero() {
		limits.PodMinCPU = min.String()
	}
	if min := item.Min[corev1.ResourceMemory]; !min.IsZero() {
		limits.PodMinMemory = min.String()
	}
	if min := item.Min[corev1.ResourceEphemeralStorage]; !min.IsZero() {
		limits.PodMinEphemeralStorage = min.String()
	}
}

// parseContainerLimits 解析 Container 级别限制到 LimitRangeLimits
func (l *limitRangeOperator) parseContainerLimits(item *corev1.LimitRangeItem, limits *types.LimitRangeLimits) {
	// Max limits
	if max := item.Max[corev1.ResourceCPU]; !max.IsZero() {
		limits.ContainerMaxCPU = max.String()
	}
	if max := item.Max[corev1.ResourceMemory]; !max.IsZero() {
		limits.ContainerMaxMemory = max.String()
	}
	if max := item.Max[corev1.ResourceEphemeralStorage]; !max.IsZero() {
		limits.ContainerMaxEphemeralStorage = max.String()
	}

	// Min limits
	if min := item.Min[corev1.ResourceCPU]; !min.IsZero() {
		limits.ContainerMinCPU = min.String()
	}
	if min := item.Min[corev1.ResourceMemory]; !min.IsZero() {
		limits.ContainerMinMemory = min.String()
	}
	if min := item.Min[corev1.ResourceEphemeralStorage]; !min.IsZero() {
		limits.ContainerMinEphemeralStorage = min.String()
	}

	// Default limits
	if def := item.Default[corev1.ResourceCPU]; !def.IsZero() {
		limits.ContainerDefaultCPU = def.String()
	}
	if def := item.Default[corev1.ResourceMemory]; !def.IsZero() {
		limits.ContainerDefaultMemory = def.String()
	}
	if def := item.Default[corev1.ResourceEphemeralStorage]; !def.IsZero() {
		limits.ContainerDefaultEphemeralStorage = def.String()
	}

	// Default requests
	if defReq := item.DefaultRequest[corev1.ResourceCPU]; !defReq.IsZero() {
		limits.ContainerDefaultRequestCPU = defReq.String()
	}
	if defReq := item.DefaultRequest[corev1.ResourceMemory]; !defReq.IsZero() {
		limits.ContainerDefaultRequestMemory = defReq.String()
	}
	if defReq := item.DefaultRequest[corev1.ResourceEphemeralStorage]; !defReq.IsZero() {
		limits.ContainerDefaultRequestEphemeralStorage = defReq.String()
	}
}

// parsePVCLimits 解析 PVC 级别限制到 LimitRangeLimits
func (l *limitRangeOperator) parsePVCLimits(item *corev1.LimitRangeItem, limits *types.LimitRangeLimits) {
	if max := item.Max[corev1.ResourceStorage]; !max.IsZero() {
		limits.PVCMaxStorage = max.String()
	}
	if min := item.Min[corev1.ResourceStorage]; !min.IsZero() {
		limits.PVCMinStorage = min.String()
	}
}

// formatMemoryToGiB 格式化内存为 GiB

// formatMemoryToMiB 格式化内存为 MiB

// formatStorageToGiB 格式化存储为 GiB

// formatStorageToMiB 格式化存储为 MiB

// formatDuration 格式化时间间隔
func (l *limitRangeOperator) formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	if hours > 0 {
		return fmt.Sprintf("%dd%dh", days, hours)
	}
	return fmt.Sprintf("%dd", days)
}
