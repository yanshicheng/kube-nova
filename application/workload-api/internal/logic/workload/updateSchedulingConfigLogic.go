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

type UpdateSchedulingConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 修改调度配置
func NewUpdateSchedulingConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateSchedulingConfigLogic {
	return &UpdateSchedulingConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateSchedulingConfigLogic) UpdateSchedulingConfig(req *types.UpdateSchedulingConfigRequest) (resp string, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	updateReq := convertToK8sUpdateSchedulingConfigRequest(req)
	resourceType := strings.ToUpper(versionDetail.ResourceType)

	switch resourceType {
	case "DEPLOYMENT":
		err = client.Deployment().UpdateSchedulingConfig(versionDetail.Namespace, versionDetail.ResourceName, updateReq)
	case "STATEFULSET":
		err = client.StatefulSet().UpdateSchedulingConfig(versionDetail.Namespace, versionDetail.ResourceName, updateReq)
	case "DAEMONSET":
		err = client.DaemonSet().UpdateSchedulingConfig(versionDetail.Namespace, versionDetail.ResourceName, updateReq)
	case "JOB":
		err = client.Job().UpdateSchedulingConfig(versionDetail.Namespace, versionDetail.ResourceName, updateReq)
	case "CRONJOB":
		err = client.CronJob().UpdateSchedulingConfig(versionDetail.Namespace, versionDetail.ResourceName, updateReq)
	default:
		return "", fmt.Errorf("资源类型 %s 不支持修改调度配置", resourceType)
	}

	// 构建调度配置详情
	nodeSelectorCount := len(req.NodeSelector)
	tolerationCount := len(req.Tolerations)
	hasAffinity := req.Affinity != nil

	if err != nil {
		l.Errorf("修改调度配置失败: %v", err)
		recordAuditLog(l.ctx, l.svcCtx, versionDetail, "修改调度配置",
			fmt.Sprintf("%s %s/%s 修改调度配置失败, NodeSelector数量: %d, Tolerations数量: %d, 是否配置亲和性: %v, 错误: %v", resourceType, versionDetail.Namespace, versionDetail.ResourceName, nodeSelectorCount, tolerationCount, hasAffinity, err), 2)
		return "", fmt.Errorf("修改调度配置失败")
	}

	recordAuditLog(l.ctx, l.svcCtx, versionDetail, "修改调度配置",
		fmt.Sprintf("%s %s/%s 修改调度配置成功, NodeSelector数量: %d, Tolerations数量: %d, 是否配置亲和性: %v", resourceType, versionDetail.Namespace, versionDetail.ResourceName, nodeSelectorCount, tolerationCount, hasAffinity), 1)
	return "修改调度配置成功", nil
}

func convertToK8sUpdateSchedulingConfigRequest(req *types.UpdateSchedulingConfigRequest) *k8sTypes.UpdateSchedulingConfigRequest {
	result := &k8sTypes.UpdateSchedulingConfigRequest{
		NodeSelector:      req.NodeSelector,
		SchedulerName:     req.SchedulerName,
		PriorityClassName: req.PriorityClassName,
		Priority:          &req.Priority,
		RuntimeClassName:  &req.RuntimeClassName,
	}

	if req.Affinity != nil {
		result.Affinity = convertToK8sAffinityConfig(req.Affinity)
	}

	if len(req.Tolerations) > 0 {
		result.Tolerations = convertToK8sTolerationConfigs(req.Tolerations)
	}

	if len(req.TopologySpreadConstraints) > 0 {
		result.TopologySpreadConstraints = convertToK8sTopologySpreadConstraintConfigs(req.TopologySpreadConstraints)
	}

	return result
}

func convertToK8sAffinityConfig(affinity *types.AffinityConfig) *k8sTypes.AffinityConfig {
	result := &k8sTypes.AffinityConfig{}

	if affinity.NodeAffinity != nil {
		result.NodeAffinity = &k8sTypes.NodeAffinityConfig{}
		if affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
			result.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = convertToK8sNodeSelectorConfig(
				affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution,
			)
		}
		if len(affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution) > 0 {
			result.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = convertToK8sPreferredSchedulingTermConfigs(
				affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution,
			)
		}
	}

	if affinity.PodAffinity != nil {
		result.PodAffinity = convertToK8sPodAffinityConfig(affinity.PodAffinity)
	}

	if affinity.PodAntiAffinity != nil {
		result.PodAntiAffinity = convertToK8sPodAntiAffinityConfig(affinity.PodAntiAffinity)
	}

	return result
}

func convertToK8sNodeSelectorConfig(selector *types.NodeSelectorConfig) *k8sTypes.NodeSelectorConfig {
	result := &k8sTypes.NodeSelectorConfig{
		NodeSelectorTerms: make([]k8sTypes.NodeSelectorTermConfig, 0, len(selector.NodeSelectorTerms)),
	}

	for _, term := range selector.NodeSelectorTerms {
		result.NodeSelectorTerms = append(result.NodeSelectorTerms, k8sTypes.NodeSelectorTermConfig{
			MatchExpressions: convertToK8sNodeSelectorRequirements(term.MatchExpressions),
			MatchFields:      convertToK8sNodeSelectorRequirements(term.MatchFields),
		})
	}

	return result
}

func convertToK8sNodeSelectorRequirements(reqs []types.NodeSelectorRequirementConfig) []k8sTypes.NodeSelectorRequirementConfig {
	result := make([]k8sTypes.NodeSelectorRequirementConfig, 0, len(reqs))
	for _, req := range reqs {
		result = append(result, k8sTypes.NodeSelectorRequirementConfig{
			Key:      req.Key,
			Operator: req.Operator,
			Values:   req.Values,
		})
	}
	return result
}

func convertToK8sPreferredSchedulingTermConfigs(terms []types.PreferredSchedulingTermConfig) []k8sTypes.PreferredSchedulingTermConfig {
	result := make([]k8sTypes.PreferredSchedulingTermConfig, 0, len(terms))
	for _, term := range terms {
		result = append(result, k8sTypes.PreferredSchedulingTermConfig{
			Weight: term.Weight,
			Preference: k8sTypes.NodeSelectorTermConfig{
				MatchExpressions: convertToK8sNodeSelectorRequirements(term.Preference.MatchExpressions),
				MatchFields:      convertToK8sNodeSelectorRequirements(term.Preference.MatchFields),
			},
		})
	}
	return result
}

func convertToK8sPodAffinityConfig(affinity *types.PodAffinityConfig) *k8sTypes.PodAffinityConfig {
	result := &k8sTypes.PodAffinityConfig{}

	if len(affinity.RequiredDuringSchedulingIgnoredDuringExecution) > 0 {
		result.RequiredDuringSchedulingIgnoredDuringExecution = convertToK8sPodAffinityTerms(
			affinity.RequiredDuringSchedulingIgnoredDuringExecution,
		)
	}

	if len(affinity.PreferredDuringSchedulingIgnoredDuringExecution) > 0 {
		result.PreferredDuringSchedulingIgnoredDuringExecution = convertToK8sWeightedPodAffinityTerms(
			affinity.PreferredDuringSchedulingIgnoredDuringExecution,
		)
	}

	return result
}

func convertToK8sPodAntiAffinityConfig(affinity *types.PodAntiAffinityConfig) *k8sTypes.PodAntiAffinityConfig {
	result := &k8sTypes.PodAntiAffinityConfig{}

	if len(affinity.RequiredDuringSchedulingIgnoredDuringExecution) > 0 {
		result.RequiredDuringSchedulingIgnoredDuringExecution = convertToK8sPodAffinityTerms(
			affinity.RequiredDuringSchedulingIgnoredDuringExecution,
		)
	}

	if len(affinity.PreferredDuringSchedulingIgnoredDuringExecution) > 0 {
		result.PreferredDuringSchedulingIgnoredDuringExecution = convertToK8sWeightedPodAffinityTerms(
			affinity.PreferredDuringSchedulingIgnoredDuringExecution,
		)
	}

	return result
}

func convertToK8sPodAffinityTerms(terms []types.PodAffinityTermConfig) []k8sTypes.PodAffinityTermConfig {
	result := make([]k8sTypes.PodAffinityTermConfig, 0, len(terms))
	for _, term := range terms {
		converted := k8sTypes.PodAffinityTermConfig{
			Namespaces:  term.Namespaces,
			TopologyKey: term.TopologyKey,
		}
		if term.LabelSelector != nil {
			converted.LabelSelector = convertToK8sLabelSelectorConfig(term.LabelSelector)
		}
		if term.NamespaceSelector != nil {
			converted.NamespaceSelector = convertToK8sLabelSelectorConfig(term.NamespaceSelector)
		}
		result = append(result, converted)
	}
	return result
}

func convertToK8sWeightedPodAffinityTerms(terms []types.WeightedPodAffinityTermConfig) []k8sTypes.WeightedPodAffinityTermConfig {
	result := make([]k8sTypes.WeightedPodAffinityTermConfig, 0, len(terms))
	for _, term := range terms {
		converted := k8sTypes.WeightedPodAffinityTermConfig{
			Weight: term.Weight,
			PodAffinityTerm: k8sTypes.PodAffinityTermConfig{
				Namespaces:  term.PodAffinityTerm.Namespaces,
				TopologyKey: term.PodAffinityTerm.TopologyKey,
			},
		}
		if term.PodAffinityTerm.LabelSelector != nil {
			converted.PodAffinityTerm.LabelSelector = convertToK8sLabelSelectorConfig(term.PodAffinityTerm.LabelSelector)
		}
		if term.PodAffinityTerm.NamespaceSelector != nil {
			converted.PodAffinityTerm.NamespaceSelector = convertToK8sLabelSelectorConfig(term.PodAffinityTerm.NamespaceSelector)
		}
		result = append(result, converted)
	}
	return result
}

func convertToK8sLabelSelectorConfig(selector *types.LabelSelectorConfig) *k8sTypes.LabelSelectorConfig {
	result := &k8sTypes.LabelSelectorConfig{
		MatchLabels: selector.MatchLabels,
	}

	if len(selector.MatchExpressions) > 0 {
		result.MatchExpressions = make([]k8sTypes.LabelSelectorRequirementConfig, 0, len(selector.MatchExpressions))
		for _, expr := range selector.MatchExpressions {
			result.MatchExpressions = append(result.MatchExpressions, k8sTypes.LabelSelectorRequirementConfig{
				Key:      expr.Key,
				Operator: expr.Operator,
				Values:   expr.Values,
			})
		}
	}

	return result
}

func convertToK8sTolerationConfigs(tolerations []types.TolerationConfig) []k8sTypes.TolerationConfig {
	result := make([]k8sTypes.TolerationConfig, 0, len(tolerations))
	for _, t := range tolerations {
		result = append(result, k8sTypes.TolerationConfig{
			Key:               t.Key,
			Operator:          t.Operator,
			Value:             t.Value,
			Effect:            t.Effect,
			TolerationSeconds: &t.TolerationSeconds,
		})
	}
	return result
}

func convertToK8sTopologySpreadConstraintConfigs(constraints []types.TopologySpreadConstraintConfig) []k8sTypes.TopologySpreadConstraintConfig {
	result := make([]k8sTypes.TopologySpreadConstraintConfig, 0, len(constraints))
	for _, c := range constraints {
		converted := k8sTypes.TopologySpreadConstraintConfig{
			MaxSkew:            c.MaxSkew,
			TopologyKey:        c.TopologyKey,
			WhenUnsatisfiable:  c.WhenUnsatisfiable,
			MinDomains:         &c.MinDomains,
			NodeAffinityPolicy: &c.NodeAffinityPolicy,
			NodeTaintsPolicy:   &c.NodeTaintsPolicy,
		}
		if c.LabelSelector != nil {
			converted.LabelSelector = convertToK8sLabelSelectorConfig(c.LabelSelector)
		}
		result = append(result, converted)
	}
	return result
}
