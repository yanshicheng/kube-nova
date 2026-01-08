package autoscaling

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/logic"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	k8stypes "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type HPAGetDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 HPA 详情
func NewHPAGetDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HPAGetDetailLogic {
	return &HPAGetDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *HPAGetDetailLogic) HPAGetDetail(req *types.VersionIdRequest) (resp *types.HPADetail, err error) {
	if req == nil {
		l.Error("HPAGetDetail: 请求参数为 nil")
		return nil, fmt.Errorf("请求参数不能为空")
	}

	client, versionDetail, err := logic.GetClusterClientAndVersion(l.ctx, l.svcCtx, req.VersionId, l.Logger)
	if err != nil {
		return nil, err
	}

	if client == nil {
		l.Error("集群客户端为空")
		return nil, fmt.Errorf("集群客户端为空")
	}

	if versionDetail == nil {
		l.Error("版本详情为空")
		return nil, fmt.Errorf("版本详情为空")
	}

	hpaOperator := client.HPA()

	if hpaOperator == nil {
		l.Error("HPA 操作器为空")
		return nil, fmt.Errorf("HPA 操作器未初始化")
	}

	var detail *k8stypes.HPADetail
	switch strings.ToLower(versionDetail.ResourceType) {
	case "deployment":
		deployment, err := client.Deployment().Get(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("获取 Deployment 详情失败: %v", err)
			l.Errorf("Deployment %s/%s 不存在，无法查询 HPA", versionDetail.Namespace, versionDetail.ResourceName)
			return nil, nil
		}

		if deployment == nil {
			l.Errorf("Deployment %s/%s 为空", versionDetail.Namespace, versionDetail.ResourceName)
			return nil, nil
		}

		detail, err = hpaOperator.GetDetailByTargetRef(versionDetail.Namespace, k8stypes.TargetRefInfo{
			APIVersion: deployment.APIVersion,
			Kind:       deployment.Kind,
			Name:       versionDetail.ResourceName,
		})
		if err != nil {

			l.Infof("资源 %s/%s 没有关联的 HPA", versionDetail.Namespace, versionDetail.ResourceName)
			return nil, nil
		}

	case "statefulset":
		statefulSet, err := client.StatefulSet().Get(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("获取 StatefulSet 详情失败: %v", err)
			l.Errorf("StatefulSet %s/%s 不存在，无法查询 HPA", versionDetail.Namespace, versionDetail.ResourceName)
			return nil, nil
		}

		if statefulSet == nil {
			l.Errorf("StatefulSet %s/%s 为空", versionDetail.Namespace, versionDetail.ResourceName)
			return nil, nil
		}

		detail, err = hpaOperator.GetDetailByTargetRef(versionDetail.Namespace, k8stypes.TargetRefInfo{
			APIVersion: statefulSet.APIVersion,
			Kind:       statefulSet.Kind,
			Name:       versionDetail.ResourceName,
		})
		if err != nil {
			l.Infof("资源 %s/%s 没有关联的 HPA", versionDetail.Namespace, versionDetail.ResourceName)
			return nil, nil
		}

	default:
		l.Errorf("不支持的资源类型: %s，跳过 HPA 查询", versionDetail.ResourceType)
		return nil, nil
	}

	if detail == nil {
		l.Infof("资源 %s/%s 没有关联的 HPA", versionDetail.Namespace, versionDetail.ResourceName)
		return nil, nil
	}

	// 转换
	resp = l.convertHPADetail(detail)
	l.Infof("获取 HPA 详情成功: %s/%s", detail.Namespace, detail.Name)
	return resp, nil
}

func (l *HPAGetDetailLogic) convertHPADetail(detail *k8stypes.HPADetail) *types.HPADetail {
	if detail == nil {
		l.Error("convertHPADetail: detail 为 nil")
		return nil
	}

	result := &types.HPADetail{
		Name:              detail.Name,
		Namespace:         detail.Namespace,
		TargetRef:         l.convertTargetRefInfo(detail.TargetRef),
		MaxReplicas:       detail.MaxReplicas,
		Metrics:           l.convertHPAMetrics(detail.Metrics),
		CurrentReplicas:   detail.CurrentReplicas,
		DesiredReplicas:   detail.DesiredReplicas,
		Age:               detail.Age,
		CreationTimestamp: detail.CreationTimestamp,
	}

	// 处理 MinReplicas (指针转非指针)
	if detail.MinReplicas != nil {
		result.MinReplicas = *detail.MinReplicas
	} else {
		result.MinReplicas = 1 // HPA 默认值
	}

	// 处理 Behavior (指针转非指针)
	if detail.Behavior != nil {
		result.Behavior = l.convertHPABehavior(detail.Behavior)
	}

	// 处理可选字段
	if len(detail.CurrentMetrics) > 0 {
		result.CurrentMetrics = l.convertCurrentMetrics(detail.CurrentMetrics)
	}

	if len(detail.Conditions) > 0 {
		result.Conditions = l.convertConditions(detail.Conditions)
	}

	if len(detail.Labels) > 0 {
		result.Labels = detail.Labels
	}

	if len(detail.Annotations) > 0 {
		result.Annotations = detail.Annotations
	}

	return result
}

// 转换 TargetRefInfo
func (l *HPAGetDetailLogic) convertTargetRefInfo(ref k8stypes.TargetRefInfo) types.TargetRefInfo {
	return types.TargetRefInfo{
		Kind:       ref.Kind,
		Name:       ref.Name,
		ApiVersion: ref.APIVersion,
	}
}

// 转换 HPAMetric 列表
func (l *HPAGetDetailLogic) convertHPAMetrics(metrics []k8stypes.HPAMetric) []types.HPAMetric {
	if len(metrics) == 0 {
		return nil
	}

	result := make([]types.HPAMetric, 0, len(metrics))
	for _, m := range metrics {
		metric := types.HPAMetric{
			Type: m.Type,
		}

		if m.Resource != nil {
			metric.Resource = l.convertHPAResourceMetric(m.Resource)
		}
		if m.Pods != nil {
			metric.Pods = l.convertHPAPodsMetric(m.Pods)
		}
		if m.Object != nil {
			metric.Object = l.convertHPAObjectMetric(m.Object)
		}
		if m.External != nil {
			metric.External = l.convertHPAExternalMetric(m.External)
		}
		if m.ContainerResource != nil {
			metric.ContainerResource = l.convertHPAContainerResourceMetric(m.ContainerResource)
		}

		result = append(result, metric)
	}
	return result
}

// 转换 HPAResourceMetric
func (l *HPAGetDetailLogic) convertHPAResourceMetric(metric *k8stypes.HPAResourceMetric) types.HPAResourceMetric {
	return types.HPAResourceMetric{
		Name:   metric.Name,
		Target: l.convertHPAMetricTarget(metric.Target),
	}
}

// 转换 HPAPodsMetric
func (l *HPAGetDetailLogic) convertHPAPodsMetric(metric *k8stypes.HPAPodsMetric) types.HPAPodsMetric {
	return types.HPAPodsMetric{
		Metric: l.convertHPAMetricDef(metric.Metric),
		Target: l.convertHPAMetricTarget(metric.Target),
	}
}

// 转换 HPAObjectMetric
func (l *HPAGetDetailLogic) convertHPAObjectMetric(metric *k8stypes.HPAObjectMetric) types.HPAObjectMetric {
	return types.HPAObjectMetric{
		Metric: l.convertHPAMetricDef(metric.Metric),
		DescribedObject: types.HPADescribedObject{
			ApiVersion: metric.DescribedObject.APIVersion,
			Kind:       metric.DescribedObject.Kind,
			Name:       metric.DescribedObject.Name,
		},
		Target: l.convertHPAMetricTarget(metric.Target),
	}
}

// 转换 HPAExternalMetric
func (l *HPAGetDetailLogic) convertHPAExternalMetric(metric *k8stypes.HPAExternalMetric) types.HPAExternalMetric {
	return types.HPAExternalMetric{
		Metric: l.convertHPAMetricDef(metric.Metric),
		Target: l.convertHPAMetricTarget(metric.Target),
	}
}

// 转换 HPAContainerResourceMetric
func (l *HPAGetDetailLogic) convertHPAContainerResourceMetric(metric *k8stypes.HPAContainerResourceMetric) types.HPAContainerResourceMetric {
	return types.HPAContainerResourceMetric{
		Name:      metric.Name,
		Container: metric.Container,
		Target:    l.convertHPAMetricTarget(metric.Target),
	}
}

// 转换 HPAMetricDef
func (l *HPAGetDetailLogic) convertHPAMetricDef(def k8stypes.HPAMetricDef) types.HPAMetricDef {
	return types.HPAMetricDef{
		Name:     def.Name,
		Selector: def.Selector,
	}
}

// 转换 HPAMetricTarget
func (l *HPAGetDetailLogic) convertHPAMetricTarget(target k8stypes.HPAMetricTarget) types.HPAMetricTarget {
	result := types.HPAMetricTarget{
		Type: target.Type,
	}

	if target.AverageUtilization != nil {
		result.AverageUtilization = *target.AverageUtilization
	}

	result.AverageValue = target.AverageValue
	result.Value = target.Value

	return result
}

// 转换 CurrentMetrics 列表
func (l *HPAGetDetailLogic) convertCurrentMetrics(metrics []k8stypes.HPACurrentMetric) []types.HPACurrentMetric {
	if len(metrics) == 0 {
		return nil
	}

	result := make([]types.HPACurrentMetric, 0, len(metrics))
	for _, m := range metrics {
		current := types.HPACurrentMetricValue{}

		if m.Current.AverageUtilization != nil {
			current.AverageUtilization = *m.Current.AverageUtilization
		}
		current.AverageValue = m.Current.AverageValue
		current.Value = m.Current.Value

		result = append(result, types.HPACurrentMetric{
			Type:    m.Type,
			Current: current,
		})
	}
	return result
}

// 转换 Conditions 列表
func (l *HPAGetDetailLogic) convertConditions(conditions []k8stypes.HPACondition) []types.HPACondition {
	if len(conditions) == 0 {
		return nil
	}

	result := make([]types.HPACondition, 0, len(conditions))
	for _, c := range conditions {
		result = append(result, types.HPACondition{
			Type:               c.Type,
			Status:             c.Status,
			Reason:             c.Reason,
			Message:            c.Message,
			LastTransitionTime: c.LastTransitionTime,
		})
	}
	return result
}

func (l *HPAGetDetailLogic) convertHPABehavior(behavior *k8stypes.HPABehavior) types.HPABehavior {
	result := types.HPABehavior{}

	// 注意：types.HPABehavior 的字段是非指针类型
	if behavior.ScaleUp != nil {
		result.ScaleUp = l.convertHPAScalingRules(behavior.ScaleUp)
	}

	if behavior.ScaleDown != nil {
		result.ScaleDown = l.convertHPAScalingRules(behavior.ScaleDown)
	}

	return result
}

// 转换 HPAScalingRules（返回非指针类型）
func (l *HPAGetDetailLogic) convertHPAScalingRules(rules *k8stypes.HPAScalingRules) types.HPAScalingRules {
	result := types.HPAScalingRules{
		SelectPolicy: rules.SelectPolicy,
	}

	if rules.StabilizationWindowSeconds != nil {
		result.StabilizationWindowSeconds = *rules.StabilizationWindowSeconds
	}

	if len(rules.Policies) > 0 {
		result.Policies = l.convertScalingPolicies(rules.Policies)
	}

	return result
}

// 转换 ScalingPolicies
func (l *HPAGetDetailLogic) convertScalingPolicies(policies []k8stypes.HPAScalingPolicy) []types.HPAScalingPolicy {
	if len(policies) == 0 {
		return nil
	}

	result := make([]types.HPAScalingPolicy, 0, len(policies))
	for _, p := range policies {
		result = append(result, types.HPAScalingPolicy{
			Type:          p.Type,
			Value:         p.Value,
			PeriodSeconds: p.PeriodSeconds,
		})
	}
	return result
}
