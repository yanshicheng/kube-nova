package operator

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
)

const (
	evictionTimeout  = 5 * time.Minute
	podDeleteTimeout = 300 * time.Second
	retryInterval    = 2 * time.Second
)

type nodeOperator struct {
	BaseOperator
	client          kubernetes.Interface
	config          *rest.Config
	nodeLister      v1.NodeLister
	nodeInformer    cache.SharedIndexInformer
	informerFactory informers.SharedInformerFactory
}

func NewNodeOperator(ctx context.Context, client kubernetes.Interface, config *rest.Config) types.NodeOperator {
	return &nodeOperator{
		BaseOperator:    NewBaseOperator(ctx, false),
		client:          client,
		config:          config,
		informerFactory: nil,
		nodeLister:      nil,
		nodeInformer:    nil,
	}
}

func NewNodeOperatorWithInformer(
	ctx context.Context,
	client kubernetes.Interface,
	config *rest.Config,
	informerFactory informers.SharedInformerFactory,
) types.NodeOperator {
	var nodeLister v1.NodeLister
	var nodeInformer cache.SharedIndexInformer

	if informerFactory != nil {
		nodeInformer = informerFactory.Core().V1().Nodes().Informer()
		nodeLister = informerFactory.Core().V1().Nodes().Lister()
	}

	return &nodeOperator{
		BaseOperator:    NewBaseOperator(ctx, informerFactory != nil),
		client:          client,
		config:          config,
		informerFactory: informerFactory,
		nodeLister:      nodeLister,
		nodeInformer:    nodeInformer,
	}
}

// List 获取节点列表（支持搜索、分页、排序）
func (n *nodeOperator) List(req *types.NodeListRequest) (*types.NodeListResponse, error) {
	n.log.Infof("开始获取节点列表, 参数: page=%d, pageSize=%d, orderField=%s, isAsc=%v, search=%s",
		req.Page, req.PageSize, req.OrderField, req.IsAsc, req.Search)

	var nodes []*corev1.Node
	var err error

	// 优先使用 Informer
	if n.nodeLister != nil {
		n.log.Debug("从缓存获取节点列表")
		nodes, err = n.nodeLister.List(nil)
		if err != nil {
			n.log.Errorf("通过Lister获取节点列表失败: %v", err)
			return nil, fmt.Errorf("获取节点列表失败")
		}
		n.log.Infof("从缓存获取到 %d 个节点", len(nodes))
	} else {
		n.log.Debug("通过API获取节点列表")
		nodeList, err := n.client.CoreV1().Nodes().List(n.ctx, metav1.ListOptions{})
		if err != nil {
			n.log.Errorf("通过API获取节点列表失败: %v", err)
			return nil, fmt.Errorf("获取节点列表失败")
		}
		nodes = make([]*corev1.Node, len(nodeList.Items))
		for i := range nodeList.Items {
			nodes[i] = &nodeList.Items[i]
		}
		n.log.Infof("通过API获取到 %d 个节点", len(nodes))
	}

	// 转换为 NodeInfo
	n.log.Debug("转换节点信息格式")
	var nodeInfos []*types.NodeInfo
	for _, node := range nodes {
		nodeInfo := n.convertToNodeInfo(node)
		nodeInfos = append(nodeInfos, nodeInfo)
	}

	// 搜索过滤
	if req.Search != "" {
		nodeInfos = n.filterNodes(nodeInfos, req.Search)
		n.log.Infof("搜索关键词 '%s' 后剩余 %d 个节点", req.Search, len(nodeInfos))
	}

	total := uint64(len(nodeInfos))

	// 排序
	n.sortNodes(nodeInfos, req.OrderField, req.IsAsc)

	// 分页
	start, end := n.calculatePagination(len(nodeInfos), int(req.Page), int(req.PageSize))
	pagedNodes := nodeInfos[start:end]

	n.log.Infof("成功获取节点列表，总数: %d, 当前页: %d, 每页: %d, 返回: %d",
		total, req.Page, req.PageSize, len(pagedNodes))

	return &types.NodeListResponse{
		Total: total,
		Items: pagedNodes,
	}, nil
}

// GetClusterResourceTotal 获取集群资源总量
func (n *nodeOperator) GetClusterResourceTotal() (*types.ClusterResourceTotal, error) {
	n.log.Info("开始获取集群资源总量")

	var nodes []*corev1.Node
	var err error

	// 优先使用 Informer
	if n.nodeLister != nil {
		n.log.Debug("从缓存获取节点列表")
		nodes, err = n.nodeLister.List(nil)
		if err != nil {
			n.log.Errorf("通过Lister获取节点列表失败: %v", err)
			return nil, fmt.Errorf("获取节点列表失败")
		}
	} else {
		n.log.Debug("通过API获取节点列表")
		nodeList, err := n.client.CoreV1().Nodes().List(n.ctx, metav1.ListOptions{})
		if err != nil {
			n.log.Errorf("通过API获取节点列表失败: %v", err)
			return nil, fmt.Errorf("获取节点列表失败")
		}
		nodes = make([]*corev1.Node, len(nodeList.Items))
		for i := range nodeList.Items {
			nodes[i] = &nodeList.Items[i]
		}
	}

	var totalCpu float64
	var totalMemoryBytes int64
	var totalPods int64

	// 统计所有节点的资源
	for _, node := range nodes {
		if cpu := node.Status.Capacity.Cpu(); cpu != nil {
			totalCpu += float64(cpu.Value())
		}
		if memory := node.Status.Capacity.Memory(); memory != nil {
			totalMemoryBytes += memory.Value()
		}
		if pods := node.Status.Capacity.Pods(); pods != nil {
			totalPods += pods.Value()
		}
	}

	// 将内存从字节转换为 GiB
	const GiB = 1024 * 1024 * 1024
	totalMemoryGiB := float64(totalMemoryBytes) / float64(GiB)

	result := &types.ClusterResourceTotal{
		TotalCpu:    totalCpu,
		TotalMemory: totalMemoryGiB,
		TotalPods:   totalPods,
		TotalNodes:  len(nodes),
	}

	n.log.Infof("集群资源总量: CPU=%.2f核, 内存=%.2fGiB, Pod总数=%d, 节点数=%d",
		result.TotalCpu, result.TotalMemory, result.TotalPods, result.TotalNodes)

	return result, nil
}

// filterNodes 根据搜索关键词过滤节点
func (n *nodeOperator) filterNodes(nodes []*types.NodeInfo, search string) []*types.NodeInfo {
	if search == "" {
		return nodes
	}

	search = strings.ToLower(search)
	var filtered []*types.NodeInfo

	for _, node := range nodes {
		if strings.Contains(strings.ToLower(node.NodeName), search) ||
			strings.Contains(strings.ToLower(node.NodeIp), search) ||
			strings.Contains(strings.ToLower(node.HostName), search) ||
			strings.Contains(strings.ToLower(node.Roles), search) ||
			strings.Contains(strings.ToLower(node.Status), search) {
			filtered = append(filtered, node)
		}
	}

	return filtered
}

// sortNodes 对节点列表进行排序
func (n *nodeOperator) sortNodes(nodes []*types.NodeInfo, orderField string, isAsc bool) {
	sort.Slice(nodes, func(i, j int) bool {
		var less bool

		switch orderField {
		case "nodeName":
			less = nodes[i].NodeName < nodes[j].NodeName
		case "nodeIp":
			less = nodes[i].NodeIp < nodes[j].NodeIp
		case "hostName":
			less = nodes[i].HostName < nodes[j].HostName
		case "status":
			less = nodes[i].Status < nodes[j].Status
		case "cpu":
			less = nodes[i].Cpu < nodes[j].Cpu
		case "memory":
			less = nodes[i].Memory < nodes[j].Memory
		case "joinAt":
			less = nodes[i].JoinAt < nodes[j].JoinAt
		case "pods":
			less = nodes[i].Pods < nodes[j].Pods
		case "podsUsage":
			less = nodes[i].PodsUsage < nodes[j].PodsUsage
		default:
			less = nodes[i].NodeName < nodes[j].NodeName
		}

		if isAsc {
			return less
		}
		return !less
	})
}

// calculatePagination 计算分页
func (n *nodeOperator) calculatePagination(total, page, pageSize int) (start, end int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}

	start = (page - 1) * pageSize
	if start > total {
		start = total
	}

	end = start + pageSize
	if end > total {
		end = total
	}

	return start, end
}

// convertToNodeInfo 将 K8s Node 转换为 NodeInfo
func (n *nodeOperator) convertToNodeInfo(node *corev1.Node) *types.NodeInfo {
	n.log.Debugf("转换节点信息: %s", node.Name)

	nodeInfo := &types.NodeInfo{
		NodeName:        node.Name,
		NodeUuid:        node.Status.NodeInfo.SystemUUID,
		OsImage:         node.Status.NodeInfo.OSImage,
		KernelVersion:   node.Status.NodeInfo.KernelVersion,
		OperatingSystem: node.Status.NodeInfo.OperatingSystem,
		Architecture:    node.Status.NodeInfo.Architecture,
		Runtime:         node.Status.NodeInfo.ContainerRuntimeVersion,
		KubeletVersion:  node.Status.NodeInfo.KubeletVersion,
		JoinAt:          node.CreationTimestamp.Unix(),
		PodCidr:         node.Spec.PodCIDR,
	}

	nodeInfo.Pods = node.Status.Capacity.Pods().Value()

	for _, addr := range node.Status.Addresses {
		switch addr.Type {
		case corev1.NodeInternalIP:
			nodeInfo.NodeIp = addr.Address
		case corev1.NodeHostName:
			nodeInfo.HostName = addr.Address
		}
	}

	nodeInfo.Roles = n.getNodeRoles(node)

	if cpu := node.Status.Capacity.Cpu(); cpu != nil {
		nodeInfo.Cpu = float64(cpu.Value())
	}
	if memory := node.Status.Capacity.Memory(); memory != nil {
		nodeInfo.Memory = memory.Value()
	}

	nodeInfo.IsGpu = n.hasGPU(node)
	nodeInfo.Unschedulable = n.getSchedulableStatus(node)
	nodeInfo.Taints = n.getTaintsString(node)
	nodeInfo.Status = n.getNodeStatus(node)

	if len(node.Spec.PodCIDRs) > 0 {
		nodeInfo.PodCidrs = strings.Join(node.Spec.PodCIDRs, ",")
	}

	n.log.Debugf("节点信息转换完成: %s, 状态: %s", node.Name, nodeInfo.Status)
	return nodeInfo
}

func (n *nodeOperator) getNodeRoles(node *corev1.Node) string {
	var roles []string
	for label := range node.Labels {

		if strings.HasPrefix(label, "node-role.kubernetes.io/") {
			role := strings.TrimPrefix(label, "node-role.kubernetes.io/")
			if role == "" {
				if strings.Contains(label, "master") {
					role = "master"
				} else if strings.Contains(label, "control-plane") {
					role = "control-plane"
				}
			}
			if role != "" {
				roles = append(roles, role)
			}
		}
	}
	if len(roles) == 0 {
		roles = append(roles, "worker")
	}
	return strings.Join(roles, ",")
}

func (n *nodeOperator) hasGPU(node *corev1.Node) int64 {
	for resourceName := range node.Status.Capacity {
		resourceNameStr := string(resourceName)
		if strings.Contains(resourceNameStr, "gpu") ||
			strings.Contains(resourceNameStr, "nvidia.com") ||
			strings.Contains(resourceNameStr, "amd.com") ||
			strings.Contains(resourceNameStr, "intel.com") {
			n.log.Debugf("节点 %s 检测到GPU资源: %s", node.Name, resourceNameStr)
			return 1
		}
	}
	return 0
}

func (n *nodeOperator) getSchedulableStatus(node *corev1.Node) int64 {
	if node.Spec.Unschedulable {
		return 2
	}
	return 1
}

func (n *nodeOperator) getTaintsString(node *corev1.Node) string {
	if len(node.Spec.Taints) == 0 {
		return ""
	}
	var taints []string
	for _, taint := range node.Spec.Taints {
		var taintStr string
		if taint.Value != "" {
			taintStr = fmt.Sprintf("%s=%s:%s", taint.Key, taint.Value, taint.Effect)
		} else {
			taintStr = fmt.Sprintf("%s:%s", taint.Key, taint.Effect)
		}
		taints = append(taints, taintStr)
	}
	return strings.Join(taints, ",")
}

func (n *nodeOperator) getNodeStatus(node *corev1.Node) string {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			if condition.Status == corev1.ConditionTrue {
				return "Ready"
			} else {
				return "NotReady"
			}
		}
	}
	return "Unknown"
}

func (n *nodeOperator) GetNodeDetail(nodeName string) (*types.NodeInfo, error) {
	n.log.Infof("获取节点详情: %s", nodeName)
	var node *corev1.Node
	var err error

	// 优先使用 Informer
	if n.nodeLister != nil {
		n.log.Debug("从缓存获取节点详情")
		node, err = n.nodeLister.Get(nodeName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				n.log.Errorf("节点不存在: %s", nodeName)
				return nil, fmt.Errorf("节点 %s 不存在", nodeName)
			}
			n.log.Infof("缓存获取失败，尝试通过API获取节点 %s: %v", nodeName, err)
			node, err = n.client.CoreV1().Nodes().Get(n.ctx, nodeName, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					n.log.Errorf("通过API确认节点不存在: %s", nodeName)
					return nil, fmt.Errorf("节点 %s 不存在", nodeName)
				}
				n.log.Errorf("通过API获取节点失败: %s, error=%v", nodeName, err)
				return nil, fmt.Errorf("获取节点失败")
			}
		}
	} else {
		n.log.Debug("直接通过API获取节点详情")
		node, err = n.client.CoreV1().Nodes().Get(n.ctx, nodeName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				n.log.Errorf("节点不存在: %s", nodeName)
				return nil, fmt.Errorf("节点 %s 不存在", nodeName)
			}
			n.log.Errorf("通过API获取节点失败: %s, error=%v", nodeName, err)
			return nil, fmt.Errorf("获取节点失败")
		}
	}

	n.log.Debug("获取节点上的Pod使用情况")
	podCount, err := n.getNodePodCount(nodeName)
	if err != nil {
		n.log.Errorf("获取节点Pod数量失败: %v，将使用默认值0", err)
		podCount = 0
	}

	nodeInfo := n.convertToNodeInfo(node)
	nodeInfo.PodsUsage = int64(podCount)

	n.log.Infof("成功获取节点详情: %s, 状态: %s, Pod使用: %d/%d",
		nodeName, nodeInfo.Status, nodeInfo.PodsUsage, nodeInfo.Pods)

	return nodeInfo, nil
}

func (n *nodeOperator) getNodePodCount(nodeName string) (int, error) {
	fieldSelector := fields.OneTermEqualSelector("spec.nodeName", nodeName).String()
	pods, err := n.client.CoreV1().Pods("").List(n.ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return 0, fmt.Errorf("获取节点Pod列表失败: %v", err)
	}

	runningPods := 0
	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodSucceeded && pod.Status.Phase != corev1.PodFailed {
			runningPods++
		}
	}
	return runningPods, nil
}

func (n *nodeOperator) AddLabels(nodeName, key, value string) error {
	n.log.Infof("添加节点标签: node=%s, %s=%s", nodeName, key, value)

	node, err := n.getNodeFromCache(nodeName)
	if err != nil {
		return err
	}

	if types.IsSystemLabel(key) {
		n.log.Errorf("不能添加系统标签: %s", key)
		return fmt.Errorf("不能添加系统标签: %s", key)
	}

	if node.Labels == nil {
		node.Labels = make(map[string]string)
	}
	node.Labels[key] = value

	_, err = n.client.CoreV1().Nodes().Update(n.ctx, node, metav1.UpdateOptions{})
	if err != nil {
		n.log.Errorf("更新节点标签失败: %s, error=%v", nodeName, err)
		return fmt.Errorf("更新节点失败")
	}

	n.log.Infof("节点 %s 添加标签成功: %s=%s", nodeName, key, value)
	return nil
}

func (n *nodeOperator) DeleteLabels(nodeName string, key string) error {
	n.log.Infof("删除节点标签: node=%s, key=%s", nodeName, key)

	node, err := n.getNodeFromCache(nodeName)
	if err != nil {
		return err
	}

	if types.IsSystemLabel(key) {
		n.log.Errorf("不能删除系统标签: %s", key)
		return fmt.Errorf("不能删除系统标签: %s", key)
	}

	delete(node.Labels, key)
	_, err = n.client.CoreV1().Nodes().Update(n.ctx, node, metav1.UpdateOptions{})
	if err != nil {
		n.log.Errorf("更新节点失败: %s, error=%v", nodeName, err)
		return fmt.Errorf("更新节点失败")
	}

	n.log.Infof("节点 %s 删除标签成功: %s", nodeName, key)
	return nil
}

func (n *nodeOperator) GetLabels(nodeName string) ([]*types.LabelsInfo, error) {
	n.log.Debugf("获取节点标签: %s", nodeName)

	// 优先使用 Informer
	if n.nodeLister != nil {
		node, err := n.nodeLister.Get(nodeName)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				n.log.Errorf("缓存获取节点失败: %s, error=%v", nodeName, err)
			}
			n.log.Debug("降级到API获取节点标签")
			node, err = n.client.CoreV1().Nodes().Get(n.ctx, nodeName, metav1.GetOptions{})
			if err != nil {
				n.log.Errorf("API获取节点失败: %s, error=%v", nodeName, err)
				return nil, fmt.Errorf("获取节点失败")
			}
		}
		n.log.Debugf("获取到 %d 个标签", len(node.Labels))
		return getLabelInfos(node.Labels), nil
	}

	node, err := n.client.CoreV1().Nodes().Get(n.ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		n.log.Errorf("获取节点失败: %s, error=%v", nodeName, err)
		return nil, fmt.Errorf("获取节点失败")
	}

	n.log.Debugf("获取到 %d 个标签", len(node.Labels))
	return getLabelInfos(node.Labels), nil
}

func getLabelInfos(labels map[string]string) []*types.LabelsInfo {
	var labelInfos []*types.LabelsInfo
	for key, value := range labels {
		labelInfos = append(labelInfos, &types.LabelsInfo{
			Key:      key,
			Value:    value,
			IsDelete: !types.IsSystemLabel(key),
		})
	}
	return labelInfos
}

func (n *nodeOperator) AddAnnotations(nodeName, key, value string) error {
	n.log.Infof("添加节点注解: node=%s, %s=%s", nodeName, key, value)

	node, err := n.getNodeFromCache(nodeName)
	if err != nil {
		return err
	}

	if types.IsSystemAnnotation(key) {
		n.log.Errorf("不能添加系统注解: %s", key)
		return fmt.Errorf("不能添加系统注解: %s", key)
	}

	if node.Annotations == nil {
		node.Annotations = make(map[string]string)
	}
	node.Annotations[key] = value

	_, err = n.client.CoreV1().Nodes().Update(n.ctx, node, metav1.UpdateOptions{})
	if err != nil {
		n.log.Errorf("更新节点注解失败: %s, error=%v", nodeName, err)
		return fmt.Errorf("更新节点失败")
	}

	n.log.Infof("节点 %s 添加注解成功: %s=%s", nodeName, key, value)
	return nil
}

func (n *nodeOperator) DeleteAnnotations(nodeName string, key string) error {
	n.log.Infof("删除节点注解: node=%s, key=%s", nodeName, key)

	node, err := n.getNodeFromCache(nodeName)
	if err != nil {
		return err
	}

	if types.IsSystemAnnotation(key) {
		n.log.Errorf("不能删除系统注解: %s", key)
		return fmt.Errorf("不能删除系统注解: %s", key)
	}

	delete(node.Annotations, key)
	_, err = n.client.CoreV1().Nodes().Update(n.ctx, node, metav1.UpdateOptions{})
	if err != nil {
		n.log.Errorf("更新节点失败: %s, error=%v", nodeName, err)
		return fmt.Errorf("更新节点失败")
	}

	n.log.Infof("节点 %s 删除注解成功: %s", nodeName, key)
	return nil
}

func (n *nodeOperator) GetAnnotations(nodeName string) ([]*types.LabelsInfo, error) {
	n.log.Debugf("获取节点注解: %s", nodeName)

	// 优先使用 Informer
	if n.nodeLister != nil {
		node, err := n.nodeLister.Get(nodeName)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				n.log.Errorf("缓存获取节点失败: %s, error=%v", nodeName, err)
			}
			n.log.Debug("降级到API获取节点注解")
			node, err = n.client.CoreV1().Nodes().Get(n.ctx, nodeName, metav1.GetOptions{})
			if err != nil {
				n.log.Errorf("API获取节点失败: %s, error=%v", nodeName, err)
				return nil, fmt.Errorf("获取节点失败")
			}
		}
		n.log.Debugf("获取到 %d 个注解", len(node.Annotations))
		return getAnnotations(node.Annotations), nil
	}

	node, err := n.client.CoreV1().Nodes().Get(n.ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		n.log.Errorf("获取节点失败: %s, error=%v", nodeName, err)
		return nil, fmt.Errorf("获取节点失败")
	}

	n.log.Debugf("获取到 %d 个注解", len(node.Annotations))
	return getAnnotations(node.Annotations), nil
}

func getAnnotations(annotations map[string]string) []*types.LabelsInfo {
	var annotationInfos []*types.LabelsInfo
	for key, value := range annotations {
		annotationInfos = append(annotationInfos, &types.LabelsInfo{
			Key:      key,
			Value:    value,
			IsDelete: !types.IsSystemAnnotation(key),
		})
	}
	return annotationInfos
}

func (n *nodeOperator) AddTaint(nodeName, key, value, effect string) error {
	n.log.Infof("添加节点污点: node=%s, %s=%s:%s", nodeName, key, value, effect)

	node, err := n.getNodeFromCache(nodeName)
	if err != nil {
		return err
	}

	if types.IsSystemTaint(key) {
		n.log.Errorf("不能添加系统污点: %s", key)
		return fmt.Errorf("不能添加系统污点: %s", key)
	}

	taint := corev1.Taint{
		Key:    key,
		Value:  value,
		Effect: corev1.TaintEffect(effect),
	}
	node.Spec.Taints = append(node.Spec.Taints, taint)

	_, err = n.client.CoreV1().Nodes().Update(n.ctx, node, metav1.UpdateOptions{})
	if err != nil {
		n.log.Errorf("更新节点污点失败: %s, error=%v", nodeName, err)
		return fmt.Errorf("更新节点失败")
	}

	n.log.Infof("节点 %s 添加污点成功: %s=%s:%s", nodeName, key, value, effect)
	return nil
}

func (n *nodeOperator) DeleteTaint(nodeName, key, effect string) error {
	n.log.Infof("开始删除节点污点: node=%s, key=%s, effect=%s", nodeName, key, effect)

	if types.IsSystemTaint(key) {
		n.log.Errorf("不能删除系统污点: %s", key)
		return fmt.Errorf("不能删除系统污点: %s", key)
	}

	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		n.log.Debug("获取最新节点数据")
		node, err := n.client.CoreV1().Nodes().Get(n.ctx, nodeName, metav1.GetOptions{})
		if err != nil {
			n.log.Errorf("获取节点失败: %s, error=%v", nodeName, err)
			return fmt.Errorf("获取节点失败")
		}

		originalTaintsCount := len(node.Spec.Taints)
		n.log.Debugf("当前节点污点数量: %d", originalTaintsCount)

		newTaints := make([]corev1.Taint, 0, len(node.Spec.Taints))
		found := false

		for _, taint := range node.Spec.Taints {
			if taint.Key == key && taint.Effect == corev1.TaintEffect(effect) {
				found = true
				n.log.Infof("找到要删除的污点: Key=%s, Value=%s, Effect=%s",
					taint.Key, taint.Value, taint.Effect)
				continue
			}
			newTaints = append(newTaints, taint)
		}

		if !found {
			n.log.Infof("节点 %s 上不存在污点 %s:%s，无需删除", nodeName, key, effect)
			return nil
		}

		node.Spec.Taints = newTaints

		n.log.Debug("执行节点更新操作")
		updatedNode, err := n.client.CoreV1().Nodes().Update(n.ctx, node, metav1.UpdateOptions{})
		if err != nil {
			n.log.Errorf("更新节点失败: %v", err)
			return err
		}

		n.log.Infof("成功删除节点 %s 的污点 %s:%s (污点数量: %d -> %d)",
			nodeName, key, effect, originalTaintsCount, len(updatedNode.Spec.Taints))

		return nil
	})

	if retryErr != nil {
		n.log.Errorf("删除节点污点失败: %s, error=%v", nodeName, retryErr)
		return fmt.Errorf("删除节点污点失败")
	}

	n.log.Debug("最终验证污点是否已删除")
	node, err := n.client.CoreV1().Nodes().Get(n.ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		n.log.Errorf("验证节点状态失败: %v", err)
		return fmt.Errorf("验证节点状态失败")
	}

	for _, taint := range node.Spec.Taints {
		if taint.Key == key && taint.Effect == corev1.TaintEffect(effect) {
			n.log.Errorf("污点删除失败，节点 %s 上仍存在污点 %s:%s", nodeName, key, effect)
			return fmt.Errorf("污点删除失败，仍存在该污点")
		}
	}

	n.log.Infof("确认节点 %s 的污点 %s:%s 已成功删除", nodeName, key, effect)
	return nil
}

func (n *nodeOperator) GetTaints(nodeName string) ([]*types.TaintInfo, error) {
	n.log.Debugf("获取节点污点: %s", nodeName)

	var node *corev1.Node
	var err error

	// 优先使用 Informer
	if n.nodeLister != nil {
		node, err = n.nodeLister.Get(nodeName)
		if err != nil && !apierrors.IsNotFound(err) {
			n.log.Errorf("缓存获取节点失败: %s, error=%v", nodeName, err)
			node, err = n.client.CoreV1().Nodes().Get(n.ctx, nodeName, metav1.GetOptions{})
		}
	} else {
		node, err = n.client.CoreV1().Nodes().Get(n.ctx, nodeName, metav1.GetOptions{})
	}

	if err != nil {
		n.log.Errorf("获取节点失败: %s, error=%v", nodeName, err)
		return nil, fmt.Errorf("获取节点失败")
	}

	taints := make([]*types.TaintInfo, 0, len(node.Spec.Taints))
	for _, taint := range node.Spec.Taints {
		timeStr := ""
		if taint.TimeAdded != nil {
			timeStr = taint.TimeAdded.Format("2006-01-02 15:04:05")
		}
		taints = append(taints, &types.TaintInfo{
			Key:      taint.Key,
			Value:    taint.Value,
			Effect:   string(taint.Effect),
			Time:     timeStr,
			IsDelete: !types.IsSystemTaint(taint.Key),
		})
	}

	n.log.Debugf("获取到 %d 个污点", len(taints))
	return taints, nil
}

func (n *nodeOperator) DisableScheduling(nodeName string) error {
	n.log.Infof("禁用节点调度: %s", nodeName)

	node, err := n.getNodeFromCache(nodeName)
	if err != nil {
		return err
	}

	node.Spec.Unschedulable = true
	_, err = n.client.CoreV1().Nodes().Update(n.ctx, node, metav1.UpdateOptions{})
	if err != nil {
		n.log.Errorf("更新节点调度状态失败: %s, error=%v", nodeName, err)
		return fmt.Errorf("更新节点失败")
	}

	n.log.Infof("节点 %s 已禁用调度", nodeName)
	return nil
}

func (n *nodeOperator) EnableScheduling(nodeName string) error {
	n.log.Infof("启用节点调度: %s", nodeName)

	node, err := n.client.CoreV1().Nodes().Get(n.ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		n.log.Errorf("获取节点失败: %s, error=%v", nodeName, err)
		return fmt.Errorf("获取节点失败")
	}

	node.Spec.Unschedulable = false
	_, err = n.client.CoreV1().Nodes().Update(n.ctx, node, metav1.UpdateOptions{})
	if err != nil {
		n.log.Errorf("更新节点调度状态失败: %s, error=%v", nodeName, err)
		return fmt.Errorf("更新节点失败")
	}

	n.log.Infof("节点 %s 已启用调度", nodeName)
	return nil
}

func (n *nodeOperator) DrainNode(nodeName string) error {
	n.log.Infof("开始节点下线流程: %s", nodeName)

	n.log.Info("步骤1：标记节点为不可调度")
	if err := n.DisableScheduling(nodeName); err != nil {
		n.log.Errorf("标记节点为不可调度失败: %v", err)
		return fmt.Errorf("标记节点为不可调度失败")
	}

	n.log.Info("步骤2：获取节点上的所有Pod")
	fieldSelector := fields.OneTermEqualSelector("spec.nodeName", nodeName).String()
	pods, err := n.client.CoreV1().Pods("").List(n.ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		n.log.Errorf("列出节点上的Pod失败: %s, error=%v", nodeName, err)
		return fmt.Errorf("列出节点上的Pod失败")
	}

	n.log.Infof("节点 %s 上有 %d 个Pod", nodeName, len(pods.Items))

	var errors []error
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			n.log.Debugf("跳过已终止的Pod: %s/%s", pod.Namespace, pod.Name)
			continue
		}

		if n.isDaemonSetPod(&pod) {
			n.log.Infof("跳过DaemonSet管理的Pod: %s/%s", pod.Namespace, pod.Name)
			continue
		}

		if n.isMirrorPod(&pod) {
			n.log.Infof("跳过镜像Pod: %s/%s", pod.Namespace, pod.Name)
			continue
		}

		n.log.Infof("驱逐Pod: %s/%s", pod.Namespace, pod.Name)
		if err := n.evictPod(&pod); err != nil {
			errors = append(errors, fmt.Errorf("驱逐Pod失败 %s/%s", pod.Namespace, pod.Name))
			n.log.Errorf("驱逐Pod失败: %s/%s, error=%v", pod.Namespace, pod.Name, err)
		} else {
			n.log.Infof("成功驱逐Pod: %s/%s", pod.Namespace, pod.Name)
		}
	}

	n.log.Info("步骤3：等待所有Pod终止")
	if err := n.waitForPodsDeleted(nodeName); err != nil {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		n.log.Errorf("节点驱逐过程中有 %d 个错误", len(errors))
		return fmt.Errorf("节点驱逐失败，有 %d 个错误", len(errors))
	}

	n.log.Infof("节点 %s 下线成功", nodeName)
	return nil
}

func (n *nodeOperator) evictPod(pod *corev1.Pod) error {
	ctx, cancel := context.WithTimeout(n.ctx, podDeleteTimeout)
	defer cancel()

	eviction := &policyv1.Eviction{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		},
		DeleteOptions: &metav1.DeleteOptions{
			GracePeriodSeconds: pod.Spec.TerminationGracePeriodSeconds,
		},
	}

	n.log.Debugf("执行Pod驱逐: %s/%s", pod.Namespace, pod.Name)
	err := n.client.PolicyV1().Evictions(pod.Namespace).Evict(ctx, eviction)
	if err != nil {
		if apierrors.IsNotFound(err) {
			n.log.Debug("Pod已不存在，无需驱逐")
			return nil
		}
		if apierrors.IsForbidden(err) || apierrors.IsMethodNotSupported(err) {
			n.log.Infof("Eviction API不可用，回退到直接删除Pod: %s/%s",
				pod.Namespace, pod.Name)
			return n.client.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{
				GracePeriodSeconds: pod.Spec.TerminationGracePeriodSeconds,
			})
		}
		return err
	}

	return nil
}

func (n *nodeOperator) waitForPodsDeleted(nodeName string) error {
	n.log.Infof("等待节点 %s 上的Pod被删除", nodeName)

	ctx, cancel := context.WithTimeout(n.ctx, evictionTimeout)
	defer cancel()

	return wait.PollUntilContextCancel(ctx, retryInterval, true, func(ctx context.Context) (bool, error) {
		fieldSelector := fields.OneTermEqualSelector("spec.nodeName", nodeName).String()
		pods, err := n.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{
			FieldSelector: fieldSelector,
		})
		if err != nil {
			n.log.Errorf("列出节点上的Pod失败: %v", err)
			return false, err
		}

		remainingPods := 0
		for _, pod := range pods.Items {
			if n.isDaemonSetPod(&pod) || n.isMirrorPod(&pod) {
				continue
			}
			if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
				continue
			}
			remainingPods++
		}

		if remainingPods > 0 {
			n.log.Infof("等待节点 %s 上的Pod被删除，还剩 %d 个Pod", nodeName, remainingPods)
			return false, nil
		}

		n.log.Infof("节点 %s 上的所有Pod已被删除", nodeName)
		return true, nil
	})
}

func (n *nodeOperator) isDaemonSetPod(pod *corev1.Pod) bool {
	for _, owner := range pod.OwnerReferences {
		if owner.Kind == "DaemonSet" {
			return true
		}
	}
	return false
}

func (n *nodeOperator) isMirrorPod(pod *corev1.Pod) bool {
	if _, exists := pod.Annotations[corev1.MirrorPodAnnotationKey]; exists {
		return true
	}
	return false
}

func (n *nodeOperator) getNodeFromCache(nodeName string) (*corev1.Node, error) {
	n.log.Debugf("获取节点信息: %s", nodeName)

	// 优先使用 Informer
	if n.nodeLister != nil {
		n.log.Debug("从缓存获取节点")
		node, err := n.nodeLister.Get(nodeName)
		if err == nil {
			return node.DeepCopy(), nil
		}
		if !apierrors.IsNotFound(err) {
			n.log.Infof("缓存获取节点失败，降级到API: %s, error=%v", nodeName, err)
		}
	}

	n.log.Debug("从API获取节点")
	node, err := n.client.CoreV1().Nodes().Get(n.ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		n.log.Errorf("获取节点失败: %s, error=%v", nodeName, err)
		return nil, fmt.Errorf("获取节点失败")
	}
	return node, nil
}

func (n *nodeOperator) Describe(nodeName string) (string, error) {
	n.log.Infof("获取节点描述信息: %s", nodeName)

	var node *corev1.Node
	var err error

	// 优先使用 Informer
	if n.nodeLister != nil {
		node, err = n.nodeLister.Get(nodeName)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				n.log.Errorf("缓存获取节点失败: %s, error=%v", nodeName, err)
			}
			node, err = n.client.CoreV1().Nodes().Get(n.ctx, nodeName, metav1.GetOptions{})
		}
	} else {
		node, err = n.client.CoreV1().Nodes().Get(n.ctx, nodeName, metav1.GetOptions{})
	}

	if err != nil {
		if apierrors.IsNotFound(err) {
			n.log.Errorf("节点不存在: %s", nodeName)
			return "", fmt.Errorf("节点 %s 不存在", nodeName)
		}
		n.log.Errorf("获取节点失败: %s, error=%v", nodeName, err)
		return "", fmt.Errorf("获取节点失败")
	}

	var buf strings.Builder

	buf.WriteString(fmt.Sprintf("Name:               %s\n", node.Name))

	roles := n.getNodeRoles(node)
	buf.WriteString(fmt.Sprintf("Roles:              %s\n", roles))

	buf.WriteString("Labels:             ")
	if len(node.Labels) == 0 {
		buf.WriteString("<none>\n")
	} else {
		first := true
		for k, v := range node.Labels {
			if !first {
				buf.WriteString("                    ")
			}
			buf.WriteString(fmt.Sprintf("%s=%s\n", k, v))
			first = false
		}
	}

	buf.WriteString("Annotations:        ")
	if len(node.Annotations) == 0 {
		buf.WriteString("<none>\n")
	} else {
		first := true
		for k, v := range node.Annotations {
			if !first {
				buf.WriteString("                    ")
			}
			displayValue := v
			if len(v) > 80 {
				displayValue = v[:77] + "..."
			}
			buf.WriteString(fmt.Sprintf("%s: %s\n", k, displayValue))
			first = false
		}
	}

	if !node.CreationTimestamp.IsZero() {
		buf.WriteString(fmt.Sprintf("CreationTimestamp:  %s\n", node.CreationTimestamp.Format(time.RFC1123)))
	} else {
		buf.WriteString("CreationTimestamp:  <unset>\n")
	}

	buf.WriteString("Taints:             ")
	if len(node.Spec.Taints) == 0 {
		buf.WriteString("<none>\n")
	} else {
		for i, taint := range node.Spec.Taints {
			if i > 0 {
				buf.WriteString("                    ")
			}
			if taint.Value != "" {
				buf.WriteString(fmt.Sprintf("%s=%s:%s\n", taint.Key, taint.Value, taint.Effect))
			} else {
				buf.WriteString(fmt.Sprintf("%s:%s\n", taint.Key, taint.Effect))
			}
		}
	}

	buf.WriteString(fmt.Sprintf("Unschedulable:      %v\n", node.Spec.Unschedulable))

	buf.WriteString("Conditions:\n")
	buf.WriteString("  Type             Status  LastHeartbeatTime                 LastTransitionTime                Reason                       Message\n")
	buf.WriteString("  ----             ------  -----------------                 ------------------                ------                       -------\n")
	if len(node.Status.Conditions) > 0 {
		for _, cond := range node.Status.Conditions {
			lastHeartbeat := "<unknown>"
			if !cond.LastHeartbeatTime.IsZero() {
				lastHeartbeat = cond.LastHeartbeatTime.Format("Mon, 02 Jan 2006 15:04:05 MST")
			}

			lastTransition := "<unknown>"
			if !cond.LastTransitionTime.IsZero() {
				lastTransition = cond.LastTransitionTime.Format("Mon, 02 Jan 2006 15:04:05 MST")
			}

			message := cond.Message
			if len(message) > 50 {
				message = message[:47] + "..."
			}

			buf.WriteString(fmt.Sprintf("  %-16s %-7s %-33s %-33s %-28s %s\n",
				cond.Type,
				cond.Status,
				lastHeartbeat,
				lastTransition,
				cond.Reason,
				message,
			))
		}
	} else {
		buf.WriteString("  <none>\n")
	}

	buf.WriteString("Addresses:\n")
	if len(node.Status.Addresses) > 0 {
		for _, addr := range node.Status.Addresses {
			buf.WriteString(fmt.Sprintf("  %-16s %s\n", addr.Type+":", addr.Address))
		}
	} else {
		buf.WriteString("  <none>\n")
	}

	buf.WriteString("Capacity:\n")
	if len(node.Status.Capacity) > 0 {
		if cpu := node.Status.Capacity.Cpu(); cpu != nil && !cpu.IsZero() {
			buf.WriteString(fmt.Sprintf("  cpu:                %s\n", cpu.String()))
		}
		if ephemeralStorage := node.Status.Capacity.StorageEphemeral(); ephemeralStorage != nil && !ephemeralStorage.IsZero() {
			buf.WriteString(fmt.Sprintf("  ephemeral-storage:  %s\n", ephemeralStorage.String()))
		}
		for resourceName, quantity := range node.Status.Capacity {
			if strings.HasPrefix(string(resourceName), "hugepages-") {
				buf.WriteString(fmt.Sprintf("  %-19s %s\n", resourceName+":", quantity.String()))
			}
		}
		if memory := node.Status.Capacity.Memory(); memory != nil && !memory.IsZero() {
			buf.WriteString(fmt.Sprintf("  memory:             %s\n", memory.String()))
		}
		if pods := node.Status.Capacity.Pods(); pods != nil && !pods.IsZero() {
			buf.WriteString(fmt.Sprintf("  pods:               %s\n", pods.String()))
		}
	} else {
		buf.WriteString("  <none>\n")
	}

	buf.WriteString("Allocatable:\n")
	if len(node.Status.Allocatable) > 0 {
		if cpu := node.Status.Allocatable.Cpu(); cpu != nil && !cpu.IsZero() {
			buf.WriteString(fmt.Sprintf("  cpu:                %s\n", cpu.String()))
		}
		if ephemeralStorage := node.Status.Allocatable.StorageEphemeral(); ephemeralStorage != nil && !ephemeralStorage.IsZero() {
			buf.WriteString(fmt.Sprintf("  ephemeral-storage:  %s\n", ephemeralStorage.String()))
		}
		for resourceName, quantity := range node.Status.Allocatable {
			if strings.HasPrefix(string(resourceName), "hugepages-") {
				buf.WriteString(fmt.Sprintf("  %-19s %s\n", resourceName+":", quantity.String()))
			}
		}
		if memory := node.Status.Allocatable.Memory(); memory != nil && !memory.IsZero() {
			buf.WriteString(fmt.Sprintf("  memory:             %s\n", memory.String()))
		}
		if pods := node.Status.Allocatable.Pods(); pods != nil && !pods.IsZero() {
			buf.WriteString(fmt.Sprintf("  pods:               %s\n", pods.String()))
		}
	} else {
		buf.WriteString("  <none>\n")
	}

	buf.WriteString("System Info:\n")
	buf.WriteString(fmt.Sprintf("  Machine ID:                 %s\n", node.Status.NodeInfo.MachineID))
	buf.WriteString(fmt.Sprintf("  System UUID:                %s\n", node.Status.NodeInfo.SystemUUID))
	buf.WriteString(fmt.Sprintf("  Boot ID:                    %s\n", node.Status.NodeInfo.BootID))
	buf.WriteString(fmt.Sprintf("  Kernel Version:             %s\n", node.Status.NodeInfo.KernelVersion))
	buf.WriteString(fmt.Sprintf("  OS Image:                   %s\n", node.Status.NodeInfo.OSImage))
	buf.WriteString(fmt.Sprintf("  Operating System:           %s\n", node.Status.NodeInfo.OperatingSystem))
	buf.WriteString(fmt.Sprintf("  Architecture:               %s\n", node.Status.NodeInfo.Architecture))
	buf.WriteString(fmt.Sprintf("  Container Runtime Version:  %s\n", node.Status.NodeInfo.ContainerRuntimeVersion))
	buf.WriteString(fmt.Sprintf("  Kubelet Version:            %s\n", node.Status.NodeInfo.KubeletVersion))
	buf.WriteString(fmt.Sprintf("  Kube-Proxy Version:         %s\n", node.Status.NodeInfo.KubeProxyVersion))

	if node.Spec.PodCIDR != "" {
		buf.WriteString(fmt.Sprintf("PodCIDR:                      %s\n", node.Spec.PodCIDR))
	}
	if len(node.Spec.PodCIDRs) > 0 {
		buf.WriteString(fmt.Sprintf("PodCIDRs:                     %s\n", strings.Join(node.Spec.PodCIDRs, ",")))
	}

	n.log.Debug("获取节点上的Pod列表")
	fieldSelector := fields.OneTermEqualSelector("spec.nodeName", nodeName).String()
	podList, err := n.client.CoreV1().Pods("").List(n.ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})

	var runningPods []corev1.Pod
	var totalCPURequests int64
	var totalMemoryRequests int64
	var totalCPULimits int64
	var totalMemoryLimits int64

	if err == nil {
		for _, pod := range podList.Items {
			if pod.Status.Phase != corev1.PodSucceeded && pod.Status.Phase != corev1.PodFailed {
				runningPods = append(runningPods, pod)

				for _, container := range pod.Spec.Containers {
					if cpu := container.Resources.Requests.Cpu(); cpu != nil {
						totalCPURequests += cpu.MilliValue()
					}
					if memory := container.Resources.Requests.Memory(); memory != nil {
						totalMemoryRequests += memory.Value()
					}
					if cpu := container.Resources.Limits.Cpu(); cpu != nil {
						totalCPULimits += cpu.MilliValue()
					}
					if memory := container.Resources.Limits.Memory(); memory != nil {
						totalMemoryLimits += memory.Value()
					}
				}
			}
		}

		buf.WriteString(fmt.Sprintf("Non-terminated Pods:          (%d in total)\n", len(runningPods)))
		buf.WriteString("  Namespace                   Name                                          CPU Requests  CPU Limits  Memory Requests  Memory Limits  Age\n")
		buf.WriteString("  ---------                   ----                                          ------------  ----------  ---------------  -------------  ---\n")

		if len(runningPods) > 0 {
			for _, pod := range runningPods {
				var podCPURequests int64
				var podMemoryRequests int64
				var podCPULimits int64
				var podMemoryLimits int64

				for _, container := range pod.Spec.Containers {
					if cpu := container.Resources.Requests.Cpu(); cpu != nil {
						podCPURequests += cpu.MilliValue()
					}
					if memory := container.Resources.Requests.Memory(); memory != nil {
						podMemoryRequests += memory.Value()
					}
					if cpu := container.Resources.Limits.Cpu(); cpu != nil {
						podCPULimits += cpu.MilliValue()
					}
					if memory := container.Resources.Limits.Memory(); memory != nil {
						podMemoryLimits += memory.Value()
					}
				}

				age := time.Since(pod.CreationTimestamp.Time)
				ageStr := n.formatDuration(age)

				buf.WriteString(fmt.Sprintf("  %-27s %-45s %-13s %-11s %-16s %-14s %s\n",
					pod.Namespace,
					pod.Name,
					n.formatCPU(podCPURequests),
					n.formatCPU(podCPULimits),
					n.formatMemory(podMemoryRequests),
					n.formatMemory(podMemoryLimits),
					ageStr,
				))
			}
		} else {
			buf.WriteString("  <none>\n")
		}
	} else {
		n.log.Errorf("获取节点Pod列表失败: %v", err)
		buf.WriteString("Non-terminated Pods:          <error fetching pods>\n")
	}

	if err == nil {
		buf.WriteString("Allocated resources:\n")
		buf.WriteString("  (Total limits may be over 100 percent, i.e., overcommitted.)\n")
		buf.WriteString("  Resource           Requests     Limits\n")
		buf.WriteString("  --------           --------     ------\n")

		cpuCapacity := node.Status.Allocatable.Cpu()
		if cpuCapacity != nil && !cpuCapacity.IsZero() {
			cpuRequestPercent := float64(totalCPURequests) / float64(cpuCapacity.MilliValue()) * 100
			cpuLimitPercent := float64(totalCPULimits) / float64(cpuCapacity.MilliValue()) * 100
			buf.WriteString(fmt.Sprintf("  cpu                %-12s %-6s\n",
				fmt.Sprintf("%s (%.0f%%)", n.formatCPU(totalCPURequests), cpuRequestPercent),
				fmt.Sprintf("%s (%.0f%%)", n.formatCPU(totalCPULimits), cpuLimitPercent),
			))
		}

		memCapacity := node.Status.Allocatable.Memory()
		if memCapacity != nil && !memCapacity.IsZero() {
			memRequestPercent := float64(totalMemoryRequests) / float64(memCapacity.Value()) * 100
			memLimitPercent := float64(totalMemoryLimits) / float64(memCapacity.Value()) * 100
			buf.WriteString(fmt.Sprintf("  memory             %-12s %-6s\n",
				fmt.Sprintf("%s (%.0f%%)", n.formatMemory(totalMemoryRequests), memRequestPercent),
				fmt.Sprintf("%s (%.0f%%)", n.formatMemory(totalMemoryLimits), memLimitPercent),
			))
		}

		podsCapacity := node.Status.Allocatable.Pods()
		if podsCapacity != nil && !podsCapacity.IsZero() {
			podsPercent := float64(len(runningPods)) / float64(podsCapacity.Value()) * 100
			buf.WriteString(fmt.Sprintf("  pods               %d (%.0f%%)\n",
				len(runningPods), podsPercent,
			))
		}
	}

	buf.WriteString("Events:\n")
	n.log.Debug("获取节点事件")
	eventList, err := n.client.CoreV1().Events("").List(n.ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Node,involvedObject.uid=%s",
			nodeName, node.UID),
	})

	if err == nil && len(eventList.Items) > 0 {
		buf.WriteString("  Type    Reason                   Age                  From        Message\n")
		buf.WriteString("  ----    ------                   ----                 ----        -------\n")

		events := eventList.Items
		for i := 0; i < len(events)-1; i++ {
			for j := i + 1; j < len(events); j++ {
				if events[i].LastTimestamp.Before(&events[j].LastTimestamp) {
					events[i], events[j] = events[j], events[i]
				}
			}
		}

		limit := 20
		if len(events) < limit {
			limit = len(events)
		}

		for i := 0; i < limit; i++ {
			event := events[i]

			var ageStr string
			if !event.LastTimestamp.IsZero() {
				age := time.Since(event.LastTimestamp.Time)
				ageStr = n.formatDuration(age)
			} else {
				ageStr = "<unknown>"
			}

			message := event.Message
			if len(message) > 80 {
				message = message[:77] + "..."
			}

			source := event.Source.Component
			if event.Source.Host != "" {
				source = fmt.Sprintf("%s, %s", source, event.Source.Host)
			}

			buf.WriteString(fmt.Sprintf("  %-7s %-24s %-20s %-11s %s\n",
				event.Type,
				event.Reason,
				ageStr,
				source,
				message,
			))
		}
	} else if err != nil {
		n.log.Errorf("获取节点事件失败: %v", err)
		buf.WriteString("  <error fetching events>\n")
	} else {
		buf.WriteString("  <none>\n")
	}

	n.log.Infof("成功生成节点描述信息: %s", nodeName)
	return buf.String(), nil
}

func (n *nodeOperator) formatCPU(milliCores int64) string {
	if milliCores == 0 {
		return "0"
	}
	if milliCores < 1000 {
		return fmt.Sprintf("%dm", milliCores)
	}
	return fmt.Sprintf("%.1f", float64(milliCores)/1000.0)
}

func (n *nodeOperator) formatMemory(bytes int64) string {
	if bytes == 0 {
		return "0"
	}

	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	if bytes < KB {
		return fmt.Sprintf("%dB", bytes)
	} else if bytes < MB {
		return fmt.Sprintf("%dKi", bytes/KB)
	} else if bytes < GB {
		return fmt.Sprintf("%dMi", bytes/MB)
	}
	return fmt.Sprintf("%.1fGi", float64(bytes)/float64(GB))
}

func (n *nodeOperator) formatDuration(duration time.Duration) string {
	if duration < time.Minute {
		return fmt.Sprintf("%ds", int(duration.Seconds()))
	} else if duration < time.Hour {
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	} else if duration < 24*time.Hour {
		return fmt.Sprintf("%dh", int(duration.Hours()))
	} else if duration < 7*24*time.Hour {
		return fmt.Sprintf("%dd", int(duration.Hours()/24))
	}
	return fmt.Sprintf("%dw", int(duration.Hours()/24/7))
}
