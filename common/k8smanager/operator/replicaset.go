package operator

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/yanshicheng/kube-nova/common/k8smanager/types"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
)

// replicaSetOperator ReplicaSet 操作器实现
type replicaSetOperator struct {
	BaseOperator
	client             kubernetes.Interface
	informerFactory    informers.SharedInformerFactory
	replicaSetLister   v1.ReplicaSetLister
	replicaSetInformer cache.SharedIndexInformer
}

// NewReplicaSetOperator 创建 ReplicaSet 操作器
func NewReplicaSetOperator(ctx context.Context, client kubernetes.Interface) types.ReplicaSetOperator {
	return &replicaSetOperator{
		BaseOperator: NewBaseOperator(ctx, false),
		client:       client,
	}
}

// NewReplicaSetOperatorWithInformer 创建带 Informer 的 ReplicaSet 操作器
func NewReplicaSetOperatorWithInformer(
	ctx context.Context,
	client kubernetes.Interface,
	informerFactory informers.SharedInformerFactory,
) types.ReplicaSetOperator {
	var replicaSetLister v1.ReplicaSetLister
	var replicaSetInformer cache.SharedIndexInformer

	if informerFactory != nil {
		replicaSetInformer = informerFactory.Apps().V1().ReplicaSets().Informer()
		replicaSetLister = informerFactory.Apps().V1().ReplicaSets().Lister()
	}

	return &replicaSetOperator{
		BaseOperator:       NewBaseOperator(ctx, informerFactory != nil),
		client:             client,
		informerFactory:    informerFactory,
		replicaSetLister:   replicaSetLister,
		replicaSetInformer: replicaSetInformer,
	}
}

// Create 创建 ReplicaSet
func (r *replicaSetOperator) Create(replicaSet *appsv1.ReplicaSet) (*appsv1.ReplicaSet, error) {
	if replicaSet == nil || replicaSet.Name == "" || replicaSet.Namespace == "" {
		r.log.Error("创建ReplicaSet失败：参数不完整")
		return nil, fmt.Errorf("ReplicaSet对象、名称和命名空间不能为空")
	}

	if replicaSet.Labels == nil {
		replicaSet.Labels = make(map[string]string)
	}
	if replicaSet.Annotations == nil {
		replicaSet.Annotations = make(map[string]string)
	}

	r.log.Infof("开始创建ReplicaSet: %s/%s", replicaSet.Namespace, replicaSet.Name)

	created, err := r.client.AppsV1().ReplicaSets(replicaSet.Namespace).Create(r.ctx, replicaSet, metav1.CreateOptions{})
	if err != nil {
		r.log.Errorf("创建ReplicaSet失败: %v", err)
		return nil, fmt.Errorf("创建ReplicaSet失败: %v", err)
	}

	r.log.Infof("ReplicaSet创建成功: %s/%s", created.Namespace, created.Name)
	return created, nil
}

// Get 获取 ReplicaSet
func (r *replicaSetOperator) Get(namespace, name string) (*appsv1.ReplicaSet, error) {
	if namespace == "" || name == "" {
		return nil, fmt.Errorf("命名空间和名称不能为空")
	}

	r.log.Infof("获取ReplicaSet: %s/%s", namespace, name)

	if r.replicaSetLister != nil {
		replicaSet, err := r.replicaSetLister.ReplicaSets(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("ReplicaSet %s/%s 不存在", namespace, name)
			}
			replicaSet, apiErr := r.client.AppsV1().ReplicaSets(namespace).Get(r.ctx, name, metav1.GetOptions{})
			if apiErr != nil {
				return nil, fmt.Errorf("获取ReplicaSet失败")
			}
			return replicaSet, nil
		}
		return replicaSet, nil
	}

	replicaSet, err := r.client.AppsV1().ReplicaSets(namespace).Get(r.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("ReplicaSet %s/%s 不存在", namespace, name)
		}
		return nil, fmt.Errorf("获取ReplicaSet失败")
	}

	return replicaSet, nil
}

// Update 更新 ReplicaSet
func (r *replicaSetOperator) Update(replicaSet *appsv1.ReplicaSet) (*appsv1.ReplicaSet, error) {
	if replicaSet == nil || replicaSet.Name == "" || replicaSet.Namespace == "" {
		return nil, fmt.Errorf("ReplicaSet对象、名称和命名空间不能为空")
	}

	r.log.Infof("更新ReplicaSet: %s/%s", replicaSet.Namespace, replicaSet.Name)

	updated, err := r.client.AppsV1().ReplicaSets(replicaSet.Namespace).Update(r.ctx, replicaSet, metav1.UpdateOptions{})
	if err != nil {
		r.log.Errorf("更新ReplicaSet失败: %v", err)
		return nil, fmt.Errorf("更新ReplicaSet失败")
	}

	r.log.Infof("ReplicaSet更新成功: %s/%s", updated.Namespace, updated.Name)
	return updated, nil
}

// Delete 删除 ReplicaSet
func (r *replicaSetOperator) Delete(namespace, name string) error {
	if namespace == "" || name == "" {
		return fmt.Errorf("命名空间和名称不能为空")
	}

	r.log.Infof("删除ReplicaSet: %s/%s", namespace, name)

	err := r.client.AppsV1().ReplicaSets(namespace).Delete(r.ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		r.log.Errorf("删除ReplicaSet失败: %v", err)
		return fmt.Errorf("删除ReplicaSet失败")
	}

	r.log.Infof("ReplicaSet删除成功: %s/%s", namespace, name)
	return nil
}

// List 列出 ReplicaSet
func (r *replicaSetOperator) List(namespace string, req types.ListRequest) (*types.ListReplicaSetResponse, error) {
	r.log.Infof("列出ReplicaSet: namespace=%s", namespace)

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

	var replicaSets []*appsv1.ReplicaSet
	var err error

	if r.useInformer && r.replicaSetLister != nil {
		replicaSets, err = r.replicaSetLister.ReplicaSets(namespace).List(selector)
		if err != nil {
			return nil, fmt.Errorf("获取ReplicaSet列表失败")
		}
	} else {
		listOpts := metav1.ListOptions{LabelSelector: selector.String()}
		replicaSetList, err := r.client.AppsV1().ReplicaSets(namespace).List(r.ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("获取ReplicaSet列表失败")
		}
		replicaSets = make([]*appsv1.ReplicaSet, len(replicaSetList.Items))
		for i := range replicaSetList.Items {
			replicaSets[i] = &replicaSetList.Items[i]
		}
	}

	if req.Search != "" {
		filtered := make([]*appsv1.ReplicaSet, 0)
		searchLower := strings.ToLower(req.Search)
		for _, rs := range replicaSets {
			if strings.Contains(strings.ToLower(rs.Name), searchLower) {
				filtered = append(filtered, rs)
			}
		}
		replicaSets = filtered
	}

	sort.Slice(replicaSets, func(i, j int) bool {
		var less bool
		switch req.SortBy {
		case "creationTime", "creationTimestamp":
			less = replicaSets[i].CreationTimestamp.Before(&replicaSets[j].CreationTimestamp)
		default:
			less = replicaSets[i].Name < replicaSets[j].Name
		}
		if req.SortDesc {
			return !less
		}
		return less
	})

	total := len(replicaSets)
	totalPages := (total + req.PageSize - 1) / req.PageSize
	start := (req.Page - 1) * req.PageSize
	end := start + req.PageSize

	if start >= total {
		return &types.ListReplicaSetResponse{
			ListResponse: types.ListResponse{
				Total:      total,
				Page:       req.Page,
				PageSize:   req.PageSize,
				TotalPages: totalPages,
			},
			Items: []types.ReplicaSetInfo{},
		}, nil
	}

	if end > total {
		end = total
	}

	pageReplicaSets := replicaSets[start:end]
	items := make([]types.ReplicaSetInfo, len(pageReplicaSets))
	for i, rs := range pageReplicaSets {
		items[i] = r.convertToReplicaSetInfo(rs)
	}

	return &types.ListReplicaSetResponse{
		ListResponse: types.ListResponse{
			Total:      total,
			Page:       req.Page,
			PageSize:   req.PageSize,
			TotalPages: totalPages,
		},
		Items: items,
	}, nil
}

func (r *replicaSetOperator) convertToReplicaSetInfo(rs *appsv1.ReplicaSet) types.ReplicaSetInfo {
	images := make([]string, 0)
	for _, container := range rs.Spec.Template.Spec.Containers {
		images = append(images, container.Image)
	}

	replicas := int32(0)
	if rs.Spec.Replicas != nil {
		replicas = *rs.Spec.Replicas
	}

	// 收集所属控制器信息
	owners := make([]string, 0)
	for _, owner := range rs.OwnerReferences {
		owners = append(owners, fmt.Sprintf("%s/%s", owner.Kind, owner.Name))
	}

	return types.ReplicaSetInfo{
		Name:              rs.Name,
		Namespace:         rs.Namespace,
		Replicas:          replicas,
		ReadyReplicas:     rs.Status.ReadyReplicas,
		AvailableReplicas: rs.Status.AvailableReplicas,
		CreationTimestamp: rs.CreationTimestamp.Time,
		Images:            images,
		OwnerReferences:   owners,
	}
}

// Watch 监视 ReplicaSet
func (r *replicaSetOperator) Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return r.client.AppsV1().ReplicaSets(namespace).Watch(r.ctx, opts)
}

// UpdateLabels 更新标签
func (r *replicaSetOperator) UpdateLabels(namespace, name string, labels map[string]string) error {
	replicaSet, err := r.Get(namespace, name)
	if err != nil {
		return err
	}

	if replicaSet.Labels == nil {
		replicaSet.Labels = make(map[string]string)
	}
	for k, v := range labels {
		replicaSet.Labels[k] = v
	}

	_, err = r.Update(replicaSet)
	return err
}

// UpdateAnnotations 更新注解
func (r *replicaSetOperator) UpdateAnnotations(namespace, name string, annotations map[string]string) error {
	replicaSet, err := r.Get(namespace, name)
	if err != nil {
		return err
	}

	if replicaSet.Annotations == nil {
		replicaSet.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		replicaSet.Annotations[k] = v
	}

	_, err = r.Update(replicaSet)
	return err
}

// Scale 扩缩容
func (r *replicaSetOperator) Scale(namespace, name string, replicas int32) error {
	r.log.Infof("扩缩容ReplicaSet: %s/%s, replicas=%d", namespace, name, replicas)

	replicaSet, err := r.Get(namespace, name)
	if err != nil {
		return err
	}

	replicaSet.Spec.Replicas = &replicas
	_, err = r.Update(replicaSet)
	if err != nil {
		return err
	}

	r.log.Infof("ReplicaSet扩缩容成功: %s/%s", namespace, name)
	return nil
}

// GetEvents 获取 ReplicaSet 相关的事件
func (r *replicaSetOperator) GetEvents(namespace, name string) ([]types.EventInfo, error) {
	if namespace == "" || name == "" {
		return nil, fmt.Errorf("命名空间和名称不能为空")
	}

	r.log.Infof("获取ReplicaSet事件: %s/%s", namespace, name)

	// 获取 ReplicaSet
	replicaSet, err := r.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	// 获取事件列表
	eventList, err := r.client.CoreV1().Events(namespace).List(r.ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=ReplicaSet,involvedObject.uid=%s", name, replicaSet.UID),
	})
	if err != nil {
		r.log.Errorf("获取事件列表失败: %v", err)
		return nil, fmt.Errorf("获取事件列表失败")
	}

	// 转换为 EventInfo
	events := make([]types.EventInfo, 0, len(eventList.Items))
	for _, event := range eventList.Items {
		events = append(events, types.EventInfo{
			Type:               event.Type,
			Reason:             event.Reason,
			Message:            event.Message,
			Count:              event.Count,
			FirstTimestamp:     event.FirstTimestamp.Time.Unix(),
			LastTimestamp:      event.LastTimestamp.Time.Unix(),
			Source:             event.Source.Component,
			InvolvedObjectKind: event.InvolvedObject.Kind,
			InvolvedObjectName: event.InvolvedObject.Name,
		})
	}

	// 按最后时间排序（降序）
	//sort.Slice(events, func(i, j int) bool {
	//	return events[i].LastTimestamp.After(events[j].LastTimestamp)
	//})
	// 按最后时间排序（降序）
	sort.Slice(events, func(i, j int) bool {
		return events[i].LastTimestamp > events[j].LastTimestamp
	})
	r.log.Infof("获取到 %d 个事件: %s/%s", len(events), namespace, name)

	return events, nil
}
