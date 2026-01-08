package operator

import (
	"context"
	"fmt"
	"sort"
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

type configMapOperator struct {
	BaseOperator
	client          kubernetes.Interface
	informerFactory informers.SharedInformerFactory
	cmLister        corev1lister.ConfigMapLister
}

func NewConfigMapOperator(ctx context.Context, client kubernetes.Interface) types.ConfigMapOperator {
	return &configMapOperator{
		BaseOperator: NewBaseOperator(ctx, false),
		client:       client,
	}
}

func NewConfigMapOperatorWithInformer(
	ctx context.Context,
	client kubernetes.Interface,
	informerFactory informers.SharedInformerFactory,
) types.ConfigMapOperator {
	var cmLister corev1lister.ConfigMapLister

	if informerFactory != nil {
		cmLister = informerFactory.Core().V1().ConfigMaps().Lister()
	}

	return &configMapOperator{
		BaseOperator:    NewBaseOperator(ctx, informerFactory != nil),
		client:          client,
		informerFactory: informerFactory,
		cmLister:        cmLister,
	}
}

func (c *configMapOperator) Create(cm *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	if cm == nil || cm.Name == "" || cm.Namespace == "" {
		return nil, fmt.Errorf("ConfigMap对象、名称和命名空间不能为空")
	}

	created, err := c.client.CoreV1().ConfigMaps(cm.Namespace).Create(c.ctx, cm, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("创建ConfigMap失败: %v", err)
	}

	return created, nil
}

func (c *configMapOperator) Update(cm *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	if cm == nil || cm.Name == "" || cm.Namespace == "" {
		return nil, fmt.Errorf("ConfigMap对象、名称和命名空间不能为空")
	}

	updated, err := c.client.CoreV1().ConfigMaps(cm.Namespace).Update(c.ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("更新ConfigMap失败")
	}

	return updated, nil
}

func (c *configMapOperator) Delete(namespace, name string) error {
	if namespace == "" || name == "" {
		return fmt.Errorf("命名空间和名称不能为空")
	}

	err := c.client.CoreV1().ConfigMaps(namespace).Delete(c.ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("删除ConfigMap失败")
	}

	return nil
}

// ConfigMap Operator 修改后的 List 方法
func (c *configMapOperator) List(namespace string, search string, labelSelector string) (*types.ListConfigMapResponse, error) {
	var selector labels.Selector = labels.Everything()
	if labelSelector != "" {
		parsedSelector, err := labels.Parse(labelSelector)
		if err != nil {
			return nil, fmt.Errorf("解析标签选择器失败")
		}
		selector = parsedSelector
	}

	var configMaps []*corev1.ConfigMap
	var err error

	if c.useInformer && c.cmLister != nil {
		configMaps, err = c.cmLister.ConfigMaps(namespace).List(selector)
		if err != nil {
			return nil, fmt.Errorf("获取ConfigMap列表失败")
		}
	} else {
		listOpts := metav1.ListOptions{LabelSelector: selector.String()}
		cmList, err := c.client.CoreV1().ConfigMaps(namespace).List(c.ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("获取ConfigMap列表失败")
		}
		configMaps = make([]*corev1.ConfigMap, len(cmList.Items))
		for i := range cmList.Items {
			configMaps[i] = &cmList.Items[i]
		}
	}

	// 搜索过滤
	if search != "" {
		filtered := make([]*corev1.ConfigMap, 0)
		searchLower := strings.ToLower(search)
		for _, cm := range configMaps {
			if strings.Contains(strings.ToLower(cm.Name), searchLower) {
				filtered = append(filtered, cm)
			}
		}
		configMaps = filtered
	}

	// 转换为响应格式
	items := make([]types.ConfigMapInfo, len(configMaps))
	for i, cm := range configMaps {
		items[i] = types.ConfigMapInfo{
			Name:              cm.Name,
			Namespace:         cm.Namespace,
			Data:              cm.Data,
			BinaryData:        cm.BinaryData,
			Labels:            cm.Labels,
			Annotations:       cm.Annotations,
			Age:               time.Since(cm.CreationTimestamp.Time).String(),
			CreationTimestamp: cm.CreationTimestamp.UnixMilli(),
			DataCount:         len(cm.Data) + len(cm.BinaryData),
		}
	}

	return &types.ListConfigMapResponse{
		Total: len(items),
		Items: items,
	}, nil
}

// ConfigMap Operator 新增的 GetData 方法
func (c *configMapOperator) GetData(namespace, name string) (*types.GetConfigMapDataResponse, error) {
	cm, err := c.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	data := make([]types.ConfigMapDataItem, 0, len(cm.Data))
	for key, value := range cm.Data {
		data = append(data, types.ConfigMapDataItem{
			Key:   key,
			Value: value,
		})
	}

	// 按 key 排序，保证顺序一致
	sort.Slice(data, func(i, j int) bool {
		return data[i].Key < data[j].Key
	})

	return &types.GetConfigMapDataResponse{
		Name:      name,
		Namespace: namespace,
		Data:      data,
	}, nil
}

func (c *configMapOperator) Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return c.client.CoreV1().ConfigMaps(namespace).Watch(c.ctx, opts)
}

func (c *configMapOperator) UpdateLabels(namespace, name string, labels map[string]string) error {
	cm, err := c.Get(namespace, name)
	if err != nil {
		return err
	}

	if cm.Labels == nil {
		cm.Labels = make(map[string]string)
	}
	for k, v := range labels {
		cm.Labels[k] = v
	}

	_, err = c.Update(cm)
	return err
}

func (c *configMapOperator) UpdateAnnotations(namespace, name string, annotations map[string]string) error {
	cm, err := c.Get(namespace, name)
	if err != nil {
		return err
	}

	if cm.Annotations == nil {
		cm.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		cm.Annotations[k] = v
	}

	_, err = c.Update(cm)
	return err
}

func (c *configMapOperator) UpdateKey(namespace, name, key, value string) error {
	cm, err := c.Get(namespace, name)
	if err != nil {
		return err
	}

	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	cm.Data[key] = value

	_, err = c.Update(cm)
	return err
}

func (c *configMapOperator) DeleteKey(namespace, name, key string) error {
	cm, err := c.Get(namespace, name)
	if err != nil {
		return err
	}

	if cm.Data != nil {
		delete(cm.Data, key)
	}
	if cm.BinaryData != nil {
		delete(cm.BinaryData, key)
	}

	_, err = c.Update(cm)
	return err
}

func (c *configMapOperator) UpdateData(namespace, name string, data map[string]string) error {
	cm, err := c.Get(namespace, name)
	if err != nil {
		return err
	}

	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}

	for k, v := range data {
		cm.Data[k] = v
	}

	_, err = c.Update(cm)
	return err
}

// GetUsage 获取 ConfigMap 的使用情况
func (c *configMapOperator) GetUsage(namespace, name string) (*types.ConfigMapUsageResponse, error) {
	_, err := c.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	response := &types.ConfigMapUsageResponse{
		ConfigMapName:      name,
		ConfigMapNamespace: namespace,
		UsedBy:             make([]types.ConfigMapUsageReference, 0),
	}

	// 用于存储已经扫描过的工作负载创建的 Pod
	managedPods := make(map[string]bool)

	// 1. 扫描 Deployment
	deployments, err := c.client.AppsV1().Deployments(namespace).List(c.ctx, metav1.ListOptions{})
	if err == nil {
		for _, deploy := range deployments.Items {
			if ref := c.checkPodTemplateForConfigMap(name, &deploy.Spec.Template); ref != nil {
				ref.ResourceType = "Deployment"
				ref.ResourceName = deploy.Name
				ref.Namespace = deploy.Namespace
				response.UsedBy = append(response.UsedBy, *ref)

				// 标记由此 Deployment 管理的 Pod
				c.markManagedPods(namespace, deploy.Spec.Selector, managedPods)
			}
		}
	}

	// 2. 扫描 StatefulSet
	statefulSets, err := c.client.AppsV1().StatefulSets(namespace).List(c.ctx, metav1.ListOptions{})
	if err == nil {
		for _, sts := range statefulSets.Items {
			if ref := c.checkPodTemplateForConfigMap(name, &sts.Spec.Template); ref != nil {
				ref.ResourceType = "StatefulSet"
				ref.ResourceName = sts.Name
				ref.Namespace = sts.Namespace
				response.UsedBy = append(response.UsedBy, *ref)

				// 标记由此 StatefulSet 管理的 Pod
				c.markManagedPods(namespace, sts.Spec.Selector, managedPods)
			}
		}
	}

	// 3. 扫描 DaemonSet
	daemonSets, err := c.client.AppsV1().DaemonSets(namespace).List(c.ctx, metav1.ListOptions{})
	if err == nil {
		for _, ds := range daemonSets.Items {
			if ref := c.checkPodTemplateForConfigMap(name, &ds.Spec.Template); ref != nil {
				ref.ResourceType = "DaemonSet"
				ref.ResourceName = ds.Name
				ref.Namespace = ds.Namespace
				response.UsedBy = append(response.UsedBy, *ref)

				// 标记由此 DaemonSet 管理的 Pod
				c.markManagedPods(namespace, ds.Spec.Selector, managedPods)
			}
		}
	}

	// 4. 扫描 Job
	jobs, err := c.client.BatchV1().Jobs(namespace).List(c.ctx, metav1.ListOptions{})
	if err == nil {
		for _, job := range jobs.Items {
			if ref := c.checkPodTemplateForConfigMap(name, &job.Spec.Template); ref != nil {
				ref.ResourceType = "Job"
				ref.ResourceName = job.Name
				ref.Namespace = job.Namespace
				response.UsedBy = append(response.UsedBy, *ref)

				// 标记由此 Job 管理的 Pod
				c.markManagedPods(namespace, job.Spec.Selector, managedPods)
			}
		}
	}

	// 5. 扫描 CronJob
	cronJobs, err := c.client.BatchV1().CronJobs(namespace).List(c.ctx, metav1.ListOptions{})
	if err == nil {
		for _, cronJob := range cronJobs.Items {
			if ref := c.checkPodTemplateForConfigMap(name, &cronJob.Spec.JobTemplate.Spec.Template); ref != nil {
				ref.ResourceType = "CronJob"
				ref.ResourceName = cronJob.Name
				ref.Namespace = cronJob.Namespace
				response.UsedBy = append(response.UsedBy, *ref)
			}
		}
	}

	// 6. 扫描独立的 Pod（不由上述工作负载管理的 Pod）
	pods, err := c.client.CoreV1().Pods(namespace).List(c.ctx, metav1.ListOptions{})
	if err == nil {
		for _, pod := range pods.Items {
			// 构建 Pod 的唯一标识
			podKey := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)

			// 跳过已被工作负载管理的 Pod
			if managedPods[podKey] {
				continue
			}

			// 检查 Pod 是否直接引用了 ConfigMap
			if ref := c.checkPodForConfigMap(name, &pod); ref != nil {
				ref.ResourceType = "Pod"
				ref.ResourceName = pod.Name
				ref.Namespace = pod.Namespace
				response.UsedBy = append(response.UsedBy, *ref)
			}
		}
	}

	response.TotalUsageCount = len(response.UsedBy)
	response.CanDelete = response.TotalUsageCount == 0
	if !response.CanDelete {
		response.DeleteWarning = fmt.Sprintf("ConfigMap被%d个资源引用，删除可能导致这些资源异常", response.TotalUsageCount)
	}

	return response, nil
}

// checkPodTemplateForConfigMap 检查 Pod 模板是否引用了指定的 ConfigMap
func (c *configMapOperator) checkPodTemplateForConfigMap(configMapName string, template *corev1.PodTemplateSpec) *types.ConfigMapUsageReference {
	usageTypes := make([]string, 0)
	usedKeys := make([]string, 0)
	containerNames := make([]string, 0)

	// 检查 Volume
	for _, vol := range template.Spec.Volumes {
		if vol.ConfigMap != nil && vol.ConfigMap.Name == configMapName {
			usageTypes = append(usageTypes, "volume")
			for _, item := range vol.ConfigMap.Items {
				usedKeys = append(usedKeys, item.Key)
			}
		}
	}

	// 检查所有容器（包括 InitContainers 和普通 Containers）
	allContainers := append(template.Spec.InitContainers, template.Spec.Containers...)
	for _, container := range allContainers {
		found := false

		// 检查环境变量
		for _, env := range container.Env {
			if env.ValueFrom != nil && env.ValueFrom.ConfigMapKeyRef != nil &&
				env.ValueFrom.ConfigMapKeyRef.Name == configMapName {
				if !found {
					usageTypes = append(usageTypes, "env")
					containerNames = append(containerNames, container.Name)
					found = true
				}
				usedKeys = append(usedKeys, env.ValueFrom.ConfigMapKeyRef.Key)
			}
		}

		// 检查 EnvFrom
		for _, envFrom := range container.EnvFrom {
			if envFrom.ConfigMapRef != nil && envFrom.ConfigMapRef.Name == configMapName {
				if !found {
					usageTypes = append(usageTypes, "envFrom")
					containerNames = append(containerNames, container.Name)
					found = true
				}
			}
		}
	}

	if len(usageTypes) > 0 {
		return &types.ConfigMapUsageReference{
			UsageType:      usageTypes,
			UsedKeys:       usedKeys,
			ContainerNames: containerNames,
		}
	}

	return nil
}

// checkPodForConfigMap 检查 Pod 是否引用了指定的 ConfigMap
func (c *configMapOperator) checkPodForConfigMap(configMapName string, pod *corev1.Pod) *types.ConfigMapUsageReference {
	usageTypes := make([]string, 0)
	usedKeys := make([]string, 0)
	containerNames := make([]string, 0)

	// 检查 Volume
	for _, vol := range pod.Spec.Volumes {
		if vol.ConfigMap != nil && vol.ConfigMap.Name == configMapName {
			usageTypes = append(usageTypes, "volume")
			for _, item := range vol.ConfigMap.Items {
				usedKeys = append(usedKeys, item.Key)
			}
		}
	}

	// 检查所有容器
	allContainers := append(pod.Spec.InitContainers, pod.Spec.Containers...)
	for _, container := range allContainers {
		found := false

		// 检查环境变量
		for _, env := range container.Env {
			if env.ValueFrom != nil && env.ValueFrom.ConfigMapKeyRef != nil &&
				env.ValueFrom.ConfigMapKeyRef.Name == configMapName {
				if !found {
					usageTypes = append(usageTypes, "env")
					containerNames = append(containerNames, container.Name)
					found = true
				}
				usedKeys = append(usedKeys, env.ValueFrom.ConfigMapKeyRef.Key)
			}
		}

		// 检查 EnvFrom
		for _, envFrom := range container.EnvFrom {
			if envFrom.ConfigMapRef != nil && envFrom.ConfigMapRef.Name == configMapName {
				if !found {
					usageTypes = append(usageTypes, "envFrom")
					containerNames = append(containerNames, container.Name)
					found = true
				}
			}
		}
	}

	if len(usageTypes) > 0 {
		return &types.ConfigMapUsageReference{
			UsageType:      usageTypes,
			UsedKeys:       usedKeys,
			ContainerNames: containerNames,
		}
	}

	return nil
}

// markManagedPods 标记由指定 selector 管理的 Pod
func (c *configMapOperator) markManagedPods(namespace string, selector *metav1.LabelSelector, managedPods map[string]bool) {
	if selector == nil {
		return
	}

	// 将 LabelSelector 转换为字符串
	selectorStr := labels.Set(selector.MatchLabels).String()
	pods, err := c.client.CoreV1().Pods(namespace).List(c.ctx, metav1.ListOptions{
		LabelSelector: selectorStr,
	})

	if err == nil {
		for _, pod := range pods.Items {
			podKey := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
			managedPods[podKey] = true
		}
	}
}

func (c *configMapOperator) CanDelete(namespace, name string) (bool, string, error) {
	usage, err := c.GetUsage(namespace, name)
	if err != nil {
		return false, "", err
	}

	return usage.CanDelete, usage.DeleteWarning, nil
}

func (c *configMapOperator) Get(namespace, name string) (*corev1.ConfigMap, error) {
	if namespace == "" || name == "" {
		return nil, fmt.Errorf("命名空间和名称不能为空")
	}

	if c.cmLister != nil {
		cm, err := c.cmLister.ConfigMaps(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("ConfigMap %s/%s 不存在", namespace, name)
			}
			cm, apiErr := c.client.CoreV1().ConfigMaps(namespace).Get(c.ctx, name, metav1.GetOptions{})
			if apiErr != nil {
				return nil, fmt.Errorf("获取ConfigMap失败")
			}
			cm.TypeMeta = metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			}
			return cm, nil
		}
		cm.TypeMeta = metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		}
		return cm, nil
	}

	cm, err := c.client.CoreV1().ConfigMaps(namespace).Get(c.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("ConfigMap %s/%s 不存在", namespace, name)
		}
		return nil, fmt.Errorf("获取ConfigMap失败")
	}

	//  手动注入 TypeMeta
	cm.TypeMeta = metav1.TypeMeta{
		Kind:       "ConfigMap",
		APIVersion: "v1",
	}

	return cm, nil
}

func (c *configMapOperator) GetYaml(namespace, name string) (string, error) {
	cm, err := c.Get(namespace, name)
	if err != nil {
		return "", err
	}

	//  确保有 TypeMeta
	cm.TypeMeta = metav1.TypeMeta{
		Kind:       "ConfigMap",
		APIVersion: "v1",
	}

	cm.ManagedFields = nil
	yamlBytes, err := yaml.Marshal(cm)
	if err != nil {
		return "", fmt.Errorf("转换为YAML失败: %v", err)
	}

	return string(yamlBytes), nil
}
