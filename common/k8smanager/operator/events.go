package operator

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

type eventOperator struct {
	BaseOperator
	client          kubernetes.Interface
	informerFactory informers.SharedInformerFactory
	eventLister     corelisters.EventLister
	eventInformer   cache.SharedIndexInformer
}

func NewEventOperator(ctx context.Context, client kubernetes.Interface) types.EventOperator {
	return &eventOperator{
		BaseOperator: NewBaseOperator(ctx, false),
		client:       client,
	}
}

func NewEventOperatorWithInformer(
	ctx context.Context,
	client kubernetes.Interface,
	informerFactory informers.SharedInformerFactory,
) types.EventOperator {
	var eventLister corelisters.EventLister
	var eventInformer cache.SharedIndexInformer

	if informerFactory != nil {
		eventInformer = informerFactory.Core().V1().Events().Informer()
		eventLister = informerFactory.Core().V1().Events().Lister()
	}

	return &eventOperator{
		BaseOperator:    NewBaseOperator(ctx, informerFactory != nil),
		client:          client,
		informerFactory: informerFactory,
		eventLister:     eventLister,
		eventInformer:   eventInformer,
	}
}

// normalizeKind 标准化 Kubernetes Kind 名称
// 将常见的资源类型转换为标准的 Kubernetes Kind（首字母大写）
func normalizeKind(kind string) string {
	if kind == "" {
		return ""
	}

	// 转换为小写进行匹配
	lowerKind := strings.ToLower(kind)

	// 常见资源类型映射
	kindMap := map[string]string{
		"pod":                    "Pod",
		"pods":                   "Pod",
		"deployment":             "Deployment",
		"deployments":            "Deployment",
		"statefulset":            "StatefulSet",
		"statefulsets":           "StatefulSet",
		"daemonset":              "DaemonSet",
		"daemonsets":             "DaemonSet",
		"replicaset":             "ReplicaSet",
		"replicasets":            "ReplicaSet",
		"job":                    "Job",
		"jobs":                   "Job",
		"cronjob":                "CronJob",
		"cronjobs":               "CronJob",
		"service":                "Service",
		"services":               "Service",
		"configmap":              "ConfigMap",
		"configmaps":             "ConfigMap",
		"secret":                 "Secret",
		"secrets":                "Secret",
		"persistentvolumeclaim":  "PersistentVolumeClaim",
		"persistentvolumeclaims": "PersistentVolumeClaim",
		"pvc":                    "PersistentVolumeClaim",
		"pvcs":                   "PersistentVolumeClaim",
		"node":                   "Node",
		"nodes":                  "Node",
		"namespace":              "Namespace",
		"namespaces":             "Namespace",
		"ingress":                "Ingress",
		"ingresses":              "Ingress",
		"endpoints":              "Endpoints",
		"serviceaccount":         "ServiceAccount",
		"serviceaccounts":        "ServiceAccount",
		"replicationcontroller":  "ReplicationController",
		"replicationcontrollers": "ReplicationController",
	}

	// 查找映射
	if standardKind, exists := kindMap[lowerKind]; exists {
		return standardKind
	}

	// 如果没有找到映射，则将首字母大写
	// 这样可以处理一些不在映射表中的资源类型
	if len(kind) > 0 {
		return strings.ToUpper(kind[:1]) + kind[1:]
	}

	return kind
}

// GetPodEvents 获取 Pod 的事件（带分页）
func (e *eventOperator) GetPodEvents(namespace, podName string, page, pageSize int) (*types.EventsListResponse, error) {
	if namespace == "" || podName == "" {
		return nil, fmt.Errorf("命名空间和Pod名称不能为空")
	}

	// 参数校验
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	// 先尝试从 API 获取 Pod，确保 UID 正确
	pod, err := e.client.CoreV1().Pods(namespace).Get(e.ctx, podName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("Pod %s/%s 不存在", namespace, podName)
		}
		return nil, fmt.Errorf("获取Pod失败: %v", err)
	}

	return e.GetResourceEvents(namespace, "Pod", podName, string(pod.UID), page, pageSize)
}

// GetNamespaceEvents 获取命名空间的所有事件（带分页）
func (e *eventOperator) GetNamespaceEvents(namespace string, page, pageSize int) (*types.EventsListResponse, error) {
	if namespace == "" {
		return nil, fmt.Errorf("命名空间不能为空")
	}

	// 参数校验
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	var events []*corev1.Event
	var err error

	if e.useInformer && e.eventLister != nil {
		events, err = e.eventLister.Events(namespace).List(nil)
		if err != nil {
			// 如果 Informer 失败，fallback 到 API
			eventList, apiErr := e.client.CoreV1().Events(namespace).List(e.ctx, metav1.ListOptions{})
			if apiErr != nil {
				return nil, fmt.Errorf("获取命名空间事件失败: %v", apiErr)
			}
			events = make([]*corev1.Event, len(eventList.Items))
			for i := range eventList.Items {
				events[i] = &eventList.Items[i]
			}
		}
	} else {
		eventList, err := e.client.CoreV1().Events(namespace).List(e.ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("获取命名空间事件失败: %v", err)
		}
		events = make([]*corev1.Event, len(eventList.Items))
		for i := range eventList.Items {
			events[i] = &eventList.Items[i]
		}
	}

	return e.convertSortAndPaginate(events, page, pageSize), nil
}

// GetResourceEvents 获取特定资源的事件（带分页）- 包含相关资源
func (e *eventOperator) GetResourceEvents(namespace, resourceKind, resourceName, resourceUID string, page, pageSize int) (*types.EventsListResponse, error) {
	if namespace == "" || resourceKind == "" || resourceName == "" {
		return nil, fmt.Errorf("命名空间、资源类型和资源名称不能为空")
	}

	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	return e.QueryEvents(types.EventsQueryRequest{
		Namespace:          namespace,
		InvolvedObjectKind: resourceKind,
		InvolvedObjectName: resourceName,
		InvolvedObjectUID:  resourceUID,
		Page:               page,
		PageSize:           pageSize,
	})
}

// getRelatedResourceUIDs 获取与指定资源相关的所有资源 UID
// 这包括资源本身、它的子资源（通过 labels 或 ownerReferences）
func (e *eventOperator) getRelatedResourceUIDs(namespace, resourceKind, resourceName string) (map[string]bool, error) {
	relatedUIDs := make(map[string]bool)
	standardKind := normalizeKind(resourceKind)

	fmt.Printf("\n查找相关资源 UIDs: Kind=%s, Name=%s, Namespace=%s\n", standardKind, resourceName, namespace)

	switch standardKind {
	case "Deployment":
		// 1. 获取 Deployment 本身
		deployment, err := e.client.AppsV1().Deployments(namespace).Get(e.ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			if !errors.IsNotFound(err) {
				return nil, fmt.Errorf("获取 Deployment 失败: %v", err)
			}
			fmt.Printf(" Deployment 不存在: %s\n", resourceName)
		} else {
			relatedUIDs[string(deployment.UID)] = true

			// 2. 获取 Deployment 管理的 ReplicaSets
			selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
			if err == nil {
				replicaSets, err := e.client.AppsV1().ReplicaSets(namespace).List(e.ctx, metav1.ListOptions{
					LabelSelector: selector.String(),
				})
				if err == nil {
					fmt.Printf(" 找到 %d 个 ReplicaSet\n", len(replicaSets.Items))
					for _, rs := range replicaSets.Items {
						// 检查是否由此 Deployment 拥有
						if isOwnedBy(&rs.ObjectMeta, "Deployment", deployment.Name) {
							relatedUIDs[string(rs.UID)] = true
							fmt.Printf("    - ReplicaSet: %s (UID: %s)\n", rs.Name, rs.UID)
						}
					}
				}
			}

			// 3. 获取所有相关的 Pods
			pods, err := e.client.CoreV1().Pods(namespace).List(e.ctx, metav1.ListOptions{
				LabelSelector: selector.String(),
			})
			if err == nil {
				fmt.Printf("找到 %d 个 Pod\n", len(pods.Items))
				for _, pod := range pods.Items {
					relatedUIDs[string(pod.UID)] = true
					fmt.Printf("    - Pod: %s (UID: %s)\n", pod.Name, pod.UID)
				}
			}
		}

	case "StatefulSet":
		// 1. 获取 StatefulSet 本身
		statefulSet, err := e.client.AppsV1().StatefulSets(namespace).Get(e.ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			if !errors.IsNotFound(err) {
				return nil, fmt.Errorf("获取 StatefulSet 失败: %v", err)
			}
			fmt.Printf("  StatefulSet 不存在: %s\n", resourceName)
		} else {
			relatedUIDs[string(statefulSet.UID)] = true
			fmt.Printf(" StatefulSet UID: %s\n", statefulSet.UID)

			// 2. 获取 StatefulSet 管理的 Pods
			selector, err := metav1.LabelSelectorAsSelector(statefulSet.Spec.Selector)
			if err == nil {
				pods, err := e.client.CoreV1().Pods(namespace).List(e.ctx, metav1.ListOptions{
					LabelSelector: selector.String(),
				})
				if err == nil {
					fmt.Printf("找到 %d 个 Pod\n", len(pods.Items))
					for _, pod := range pods.Items {
						relatedUIDs[string(pod.UID)] = true
						fmt.Printf("    - Pod: %s (UID: %s)\n", pod.Name, pod.UID)
					}
				}
			}
		}

	case "DaemonSet":
		// 1. 获取 DaemonSet 本身
		daemonSet, err := e.client.AppsV1().DaemonSets(namespace).Get(e.ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			if !errors.IsNotFound(err) {
				return nil, fmt.Errorf("获取 DaemonSet 失败: %v", err)
			}
		} else {
			relatedUIDs[string(daemonSet.UID)] = true

			// 2. 获取 DaemonSet 管理的 Pods
			selector, err := metav1.LabelSelectorAsSelector(daemonSet.Spec.Selector)
			if err == nil {
				pods, err := e.client.CoreV1().Pods(namespace).List(e.ctx, metav1.ListOptions{
					LabelSelector: selector.String(),
				})
				if err == nil {
					for _, pod := range pods.Items {
						relatedUIDs[string(pod.UID)] = true
						fmt.Printf("    - Pod: %s (UID: %s)\n", pod.Name, pod.UID)
					}
				}
			}
		}

	case "Job":
		// 1. 获取 Job 本身
		job, err := e.client.BatchV1().Jobs(namespace).Get(e.ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			if !errors.IsNotFound(err) {
				return nil, fmt.Errorf("获取 Job 失败: %v", err)
			}
		} else {
			relatedUIDs[string(job.UID)] = true

			// 2. 获取 Job 管理的 Pods
			selector, err := metav1.LabelSelectorAsSelector(job.Spec.Selector)
			if err == nil {
				pods, err := e.client.CoreV1().Pods(namespace).List(e.ctx, metav1.ListOptions{
					LabelSelector: selector.String(),
				})
				if err == nil {
					for _, pod := range pods.Items {
						relatedUIDs[string(pod.UID)] = true
						fmt.Printf("    - Pod: %s (UID: %s)\n", pod.Name, pod.UID)
					}
				}
			}
		}

	case "CronJob":
		// 1. 获取 CronJob 本身
		cronJob, err := e.client.BatchV1().CronJobs(namespace).Get(e.ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			if !errors.IsNotFound(err) {
				return nil, fmt.Errorf("获取 CronJob 失败: %v", err)
			}
		} else {
			relatedUIDs[string(cronJob.UID)] = true

			// 2. 获取 CronJob 创建的 Jobs
			jobs, err := e.client.BatchV1().Jobs(namespace).List(e.ctx, metav1.ListOptions{})
			if err == nil {
				for _, job := range jobs.Items {
					if isOwnedBy(&job.ObjectMeta, "CronJob", cronJob.Name) {
						relatedUIDs[string(job.UID)] = true
						fmt.Printf("  Job: %s (UID: %s)\n", job.Name, job.UID)

						// 3. 获取每个 Job 的 Pods
						if job.Spec.Selector != nil {
							selector, err := metav1.LabelSelectorAsSelector(job.Spec.Selector)
							if err == nil {
								pods, err := e.client.CoreV1().Pods(namespace).List(e.ctx, metav1.ListOptions{
									LabelSelector: selector.String(),
								})
								if err == nil {
									for _, pod := range pods.Items {
										relatedUIDs[string(pod.UID)] = true
										fmt.Printf("    - Pod: %s (UID: %s)\n", pod.Name, pod.UID)
									}
								}
							}
						}
					}
				}
			}
		}

	case "Pod":
		// Pod 只查询自己
		pod, err := e.client.CoreV1().Pods(namespace).Get(e.ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			if !errors.IsNotFound(err) {
				return nil, fmt.Errorf("获取 Pod 失败: %v", err)
			}
		} else {
			relatedUIDs[string(pod.UID)] = true
		}

	default:
		// 其他类型的资源，尝试直接获取
	}

	return relatedUIDs, nil
}

// isOwnedBy 检查资源是否由指定的 owner 拥有
func isOwnedBy(meta *metav1.ObjectMeta, ownerKind, ownerName string) bool {
	for _, owner := range meta.OwnerReferences {
		if owner.Kind == ownerKind && owner.Name == ownerName {
			return true
		}
	}
	return false
}

// QueryEvents 根据条件查询事件（带分页）- 支持查询相关资源的事件
func (e *eventOperator) QueryEvents(req types.EventsQueryRequest) (*types.EventsListResponse, error) {
	// 参数校验
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	standardKind := normalizeKind(req.InvolvedObjectKind)
	if req.InvolvedObjectKind != "" && req.InvolvedObjectKind != standardKind {
		fmt.Printf("  - Kind 标准化: %s → %s\n", req.InvolvedObjectKind, standardKind)
	}

	var events []*corev1.Event
	var err error

	// 获取指定命名空间的所有事件
	if req.Namespace != "" {
		if e.useInformer && e.eventLister != nil {
			events, err = e.eventLister.Events(req.Namespace).List(labels.Everything())
			if err != nil {
				eventList, apiErr := e.client.CoreV1().Events(req.Namespace).List(e.ctx, metav1.ListOptions{})
				if apiErr != nil {
					return nil, fmt.Errorf("查询事件失败: %v", apiErr)
				}
				events = make([]*corev1.Event, len(eventList.Items))
				for i := range eventList.Items {
					events[i] = &eventList.Items[i]
				}
			}
		} else {
			eventList, err := e.client.CoreV1().Events(req.Namespace).List(e.ctx, metav1.ListOptions{})
			if err != nil {
				return nil, fmt.Errorf("查询事件失败: %v", err)
			}
			events = make([]*corev1.Event, len(eventList.Items))
			for i := range eventList.Items {
				events[i] = &eventList.Items[i]
			}
		}
	} else {
		// 获取所有命名空间的事件
		if e.useInformer && e.eventLister != nil {
			events, err = e.eventLister.List(labels.Everything())
			if err != nil {
				eventList, apiErr := e.client.CoreV1().Events("").List(e.ctx, metav1.ListOptions{})
				if apiErr != nil {
					return nil, fmt.Errorf("查询事件失败: %v", apiErr)
				}
				events = make([]*corev1.Event, len(eventList.Items))
				for i := range eventList.Items {
					events[i] = &eventList.Items[i]
				}
			}
		} else {
			eventList, err := e.client.CoreV1().Events("").List(e.ctx, metav1.ListOptions{})
			if err != nil {
				return nil, fmt.Errorf("查询事件失败: %v", err)
			}
			events = make([]*corev1.Event, len(eventList.Items))
			for i := range eventList.Items {
				events[i] = &eventList.Items[i]
			}
		}
	}

	fmt.Printf("从 K8s 获取到的事件总数: %d\n", len(events))

	// 如果指定了 InvolvedObjectKind 和 InvolvedObjectName，获取相关资源的 UIDs
	var relatedUIDs map[string]bool
	if standardKind != "" && req.InvolvedObjectName != "" && req.Namespace != "" {
		relatedUIDs, err = e.getRelatedResourceUIDs(req.Namespace, standardKind, req.InvolvedObjectName)
		if err != nil {
			fmt.Printf("获取相关资源 UID 失败: %v，将使用名称过滤\n", err)
			relatedUIDs = nil
		}
	}

	// 根据条件过滤
	filteredEvents := make([]*corev1.Event, 0)
	filterStats := make(map[string]int)
	filterStats["total"] = len(events)

	for _, event := range events {
		if relatedUIDs != nil && len(relatedUIDs) > 0 {
			if !relatedUIDs[string(event.InvolvedObject.UID)] {
				filterStats["filtered_by_uid"]++
				continue
			}
		} else {
			// 旧逻辑：按 Kind 和 Name 过滤
			if standardKind != "" && event.InvolvedObject.Kind != standardKind {
				filterStats["filtered_by_kind"]++
				continue
			}

			if req.InvolvedObjectName != "" && event.InvolvedObject.Name != req.InvolvedObjectName {
				filterStats["filtered_by_name"]++
				continue
			}
		}

		// 过滤 Type
		if req.Type != "" && event.Type != req.Type {
			filterStats["filtered_by_type"]++
			continue
		}

		filterStats["passed"]++
		filteredEvents = append(filteredEvents, event)
	}

	fmt.Printf("  - 被 Type 过滤: %d\n", filterStats["filtered_by_type"])

	if len(filteredEvents) > 0 {
		fmt.Printf("\n通过过滤的前5个事件:\n")
		for i := 0; i < len(filteredEvents) && i < 5; i++ {
			event := filteredEvents[i]
			fmt.Printf(" [%d] %s (%s/%s) - %s: %s\n",
				i+1,
				event.Name,
				event.InvolvedObject.Kind,
				event.InvolvedObject.Name,
				event.Reason,
				event.Message)
		}
	}

	result := e.convertSortAndPaginate(filteredEvents, req.Page, req.PageSize)

	return result, nil
}

// GetAllEvents 获取所有事件（跨命名空间，带分页）
func (e *eventOperator) GetAllEvents(page, pageSize int) (*types.EventsListResponse, error) {
	// 参数校验
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	var events []*corev1.Event
	var err error

	if e.useInformer && e.eventLister != nil {
		events, err = e.eventLister.List(nil)
		if err != nil {
			eventList, apiErr := e.client.CoreV1().Events("").List(e.ctx, metav1.ListOptions{})
			if apiErr != nil {
				return nil, fmt.Errorf("获取所有事件失败: %v", apiErr)
			}
			events = make([]*corev1.Event, len(eventList.Items))
			for i := range eventList.Items {
				events[i] = &eventList.Items[i]
			}
		}
	} else {
		eventList, err := e.client.CoreV1().Events("").List(e.ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("获取所有事件失败: %v", err)
		}
		events = make([]*corev1.Event, len(eventList.Items))
		for i := range eventList.Items {
			events[i] = &eventList.Items[i]
		}
	}

	return e.convertSortAndPaginate(events, page, pageSize), nil
}

// convertSortAndPaginate 转换、排序并分页事件（最新的在前面）
func (e *eventOperator) convertSortAndPaginate(events []*corev1.Event, page, pageSize int) *types.EventsListResponse {
	// 转换为 EventInfo
	result := make([]types.EventInfo, 0, len(events))
	for _, event := range events {
		result = append(result, types.ConvertK8sEventToEventInfo(event))
	}

	// 按最后发生时间降序排序（最新的在前面）
	sort.Slice(result, func(i, j int) bool {
		return result[i].LastTimestamp > result[j].LastTimestamp
	})

	total := len(result)
	totalPages := (total + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}

	// 计算分页
	start := (page - 1) * pageSize
	end := start + pageSize

	// 边界检查
	if start >= total {
		// 如果起始位置超出范围，返回空数据
		return &types.EventsListResponse{
			Items:      []types.EventInfo{},
			Total:      total,
			Page:       page,
			PageSize:   pageSize,
			TotalPages: totalPages,
		}
	}

	if end > total {
		end = total
	}

	pagedResult := result[start:end]

	return &types.EventsListResponse{
		Items:      pagedResult,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}
}
