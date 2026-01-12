package operator

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// podOperator Pod 操作器实现
type podOperator struct {
	BaseOperator
	client          kubernetes.Interface
	config          *rest.Config
	podLister       v1.PodLister
	podInformer     cache.SharedIndexInformer
	informerFactory informers.SharedInformerFactory
}

// NewPodOperator 创建 Pod 操作器（不使用 Informer）
func NewPodOperator(ctx context.Context, client kubernetes.Interface, config *rest.Config) types.PodOperator {
	return &podOperator{
		BaseOperator:    NewBaseOperator(ctx, false),
		client:          client,
		config:          config,
		informerFactory: nil,
	}
}

// NewPodOperatorWithInformer 创建带 Informer 的 Pod 操作器
func NewPodOperatorWithInformer(
	ctx context.Context,
	client kubernetes.Interface,
	config *rest.Config,
	informerFactory informers.SharedInformerFactory,
) types.PodOperator {
	var podLister v1.PodLister
	var podInformer cache.SharedIndexInformer

	if informerFactory != nil {
		podInformer = informerFactory.Core().V1().Pods().Informer()
		podLister = informerFactory.Core().V1().Pods().Lister()
	}

	return &podOperator{
		BaseOperator:    NewBaseOperator(ctx, informerFactory != nil),
		client:          client,
		config:          config,
		informerFactory: informerFactory,
		podLister:       podLister,
		podInformer:     podInformer,
	}
}

// ========== 基础 CRUD 操作 ==========

// Get 获取 Pod
func (p *podOperator) Get(namespace, name string) (*corev1.Pod, error) {
	if namespace == "" || name == "" {
		p.log.Error("获取Pod失败：命名空间和Pod名称不能为空")
		return nil, fmt.Errorf("命名空间和Pod名称不能为空")
	}

	p.log.Infof("开始获取Pod: namespace=%s, name=%s", namespace, name)

	if p.podLister != nil {
		pod, err := p.podLister.Pods(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("pod %s/%s 不存在", namespace, name)
			}
			pod, apiErr := p.client.CoreV1().Pods(namespace).Get(p.ctx, name, metav1.GetOptions{})
			if apiErr != nil {
				if errors.IsNotFound(apiErr) {
					return nil, fmt.Errorf("Pod %s/%s 不存在", namespace, name)
				}
				return nil, fmt.Errorf("获取Pod失败: %s/%s", namespace, name)
			}
			pod.TypeMeta = metav1.TypeMeta{
				APIVersion: pod.APIVersion,
				Kind:       pod.Kind,
			}
			return pod, nil
		}
		pod.TypeMeta = metav1.TypeMeta{
			APIVersion: pod.APIVersion,
			Kind:       pod.Kind,
		}
		return pod, nil
	}

	pod, err := p.client.CoreV1().Pods(namespace).Get(p.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("Pod %s/%s 不存在", namespace, name)
		}
		return nil, fmt.Errorf("获取Pod失败: %s/%s", namespace, name)
	}
	pod.TypeMeta = metav1.TypeMeta{
		APIVersion: pod.APIVersion,
		Kind:       pod.Kind,
	}
	return pod, nil
}

// List 列出 Pod
func (p *podOperator) List(namespace string, req types.ListRequest) (*types.ListPodResponse, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}
	if req.SortBy == "" {
		req.SortBy = "name"
	}

	var selector labels.Selector = labels.Everything()
	if req.Labels != "" {
		parsedSelector, err := labels.Parse(req.Labels)
		if err != nil {
			return nil, fmt.Errorf("解析标签选择器失败")
		}
		selector = parsedSelector
	}

	var pods []*corev1.Pod
	var err error

	if p.useInformer && p.podLister != nil {
		pods, err = p.podLister.Pods(namespace).List(selector)
		if err != nil {
			return nil, fmt.Errorf("获取Pod列表失败")
		}
	} else {
		listOpts := metav1.ListOptions{LabelSelector: selector.String()}
		podList, err := p.client.CoreV1().Pods(namespace).List(p.ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("获取Pod列表失败")
		}
		pods = make([]*corev1.Pod, len(podList.Items))
		for i := range podList.Items {
			pods[i] = &podList.Items[i]
		}
	}

	if req.Search != "" {
		filtered := make([]*corev1.Pod, 0)
		searchLower := strings.ToLower(req.Search)
		for _, pod := range pods {
			if strings.Contains(strings.ToLower(pod.Name), searchLower) {
				filtered = append(filtered, pod)
			}
		}
		pods = filtered
	}

	sort.Slice(pods, func(i, j int) bool {
		var less bool
		switch req.SortBy {
		case "creationTime", "creationTimestamp":
			less = pods[i].CreationTimestamp.Before(&pods[j].CreationTimestamp)
		case "status":
			less = string(pods[i].Status.Phase) < string(pods[j].Status.Phase)
		default:
			less = pods[i].Name < pods[j].Name
		}
		if req.SortDesc {
			return !less
		}
		return less
	})

	total := len(pods)
	totalPages := (total + req.PageSize - 1) / req.PageSize
	start := (req.Page - 1) * req.PageSize
	end := start + req.PageSize

	if start >= total {
		return &types.ListPodResponse{
			ListResponse: types.ListResponse{
				Total:      total,
				Page:       req.Page,
				PageSize:   req.PageSize,
				TotalPages: totalPages,
			},
			Items: []types.PodDetailInfo{},
		}, nil
	}

	if end > total {
		end = total
	}

	pagePods := pods[start:end]
	items := make([]types.PodDetailInfo, len(pagePods))
	for i, pod := range pagePods {
		items[i] = p.convertToPodInfo(pod)
	}

	return &types.ListPodResponse{
		ListResponse: types.ListResponse{
			Total:      total,
			Page:       req.Page,
			PageSize:   req.PageSize,
			TotalPages: totalPages,
		},
		Items: items,
	}, nil
}

// toYAML 将 Kubernetes 对象转换为 YAML 字符串
func toYAML(obj runtime.Object, apiVersion, kind string) (string, error) {
	// 设置 TypeMeta
	if gvk := obj.GetObjectKind().GroupVersionKind(); gvk.Empty() {
		obj.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "",
			Version: apiVersion,
			Kind:    kind,
		})
	}

	yamlPrinter := &printers.YAMLPrinter{}
	var buf bytes.Buffer
	err := yamlPrinter.PrintObj(obj, &buf)
	if err != nil {
		return "", fmt.Errorf("转换为YAML失败: %v", err)
	}

	return buf.String(), nil
}

// 然后在 GetYaml 中使用
func (p *podOperator) GetYaml(namespace, name string) (string, error) {
	pod, err := p.Get(namespace, name)
	if err != nil {
		return "", err
	}

	return toYAML(pod, "v1", "Pod")
}

// Delete 删除 Pod
func (p *podOperator) Delete(namespace, name string) error {
	if namespace == "" || name == "" {
		return fmt.Errorf("命名空间和名称不能为空")
	}

	err := p.client.CoreV1().Pods(namespace).Delete(p.ctx, name, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("删除Pod失败")
	}

	return nil
}

// BatchDelete 批量删除 Pod
func (p *podOperator) BatchDelete(namespace string, names []string, opts metav1.DeleteOptions) (succeeded int, failed int, errors []error) {
	if namespace == "" {
		err := fmt.Errorf("命名空间不能为空")
		return 0, len(names), []error{err}
	}
	if len(names) == 0 {
		return 0, 0, nil
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(names))
	semaphore := make(chan struct{}, 10)

	for _, name := range names {
		if name == "" {
			failed++
			errors = append(errors, fmt.Errorf("pod名称为空"))
			continue
		}

		wg.Add(1)
		go func(podName string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if err := p.Delete(namespace, podName); err != nil {
				errChan <- fmt.Errorf("删除Pod %s 失败", podName)
			} else {
				errChan <- nil
			}
		}(name)
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	for err := range errChan {
		if err != nil {
			failed++
			errors = append(errors, err)
		} else {
			succeeded++
		}
	}

	return succeeded, failed, errors
}

// DeleteBySelector 根据标签选择器删除 Pod
func (p *podOperator) DeleteBySelector(namespace string, labelSelector string, opts metav1.DeleteOptions) error {
	if namespace == "" {
		return fmt.Errorf("命名空间不能为空")
	}
	if labelSelector == "" {
		return fmt.Errorf("标签选择器不能为空")
	}

	listOpts := metav1.ListOptions{
		LabelSelector: labelSelector,
	}

	podList, err := p.client.CoreV1().Pods(namespace).List(p.ctx, listOpts)
	if err != nil {
		return fmt.Errorf("列出匹配选择器的Pod失败")
	}

	if len(podList.Items) == 0 {
		return nil
	}

	podNames := make([]string, 0, len(podList.Items))
	for _, pod := range podList.Items {
		podNames = append(podNames, pod.Name)
	}

	succeeded, failed, errs := p.BatchDelete(namespace, podNames, opts)

	if failed > 0 {
		errMsg := fmt.Sprintf("删除Pod时有 %d 个失败", failed)
		if len(errs) > 0 {
			for i, e := range errs {
				if i == 0 {
					errMsg += ": "
				} else {
					errMsg += "; "
				}
				errMsg += e.Error()
			}
		}
		return fmt.Errorf(errMsg)
	}

	p.log.Infof("成功删除所有匹配的Pod: count=%d", succeeded)
	return nil
}

// Create 创建 Pod
func (p *podOperator) Create(pod *corev1.Pod) (*corev1.Pod, error) {
	if pod == nil {
		return nil, fmt.Errorf("pod对象不能为空")
	}
	if pod.Name == "" || pod.Namespace == "" {
		return nil, fmt.Errorf("pod名称和命名空间不能为空")
	}

	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}
	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}
	injectCommonAnnotations(pod)
	createdPod, err := p.client.CoreV1().Pods(pod.Namespace).Create(p.ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("创建Pod失败: %v", err)
	}

	return createdPod, nil
}

// Update 更新 Pod
func (p *podOperator) Update(pod *corev1.Pod) (*corev1.Pod, error) {
	if pod == nil || pod.Name == "" || pod.Namespace == "" {
		return nil, fmt.Errorf("pod对象、名称和命名空间不能为空")
	}

	updatedPod, err := p.client.CoreV1().Pods(pod.Namespace).Update(p.ctx, pod, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("更新Pod失败")
	}

	return updatedPod, nil
}

// ========== Part 2: 继续 operator/pod.go ==========
// 将此部分添加到 Part 1 之后

// ========== 容器操作 ==========

// GetContainers 获取 Pod 的所有容器
func (p *podOperator) GetContainers(namespace, name string) ([]corev1.Container, error) {
	pod, err := p.Get(namespace, name)
	if err != nil {
		return nil, fmt.Errorf("获取Pod失败")
	}
	return pod.Spec.Containers, nil
}

// GetDefaultContainer 获取 Pod 的默认容器名称
func (p *podOperator) GetDefaultContainer(namespace, name string) (string, error) {
	containers, err := p.GetContainers(namespace, name)
	if err != nil {
		return "", err
	}

	if len(containers) == 0 {
		return "", fmt.Errorf("pod没有容器")
	}

	return containers[0].Name, nil
}

// GetAllContainers 获取 Pod 的所有容器（包括 Init 容器和临时容器）
func (p *podOperator) GetAllContainers(namespace, name string) (*types.ContainerInfoList, error) {
	if namespace == "" || name == "" {
		return nil, fmt.Errorf("命名空间和Pod名称不能为空")
	}

	pod, err := p.Get(namespace, name)
	if err != nil {
		return nil, fmt.Errorf("获取Pod失败: %v", err)
	}

	containers := &types.ContainerInfoList{
		InitContainers:      make([]types.ContainerInfo, 0),
		Containers:          make([]types.ContainerInfo, 0),
		EphemeralContainers: make([]types.ContainerInfo, 0),
	}

	// 普通容器状态映射
	containerStatusMap := make(map[string]*types.ContainerStatusDetails)
	for i := range pod.Status.ContainerStatuses {
		cs := &pod.Status.ContainerStatuses[i]
		containerStatusMap[cs.Name] = &types.ContainerStatusDetails{
			State:        convertContainerState(cs.State),
			LastState:    convertContainerState(cs.LastTerminationState),
			Ready:        cs.Ready,
			RestartCount: cs.RestartCount,
			Image:        cs.Image,
			ImageID:      cs.ImageID,
			ContainerID:  cs.ContainerID,
			Started:      cs.Started,
		}
	}

	// Init 容器状态映射
	initContainerStatusMap := make(map[string]*types.ContainerStatusDetails)
	for i := range pod.Status.InitContainerStatuses {
		cs := &pod.Status.InitContainerStatuses[i]
		initContainerStatusMap[cs.Name] = &types.ContainerStatusDetails{
			State:        convertContainerState(cs.State),
			LastState:    convertContainerState(cs.LastTerminationState),
			Ready:        cs.Ready,
			RestartCount: cs.RestartCount,
			Image:        cs.Image,
			ImageID:      cs.ImageID,
			ContainerID:  cs.ContainerID,
			Started:      cs.Started,
		}
	}

	// 临时容器状态映射
	ephemeralContainerStatusMap := make(map[string]*types.ContainerStatusDetails)
	for i := range pod.Status.EphemeralContainerStatuses {
		cs := &pod.Status.EphemeralContainerStatuses[i]
		ephemeralContainerStatusMap[cs.Name] = &types.ContainerStatusDetails{
			State:        convertContainerState(cs.State),
			LastState:    convertContainerState(cs.LastTerminationState),
			Ready:        cs.Ready,
			RestartCount: cs.RestartCount,
			Image:        cs.Image,
			ImageID:      cs.ImageID,
			ContainerID:  cs.ContainerID,
			Started:      cs.Started,
		}
	}

	// 添加 Init 容器
	for _, c := range pod.Spec.InitContainers {
		containerInfo := types.ContainerInfo{
			Name:  c.Name,
			Image: c.Image,
		}
		if status, exists := initContainerStatusMap[c.Name]; exists {
			containerInfo.Ready = status.Ready
			containerInfo.RestartCount = status.RestartCount
			containerInfo.State = status.State.Type
			containerInfo.Status = status
		} else {
			containerInfo.State = "Waiting"
		}
		containers.InitContainers = append(containers.InitContainers, containerInfo)
	}

	// 添加普通容器
	for _, c := range pod.Spec.Containers {
		containerInfo := types.ContainerInfo{
			Name:  c.Name,
			Image: c.Image,
		}
		if status, exists := containerStatusMap[c.Name]; exists {
			containerInfo.Ready = status.Ready
			containerInfo.RestartCount = status.RestartCount
			containerInfo.State = status.State.Type
			containerInfo.Status = status
		} else {
			containerInfo.State = "Waiting"
		}
		containers.Containers = append(containers.Containers, containerInfo)
	}

	// 添加临时容器
	for _, c := range pod.Spec.EphemeralContainers {
		containerInfo := types.ContainerInfo{
			Name:  c.Name,
			Image: c.Image,
		}
		if status, exists := ephemeralContainerStatusMap[c.Name]; exists {
			containerInfo.Ready = status.Ready
			containerInfo.RestartCount = status.RestartCount
			containerInfo.State = status.State.Type
			containerInfo.Status = status
		} else {
			containerInfo.State = "Waiting"
		}
		containers.EphemeralContainers = append(containers.EphemeralContainers, containerInfo)
	}

	return containers, nil
}

// ========== 状态检查 ==========

// IsReady 检查 Pod 是否就绪
func (p *podOperator) IsReady(namespace, name string) (bool, error) {
	pod, err := p.Get(namespace, name)
	if err != nil {
		return false, err
	}

	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue, nil
		}
	}
	return false, nil
}

// GetPhase 获取 Pod 阶段
func (p *podOperator) GetPhase(namespace, name string) (corev1.PodPhase, error) {
	if namespace == "" || name == "" {
		return "", fmt.Errorf("命名空间和Pod名称不能为空")
	}

	pod, err := p.Get(namespace, name)
	if err != nil {
		return "", err
	}

	return pod.Status.Phase, nil
}

// IsRunning 检查 Pod 是否运行中
func (p *podOperator) IsRunning(namespace, name string) (bool, error) {
	phase, err := p.GetPhase(namespace, name)
	if err != nil {
		return false, err
	}
	return phase == corev1.PodRunning, nil
}

// IsPending 检查 Pod 是否等待中
func (p *podOperator) IsPending(namespace, name string) (bool, error) {
	phase, err := p.GetPhase(namespace, name)
	if err != nil {
		return false, err
	}
	return phase == corev1.PodPending, nil
}

// IsSucceeded 检查 Pod 是否成功
func (p *podOperator) IsSucceeded(namespace, name string) (bool, error) {
	phase, err := p.GetPhase(namespace, name)
	if err != nil {
		return false, err
	}
	return phase == corev1.PodSucceeded, nil
}

// IsFailed 检查 Pod 是否失败
func (p *podOperator) IsFailed(namespace, name string) (bool, error) {
	phase, err := p.GetPhase(namespace, name)
	if err != nil {
		return false, err
	}
	return phase == corev1.PodFailed, nil
}

// IsTerminating 检查 Pod 是否正在终止
func (p *podOperator) IsTerminating(namespace, name string) (bool, error) {
	pod, err := p.Get(namespace, name)
	if err != nil {
		return false, err
	}
	return pod.DeletionTimestamp != nil, nil
}

// ========== 高级操作 ==========

// Evict 驱逐 Pod
func (p *podOperator) Evict(namespace, name string) error {
	eviction := &policyv1.Eviction{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		DeleteOptions: &metav1.DeleteOptions{},
	}

	err := p.client.CoreV1().Pods(namespace).EvictV1(p.ctx, eviction)
	if err != nil {
		return fmt.Errorf("驱逐Pod失败: %s/%s", namespace, name)
	}
	return nil
}

// UpdateLabels 更新 Pod 标签
func (p *podOperator) UpdateLabels(namespace, name string, labels map[string]string) error {
	pod, err := p.Get(namespace, name)
	if err != nil {
		return err
	}

	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}
	for k, v := range labels {
		pod.Labels[k] = v
	}

	_, err = p.client.CoreV1().Pods(namespace).Update(p.ctx, pod, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("更新Pod标签失败")
	}
	return nil
}

// UpdateAnnotations 更新 Pod 注解
func (p *podOperator) UpdateAnnotations(namespace, name string, annotations map[string]string) error {
	pod, err := p.Get(namespace, name)
	if err != nil {
		return err
	}

	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		pod.Annotations[k] = v
	}

	_, err = p.client.CoreV1().Pods(namespace).Update(p.ctx, pod, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("更新Pod注解失败")
	}
	return nil
}

// InjectEphemeralContainer 注入临时容器
func (p *podOperator) InjectEphemeralContainer(req *types.InjectEphemeralContainerRequest) error {
	if req == nil || req.PodName == "" || req.Namespace == "" {
		return fmt.Errorf("请求参数不完整")
	}

	pod, err := p.Get(req.Namespace, req.PodName)
	if err != nil {
		return err
	}

	containerName := req.ContainerName
	if containerName == "" {
		containerName = "debugger"
	}

	image := req.Image
	if image == "" {
		image = "nicolaka/netshoot:latest"
	}

	command := req.Command
	if len(command) == 0 {
		command = []string{"sh"}
	}

	ephemeralContainer := corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:    containerName,
			Image:   image,
			Command: command,
			Args:    req.Args,
			TTY:     true,
			Stdin:   true,
		},
	}

	pod.Spec.EphemeralContainers = append(pod.Spec.EphemeralContainers, ephemeralContainer)

	_, err = p.client.CoreV1().Pods(req.Namespace).UpdateEphemeralContainers(
		p.ctx,
		req.PodName,
		pod,
		metav1.UpdateOptions{},
	)

	if err != nil {
		return fmt.Errorf("注入临时容器失败: %v", err)
	}
	return nil
}

// ========== 资源查询操作 ==========

// GetResourcePodsDetailList 获取指定资源的 Pods 详细信息
func (p *podOperator) GetResourcePodsDetailList(req *types.GetResourcePodsDetailRequest) ([]types.PodDetailInfo, error) {
	if req == nil || req.Namespace == "" || req.ResourceType == "" {
		return nil, fmt.Errorf("请求参数不完整")
	}

	var pods []*corev1.Pod
	var err error

	switch strings.ToLower(req.ResourceType) {
	case "pod", "pods", "po":
		pods, err = p.getPodsForPodType(req.Namespace, req.ResourceName)
	case "deployment", "deployments", "deploy":
		pods, err = p.getPodsForDeployment(req.Namespace, req.ResourceName)
	case "statefulset", "statefulsets", "sts":
		pods, err = p.getPodsForStatefulSet(req.Namespace, req.ResourceName)
	case "daemonset", "daemonsets", "ds":
		pods, err = p.getPodsForDaemonSet(req.Namespace, req.ResourceName)
	case "replicaset", "replicasets", "rs":
		pods, err = p.getPodsForReplicaSet(req.Namespace, req.ResourceName)
	case "job", "jobs":
		pods, err = p.getPodsForJob(req.Namespace, req.ResourceName)
	case "cronjob", "cronjobs", "cj":
		pods, err = p.getPodsForCronJob(req.Namespace, req.ResourceName)
	default:
		return nil, fmt.Errorf("不支持的资源类型: %s", req.ResourceType)
	}

	if err != nil {
		return nil, err
	}

	result := make([]types.PodDetailInfo, 0, len(pods))
	for _, pod := range pods {
		result = append(result, p.convertToPodDetailInfo(pod))
	}

	return result, nil
}

// GetResourcePods 获取资源关联的 Pod 列表
func (p *podOperator) GetResourcePods(req *types.GetPodsRequest) ([]types.PodDetailInfo, error) {
	var pods *corev1.PodList
	var err error

	// 根据资源类型获取对应的 Pod
	switch strings.ToLower(req.ResourceType) {
	case "pod":
		if req.ResourceName != "" {
			pod, err := p.client.CoreV1().Pods(req.Namespace).Get(p.ctx, req.ResourceName, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("获取 Pod 失败: %w", err)
			}
			pods = &corev1.PodList{Items: []corev1.Pod{*pod}}
		} else {
			pods, err = p.client.CoreV1().Pods(req.Namespace).List(p.ctx, metav1.ListOptions{})
			if err != nil {
				return nil, fmt.Errorf("获取 Pod 列表失败: %w", err)
			}
		}

	case "deployment":
		deployment, err := p.client.AppsV1().Deployments(req.Namespace).Get(p.ctx, req.ResourceName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("获取 Deployment 失败: %w", err)
		}
		selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
		if err != nil {
			return nil, fmt.Errorf("解析 label selector 失败: %w", err)
		}
		pods, err = p.client.CoreV1().Pods(req.Namespace).List(p.ctx, metav1.ListOptions{
			LabelSelector: selector.String(),
		})
		if err != nil {
			return nil, fmt.Errorf("获取 Deployment Pods 失败: %w", err)
		}

	case "statefulset", "sts":
		sts, err := p.client.AppsV1().StatefulSets(req.Namespace).Get(p.ctx, req.ResourceName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("获取 StatefulSet 失败: %w", err)
		}
		selector, err := metav1.LabelSelectorAsSelector(sts.Spec.Selector)
		if err != nil {
			return nil, fmt.Errorf("解析 label selector 失败: %w", err)
		}
		pods, err = p.client.CoreV1().Pods(req.Namespace).List(p.ctx, metav1.ListOptions{
			LabelSelector: selector.String(),
		})
		if err != nil {
			return nil, fmt.Errorf("获取 StatefulSet Pods 失败: %w", err)
		}

	case "daemonset", "ds":
		ds, err := p.client.AppsV1().DaemonSets(req.Namespace).Get(p.ctx, req.ResourceName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("获取 DaemonSet 失败: %w", err)
		}
		selector, err := metav1.LabelSelectorAsSelector(ds.Spec.Selector)
		if err != nil {
			return nil, fmt.Errorf("解析 label selector 失败: %w", err)
		}
		pods, err = p.client.CoreV1().Pods(req.Namespace).List(p.ctx, metav1.ListOptions{
			LabelSelector: selector.String(),
		})
		if err != nil {
			return nil, fmt.Errorf("获取 DaemonSet Pods 失败: %w", err)
		}

	case "replicaset", "rs":
		rs, err := p.client.AppsV1().ReplicaSets(req.Namespace).Get(p.ctx, req.ResourceName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("获取 ReplicaSet 失败: %w", err)
		}
		selector, err := metav1.LabelSelectorAsSelector(rs.Spec.Selector)
		if err != nil {
			return nil, fmt.Errorf("解析 label selector 失败: %w", err)
		}
		pods, err = p.client.CoreV1().Pods(req.Namespace).List(p.ctx, metav1.ListOptions{
			LabelSelector: selector.String(),
		})
		if err != nil {
			return nil, fmt.Errorf("获取 ReplicaSet Pods 失败: %w", err)
		}

	case "job":
		job, err := p.client.BatchV1().Jobs(req.Namespace).Get(p.ctx, req.ResourceName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("获取 Job 失败: %w", err)
		}
		selector, err := metav1.LabelSelectorAsSelector(job.Spec.Selector)
		if err != nil {
			return nil, fmt.Errorf("解析 label selector 失败: %w", err)
		}
		pods, err = p.client.CoreV1().Pods(req.Namespace).List(p.ctx, metav1.ListOptions{
			LabelSelector: selector.String(),
		})
		if err != nil {
			return nil, fmt.Errorf("获取 Job Pods 失败: %w", err)
		}

	case "cronjob":
		cronJob, err := p.client.BatchV1().CronJobs(req.Namespace).Get(p.ctx, req.ResourceName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("获取 CronJob 失败: %w", err)
		}
		jobs, err := p.client.BatchV1().Jobs(req.Namespace).List(p.ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("获取 Jobs 失败: %w", err)
		}
		var cronJobPods []corev1.Pod
		for _, job := range jobs.Items {
			for _, owner := range job.OwnerReferences {
				if owner.Kind == "CronJob" && owner.Name == cronJob.Name {
					selector, _ := metav1.LabelSelectorAsSelector(job.Spec.Selector)
					jobPods, err := p.client.CoreV1().Pods(req.Namespace).List(p.ctx, metav1.ListOptions{
						LabelSelector: selector.String(),
					})
					if err == nil {
						cronJobPods = append(cronJobPods, jobPods.Items...)
					}
					break
				}
			}
		}
		pods = &corev1.PodList{Items: cronJobPods}

	default:
		return nil, fmt.Errorf("不支持的资源类型: %s", req.ResourceType)
	}

	result := make([]types.PodDetailInfo, 0, len(pods.Items))
	for _, pod := range pods.Items {
		podInfo := p.convertPodToPodDetailInfo(&pod)
		result = append(result, podInfo)
	}

	return result, nil
}

// convertPodToPodDetailInfo 将 K8s Pod 转换为 PodDetailInfo
func (p *podOperator) convertPodToPodDetailInfo(pod *corev1.Pod) types.PodDetailInfo {
	status := p.getPodStatus(pod)
	ready := p.getPodReadyStatus(pod)
	restarts := p.getPodRestartCount(pod)
	age := p.getPodAge(pod)

	return types.PodDetailInfo{
		Name:         pod.Name,
		Namespace:    pod.Namespace,
		Status:       status,
		Ready:        ready,
		Restarts:     restarts,
		Age:          age,
		Node:         pod.Spec.NodeName,
		PodIP:        pod.Status.PodIP,
		Labels:       pod.Labels,
		CreationTime: pod.CreationTimestamp.Unix(),
	}
}

// getPodStatus 完整的 Pod 状态判断逻辑
func (p *podOperator) getPodStatus(pod *corev1.Pod) string {
	return getPodStatusCore(pod)
}

// hasPodReadyCondition 检查 Pod 是否处于 Ready 状态（方法版本）
func (p *podOperator) hasPodReadyCondition(conditions []corev1.PodCondition) bool {
	return hasPodReadyConditionStatic(conditions)
}

// normalizeStatus 标准化状态字符串
func (p *podOperator) normalizeStatus(status string) string {
	return normalizeStatusStatic(status)
}

// getPodReadyStatus 获取 Pod 就绪状态
func (p *podOperator) getPodReadyStatus(pod *corev1.Pod) string {
	readyContainers := 0
	totalContainers := len(pod.Status.ContainerStatuses)

	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.Ready {
			readyContainers++
		}
	}

	return fmt.Sprintf("%d/%d", readyContainers, totalContainers)
}

// getPodRestartCount 获取 Pod 总重启次数
func (p *podOperator) getPodRestartCount(pod *corev1.Pod) int32 {
	var totalRestarts int32
	for _, containerStatus := range pod.Status.ContainerStatuses {
		totalRestarts += containerStatus.RestartCount
	}
	return totalRestarts
}

// getPodAge 计算 Pod 运行时长
func (p *podOperator) getPodAge(pod *corev1.Pod) string {
	duration := time.Since(pod.CreationTimestamp.Time)

	days := int(duration.Hours() / 24)
	hours := int(duration.Hours()) % 24
	minutes := int(duration.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd%dh", days, hours)
	} else if hours > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	} else {
		return fmt.Sprintf("%dm", minutes)
	}
}

// getContainerState 获取容器状态字符串
func (p *podOperator) getContainerState(status *corev1.ContainerStatus) string {
	if status.State.Running != nil {
		return "Running"
	}
	if status.State.Waiting != nil {
		return "Waiting"
	}
	if status.State.Terminated != nil {
		return "Terminated"
	}
	return "Unknown"
}

// getContainerStatusDetails 获取容器详细状态
func (p *podOperator) getContainerStatusDetails(status *corev1.ContainerStatus) *types.ContainerStatusDetails {
	details := &types.ContainerStatusDetails{
		Ready:        status.Ready,
		RestartCount: status.RestartCount,
		Image:        status.Image,
		ImageID:      status.ImageID,
		ContainerID:  status.ContainerID,
		Started:      status.Started,
	}

	details.State = p.convertContainerState(status.State)
	details.LastState = p.convertContainerState(status.LastTerminationState)

	return details
}

// convertContainerState 转换容器状态
func (p *podOperator) convertContainerState(state corev1.ContainerState) types.ContainerStateInfo {
	stateInfo := types.ContainerStateInfo{}

	if state.Running != nil {
		stateInfo.Type = "Running"
		stateInfo.Running = &types.ContainerStateRunning{
			StartedAt: state.Running.StartedAt.UnixMilli(),
		}
	} else if state.Waiting != nil {
		stateInfo.Type = "Waiting"
		stateInfo.Waiting = &types.ContainerStateWaiting{
			Reason:  state.Waiting.Reason,
			Message: state.Waiting.Message,
		}
	} else if state.Terminated != nil {
		stateInfo.Type = "Terminated"
		stateInfo.Terminated = &types.ContainerStateTerminated{
			ExitCode:   state.Terminated.ExitCode,
			Reason:     state.Terminated.Reason,
			Message:    state.Terminated.Message,
			StartedAt:  state.Terminated.StartedAt.UnixMilli(),
			FinishedAt: state.Terminated.FinishedAt.UnixMilli(),
		}
	}

	return stateInfo
}

// GetStandalonePods 获取所有裸 Pod（不属于任何控制器）
func (p *podOperator) GetStandalonePods(namespace string) ([]types.PodDetailInfo, error) {
	podList, err := p.client.CoreV1().Pods(namespace).List(p.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取Pod列表失败")
	}

	standalonePods := make([]types.PodDetailInfo, 0)
	for _, pod := range podList.Items {
		if len(pod.OwnerReferences) == 0 {
			standalonePods = append(standalonePods, p.convertToPodInfo(&pod))
		}
	}

	return standalonePods, nil
}

// ========== 资源查询辅助方法 ==========

func (p *podOperator) getPodsForPodType(namespace, name string) ([]*corev1.Pod, error) {
	if name != "" {
		pod, err := p.Get(namespace, name)
		if err != nil {
			return nil, err
		}
		return []*corev1.Pod{pod}, nil
	}

	podList, err := p.client.CoreV1().Pods(namespace).List(p.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取Pod列表失败")
	}

	standalonePods := make([]*corev1.Pod, 0)
	for i := range podList.Items {
		if len(podList.Items[i].OwnerReferences) == 0 {
			standalonePods = append(standalonePods, &podList.Items[i])
		}
	}
	return standalonePods, nil
}

func (p *podOperator) getPodsForDeployment(namespace, name string) ([]*corev1.Pod, error) {
	if name == "" {
		return nil, fmt.Errorf("Deployment名称不能为空")
	}

	deployment, err := p.client.AppsV1().Deployments(namespace).Get(p.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("Deployment %s/%s 不存在", namespace, name)
		}
		return nil, fmt.Errorf("获取Deployment失败")
	}

	labelSelector := metav1.FormatLabelSelector(deployment.Spec.Selector)
	return p.getPodsBySelector(namespace, labelSelector)
}

func (p *podOperator) getPodsForStatefulSet(namespace, name string) ([]*corev1.Pod, error) {
	if name == "" {
		return nil, fmt.Errorf("StatefulSet名称不能为空")
	}

	sts, err := p.client.AppsV1().StatefulSets(namespace).Get(p.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("StatefulSet %s/%s 不存在", namespace, name)
		}
		return nil, fmt.Errorf("获取StatefulSet失败")
	}

	labelSelector := metav1.FormatLabelSelector(sts.Spec.Selector)
	return p.getPodsBySelector(namespace, labelSelector)
}

func (p *podOperator) getPodsForDaemonSet(namespace, name string) ([]*corev1.Pod, error) {
	if name == "" {
		return nil, fmt.Errorf("DaemonSet名称不能为空")
	}

	ds, err := p.client.AppsV1().DaemonSets(namespace).Get(p.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("DaemonSet %s/%s 不存在", namespace, name)
		}
		return nil, fmt.Errorf("获取DaemonSet失败")
	}

	labelSelector := metav1.FormatLabelSelector(ds.Spec.Selector)
	return p.getPodsBySelector(namespace, labelSelector)
}

func (p *podOperator) getPodsForReplicaSet(namespace, name string) ([]*corev1.Pod, error) {
	if name == "" {
		return nil, fmt.Errorf("ReplicaSet名称不能为空")
	}

	rs, err := p.client.AppsV1().ReplicaSets(namespace).Get(p.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("ReplicaSet %s/%s 不存在", namespace, name)
		}
		return nil, fmt.Errorf("获取ReplicaSet失败")
	}

	labelSelector := metav1.FormatLabelSelector(rs.Spec.Selector)
	return p.getPodsBySelector(namespace, labelSelector)
}

func (p *podOperator) getPodsForJob(namespace, name string) ([]*corev1.Pod, error) {
	if name == "" {
		return nil, fmt.Errorf("Job名称不能为空")
	}

	job, err := p.client.BatchV1().Jobs(namespace).Get(p.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("Job %s/%s 不存在", namespace, name)
		}
		return nil, fmt.Errorf("获取Job失败")
	}

	labelSelector := metav1.FormatLabelSelector(job.Spec.Selector)
	return p.getPodsBySelector(namespace, labelSelector)
}

func (p *podOperator) getPodsForCronJob(namespace, name string) ([]*corev1.Pod, error) {
	if name == "" {
		return nil, fmt.Errorf("CronJob名称不能为空")
	}

	cronJob, err := p.client.BatchV1().CronJobs(namespace).Get(p.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("CronJob %s/%s 不存在", namespace, name)
		}
		return nil, fmt.Errorf("获取CronJob失败")
	}

	jobList, err := p.client.BatchV1().Jobs(namespace).List(p.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取Job列表失败")
	}

	allPods := make([]*corev1.Pod, 0)
	for _, job := range jobList.Items {
		isOwned := false
		for _, owner := range job.OwnerReferences {
			if owner.Kind == "CronJob" && owner.Name == cronJob.Name && owner.UID == cronJob.UID {
				isOwned = true
				break
			}
		}

		if !isOwned {
			continue
		}

		labelSelector := metav1.FormatLabelSelector(job.Spec.Selector)
		pods, err := p.getPodsBySelector(namespace, labelSelector)
		if err != nil {
			continue
		}
		allPods = append(allPods, pods...)
	}
	return allPods, nil
}

func (p *podOperator) getPodsBySelector(namespace, labelSelector string) ([]*corev1.Pod, error) {
	p.log.Debugf("通过标签选择器获取Pods: namespace=%s, selector=%s", namespace, labelSelector)

	// 解析标签选择器
	selector, err := labels.Parse(labelSelector)
	if err != nil {
		p.log.Errorf("解析标签选择器失败: %s, error=%v", labelSelector, err)
		return nil, fmt.Errorf("解析标签选择器失败: %v", err)
	}

	var pods []*corev1.Pod

	//  优先使用 Informer
	if p.useInformer && p.podLister != nil {
		p.log.Debug("使用Informer通过标签选择器获取Pods")
		pods, err = p.podLister.Pods(namespace).List(selector)
		if err != nil {
			p.log.Errorf("Informer获取Pods失败: %v", err)
			// 降级到 API 调用
			p.log.Debug("Informer失败，降级使用API")
			podList, apiErr := p.client.CoreV1().Pods(namespace).List(p.ctx, metav1.ListOptions{
				LabelSelector: labelSelector,
			})
			if apiErr != nil {
				return nil, fmt.Errorf("获取Pods失败: %v", apiErr)
			}
			pods = make([]*corev1.Pod, len(podList.Items))
			for i := range podList.Items {
				pods[i] = &podList.Items[i]
			}
		}
		p.log.Debugf("成功获取 %d 个Pods", len(pods))
	} else {
		// 降级到 API 调用
		p.log.Debug("使用API通过标签选择器获取Pods")
		podList, err := p.client.CoreV1().Pods(namespace).List(p.ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			p.log.Errorf("API获取Pods失败: %v", err)
			return nil, fmt.Errorf("获取Pods失败: %v", err)
		}
		pods = make([]*corev1.Pod, len(podList.Items))
		for i := range podList.Items {
			pods[i] = &podList.Items[i]
		}
		p.log.Debugf("成功获取 %d 个Pods", len(pods))
	}

	return pods, nil
}

// Deployment Pods 辅助方法
func (p *podOperator) getDeploymentPods(namespace, name string) ([]types.PodDetailInfo, error) {
	if name == "" {
		return nil, fmt.Errorf("Deployment名称不能为空")
	}

	deployment, err := p.client.AppsV1().Deployments(namespace).Get(p.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("Deployment %s/%s 不存在", namespace, name)
		}
		return nil, fmt.Errorf("获取Deployment失败")
	}

	rsList, err := p.client.AppsV1().ReplicaSets(namespace).List(p.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取ReplicaSet列表失败")
	}

	var podList []types.PodDetailInfo
	for _, rs := range rsList.Items {
		isOwned := false
		for _, owner := range rs.OwnerReferences {
			if owner.Kind == "Deployment" && owner.Name == deployment.Name && owner.UID == deployment.UID {
				isOwned = true
				break
			}
		}

		if !isOwned {
			continue
		}

		rsPods, err := p.getReplicaSetPods(namespace, rs.Name)
		if err != nil {
			continue
		}
		podList = append(podList, rsPods...)
	}

	return podList, nil
}

func (p *podOperator) getStatefulSetPods(namespace, name string) ([]types.PodDetailInfo, error) {
	if name == "" {
		return nil, fmt.Errorf("StatefulSet名称不能为空")
	}

	sts, err := p.client.AppsV1().StatefulSets(namespace).Get(p.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("StatefulSet %s/%s 不存在", namespace, name)
		}
		return nil, fmt.Errorf("获取StatefulSet失败")
	}

	labelSelector := metav1.FormatLabelSelector(sts.Spec.Selector)
	return p.getPodsByLabelSelector(namespace, labelSelector)
}

func (p *podOperator) getDaemonSetPods(namespace, name string) ([]types.PodDetailInfo, error) {
	if name == "" {
		return nil, fmt.Errorf("DaemonSet名称不能为空")
	}

	ds, err := p.client.AppsV1().DaemonSets(namespace).Get(p.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("DaemonSet %s/%s 不存在", namespace, name)
		}
		return nil, fmt.Errorf("获取DaemonSet失败")
	}

	labelSelector := metav1.FormatLabelSelector(ds.Spec.Selector)
	return p.getPodsByLabelSelector(namespace, labelSelector)
}

func (p *podOperator) getReplicaSetPods(namespace, name string) ([]types.PodDetailInfo, error) {
	if name == "" {
		return nil, fmt.Errorf("ReplicaSet名称不能为空")
	}

	rs, err := p.client.AppsV1().ReplicaSets(namespace).Get(p.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("ReplicaSet %s/%s 不存在", namespace, name)
		}
		return nil, fmt.Errorf("获取ReplicaSet失败")
	}

	labelSelector := metav1.FormatLabelSelector(rs.Spec.Selector)
	return p.getPodsByLabelSelector(namespace, labelSelector)
}

func (p *podOperator) getJobPods(namespace, name string) ([]types.PodDetailInfo, error) {
	if name == "" {
		return nil, fmt.Errorf("Job名称不能为空")
	}

	job, err := p.client.BatchV1().Jobs(namespace).Get(p.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("Job %s/%s 不存在", namespace, name)
		}
		return nil, fmt.Errorf("获取Job失败")
	}

	labelSelector := metav1.FormatLabelSelector(job.Spec.Selector)
	return p.getPodsByLabelSelector(namespace, labelSelector)
}

func (p *podOperator) getPodsByLabelSelector(namespace, labelSelector string) ([]types.PodDetailInfo, error) {
	p.log.Debugf("通过标签选择器获取PodDetailInfo: namespace=%s, selector=%s", namespace, labelSelector)

	// 解析标签选择器
	selector, err := labels.Parse(labelSelector)
	if err != nil {
		p.log.Errorf("解析标签选择器失败: %s, error=%v", labelSelector, err)
		return nil, fmt.Errorf("解析标签选择器失败: %v", err)
	}

	var pods []*corev1.Pod

	if p.useInformer && p.podLister != nil {
		p.log.Debug("使用Informer获取Pods")
		pods, err = p.podLister.Pods(namespace).List(selector)
		if err != nil {
			p.log.Errorf("Informer获取Pods失败: %v", err)
			// 降级到 API 调用
			p.log.Debug("Informer失败，降级使用API")
			podList, apiErr := p.client.CoreV1().Pods(namespace).List(p.ctx, metav1.ListOptions{
				LabelSelector: labelSelector,
			})
			if apiErr != nil {
				return nil, fmt.Errorf("获取Pods失败: %v", apiErr)
			}
			pods = make([]*corev1.Pod, len(podList.Items))
			for i := range podList.Items {
				pods[i] = &podList.Items[i]
			}
		}
	} else {
		// 降级到 API 调用
		p.log.Debug("使用API获取Pods")
		podList, err := p.client.CoreV1().Pods(namespace).List(p.ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			p.log.Errorf("API获取Pods失败: %v", err)
			return nil, fmt.Errorf("获取Pods失败: %v", err)
		}
		pods = make([]*corev1.Pod, len(podList.Items))
		for i := range podList.Items {
			pods[i] = &podList.Items[i]
		}
	}

	// 转换为 PodDetailInfo
	result := make([]types.PodDetailInfo, 0, len(pods))
	for _, pod := range pods {
		result = append(result, p.convertToPodInfo(pod))
	}

	p.log.Debugf("成功获取并转换 %d 个Pods", len(result))
	return result, nil
}

// ========== 标签和选择器操作 ==========

// GetPodLabels 获取 Pod 标签
func (p *podOperator) GetPodLabels(namespace, name string) (map[string]string, error) {
	if namespace == "" || name == "" {
		return nil, fmt.Errorf("命名空间和名称不能为空")
	}

	p.log.Infof("获取Pod标签: namespace=%s, name=%s", namespace, name)

	pod, err := p.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	if pod.Labels == nil {
		return make(map[string]string), nil
	}

	return pod.Labels, nil
}

// GetPodSelectorLabels 获取 Pod 选择器标签
func (p *podOperator) GetPodSelectorLabels(namespace, name string) (map[string]string, error) {
	if namespace == "" || name == "" {
		return nil, fmt.Errorf("命名空间和名称不能为空")
	}

	p.log.Infof("获取Pod选择器标签: namespace=%s, name=%s", namespace, name)

	pod, err := p.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	// 如果 Pod 有 owner reference，获取其选择器标签
	if len(pod.OwnerReferences) > 0 {
		ownerRef := pod.OwnerReferences[0]

		switch ownerRef.Kind {
		case "ReplicaSet":
			rs, err := p.client.AppsV1().ReplicaSets(namespace).Get(p.ctx, ownerRef.Name, metav1.GetOptions{})
			if err == nil && rs.Spec.Selector != nil {
				return rs.Spec.Selector.MatchLabels, nil
			}
		case "StatefulSet":
			sts, err := p.client.AppsV1().StatefulSets(namespace).Get(p.ctx, ownerRef.Name, metav1.GetOptions{})
			if err == nil && sts.Spec.Selector != nil {
				return sts.Spec.Selector.MatchLabels, nil
			}
		case "DaemonSet":
			ds, err := p.client.AppsV1().DaemonSets(namespace).Get(p.ctx, ownerRef.Name, metav1.GetOptions{})
			if err == nil && ds.Spec.Selector != nil {
				return ds.Spec.Selector.MatchLabels, nil
			}
		case "Job":
			job, err := p.client.BatchV1().Jobs(namespace).Get(p.ctx, ownerRef.Name, metav1.GetOptions{})
			if err == nil && job.Spec.Selector != nil {
				return job.Spec.Selector.MatchLabels, nil
			}
		}
	}

	// 降级返回 Pod 自己的标签
	if pod.Labels == nil {
		return make(map[string]string), nil
	}
	return pod.Labels, nil
}

// GetVersionStatus 获取版本状态
func (p *podOperator) GetVersionStatus(namespace, name string) (*types.ResourceStatus, error) {
	if namespace == "" || name == "" {
		return nil, fmt.Errorf("命名空间和名称不能为空")
	}

	p.log.Infof("获取版本状态: namespace=%s, name=%s", namespace, name)

	pod, err := p.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	status := &types.ResourceStatus{
		Ready: false,
	}

	// 判断 Pod 状态
	switch pod.Status.Phase {
	case corev1.PodRunning:
		status.Status = types.StatusRunning
		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodReady {
				status.Ready = condition.Status == corev1.ConditionTrue
				break
			}
		}
		if status.Ready {
			status.Message = "Pod 正在运行且就绪"
		} else {
			status.Message = "Pod 正在运行但未就绪"
		}

	case corev1.PodPending:
		status.Status = types.StatusCreating
		status.Message = "Pod 等待调度或初始化中"

	case corev1.PodSucceeded:
		status.Status = types.StatusStopped
		status.Ready = true
		status.Message = "Pod 已成功完成"

	case corev1.PodFailed:
		status.Status = types.StatusError
		status.Message = "Pod 执行失败"

	default:
		status.Status = types.StatusError
		status.Message = fmt.Sprintf("未知状态: %s", pod.Status.Phase)
	}

	// 检查删除时间戳
	if pod.DeletionTimestamp != nil {
		status.Status = types.StatusStopping
		status.Message = "Pod 正在终止"
		status.Ready = false
	}

	return status, nil
}

// GetPods 获取 Pods（兼容方法）
func (p *podOperator) GetPods(namespace, name string) ([]types.PodDetailInfo, error) {
	p.log.Infof("获取Pods: namespace=%s, name=%s", namespace, name)

	// 如果 name 为空，返回命名空间下所有 Pod
	if name == "" {
		req := types.ListRequest{
			Page:     1,
			PageSize: 1000,
		}
		resp, err := p.List(namespace, req)
		if err != nil {
			return nil, err
		}
		return resp.Items, nil
	}

	// 否则返回单个 Pod
	pod, err := p.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	return []types.PodDetailInfo{p.convertToPodDetailInfo(pod)}, nil
}

// GetEvents 获取 Pod 事件
func (p *podOperator) GetEvents(namespace, name string) ([]types.EventInfo, error) {
	if namespace == "" || name == "" {
		return nil, fmt.Errorf("命名空间和名称不能为空")
	}

	p.log.Infof("获取Pod事件: namespace=%s, name=%s", namespace, name)

	pod, err := p.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	// 获取与该 Pod 相关的事件
	fieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.namespace=%s,involvedObject.kind=Pod,involvedObject.uid=%s",
		name, namespace, pod.UID)

	eventList, err := p.client.CoreV1().Events(namespace).List(p.ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		p.log.Errorf("获取事件列表失败: %v", err)
		return nil, fmt.Errorf("获取事件列表失败: %v", err)
	}

	events := make([]types.EventInfo, 0, len(eventList.Items))
	for _, event := range eventList.Items {
		eventInfo := types.EventInfo{
			Type:               event.Type,
			Reason:             event.Reason,
			Message:            event.Message,
			Count:              event.Count,
			Source:             fmt.Sprintf("%s/%s", event.Source.Component, event.Source.Host),
			InvolvedObjectKind: event.InvolvedObject.Kind,
			InvolvedObjectName: event.InvolvedObject.Name,
		}

		if !event.FirstTimestamp.IsZero() {
			eventInfo.FirstTimestamp = event.FirstTimestamp.UnixMilli()
		}
		if !event.LastTimestamp.IsZero() {
			eventInfo.LastTimestamp = event.LastTimestamp.UnixMilli()
		}

		events = append(events, eventInfo)
	}

	p.log.Infof("找到 %d 个事件", len(events))
	return events, nil
}

// ========== 辅助转换方法 ==========

// convertToPodInfo 将 corev1.Pod 转换为 types.PodDetailInfo（简化版）
func (p *podOperator) convertToPodInfo(pod *corev1.Pod) types.PodDetailInfo {
	info := types.PodDetailInfo{
		Name:         pod.Name,
		Namespace:    pod.Namespace,
		Status:       p.getPodStatus(pod),
		Node:         pod.Spec.NodeName,
		PodIP:        pod.Status.PodIP,
		Labels:       pod.Labels,
		CreationTime: pod.CreationTimestamp.UnixMilli(),
		Age:          formatAge(pod.CreationTimestamp.Time),
	}

	// 计算就绪状态
	readyCount := 0
	totalCount := len(pod.Status.ContainerStatuses)
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Ready {
			readyCount++
		}
	}
	info.Ready = fmt.Sprintf("%d/%d", readyCount, totalCount)

	// 计算重启次数
	restarts := int32(0)
	for _, cs := range pod.Status.ContainerStatuses {
		restarts += cs.RestartCount
	}
	info.Restarts = restarts

	return info
}

// convertToPodDetailInfo 将 corev1.Pod 转换为 types.PodDetailInfo（完整版）
func (p *podOperator) convertToPodDetailInfo(pod *corev1.Pod) types.PodDetailInfo {
	info := p.convertToPodInfo(pod)

	// 添加容器信息

	// 构建容器状态映射
	containerStatusMap := make(map[string]*types.ContainerStatusDetails)
	for i := range pod.Status.ContainerStatuses {
		cs := &pod.Status.ContainerStatuses[i]
		containerStatusMap[cs.Name] = &types.ContainerStatusDetails{
			State:        convertContainerState(cs.State),
			LastState:    convertContainerState(cs.LastTerminationState),
			Ready:        cs.Ready,
			RestartCount: cs.RestartCount,
			Image:        cs.Image,
			ImageID:      cs.ImageID,
			ContainerID:  cs.ContainerID,
			Started:      cs.Started,
		}
	}

	// Init 容器状态映射
	initContainerStatusMap := make(map[string]*types.ContainerStatusDetails)
	for i := range pod.Status.InitContainerStatuses {
		cs := &pod.Status.InitContainerStatuses[i]
		initContainerStatusMap[cs.Name] = &types.ContainerStatusDetails{
			State:        convertContainerState(cs.State),
			LastState:    convertContainerState(cs.LastTerminationState),
			Ready:        cs.Ready,
			RestartCount: cs.RestartCount,
			Image:        cs.Image,
			ImageID:      cs.ImageID,
			ContainerID:  cs.ContainerID,
			Started:      cs.Started,
		}
	}

	// 临时容器状态映射
	ephemeralContainerStatusMap := make(map[string]*types.ContainerStatusDetails)
	for i := range pod.Status.EphemeralContainerStatuses {
		cs := &pod.Status.EphemeralContainerStatuses[i]
		ephemeralContainerStatusMap[cs.Name] = &types.ContainerStatusDetails{
			State:        convertContainerState(cs.State),
			LastState:    convertContainerState(cs.LastTerminationState),
			Ready:        cs.Ready,
			RestartCount: cs.RestartCount,
			Image:        cs.Image,
			ImageID:      cs.ImageID,
			ContainerID:  cs.ContainerID,
			Started:      cs.Started,
		}
	}

	// 填充 Init 容器信息
	for _, c := range pod.Spec.InitContainers {
		containerInfo := types.ContainerInfo{
			Name:  c.Name,
			Image: c.Image,
		}
		if status, exists := initContainerStatusMap[c.Name]; exists {
			containerInfo.Ready = status.Ready
			containerInfo.RestartCount = status.RestartCount
			containerInfo.State = status.State.Type
			containerInfo.Status = status
		} else {
			containerInfo.State = "Waiting"
		}
	}

	// 填充普通容器信息
	for _, c := range pod.Spec.Containers {
		containerInfo := types.ContainerInfo{
			Name:  c.Name,
			Image: c.Image,
		}
		if status, exists := containerStatusMap[c.Name]; exists {
			containerInfo.Ready = status.Ready
			containerInfo.RestartCount = status.RestartCount
			containerInfo.State = status.State.Type
			containerInfo.Status = status
		} else {
			containerInfo.State = "Waiting"
		}
	}

	// 填充临时容器信息
	for _, c := range pod.Spec.EphemeralContainers {
		containerInfo := types.ContainerInfo{
			Name:  c.Name,
			Image: c.Image,
		}
		if status, exists := ephemeralContainerStatusMap[c.Name]; exists {
			containerInfo.Ready = status.Ready
			containerInfo.RestartCount = status.RestartCount
			containerInfo.State = status.State.Type
			containerInfo.Status = status
		} else {
			containerInfo.State = "Waiting"
		}
	}

	return info
}

// convertContainerState 转换容器状态
func convertContainerState(state corev1.ContainerState) types.ContainerStateInfo {
	stateInfo := types.ContainerStateInfo{}

	if state.Running != nil {
		stateInfo.Type = "Running"
		stateInfo.Running = &types.ContainerStateRunning{
			StartedAt: state.Running.StartedAt.UnixMilli(),
		}
	} else if state.Waiting != nil {
		stateInfo.Type = "Waiting"
		stateInfo.Waiting = &types.ContainerStateWaiting{
			Reason:  state.Waiting.Reason,
			Message: state.Waiting.Message,
		}
	} else if state.Terminated != nil {
		stateInfo.Type = "Terminated"
		stateInfo.Terminated = &types.ContainerStateTerminated{
			ExitCode:   state.Terminated.ExitCode,
			Reason:     state.Terminated.Reason,
			Message:    state.Terminated.Message,
			StartedAt:  state.Terminated.StartedAt.UnixMilli(),
			FinishedAt: state.Terminated.FinishedAt.UnixMilli(),
		}
	}

	return stateInfo
}

// ensureContainer 确保容器名称有效
func (p *podOperator) ensureContainer(namespace, podName, container string) (string, error) {
	// 如果已提供容器名称，直接返回
	if container != "" {
		return container, nil
	}

	// 获取默认容器
	p.log.Debugf("未指定容器，获取默认容器: %s/%s", namespace, podName)
	defaultContainer, err := p.GetDefaultContainer(namespace, podName)
	if err != nil {
		p.log.Errorf("获取默认容器失败: %v", err)
		return "", fmt.Errorf("未指定容器且无法获取默认容器: %v", err)
	}

	p.log.Debugf("使用默认容器: %s", defaultContainer)
	return defaultContainer, nil
}

// formatAge 格式化时间差
func formatAge(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return fmt.Sprintf("%ds", int(duration.Seconds()))
	} else if duration < time.Hour {
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	} else if duration < 24*time.Hour {
		return fmt.Sprintf("%dh", int(duration.Hours()))
	} else {
		return fmt.Sprintf("%dd", int(duration.Hours()/24))
	}
}

// isPodReady 检查 Pod 是否就绪
func isPodReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

// GetPodOwnerInfo 获取 Pod 的所有者信息
func (p *podOperator) GetPodOwnerInfo(namespace, name string) (map[string]interface{}, error) {
	pod, err := p.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	ownerInfo := make(map[string]interface{})

	if len(pod.OwnerReferences) == 0 {
		ownerInfo["type"] = "Standalone"
		ownerInfo["message"] = "独立 Pod，无控制器管理"
		return ownerInfo, nil
	}

	owner := pod.OwnerReferences[0]
	ownerInfo["kind"] = owner.Kind
	ownerInfo["name"] = owner.Name
	ownerInfo["uid"] = string(owner.UID)
	ownerInfo["controller"] = owner.Controller != nil && *owner.Controller

	return ownerInfo, nil
}

// GetPodResourceUsage 获取 Pod 资源使用情况（需要 metrics-server）
func (p *podOperator) GetPodResourceUsage(namespace, name string) (map[string]interface{}, error) {
	// 这个方法需要访问 metrics API
	// 返回占位信息
	return map[string]interface{}{
		"message": "需要 metrics-server 支持",
	}, nil
}

// IsPodHealthy 综合判断 Pod 是否健康
func (p *podOperator) IsPodHealthy(namespace, name string) (bool, string, error) {
	pod, err := p.Get(namespace, name)
	if err != nil {
		return false, "", err
	}

	// 检查是否正在删除
	if pod.DeletionTimestamp != nil {
		return false, "Pod 正在终止", nil
	}

	// 检查 Phase
	if pod.Status.Phase == corev1.PodFailed {
		return false, "Pod 已失败", nil
	}

	if pod.Status.Phase == corev1.PodPending {
		return false, "Pod 等待调度", nil
	}

	// 检查容器状态
	for _, cs := range pod.Status.ContainerStatuses {
		if !cs.Ready {
			if cs.State.Waiting != nil {
				return false, fmt.Sprintf("容器 %s 等待中: %s", cs.Name, cs.State.Waiting.Reason), nil
			}
			if cs.State.Terminated != nil {
				return false, fmt.Sprintf("容器 %s 已终止: %s", cs.Name, cs.State.Terminated.Reason), nil
			}
			return false, fmt.Sprintf("容器 %s 未就绪", cs.Name), nil
		}

		// 检查重启次数
		if cs.RestartCount > 5 {
			return false, fmt.Sprintf("容器 %s 重启次数过多: %d", cs.Name, cs.RestartCount), nil
		}
	}

	// 检查就绪条件
	if !isPodReady(pod) {
		return false, "Pod 未就绪", nil
	}

	return true, "Pod 健康", nil
}

// GetPodConditions 获取 Pod 所有条件
func (p *podOperator) GetPodConditions(namespace, name string) ([]map[string]interface{}, error) {
	pod, err := p.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	conditions := make([]map[string]interface{}, 0, len(pod.Status.Conditions))
	for _, condition := range pod.Status.Conditions {
		conditionMap := map[string]interface{}{
			"type":               string(condition.Type),
			"status":             string(condition.Status),
			"reason":             condition.Reason,
			"message":            condition.Message,
			"lastTransitionTime": condition.LastTransitionTime.UnixMilli(),
			"lastProbeTime":      condition.LastProbeTime.UnixMilli(),
		}
		conditions = append(conditions, conditionMap)
	}

	return conditions, nil
}

// GetPodVolumes 获取 Pod 的卷信息
func (p *podOperator) GetPodVolumes(namespace, name string) ([]map[string]interface{}, error) {
	pod, err := p.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	volumes := make([]map[string]interface{}, 0, len(pod.Spec.Volumes))
	for _, vol := range pod.Spec.Volumes {
		volumeInfo := map[string]interface{}{
			"name": vol.Name,
		}

		// 确定卷类型
		if vol.HostPath != nil {
			volumeInfo["type"] = "HostPath"
			volumeInfo["path"] = vol.HostPath.Path
		} else if vol.EmptyDir != nil {
			volumeInfo["type"] = "EmptyDir"
		} else if vol.ConfigMap != nil {
			volumeInfo["type"] = "ConfigMap"
			volumeInfo["configMapName"] = vol.ConfigMap.Name
		} else if vol.Secret != nil {
			volumeInfo["type"] = "Secret"
			volumeInfo["secretName"] = vol.Secret.SecretName
		} else if vol.PersistentVolumeClaim != nil {
			volumeInfo["type"] = "PersistentVolumeClaim"
			volumeInfo["claimName"] = vol.PersistentVolumeClaim.ClaimName
		} else {
			volumeInfo["type"] = "Other"
		}

		volumes = append(volumes, volumeInfo)
	}

	return volumes, nil
}

// ValidatePodSpec 验证 Pod 规范
func (p *podOperator) ValidatePodSpec(pod *corev1.Pod) []string {
	var issues []string

	if pod == nil {
		return []string{"Pod 对象为空"}
	}

	// 检查名称
	if pod.Name == "" {
		issues = append(issues, "Pod 名称不能为空")
	}
	if pod.Namespace == "" {
		issues = append(issues, "命名空间不能为空")
	}

	// 检查容器
	if len(pod.Spec.Containers) == 0 {
		issues = append(issues, "至少需要一个容器")
	}

	for i, container := range pod.Spec.Containers {
		if container.Name == "" {
			issues = append(issues, fmt.Sprintf("容器 #%d 名称不能为空", i))
		}
		if container.Image == "" {
			issues = append(issues, fmt.Sprintf("容器 %s 镜像不能为空", container.Name))
		}
	}

	return issues
}

// GetPodQoSClass 获取 Pod 的 QoS 类别
func (p *podOperator) GetPodQoSClass(namespace, name string) (string, error) {
	pod, err := p.Get(namespace, name)
	if err != nil {
		return "", err
	}

	return string(pod.Status.QOSClass), nil
}

// GetPodNodeSelector 获取 Pod 的节点选择器
func (p *podOperator) GetPodNodeSelector(namespace, name string) (map[string]string, error) {
	pod, err := p.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	if pod.Spec.NodeSelector == nil {
		return make(map[string]string), nil
	}

	return pod.Spec.NodeSelector, nil
}

// GetPodTolerations 获取 Pod 的容忍度
func (p *podOperator) GetPodTolerations(namespace, name string) ([]map[string]interface{}, error) {
	pod, err := p.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	tolerations := make([]map[string]interface{}, 0, len(pod.Spec.Tolerations))
	for _, tol := range pod.Spec.Tolerations {
		tolInfo := map[string]interface{}{
			"key":      tol.Key,
			"operator": string(tol.Operator),
			"value":    tol.Value,
			"effect":   string(tol.Effect),
		}
		if tol.TolerationSeconds != nil {
			tolInfo["tolerationSeconds"] = *tol.TolerationSeconds
		}
		tolerations = append(tolerations, tolInfo)
	}

	return tolerations, nil
}

// GetContainerByName 根据名称获取容器信息
func (p *podOperator) GetContainerByName(namespace, podName, containerName string) (*corev1.Container, error) {
	pod, err := p.Get(namespace, podName)
	if err != nil {
		return nil, err
	}

	// 查找普通容器
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == containerName {
			return &pod.Spec.Containers[i], nil
		}
	}

	// 查找 Init 容器
	for i := range pod.Spec.InitContainers {
		if pod.Spec.InitContainers[i].Name == containerName {
			return &pod.Spec.InitContainers[i], nil
		}
	}

	return nil, fmt.Errorf("容器 %s 不存在", containerName)
}

// FormatPodStatus 格式化 Pod 状态为可读字符串（导出函数）
func FormatPodStatus(pod *corev1.Pod) string {
	return getPodStatusCore(pod)
}

// GetPodRestartPolicy 获取 Pod 重启策略
func (p *podOperator) GetPodRestartPolicy(namespace, name string) (string, error) {
	pod, err := p.Get(namespace, name)
	if err != nil {
		return "", err
	}

	return string(pod.Spec.RestartPolicy), nil
}

// getPodStatusCore 核心状态判断逻辑（包级别静态函数）
func getPodStatusCore(pod *corev1.Pod) string {
	// 1. 最高优先级：删除状态
	if pod.DeletionTimestamp != nil {
		return "Terminating"
	}

	// 2. 驱逐状态
	if pod.Status.Reason == "Evicted" {
		return "Evicted"
	}

	// 3. 检查调度状态
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodScheduled && condition.Status == corev1.ConditionFalse {
			if condition.Reason == "Unschedulable" {
				return "Unschedulable"
			}
		}
	}

	// 4. 根据 Pod Phase 进行细分判断
	reason := string(pod.Status.Phase)
	if pod.Status.Reason != "" {
		reason = pod.Status.Reason
	}

	// 5. 检查初始化容器状态
	initializing := false
	for i, container := range pod.Status.InitContainerStatuses {
		switch {
		case container.State.Terminated != nil && container.State.Terminated.ExitCode == 0:
			continue
		case container.State.Terminated != nil:
			if len(container.State.Terminated.Reason) == 0 {
				if container.State.Terminated.Signal != 0 {
					reason = fmt.Sprintf("Init:Signal:%d", container.State.Terminated.Signal)
				} else {
					reason = fmt.Sprintf("Init:ExitCode:%d", container.State.Terminated.ExitCode)
				}
			} else {
				reason = "Init:" + container.State.Terminated.Reason
			}
			initializing = true
		case container.State.Waiting != nil && len(container.State.Waiting.Reason) > 0 && container.State.Waiting.Reason != "PodInitializing":
			reason = "Init:" + container.State.Waiting.Reason
			initializing = true
		default:
			reason = fmt.Sprintf("Init:%d/%d", i, len(pod.Spec.InitContainers))
			initializing = true
		}
		break
	}

	// 6.  检查普通容器状态 - 优先显示错误状态
	if !initializing {
		hasRunning := false
		hasWaiting := false
		hasTerminated := false
		var waitingReason string
		var terminatedReason string

		//  第一遍遍历：收集所有容器状态，优先级：OOMKilled > CrashLoopBackOff > 其他错误
		for _, container := range pod.Status.ContainerStatuses {
			//  检查 OOMKilled（最高优先级）
			if container.State.Terminated != nil && container.State.Terminated.Reason == "OOMKilled" {
				return "OOMKilled"
			}
			if container.State.Waiting != nil && container.State.Waiting.Reason == "CrashLoopBackOff" {
				// 检查 LastTerminationState 是否是 OOMKilled
				if container.LastTerminationState.Terminated != nil &&
					container.LastTerminationState.Terminated.Reason == "OOMKilled" {
					return "OOMKilled"
				}
			}

			//  检查 Waiting 状态
			if container.State.Waiting != nil && container.State.Waiting.Reason != "" {
				hasWaiting = true
				if waitingReason == "" ||
					container.State.Waiting.Reason == "CrashLoopBackOff" ||
					container.State.Waiting.Reason == "ImagePullBackOff" ||
					container.State.Waiting.Reason == "ErrImagePull" {
					waitingReason = container.State.Waiting.Reason
				}
			}

			//  检查 Terminated 状态
			if container.State.Terminated != nil {
				hasTerminated = true
				if terminatedReason == "" {
					if container.State.Terminated.Reason != "" {
						terminatedReason = container.State.Terminated.Reason
					} else if container.State.Terminated.Signal != 0 {
						terminatedReason = fmt.Sprintf("Signal:%d", container.State.Terminated.Signal)
					} else {
						terminatedReason = fmt.Sprintf("ExitCode:%d", container.State.Terminated.ExitCode)
					}
				}
			}

			//  检查 Running 状态
			if container.State.Running != nil {
				hasRunning = true
			}
		}

		//  第二步：按优先级决定状态
		// 优先级：Waiting > Terminated > Running
		if hasWaiting {
			reason = waitingReason
		} else if hasTerminated {
			reason = terminatedReason
		} else if hasRunning {
			// 所有容器都在运行
			if reason == "Completed" {
				if hasPodReadyConditionStatic(pod.Status.Conditions) {
					reason = "Running"
				} else {
					reason = "NotReady"
				}
			} else if pod.Status.Phase == corev1.PodRunning {
				//  额外检查：即使所有容器都在 Running，也要检查 Ready 状态
				if !hasPodReadyConditionStatic(pod.Status.Conditions) {
					reason = "NotReady"
				} else {
					reason = "Running"
				}
			}
		}
	}

	return normalizeStatusStatic(reason)
}

// hasPodReadyConditionStatic 包级别辅助函数
func hasPodReadyConditionStatic(conditions []corev1.PodCondition) bool {
	for _, condition := range conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// normalizeStatusStatic 包级别状态标准化函数
func normalizeStatusStatic(status string) string {
	statusMap := map[string]string{
		"ContainerCreating":          "ContainerCreating",
		"PodInitializing":            "PodInitializing",
		"CreateContainerConfigError": "CreateContainerConfigError",
		"CreateContainerError":       "CreateContainerError",
		"ErrImagePull":               "ErrImagePull",
		"ImagePullBackOff":           "ImagePullBackOff",
		"InvalidImageName":           "InvalidImageName",
		"CrashLoopBackOff":           "CrashLoopBackOff",
		"RunContainerError":          "RunContainerError",
		"KillContainerError":         "KillContainerError",
		"FailedMount":                "FailedMount",
		"FailedAttachVolume":         "FailedAttachVolume",
		"FailedCreatePodSandBox":     "FailedCreatePodSandBox",
		"NetworkPluginNotReady":      "NetworkPluginNotReady",
		"Unschedulable":              "Unschedulable",
		"Completed":                  "Completed",
		"Succeeded":                  "Succeeded",
		"Error":                      "Error",
		"Failed":                     "Failed",
		"Evicted":                    "Evicted",
		"Unknown":                    "Unknown",
		"OOMKilled":                  "OOMKilled",
		"PostStartHookError":         "PostStartHookError",
		"NotReady":                   "NotReady",
	}

	if mapped, ok := statusMap[status]; ok {
		return mapped
	}

	if strings.HasPrefix(status, "Init:") {
		return status
	}

	if strings.HasPrefix(status, "Signal:") || strings.HasPrefix(status, "ExitCode:") {
		return "Error"
	}

	return status
}

// GetPodServiceAccountName 获取 Pod 的 ServiceAccount 名称
func (p *podOperator) GetPodServiceAccountName(namespace, name string) (string, error) {
	pod, err := p.Get(namespace, name)
	if err != nil {
		return "", err
	}

	return pod.Spec.ServiceAccountName, nil
}

// GetPodSecurityContext 获取 Pod 安全上下文
func (p *podOperator) GetPodSecurityContext(namespace, name string) (map[string]interface{}, error) {
	pod, err := p.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	securityContext := make(map[string]interface{})

	if pod.Spec.SecurityContext != nil {
		sc := pod.Spec.SecurityContext

		if sc.RunAsUser != nil {
			securityContext["runAsUser"] = *sc.RunAsUser
		}
		if sc.RunAsGroup != nil {
			securityContext["runAsGroup"] = *sc.RunAsGroup
		}
		if sc.FSGroup != nil {
			securityContext["fsGroup"] = *sc.FSGroup
		}
		if sc.RunAsNonRoot != nil {
			securityContext["runAsNonRoot"] = *sc.RunAsNonRoot
		}

		securityContext["seLinuxOptions"] = sc.SELinuxOptions
		securityContext["supplementalGroups"] = sc.SupplementalGroups
	}

	return securityContext, nil
}
func (p *podOperator) GetDescribe(namespace, name string) (string, error) {
	pod, err := p.Get(namespace, name)
	if err != nil {
		return "", err
	}

	var buf strings.Builder

	// ========== 基本信息 ==========
	buf.WriteString(fmt.Sprintf("Name:         %s\n", pod.Name))
	buf.WriteString(fmt.Sprintf("Namespace:    %s\n", pod.Namespace))
	buf.WriteString(fmt.Sprintf("Priority:     %d\n", func() int32 {
		if pod.Spec.Priority != nil {
			return *pod.Spec.Priority
		}
		return 0
	}()))

	if pod.Spec.PriorityClassName != "" {
		buf.WriteString(fmt.Sprintf("Priority Class Name:  %s\n", pod.Spec.PriorityClassName))
	}

	buf.WriteString(fmt.Sprintf("Service Account:  %s\n", pod.Spec.ServiceAccountName))

	if pod.Spec.NodeName != "" {
		buf.WriteString(fmt.Sprintf("Node:         %s\n", pod.Spec.NodeName))
	} else {
		buf.WriteString("Node:         <none>\n")
	}

	if pod.Status.NominatedNodeName != "" {
		buf.WriteString(fmt.Sprintf("Nominated Node:   %s\n", pod.Status.NominatedNodeName))
	}

	if pod.Status.StartTime != nil {
		buf.WriteString(fmt.Sprintf("Start Time:   %s\n", pod.Status.StartTime.Format(time.RFC1123)))
	} else {
		buf.WriteString("Start Time:   <none>\n")
	}

	// Labels
	buf.WriteString("Labels:       ")
	if len(pod.Labels) == 0 {
		buf.WriteString("<none>\n")
	} else {
		first := true
		for k, v := range pod.Labels {
			if !first {
				buf.WriteString("              ")
			}
			buf.WriteString(fmt.Sprintf("%s=%s\n", k, v))
			first = false
		}
	}

	// Annotations
	buf.WriteString("Annotations:  ")
	if len(pod.Annotations) == 0 {
		buf.WriteString("<none>\n")
	} else {
		first := true
		for k, v := range pod.Annotations {
			if !first {
				buf.WriteString("              ")
			}
			buf.WriteString(fmt.Sprintf("%s: %s\n", k, v))
			first = false
		}
	}

	// Status
	buf.WriteString(fmt.Sprintf("Status:       %s\n", pod.Status.Phase))
	if pod.Status.Reason != "" {
		buf.WriteString(fmt.Sprintf("Reason:       %s\n", pod.Status.Reason))
	}
	if pod.Status.Message != "" {
		buf.WriteString(fmt.Sprintf("Message:      %s\n", pod.Status.Message))
	}

	if pod.Status.PodIP != "" {
		buf.WriteString(fmt.Sprintf("IP:           %s\n", pod.Status.PodIP))
	} else {
		buf.WriteString("IP:           \n")
	}

	buf.WriteString("IPs:          ")
	if len(pod.Status.PodIPs) > 0 {
		buf.WriteString("\n")
		for _, ip := range pod.Status.PodIPs {
			buf.WriteString(fmt.Sprintf("  IP:           %s\n", ip.IP))
		}
	} else {
		buf.WriteString("<none>\n")
	}

	// Controlled By
	if len(pod.OwnerReferences) > 0 {
		owner := pod.OwnerReferences[0]
		buf.WriteString(fmt.Sprintf("Controlled By:  %s/%s\n", owner.Kind, owner.Name))
	}

	// ========== Init Containers ==========
	if len(pod.Spec.InitContainers) > 0 {
		buf.WriteString("Init Containers:\n")
		for i, container := range pod.Spec.InitContainers {
			buf.WriteString(fmt.Sprintf("  %s:\n", container.Name))

			if i < len(pod.Status.InitContainerStatuses) {
				buf.WriteString(fmt.Sprintf("    Container ID:   %s\n", pod.Status.InitContainerStatuses[i].ContainerID))
			} else {
				buf.WriteString("    Container ID:   \n")
			}

			buf.WriteString(fmt.Sprintf("    Image:          %s\n", container.Image))

			if i < len(pod.Status.InitContainerStatuses) {
				buf.WriteString(fmt.Sprintf("    Image ID:       %s\n", pod.Status.InitContainerStatuses[i].ImageID))
			} else {
				buf.WriteString("    Image ID:       \n")
			}

			if len(container.Ports) > 0 {
				buf.WriteString("    Port:       ")
				for j, port := range container.Ports {
					if j > 0 {
						buf.WriteString(", ")
					}
					buf.WriteString(fmt.Sprintf("%d/%s", port.ContainerPort, port.Protocol))
				}
				buf.WriteString("\n")

				// Host Port
				buf.WriteString("    Host Port:  ")
				for j, port := range container.Ports {
					if j > 0 {
						buf.WriteString(", ")
					}
					if port.HostPort > 0 {
						buf.WriteString(fmt.Sprintf("%d/%s", port.HostPort, port.Protocol))
					} else {
						buf.WriteString("0/" + string(port.Protocol))
					}
				}
				buf.WriteString("\n")
			} else {
				buf.WriteString("    Port:       <none>\n")
				buf.WriteString("    Host Port:  <none>\n")
			}

			if len(container.Command) > 0 {
				buf.WriteString("    Command:\n")
				for _, cmd := range container.Command {
					buf.WriteString(fmt.Sprintf("      %s\n", cmd))
				}
			}

			if i < len(pod.Status.InitContainerStatuses) {
				cs := pod.Status.InitContainerStatuses[i]

				if cs.State.Running != nil {
					buf.WriteString("    State:          Running\n")
					if !cs.State.Running.StartedAt.IsZero() {
						buf.WriteString(fmt.Sprintf("      Started:      %s\n", cs.State.Running.StartedAt.Format(time.RFC1123)))
					}
				} else if cs.State.Waiting != nil {
					buf.WriteString("    State:          Waiting\n")
					buf.WriteString(fmt.Sprintf("      Reason:       %s\n", cs.State.Waiting.Reason))
					if cs.State.Waiting.Message != "" {
						buf.WriteString(fmt.Sprintf("      Message:      %s\n", cs.State.Waiting.Message))
					}
				} else if cs.State.Terminated != nil {
					buf.WriteString("    State:          Terminated\n")
					buf.WriteString(fmt.Sprintf("      Reason:       %s\n", cs.State.Terminated.Reason))
					buf.WriteString(fmt.Sprintf("      Exit Code:    %d\n", cs.State.Terminated.ExitCode))
					if !cs.State.Terminated.StartedAt.IsZero() {
						buf.WriteString(fmt.Sprintf("      Started:      %s\n", cs.State.Terminated.StartedAt.Format(time.RFC1123)))
					}
					if !cs.State.Terminated.FinishedAt.IsZero() {
						buf.WriteString(fmt.Sprintf("      Finished:     %s\n", cs.State.Terminated.FinishedAt.Format(time.RFC1123)))
					}
				}

				buf.WriteString(fmt.Sprintf("    Ready:          %v\n", cs.Ready))
				buf.WriteString(fmt.Sprintf("    Restart Count:  %d\n", cs.RestartCount))
			}

			if len(container.Resources.Limits) > 0 {
				buf.WriteString("    Limits:\n")
				if cpu := container.Resources.Limits.Cpu(); cpu != nil && !cpu.IsZero() {
					buf.WriteString(fmt.Sprintf("      cpu:                %s\n", cpu.String()))
				}
				if mem := container.Resources.Limits.Memory(); mem != nil && !mem.IsZero() {
					buf.WriteString(fmt.Sprintf("      memory:             %s\n", mem.String()))
				}
				if storage := container.Resources.Limits.StorageEphemeral(); storage != nil && !storage.IsZero() {
					buf.WriteString(fmt.Sprintf("      ephemeral-storage:  %s\n", storage.String()))
				}
			}

			if len(container.Resources.Requests) > 0 {
				buf.WriteString("    Requests:\n")
				if cpu := container.Resources.Requests.Cpu(); cpu != nil && !cpu.IsZero() {
					buf.WriteString(fmt.Sprintf("      cpu:                %s\n", cpu.String()))
				}
				if mem := container.Resources.Requests.Memory(); mem != nil && !mem.IsZero() {
					buf.WriteString(fmt.Sprintf("      memory:             %s\n", mem.String()))
				}
				if storage := container.Resources.Requests.StorageEphemeral(); storage != nil && !storage.IsZero() {
					buf.WriteString(fmt.Sprintf("      ephemeral-storage:  %s\n", storage.String()))
				}
			}

			// Environment
			if len(container.Env) > 0 {
				buf.WriteString("    Environment:\n")
				for _, env := range container.Env {
					if env.ValueFrom != nil {
						if env.ValueFrom.FieldRef != nil {
							buf.WriteString(fmt.Sprintf("      %s:   (%s)\n",
								env.Name, env.ValueFrom.FieldRef.FieldPath))
						} else if env.ValueFrom.SecretKeyRef != nil {
							buf.WriteString(fmt.Sprintf("      %s:  <set to the key '%s' in secret '%s'>  Optional: %v\n",
								env.Name, env.ValueFrom.SecretKeyRef.Key,
								env.ValueFrom.SecretKeyRef.Name,
								env.ValueFrom.SecretKeyRef.Optional != nil && *env.ValueFrom.SecretKeyRef.Optional))
						} else if env.ValueFrom.ConfigMapKeyRef != nil {
							buf.WriteString(fmt.Sprintf("      %s:  <set to the key '%s' in config map '%s'>  Optional: %v\n",
								env.Name, env.ValueFrom.ConfigMapKeyRef.Key,
								env.ValueFrom.ConfigMapKeyRef.Name,
								env.ValueFrom.ConfigMapKeyRef.Optional != nil && *env.ValueFrom.ConfigMapKeyRef.Optional))
						} else {
							buf.WriteString(fmt.Sprintf("      %s:  <set from source>\n", env.Name))
						}
					} else {
						buf.WriteString(fmt.Sprintf("      %s:  %s\n", env.Name, env.Value))
					}
				}
			}

			// Mounts
			if len(container.VolumeMounts) > 0 {
				buf.WriteString("    Mounts:\n")
				for _, mount := range container.VolumeMounts {
					buf.WriteString(fmt.Sprintf("      %s from %s (%s)\n", mount.MountPath, mount.Name, func() string {
						if mount.ReadOnly {
							return "ro"
						}
						return "rw"
					}()))
				}
			}
		}
	}

	// ========== Containers ==========
	buf.WriteString("Containers:\n")
	for i, container := range pod.Spec.Containers {
		buf.WriteString(fmt.Sprintf("  %s:\n", container.Name))

		if i < len(pod.Status.ContainerStatuses) {
			buf.WriteString(fmt.Sprintf("    Container ID:  %s\n", pod.Status.ContainerStatuses[i].ContainerID))
		} else {
			buf.WriteString("    Container ID:  \n")
		}

		buf.WriteString(fmt.Sprintf("    Image:         %s\n", container.Image))

		if i < len(pod.Status.ContainerStatuses) {
			buf.WriteString(fmt.Sprintf("    Image ID:      %s\n", pod.Status.ContainerStatuses[i].ImageID))
		} else {
			buf.WriteString("    Image ID:      \n")
		}

		if len(container.Ports) > 0 {
			buf.WriteString("    Port:          ")
			for j, port := range container.Ports {
				if j > 0 {
					buf.WriteString(", ")
				}
				buf.WriteString(fmt.Sprintf("%d/%s", port.ContainerPort, port.Protocol))
			}
			buf.WriteString("\n")

			// Host Port
			buf.WriteString("    Host Port:     ")
			for j, port := range container.Ports {
				if j > 0 {
					buf.WriteString(", ")
				}
				if port.HostPort > 0 {
					buf.WriteString(fmt.Sprintf("%d/%s", port.HostPort, port.Protocol))
				} else {
					buf.WriteString("0/" + string(port.Protocol))
				}
			}
			buf.WriteString("\n")
		} else {
			buf.WriteString("    Port:          <none>\n")
			buf.WriteString("    Host Port:     <none>\n")
		}

		if len(container.Command) > 0 {
			buf.WriteString("    Command:\n")
			for _, cmd := range container.Command {
				buf.WriteString(fmt.Sprintf("      %s\n", cmd))
			}
		}

		if i < len(pod.Status.ContainerStatuses) {
			cs := pod.Status.ContainerStatuses[i]

			if cs.State.Running != nil {
				buf.WriteString("    State:          Running\n")
				if !cs.State.Running.StartedAt.IsZero() {
					buf.WriteString(fmt.Sprintf("      Started:      %s\n", cs.State.Running.StartedAt.Format(time.RFC1123)))
				}
			} else if cs.State.Waiting != nil {
				buf.WriteString("    State:          Waiting\n")
				buf.WriteString(fmt.Sprintf("      Reason:       %s\n", cs.State.Waiting.Reason))
				if cs.State.Waiting.Message != "" {
					buf.WriteString(fmt.Sprintf("      Message:      %s\n", cs.State.Waiting.Message))
				}
			} else if cs.State.Terminated != nil {
				buf.WriteString("    State:          Terminated\n")
				buf.WriteString(fmt.Sprintf("      Reason:       %s\n", cs.State.Terminated.Reason))
				buf.WriteString(fmt.Sprintf("      Exit Code:    %d\n", cs.State.Terminated.ExitCode))
				if !cs.State.Terminated.StartedAt.IsZero() {
					buf.WriteString(fmt.Sprintf("      Started:      %s\n", cs.State.Terminated.StartedAt.Format(time.RFC1123)))
				}
				if !cs.State.Terminated.FinishedAt.IsZero() {
					buf.WriteString(fmt.Sprintf("      Finished:     %s\n", cs.State.Terminated.FinishedAt.Format(time.RFC1123)))
				}
			}

			if cs.LastTerminationState.Terminated != nil {
				buf.WriteString("    Last State:     Terminated\n")
				buf.WriteString(fmt.Sprintf("      Reason:       %s\n", cs.LastTerminationState.Terminated.Reason))
				buf.WriteString(fmt.Sprintf("      Exit Code:    %d\n", cs.LastTerminationState.Terminated.ExitCode))
				if !cs.LastTerminationState.Terminated.StartedAt.IsZero() {
					buf.WriteString(fmt.Sprintf("      Started:      %s\n", cs.LastTerminationState.Terminated.StartedAt.Format(time.RFC1123)))
				}
				if !cs.LastTerminationState.Terminated.FinishedAt.IsZero() {
					buf.WriteString(fmt.Sprintf("      Finished:     %s\n", cs.LastTerminationState.Terminated.FinishedAt.Format(time.RFC1123)))
				}
			}

			buf.WriteString(fmt.Sprintf("    Ready:          %v\n", cs.Ready))
			buf.WriteString(fmt.Sprintf("    Restart Count:  %d\n", cs.RestartCount))
		}

		if len(container.Resources.Limits) > 0 {
			buf.WriteString("    Limits:\n")
			if cpu := container.Resources.Limits.Cpu(); cpu != nil && !cpu.IsZero() {
				buf.WriteString(fmt.Sprintf("      cpu:                %s\n", cpu.String()))
			}
			if mem := container.Resources.Limits.Memory(); mem != nil && !mem.IsZero() {
				buf.WriteString(fmt.Sprintf("      memory:             %s\n", mem.String()))
			}
			if storage := container.Resources.Limits.StorageEphemeral(); storage != nil && !storage.IsZero() {
				buf.WriteString(fmt.Sprintf("      ephemeral-storage:  %s\n", storage.String()))
			}
		}

		if len(container.Resources.Requests) > 0 {
			buf.WriteString("    Requests:\n")
			if cpu := container.Resources.Requests.Cpu(); cpu != nil && !cpu.IsZero() {
				buf.WriteString(fmt.Sprintf("      cpu:                %s\n", cpu.String()))
			}
			if mem := container.Resources.Requests.Memory(); mem != nil && !mem.IsZero() {
				buf.WriteString(fmt.Sprintf("      memory:             %s\n", mem.String()))
			}
			if storage := container.Resources.Requests.StorageEphemeral(); storage != nil && !storage.IsZero() {
				buf.WriteString(fmt.Sprintf("      ephemeral-storage:  %s\n", storage.String()))
			}
		}

		if container.LivenessProbe != nil {
			buf.WriteString(fmt.Sprintf("    Liveness:       %s\n", p.formatProbeForDescribe(container.LivenessProbe)))
		}
		if container.ReadinessProbe != nil {
			buf.WriteString(fmt.Sprintf("    Readiness:      %s\n", p.formatProbeForDescribe(container.ReadinessProbe)))
		}
		if container.StartupProbe != nil {
			buf.WriteString(fmt.Sprintf("    Startup:        %s\n", p.formatProbeForDescribe(container.StartupProbe)))
		}

		// Environment
		if len(container.Env) > 0 {
			buf.WriteString("    Environment:\n")
			for _, env := range container.Env {
				if env.ValueFrom != nil {
					if env.ValueFrom.FieldRef != nil {
						buf.WriteString(fmt.Sprintf("      %s:   (%s)\n",
							env.Name, env.ValueFrom.FieldRef.FieldPath))
					} else if env.ValueFrom.SecretKeyRef != nil {
						buf.WriteString(fmt.Sprintf("      %s:  <set to the key '%s' in secret '%s'>  Optional: %v\n",
							env.Name, env.ValueFrom.SecretKeyRef.Key,
							env.ValueFrom.SecretKeyRef.Name,
							env.ValueFrom.SecretKeyRef.Optional != nil && *env.ValueFrom.SecretKeyRef.Optional))
					} else if env.ValueFrom.ConfigMapKeyRef != nil {
						buf.WriteString(fmt.Sprintf("      %s:  <set to the key '%s' in config map '%s'>  Optional: %v\n",
							env.Name, env.ValueFrom.ConfigMapKeyRef.Key,
							env.ValueFrom.ConfigMapKeyRef.Name,
							env.ValueFrom.ConfigMapKeyRef.Optional != nil && *env.ValueFrom.ConfigMapKeyRef.Optional))
					} else if env.ValueFrom.ResourceFieldRef != nil {
						buf.WriteString(fmt.Sprintf("      %s:  <set from resource>\n", env.Name))
					} else {
						buf.WriteString(fmt.Sprintf("      %s:  <set from source>\n", env.Name))
					}
				} else {
					buf.WriteString(fmt.Sprintf("      %s:  %s\n", env.Name, env.Value))
				}
			}
		}

		// Mounts
		if len(container.VolumeMounts) > 0 {
			buf.WriteString("    Mounts:\n")
			for _, mount := range container.VolumeMounts {
				buf.WriteString(fmt.Sprintf("      %s from %s (%s)\n", mount.MountPath, mount.Name, func() string {
					if mount.ReadOnly {
						return "ro"
					}
					return "rw"
				}()))
			}
		}
	}

	// ========== Conditions ==========
	buf.WriteString("Conditions:\n")
	if len(pod.Status.Conditions) > 0 {
		buf.WriteString("  Type           Status\n")
		for _, cond := range pod.Status.Conditions {
			buf.WriteString(fmt.Sprintf("  %-14s %s\n", cond.Type, cond.Status))
		}
	} else {
		buf.WriteString("  <none>\n")
	}

	// ========== Volumes ==========
	buf.WriteString("Volumes:\n")
	if len(pod.Spec.Volumes) > 0 {
		for _, vol := range pod.Spec.Volumes {
			buf.WriteString(fmt.Sprintf("  %s:\n", vol.Name))

			if vol.ConfigMap != nil {
				buf.WriteString("    Type:       ConfigMap (a volume populated by a ConfigMap)\n")
				buf.WriteString(fmt.Sprintf("    Name:       %s\n", vol.ConfigMap.Name))
				if vol.ConfigMap.Optional != nil {
					buf.WriteString(fmt.Sprintf("    Optional:   %v\n", *vol.ConfigMap.Optional))
				}
			} else if vol.Secret != nil {
				buf.WriteString("    Type:       Secret (a volume populated by a Secret)\n")
				buf.WriteString(fmt.Sprintf("    SecretName: %s\n", vol.Secret.SecretName))
				if vol.Secret.Optional != nil {
					buf.WriteString(fmt.Sprintf("    Optional:   %v\n", *vol.Secret.Optional))
				}
			} else if vol.PersistentVolumeClaim != nil {
				buf.WriteString("    Type:       PersistentVolumeClaim (a reference to a PersistentVolumeClaim in the same namespace)\n")
				buf.WriteString(fmt.Sprintf("    ClaimName:  %s\n", vol.PersistentVolumeClaim.ClaimName))
				buf.WriteString(fmt.Sprintf("    ReadOnly:   %v\n", vol.PersistentVolumeClaim.ReadOnly))
			} else if vol.EmptyDir != nil {
				buf.WriteString("    Type:       EmptyDir (a temporary directory that shares a pod's lifetime)\n")
				if vol.EmptyDir.Medium != "" {
					buf.WriteString(fmt.Sprintf("    Medium:     %s\n", vol.EmptyDir.Medium))
				} else {
					buf.WriteString("    Medium:\n")
				}
				if vol.EmptyDir.SizeLimit != nil && !vol.EmptyDir.SizeLimit.IsZero() {
					buf.WriteString(fmt.Sprintf("    SizeLimit:  %s\n", vol.EmptyDir.SizeLimit.String()))
				}
			} else if vol.HostPath != nil {
				buf.WriteString("    Type:       HostPath (bare host directory volume)\n")
				buf.WriteString(fmt.Sprintf("    Path:       %s\n", vol.HostPath.Path))
				if vol.HostPath.Type != nil {
					buf.WriteString(fmt.Sprintf("    HostPathType: %s\n", *vol.HostPath.Type))
				}
			} else if vol.Projected != nil {
				buf.WriteString("    Type:                    Projected (a volume that contains injected data from multiple sources)\n")
				if len(vol.Projected.Sources) > 0 {
					for _, source := range vol.Projected.Sources {
						if source.ServiceAccountToken != nil {
							buf.WriteString(fmt.Sprintf("    TokenExpirationSeconds:  %d\n", source.ServiceAccountToken.ExpirationSeconds))
						}
						if source.ConfigMap != nil {
							buf.WriteString(fmt.Sprintf("    ConfigMapName:           %s\n", source.ConfigMap.Name))
							if source.ConfigMap.Optional != nil {
								buf.WriteString(fmt.Sprintf("    ConfigMapOptional:       %v\n", *source.ConfigMap.Optional))
							} else {
								buf.WriteString("    ConfigMapOptional:       <nil>\n")
							}
						}
						if source.DownwardAPI != nil {
							buf.WriteString("    DownwardAPI:             true\n")
						}
					}
				}
			} else {
				buf.WriteString("    Type:       <unknown>\n")
			}
		}
	} else {
		buf.WriteString("  <none>\n")
	}

	// ========== QoS Class ==========
	if pod.Status.QOSClass != "" {
		buf.WriteString(fmt.Sprintf("QoS Class:       %s\n", pod.Status.QOSClass))
	} else {
		buf.WriteString("QoS Class:       <none>\n")
	}

	// ========== Node Selectors ==========
	if len(pod.Spec.NodeSelector) > 0 {
		buf.WriteString("Node-Selectors:  ")
		first := true
		for k, v := range pod.Spec.NodeSelector {
			if !first {
				buf.WriteString("                 ")
			}
			buf.WriteString(fmt.Sprintf("%s=%s\n", k, v))
			first = false
		}
	} else {
		buf.WriteString("Node-Selectors:  <none>\n")
	}

	// ========== Tolerations ==========
	buf.WriteString("Tolerations:     ")
	if len(pod.Spec.Tolerations) > 0 {
		for i, tol := range pod.Spec.Tolerations {
			if i > 0 {
				buf.WriteString("                 ")
			}

			if tol.Key != "" {
				buf.WriteString(tol.Key)
			}

			if tol.Operator == corev1.TolerationOpEqual {
				buf.WriteString(fmt.Sprintf("=%s", tol.Value))
			} else if tol.Operator == corev1.TolerationOpExists {
				if tol.Key != "" {
					buf.WriteString(" op=Exists")
				}
			}

			if tol.Effect != "" {
				buf.WriteString(fmt.Sprintf(":%s", tol.Effect))
			}

			if tol.TolerationSeconds != nil {
				buf.WriteString(fmt.Sprintf(" for %ds", *tol.TolerationSeconds))
			}
			buf.WriteString("\n")
		}
	} else {
		buf.WriteString("<none>\n")
	}

	// ========== Events ==========
	buf.WriteString("Events:\n")
	events, err := p.GetEvents(namespace, name)
	if err == nil && len(events) > 0 {
		buf.WriteString("  Type     Reason            Age                From               Message\n")
		buf.WriteString("  ----     ------            ----               ----               -------\n")

		limit := 10
		if len(events) < limit {
			limit = len(events)
		}

		for i := 0; i < limit; i++ {
			event := events[i]

			var ageStr string
			if event.LastTimestamp > 0 {
				age := time.Since(time.UnixMilli(event.LastTimestamp)).Round(time.Second)
				ageStr = p.formatDurationForDescribe(age)

				// 如果事件有多次发生，显示次数
				if event.Count > 1 {
					ageStr = fmt.Sprintf("%s (x%d over %s)",
						p.formatDurationForDescribe(time.Since(time.UnixMilli(event.FirstTimestamp)).Round(time.Second)),
						event.Count,
						p.formatDurationForDescribe(time.Since(time.UnixMilli(event.FirstTimestamp)).Round(time.Second)))
				}
			} else {
				ageStr = "<unknown>"
			}

			buf.WriteString(fmt.Sprintf("  %-7s %-15s %-18s %-18s %s\n",
				event.Type, event.Reason, ageStr, event.Source, event.Message))
		}
	} else {
		buf.WriteString("  <none>\n")
	}

	return buf.String(), nil
}

func (p *podOperator) formatProbeForDescribe(probe *corev1.Probe) string {
	if probe == nil {
		return ""
	}

	var parts []string

	if probe.HTTPGet != nil {
		host := probe.HTTPGet.Host
		if host == "" {
			host = "<node-ip>"
		}
		parts = append(parts, fmt.Sprintf("http-get %s:%s%s",
			host, probe.HTTPGet.Port.String(), probe.HTTPGet.Path))
	} else if probe.TCPSocket != nil {
		parts = append(parts, fmt.Sprintf("tcp-socket :%s", probe.TCPSocket.Port.String()))
	} else if probe.Exec != nil {
		if len(probe.Exec.Command) > 0 {
			parts = append(parts, fmt.Sprintf("exec %v", probe.Exec.Command))
		} else {
			parts = append(parts, "exec")
		}
	} else if probe.GRPC != nil {
		parts = append(parts, fmt.Sprintf("grpc <pod>:%d", probe.GRPC.Port))
	}

	parts = append(parts, fmt.Sprintf("delay=%ds", probe.InitialDelaySeconds))
	parts = append(parts, fmt.Sprintf("timeout=%ds", probe.TimeoutSeconds))
	parts = append(parts, fmt.Sprintf("period=%ds", probe.PeriodSeconds))

	if probe.SuccessThreshold > 0 {
		parts = append(parts, fmt.Sprintf("success=%d", probe.SuccessThreshold))
	}
	if probe.FailureThreshold > 0 {
		parts = append(parts, fmt.Sprintf("failure=%d", probe.FailureThreshold))
	}

	return strings.Join(parts, " ")
}

func (p *podOperator) formatDurationForDescribe(duration time.Duration) string {
	if duration < 0 {
		return "0s"
	}
	if duration < time.Minute {
		return fmt.Sprintf("%ds", int(duration.Seconds()))
	} else if duration < time.Hour {
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	} else if duration < 24*time.Hour {
		return fmt.Sprintf("%dh", int(duration.Hours()))
	} else {
		return fmt.Sprintf("%dd", int(duration.Hours()/24))
	}
}
