package workload

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	k8sTypes "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSchedulingConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetSchedulingConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSchedulingConfigLogic {
	return &GetSchedulingConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetSchedulingConfigLogic) GetSchedulingConfig(req *types.DefaultIdRequest) (resp *types.CommSchedulingConfig, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	resourceType := strings.ToUpper(versionDetail.ResourceType)

	// 注意：k8smanager 层返回的是 *SchedulingConfig（旧类型）
	var config *k8sTypes.SchedulingConfig

	switch resourceType {
	case "DEPLOYMENT":
		config, err = client.Deployment().GetSchedulingConfig(versionDetail.Namespace, versionDetail.ResourceName)
	case "STATEFULSET":
		config, err = client.StatefulSet().GetSchedulingConfig(versionDetail.Namespace, versionDetail.ResourceName)
	case "DAEMONSET":
		config, err = client.DaemonSet().GetSchedulingConfig(versionDetail.Namespace, versionDetail.ResourceName)
	case "JOB":
		config, err = client.Job().GetSchedulingConfig(versionDetail.Namespace, versionDetail.ResourceName)
	case "CRONJOB":
		config, err = client.CronJob().GetSchedulingConfig(versionDetail.Namespace, versionDetail.ResourceName)
	default:
		return nil, fmt.Errorf("资源类型 %s 不支持查询调度配置", resourceType)
	}

	if err != nil {
		l.Errorf("获取调度配置失败: %v", err)
		return nil, fmt.Errorf("获取调度配置失败")
	}

	// 将 k8sTypes.SchedulingConfig 转换为 types.CommSchedulingConfig
	resp = convertSchedulingConfigToCommSchedulingConfig(config)
	return resp, nil
}

// convertSchedulingConfigToCommSchedulingConfig 将 k8sTypes.SchedulingConfig 转换为 types.CommSchedulingConfig
func convertSchedulingConfigToCommSchedulingConfig(config *k8sTypes.SchedulingConfig) *types.CommSchedulingConfig {
	if config == nil {
		return &types.CommSchedulingConfig{}
	}

	result := &types.CommSchedulingConfig{
		NodeSelector: config.NodeSelector,
		NodeName:     config.NodeName,
	}

	// 转换 Affinity
	if config.Affinity != nil {
		result.Affinity = convertAffinityConfigToCommAffinity(config.Affinity)
	}

	// 转换 Tolerations
	if len(config.Tolerations) > 0 {
		result.Tolerations = convertTolerationConfigsToCommTolerations(config.Tolerations)
	}

	// 转换 TopologySpreadConstraints
	if len(config.TopologySpreadConstraints) > 0 {
		result.TopologySpreadConstraints = convertTopologySpreadConstraintConfigsToComm(config.TopologySpreadConstraints)
	}

	return result
}

// convertAffinityConfigToCommAffinity 转换亲和性配置
func convertAffinityConfigToCommAffinity(affinity *k8sTypes.AffinityConfig) *types.CommAffinityConfig {
	if affinity == nil {
		return nil
	}

	result := &types.CommAffinityConfig{}

	// 转换 NodeAffinity
	if affinity.NodeAffinity != nil {
		result.NodeAffinity = &types.CommNodeAffinity{}

		if affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
			result.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = convertNodeSelectorConfigToCommNodeSelector(
				affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution,
			)
		}

		if len(affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution) > 0 {
			result.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = convertPreferredSchedulingTermConfigsToComm(
				affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution,
			)
		}
	}

	// 转换 PodAffinity
	if affinity.PodAffinity != nil {
		result.PodAffinity = convertPodAffinityConfigToCommPodAffinity(affinity.PodAffinity)
	}

	// 转换 PodAntiAffinity
	if affinity.PodAntiAffinity != nil {
		result.PodAntiAffinity = convertPodAntiAffinityConfigToCommPodAntiAffinity(affinity.PodAntiAffinity)
	}

	return result
}

// convertNodeSelectorConfigToCommNodeSelector 转换节点选择器
func convertNodeSelectorConfigToCommNodeSelector(selector *k8sTypes.NodeSelectorConfig) *types.CommNodeSelector {
	if selector == nil {
		return nil
	}

	result := &types.CommNodeSelector{
		NodeSelectorTerms: make([]types.CommNodeSelectorTerm, 0, len(selector.NodeSelectorTerms)),
	}

	for _, term := range selector.NodeSelectorTerms {
		result.NodeSelectorTerms = append(result.NodeSelectorTerms, types.CommNodeSelectorTerm{
			MatchExpressions: convertNodeSelectorRequirementConfigsToComm(term.MatchExpressions),
			MatchFields:      convertNodeSelectorRequirementConfigsToComm(term.MatchFields),
		})
	}

	return result
}

// convertNodeSelectorRequirementConfigsToComm 转换节点选择器要求
func convertNodeSelectorRequirementConfigsToComm(reqs []k8sTypes.NodeSelectorRequirementConfig) []types.CommNodeSelectorRequirement {
	result := make([]types.CommNodeSelectorRequirement, 0, len(reqs))
	for _, req := range reqs {
		result = append(result, types.CommNodeSelectorRequirement{
			Key:      req.Key,
			Operator: req.Operator,
			Values:   req.Values,
		})
	}
	return result
}

// convertPreferredSchedulingTermConfigsToComm 转换偏好调度条件
func convertPreferredSchedulingTermConfigsToComm(terms []k8sTypes.PreferredSchedulingTermConfig) []types.CommPreferredSchedulingTerm {
	result := make([]types.CommPreferredSchedulingTerm, 0, len(terms))
	for _, term := range terms {
		result = append(result, types.CommPreferredSchedulingTerm{
			Weight: term.Weight,
			Preference: types.CommNodeSelectorTerm{
				MatchExpressions: convertNodeSelectorRequirementConfigsToComm(term.Preference.MatchExpressions),
				MatchFields:      convertNodeSelectorRequirementConfigsToComm(term.Preference.MatchFields),
			},
		})
	}
	return result
}

// convertPodAffinityConfigToCommPodAffinity 转换 Pod 亲和性
func convertPodAffinityConfigToCommPodAffinity(affinity *k8sTypes.PodAffinityConfig) *types.CommPodAffinity {
	if affinity == nil {
		return nil
	}

	result := &types.CommPodAffinity{}

	if len(affinity.RequiredDuringSchedulingIgnoredDuringExecution) > 0 {
		result.RequiredDuringSchedulingIgnoredDuringExecution = convertPodAffinityTermConfigsToComm(
			affinity.RequiredDuringSchedulingIgnoredDuringExecution,
		)
	}

	if len(affinity.PreferredDuringSchedulingIgnoredDuringExecution) > 0 {
		result.PreferredDuringSchedulingIgnoredDuringExecution = convertWeightedPodAffinityTermConfigsToComm(
			affinity.PreferredDuringSchedulingIgnoredDuringExecution,
		)
	}

	return result
}

// convertPodAntiAffinityConfigToCommPodAntiAffinity 转换 Pod 反亲和性
func convertPodAntiAffinityConfigToCommPodAntiAffinity(affinity *k8sTypes.PodAntiAffinityConfig) *types.CommPodAntiAffinity {
	if affinity == nil {
		return nil
	}

	result := &types.CommPodAntiAffinity{}

	if len(affinity.RequiredDuringSchedulingIgnoredDuringExecution) > 0 {
		result.RequiredDuringSchedulingIgnoredDuringExecution = convertPodAffinityTermConfigsToComm(
			affinity.RequiredDuringSchedulingIgnoredDuringExecution,
		)
	}

	if len(affinity.PreferredDuringSchedulingIgnoredDuringExecution) > 0 {
		result.PreferredDuringSchedulingIgnoredDuringExecution = convertWeightedPodAffinityTermConfigsToComm(
			affinity.PreferredDuringSchedulingIgnoredDuringExecution,
		)
	}

	return result
}

// convertPodAffinityTermConfigsToComm 转换 Pod 亲和性条件
func convertPodAffinityTermConfigsToComm(terms []k8sTypes.PodAffinityTermConfig) []types.CommPodAffinityTerm {
	result := make([]types.CommPodAffinityTerm, 0, len(terms))
	for _, term := range terms {
		converted := types.CommPodAffinityTerm{
			Namespaces:  term.Namespaces,
			TopologyKey: term.TopologyKey,
		}
		if term.LabelSelector != nil {
			converted.LabelSelector = convertLabelSelectorConfigToComm(term.LabelSelector)
		}
		if term.NamespaceSelector != nil {
			converted.NamespaceSelector = convertLabelSelectorConfigToComm(term.NamespaceSelector)
		}
		result = append(result, converted)
	}
	return result
}

// convertWeightedPodAffinityTermConfigsToComm 转换带权重的 Pod 亲和性条件
func convertWeightedPodAffinityTermConfigsToComm(terms []k8sTypes.WeightedPodAffinityTermConfig) []types.CommWeightedPodAffinityTerm {
	result := make([]types.CommWeightedPodAffinityTerm, 0, len(terms))
	for _, term := range terms {
		converted := types.CommWeightedPodAffinityTerm{
			Weight: term.Weight,
			PodAffinityTerm: types.CommPodAffinityTerm{
				Namespaces:  term.PodAffinityTerm.Namespaces,
				TopologyKey: term.PodAffinityTerm.TopologyKey,
			},
		}
		if term.PodAffinityTerm.LabelSelector != nil {
			converted.PodAffinityTerm.LabelSelector = convertLabelSelectorConfigToComm(term.PodAffinityTerm.LabelSelector)
		}
		if term.PodAffinityTerm.NamespaceSelector != nil {
			converted.PodAffinityTerm.NamespaceSelector = convertLabelSelectorConfigToComm(term.PodAffinityTerm.NamespaceSelector)
		}
		result = append(result, converted)
	}
	return result
}

// convertLabelSelectorConfigToComm 转换标签选择器
func convertLabelSelectorConfigToComm(selector *k8sTypes.LabelSelectorConfig) *types.CommLabelSelectorConfig {
	if selector == nil {
		return nil
	}

	result := &types.CommLabelSelectorConfig{
		MatchLabels: selector.MatchLabels,
	}

	if len(selector.MatchExpressions) > 0 {
		result.MatchExpressions = make([]types.CommLabelSelectorRequirement, 0, len(selector.MatchExpressions))
		for _, expr := range selector.MatchExpressions {
			result.MatchExpressions = append(result.MatchExpressions, types.CommLabelSelectorRequirement{
				Key:      expr.Key,
				Operator: expr.Operator,
				Values:   expr.Values,
			})
		}
	}

	return result
}

// convertTolerationConfigsToCommTolerations 转换容忍配置
func convertTolerationConfigsToCommTolerations(tolerations []k8sTypes.TolerationConfig) []types.CommToleration {
	result := make([]types.CommToleration, 0, len(tolerations))
	for _, t := range tolerations {
		toleration := types.CommToleration{
			Key:      t.Key,
			Operator: t.Operator,
			Value:    t.Value,
			Effect:   t.Effect,
		}
		if t.TolerationSeconds != nil {
			toleration.TolerationSeconds = *t.TolerationSeconds
		}
		result = append(result, toleration)
	}
	return result
}

// convertTopologySpreadConstraintConfigsToComm 转换拓扑分布约束
func convertTopologySpreadConstraintConfigsToComm(constraints []k8sTypes.TopologySpreadConstraintConfig) []types.CommTopologySpreadConstraint {
	result := make([]types.CommTopologySpreadConstraint, 0, len(constraints))
	for _, c := range constraints {
		converted := types.CommTopologySpreadConstraint{
			MaxSkew:           c.MaxSkew,
			TopologyKey:       c.TopologyKey,
			WhenUnsatisfiable: c.WhenUnsatisfiable,
		}

		if c.MinDomains != nil {
			converted.MinDomains = *c.MinDomains
		}

		if c.NodeAffinityPolicy != nil {
			converted.NodeAffinityPolicy = *c.NodeAffinityPolicy
		}

		if c.NodeTaintsPolicy != nil {
			converted.NodeTaintsPolicy = *c.NodeTaintsPolicy
		}

		if c.LabelSelector != nil {
			converted.LabelSelector = convertLabelSelectorConfigToComm(c.LabelSelector)
		}

		result = append(result, converted)
	}
	return result
}
