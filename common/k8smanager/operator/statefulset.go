package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/yaml"
)

type statefulSetOperator struct {
	BaseOperator
	client              kubernetes.Interface
	informerFactory     informers.SharedInformerFactory
	statefulSetLister   v1.StatefulSetLister
	statefulSetInformer cache.SharedIndexInformer
}

func NewStatefulSetOperator(ctx context.Context, client kubernetes.Interface) types.StatefulSetOperator {
	return &statefulSetOperator{
		BaseOperator: NewBaseOperator(ctx, false),
		client:       client,
	}
}

func NewStatefulSetOperatorWithInformer(
	ctx context.Context,
	client kubernetes.Interface,
	informerFactory informers.SharedInformerFactory,
) types.StatefulSetOperator {
	var statefulSetLister v1.StatefulSetLister
	var statefulSetInformer cache.SharedIndexInformer

	if informerFactory != nil {
		statefulSetInformer = informerFactory.Apps().V1().StatefulSets().Informer()
		statefulSetLister = informerFactory.Apps().V1().StatefulSets().Lister()
	}

	return &statefulSetOperator{
		BaseOperator:        NewBaseOperator(ctx, informerFactory != nil),
		client:              client,
		informerFactory:     informerFactory,
		statefulSetLister:   statefulSetLister,
		statefulSetInformer: statefulSetInformer,
	}
}

func (s *statefulSetOperator) Create(statefulSet *appsv1.StatefulSet) (*appsv1.StatefulSet, error) {
	if statefulSet == nil || statefulSet.Name == "" || statefulSet.Namespace == "" {
		return nil, fmt.Errorf("StatefulSet对象、名称和命名空间不能为空")
	}

	if statefulSet.Labels == nil {
		statefulSet.Labels = make(map[string]string)
	}
	if statefulSet.Annotations == nil {
		statefulSet.Annotations = make(map[string]string)
	}
	injectCommonAnnotations(statefulSet)
	created, err := s.client.AppsV1().StatefulSets(statefulSet.Namespace).Create(s.ctx, statefulSet, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("创建StatefulSet失败: %v", err)
	}

	return created, nil
}

func (s *statefulSetOperator) Get(namespace, name string) (*appsv1.StatefulSet, error) {
	if namespace == "" || name == "" {
		return nil, fmt.Errorf("命名空间和名称不能为空")
	}

	if s.statefulSetLister != nil {
		statefulSet, err := s.statefulSetLister.StatefulSets(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("StatefulSet %s/%s 不存在", namespace, name)
			}
			statefulSet, apiErr := s.client.AppsV1().StatefulSets(namespace).Get(s.ctx, name, metav1.GetOptions{})
			if apiErr != nil {
				return nil, fmt.Errorf("获取StatefulSet失败")
			}
			statefulSet.TypeMeta = metav1.TypeMeta{
				Kind:       "StatefulSet",
				APIVersion: "apps/v1",
			}
			statefulSet.Name = name
			return statefulSet, nil
		}
		statefulSet.TypeMeta = metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		}
		statefulSet.Name = name
		return statefulSet, nil
	}

	statefulSet, err := s.client.AppsV1().StatefulSets(namespace).Get(s.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("StatefulSet %s/%s 不存在", namespace, name)
		}
		return nil, fmt.Errorf("获取StatefulSet失败")
	}
	statefulSet.TypeMeta = metav1.TypeMeta{
		Kind:       "StatefulSet",
		APIVersion: "apps/v1",
	}
	statefulSet.Name = name
	return statefulSet, nil
}

func (s *statefulSetOperator) Update(statefulSet *appsv1.StatefulSet) (*appsv1.StatefulSet, error) {
	if statefulSet == nil || statefulSet.Name == "" || statefulSet.Namespace == "" {
		return nil, fmt.Errorf("StatefulSet对象、名称和命名空间不能为空")
	}

	updated, err := s.client.AppsV1().StatefulSets(statefulSet.Namespace).Update(s.ctx, statefulSet, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("更新StatefulSet失败: %v", err)
	}

	return updated, nil
}

func (s *statefulSetOperator) Delete(namespace, name string) error {
	if namespace == "" || name == "" {
		return fmt.Errorf("命名空间和名称不能为空")
	}

	err := s.client.AppsV1().StatefulSets(namespace).Delete(s.ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("删除StatefulSet失败: %v", err)
	}

	return nil
}

func (s *statefulSetOperator) List(namespace string, req types.ListRequest) (*types.ListStatefulSetResponse, error) {
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

	var statefulSets []*appsv1.StatefulSet
	var err error

	if s.useInformer && s.statefulSetLister != nil {
		statefulSets, err = s.statefulSetLister.StatefulSets(namespace).List(selector)
		if err != nil {
			return nil, fmt.Errorf("获取StatefulSet列表失败")
		}
	} else {
		listOpts := metav1.ListOptions{LabelSelector: selector.String()}
		statefulSetList, err := s.client.AppsV1().StatefulSets(namespace).List(s.ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("获取StatefulSet列表失败")
		}
		statefulSets = make([]*appsv1.StatefulSet, len(statefulSetList.Items))
		for i := range statefulSetList.Items {
			statefulSets[i] = &statefulSetList.Items[i]
		}
	}

	if req.Search != "" {
		filtered := make([]*appsv1.StatefulSet, 0)
		searchLower := strings.ToLower(req.Search)
		for _, sts := range statefulSets {
			if strings.Contains(strings.ToLower(sts.Name), searchLower) {
				filtered = append(filtered, sts)
			}
		}
		statefulSets = filtered
	}

	sort.Slice(statefulSets, func(i, j int) bool {
		var less bool
		switch req.SortBy {
		case "creationTime", "creationTimestamp":
			less = statefulSets[i].CreationTimestamp.Before(&statefulSets[j].CreationTimestamp)
		default:
			less = statefulSets[i].Name < statefulSets[j].Name
		}
		if req.SortDesc {
			return !less
		}
		return less
	})

	total := len(statefulSets)
	totalPages := (total + req.PageSize - 1) / req.PageSize
	start := (req.Page - 1) * req.PageSize
	end := start + req.PageSize

	if start >= total {
		return &types.ListStatefulSetResponse{
			ListResponse: types.ListResponse{
				Total:      total,
				Page:       req.Page,
				PageSize:   req.PageSize,
				TotalPages: totalPages,
			},
			Items: []types.StatefulSetInfo{},
		}, nil
	}

	if end > total {
		end = total
	}

	pageStatefulSets := statefulSets[start:end]
	items := make([]types.StatefulSetInfo, len(pageStatefulSets))
	for i, sts := range pageStatefulSets {
		items[i] = s.convertToStatefulSetInfo(sts)
	}

	return &types.ListStatefulSetResponse{
		ListResponse: types.ListResponse{
			Total:      total,
			Page:       req.Page,
			PageSize:   req.PageSize,
			TotalPages: totalPages,
		},
		Items: items,
	}, nil
}

func (s *statefulSetOperator) convertToStatefulSetInfo(sts *appsv1.StatefulSet) types.StatefulSetInfo {
	images := make([]string, 0)
	for _, container := range sts.Spec.Template.Spec.Containers {
		images = append(images, container.Image)
	}

	replicas := int32(0)
	if sts.Spec.Replicas != nil {
		replicas = *sts.Spec.Replicas
	}

	return types.StatefulSetInfo{
		Name:              sts.Name,
		Namespace:         sts.Namespace,
		Replicas:          replicas,
		ReadyReplicas:     sts.Status.ReadyReplicas,
		CurrentReplicas:   sts.Status.CurrentReplicas,
		CreationTimestamp: sts.CreationTimestamp.Time,
		Images:            images,
	}
}

func (s *statefulSetOperator) Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return s.client.AppsV1().StatefulSets(namespace).Watch(s.ctx, opts)
}

func (s *statefulSetOperator) UpdateLabels(namespace, name string, labels map[string]string) error {
	statefulSet, err := s.Get(namespace, name)
	if err != nil {
		return err
	}

	if statefulSet.Labels == nil {
		statefulSet.Labels = make(map[string]string)
	}
	for k, v := range labels {
		statefulSet.Labels[k] = v
	}

	_, err = s.Update(statefulSet)
	return err
}

func (s *statefulSetOperator) UpdateAnnotations(namespace, name string, annotations map[string]string) error {
	statefulSet, err := s.Get(namespace, name)
	if err != nil {
		return err
	}

	if statefulSet.Annotations == nil {
		statefulSet.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		statefulSet.Annotations[k] = v
	}

	_, err = s.Update(statefulSet)
	return err
}

func (s *statefulSetOperator) GetYaml(namespace, name string) (string, error) {
	statefulSet, err := s.Get(namespace, name)
	if err != nil {
		return "", err
	}

	// 设置 TypeMeta
	statefulSet.TypeMeta = metav1.TypeMeta{
		Kind:       "StatefulSet",
		APIVersion: "apps/v1",
	}
	statefulSet.Name = name
	statefulSet.ManagedFields = nil

	yamlBytes, err := yaml.Marshal(statefulSet)
	if err != nil {
		return "", fmt.Errorf("转换为YAML失败: %v", err)
	}

	return string(yamlBytes), nil
}

func (s *statefulSetOperator) GetPods(namespace, name string) ([]types.PodDetailInfo, error) {
	statefulSet, err := s.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	labelSelector := metav1.FormatLabelSelector(statefulSet.Spec.Selector)
	podList, err := s.client.CoreV1().Pods(namespace).List(s.ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("获取Pod列表失败: %v", err)
	}

	pods := make([]types.PodDetailInfo, 0, len(podList.Items))
	for i := range podList.Items {
		pod := &podList.Items[i]
		pods = append(pods, s.convertToPodDetailInfo(pod))
	}

	return pods, nil
}

func (s *statefulSetOperator) convertToPodDetailInfo(pod *corev1.Pod) types.PodDetailInfo {
	var restarts int32
	for _, cs := range pod.Status.ContainerStatuses {
		restarts += cs.RestartCount
	}

	readyCount := 0
	totalCount := len(pod.Status.ContainerStatuses)
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Ready {
			readyCount++
		}
	}

	age := time.Since(pod.CreationTimestamp.Time).Round(time.Second).String()

	return types.PodDetailInfo{
		Name:         pod.Name,
		Namespace:    pod.Namespace,
		Status:       string(pod.Status.Phase),
		Ready:        fmt.Sprintf("%d/%d", readyCount, totalCount),
		Restarts:     restarts,
		Age:          age,
		Node:         pod.Spec.NodeName,
		PodIP:        pod.Status.PodIP,
		Labels:       pod.Labels,
		CreationTime: pod.CreationTimestamp.UnixMilli(),
	}
}

func (s *statefulSetOperator) GetContainerImages(namespace, name string) (*types.ContainerInfoList, error) {
	statefulSet, err := s.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	result := &types.ContainerInfoList{
		InitContainers: make([]types.ContainerInfo, 0),
		Containers:     make([]types.ContainerInfo, 0),
	}

	for _, container := range statefulSet.Spec.Template.Spec.InitContainers {
		result.InitContainers = append(result.InitContainers, types.ContainerInfo{
			Name:  container.Name,
			Image: container.Image,
		})
	}

	for _, container := range statefulSet.Spec.Template.Spec.Containers {
		result.Containers = append(result.Containers, types.ContainerInfo{
			Name:  container.Name,
			Image: container.Image,
		})
	}

	return result, nil
}

func (s *statefulSetOperator) UpdateImage(req *types.UpdateImageRequest) error {
	// 参数验证
	if req == nil {
		return fmt.Errorf("请求不能为空")
	}
	if req.Namespace == "" {
		return fmt.Errorf("命名空间不能为空")
	}
	if req.Name == "" {
		return fmt.Errorf("StatefulSet名称不能为空")
	}
	if req.ContainerName == "" {
		return fmt.Errorf("容器名称不能为空")
	}
	if req.Image == "" {
		return fmt.Errorf("镜像不能为空")
	}

	// 基本镜像格式验证
	if err := validateImageFormat(req.Image); err != nil {
		return fmt.Errorf("镜像格式无效: %v", err)
	}

	// 获取 StatefulSet
	statefulSet, err := s.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	// 查找并更新容器镜像
	var oldImage string
	found := false

	// 先在 InitContainers 中查找
	for i := range statefulSet.Spec.Template.Spec.InitContainers {
		container := &statefulSet.Spec.Template.Spec.InitContainers[i]
		if container.Name == req.ContainerName {
			oldImage = container.Image
			container.Image = req.Image
			found = true
			break
		}
	}

	// 再在普通 Containers 中查找
	if !found {
		for i := range statefulSet.Spec.Template.Spec.Containers {
			container := &statefulSet.Spec.Template.Spec.Containers[i]
			if container.Name == req.ContainerName {
				oldImage = container.Image
				container.Image = req.Image
				found = true
				break
			}
		}
	}

	// 最后在 EphemeralContainers 中查找
	if !found {
		for i := range statefulSet.Spec.Template.Spec.EphemeralContainers {
			container := &statefulSet.Spec.Template.Spec.EphemeralContainers[i]
			if container.Name == req.ContainerName {
				oldImage = container.Image
				container.Image = req.Image
				found = true
				break
			}
		}
	}

	if !found {
		// 提供更有用的错误信息
		availableContainers := s.getAvailableContainerNames(statefulSet)
		return fmt.Errorf("未找到容器 '%s'，可用容器: %v", req.ContainerName, availableContainers)
	}

	// 如果镜像没有变化，直接返回（幂等）
	if oldImage == req.Image {
		return nil
	}

	// 设置变更原因（用于回滚历史）
	if statefulSet.Annotations == nil {
		statefulSet.Annotations = make(map[string]string)
	}

	changeCause := req.Reason
	if changeCause == "" {
		changeCause = fmt.Sprintf("image updated: %s %s -> %s",
			req.ContainerName, extractImageTag(oldImage), extractImageTag(req.Image))
	}
	statefulSet.Annotations["kubernetes.io/change-cause"] = changeCause

	_, err = s.Update(statefulSet)
	if err != nil {
		return fmt.Errorf("更新StatefulSet失败: %v", err)
	}

	return nil
}

func (s *statefulSetOperator) UpdateImages(req *types.UpdateImagesRequest) error {
	// 参数验证
	if req == nil {
		return fmt.Errorf("请求不能为空")
	}
	if req.Namespace == "" {
		return fmt.Errorf("命名空间不能为空")
	}
	if req.Name == "" {
		return fmt.Errorf("StatefulSet名称不能为空")
	}

	// 检查是否有需要更新的容器
	totalRequested := len(req.Containers.InitContainers) + len(req.Containers.Containers)
	if req.Containers.EphemeralContainers != nil {
		totalRequested += len(req.Containers.EphemeralContainers)
	}
	if totalRequested == 0 {
		return fmt.Errorf("未指定要更新的容器")
	}

	// 验证所有镜像格式
	for _, c := range req.Containers.InitContainers {
		if err := validateImageFormat(c.Image); err != nil {
			return fmt.Errorf("InitContainer '%s' 镜像格式无效: %v", c.Name, err)
		}
	}
	for _, c := range req.Containers.Containers {
		if err := validateImageFormat(c.Image); err != nil {
			return fmt.Errorf("Container '%s' 镜像格式无效: %v", c.Name, err)
		}
	}
	if req.Containers.EphemeralContainers != nil {
		for _, c := range req.Containers.EphemeralContainers {
			if err := validateImageFormat(c.Image); err != nil {
				return fmt.Errorf("EphemeralContainer '%s' 镜像格式无效: %v", c.Name, err)
			}
		}
	}

	// 获取 StatefulSet
	statefulSet, err := s.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	// 构建容器名到索引的映射，提高查找效率
	initContainerMap := make(map[string]int)
	for i, c := range statefulSet.Spec.Template.Spec.InitContainers {
		initContainerMap[c.Name] = i
	}

	containerMap := make(map[string]int)
	for i, c := range statefulSet.Spec.Template.Spec.Containers {
		containerMap[c.Name] = i
	}

	ephemeralContainerMap := make(map[string]int)
	for i, c := range statefulSet.Spec.Template.Spec.EphemeralContainers {
		ephemeralContainerMap[c.Name] = i
	}

	// 记录变更
	var changes []string

	// 更新 InitContainers
	for _, img := range req.Containers.InitContainers {
		if idx, exists := initContainerMap[img.Name]; exists {
			container := &statefulSet.Spec.Template.Spec.InitContainers[idx]
			if container.Image != img.Image {
				changes = append(changes, fmt.Sprintf("%s=%s", img.Name, extractImageTag(img.Image)))
				container.Image = img.Image
			}
		}
	}

	// 更新 Containers
	for _, img := range req.Containers.Containers {
		if idx, exists := containerMap[img.Name]; exists {
			container := &statefulSet.Spec.Template.Spec.Containers[idx]
			if container.Image != img.Image {
				changes = append(changes, fmt.Sprintf("%s=%s", img.Name, extractImageTag(img.Image)))
				container.Image = img.Image
			}
		}
	}

	// 更新 EphemeralContainers
	if req.Containers.EphemeralContainers != nil {
		for _, img := range req.Containers.EphemeralContainers {
			if idx, exists := ephemeralContainerMap[img.Name]; exists {
				container := &statefulSet.Spec.Template.Spec.EphemeralContainers[idx]
				if container.Image != img.Image {
					changes = append(changes, fmt.Sprintf("%s=%s", img.Name, extractImageTag(img.Image)))
					container.Image = img.Image
				}
			}
		}
	}

	// 如果没有实际变更，直接返回（幂等）
	if len(changes) == 0 {
		return nil
	}

	// 设置变更原因
	if statefulSet.Annotations == nil {
		statefulSet.Annotations = make(map[string]string)
	}

	changeCause := req.Reason
	if changeCause == "" {
		changeCause = fmt.Sprintf("images updated: %s", strings.Join(changes, ", "))
	}
	statefulSet.Annotations["kubernetes.io/change-cause"] = changeCause

	// 执行更新
	_, err = s.Update(statefulSet)
	if err != nil {
		return fmt.Errorf("更新StatefulSet失败: %v", err)
	}

	return nil
}

// getAvailableContainerNames 获取可用的容器名称列表
func (s *statefulSetOperator) getAvailableContainerNames(statefulSet *appsv1.StatefulSet) []string {
	names := make([]string, 0)
	for _, c := range statefulSet.Spec.Template.Spec.InitContainers {
		names = append(names, c.Name+" (init)")
	}
	for _, c := range statefulSet.Spec.Template.Spec.Containers {
		names = append(names, c.Name)
	}
	for _, c := range statefulSet.Spec.Template.Spec.EphemeralContainers {
		names = append(names, c.Name+" (ephemeral)")
	}
	return names
}

func (s *statefulSetOperator) GetReplicas(namespace, name string) (*types.ReplicasInfo, error) {
	statefulSet, err := s.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	replicas := int32(0)
	if statefulSet.Spec.Replicas != nil {
		replicas = *statefulSet.Spec.Replicas
	}

	return &types.ReplicasInfo{
		Replicas:          replicas,
		ReadyReplicas:     statefulSet.Status.ReadyReplicas,
		CurrentReplicas:   statefulSet.Status.CurrentReplicas,
		UpdatedReplicas:   statefulSet.Status.UpdatedReplicas,
		AvailableReplicas: statefulSet.Status.AvailableReplicas,
	}, nil
}

func (s *statefulSetOperator) Scale(sc *types.ScaleRequest) error {
	statefulSet, err := s.Get(sc.Namespace, sc.Name)
	if err != nil {
		return err
	}

	statefulSet.Spec.Replicas = &sc.Replicas
	_, err = s.Update(statefulSet)
	return err
}

func (s *statefulSetOperator) GetUpdateStrategy(namespace, name string) (*types.UpdateStrategyResponse, error) {
	statefulSet, err := s.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	response := &types.UpdateStrategyResponse{
		Type: string(statefulSet.Spec.UpdateStrategy.Type),
	}

	if statefulSet.Spec.UpdateStrategy.RollingUpdate != nil {
		rollingUpdate := &types.RollingUpdateConfig{}
		if statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition != nil {
			rollingUpdate.Partition = *statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition
		}
		if statefulSet.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable != nil {
			rollingUpdate.MaxUnavailable = statefulSet.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable.String()
		}
		response.RollingUpdate = rollingUpdate
	}

	return response, nil
}

func (s *statefulSetOperator) UpdateStrategy(req *types.UpdateStrategyRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" {
		return fmt.Errorf("请求参数不完整")
	}

	statefulSet, err := s.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	statefulSet.Spec.UpdateStrategy.Type = appsv1.StatefulSetUpdateStrategyType(req.Type)

	if req.Type == "RollingUpdate" && req.RollingUpdate != nil {
		if statefulSet.Spec.UpdateStrategy.RollingUpdate == nil {
			statefulSet.Spec.UpdateStrategy.RollingUpdate = &appsv1.RollingUpdateStatefulSetStrategy{}
		}

		// 支持设置 Partition 为 0（表示更新所有 Pod）
		statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition = &req.RollingUpdate.Partition

		if req.RollingUpdate.MaxUnavailable != "" {
			maxUnavailable := intstr.Parse(req.RollingUpdate.MaxUnavailable)
			statefulSet.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable = &maxUnavailable
		} else {
			statefulSet.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable = nil
		}
	} else {
		// 当策略不是 RollingUpdate 时，清空 RollingUpdate 配置
		statefulSet.Spec.UpdateStrategy.RollingUpdate = nil
	}

	_, err = s.Update(statefulSet)
	return err
}

func (s *statefulSetOperator) GetRevisions(namespace, name string) ([]types.RevisionInfo, error) {
	statefulSet, err := s.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	revisionList, err := s.client.AppsV1().ControllerRevisions(namespace).List(s.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取ControllerRevision列表失败")
	}

	revisions := make([]types.RevisionInfo, 0)
	for i := range revisionList.Items {
		cr := &revisionList.Items[i]

		isOwned := false
		for _, owner := range cr.OwnerReferences {
			if owner.Kind == "StatefulSet" && owner.Name == statefulSet.Name && owner.UID == statefulSet.UID {
				isOwned = true
				break
			}
		}

		if !isOwned {
			continue
		}

		images := make([]string, 0)
		var revisionData appsv1.StatefulSet
		if err := json.Unmarshal(cr.Data.Raw, &revisionData); err == nil {
			for _, container := range revisionData.Spec.Template.Spec.Containers {
				images = append(images, container.Image)
			}
		}

		replicas := int32(0)
		if revisionData.Spec.Replicas != nil {
			replicas = *revisionData.Spec.Replicas
		}

		revisions = append(revisions, types.RevisionInfo{
			Revision:          cr.Revision,
			CreationTimestamp: cr.CreationTimestamp.UnixMilli(),
			Images:            images,
			Replicas:          replicas,
			Reason:            "StatefulSet Update",
		})
	}

	sort.Slice(revisions, func(i, j int) bool {
		return revisions[i].Revision > revisions[j].Revision
	})

	return revisions, nil
}

func (s *statefulSetOperator) GetConfigHistory(namespace, name string) ([]types.ConfigHistoryInfo, error) {
	return nil, fmt.Errorf("StatefulSet不支持业务层配置历史")
}

func (s *statefulSetOperator) Rollback(req *types.RollbackToRevisionRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" {
		return fmt.Errorf("请求参数不完整")
	}

	statefulSet, err := s.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	revisionList, err := s.client.AppsV1().ControllerRevisions(req.Namespace).List(s.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("获取ControllerRevision列表失败")
	}

	var targetRevision *appsv1.ControllerRevision
	for i := range revisionList.Items {
		cr := &revisionList.Items[i]

		isOwned := false
		for _, owner := range cr.OwnerReferences {
			if owner.Kind == "StatefulSet" && owner.Name == statefulSet.Name && owner.UID == statefulSet.UID {
				isOwned = true
				break
			}
		}

		if !isOwned {
			continue
		}

		if cr.Revision == req.Revision {
			targetRevision = cr
			break
		}
	}

	if targetRevision == nil {
		return fmt.Errorf("未找到目标版本: %d", req.Revision)
	}

	var revisionData appsv1.StatefulSet
	if err := json.Unmarshal(targetRevision.Data.Raw, &revisionData); err != nil {
		return fmt.Errorf("解析历史版本数据失败: %v", err)
	}

	statefulSet.Spec = revisionData.Spec
	_, err = s.Update(statefulSet)
	return err
}

func (s *statefulSetOperator) RollbackToConfig(req *types.RollbackToConfigRequest) error {
	return fmt.Errorf("StatefulSet不支持业务层配置历史回滚")
}

func (s *statefulSetOperator) GetEnvVars(namespace, name string) (*types.EnvVarsResponse, error) {
	statefulSet, err := s.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	response := &types.EnvVarsResponse{
		Containers: make([]types.ContainerEnvVars, 0),
	}

	for _, container := range statefulSet.Spec.Template.Spec.Containers {
		envVars := make([]types.EnvVar, 0)
		for _, env := range container.Env {
			envVar := types.EnvVar{
				Name:   env.Name,
				Source: types.EnvVarSource{Type: "value", Value: env.Value},
			}

			if env.ValueFrom != nil {
				if env.ValueFrom.ConfigMapKeyRef != nil {
					envVar.Source.Type = "configMapKeyRef"
					envVar.Source.ConfigMapKeyRef = &types.ConfigMapKeySelector{
						Name:     env.ValueFrom.ConfigMapKeyRef.Name,
						Key:      env.ValueFrom.ConfigMapKeyRef.Key,
						Optional: env.ValueFrom.ConfigMapKeyRef.Optional != nil && *env.ValueFrom.ConfigMapKeyRef.Optional,
					}
				} else if env.ValueFrom.SecretKeyRef != nil {
					envVar.Source.Type = "secretKeyRef"
					envVar.Source.SecretKeyRef = &types.SecretKeySelector{
						Name:     env.ValueFrom.SecretKeyRef.Name,
						Key:      env.ValueFrom.SecretKeyRef.Key,
						Optional: env.ValueFrom.SecretKeyRef.Optional != nil && *env.ValueFrom.SecretKeyRef.Optional,
					}
				} else if env.ValueFrom.FieldRef != nil {
					envVar.Source.Type = "fieldRef"
					envVar.Source.FieldRef = &types.ObjectFieldSelector{
						FieldPath: env.ValueFrom.FieldRef.FieldPath,
					}
				} else if env.ValueFrom.ResourceFieldRef != nil {
					envVar.Source.Type = "resourceFieldRef"
					divisor := ""
					if env.ValueFrom.ResourceFieldRef.Divisor.String() != "" {
						divisor = env.ValueFrom.ResourceFieldRef.Divisor.String()
					}
					envVar.Source.ResourceFieldRef = &types.ResourceFieldSelector{
						ContainerName: env.ValueFrom.ResourceFieldRef.ContainerName,
						Resource:      env.ValueFrom.ResourceFieldRef.Resource,
						Divisor:       divisor,
					}
				}
			}

			envVars = append(envVars, envVar)
		}

		response.Containers = append(response.Containers, types.ContainerEnvVars{
			ContainerName: container.Name,
			Env:           envVars,
		})
	}

	return response, nil
}

func (s *statefulSetOperator) UpdateEnvVars(req *types.UpdateEnvVarsRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" || req.ContainerName == "" {
		return fmt.Errorf("请求参数不完整")
	}

	statefulSet, err := s.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	found := false
	for i := range statefulSet.Spec.Template.Spec.Containers {
		if statefulSet.Spec.Template.Spec.Containers[i].Name == req.ContainerName {
			envVars := make([]corev1.EnvVar, 0)
			for _, env := range req.Env {
				envVar := corev1.EnvVar{Name: env.Name}

				switch env.Source.Type {
				case "value":
					envVar.Value = env.Source.Value
				case "configMapKeyRef":
					if env.Source.ConfigMapKeyRef != nil {
						envVar.ValueFrom = &corev1.EnvVarSource{
							ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: env.Source.ConfigMapKeyRef.Name},
								Key:                  env.Source.ConfigMapKeyRef.Key,
								Optional:             &env.Source.ConfigMapKeyRef.Optional,
							},
						}
					}
				case "secretKeyRef":
					if env.Source.SecretKeyRef != nil {
						envVar.ValueFrom = &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: env.Source.SecretKeyRef.Name},
								Key:                  env.Source.SecretKeyRef.Key,
								Optional:             &env.Source.SecretKeyRef.Optional,
							},
						}
					}
				case "fieldRef":
					if env.Source.FieldRef != nil {
						envVar.ValueFrom = &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{FieldPath: env.Source.FieldRef.FieldPath},
						}
					}
				case "resourceFieldRef":
					if env.Source.ResourceFieldRef != nil {
						var divisor resource.Quantity
						if env.Source.ResourceFieldRef.Divisor != "" {
							divisor, _ = resource.ParseQuantity(env.Source.ResourceFieldRef.Divisor)
						}
						envVar.ValueFrom = &corev1.EnvVarSource{
							ResourceFieldRef: &corev1.ResourceFieldSelector{
								ContainerName: env.Source.ResourceFieldRef.ContainerName,
								Resource:      env.Source.ResourceFieldRef.Resource,
								Divisor:       divisor,
							},
						}
					}
				}

				envVars = append(envVars, envVar)
			}

			statefulSet.Spec.Template.Spec.Containers[i].Env = envVars
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("未找到容器: %s", req.ContainerName)
	}

	_, err = s.Update(statefulSet)
	return err
}

func (s *statefulSetOperator) GetPauseStatus(namespace, name string) (*types.PauseStatusResponse, error) {
	return &types.PauseStatusResponse{
		Paused:      false,
		SupportType: "none",
	}, nil
}

func (s *statefulSetOperator) GetResources(namespace, name string) (*types.ResourcesResponse, error) {
	statefulSet, err := s.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	response := &types.ResourcesResponse{
		Containers: make([]types.ContainerResources, 0),
	}

	for _, container := range statefulSet.Spec.Template.Spec.Containers {
		resources := types.ContainerResources{
			ContainerName: container.Name,
			Resources: types.ResourceRequirements{
				Limits: types.ResourceList{
					Cpu:    container.Resources.Limits.Cpu().String(),
					Memory: container.Resources.Limits.Memory().String(),
				},
				Requests: types.ResourceList{
					Cpu:    container.Resources.Requests.Cpu().String(),
					Memory: container.Resources.Requests.Memory().String(),
				},
			},
		}
		response.Containers = append(response.Containers, resources)
	}

	return response, nil
}

func (s *statefulSetOperator) UpdateResources(req *types.UpdateResourcesRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" || req.ContainerName == "" {
		return fmt.Errorf("请求参数不完整")
	}

	statefulSet, err := s.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	found := false
	for i := range statefulSet.Spec.Template.Spec.Containers {
		if statefulSet.Spec.Template.Spec.Containers[i].Name == req.ContainerName {
			if req.Resources.Limits.Cpu != "" {
				cpuLimit, err := resource.ParseQuantity(req.Resources.Limits.Cpu)
				if err != nil {
					return fmt.Errorf("解析CPU限制失败: %v", err)
				}
				if statefulSet.Spec.Template.Spec.Containers[i].Resources.Limits == nil {
					statefulSet.Spec.Template.Spec.Containers[i].Resources.Limits = corev1.ResourceList{}
				}
				statefulSet.Spec.Template.Spec.Containers[i].Resources.Limits[corev1.ResourceCPU] = cpuLimit
			}

			if req.Resources.Limits.Memory != "" {
				memLimit, err := resource.ParseQuantity(req.Resources.Limits.Memory)
				if err != nil {
					return fmt.Errorf("解析内存限制失败: %v", err)
				}
				if statefulSet.Spec.Template.Spec.Containers[i].Resources.Limits == nil {
					statefulSet.Spec.Template.Spec.Containers[i].Resources.Limits = corev1.ResourceList{}
				}
				statefulSet.Spec.Template.Spec.Containers[i].Resources.Limits[corev1.ResourceMemory] = memLimit
			}

			if req.Resources.Requests.Cpu != "" {
				cpuRequest, err := resource.ParseQuantity(req.Resources.Requests.Cpu)
				if err != nil {
					return fmt.Errorf("解析CPU请求失败: %v", err)
				}
				if statefulSet.Spec.Template.Spec.Containers[i].Resources.Requests == nil {
					statefulSet.Spec.Template.Spec.Containers[i].Resources.Requests = corev1.ResourceList{}
				}
				statefulSet.Spec.Template.Spec.Containers[i].Resources.Requests[corev1.ResourceCPU] = cpuRequest
			}

			if req.Resources.Requests.Memory != "" {
				memRequest, err := resource.ParseQuantity(req.Resources.Requests.Memory)
				if err != nil {
					return fmt.Errorf("解析内存请求失败: %v", err)
				}
				if statefulSet.Spec.Template.Spec.Containers[i].Resources.Requests == nil {
					statefulSet.Spec.Template.Spec.Containers[i].Resources.Requests = corev1.ResourceList{}
				}
				statefulSet.Spec.Template.Spec.Containers[i].Resources.Requests[corev1.ResourceMemory] = memRequest
			}

			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("未找到容器: %s", req.ContainerName)
	}

	_, err = s.Update(statefulSet)
	return err
}

func (s *statefulSetOperator) GetProbes(namespace, name string) (*types.ProbesResponse, error) {
	statefulSet, err := s.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	response := &types.ProbesResponse{
		Containers: make([]types.ContainerProbes, 0),
	}

	for _, container := range statefulSet.Spec.Template.Spec.Containers {
		containerProbes := types.ContainerProbes{
			ContainerName: container.Name,
		}

		if container.LivenessProbe != nil {
			containerProbes.LivenessProbe = s.convertProbe(container.LivenessProbe)
		}
		if container.ReadinessProbe != nil {
			containerProbes.ReadinessProbe = s.convertProbe(container.ReadinessProbe)
		}
		if container.StartupProbe != nil {
			containerProbes.StartupProbe = s.convertProbe(container.StartupProbe)
		}

		response.Containers = append(response.Containers, containerProbes)
	}

	return response, nil
}

func (s *statefulSetOperator) convertProbe(probe *corev1.Probe) *types.Probe {
	result := &types.Probe{
		InitialDelaySeconds: probe.InitialDelaySeconds,
		TimeoutSeconds:      probe.TimeoutSeconds,
		PeriodSeconds:       probe.PeriodSeconds,
		SuccessThreshold:    probe.SuccessThreshold,
		FailureThreshold:    probe.FailureThreshold,
	}

	if probe.HTTPGet != nil {
		result.Type = "httpGet"
		headers := make([]types.HTTPHeader, 0)
		for _, h := range probe.HTTPGet.HTTPHeaders {
			headers = append(headers, types.HTTPHeader{Name: h.Name, Value: h.Value})
		}
		result.HttpGet = &types.HTTPGetAction{
			Path:        probe.HTTPGet.Path,
			Port:        probe.HTTPGet.Port.IntVal,
			Host:        probe.HTTPGet.Host,
			Scheme:      string(probe.HTTPGet.Scheme),
			HttpHeaders: headers,
		}
	} else if probe.TCPSocket != nil {
		result.Type = "tcpSocket"
		result.TcpSocket = &types.TCPSocketAction{
			Port: probe.TCPSocket.Port.IntVal,
			Host: probe.TCPSocket.Host,
		}
	} else if probe.Exec != nil {
		result.Type = "exec"
		result.Exec = &types.ExecAction{Command: probe.Exec.Command}
	} else if probe.GRPC != nil {
		result.Type = "grpc"
		service := ""
		if probe.GRPC.Service != nil {
			service = *probe.GRPC.Service
		}
		result.Grpc = &types.GRPCAction{Port: probe.GRPC.Port, Service: service}
	}

	return result
}

func (s *statefulSetOperator) UpdateProbes(req *types.UpdateProbesRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" || req.ContainerName == "" {
		return fmt.Errorf("请求参数不完整")
	}

	statefulSet, err := s.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	found := false
	for i := range statefulSet.Spec.Template.Spec.Containers {
		if statefulSet.Spec.Template.Spec.Containers[i].Name == req.ContainerName {
			if req.LivenessProbe != nil {
				statefulSet.Spec.Template.Spec.Containers[i].LivenessProbe = s.buildProbe(req.LivenessProbe)
			}
			if req.ReadinessProbe != nil {
				statefulSet.Spec.Template.Spec.Containers[i].ReadinessProbe = s.buildProbe(req.ReadinessProbe)
			}
			if req.StartupProbe != nil {
				statefulSet.Spec.Template.Spec.Containers[i].StartupProbe = s.buildProbe(req.StartupProbe)
			}
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("未找到容器: %s", req.ContainerName)
	}

	_, err = s.Update(statefulSet)
	return err
}

func (s *statefulSetOperator) buildProbe(probe *types.Probe) *corev1.Probe {
	result := &corev1.Probe{
		InitialDelaySeconds: probe.InitialDelaySeconds,
		TimeoutSeconds:      probe.TimeoutSeconds,
		PeriodSeconds:       probe.PeriodSeconds,
		SuccessThreshold:    probe.SuccessThreshold,
		FailureThreshold:    probe.FailureThreshold,
	}

	switch probe.Type {
	case "httpGet":
		if probe.HttpGet != nil {
			headers := make([]corev1.HTTPHeader, 0)
			for _, h := range probe.HttpGet.HttpHeaders {
				headers = append(headers, corev1.HTTPHeader{Name: h.Name, Value: h.Value})
			}
			result.HTTPGet = &corev1.HTTPGetAction{
				Path:        probe.HttpGet.Path,
				Port:        intstr.FromInt(int(probe.HttpGet.Port)),
				Host:        probe.HttpGet.Host,
				Scheme:      corev1.URIScheme(probe.HttpGet.Scheme),
				HTTPHeaders: headers,
			}
		}
	case "tcpSocket":
		if probe.TcpSocket != nil {
			result.TCPSocket = &corev1.TCPSocketAction{
				Port: intstr.FromInt(int(probe.TcpSocket.Port)),
				Host: probe.TcpSocket.Host,
			}
		}
	case "exec":
		if probe.Exec != nil {
			result.Exec = &corev1.ExecAction{Command: probe.Exec.Command}
		}
	case "grpc":
		if probe.Grpc != nil {
			service := probe.Grpc.Service
			result.GRPC = &corev1.GRPCAction{Port: probe.Grpc.Port, Service: &service}
		}
	}

	return result
}

func (s *statefulSetOperator) Stop(namespace, name string) error {
	statefulSet, err := s.Get(namespace, name)
	if err != nil {
		return err
	}

	currentReplicas := int32(0)
	if statefulSet.Spec.Replicas != nil {
		currentReplicas = *statefulSet.Spec.Replicas
	}

	if statefulSet.Annotations == nil {
		statefulSet.Annotations = make(map[string]string)
	}
	statefulSet.Annotations[AnnotationReplicas] = strconv.Itoa(int(currentReplicas))

	zero := int32(0)
	statefulSet.Spec.Replicas = &zero

	_, err = s.Update(statefulSet)
	return err
}

func (s *statefulSetOperator) Start(namespace, name string) error {
	statefulSet, err := s.Get(namespace, name)
	if err != nil {
		return err
	}

	replicas := int32(1)
	if statefulSet.Annotations != nil {
		if replicasStr, ok := statefulSet.Annotations[AnnotationReplicas]; ok {
			if r, err := strconv.Atoi(replicasStr); err == nil && r > 0 {
				replicas = int32(r)
			}
		}
	}

	statefulSet.Spec.Replicas = &replicas
	_, err = s.Update(statefulSet)
	return err
}

func (s *statefulSetOperator) Restart(namespace, name string) error {
	statefulSet, err := s.Get(namespace, name)
	if err != nil {
		return err
	}

	if statefulSet.Spec.Template.Annotations == nil {
		statefulSet.Spec.Template.Annotations = make(map[string]string)
	}
	statefulSet.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	_, err = s.Update(statefulSet)
	return err
}

// statefulset.go - operator (假设有类似 Deployment 的结构)
func (s *statefulSetOperator) GetPodLabels(namespace, name string) (map[string]string, error) {
	var statefulSet *appsv1.StatefulSet
	var err error

	if s.useInformer && s.statefulSetLister != nil {
		statefulSet, err = s.statefulSetLister.StatefulSets(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("StatefulSet %s/%s 不存在", namespace, name)
			}
			statefulSet, err = s.client.AppsV1().StatefulSets(namespace).Get(s.ctx, name, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("获取StatefulSet失败")
			}
		}
	} else {
		statefulSet, err = s.client.AppsV1().StatefulSets(namespace).Get(s.ctx, name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("StatefulSet %s/%s 不存在", namespace, name)
			}
			return nil, fmt.Errorf("获取StatefulSet失败")
		}
	}

	if statefulSet.Spec.Template.Labels == nil {
		return make(map[string]string), nil
	}

	labels := make(map[string]string)
	for k, v := range statefulSet.Spec.Template.Labels {
		labels[k] = v
	}
	return labels, nil
}

func (s *statefulSetOperator) GetPodSelectorLabels(namespace, name string) (map[string]string, error) {
	var statefulSet *appsv1.StatefulSet
	var err error

	if s.useInformer && s.statefulSetLister != nil {
		statefulSet, err = s.statefulSetLister.StatefulSets(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("StatefulSet %s/%s 不存在", namespace, name)
			}
			statefulSet, err = s.client.AppsV1().StatefulSets(namespace).Get(s.ctx, name, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("获取StatefulSet失败")
			}
		}
	} else {
		statefulSet, err = s.client.AppsV1().StatefulSets(namespace).Get(s.ctx, name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("StatefulSet %s/%s 不存在", namespace, name)
			}
			return nil, fmt.Errorf("获取StatefulSet失败")
		}
	}

	if statefulSet.Spec.Selector == nil || statefulSet.Spec.Selector.MatchLabels == nil {
		return make(map[string]string), nil
	}

	labels := make(map[string]string)
	for k, v := range statefulSet.Spec.Selector.MatchLabels {
		labels[k] = v
	}
	return labels, nil
}

func (s *statefulSetOperator) GetVersionStatus(namespace, name string) (*types.ResourceStatus, error) {
	var statefulSet *appsv1.StatefulSet
	var err error

	if s.useInformer && s.statefulSetLister != nil {
		statefulSet, err = s.statefulSetLister.StatefulSets(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("StatefulSet %s/%s 不存在", namespace, name)
			}
			statefulSet, err = s.client.AppsV1().StatefulSets(namespace).Get(s.ctx, name, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("获取StatefulSet失败")
			}
		}
	} else {
		statefulSet, err = s.client.AppsV1().StatefulSets(namespace).Get(s.ctx, name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("StatefulSet %s/%s 不存在", namespace, name)
			}
			return nil, fmt.Errorf("获取StatefulSet失败")
		}
	}

	replicas := int32(0)
	if statefulSet.Spec.Replicas != nil {
		replicas = *statefulSet.Spec.Replicas
	}

	status := &types.ResourceStatus{
		Ready: false,
	}

	// 停止状态：副本数为 0
	if replicas == 0 {
		status.Status = types.StatusStopped
		status.Message = "副本数为 0，已停止"
		status.Ready = true
		return status, nil
	}

	// 创建中：刚创建，还没有就绪副本
	if statefulSet.Status.ReadyReplicas == 0 && statefulSet.Status.Replicas == 0 {
		age := time.Since(statefulSet.CreationTimestamp.Time)
		if age < 30*time.Second {
			status.Status = types.StatusCreating
			status.Message = "正在创建 Pod"
			return status, nil
		}
	}

	// 运行中：就绪副本数等于期望副本数
	if statefulSet.Status.ReadyReplicas == replicas &&
		statefulSet.Status.CurrentReplicas == replicas &&
		statefulSet.Status.UpdatedReplicas == replicas {
		status.Status = types.StatusRunning
		status.Message = fmt.Sprintf("所有副本运行正常 (%d/%d)", statefulSet.Status.ReadyReplicas, replicas)
		status.Ready = true
		return status, nil
	}

	// 更新中
	if statefulSet.Status.UpdatedReplicas < replicas {
		status.Status = types.StatusRunning
		status.Message = fmt.Sprintf("正在更新中 (已更新: %d/%d, 就绪: %d/%d)",
			statefulSet.Status.UpdatedReplicas, replicas,
			statefulSet.Status.ReadyReplicas, replicas)
		return status, nil
	}

	// 异常：就绪副本数少于期望副本数
	if statefulSet.Status.ReadyReplicas < replicas {
		age := time.Since(statefulSet.CreationTimestamp.Time)
		if age > 5*time.Minute {
			status.Status = types.StatusError
			status.Message = fmt.Sprintf("部分副本异常 (就绪: %d/%d)", statefulSet.Status.ReadyReplicas, replicas)
			return status, nil
		}
		status.Status = types.StatusCreating
		status.Message = fmt.Sprintf("正在启动 (就绪: %d/%d)", statefulSet.Status.ReadyReplicas, replicas)
		return status, nil
	}

	status.Status = types.StatusRunning
	status.Message = "运行中"
	status.Ready = true
	return status, nil
}

// ==================== 调度配置相关 ====================

// GetSchedulingConfig 获取调度配置
func (s *statefulSetOperator) GetSchedulingConfig(namespace, name string) (*types.SchedulingConfig, error) {
	statefulSet, err := s.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	return convertPodSpecToSchedulingConfig(&statefulSet.Spec.Template.Spec), nil
}

// UpdateSchedulingConfig 更新调度配置
func (s *statefulSetOperator) UpdateSchedulingConfig(namespace, name string, config *types.UpdateSchedulingConfigRequest) error {
	if config == nil {
		return fmt.Errorf("调度配置不能为空")
	}

	statefulSet, err := s.Get(namespace, name)
	if err != nil {
		return err
	}

	// 更新 NodeSelector
	if config.NodeSelector != nil {
		statefulSet.Spec.Template.Spec.NodeSelector = config.NodeSelector
	}

	// 更新 NodeName
	if config.NodeName != "" {
		statefulSet.Spec.Template.Spec.NodeName = config.NodeName
	}

	// 更新 Affinity
	if config.Affinity != nil {
		statefulSet.Spec.Template.Spec.Affinity = convertAffinityConfigToK8s(config.Affinity)
	}

	// 更新 Tolerations
	if config.Tolerations != nil {
		statefulSet.Spec.Template.Spec.Tolerations = convertTolerationsConfigToK8s(config.Tolerations)
	}

	// 更新 TopologySpreadConstraints
	if config.TopologySpreadConstraints != nil {
		statefulSet.Spec.Template.Spec.TopologySpreadConstraints = convertTopologySpreadConstraintsToK8s(config.TopologySpreadConstraints)
	}

	// 更新 SchedulerName
	if config.SchedulerName != "" {
		statefulSet.Spec.Template.Spec.SchedulerName = config.SchedulerName
	}

	// 更新 PriorityClassName
	if config.PriorityClassName != "" {
		statefulSet.Spec.Template.Spec.PriorityClassName = config.PriorityClassName
	}

	// 更新 Priority
	if config.Priority != nil {
		statefulSet.Spec.Template.Spec.Priority = config.Priority
	}

	// 更新 RuntimeClassName
	if config.RuntimeClassName != nil {
		statefulSet.Spec.Template.Spec.RuntimeClassName = config.RuntimeClassName
	}

	_, err = s.Update(statefulSet)
	return err
}

// ==================== 存储配置相关 ====================

// GetStorageConfig 获取存储配置
func (s *statefulSetOperator) GetStorageConfig(namespace, name string) (*types.StorageConfig, error) {
	statefulSet, err := s.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	config := convertPodSpecToStorageConfig(&statefulSet.Spec.Template.Spec)

	// StatefulSet 特有：添加 VolumeClaimTemplates
	if len(statefulSet.Spec.VolumeClaimTemplates) > 0 {
		config.VolumeClaimTemplates = make([]types.PersistentVolumeClaimConfig, 0)
		for _, pvc := range statefulSet.Spec.VolumeClaimTemplates {
			config.VolumeClaimTemplates = append(config.VolumeClaimTemplates, convertPVCToConfig(&pvc))
		}
	}

	return config, nil
}

// UpdateStorageConfig 更新存储配置
func (s *statefulSetOperator) UpdateStorageConfig(namespace, name string, config *types.UpdateStorageConfigRequest) error {
	if config == nil {
		return fmt.Errorf("存储配置不能为空")
	}

	statefulSet, err := s.Get(namespace, name)
	if err != nil {
		return err
	}

	// 更新 Volumes
	if config.Volumes != nil {
		statefulSet.Spec.Template.Spec.Volumes = convertVolumesConfigToK8s(config.Volumes)
	}

	// 更新 VolumeMounts
	if config.VolumeMounts != nil {
		for _, vmConfig := range config.VolumeMounts {
			for i := range statefulSet.Spec.Template.Spec.Containers {
				if statefulSet.Spec.Template.Spec.Containers[i].Name == vmConfig.ContainerName {
					statefulSet.Spec.Template.Spec.Containers[i].VolumeMounts = convertVolumeMountsToK8s(vmConfig.Mounts)
					break
				}
			}
		}
	}

	// 更新 VolumeClaimTemplates（StatefulSet 特有）
	if config.VolumeClaimTemplates != nil {
		statefulSet.Spec.VolumeClaimTemplates = convertPVCConfigsToK8s(config.VolumeClaimTemplates)
	}

	_, err = s.Update(statefulSet)
	return err
}

// ==================== Events 相关 ====================

// GetEvents 获取 StatefulSet 的事件（已存在，确保实现正确）
func (s *statefulSetOperator) GetEvents(namespace, name string) ([]types.EventInfo, error) {
	statefulSet, err := s.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	eventList, err := s.client.CoreV1().Events(namespace).List(s.ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=StatefulSet,involvedObject.uid=%s",
			name, statefulSet.UID),
	})
	if err != nil {
		return nil, fmt.Errorf("获取事件列表失败: %v", err)
	}

	events := make([]types.EventInfo, 0, len(eventList.Items))
	for i := range eventList.Items {
		events = append(events, types.ConvertK8sEventToEventInfo(&eventList.Items[i]))
	}

	// 按最后发生时间降序排序
	sort.Slice(events, func(i, j int) bool {
		return events[i].LastTimestamp > events[j].LastTimestamp
	})

	return events, nil
}
func (s *statefulSetOperator) GetDescribe(namespace, name string) (string, error) {
	statefulSet, err := s.Get(namespace, name)
	if err != nil {
		return "", err
	}

	var buf strings.Builder

	// ========== 基本信息 ==========
	buf.WriteString(fmt.Sprintf("Name:               %s\n", statefulSet.Name))
	buf.WriteString(fmt.Sprintf("Namespace:          %s\n", statefulSet.Namespace))

	if !statefulSet.CreationTimestamp.IsZero() {
		buf.WriteString(fmt.Sprintf("CreationTimestamp:  %s\n", statefulSet.CreationTimestamp.Format(time.RFC1123)))
	} else {
		buf.WriteString("CreationTimestamp:  <unset>\n")
	}

	if statefulSet.Spec.Selector != nil {
		buf.WriteString(fmt.Sprintf("Selector:           %s\n", metav1.FormatLabelSelector(statefulSet.Spec.Selector)))
	} else {
		buf.WriteString("Selector:           <none>\n")
	}

	// Labels
	buf.WriteString("Labels:             ")
	if len(statefulSet.Labels) == 0 {
		buf.WriteString("<none>\n")
	} else {
		first := true
		for k, v := range statefulSet.Labels {
			if !first {
				buf.WriteString("                    ")
			}
			buf.WriteString(fmt.Sprintf("%s=%s\n", k, v))
			first = false
		}
	}

	// Annotations
	buf.WriteString("Annotations:        ")
	if len(statefulSet.Annotations) == 0 {
		buf.WriteString("<none>\n")
	} else {
		first := true
		for k, v := range statefulSet.Annotations {
			if !first {
				buf.WriteString("                    ")
			}
			buf.WriteString(fmt.Sprintf("%s: %s\n", k, v))
			first = false
		}
	}

	replicas := int32(1)
	if statefulSet.Spec.Replicas != nil {
		replicas = *statefulSet.Spec.Replicas
	}

	buf.WriteString(fmt.Sprintf("Replicas:           %d desired | %d total\n", replicas, statefulSet.Status.Replicas))
	buf.WriteString(fmt.Sprintf("Update Strategy:    %s\n", statefulSet.Spec.UpdateStrategy.Type))

	if statefulSet.Spec.UpdateStrategy.RollingUpdate != nil && statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition != nil {
		buf.WriteString(fmt.Sprintf("  Partition:        %d\n", *statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition))
	}

	buf.WriteString(fmt.Sprintf("Pods Status:        %d Running / %d Ready / %d Updated\n",
		statefulSet.Status.Replicas,
		statefulSet.Status.ReadyReplicas,
		statefulSet.Status.UpdatedReplicas))

	if statefulSet.Spec.ServiceName != "" {
		buf.WriteString(fmt.Sprintf("Service Name:       %s\n", statefulSet.Spec.ServiceName))
	}

	if statefulSet.Spec.PodManagementPolicy != "" {
		buf.WriteString(fmt.Sprintf("Pod Management Policy: %s\n", statefulSet.Spec.PodManagementPolicy))
	}

	// ========== Pod Template ==========
	buf.WriteString("Pod Template:\n")
	buf.WriteString("  Labels:  ")
	if len(statefulSet.Spec.Template.Labels) == 0 {
		buf.WriteString("<none>\n")
	} else {
		first := true
		for k, v := range statefulSet.Spec.Template.Labels {
			if !first {
				buf.WriteString("           ")
			}
			buf.WriteString(fmt.Sprintf("%s=%s\n", k, v))
			first = false
		}
	}

	if statefulSet.Spec.Template.Spec.ServiceAccountName != "" {
		buf.WriteString(fmt.Sprintf("  Service Account:  %s\n", statefulSet.Spec.Template.Spec.ServiceAccountName))
	} else {
		buf.WriteString("  Service Account:  default\n")
	}

	// Init Containers
	if len(statefulSet.Spec.Template.Spec.InitContainers) > 0 {
		buf.WriteString("  Init Containers:\n")
		for _, container := range statefulSet.Spec.Template.Spec.InitContainers {
			buf.WriteString(fmt.Sprintf("   %s:\n", container.Name))
			buf.WriteString(fmt.Sprintf("    Image:      %s\n", container.Image))

			if len(container.Ports) > 0 {
				for _, port := range container.Ports {
					buf.WriteString(fmt.Sprintf("    Port:       %d/%s\n", port.ContainerPort, port.Protocol))
				}
			}

			if len(container.Resources.Limits) > 0 {
				buf.WriteString("    Limits:\n")
				if cpu := container.Resources.Limits.Cpu(); cpu != nil && !cpu.IsZero() {
					buf.WriteString(fmt.Sprintf("      cpu:     %s\n", cpu.String()))
				}
				if mem := container.Resources.Limits.Memory(); mem != nil && !mem.IsZero() {
					buf.WriteString(fmt.Sprintf("      memory:  %s\n", mem.String()))
				}
				if storage := container.Resources.Limits.StorageEphemeral(); storage != nil && !storage.IsZero() {
					buf.WriteString(fmt.Sprintf("      ephemeral-storage:  %s\n", storage.String()))
				}
			}

			if len(container.Resources.Requests) > 0 {
				buf.WriteString("    Requests:\n")
				if cpu := container.Resources.Requests.Cpu(); cpu != nil && !cpu.IsZero() {
					buf.WriteString(fmt.Sprintf("      cpu:     %s\n", cpu.String()))
				}
				if mem := container.Resources.Requests.Memory(); mem != nil && !mem.IsZero() {
					buf.WriteString(fmt.Sprintf("      memory:  %s\n", mem.String()))
				}
				if storage := container.Resources.Requests.StorageEphemeral(); storage != nil && !storage.IsZero() {
					buf.WriteString(fmt.Sprintf("      ephemeral-storage:  %s\n", storage.String()))
				}
			}

			if len(container.Env) > 0 {
				buf.WriteString("    Environment:\n")
				for _, env := range container.Env {
					if env.ValueFrom != nil {
						if env.ValueFrom.FieldRef != nil {
							buf.WriteString(fmt.Sprintf("      %s:   (%s)\n", env.Name, env.ValueFrom.FieldRef.FieldPath))
						} else if env.ValueFrom.SecretKeyRef != nil {
							buf.WriteString(fmt.Sprintf("      %s:  <set to the key '%s' in secret '%s'>\n",
								env.Name, env.ValueFrom.SecretKeyRef.Key, env.ValueFrom.SecretKeyRef.Name))
						} else if env.ValueFrom.ConfigMapKeyRef != nil {
							buf.WriteString(fmt.Sprintf("      %s:  <set to the key '%s' in config map '%s'>\n",
								env.Name, env.ValueFrom.ConfigMapKeyRef.Key, env.ValueFrom.ConfigMapKeyRef.Name))
						} else {
							buf.WriteString(fmt.Sprintf("      %s:  <set from source>\n", env.Name))
						}
					} else {
						buf.WriteString(fmt.Sprintf("      %s:  %s\n", env.Name, env.Value))
					}
				}
			}

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

	// Containers
	buf.WriteString("  Containers:\n")
	for _, container := range statefulSet.Spec.Template.Spec.Containers {
		buf.WriteString(fmt.Sprintf("   %s:\n", container.Name))
		buf.WriteString(fmt.Sprintf("    Image:      %s\n", container.Image))

		if container.ImagePullPolicy != "" {
			buf.WriteString(fmt.Sprintf("    Image Pull Policy:  %s\n", container.ImagePullPolicy))
		}

		if len(container.Ports) > 0 {
			for _, port := range container.Ports {
				buf.WriteString(fmt.Sprintf("    Port:       %d/%s\n", port.ContainerPort, port.Protocol))
				if port.Name != "" {
					buf.WriteString(fmt.Sprintf("    Port Name:  %s\n", port.Name))
				}
			}
		}

		if len(container.Resources.Limits) > 0 {
			buf.WriteString("    Limits:\n")
			if cpu := container.Resources.Limits.Cpu(); cpu != nil && !cpu.IsZero() {
				buf.WriteString(fmt.Sprintf("      cpu:     %s\n", cpu.String()))
			}
			if mem := container.Resources.Limits.Memory(); mem != nil && !mem.IsZero() {
				buf.WriteString(fmt.Sprintf("      memory:  %s\n", mem.String()))
			}
			if storage := container.Resources.Limits.StorageEphemeral(); storage != nil && !storage.IsZero() {
				buf.WriteString(fmt.Sprintf("      ephemeral-storage:  %s\n", storage.String()))
			}
		}

		if len(container.Resources.Requests) > 0 {
			buf.WriteString("    Requests:\n")
			if cpu := container.Resources.Requests.Cpu(); cpu != nil && !cpu.IsZero() {
				buf.WriteString(fmt.Sprintf("      cpu:     %s\n", cpu.String()))
			}
			if mem := container.Resources.Requests.Memory(); mem != nil && !mem.IsZero() {
				buf.WriteString(fmt.Sprintf("      memory:  %s\n", mem.String()))
			}
			if storage := container.Resources.Requests.StorageEphemeral(); storage != nil && !storage.IsZero() {
				buf.WriteString(fmt.Sprintf("      ephemeral-storage:  %s\n", storage.String()))
			}
		}

		if container.LivenessProbe != nil {
			buf.WriteString(fmt.Sprintf("    Liveness:   %s\n", s.formatProbeForDescribe(container.LivenessProbe)))
		}
		if container.ReadinessProbe != nil {
			buf.WriteString(fmt.Sprintf("    Readiness:  %s\n", s.formatProbeForDescribe(container.ReadinessProbe)))
		}
		if container.StartupProbe != nil {
			buf.WriteString(fmt.Sprintf("    Startup:    %s\n", s.formatProbeForDescribe(container.StartupProbe)))
		}

		if len(container.Env) > 0 {
			buf.WriteString("    Environment:\n")
			for _, env := range container.Env {
				if env.ValueFrom != nil {
					if env.ValueFrom.FieldRef != nil {
						buf.WriteString(fmt.Sprintf("      %s:   (%s)\n", env.Name, env.ValueFrom.FieldRef.FieldPath))
					} else if env.ValueFrom.SecretKeyRef != nil {
						buf.WriteString(fmt.Sprintf("      %s:  <set to the key '%s' in secret '%s'>\n",
							env.Name, env.ValueFrom.SecretKeyRef.Key, env.ValueFrom.SecretKeyRef.Name))
					} else if env.ValueFrom.ConfigMapKeyRef != nil {
						buf.WriteString(fmt.Sprintf("      %s:  <set to the key '%s' in config map '%s'>\n",
							env.Name, env.ValueFrom.ConfigMapKeyRef.Key, env.ValueFrom.ConfigMapKeyRef.Name))
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

	buf.WriteString("  Volumes:\n")
	if len(statefulSet.Spec.Template.Spec.Volumes) > 0 {
		for _, vol := range statefulSet.Spec.Template.Spec.Volumes {
			buf.WriteString(fmt.Sprintf("   %s:\n", vol.Name))
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
				buf.WriteString("    Type:       Projected (a volume that contains injected data from multiple sources)\n")
			}
		}
	} else {
		buf.WriteString("   <none>\n")
	}

	if len(statefulSet.Spec.VolumeClaimTemplates) > 0 {
		buf.WriteString("Volume Claim Templates:\n")
		for _, vct := range statefulSet.Spec.VolumeClaimTemplates {
			buf.WriteString(fmt.Sprintf("  Name:          %s\n", vct.Name))

			if vct.Spec.StorageClassName != nil && *vct.Spec.StorageClassName != "" {
				buf.WriteString(fmt.Sprintf("  StorageClass:  %s\n", *vct.Spec.StorageClassName))
			} else {
				buf.WriteString("  StorageClass:  <default>\n")
			}

			if len(vct.Labels) > 0 {
				buf.WriteString("  Labels:        ")
				first := true
				for k, v := range vct.Labels {
					if !first {
						buf.WriteString("                 ")
					}
					buf.WriteString(fmt.Sprintf("%s=%s\n", k, v))
					first = false
				}
			}

			if len(vct.Spec.AccessModes) > 0 {
				buf.WriteString("  Access Modes:  ")
				for i, mode := range vct.Spec.AccessModes {
					if i > 0 {
						buf.WriteString(", ")
					}
					buf.WriteString(string(mode))
				}
				buf.WriteString("\n")
			}

			buf.WriteString("  Requests:\n")
			if storage := vct.Spec.Resources.Requests.Storage(); storage != nil && !storage.IsZero() {
				buf.WriteString(fmt.Sprintf("    storage:  %s\n", storage.String()))
			} else {
				buf.WriteString("    storage:  <none>\n")
			}
		}
	}

	// ========== Conditions ==========
	if len(statefulSet.Status.Conditions) > 0 {
		buf.WriteString("Conditions:\n")
		buf.WriteString("  Type           Status  Reason\n")
		buf.WriteString("  ----           ------  ------\n")
		for _, cond := range statefulSet.Status.Conditions {
			buf.WriteString(fmt.Sprintf("  %-14s %-7s %s\n",
				cond.Type, cond.Status, cond.Reason))
		}
	}

	// ========== Events ==========
	buf.WriteString("Events:\n")
	events, err := s.GetEvents(namespace, name)
	if err == nil && len(events) > 0 {
		buf.WriteString("  Type    Reason          Age                From                      Message\n")
		buf.WriteString("  ----    ------          ----               ----                      -------\n")

		limit := 10
		if len(events) < limit {
			limit = len(events)
		}

		for i := 0; i < limit; i++ {
			event := events[i]

			var ageStr string
			if event.LastTimestamp > 0 {
				age := time.Since(time.UnixMilli(event.LastTimestamp)).Round(time.Second)
				ageStr = s.formatDurationForDescribe(age)
			} else {
				ageStr = "<unknown>"
			}

			buf.WriteString(fmt.Sprintf("  %-7s %-15s %-18s %-25s %s\n",
				event.Type, event.Reason, ageStr, event.Source, event.Message))
		}
	} else {
		buf.WriteString("  <none>\n")
	}

	return buf.String(), nil
}

func (s *statefulSetOperator) formatProbeForDescribe(probe *corev1.Probe) string {
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
		parts = append(parts, "exec")
	}

	parts = append(parts, fmt.Sprintf("delay=%ds", probe.InitialDelaySeconds))
	parts = append(parts, fmt.Sprintf("timeout=%ds", probe.TimeoutSeconds))
	parts = append(parts, fmt.Sprintf("period=%ds", probe.PeriodSeconds))

	return strings.Join(parts, " ")
}

func (s *statefulSetOperator) formatDurationForDescribe(duration time.Duration) string {
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

// ListAll 获取所有 StatefulSet（优先使用 informer）
func (s *statefulSetOperator) ListAll(namespace string) ([]appsv1.StatefulSet, error) {
	var statefulSets []*appsv1.StatefulSet
	var err error

	// 优先使用 informer
	if s.useInformer && s.statefulSetLister != nil {
		if namespace == "" {
			// 获取所有 namespace 的 StatefulSet
			statefulSets, err = s.statefulSetLister.List(labels.Everything())
		} else {
			// 获取指定 namespace 的 StatefulSet
			statefulSets, err = s.statefulSetLister.StatefulSets(namespace).List(labels.Everything())
		}

		if err != nil {
			// informer 失败，降级到 API 调用
			return s.listAllFromAPI(namespace)
		}
	} else {
		// 直接使用 API 调用
		return s.listAllFromAPI(namespace)
	}

	// 转换为非指针切片
	result := make([]appsv1.StatefulSet, 0, len(statefulSets))
	for _, statefulSet := range statefulSets {
		if statefulSet != nil {
			result = append(result, *statefulSet)
		}
	}

	return result, nil
}

// listAllFromAPI 通过 API 直接获取 StatefulSet 列表（内部辅助方法）
func (s *statefulSetOperator) listAllFromAPI(namespace string) ([]appsv1.StatefulSet, error) {
	statefulSetList, err := s.client.AppsV1().StatefulSets(namespace).List(s.ctx, metav1.ListOptions{})
	if err != nil {
		if namespace == "" {
			return nil, fmt.Errorf("获取所有StatefulSet失败: %v", err)
		}
		return nil, fmt.Errorf("获取命名空间 %s 的StatefulSet失败: %v", namespace, err)
	}

	return statefulSetList.Items, nil
}

// GetResourceSummary 获取 StatefulSet 的资源摘要信息
func (s *statefulSetOperator) GetResourceSummary(
	namespace string,
	name string,
	domainSuffix string,
	nodeLb []string,
	podOp types.PodOperator,
	svcOp types.ServiceOperator,
	ingressOp types.IngressOperator,
) (*types.WorkloadResourceSummary, error) {
	if namespace == "" || name == "" {
		return nil, fmt.Errorf("命名空间和名称不能为空")
	}

	// 1. 获取 StatefulSet
	statefulSet, err := s.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	// 获取 Pod 选择器标签
	selectorLabels := statefulSet.Spec.Selector.MatchLabels
	if len(selectorLabels) == 0 {
		return nil, fmt.Errorf("StatefulSet 没有选择器标签")
	}

	// 使用通用辅助函数获取摘要
	return getWorkloadResourceSummary(
		namespace,
		selectorLabels,
		domainSuffix,
		nodeLb,
		podOp,
		svcOp,
		ingressOp,
	)
}
