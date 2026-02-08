package operator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	networkingv1lister "k8s.io/client-go/listers/networking/v1"
	"sigs.k8s.io/yaml"
)

type networkPolicyOperator struct {
	BaseOperator
	client          kubernetes.Interface
	informerFactory informers.SharedInformerFactory
	npLister        networkingv1lister.NetworkPolicyLister
}

func NewNetworkPolicyOperator(ctx context.Context, client kubernetes.Interface) types.NetworkPolicyOperator {
	return &networkPolicyOperator{
		BaseOperator: NewBaseOperator(ctx, false),
		client:       client,
	}
}

func NewNetworkPolicyOperatorWithInformer(
	ctx context.Context,
	client kubernetes.Interface,
	informerFactory informers.SharedInformerFactory,
) types.NetworkPolicyOperator {
	var npLister networkingv1lister.NetworkPolicyLister

	if informerFactory != nil {
		npLister = informerFactory.Networking().V1().NetworkPolicies().Lister()
	}

	return &networkPolicyOperator{
		BaseOperator:    NewBaseOperator(ctx, informerFactory != nil),
		client:          client,
		informerFactory: informerFactory,
		npLister:        npLister,
	}
}

func (n *networkPolicyOperator) Create(np *networkingv1.NetworkPolicy) (*networkingv1.NetworkPolicy, error) {
	if np == nil || np.Name == "" || np.Namespace == "" {
		return nil, fmt.Errorf("NetworkPolicy对象、名称和命名空间不能为空")
	}
	injectCommonAnnotations(np)
	created, err := n.client.NetworkingV1().NetworkPolicies(np.Namespace).Create(n.ctx, np, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("创建NetworkPolicy失败: %v", err)
	}

	return created, nil
}

func (n *networkPolicyOperator) Update(np *networkingv1.NetworkPolicy) (*networkingv1.NetworkPolicy, error) {
	if np == nil || np.Name == "" || np.Namespace == "" {
		return nil, fmt.Errorf("NetworkPolicy对象、名称和命名空间不能为空")
	}

	updated, err := n.client.NetworkingV1().NetworkPolicies(np.Namespace).Update(n.ctx, np, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("更新NetworkPolicy失败: %v", err)
	}

	return updated, nil
}

func (n *networkPolicyOperator) Delete(namespace, name string) error {
	if namespace == "" || name == "" {
		return fmt.Errorf("命名空间和名称不能为空")
	}

	err := n.client.NetworkingV1().NetworkPolicies(namespace).Delete(n.ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("删除NetworkPolicy失败: %v", err)
	}

	return nil
}

func (n *networkPolicyOperator) Get(namespace, name string) (*networkingv1.NetworkPolicy, error) {
	if namespace == "" || name == "" {
		return nil, fmt.Errorf("命名空间和名称不能为空")
	}

	if n.npLister != nil {
		np, err := n.npLister.NetworkPolicies(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("NetworkPolicy %s/%s 不存在", namespace, name)
			}
			np, apiErr := n.client.NetworkingV1().NetworkPolicies(namespace).Get(n.ctx, name, metav1.GetOptions{})
			if apiErr != nil {
				return nil, fmt.Errorf("获取NetworkPolicy失败: %v", apiErr)
			}
			np.TypeMeta = metav1.TypeMeta{
				Kind:       "NetworkPolicy",
				APIVersion: "networking.k8s.io/v1",
			}
			return np, nil
		}
		np.TypeMeta = metav1.TypeMeta{
			Kind:       "NetworkPolicy",
			APIVersion: "networking.k8s.io/v1",
		}
		return np, nil
	}

	np, err := n.client.NetworkingV1().NetworkPolicies(namespace).Get(n.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("NetworkPolicy %s/%s 不存在", namespace, name)
		}
		return nil, fmt.Errorf("获取NetworkPolicy失败: %v", err)
	}

	np.TypeMeta = metav1.TypeMeta{
		Kind:       "NetworkPolicy",
		APIVersion: "networking.k8s.io/v1",
	}

	return np, nil
}

func (n *networkPolicyOperator) List(namespace string, search string, labelSelector string) (*types.ListNetworkPolicyResponse, error) {
	var selector labels.Selector = labels.Everything()
	if labelSelector != "" {
		parsedSelector, err := labels.Parse(labelSelector)
		if err != nil {
			return nil, fmt.Errorf("解析标签选择器失败: %v", err)
		}
		selector = parsedSelector
	}

	var networkPolicies []*networkingv1.NetworkPolicy
	var err error

	if n.useInformer && n.npLister != nil {
		networkPolicies, err = n.npLister.NetworkPolicies(namespace).List(selector)
		if err != nil {
			return nil, fmt.Errorf("获取NetworkPolicy列表失败: %v", err)
		}
	} else {
		listOpts := metav1.ListOptions{LabelSelector: selector.String()}
		npList, err := n.client.NetworkingV1().NetworkPolicies(namespace).List(n.ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("获取NetworkPolicy列表失败: %v", err)
		}
		networkPolicies = make([]*networkingv1.NetworkPolicy, len(npList.Items))
		for i := range npList.Items {
			networkPolicies[i] = &npList.Items[i]
		}
	}

	// 搜索过滤
	if search != "" {
		filtered := make([]*networkingv1.NetworkPolicy, 0)
		searchLower := strings.ToLower(search)
		for _, np := range networkPolicies {
			if strings.Contains(strings.ToLower(np.Name), searchLower) {
				filtered = append(filtered, np)
			}
		}
		networkPolicies = filtered
	}

	// 转换为响应格式
	items := make([]types.NetworkPolicyInfo, len(networkPolicies))
	for i, np := range networkPolicies {
		items[i] = n.convertToInfo(np)
	}

	return &types.ListNetworkPolicyResponse{
		Total: len(items),
		Items: items,
	}, nil
}

func (n *networkPolicyOperator) Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return n.client.NetworkingV1().NetworkPolicies(namespace).Watch(n.ctx, opts)
}

func (n *networkPolicyOperator) UpdateLabels(namespace, name string, labels map[string]string) error {
	np, err := n.Get(namespace, name)
	if err != nil {
		return err
	}

	if np.Labels == nil {
		np.Labels = make(map[string]string)
	}
	for k, v := range labels {
		np.Labels[k] = v
	}

	_, err = n.Update(np)
	return err
}

func (n *networkPolicyOperator) UpdateAnnotations(namespace, name string, annotations map[string]string) error {
	np, err := n.Get(namespace, name)
	if err != nil {
		return err
	}

	if np.Annotations == nil {
		np.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		np.Annotations[k] = v
	}

	_, err = n.Update(np)
	return err
}

func (n *networkPolicyOperator) GetYaml(namespace, name string) (string, error) {
	np, err := n.Get(namespace, name)
	if err != nil {
		return "", err
	}

	// 确保有 TypeMeta
	np.TypeMeta = metav1.TypeMeta{
		Kind:       "NetworkPolicy",
		APIVersion: "networking.k8s.io/v1",
	}

	np.ManagedFields = nil
	yamlBytes, err := yaml.Marshal(np)
	if err != nil {
		return "", fmt.Errorf("转换为YAML失败: %v", err)
	}

	return string(yamlBytes), nil
}

func (n *networkPolicyOperator) GetAffectedPods(namespace, name string) (*types.NetworkPolicyAffectedPodsResponse, error) {
	np, err := n.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	response := &types.NetworkPolicyAffectedPodsResponse{
		NetworkPolicyName: name,
		NetworkPolicyNS:   namespace,
		AffectedPods:      make([]types.NetworkPolicyAffectedPod, 0),
	}

	// 获取命名空间下的所有 Pod
	pods, err := n.client.CoreV1().Pods(namespace).List(n.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取Pod列表失败: %v", err)
	}

	// 构建 NetworkPolicy 的 Pod 选择器
	selector, err := metav1.LabelSelectorAsSelector(&np.Spec.PodSelector)
	if err != nil {
		return nil, fmt.Errorf("解析Pod选择器失败: %v", err)
	}

	// 检查每个 Pod 是否匹配 NetworkPolicy 的 Pod 选择器
	for _, pod := range pods.Items {
		matched := selector.Matches(labels.Set(pod.Labels))
		reason := ""
		if matched {
			reason = fmt.Sprintf("Pod标签 %v 匹配 NetworkPolicy 选择器", pod.Labels)
		}

		response.AffectedPods = append(response.AffectedPods, types.NetworkPolicyAffectedPod{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Labels:    pod.Labels,
			Matched:   matched,
			Reason:    reason,
		})
	}

	// 计算匹配的 Pod 数量
	for _, pod := range response.AffectedPods {
		if pod.Matched {
			response.TotalAffected++
		}
	}

	return response, nil
}

// convertToInfo 将 NetworkPolicy 转换为 Info 结构
func (n *networkPolicyOperator) convertToInfo(np *networkingv1.NetworkPolicy) types.NetworkPolicyInfo {
	info := types.NetworkPolicyInfo{
		Name:              np.Name,
		Namespace:         np.Namespace,
		Labels:            np.Labels,
		Annotations:       np.Annotations,
		Age:               time.Since(np.CreationTimestamp.Time).String(),
		CreationTimestamp: np.CreationTimestamp.UnixMilli(),
		PodSelector:       np.Spec.PodSelector.MatchLabels,
		PolicyTypes:       make([]string, 0, len(np.Spec.PolicyTypes)),
	}

	// 转换 Pod 选择器表达式
	for _, expr := range np.Spec.PodSelector.MatchExpressions {
		info.PodSelectorMatchExpressions = append(info.PodSelectorMatchExpressions, types.NPMatchExpression{
			Key:      expr.Key,
			Operator: string(expr.Operator),
			Values:   expr.Values,
		})
	}

	// 转换策略类型
	for _, pt := range np.Spec.PolicyTypes {
		info.PolicyTypes = append(info.PolicyTypes, string(pt))
	}

	// 转换入站规则
	for _, rule := range np.Spec.Ingress {
		ingressRule := types.NPIngressRuleInfo{
			Ports: make([]types.NPNetworkPolicyPortInfo, 0, len(rule.Ports)),
			From:  make([]types.NPNetworkPolicyPeerInfo, 0, len(rule.From)),
		}

		for _, port := range rule.Ports {
			portInfo := types.NPNetworkPolicyPortInfo{
				Protocol:  string(*port.Protocol),
				NamedPort: "",
			}
			if port.Port != nil {
				if port.Port.Type == intstr.Int {
					portInfo.Port = port.Port.IntVal
				} else {
					portInfo.NamedPort = port.Port.StrVal
				}
			}
			if port.EndPort != nil {
				portInfo.EndPort = *port.EndPort
			}
			ingressRule.Ports = append(ingressRule.Ports, portInfo)
		}

		for _, peer := range rule.From {
			peerInfo := n.convertPeerInfo(peer)
			ingressRule.From = append(ingressRule.From, peerInfo)
		}

		info.IngressRules = append(info.IngressRules, ingressRule)
	}

	// 转换出站规则
	for _, rule := range np.Spec.Egress {
		egressRule := types.NPEgressRuleInfo{
			Ports: make([]types.NPNetworkPolicyPortInfo, 0, len(rule.Ports)),
			To:    make([]types.NPNetworkPolicyPeerInfo, 0, len(rule.To)),
		}

		for _, port := range rule.Ports {
			portInfo := types.NPNetworkPolicyPortInfo{
				Protocol:  string(*port.Protocol),
				NamedPort: "",
			}
			if port.Port != nil {
				if port.Port.Type == intstr.Int {
					portInfo.Port = port.Port.IntVal
				} else {
					portInfo.NamedPort = port.Port.StrVal
				}
			}
			if port.EndPort != nil {
				portInfo.EndPort = *port.EndPort
			}
			egressRule.Ports = append(egressRule.Ports, portInfo)
		}

		for _, peer := range rule.To {
			peerInfo := n.convertPeerInfo(peer)
			egressRule.To = append(egressRule.To, peerInfo)
		}

		info.EgressRules = append(info.EgressRules, egressRule)
	}

	return info
}

// convertPeerInfo 将 NetworkPolicyPeer 转换为 PeerInfo 结构
func (n *networkPolicyOperator) convertPeerInfo(peer networkingv1.NetworkPolicyPeer) types.NPNetworkPolicyPeerInfo {
	peerInfo := types.NPNetworkPolicyPeerInfo{}

	if peer.PodSelector != nil {
		peerInfo.PodSelector = peer.PodSelector.MatchLabels
		for _, expr := range peer.PodSelector.MatchExpressions {
			peerInfo.PodSelectorExprs = append(peerInfo.PodSelectorExprs, types.NPMatchExpression{
				Key:      expr.Key,
				Operator: string(expr.Operator),
				Values:   expr.Values,
			})
		}
	}

	if peer.NamespaceSelector != nil {
		peerInfo.NamespaceSelector = peer.NamespaceSelector.MatchLabels
		for _, expr := range peer.NamespaceSelector.MatchExpressions {
			peerInfo.NamespaceSelectorExprs = append(peerInfo.NamespaceSelectorExprs, types.NPMatchExpression{
				Key:      expr.Key,
				Operator: string(expr.Operator),
				Values:   expr.Values,
			})
		}
	}

	if peer.IPBlock != nil {
		peerInfo.IPBlock = &types.NPIPBlockInfo{
			CIDR:   peer.IPBlock.CIDR,
			Except: peer.IPBlock.Except,
		}
	}

	return peerInfo
}

// BuildNetworkPolicyFromRequest 从请求构建 NetworkPolicy 对象
func BuildNetworkPolicyFromRequest(req *types.NetworkPolicyRequest) *networkingv1.NetworkPolicy {
	np := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:        req.Name,
			Namespace:   req.Namespace,
			Labels:      req.Labels,
			Annotations: req.Annotations,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: req.PodSelector.MatchLabels,
			},
			PolicyTypes: make([]networkingv1.PolicyType, 0, len(req.PolicyTypes)),
		},
	}

	// 设置 Pod 选择器表达式
	if len(req.PodSelector.MatchExpressions) > 0 {
		np.Spec.PodSelector.MatchExpressions = make([]metav1.LabelSelectorRequirement, len(req.PodSelector.MatchExpressions))
		for i, expr := range req.PodSelector.MatchExpressions {
			np.Spec.PodSelector.MatchExpressions[i] = metav1.LabelSelectorRequirement{
				Key:      expr.Key,
				Operator: metav1.LabelSelectorOperator(expr.Operator),
				Values:   expr.Values,
			}
		}
	}

	// 设置策略类型
	for _, pt := range req.PolicyTypes {
		np.Spec.PolicyTypes = append(np.Spec.PolicyTypes, networkingv1.PolicyType(pt))
	}

	// 设置入站规则
	if len(req.Ingress) > 0 {
		np.Spec.Ingress = make([]networkingv1.NetworkPolicyIngressRule, len(req.Ingress))
		for i, rule := range req.Ingress {
			np.Spec.Ingress[i] = networkingv1.NetworkPolicyIngressRule{
				Ports: convertPortsConfig(rule.Ports),
				From:  convertPeersConfig(rule.From),
			}
		}
	}

	// 设置出站规则
	if len(req.Egress) > 0 {
		np.Spec.Egress = make([]networkingv1.NetworkPolicyEgressRule, len(req.Egress))
		for i, rule := range req.Egress {
			np.Spec.Egress[i] = networkingv1.NetworkPolicyEgressRule{
				Ports: convertPortsConfig(rule.Ports),
				To:    convertPeersConfig(rule.To),
			}
		}
	}

	return np
}

// convertPortsConfig 转换端口配置
func convertPortsConfig(ports []types.NPNetworkPolicyPortConfig) []networkingv1.NetworkPolicyPort {
	if ports == nil {
		return nil
	}

	result := make([]networkingv1.NetworkPolicyPort, len(ports))
	for i, port := range ports {
		npPort := networkingv1.NetworkPolicyPort{
			Protocol: convertProtocol(port.Protocol),
		}

		if port.NamedPort != "" {
			npPort.Port = &intstr.IntOrString{
				Type:   intstr.String,
				StrVal: port.NamedPort,
			}
		} else if port.Port > 0 {
			npPort.Port = &intstr.IntOrString{
				Type:   intstr.Int,
				IntVal: port.Port,
			}
		}

		if port.EndPort > 0 {
			npPort.EndPort = &port.EndPort
		}

		result[i] = npPort
	}

	return result
}

// convertPeersConfig 转换对端配置
func convertPeersConfig(peers []types.NPNetworkPolicyPeerConfig) []networkingv1.NetworkPolicyPeer {
	if peers == nil {
		return nil
	}

	result := make([]networkingv1.NetworkPolicyPeer, len(peers))
	for i, peer := range peers {
		npPeer := networkingv1.NetworkPolicyPeer{}

		if peer.PodSelector != nil {
			npPeer.PodSelector = &metav1.LabelSelector{
				MatchLabels: peer.PodSelector.MatchLabels,
			}
			if len(peer.PodSelector.MatchExpressions) > 0 {
				npPeer.PodSelector.MatchExpressions = make([]metav1.LabelSelectorRequirement, len(peer.PodSelector.MatchExpressions))
				for j, expr := range peer.PodSelector.MatchExpressions {
					npPeer.PodSelector.MatchExpressions[j] = metav1.LabelSelectorRequirement{
						Key:      expr.Key,
						Operator: metav1.LabelSelectorOperator(expr.Operator),
						Values:   expr.Values,
					}
				}
			}
		}

		if peer.NamespaceSelector != nil {
			npPeer.NamespaceSelector = &metav1.LabelSelector{
				MatchLabels: peer.NamespaceSelector.MatchLabels,
			}
			if len(peer.NamespaceSelector.MatchExpressions) > 0 {
				npPeer.NamespaceSelector.MatchExpressions = make([]metav1.LabelSelectorRequirement, len(peer.NamespaceSelector.MatchExpressions))
				for j, expr := range peer.NamespaceSelector.MatchExpressions {
					npPeer.NamespaceSelector.MatchExpressions[j] = metav1.LabelSelectorRequirement{
						Key:      expr.Key,
						Operator: metav1.LabelSelectorOperator(expr.Operator),
						Values:   expr.Values,
					}
				}
			}
		}

		if peer.IPBlock != nil {
			npPeer.IPBlock = &networkingv1.IPBlock{
				CIDR:   peer.IPBlock.CIDR,
				Except: peer.IPBlock.Except,
			}
		}

		result[i] = npPeer
	}

	return result
}

// convertProtocol 转换协议字符串
func convertProtocol(protocol string) *corev1.Protocol {
	if protocol == "" {
		tcp := corev1.ProtocolTCP
		return &tcp
	}

	p := corev1.Protocol(protocol)
	return &p
}
