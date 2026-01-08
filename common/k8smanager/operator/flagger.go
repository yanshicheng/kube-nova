package operator

import (
	"context"
	"fmt"
	"strings"
	"time"

	flaggerv1 "github.com/fluxcd/flagger/pkg/apis/flagger/v1beta1"
	flaggerclientset "github.com/fluxcd/flagger/pkg/client/clientset/versioned"
	flaggerinformers "github.com/fluxcd/flagger/pkg/client/informers/externalversions"
	flaggerlisters "github.com/fluxcd/flagger/pkg/client/listers/flagger/v1beta1"
	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/yaml"
)

type flaggerOperator struct {
	BaseOperator
	client          flaggerclientset.Interface
	informerFactory flaggerinformers.SharedInformerFactory
	canaryLister    flaggerlisters.CanaryLister
}

// NewFlaggerOperator 创建 Flagger 操作器（不使用 informer）
func NewFlaggerOperator(ctx context.Context, client flaggerclientset.Interface) types.FlaggerOperator {
	return &flaggerOperator{
		BaseOperator: NewBaseOperator(ctx, false),
		client:       client,
	}
}

// NewFlaggerOperatorWithInformer 创建 Flagger 操作器（使用 informer）
func NewFlaggerOperatorWithInformer(
	ctx context.Context,
	client flaggerclientset.Interface,
	informerFactory flaggerinformers.SharedInformerFactory,
) types.FlaggerOperator {
	var canaryLister flaggerlisters.CanaryLister

	if informerFactory != nil {
		canaryLister = informerFactory.Flagger().V1beta1().Canaries().Lister()
	}

	return &flaggerOperator{
		BaseOperator:    NewBaseOperator(ctx, informerFactory != nil),
		client:          client,
		informerFactory: informerFactory,
		canaryLister:    canaryLister,
	}
}

// Create 创建 Canary
func (f *flaggerOperator) Create(canary *flaggerv1.Canary) (*flaggerv1.Canary, error) {
	if canary == nil || canary.Name == "" || canary.Namespace == "" {
		return nil, fmt.Errorf("Canary对象、名称和命名空间不能为空")
	}

	created, err := f.client.FlaggerV1beta1().Canaries(canary.Namespace).Create(f.ctx, canary, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("创建Canary失败: %v", err)
	}

	return created, nil
}

// Get 获取 Canary
func (f *flaggerOperator) Get(namespace, name string) (*flaggerv1.Canary, error) {
	if namespace == "" || name == "" {
		return nil, fmt.Errorf("命名空间和名称不能为空")
	}

	// 优先使用 informer
	if f.useInformer && f.canaryLister != nil {
		canary, err := f.canaryLister.Canaries(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, nil
			}
			// informer 出错时回退到 API 调用
			canary, apiErr := f.client.FlaggerV1beta1().Canaries(namespace).Get(f.ctx, name, metav1.GetOptions{})
			if apiErr != nil {
				if errors.IsNotFound(apiErr) {
					return nil, nil
				}
				return nil, fmt.Errorf("获取Canary失败: %v", apiErr)
			}
			canary.TypeMeta = metav1.TypeMeta{
				Kind:       "Canary",
				APIVersion: "flagger.app/v1beta1",
			}
			return canary, nil
		}
		canary.TypeMeta = metav1.TypeMeta{
			Kind:       "Canary",
			APIVersion: "flagger.app/v1beta1",
		}
		return canary, nil
	}

	// 使用 API 调用
	canary, err := f.client.FlaggerV1beta1().Canaries(namespace).Get(f.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("获取Canary失败: %v", err)
	}

	canary.TypeMeta = metav1.TypeMeta{
		Kind:       "Canary",
		APIVersion: "flagger.app/v1beta1",
	}

	return canary, nil
}

// Update 更新 Canary
func (f *flaggerOperator) Update(canary *flaggerv1.Canary) (*flaggerv1.Canary, error) {
	if canary == nil || canary.Name == "" || canary.Namespace == "" {
		return nil, fmt.Errorf("Canary对象、名称和命名空间不能为空")
	}

	updated, err := f.client.FlaggerV1beta1().Canaries(canary.Namespace).Update(f.ctx, canary, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("更新Canary失败: %v", err)
	}

	return updated, nil
}

// Delete 删除 Canary
func (f *flaggerOperator) Delete(namespace, name string) error {
	if namespace == "" || name == "" {
		return fmt.Errorf("命名空间和名称不能为空")
	}

	err := f.client.FlaggerV1beta1().Canaries(namespace).Delete(f.ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("删除Canary失败: %v", err)
	}

	return nil
}

// List 获取 Canary 列表
func (f *flaggerOperator) List(namespace string, search string, labelSelector string) (*types.ListCanaryResponse, error) {
	var selector labels.Selector = labels.Everything()
	if labelSelector != "" {
		parsedSelector, err := labels.Parse(labelSelector)
		if err != nil {
			return nil, fmt.Errorf("解析标签选择器失败: %v", err)
		}
		selector = parsedSelector
	}

	var canaries []*flaggerv1.Canary
	var err error

	// 优先使用 informer
	if f.useInformer && f.canaryLister != nil {
		canaries, err = f.canaryLister.Canaries(namespace).List(selector)
		if err != nil {
			return nil, fmt.Errorf("获取Canary列表失败: %v", err)
		}
	} else {
		// 使用 API 调用
		listOpts := metav1.ListOptions{LabelSelector: selector.String()}
		canaryList, err := f.client.FlaggerV1beta1().Canaries(namespace).List(f.ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("获取Canary列表失败: %v", err)
		}
		canaries = make([]*flaggerv1.Canary, len(canaryList.Items))
		for i := range canaryList.Items {
			canaries[i] = &canaryList.Items[i]
		}
	}

	// 搜索过滤
	if search != "" {
		filtered := make([]*flaggerv1.Canary, 0)
		searchLower := strings.ToLower(search)
		for _, canary := range canaries {
			if strings.Contains(strings.ToLower(canary.Name), searchLower) {
				filtered = append(filtered, canary)
			}
		}
		canaries = filtered
	}

	// 转换为响应格式
	items := make([]types.CanaryInfo, len(canaries))
	for i, canary := range canaries {
		items[i] = f.convertToCanaryInfo(canary)
	}

	return &types.ListCanaryResponse{
		Total: len(items),
		Items: items,
	}, nil
}

// GetYaml 获取 Canary 的 YAML 格式
func (f *flaggerOperator) GetYaml(namespace, name string) (string, error) {
	canary, err := f.Get(namespace, name)
	if err != nil {
		return "", err
	}

	if canary == nil {
		return "", fmt.Errorf("Canary %s/%s 不存在", namespace, name)
	}

	//  确保有 TypeMeta
	canary.TypeMeta = metav1.TypeMeta{
		Kind:       "Canary",
		APIVersion: "flagger.app/v1beta1",
	}

	// 清除管理字段
	canary.ManagedFields = nil

	yamlBytes, err := yaml.Marshal(canary)
	if err != nil {
		return "", fmt.Errorf("转换为YAML失败: %v", err)
	}

	return string(yamlBytes), nil
}

// GetDetail 获取 Canary 详细信息
func (f *flaggerOperator) GetDetail(namespace, name string) (*types.CanaryDetail, error) {
	canary, err := f.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	detail := &types.CanaryDetail{
		CanaryInfo: f.convertToCanaryInfo(canary),
		Analysis:   f.convertAnalysis(canary.Spec.Analysis),
		Service:    f.convertService(&canary.Spec.Service),
	}

	return detail, nil
}

// GetStatus 获取 Canary 状态信息
func (f *flaggerOperator) GetStatus(namespace, name string) (*types.CanaryStatusResponse, error) {
	canary, err := f.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	status := &types.CanaryStatusResponse{
		Name:         canary.Name,
		Namespace:    canary.Namespace,
		Phase:        string(canary.Status.Phase),
		CanaryWeight: canary.Status.CanaryWeight,
		FailedChecks: canary.Status.FailedChecks,
		Iterations:   canary.Status.Iterations,
		Conditions:   make([]types.CanaryStatusCondition, 0),
	}

	// 处理 TrackedConfigs（指针类型）
	if canary.Status.TrackedConfigs != nil {
		status.TrackedConfigs = *canary.Status.TrackedConfigs
	} else {
		status.TrackedConfigs = make(map[string]string)
	}

	// 转换状态条件
	for _, cond := range canary.Status.Conditions {
		status.Conditions = append(status.Conditions, types.CanaryStatusCondition{
			Type:               string(cond.Type),
			Status:             string(cond.Status),
			LastUpdateTime:     cond.LastUpdateTime.Format(time.RFC3339),
			LastTransitionTime: cond.LastTransitionTime.Format(time.RFC3339),
			Reason:             cond.Reason,
			Message:            cond.Message,
		})
	}

	// 最后应用的规格
	if canary.Status.LastAppliedSpec != "" {
		status.LastAppliedSpec = canary.Status.LastAppliedSpec
	}

	return status, nil
}

// Pause 暂停金丝雀发布
func (f *flaggerOperator) Pause(namespace, name string) error {
	canary, err := f.Get(namespace, name)
	if err != nil {
		return err
	}

	if canary.Annotations == nil {
		canary.Annotations = make(map[string]string)
	}

	// 设置暂停注解
	canary.Annotations["flagger.app/pause"] = "true"

	_, err = f.Update(canary)
	if err != nil {
		return fmt.Errorf("暂停Canary失败: %v", err)
	}

	return nil
}

// Resume 恢复金丝雀发布
func (f *flaggerOperator) Resume(namespace, name string) error {
	canary, err := f.Get(namespace, name)
	if err != nil {
		return err
	}

	if canary.Annotations == nil {
		return nil
	}

	// 删除暂停注解
	delete(canary.Annotations, "flagger.app/pause")

	_, err = f.Update(canary)
	if err != nil {
		return fmt.Errorf("恢复Canary失败: %v", err)
	}

	return nil
}

// Reset 重置金丝雀状态
func (f *flaggerOperator) Reset(namespace, name string) error {
	canary, err := f.Get(namespace, name)
	if err != nil {
		return err
	}

	if canary.Annotations == nil {
		canary.Annotations = make(map[string]string)
	}

	// 设置重置注解，Flagger 会重置金丝雀状态
	canary.Annotations["flagger.app/reset"] = "true"

	_, err = f.Update(canary)
	if err != nil {
		return fmt.Errorf("重置Canary失败: %v", err)
	}

	return nil
}

// ========== 辅助方法 ==========

// convertToCanaryInfo 转换为 CanaryInfo
func (f *flaggerOperator) convertToCanaryInfo(canary *flaggerv1.Canary) types.CanaryInfo {
	info := types.CanaryInfo{
		Name:      canary.Name,
		Namespace: canary.Namespace,
		TargetRef: types.TargetRefInfo{
			APIVersion: canary.Spec.TargetRef.APIVersion,
			Kind:       canary.Spec.TargetRef.Kind,
			Name:       canary.Spec.TargetRef.Name,
		},
		Status:            string(canary.Status.Phase),
		CanaryWeight:      canary.Status.CanaryWeight,
		FailedChecks:      canary.Status.FailedChecks,
		Phase:             string(canary.Status.Phase),
		Labels:            canary.Labels,
		Annotations:       canary.Annotations,
		Age:               time.Since(canary.CreationTimestamp.Time).String(),
		CreationTimestamp: canary.CreationTimestamp.UnixMilli(),
	}

	// 设置 ProgressDeadline
	if canary.Spec.ProgressDeadlineSeconds != nil {
		info.ProgressDeadline = *canary.Spec.ProgressDeadlineSeconds
	}

	// 获取最后状态变更时间
	if len(canary.Status.Conditions) > 0 {
		lastCondition := canary.Status.Conditions[len(canary.Status.Conditions)-1]
		info.LastTransition = lastCondition.LastTransitionTime.Format(time.RFC3339)
	}

	return info
}

// convertAnalysis 转换分析配置
func (f *flaggerOperator) convertAnalysis(analysis *flaggerv1.CanaryAnalysis) types.CanaryAnalysis {
	if analysis == nil {
		return types.CanaryAnalysis{
			Metrics:  make([]types.MetricInfo, 0),
			Webhooks: make([]types.WebhookInfo, 0),
			Match:    make([]types.MatchInfo, 0),
		}
	}

	result := types.CanaryAnalysis{
		Interval:   analysis.Interval,
		Threshold:  int32(analysis.Threshold),
		MaxWeight:  int32(analysis.MaxWeight),
		StepWeight: int32(analysis.StepWeight),
		Iterations: int32(analysis.Iterations),
		Metrics:    make([]types.MetricInfo, 0),
		Webhooks:   make([]types.WebhookInfo, 0),
		Match:      make([]types.MatchInfo, 0),
	}

	// 转换指标
	for _, metric := range analysis.Metrics {
		metricInfo := types.MetricInfo{
			Name:     metric.Name,
			Interval: metric.Interval,
			Query:    metric.Query,
		}

		if metric.ThresholdRange != nil {
			metricInfo.ThresholdRange = &types.ThresholdRange{
				Min: metric.ThresholdRange.Min,
				Max: metric.ThresholdRange.Max,
			}
		}

		if metric.TemplateRef != nil {
			metricInfo.TemplateRef = &types.TemplateRef{
				Name:      metric.TemplateRef.Name,
				Namespace: metric.TemplateRef.Namespace,
			}
		}

		result.Metrics = append(result.Metrics, metricInfo)
	}

	// 转换 Webhooks
	for _, webhook := range analysis.Webhooks {
		webhookInfo := types.WebhookInfo{
			Name:    webhook.Name,
			Type:    string(webhook.Type), // HookType 转换为 string
			URL:     webhook.URL,
			Timeout: webhook.Timeout,
		}

		// 处理 Metadata（指针类型）
		if webhook.Metadata != nil {
			webhookInfo.Metadata = *webhook.Metadata
		} else {
			webhookInfo.Metadata = make(map[string]string)
		}

		result.Webhooks = append(result.Webhooks, webhookInfo)
	}

	// 转换匹配规则
	for _, match := range analysis.Match {
		matchInfo := types.MatchInfo{
			Headers: make(map[string]types.StringMatch),
		}

		for key, strMatch := range match.Headers {
			matchInfo.Headers[key] = types.StringMatch{
				Exact:  strMatch.Exact,
				Prefix: strMatch.Prefix,
				Suffix: strMatch.Suffix,
				Regex:  strMatch.Regex,
			}
		}

		result.Match = append(result.Match, matchInfo)
	}

	return result
}

// convertService 转换服务配置
func (f *flaggerOperator) convertService(service *flaggerv1.CanaryService) types.CanaryService {
	if service == nil {
		return types.CanaryService{}
	}

	result := types.CanaryService{
		Port:     int32(service.Port),
		Name:     service.Name,
		PortName: service.PortName,
		Gateways: service.Gateways,
		Hosts:    service.Hosts,
	}

	// 转换 TargetPort
	if service.TargetPort.IntVal != 0 {
		result.TargetPort = service.TargetPort.IntVal
	}

	if service.TrafficPolicy != nil && service.TrafficPolicy.TLS != nil {
		result.TrafficPolicy = &types.TrafficPolicy{
			TLS: &types.TLSPolicy{
				Mode: string(service.TrafficPolicy.TLS.Mode),
			},
		}
	}

	return result
}

// ========== operator.go 中实现方法 ==========

// GetByTargetRef 通过 targetRef 查询 Canary（这个才是你需要的！）
func (f *flaggerOperator) GetByTargetRef(namespace string, apiVersion string, kind string, name string) (*flaggerv1.Canary, error) {
	if namespace == "" || apiVersion == "" || kind == "" || name == "" {
		return nil, fmt.Errorf("命名空间、apiVersion、kind 和名称不能为空")
	}

	var canaries []*flaggerv1.Canary
	var err error

	// 优先使用 informer
	if f.useInformer && f.canaryLister != nil {
		canaries, err = f.canaryLister.Canaries(namespace).List(labels.Everything())
		if err != nil {
			canaryList, apiErr := f.client.FlaggerV1beta1().Canaries(namespace).List(f.ctx, metav1.ListOptions{})
			if apiErr != nil {
				return nil, fmt.Errorf("获取Canary列表失败: %v", apiErr)
			}
			canaries = make([]*flaggerv1.Canary, len(canaryList.Items))
			for i := range canaryList.Items {
				canaries[i] = &canaryList.Items[i]
			}
		}
	} else {
		canaryList, err := f.client.FlaggerV1beta1().Canaries(namespace).List(f.ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("获取Canary列表失败: %v", err)
		}
		canaries = make([]*flaggerv1.Canary, len(canaryList.Items))
		for i := range canaryList.Items {
			canaries[i] = &canaryList.Items[i]
		}
	}

	for _, canary := range canaries {
		ref := canary.Spec.TargetRef
		if ref.APIVersion == apiVersion &&
			ref.Kind == kind &&
			ref.Name == name {
			canary.TypeMeta = metav1.TypeMeta{
				Kind:       "Canary",
				APIVersion: "flagger.app/v1beta1",
			}
			return canary, nil
		}
	}

	return nil, nil
}

// GetByAutoscalerRef 通过 autoscalerRef 查询 Canary（保留原方法，增加调试）
func (f *flaggerOperator) GetByAutoscalerRef(namespace string, apiVersion string, kind string, name string) (*flaggerv1.Canary, error) {
	if namespace == "" || apiVersion == "" || kind == "" || name == "" {
		return nil, fmt.Errorf("命名空间、apiVersion、kind 和名称不能为空")
	}

	var canaries []*flaggerv1.Canary
	var err error

	// 优先使用 informer
	if f.useInformer && f.canaryLister != nil {
		canaries, err = f.canaryLister.Canaries(namespace).List(labels.Everything())
		if err != nil {
			canaryList, apiErr := f.client.FlaggerV1beta1().Canaries(namespace).List(f.ctx, metav1.ListOptions{})
			if apiErr != nil {
				return nil, fmt.Errorf("获取Canary列表失败: %v", apiErr)
			}
			canaries = make([]*flaggerv1.Canary, len(canaryList.Items))
			for i := range canaryList.Items {
				canaries[i] = &canaryList.Items[i]
			}
		}
	} else {
		canaryList, err := f.client.FlaggerV1beta1().Canaries(namespace).List(f.ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("获取Canary列表失败: %v", err)
		}
		canaries = make([]*flaggerv1.Canary, len(canaryList.Items))
		for i := range canaryList.Items {
			canaries[i] = &canaryList.Items[i]
		}
	}

	for _, canary := range canaries {
		if canary.Spec.AutoscalerRef != nil {
			ref := canary.Spec.AutoscalerRef
			if ref.APIVersion == apiVersion &&
				ref.Kind == kind &&
				ref.Name == name {
				//  手动注入 TypeMeta
				canary.TypeMeta = metav1.TypeMeta{
					Kind:       "Canary",
					APIVersion: "flagger.app/v1beta1",
				}
				return canary, nil
			}
		}
	}

	return nil, nil
}
