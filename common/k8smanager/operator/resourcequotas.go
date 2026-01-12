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

type resourceQuotaOperator struct {
	BaseOperator
	client          kubernetes.Interface
	informerFactory informers.SharedInformerFactory
	quotaLister     corev1lister.ResourceQuotaLister
}

func NewResourceQuotaOperator(ctx context.Context, client kubernetes.Interface) types.ResourceQuotaOperator {
	return &resourceQuotaOperator{
		BaseOperator: NewBaseOperator(ctx, false),
		client:       client,
	}
}

func NewResourceQuotaOperatorWithInformer(
	ctx context.Context,
	client kubernetes.Interface,
	informerFactory informers.SharedInformerFactory,
) types.ResourceQuotaOperator {
	var quotaLister corev1lister.ResourceQuotaLister

	if informerFactory != nil {
		quotaLister = informerFactory.Core().V1().ResourceQuotas().Lister()
	}

	return &resourceQuotaOperator{
		BaseOperator:    NewBaseOperator(ctx, informerFactory != nil),
		client:          client,
		informerFactory: informerFactory,
		quotaLister:     quotaLister,
	}
}

func (r *resourceQuotaOperator) Create(quota *corev1.ResourceQuota) (*corev1.ResourceQuota, error) {
	if quota == nil || quota.Name == "" || quota.Namespace == "" {
		return nil, fmt.Errorf("ResourceQuota对象、名称和命名空间不能为空")
	}
	injectCommonAnnotations(quota)
	created, err := r.client.CoreV1().ResourceQuotas(quota.Namespace).Create(r.ctx, quota, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("创建ResourceQuota失败: %v", err)
	}

	return created, nil
}

func (r *resourceQuotaOperator) Get(namespace, name string) (*corev1.ResourceQuota, error) {
	if namespace == "" || name == "" {
		return nil, fmt.Errorf("命名空间和名称不能为空")
	}

	// 优先使用 informer
	if r.useInformer && r.quotaLister != nil {
		quota, err := r.quotaLister.ResourceQuotas(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("ResourceQuota %s/%s 不存在", namespace, name)
			}
			// informer 出错时回退到 API 调用
			quota, apiErr := r.client.CoreV1().ResourceQuotas(namespace).Get(r.ctx, name, metav1.GetOptions{})
			if apiErr != nil {
				return nil, fmt.Errorf("获取ResourceQuota失败: %v", apiErr)
			}
			quota.TypeMeta = metav1.TypeMeta{
				Kind:       "ResourceQuota",
				APIVersion: "v1",
			}
			return quota, nil
		}
		quota.TypeMeta = metav1.TypeMeta{
			Kind:       "ResourceQuota",
			APIVersion: "v1",
		}
		return quota, nil
	}

	// 使用 API 调用
	quota, err := r.client.CoreV1().ResourceQuotas(namespace).Get(r.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("ResourceQuota %s/%s 不存在", namespace, name)
		}
		return nil, fmt.Errorf("获取ResourceQuota失败: %v", err)
	}
	quota.TypeMeta = metav1.TypeMeta{
		Kind:       "ResourceQuota",
		APIVersion: "v1",
	}
	return quota, nil
}

func (r *resourceQuotaOperator) Update(quota *corev1.ResourceQuota) (*corev1.ResourceQuota, error) {
	if quota == nil || quota.Name == "" || quota.Namespace == "" {
		return nil, fmt.Errorf("ResourceQuota对象、名称和命名空间不能为空")
	}

	updated, err := r.client.CoreV1().ResourceQuotas(quota.Namespace).Update(r.ctx, quota, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("更新ResourceQuota失败: %v", err)
	}

	return updated, nil
}

func (r *resourceQuotaOperator) Delete(namespace, name string) error {
	if namespace == "" || name == "" {
		return fmt.Errorf("命名空间和名称不能为空")
	}

	err := r.client.CoreV1().ResourceQuotas(namespace).Delete(r.ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("删除ResourceQuota失败: %v", err)
	}

	return nil
}

func (r *resourceQuotaOperator) List(namespace string, search string, labelSelector string) (*types.ListResourceQuotaResponse, error) {
	var selector labels.Selector = labels.Everything()
	if labelSelector != "" {
		parsedSelector, err := labels.Parse(labelSelector)
		if err != nil {
			return nil, fmt.Errorf("解析标签选择器失败: %v", err)
		}
		selector = parsedSelector
	}

	var quotas []*corev1.ResourceQuota
	var err error

	// 优先使用 informer
	if r.useInformer && r.quotaLister != nil {
		quotas, err = r.quotaLister.ResourceQuotas(namespace).List(selector)
		if err != nil {
			return nil, fmt.Errorf("获取ResourceQuota列表失败: %v", err)
		}
	} else {
		// 使用 API 调用
		listOpts := metav1.ListOptions{LabelSelector: selector.String()}
		quotaList, err := r.client.CoreV1().ResourceQuotas(namespace).List(r.ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("获取ResourceQuota列表失败: %v", err)
		}
		quotas = make([]*corev1.ResourceQuota, len(quotaList.Items))
		for i := range quotaList.Items {
			quotas[i] = &quotaList.Items[i]
		}
	}

	// 搜索过滤
	if search != "" {
		filtered := make([]*corev1.ResourceQuota, 0)
		searchLower := strings.ToLower(search)
		for _, quota := range quotas {
			if strings.Contains(strings.ToLower(quota.Name), searchLower) {
				filtered = append(filtered, quota)
			}
		}
		quotas = filtered
	}

	// 转换为响应格式
	items := make([]types.ResourceQuotaInfo, len(quotas))
	for i, quota := range quotas {
		usedPercentage := r.calculateUsagePercentage(quota)
		items[i] = types.ResourceQuotaInfo{
			Name:              quota.Name,
			Namespace:         quota.Namespace,
			Labels:            quota.Labels,
			Annotations:       quota.Annotations,
			Age:               formatDuration(time.Since(quota.CreationTimestamp.Time)),
			CreationTimestamp: quota.CreationTimestamp.UnixMilli(),
			UsedPercentage:    usedPercentage,
			ResourceCount:     len(quota.Spec.Hard),
		}
	}

	return &types.ListResourceQuotaResponse{
		Total: len(items),
		Items: items,
	}, nil
}

func (r *resourceQuotaOperator) GetDetail(namespace, name string) (*types.ResourceQuotaDetail, error) {
	quota, err := r.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	detail := &types.ResourceQuotaDetail{
		ResourceQuotaInfo: types.ResourceQuotaInfo{
			Name:              quota.Name,
			Namespace:         quota.Namespace,
			Labels:            quota.Labels,
			Annotations:       quota.Annotations,
			Age:               formatDuration(time.Since(quota.CreationTimestamp.Time)),
			CreationTimestamp: quota.CreationTimestamp.UnixMilli(),
			UsedPercentage:    r.calculateUsagePercentage(quota),
			ResourceCount:     len(quota.Spec.Hard),
		},
	}

	// 解析资源配额
	r.parseResourceQuota(quota, detail)

	return detail, nil
}

func (r *resourceQuotaOperator) GetAllocated(namespace, name string) (*types.ResourceQuotaAllocated, error) {
	quota, err := r.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	allocated := &types.ResourceQuotaAllocated{}

	// 资源配额
	if hard := quota.Spec.Hard[corev1.ResourceRequestsCPU]; !hard.IsZero() {
		allocated.CPUAllocated = hard.String()
	}
	if hard := quota.Spec.Hard[corev1.ResourceRequestsMemory]; !hard.IsZero() {
		allocated.MemoryAllocated = hard.String()
	}
	if hard := quota.Spec.Hard[corev1.ResourceRequestsStorage]; !hard.IsZero() {
		allocated.StorageAllocated = hard.String()
	}
	if hard := quota.Spec.Hard[corev1.ResourceName("requests.nvidia.com/gpu")]; !hard.IsZero() {
		allocated.GPUAllocated = hard.String()
	}
	if hard := quota.Spec.Hard[corev1.ResourceEphemeralStorage]; !hard.IsZero() {
		allocated.EphemeralStorageAllocated = hard.String()
	}

	// 对象数量配额
	if hard := quota.Spec.Hard[corev1.ResourcePods]; !hard.IsZero() {
		allocated.PodsAllocated = hard.Value()
	}
	if hard := quota.Spec.Hard[corev1.ResourceConfigMaps]; !hard.IsZero() {
		allocated.ConfigMapsAllocated = hard.Value()
	}
	if hard := quota.Spec.Hard[corev1.ResourceSecrets]; !hard.IsZero() {
		allocated.SecretsAllocated = hard.Value()
	}
	if hard := quota.Spec.Hard[corev1.ResourcePersistentVolumeClaims]; !hard.IsZero() {
		allocated.PVCsAllocated = hard.Value()
	}
	if hard := quota.Spec.Hard[corev1.ResourceServices]; !hard.IsZero() {
		allocated.ServicesAllocated = hard.Value()
	}
	if hard := quota.Spec.Hard[corev1.ResourceServicesLoadBalancers]; !hard.IsZero() {
		allocated.LoadBalancersAllocated = hard.Value()
	}
	if hard := quota.Spec.Hard[corev1.ResourceServicesNodePorts]; !hard.IsZero() {
		allocated.NodePortsAllocated = hard.Value()
	}

	// 工作负载配额
	if hard := quota.Spec.Hard[corev1.ResourceName("count/deployments.apps")]; !hard.IsZero() {
		allocated.DeploymentsAllocated = hard.Value()
	}
	if hard := quota.Spec.Hard[corev1.ResourceName("count/jobs.batch")]; !hard.IsZero() {
		allocated.JobsAllocated = hard.Value()
	}
	if hard := quota.Spec.Hard[corev1.ResourceName("count/cronjobs.batch")]; !hard.IsZero() {
		allocated.CronJobsAllocated = hard.Value()
	}
	if hard := quota.Spec.Hard[corev1.ResourceName("count/daemonsets.apps")]; !hard.IsZero() {
		allocated.DaemonSetsAllocated = hard.Value()
	}
	if hard := quota.Spec.Hard[corev1.ResourceName("count/statefulsets.apps")]; !hard.IsZero() {
		allocated.StatefulSetsAllocated = hard.Value()
	}
	if hard := quota.Spec.Hard[corev1.ResourceName("count/ingresses.networking.k8s.io")]; !hard.IsZero() {
		allocated.IngressesAllocated = hard.Value()
	}

	return allocated, nil
}

func (r *resourceQuotaOperator) Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return r.client.CoreV1().ResourceQuotas(namespace).Watch(r.ctx, opts)
}

func (r *resourceQuotaOperator) UpdateLabels(namespace, name string, labels map[string]string) error {
	quota, err := r.Get(namespace, name)
	if err != nil {
		return err
	}

	if quota.Labels == nil {
		quota.Labels = make(map[string]string)
	}
	for k, v := range labels {
		quota.Labels[k] = v
	}

	_, err = r.Update(quota)
	return err
}

func (r *resourceQuotaOperator) UpdateAnnotations(namespace, name string, annotations map[string]string) error {
	quota, err := r.Get(namespace, name)
	if err != nil {
		return err
	}

	if quota.Annotations == nil {
		quota.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		quota.Annotations[k] = v
	}

	_, err = r.Update(quota)
	return err
}

func (r *resourceQuotaOperator) GetYaml(namespace, name string) (string, error) {
	quota, err := r.Get(namespace, name)
	if err != nil {
		return "", err
	}

	//
	quota.TypeMeta = metav1.TypeMeta{
		APIVersion: "v1",
		Kind:       "ResourceQuota",
	}
	quota.ManagedFields = nil

	yamlBytes, err := yaml.Marshal(quota)
	if err != nil {
		return "", fmt.Errorf("转换为YAML失败: %v", err)
	}

	return string(yamlBytes), nil
}

func (r *resourceQuotaOperator) UpdateQuotaSpec(namespace, name string, hard corev1.ResourceList) error {
	quota, err := r.Get(namespace, name)
	if err != nil {
		return err
	}

	quota.Spec.Hard = hard

	_, err = r.Update(quota)
	return err
}

func (r *resourceQuotaOperator) CheckQuotaExceeded(namespace, name string) (bool, []string, error) {
	quota, err := r.Get(namespace, name)
	if err != nil {
		return false, nil, err
	}

	exceededResources := make([]string, 0)

	for resourceName, hardLimit := range quota.Spec.Hard {
		if used, exists := quota.Status.Used[resourceName]; exists {
			if used.Cmp(hardLimit) > 0 {
				exceededResources = append(exceededResources, fmt.Sprintf("%s: 已使用 %s / 限额 %s",
					resourceName, used.String(), hardLimit.String()))
			}
		}
	}

	return len(exceededResources) > 0, exceededResources, nil
}

// parseResourceQuota 解析 ResourceQuota 的详细信息
func (r *resourceQuotaOperator) parseResourceQuota(quota *corev1.ResourceQuota, detail *types.ResourceQuotaDetail) {
	// 资源配额 - CPU
	if hard := quota.Spec.Hard[corev1.ResourceRequestsCPU]; !hard.IsZero() {
		detail.CPUHard = hard.String()
		if used := quota.Status.Used[corev1.ResourceRequestsCPU]; !used.IsZero() {
			detail.CPUUsed = used.String()
		}
	}

	// 资源配额 - Memory
	if hard := quota.Spec.Hard[corev1.ResourceRequestsMemory]; !hard.IsZero() {
		detail.MemoryHard = hard.String()
		if used := quota.Status.Used[corev1.ResourceRequestsMemory]; !used.IsZero() {
			detail.MemoryUsed = used.String()
		}
	}

	// 资源配额 - Storage
	if hard := quota.Spec.Hard[corev1.ResourceRequestsStorage]; !hard.IsZero() {
		detail.StorageHard = hard.String()
		if used := quota.Status.Used[corev1.ResourceRequestsStorage]; !used.IsZero() {
			detail.StorageUsed = used.String()
		}
	}

	// 资源配额 - GPU
	gpuResource := corev1.ResourceName("requests.nvidia.com/gpu")
	if hard := quota.Spec.Hard[gpuResource]; !hard.IsZero() {
		detail.GPUHard = hard.String()
		if used := quota.Status.Used[gpuResource]; !used.IsZero() {
			detail.GPUUsed = used.String()
		}
	}

	// 资源配额 - Ephemeral Storage
	if hard := quota.Spec.Hard[corev1.ResourceEphemeralStorage]; !hard.IsZero() {
		detail.EphemeralStorageHard = hard.String()
		if used := quota.Status.Used[corev1.ResourceEphemeralStorage]; !used.IsZero() {
			detail.EphemeralStorageUsed = used.String()
		}
	}

	// 对象数量配额
	r.parseCountQuota(quota, detail, corev1.ResourcePods, &detail.PodsHard, &detail.PodsUsed)
	r.parseCountQuota(quota, detail, corev1.ResourceConfigMaps, &detail.ConfigMapsHard, &detail.ConfigMapsUsed)
	r.parseCountQuota(quota, detail, corev1.ResourceSecrets, &detail.SecretsHard, &detail.SecretsUsed)
	r.parseCountQuota(quota, detail, corev1.ResourcePersistentVolumeClaims, &detail.PVCsHard, &detail.PVCsUsed)
	r.parseCountQuota(quota, detail, corev1.ResourceServices, &detail.ServicesHard, &detail.ServicesUsed)
	r.parseCountQuota(quota, detail, corev1.ResourceServicesLoadBalancers, &detail.LoadBalancersHard, &detail.LoadBalancersUsed)
	r.parseCountQuota(quota, detail, corev1.ResourceServicesNodePorts, &detail.NodePortsHard, &detail.NodePortsUsed)

	// 工作负载配额
	r.parseCountQuota(quota, detail, corev1.ResourceName("count/deployments.apps"), &detail.DeploymentsHard, &detail.DeploymentsUsed)
	r.parseCountQuota(quota, detail, corev1.ResourceName("count/jobs.batch"), &detail.JobsHard, &detail.JobsUsed)
	r.parseCountQuota(quota, detail, corev1.ResourceName("count/cronjobs.batch"), &detail.CronJobsHard, &detail.CronJobsUsed)
	r.parseCountQuota(quota, detail, corev1.ResourceName("count/daemonsets.apps"), &detail.DaemonSetsHard, &detail.DaemonSetsUsed)
	r.parseCountQuota(quota, detail, corev1.ResourceName("count/statefulsets.apps"), &detail.StatefulSetsHard, &detail.StatefulSetsUsed)
	r.parseCountQuota(quota, detail, corev1.ResourceName("count/ingresses.networking.k8s.io"), &detail.IngressesHard, &detail.IngressesUsed)
}

// parseCountQuota 解析数量类配额
func (r *resourceQuotaOperator) parseCountQuota(quota *corev1.ResourceQuota, detail *types.ResourceQuotaDetail,
	resourceName corev1.ResourceName, hardPtr, usedPtr *int64) {
	if hard := quota.Spec.Hard[resourceName]; !hard.IsZero() {
		*hardPtr = hard.Value()
		if used := quota.Status.Used[resourceName]; !used.IsZero() {
			*usedPtr = used.Value()
		}
	}
}

// calculateUsagePercentage 计算整体使用率
func (r *resourceQuotaOperator) calculateUsagePercentage(quota *corev1.ResourceQuota) float64 {
	if len(quota.Spec.Hard) == 0 {
		return 0
	}

	var totalUsage float64
	count := 0

	for resourceName, hardLimit := range quota.Spec.Hard {
		if used, exists := quota.Status.Used[resourceName]; exists && !hardLimit.IsZero() {
			usagePercent := float64(used.MilliValue()) / float64(hardLimit.MilliValue()) * 100
			totalUsage += usagePercent
			count++
		}
	}

	if count == 0 {
		return 0
	}

	return totalUsage / float64(count)
}
