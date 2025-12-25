package workload

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	types2 "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSchedulingConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询调度配置
func NewGetSchedulingConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSchedulingConfigLogic {
	return &GetSchedulingConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetSchedulingConfigLogic) GetSchedulingConfig(req *types.DefaultIdRequest) (resp *types.SchedulingConfig, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	resourceType := strings.ToUpper(versionDetail.ResourceType)

	var config *types2.SchedulingConfig

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

	resp = convertToSchedulingConfig(config)
	return resp, nil
}

func convertToSchedulingConfig(config *types2.SchedulingConfig) *types.SchedulingConfig {
	result := &types.SchedulingConfig{
		NodeSelector:      config.NodeSelector,
		NodeName:          config.NodeName,
		SchedulerName:     config.SchedulerName,
		PriorityClassName: config.PriorityClassName,
	}

	// 安全处理指针字段
	if config.Priority != nil {
		priority := *config.Priority
		result.Priority = priority
	}

	if config.RuntimeClassName != nil {
		runtimeClassName := *config.RuntimeClassName
		result.RuntimeClassName = runtimeClassName
	}

	if config.Affinity != nil {
		result.Affinity = convertToAffinityConfig(config.Affinity)
	}

	if len(config.Tolerations) > 0 {
		result.Tolerations = convertToTolerationConfigs(config.Tolerations)
	}

	if len(config.TopologySpreadConstraints) > 0 {
		result.TopologySpreadConstraints = convertToTopologySpreadConstraintConfigs(config.TopologySpreadConstraints)
	}

	return result
}

func convertToAffinityConfig(affinity *types2.AffinityConfig) *types.AffinityConfig {
	result := &types.AffinityConfig{}

	if affinity.NodeAffinity != nil {
		result.NodeAffinity = &types.NodeAffinityConfig{}
		if affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
			result.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = convertToNodeSelectorConfig(
				affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution,
			)
		}
		if len(affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution) > 0 {
			result.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = convertToPreferredSchedulingTermConfigs(
				affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution,
			)
		}
	}

	if affinity.PodAffinity != nil {
		result.PodAffinity = convertToPodAffinityConfig(affinity.PodAffinity)
	}

	if affinity.PodAntiAffinity != nil {
		result.PodAntiAffinity = convertToPodAntiAffinityConfig(affinity.PodAntiAffinity)
	}

	return result
}

func convertToNodeSelectorConfig(selector *types2.NodeSelectorConfig) *types.NodeSelectorConfig {
	result := &types.NodeSelectorConfig{
		NodeSelectorTerms: make([]types.NodeSelectorTermConfig, 0, len(selector.NodeSelectorTerms)),
	}

	for _, term := range selector.NodeSelectorTerms {
		result.NodeSelectorTerms = append(result.NodeSelectorTerms, types.NodeSelectorTermConfig{
			MatchExpressions: convertToNodeSelectorRequirements(term.MatchExpressions),
			MatchFields:      convertToNodeSelectorRequirements(term.MatchFields),
		})
	}

	return result
}

func convertToNodeSelectorRequirements(reqs []types2.NodeSelectorRequirementConfig) []types.NodeSelectorRequirementConfig {
	result := make([]types.NodeSelectorRequirementConfig, 0, len(reqs))
	for _, req := range reqs {
		result = append(result, types.NodeSelectorRequirementConfig{
			Key:      req.Key,
			Operator: req.Operator,
			Values:   req.Values,
		})
	}
	return result
}

func convertToPreferredSchedulingTermConfigs(terms []types2.PreferredSchedulingTermConfig) []types.PreferredSchedulingTermConfig {
	result := make([]types.PreferredSchedulingTermConfig, 0, len(terms))
	for _, term := range terms {
		result = append(result, types.PreferredSchedulingTermConfig{
			Weight: term.Weight,
			Preference: types.NodeSelectorTermConfig{
				MatchExpressions: convertToNodeSelectorRequirements(term.Preference.MatchExpressions),
				MatchFields:      convertToNodeSelectorRequirements(term.Preference.MatchFields),
			},
		})
	}
	return result
}

func convertToPodAffinityConfig(affinity *types2.PodAffinityConfig) *types.PodAffinityConfig {
	result := &types.PodAffinityConfig{}

	if len(affinity.RequiredDuringSchedulingIgnoredDuringExecution) > 0 {
		result.RequiredDuringSchedulingIgnoredDuringExecution = convertToPodAffinityTerms(
			affinity.RequiredDuringSchedulingIgnoredDuringExecution,
		)
	}

	if len(affinity.PreferredDuringSchedulingIgnoredDuringExecution) > 0 {
		result.PreferredDuringSchedulingIgnoredDuringExecution = convertToWeightedPodAffinityTerms(
			affinity.PreferredDuringSchedulingIgnoredDuringExecution,
		)
	}

	return result
}

func convertToPodAntiAffinityConfig(affinity *types2.PodAntiAffinityConfig) *types.PodAntiAffinityConfig {
	result := &types.PodAntiAffinityConfig{}

	if len(affinity.RequiredDuringSchedulingIgnoredDuringExecution) > 0 {
		result.RequiredDuringSchedulingIgnoredDuringExecution = convertToPodAffinityTerms(
			affinity.RequiredDuringSchedulingIgnoredDuringExecution,
		)
	}

	if len(affinity.PreferredDuringSchedulingIgnoredDuringExecution) > 0 {
		result.PreferredDuringSchedulingIgnoredDuringExecution = convertToWeightedPodAffinityTerms(
			affinity.PreferredDuringSchedulingIgnoredDuringExecution,
		)
	}

	return result
}

func convertToPodAffinityTerms(terms []types2.PodAffinityTermConfig) []types.PodAffinityTermConfig {
	result := make([]types.PodAffinityTermConfig, 0, len(terms))
	for _, term := range terms {
		converted := types.PodAffinityTermConfig{
			Namespaces:  term.Namespaces,
			TopologyKey: term.TopologyKey,
		}
		if term.LabelSelector != nil {
			converted.LabelSelector = convertToLabelSelectorConfig(term.LabelSelector)
		}
		if term.NamespaceSelector != nil {
			converted.NamespaceSelector = convertToLabelSelectorConfig(term.NamespaceSelector)
		}
		result = append(result, converted)
	}
	return result
}

func convertToWeightedPodAffinityTerms(terms []types2.WeightedPodAffinityTermConfig) []types.WeightedPodAffinityTermConfig {
	result := make([]types.WeightedPodAffinityTermConfig, 0, len(terms))
	for _, term := range terms {
		converted := types.WeightedPodAffinityTermConfig{
			Weight: term.Weight,
			PodAffinityTerm: types.PodAffinityTermConfig{
				Namespaces:  term.PodAffinityTerm.Namespaces,
				TopologyKey: term.PodAffinityTerm.TopologyKey,
			},
		}
		if term.PodAffinityTerm.LabelSelector != nil {
			converted.PodAffinityTerm.LabelSelector = convertToLabelSelectorConfig(term.PodAffinityTerm.LabelSelector)
		}
		if term.PodAffinityTerm.NamespaceSelector != nil {
			converted.PodAffinityTerm.NamespaceSelector = convertToLabelSelectorConfig(term.PodAffinityTerm.NamespaceSelector)
		}
		result = append(result, converted)
	}
	return result
}

func convertToLabelSelectorConfig(selector *types2.LabelSelectorConfig) *types.LabelSelectorConfig {
	result := &types.LabelSelectorConfig{
		MatchLabels: selector.MatchLabels,
	}

	if len(selector.MatchExpressions) > 0 {
		result.MatchExpressions = make([]types.LabelSelectorRequirementConfig, 0, len(selector.MatchExpressions))
		for _, expr := range selector.MatchExpressions {
			result.MatchExpressions = append(result.MatchExpressions, types.LabelSelectorRequirementConfig{
				Key:      expr.Key,
				Operator: expr.Operator,
				Values:   expr.Values,
			})
		}
	}

	return result
}

func convertToTolerationConfigs(tolerations []types2.TolerationConfig) []types.TolerationConfig {
	result := make([]types.TolerationConfig, 0, len(tolerations))
	for _, t := range tolerations {
		toleration := types.TolerationConfig{
			Key:      t.Key,
			Operator: t.Operator,
			Value:    t.Value,
			Effect:   t.Effect,
		}
		// 安全处理指针字段
		if t.TolerationSeconds != nil {
			tolerationSeconds := *t.TolerationSeconds
			toleration.TolerationSeconds = tolerationSeconds
		}
		result = append(result, toleration)
	}
	return result
}

func convertToTopologySpreadConstraintConfigs(constraints []types2.TopologySpreadConstraintConfig) []types.TopologySpreadConstraintConfig {
	result := make([]types.TopologySpreadConstraintConfig, 0, len(constraints))
	for _, c := range constraints {
		converted := types.TopologySpreadConstraintConfig{
			MaxSkew:           c.MaxSkew,
			TopologyKey:       c.TopologyKey,
			WhenUnsatisfiable: c.WhenUnsatisfiable,
		}

		// 安全处理指针字段
		if c.MinDomains != nil {
			minDomains := *c.MinDomains
			converted.MinDomains = minDomains
		}

		if c.NodeAffinityPolicy != nil {
			nodeAffinityPolicy := *c.NodeAffinityPolicy
			converted.NodeAffinityPolicy = nodeAffinityPolicy
		}

		if c.NodeTaintsPolicy != nil {
			nodeTaintsPolicy := *c.NodeTaintsPolicy
			converted.NodeTaintsPolicy = nodeTaintsPolicy
		}

		if c.LabelSelector != nil {
			converted.LabelSelector = convertToLabelSelectorConfig(c.LabelSelector)
		}

		result = append(result, converted)
	}
	return result
}
