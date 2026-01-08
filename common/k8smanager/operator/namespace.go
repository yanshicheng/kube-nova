package operator

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

// namespaceOperator Namespace 操作器实现
type namespaceOperator struct {
	BaseOperator // 嵌入基础操作器

	client kubernetes.Interface

	// Informer 相关
	informerFactory informers.SharedInformerFactory
	nsLister        v1.NamespaceLister
	nsInformer      cache.SharedIndexInformer
}

// NewNamespaceOperator 创建 Namespace 操作器（不使用 Informer）
func NewNamespaceOperator(ctx context.Context, client kubernetes.Interface) types.NamespaceOperator {
	return &namespaceOperator{
		BaseOperator: NewBaseOperator(ctx, false),
		client:       client,
	}
}

// NewNamespaceOperatorWithInformer 创建带 Informer 的 Namespace 操作器
func NewNamespaceOperatorWithInformer(
	ctx context.Context,
	client kubernetes.Interface,
	informerFactory informers.SharedInformerFactory,
) types.NamespaceOperator {
	var nsLister v1.NamespaceLister
	var nsInformer cache.SharedIndexInformer

	if informerFactory != nil {
		// 获取 Namespace Informer 和 Lister
		nsInformer = informerFactory.Core().V1().Namespaces().Informer()
		nsLister = informerFactory.Core().V1().Namespaces().Lister()
	}

	return &namespaceOperator{
		BaseOperator:    NewBaseOperator(ctx, informerFactory != nil),
		client:          client,
		informerFactory: informerFactory,
		nsLister:        nsLister,
		nsInformer:      nsInformer,
	}
}

// Get 获取 Namespace
func (n *namespaceOperator) Get(name string) (*corev1.Namespace, error) {
	if name == "" {
		n.log.Error("获取Namespace失败：名称不能为空")
		return nil, fmt.Errorf("Namespace名称不能为空")
	}

	n.log.Infof("开始获取Namespace: name=%s", name)

	// 优先从 informer 缓存获取
	if n.nsLister != nil {
		n.log.Debug("尝试从缓存获取Namespace")
		ns, err := n.nsLister.Get(name)
		if err != nil {
			// 如果是 NotFound 错误，直接返回
			if errors.IsNotFound(err) {
				n.log.Errorf("Namespace不存在: %s", name)
				return nil, fmt.Errorf("Namespace %s 不存在", name)
			}
			// 其他错误，尝试降级到 API
			n.log.Infof("缓存获取失败，尝试通过API获取: %v", err)

			ns, apiErr := n.client.CoreV1().Namespaces().Get(n.ctx, name, metav1.GetOptions{})
			if apiErr != nil {
				if errors.IsNotFound(apiErr) {
					n.log.Errorf("通过API确认Namespace不存在: %s", name)
					return nil, fmt.Errorf("Namespace %s 不存在", name)
				}
				n.log.Errorf("API获取Namespace失败: %s, error=%v", name, apiErr)
				return nil, fmt.Errorf("获取Namespace失败")
			}
			n.log.Infof("通过API成功获取Namespace: %s", name)
			ns.TypeMeta = metav1.TypeMeta{
				Kind:       "Namespace",
				APIVersion: "v1",
			}
			return ns, nil
		}
		ns.TypeMeta = metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		}
		return ns, nil
	} else {
		// 直接通过 API 获取
		n.log.Debug("直接通过API获取Namespace（无缓存）")
		ns, err := n.client.CoreV1().Namespaces().Get(n.ctx, name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				n.log.Errorf("Namespace不存在: %s", name)
				return nil, fmt.Errorf("namespace %s 不存在", name)
			}
			n.log.Errorf("API获取Namespace失败: %s, error=%v", name, err)
			return nil, fmt.Errorf("获取Namespace失败")
		}
		n.log.Infof("通过API成功获取Namespace: %s", name)
		ns.TypeMeta = metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		}

		return ns, nil
	}
}

// List 列出 Namespace
func (n *namespaceOperator) List(req types.ListRequest) (*types.ListNamespaceResponse, error) {
	n.log.Infof("开始列出Namespace列表: labels=%s, search=%s, sortBy=%s",
		req.Labels, req.Search, req.SortBy)

	// 设置默认值
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 100 {
		req.PageSize = 100 // 限制最大每页数量,避免一次加载过多数据
		n.log.Debug("限制每页数量为100")
	}
	if req.SortBy == "" {
		req.SortBy = "name" // 默认按名称排序
	}

	// 构建标签选择器
	var selector labels.Selector = labels.Everything()
	if req.Labels != "" {
		n.log.Debugf("解析标签选择器: %s", req.Labels)
		parsedSelector, err := labels.Parse(req.Labels)
		if err != nil {
			n.log.Errorf("解析标签选择器失败: %v", err)
			return nil, fmt.Errorf("解析标签选择器失败")
		}
		selector = parsedSelector
	}

	// 获取所有 Namespace
	var namespaces []*corev1.Namespace
	var err error

	if n.useInformer && n.nsLister != nil {
		n.log.Debug("从缓存获取Namespace列表")
		namespaces, err = n.nsLister.List(selector)
		if err != nil {
			n.log.Errorf("从缓存获取Namespace列表失败: %v", err)
			return nil, fmt.Errorf("获取Namespace列表失败")
		}
		n.log.Infof("从缓存获取到 %d 个Namespace", len(namespaces))
	} else {
		// 降级到直接调用 API
		n.log.Debug("通过API获取Namespace列表")
		listOpts := metav1.ListOptions{
			LabelSelector: selector.String(),
		}
		nsList, err := n.client.CoreV1().Namespaces().List(n.ctx, listOpts)
		if err != nil {
			n.log.Errorf("API获取Namespace列表失败: %v", err)
			return nil, fmt.Errorf("获取Namespace列表失败")
		}

		// 转换为指针切片
		namespaces = make([]*corev1.Namespace, len(nsList.Items))
		for i := range nsList.Items {
			namespaces[i] = &nsList.Items[i]
		}
		n.log.Infof("通过API获取到 %d 个Namespace", len(namespaces))
	}

	// 应用搜索过滤
	if req.Search != "" {
		n.log.Debugf("应用搜索过滤: %s", req.Search)
		filtered := make([]*corev1.Namespace, 0)
		searchLower := strings.ToLower(req.Search)

		for _, ns := range namespaces {
			// 搜索名称
			if strings.Contains(strings.ToLower(ns.Name), searchLower) {
				filtered = append(filtered, ns)
				continue
			}

			// 搜索标签
			for key, value := range ns.Labels {
				if strings.Contains(strings.ToLower(key), searchLower) ||
					strings.Contains(strings.ToLower(value), searchLower) {
					filtered = append(filtered, ns)
					break
				}
			}
		}
		namespaces = filtered
		n.log.Infof("搜索过滤后剩余 %d 个Namespace", len(namespaces))
	}

	// 排序
	n.log.Debugf("按 %s 排序，降序: %v", req.SortBy, req.SortDesc)
	sort.Slice(namespaces, func(i, j int) bool {
		var less bool
		switch req.SortBy {
		case "creationTime", "creationTimestamp":
			less = namespaces[i].CreationTimestamp.Before(&namespaces[j].CreationTimestamp)
		case "status":
			less = string(namespaces[i].Status.Phase) < string(namespaces[j].Status.Phase)
		default: // name
			less = namespaces[i].Name < namespaces[j].Name
		}

		// 如果是降序,反转结果
		if req.SortDesc {
			return !less
		}
		return less
	})

	// 计算分页信息
	total := len(namespaces)
	totalPages := (total + req.PageSize - 1) / req.PageSize
	n.log.Debugf("总数: %d, 总页数: %d, 当前页: %d", total, totalPages, req.Page)

	// 计算起始和结束索引
	start := (req.Page - 1) * req.PageSize
	end := start + req.PageSize

	// 边界检查
	if start >= total {
		// 请求的页码超出范围,返回空列表
		n.log.Infof("请求页码超出范围，返回空列表: page=%d, total=%d", req.Page, total)
		return &types.ListNamespaceResponse{
			ListResponse: types.ListResponse{
				Total:      total,
				Page:       req.Page,
				PageSize:   req.PageSize,
				TotalPages: totalPages,
			},
			Items: []types.NamespaceInfo{},
		}, nil
	}

	if end > total {
		end = total
	}

	// 获取当前页的数据
	pageNamespaces := namespaces[start:end]

	// 转换为前端需要的格式
	items := make([]types.NamespaceInfo, len(pageNamespaces))
	for i, ns := range pageNamespaces {
		items[i] = n.convertToNamespaceInfo(ns)
	}

	n.log.Infof("成功返回Namespace列表: 总数=%d, 页码=%d/%d, 返回=%d",
		total, req.Page, totalPages, len(items))

	return &types.ListNamespaceResponse{
		ListResponse: types.ListResponse{
			Total:      total,
			Page:       req.Page,
			PageSize:   req.PageSize,
			TotalPages: totalPages,
		},
		Items: items,
	}, nil
}

// convertToNamespaceInfo 将 Kubernetes Namespace 转换为 NamespaceInfo
func (n *namespaceOperator) convertToNamespaceInfo(ns *corev1.Namespace) types.NamespaceInfo {
	status := "Active"
	if ns.Status.Phase != "" {
		status = string(ns.Status.Phase)
	}

	return types.NamespaceInfo{
		Name:              ns.Name,
		Status:            status,
		CreationTimestamp: ns.CreationTimestamp.Time,
	}
}

// Create 创建 Namespace
func (n *namespaceOperator) Create(namespace *corev1.Namespace) (*corev1.Namespace, error) {
	// 参数验证
	if namespace == nil {
		n.log.Error("创建Namespace失败：对象不能为空")
		return nil, fmt.Errorf("Namespace对象不能为空")
	}
	if namespace.Name == "" {
		n.log.Error("创建Namespace失败：名称不能为空")
		return nil, fmt.Errorf("Namespace名称不能为空")
	}

	// 初始化标签和注解
	if namespace.Labels == nil {
		namespace.Labels = make(map[string]string)
		n.log.Debug("初始化空标签映射")
	}
	if namespace.Annotations == nil {
		namespace.Annotations = make(map[string]string)
		n.log.Debug("初始化空注解映射")
	}

	n.log.Infof("开始创建Namespace: %s", namespace.Name)

	// 执行创建
	createdNamespace, err := n.client.CoreV1().
		Namespaces().
		Create(n.ctx, namespace, metav1.CreateOptions{})

	if err != nil {
		n.log.Errorf("创建Namespace失败: %s, error=%v", namespace.Name, err)
		return nil, fmt.Errorf("创建Namespace失败")
	}

	n.log.Infof("Namespace创建成功: %s", createdNamespace.Name)
	return createdNamespace, nil
}

// Update 更新 Namespace
func (n *namespaceOperator) Update(namespace *corev1.Namespace) (*corev1.Namespace, error) {
	if namespace == nil {
		n.log.Error("更新Namespace失败：对象不能为空")
		return nil, fmt.Errorf("Namespace对象不能为空")
	}
	if namespace.Name == "" {
		n.log.Error("更新Namespace失败：名称不能为空")
		return nil, fmt.Errorf("Namespace名称不能为空")
	}

	n.log.Infof("开始更新Namespace: %s", namespace.Name)

	updatedNamespace, err := n.client.CoreV1().
		Namespaces().
		Update(n.ctx, namespace, metav1.UpdateOptions{})

	if err != nil {
		n.log.Errorf("更新Namespace失败: %s, error=%v", namespace.Name, err)
		return nil, fmt.Errorf("更新Namespace失败")
	}

	n.log.Infof("Namespace更新成功: %s", updatedNamespace.Name)
	return updatedNamespace, nil
}

// Delete 删除 Namespace
func (n *namespaceOperator) Delete(name string) error {
	if name == "" {
		n.log.Error("删除Namespace失败：名称不能为空")
		return fmt.Errorf("Namespace名称不能为空")
	}

	// 保护系统命名空间
	protectedNamespaces := []string{
		"kube-system",
		"kube-public",
		"kube-node-lease",
		"default",
	}

	for _, protected := range protectedNamespaces {
		if name == protected {
			n.log.Errorf("不能删除系统命名空间: %s", name)
			return fmt.Errorf("不能删除系统命名空间: %s", name)
		}
	}

	n.log.Infof("开始删除Namespace: %s", name)

	err := n.client.CoreV1().
		Namespaces().
		Delete(n.ctx, name, metav1.DeleteOptions{})

	if err != nil {
		if errors.IsNotFound(err) {
			n.log.Infof("Namespace已不存在，无需删除: %s", name)
			return nil
		}
		n.log.Errorf("删除Namespace失败: %s, error=%v", name, err)
		return fmt.Errorf("删除Namespace失败")
	}

	n.log.Infof("Namespace删除成功: %s", name)
	return nil
}

// Watch 监视 Namespace 变化
func (n *namespaceOperator) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	n.log.Infof("开始监视Namespace变化: selector=%s", opts.LabelSelector)

	watcher, err := n.client.CoreV1().
		Namespaces().
		Watch(n.ctx, opts)

	if err != nil {
		n.log.Errorf("监视Namespace失败: %v", err)
		return nil, fmt.Errorf("监视Namespace失败")
	}

	n.log.Info("成功创建Namespace监视器")
	return watcher, nil
}

// UpdateLabels 更新 Namespace 标签
func (n *namespaceOperator) UpdateLabels(name string, labels map[string]string) error {
	if name == "" {
		n.log.Error("更新标签失败：Namespace名称不能为空")
		return fmt.Errorf("Namespace名称不能为空")
	}

	n.log.Infof("更新Namespace标签: name=%s, 标签数量=%d", name, len(labels))

	// 获取当前 Namespace
	namespace, err := n.Get(name)
	if err != nil {
		return err
	}

	// 初始化标签映射
	if namespace.Labels == nil {
		namespace.Labels = make(map[string]string)
	}

	// 更新标签
	for k, v := range labels {
		namespace.Labels[k] = v
		n.log.Debugf("设置标签: %s=%s", k, v)
	}

	// 保存更新
	_, err = n.Update(namespace)
	if err != nil {
		return err
	}

	n.log.Infof("Namespace标签更新成功: %s", name)
	return nil
}

// UpdateAnnotations 更新 Namespace 注解
func (n *namespaceOperator) UpdateAnnotations(name string, annotations map[string]string) error {
	if name == "" {
		n.log.Error("更新注解失败：Namespace名称不能为空")
		return fmt.Errorf("Namespace名称不能为空")
	}

	n.log.Infof("更新Namespace注解: name=%s, 注解数量=%d", name, len(annotations))

	// 获取当前 Namespace
	namespace, err := n.Get(name)
	if err != nil {
		return err
	}

	// 初始化注解映射
	if namespace.Annotations == nil {
		namespace.Annotations = make(map[string]string)
	}

	// 更新注解
	for k, v := range annotations {
		namespace.Annotations[k] = v
		n.log.Debugf("设置注解: %s=%s", k, v)
	}

	// 保存更新
	_, err = n.Update(namespace)
	if err != nil {
		return err
	}

	n.log.Infof("Namespace注解更新成功: %s", name)
	return nil
}

// UpdateFinalizers 更新 Namespace 终结器
func (n *namespaceOperator) UpdateFinalizers(name string, finalizers []string) error {
	if name == "" {
		n.log.Error("更新终结器失败：Namespace名称不能为空")
		return fmt.Errorf("Namespace名称不能为空")
	}

	n.log.Infof("更新Namespace终结器: name=%s, 终结器数量=%d", name, len(finalizers))

	// 获取当前 Namespace
	namespace, err := n.Get(name)
	if err != nil {
		return err
	}

	// 更新终结器
	namespace.Finalizers = finalizers
	n.log.Debugf("设置终结器: %v", finalizers)

	// 保存更新
	_, err = n.Update(namespace)
	if err != nil {
		return err
	}

	n.log.Infof("Namespace终结器更新成功: %s", name)
	return nil
}

// IsActive 检查 Namespace 是否活跃
func (n *namespaceOperator) IsActive(name string) (bool, error) {
	n.log.Debugf("检查Namespace是否活跃: %s", name)

	namespace, err := n.Get(name)
	if err != nil {
		return false, err
	}

	active := namespace.Status.Phase == corev1.NamespaceActive
	n.log.Infof("Namespace %s 活跃状态: %v", name, active)
	return active, nil
}

// IsTerminating 检查 Namespace 是否正在终止
func (n *namespaceOperator) IsTerminating(name string) (bool, error) {
	n.log.Debugf("检查Namespace是否正在终止: %s", name)

	namespace, err := n.Get(name)
	if err != nil {
		return false, err
	}

	terminating := namespace.Status.Phase == corev1.NamespaceTerminating
	n.log.Infof("Namespace %s 终止状态: %v", name, terminating)
	return terminating, nil
}

// 	GetStatistics(string) (*NamespaceStatistics, error)

// GetStatistics 获取命名空间的资源统计信息
func (n *namespaceOperator) GetStatistics(name string) (*types.NamespaceStatistics, error) {
	if name == "" {
		n.log.Error("获取统计信息失败：命名空间名称不能为空")
		return nil, fmt.Errorf("命名空间名称不能为空")
	}

	n.log.Infof("开始获取命名空间统计信息: %s", name)

	// 检查命名空间是否存在
	_, err := n.Get(name)
	if err != nil {
		return nil, err
	}

	stats := &types.NamespaceStatistics{}

	// 获取 Pods 统计
	podList, err := n.client.CoreV1().Pods(name).List(n.ctx, metav1.ListOptions{})
	if err != nil {
		n.log.Errorf("获取Pod列表失败: %v", err)
		return nil, fmt.Errorf("获取Pod列表失败")
	}

	stats.Pods = int32(len(podList.Items))
	for _, pod := range podList.Items {
		if pod.Status.Phase == corev1.PodRunning {
			stats.PodsRunning++
		} else {
			stats.PodsNotRunning++
		}
	}

	// 获取 Services
	svcList, err := n.client.CoreV1().Services(name).List(n.ctx, metav1.ListOptions{})
	if err != nil {
		n.log.Errorf("获取Service列表失败: %v", err)
	} else {
		stats.Service = int32(len(svcList.Items))
	}

	// 获取 ConfigMaps
	cmList, err := n.client.CoreV1().ConfigMaps(name).List(n.ctx, metav1.ListOptions{})
	if err != nil {
		n.log.Errorf("获取ConfigMap列表失败: %v", err)
	} else {
		stats.ConfigMap = int32(len(cmList.Items))
	}

	// 获取 Secrets
	secretList, err := n.client.CoreV1().Secrets(name).List(n.ctx, metav1.ListOptions{})
	if err != nil {
		n.log.Errorf("获取Secret列表失败: %v", err)
	} else {
		stats.Secret = int32(len(secretList.Items))
	}

	// 获取 PVC
	pvcList, err := n.client.CoreV1().PersistentVolumeClaims(name).List(n.ctx, metav1.ListOptions{})
	if err != nil {
		n.log.Errorf("获取PVC列表失败: %v", err)
	} else {
		stats.Pvc = int32(len(pvcList.Items))
	}

	// 获取 Deployments
	deployList, err := n.client.AppsV1().Deployments(name).List(n.ctx, metav1.ListOptions{})
	if err != nil {
		n.log.Errorf("获取Deployment列表失败: %v", err)
	} else {
		stats.Deployment = int32(len(deployList.Items))
	}

	// 获取 StatefulSets
	stsList, err := n.client.AppsV1().StatefulSets(name).List(n.ctx, metav1.ListOptions{})
	if err != nil {
		n.log.Errorf("获取StatefulSet列表失败: %v", err)
	} else {
		stats.StatefulSet = int32(len(stsList.Items))
	}

	// 获取 DaemonSets
	dsList, err := n.client.AppsV1().DaemonSets(name).List(n.ctx, metav1.ListOptions{})
	if err != nil {
		n.log.Errorf("获取DaemonSet列表失败: %v", err)
	} else {
		stats.DaemonSet = int32(len(dsList.Items))
	}

	// 获取 Jobs
	jobList, err := n.client.BatchV1().Jobs(name).List(n.ctx, metav1.ListOptions{})
	if err != nil {
		n.log.Errorf("获取Job列表失败: %v", err)
	} else {
		stats.Job = int32(len(jobList.Items))
	}

	// 获取 CronJobs
	cronJobList, err := n.client.BatchV1().CronJobs(name).List(n.ctx, metav1.ListOptions{})
	if err != nil {
		n.log.Errorf("获取CronJob列表失败: %v", err)
	} else {
		stats.CronJob = int32(len(cronJobList.Items))
	}

	// 获取 Ingresses
	ingressList, err := n.client.NetworkingV1().Ingresses(name).List(n.ctx, metav1.ListOptions{})
	if err != nil {
		n.log.Errorf("获取Ingress列表失败: %v", err)
	} else {
		stats.Ingress = int32(len(ingressList.Items))
	}

	// 获取 ServiceAccounts
	saList, err := n.client.CoreV1().ServiceAccounts(name).List(n.ctx, metav1.ListOptions{})
	if err != nil {
		n.log.Errorf("获取ServiceAccount列表失败: %v", err)
	} else {
		stats.ServiceAccount = int32(len(saList.Items))
	}

	n.log.Infof("成功获取命名空间统计信息: namespace=%s, pods=%d, services=%d",
		name, stats.Pods, stats.Service)

	return stats, nil
}

// CreateOrUpdateResourceQuota 创建或更新资源配额
func (n *namespaceOperator) CreateOrUpdateResourceQuota(req *types.ResourceQuotaRequest) error {
	if req == nil {
		n.log.Error("创建ResourceQuota失败：请求不能为空")
		return fmt.Errorf("请求不能为空")
	}
	if req.Name == "" || req.Namespace == "" {
		n.log.Error("创建ResourceQuota失败：名称和命名空间不能为空")
		return fmt.Errorf("名称和命名空间不能为空")
	}

	n.log.Infof("创建或更新ResourceQuota: name=%s, namespace=%s", req.Name, req.Namespace)

	// 构建 ResourceQuota 对象
	resourceQuota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:        req.Name,
			Namespace:   req.Namespace,
			Labels:      req.Labels,
			Annotations: req.Annotations,
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{},
		},
	}

	if req.CPUAllocated != "" && req.CPUAllocated != "0" {
		cpuQuantity, err := resource.ParseQuantity(req.CPUAllocated)
		if err != nil {
			n.log.Errorf("解析CPU配额失败: %s, error=%v", req.CPUAllocated, err)
			return fmt.Errorf("解析CPU配额失败: %v", err)
		}
		resourceQuota.Spec.Hard[corev1.ResourceRequestsCPU] = cpuQuantity
		resourceQuota.Spec.Hard[corev1.ResourceLimitsCPU] = cpuQuantity
		n.log.Debugf("设置CPU配额: %s", cpuQuantity.String())
	}

	if req.MemoryAllocated != "" && req.MemoryAllocated != "0" {
		memQuantity, err := resource.ParseQuantity(req.MemoryAllocated)
		if err != nil {
			n.log.Errorf("解析内存配额失败: %s, error=%v", req.MemoryAllocated, err)
			return fmt.Errorf("解析内存配额失败: %v", err)
		}
		resourceQuota.Spec.Hard[corev1.ResourceRequestsMemory] = memQuantity
		resourceQuota.Spec.Hard[corev1.ResourceLimitsMemory] = memQuantity
		n.log.Debugf("设置内存配额: %s", memQuantity.String())
	}

	if req.StorageAllocated != "" && req.StorageAllocated != "0" {
		storageQuantity, err := resource.ParseQuantity(req.StorageAllocated)
		if err != nil {
			n.log.Errorf("解析存储配额失败: %s, error=%v", req.StorageAllocated, err)
			return fmt.Errorf("解析存储配额失败: %v", err)
		}
		resourceQuota.Spec.Hard[corev1.ResourceRequestsStorage] = storageQuantity
		n.log.Debugf("设置存储配额: %s", storageQuantity.String())
	}

	if req.EphemeralStorageAllocated != "" && req.EphemeralStorageAllocated != "0" {
		ephemeralQuantity, err := resource.ParseQuantity(req.EphemeralStorageAllocated)
		if err != nil {
			n.log.Errorf("解析临时存储配额失败: %s, error=%v", req.EphemeralStorageAllocated, err)
			return fmt.Errorf("解析临时存储配额失败: %v", err)
		}
		resourceQuota.Spec.Hard[corev1.ResourceRequestsEphemeralStorage] = ephemeralQuantity
		resourceQuota.Spec.Hard[corev1.ResourceLimitsEphemeralStorage] = ephemeralQuantity
		n.log.Debugf("设置临时存储配额: %s", ephemeralQuantity.String())
	}

	if req.GPUAllocated != "" && req.GPUAllocated != "0" {
		gpuQuantity, err := resource.ParseQuantity(req.GPUAllocated)
		if err != nil {
			n.log.Errorf("解析GPU配额失败: %s, error=%v", req.GPUAllocated, err)
			return fmt.Errorf("解析GPU配额失败: %v", err)
		}
		resourceQuota.Spec.Hard["requests.nvidia.com/gpu"] = gpuQuantity
		n.log.Debugf("设置GPU配额: %s", gpuQuantity.String())
	}

	if req.PodsAllocated > 0 {
		resourceQuota.Spec.Hard[corev1.ResourcePods] = *resource.NewQuantity(req.PodsAllocated, resource.DecimalSI)
	}
	if req.ConfigMapsAllocated > 0 {
		resourceQuota.Spec.Hard[corev1.ResourceConfigMaps] = *resource.NewQuantity(req.ConfigMapsAllocated, resource.DecimalSI)
	}
	if req.SecretsAllocated > 0 {
		resourceQuota.Spec.Hard[corev1.ResourceSecrets] = *resource.NewQuantity(req.SecretsAllocated, resource.DecimalSI)
	}
	if req.PVCsAllocated > 0 {
		resourceQuota.Spec.Hard[corev1.ResourcePersistentVolumeClaims] = *resource.NewQuantity(req.PVCsAllocated, resource.DecimalSI)
	}
	if req.ServicesAllocated > 0 {
		resourceQuota.Spec.Hard[corev1.ResourceServices] = *resource.NewQuantity(req.ServicesAllocated, resource.DecimalSI)
	}
	if req.LoadBalancersAllocated > 0 {
		resourceQuota.Spec.Hard[corev1.ResourceServicesLoadBalancers] = *resource.NewQuantity(req.LoadBalancersAllocated, resource.DecimalSI)
	}
	if req.NodePortsAllocated > 0 {
		resourceQuota.Spec.Hard[corev1.ResourceServicesNodePorts] = *resource.NewQuantity(req.NodePortsAllocated, resource.DecimalSI)
	}

	//  设置工作负载配额（使用 count/ 前缀）
	if req.DeploymentsAllocated > 0 {
		resourceQuota.Spec.Hard["count/deployments.apps"] = *resource.NewQuantity(req.DeploymentsAllocated, resource.DecimalSI)
	}
	if req.StatefulSetsAllocated > 0 {
		resourceQuota.Spec.Hard["count/statefulsets.apps"] = *resource.NewQuantity(req.StatefulSetsAllocated, resource.DecimalSI)
	}
	if req.DaemonSetsAllocated > 0 {
		resourceQuota.Spec.Hard["count/daemonsets.apps"] = *resource.NewQuantity(req.DaemonSetsAllocated, resource.DecimalSI)
	}
	if req.JobsAllocated > 0 {
		resourceQuota.Spec.Hard["count/jobs.batch"] = *resource.NewQuantity(req.JobsAllocated, resource.DecimalSI)
	}
	if req.CronJobsAllocated > 0 {
		resourceQuota.Spec.Hard["count/cronjobs.batch"] = *resource.NewQuantity(req.CronJobsAllocated, resource.DecimalSI)
	}
	if req.IngressesAllocated > 0 {
		resourceQuota.Spec.Hard["count/ingresses.networking.k8s.io"] = *resource.NewQuantity(req.IngressesAllocated, resource.DecimalSI)
	}

	// 尝试获取现有的 ResourceQuota
	existingQuota, err := n.client.CoreV1().ResourceQuotas(req.Namespace).Get(n.ctx, req.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// 创建新的 ResourceQuota
			n.log.Infof("ResourceQuota不存在，创建新的: %s/%s", req.Namespace, req.Name)
			_, err := n.client.CoreV1().ResourceQuotas(req.Namespace).Create(n.ctx, resourceQuota, metav1.CreateOptions{})
			if err != nil {
				n.log.Errorf("创建ResourceQuota失败: %v", err)
				return fmt.Errorf("创建ResourceQuota失败: %v", err)
			}
			n.log.Infof("ResourceQuota创建成功: %s/%s", req.Namespace, req.Name)
			return nil
		}
		n.log.Errorf("获取ResourceQuota失败: %v", err)
		return fmt.Errorf("获取ResourceQuota失败: %v", err)
	}

	// 更新现有的 ResourceQuota
	n.log.Infof("ResourceQuota已存在，执行更新: %s/%s", req.Namespace, req.Name)
	existingQuota.Spec = resourceQuota.Spec
	existingQuota.Labels = resourceQuota.Labels
	existingQuota.Annotations = resourceQuota.Annotations

	_, err = n.client.CoreV1().ResourceQuotas(req.Namespace).Update(n.ctx, existingQuota, metav1.UpdateOptions{})
	if err != nil {
		n.log.Errorf("更新ResourceQuota失败: %v", err)
		return fmt.Errorf("更新ResourceQuota失败: %v", err)
	}

	n.log.Infof("ResourceQuota更新成功: %s/%s", req.Namespace, req.Name)
	return nil
}

// CreateOrUpdateLimitRange 创建或更新 LimitRange
func (n *namespaceOperator) CreateOrUpdateLimitRange(req *types.LimitRangeRequest) error {
	if req == nil {
		n.log.Error("创建LimitRange失败：请求不能为空")
		return fmt.Errorf("请求不能为空")
	}
	if req.Name == "" || req.Namespace == "" {
		n.log.Error("创建LimitRange失败：名称和命名空间不能为空")
		return fmt.Errorf("名称和命名空间不能为空")
	}

	n.log.Infof("创建或更新LimitRange: name=%s, namespace=%s", req.Name, req.Namespace)

	// 构建 LimitRange 对象
	limitRange := &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{
			Name:        req.Name,
			Namespace:   req.Namespace,
			Labels:      req.Labels,
			Annotations: req.Annotations,
		},
		Spec: corev1.LimitRangeSpec{
			Limits: []corev1.LimitRangeItem{},
		},
	}

	//  Pod 级别限制
	podLimits := corev1.LimitRangeItem{
		Type: corev1.LimitTypePod,
		Max:  corev1.ResourceList{},
		Min:  corev1.ResourceList{},
	}

	// Pod Max 限制
	if req.PodMaxCPU != "" && req.PodMaxCPU != "0" {
		quantity, err := resource.ParseQuantity(req.PodMaxCPU)
		if err != nil {
			n.log.Errorf("解析Pod最大CPU失败: %s, error=%v", req.PodMaxCPU, err)
			return fmt.Errorf("解析Pod最大CPU失败: %v", err)
		}
		podLimits.Max[corev1.ResourceCPU] = quantity
	}
	if req.PodMaxMemory != "" && req.PodMaxMemory != "0" {
		quantity, err := resource.ParseQuantity(req.PodMaxMemory)
		if err != nil {
			n.log.Errorf("解析Pod最大内存失败: %s, error=%v", req.PodMaxMemory, err)
			return fmt.Errorf("解析Pod最大内存失败: %v", err)
		}
		podLimits.Max[corev1.ResourceMemory] = quantity
	}
	if req.PodMaxEphemeralStorage != "" && req.PodMaxEphemeralStorage != "0" {
		quantity, err := resource.ParseQuantity(req.PodMaxEphemeralStorage)
		if err != nil {
			n.log.Errorf("解析Pod最大临时存储失败: %s, error=%v", req.PodMaxEphemeralStorage, err)
			return fmt.Errorf("解析Pod最大临时存储失败: %v", err)
		}
		podLimits.Max[corev1.ResourceEphemeralStorage] = quantity
	}

	// Pod Min 限制
	if req.PodMinCPU != "" && req.PodMinCPU != "0" {
		quantity, err := resource.ParseQuantity(req.PodMinCPU)
		if err != nil {
			n.log.Errorf("解析Pod最小CPU失败: %s, error=%v", req.PodMinCPU, err)
			return fmt.Errorf("解析Pod最小CPU失败: %v", err)
		}
		podLimits.Min[corev1.ResourceCPU] = quantity
	}
	if req.PodMinMemory != "" && req.PodMinMemory != "0" {
		quantity, err := resource.ParseQuantity(req.PodMinMemory)
		if err != nil {
			n.log.Errorf("解析Pod最小内存失败: %s, error=%v", req.PodMinMemory, err)
			return fmt.Errorf("解析Pod最小内存失败: %v", err)
		}
		podLimits.Min[corev1.ResourceMemory] = quantity
	}
	if req.PodMinEphemeralStorage != "" && req.PodMinEphemeralStorage != "0" {
		quantity, err := resource.ParseQuantity(req.PodMinEphemeralStorage)
		if err != nil {
			n.log.Errorf("解析Pod最小临时存储失败: %s, error=%v", req.PodMinEphemeralStorage, err)
			return fmt.Errorf("解析Pod最小临时存储失败: %v", err)
		}
		podLimits.Min[corev1.ResourceEphemeralStorage] = quantity
	}

	if len(podLimits.Max) > 0 || len(podLimits.Min) > 0 {
		limitRange.Spec.Limits = append(limitRange.Spec.Limits, podLimits)
	}

	containerLimits := corev1.LimitRangeItem{
		Type:           corev1.LimitTypeContainer,
		Max:            corev1.ResourceList{},
		Min:            corev1.ResourceList{},
		Default:        corev1.ResourceList{},
		DefaultRequest: corev1.ResourceList{},
	}

	// Container Max 限制
	if req.ContainerMaxCPU != "" && req.ContainerMaxCPU != "0" {
		quantity, err := resource.ParseQuantity(req.ContainerMaxCPU)
		if err != nil {
			n.log.Errorf("解析Container最大CPU失败: %s, error=%v", req.ContainerMaxCPU, err)
			return fmt.Errorf("解析Container最大CPU失败: %v", err)
		}
		containerLimits.Max[corev1.ResourceCPU] = quantity
	}
	if req.ContainerMaxMemory != "" && req.ContainerMaxMemory != "0" {
		quantity, err := resource.ParseQuantity(req.ContainerMaxMemory)
		if err != nil {
			n.log.Errorf("解析Container最大内存失败: %s, error=%v", req.ContainerMaxMemory, err)
			return fmt.Errorf("解析Container最大内存失败: %v", err)
		}
		containerLimits.Max[corev1.ResourceMemory] = quantity
	}
	if req.ContainerMaxEphemeralStorage != "" && req.ContainerMaxEphemeralStorage != "0" {
		quantity, err := resource.ParseQuantity(req.ContainerMaxEphemeralStorage)
		if err != nil {
			n.log.Errorf("解析Container最大临时存储失败: %s, error=%v", req.ContainerMaxEphemeralStorage, err)
			return fmt.Errorf("解析Container最大临时存储失败: %v", err)
		}
		containerLimits.Max[corev1.ResourceEphemeralStorage] = quantity
	}

	// Container Min 限制
	if req.ContainerMinCPU != "" && req.ContainerMinCPU != "0" {
		quantity, err := resource.ParseQuantity(req.ContainerMinCPU)
		if err != nil {
			n.log.Errorf("解析Container最小CPU失败: %s, error=%v", req.ContainerMinCPU, err)
			return fmt.Errorf("解析Container最小CPU失败: %v", err)
		}
		containerLimits.Min[corev1.ResourceCPU] = quantity
	}
	if req.ContainerMinMemory != "" && req.ContainerMinMemory != "0" {
		quantity, err := resource.ParseQuantity(req.ContainerMinMemory)
		if err != nil {
			n.log.Errorf("解析Container最小内存失败: %s, error=%v", req.ContainerMinMemory, err)
			return fmt.Errorf("解析Container最小内存失败: %v", err)
		}
		containerLimits.Min[corev1.ResourceMemory] = quantity
	}
	if req.ContainerMinEphemeralStorage != "" && req.ContainerMinEphemeralStorage != "0" {
		quantity, err := resource.ParseQuantity(req.ContainerMinEphemeralStorage)
		if err != nil {
			n.log.Errorf("解析Container最小临时存储失败: %s, error=%v", req.ContainerMinEphemeralStorage, err)
			return fmt.Errorf("解析Container最小临时存储失败: %v", err)
		}
		containerLimits.Min[corev1.ResourceEphemeralStorage] = quantity
	}

	// Container Default 限制（limits）
	if req.ContainerDefaultCPU != "" && req.ContainerDefaultCPU != "0" {
		quantity, err := resource.ParseQuantity(req.ContainerDefaultCPU)
		if err != nil {
			n.log.Errorf("解析Container默认CPU限制失败: %s, error=%v", req.ContainerDefaultCPU, err)
			return fmt.Errorf("解析Container默认CPU限制失败: %v", err)
		}
		containerLimits.Default[corev1.ResourceCPU] = quantity
	}
	if req.ContainerDefaultMemory != "" && req.ContainerDefaultMemory != "0" {
		quantity, err := resource.ParseQuantity(req.ContainerDefaultMemory)
		if err != nil {
			n.log.Errorf("解析Container默认内存限制失败: %s, error=%v", req.ContainerDefaultMemory, err)
			return fmt.Errorf("解析Container默认内存限制失败: %v", err)
		}
		containerLimits.Default[corev1.ResourceMemory] = quantity
	}
	if req.ContainerDefaultEphemeralStorage != "" && req.ContainerDefaultEphemeralStorage != "0" {
		quantity, err := resource.ParseQuantity(req.ContainerDefaultEphemeralStorage)
		if err != nil {
			n.log.Errorf("解析Container默认临时存储限制失败: %s, error=%v", req.ContainerDefaultEphemeralStorage, err)
			return fmt.Errorf("解析Container默认临时存储限制失败: %v", err)
		}
		containerLimits.Default[corev1.ResourceEphemeralStorage] = quantity
	}

	// Container DefaultRequest 请求（requests）
	if req.ContainerDefaultRequestCPU != "" && req.ContainerDefaultRequestCPU != "0" {
		quantity, err := resource.ParseQuantity(req.ContainerDefaultRequestCPU)
		if err != nil {
			n.log.Errorf("解析Container默认CPU请求失败: %s, error=%v", req.ContainerDefaultRequestCPU, err)
			return fmt.Errorf("解析Container默认CPU请求失败: %v", err)
		}
		containerLimits.DefaultRequest[corev1.ResourceCPU] = quantity
	}
	if req.ContainerDefaultRequestMemory != "" && req.ContainerDefaultRequestMemory != "0" {
		quantity, err := resource.ParseQuantity(req.ContainerDefaultRequestMemory)
		if err != nil {
			n.log.Errorf("解析Container默认内存请求失败: %s, error=%v", req.ContainerDefaultRequestMemory, err)
			return fmt.Errorf("解析Container默认内存请求失败: %v", err)
		}
		containerLimits.DefaultRequest[corev1.ResourceMemory] = quantity
	}
	if req.ContainerDefaultRequestEphemeralStorage != "" && req.ContainerDefaultRequestEphemeralStorage != "0" {
		quantity, err := resource.ParseQuantity(req.ContainerDefaultRequestEphemeralStorage)
		if err != nil {
			n.log.Errorf("解析Container默认临时存储请求失败: %s, error=%v", req.ContainerDefaultRequestEphemeralStorage, err)
			return fmt.Errorf("解析Container默认临时存储请求失败: %v", err)
		}
		containerLimits.DefaultRequest[corev1.ResourceEphemeralStorage] = quantity
	}

	if len(containerLimits.Max) > 0 || len(containerLimits.Min) > 0 ||
		len(containerLimits.Default) > 0 || len(containerLimits.DefaultRequest) > 0 {
		limitRange.Spec.Limits = append(limitRange.Spec.Limits, containerLimits)
	}

	// 尝试获取现有的 LimitRange
	existingLimitRange, err := n.client.CoreV1().LimitRanges(req.Namespace).Get(n.ctx, req.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// 创建新的 LimitRange
			n.log.Infof("LimitRange不存在，创建新的: %s/%s", req.Namespace, req.Name)
			_, err := n.client.CoreV1().LimitRanges(req.Namespace).Create(n.ctx, limitRange, metav1.CreateOptions{})
			if err != nil {
				n.log.Errorf("创建LimitRange失败: %v", err)
				return fmt.Errorf("创建LimitRange失败: %v", err)
			}
			n.log.Infof("LimitRange创建成功: %s/%s", req.Namespace, req.Name)
			return nil
		}
		n.log.Errorf("获取LimitRange失败: %v", err)
		return fmt.Errorf("获取LimitRange失败: %v", err)
	}

	// 更新现有的 LimitRange
	n.log.Infof("LimitRange已存在，执行更新: %s/%s", req.Namespace, req.Name)
	existingLimitRange.Spec = limitRange.Spec
	existingLimitRange.Labels = limitRange.Labels
	existingLimitRange.Annotations = limitRange.Annotations

	_, err = n.client.CoreV1().LimitRanges(req.Namespace).Update(n.ctx, existingLimitRange, metav1.UpdateOptions{})
	if err != nil {
		n.log.Errorf("更新LimitRange失败: %v", err)
		return fmt.Errorf("更新LimitRange失败: %v", err)
	}

	n.log.Infof("LimitRange更新成功: %s/%s", req.Namespace, req.Name)
	return nil
}

// ListAll 获取所有 Namespace（不分页）
func (n *namespaceOperator) ListAll() ([]corev1.Namespace, error) {
	n.log.Info("开始获取所有Namespace列表")

	var namespaces []*corev1.Namespace
	var err error

	// 优先从 informer 缓存获取
	if n.useInformer && n.nsLister != nil {
		n.log.Debug("从缓存获取所有Namespace")
		namespaces, err = n.nsLister.List(labels.Everything())
		if err != nil {
			n.log.Errorf("从缓存获取Namespace列表失败: %v", err)
			return nil, fmt.Errorf("获取Namespace列表失败")
		}
		n.log.Infof("从缓存获取到 %d 个Namespace", len(namespaces))
	} else {
		// 降级到直接调用 API
		n.log.Debug("通过API获取所有Namespace")
		nsList, err := n.client.CoreV1().Namespaces().List(n.ctx, metav1.ListOptions{})
		if err != nil {
			n.log.Errorf("API获取Namespace列表失败: %v", err)
			return nil, fmt.Errorf("获取Namespace列表失败")
		}

		// 转换为指针切片
		namespaces = make([]*corev1.Namespace, len(nsList.Items))
		for i := range nsList.Items {
			namespaces[i] = &nsList.Items[i]
		}
		n.log.Infof("通过API获取到 %d 个Namespace", len(namespaces))
	}

	// 转换为值切片返回
	result := make([]corev1.Namespace, len(namespaces))
	for i, ns := range namespaces {
		result[i] = *ns
	}

	n.log.Infof("成功返回所有Namespace，总数: %d", len(result))
	return result, nil
}
