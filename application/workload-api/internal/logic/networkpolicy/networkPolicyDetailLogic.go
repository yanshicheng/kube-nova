package networkpolicy

import (
	"context"
	"fmt"
	"time"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type NetworkPolicyDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 NetworkPolicy 详情
func NewNetworkPolicyDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NetworkPolicyDetailLogic {
	return &NetworkPolicyDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *NetworkPolicyDetailLogic) NetworkPolicyDetail(req *types.NetworkPolicyNameRequest) (resp *types.NetworkPolicyDetail, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}

	// 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	// 获取 NetworkPolicy 操作器
	networkPolicyOp := client.NetworkPolicies()

	// 调用 Get 方法获取 NetworkPolicy 对象
	np, err := networkPolicyOp.Get(req.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取 NetworkPolicy 详情失败: %v", err)
		return nil, fmt.Errorf("获取 NetworkPolicy 详情失败")
	}

	// 转换 Pod 选择器表达式
	podSelectorMatchExpressions := make([]types.NPMatchExpression, 0)
	for _, expr := range np.Spec.PodSelector.MatchExpressions {
		podSelectorMatchExpressions = append(podSelectorMatchExpressions, types.NPMatchExpression{
			Key:      expr.Key,
			Operator: string(expr.Operator),
			Values:   expr.Values,
		})
	}

	// 转换入站规则
	ingressRules := make([]types.NPIngressRuleInfo, 0)
	for _, rule := range np.Spec.Ingress {
		ingressRule := types.NPIngressRuleInfo{
			Ports: convertK8sPortsToAPIPorts(rule.Ports),
			From:  convertK8sPeersToAPIPeers(rule.From),
		}
		ingressRules = append(ingressRules, ingressRule)
	}

	// 转换出站规则
	egressRules := make([]types.NPEgressRuleInfo, 0)
	for _, rule := range np.Spec.Egress {
		egressRule := types.NPEgressRuleInfo{
			Ports: convertK8sPortsToAPIPorts(rule.Ports),
			To:    convertK8sPeersToAPIPeers(rule.To),
		}
		egressRules = append(egressRules, egressRule)
	}

	// 转换策略类型
	policyTypes := make([]string, 0, len(np.Spec.PolicyTypes))
	for _, pt := range np.Spec.PolicyTypes {
		policyTypes = append(policyTypes, string(pt))
	}

	// 转换时间戳
	creationTimestamp := int64(0)
	if !np.CreationTimestamp.Time.IsZero() {
		creationTimestamp = np.CreationTimestamp.UnixMilli()
	}

	// 计算年龄
	age := time.Since(np.CreationTimestamp.Time).String()

	detail := &types.NetworkPolicyDetail{
		Name:                        np.Name,
		Namespace:                   np.Namespace,
		PodSelector:                 np.Spec.PodSelector.MatchLabels,
		PodSelectorMatchExpressions: podSelectorMatchExpressions,
		IngressRules:                ingressRules,
		EgressRules:                 egressRules,
		PolicyTypes:                 policyTypes,
		Labels:                      np.Labels,
		Annotations:                 np.Annotations,
		Age:                         age,
		CreationTimestamp:           creationTimestamp,
	}

	l.Infof("用户: %s, 成功获取 NetworkPolicy %s/%s 的详情", username, req.Namespace, req.Name)

	return detail, nil
}

// convertK8sPortsToAPIPorts 转换 K8s 端口到 API 端口格式
func convertK8sPortsToAPIPorts(ports []networkingv1.NetworkPolicyPort) []types.NPNetworkPolicyPortInfo {
	if ports == nil {
		return []types.NPNetworkPolicyPortInfo{}
	}
	result := make([]types.NPNetworkPolicyPortInfo, len(ports))
	for i, port := range ports {
		portInfo := types.NPNetworkPolicyPortInfo{
			Protocol:  "",
			Port:      0,
			NamedPort: "",
		}
		if port.Protocol != nil {
			portInfo.Protocol = string(*port.Protocol)
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
		result[i] = portInfo
	}
	return result
}

// convertK8sPeersToAPIPeers 转换 K8s 对端到 API 对端格式
func convertK8sPeersToAPIPeers(peers []networkingv1.NetworkPolicyPeer) []types.NPNetworkPolicyPeerInfo {
	if peers == nil {
		return []types.NPNetworkPolicyPeerInfo{}
	}
	result := make([]types.NPNetworkPolicyPeerInfo, len(peers))
	for i, peer := range peers {
		podSelectorExprs := make([]types.NPMatchExpression, 0)
		if peer.PodSelector != nil {
			for _, expr := range peer.PodSelector.MatchExpressions {
				podSelectorExprs = append(podSelectorExprs, types.NPMatchExpression{
					Key:      expr.Key,
					Operator: string(expr.Operator),
					Values:   expr.Values,
				})
			}
		}

		namespaceSelectorExprs := make([]types.NPMatchExpression, 0)
		if peer.NamespaceSelector != nil {
			for _, expr := range peer.NamespaceSelector.MatchExpressions {
				namespaceSelectorExprs = append(namespaceSelectorExprs, types.NPMatchExpression{
					Key:      expr.Key,
					Operator: string(expr.Operator),
					Values:   expr.Values,
				})
			}
		}

		peerInfo := types.NPNetworkPolicyPeerInfo{
			PodSelector:            map[string]string{},
			PodSelectorExprs:       podSelectorExprs,
			NamespaceSelector:      map[string]string{},
			NamespaceSelectorExprs: namespaceSelectorExprs,
			IPBlock:                nil,
		}

		if peer.PodSelector != nil {
			peerInfo.PodSelector = peer.PodSelector.MatchLabels
		}
		if peer.NamespaceSelector != nil {
			peerInfo.NamespaceSelector = peer.NamespaceSelector.MatchLabels
		}
		if peer.IPBlock != nil {
			peerInfo.IPBlock = &types.NPIPBlockInfo{
				CIDR:   peer.IPBlock.CIDR,
				Except: peer.IPBlock.Except,
			}
		}

		result[i] = peerInfo
	}
	return result
}
