package operator

import (
	"context"
	"fmt"
	"strings"
	"time"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringinformers "github.com/prometheus-operator/prometheus-operator/pkg/client/informers/externalversions"
	monitoringlister "github.com/prometheus-operator/prometheus-operator/pkg/client/listers/monitoring/v1"
	monitoringclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/yaml"
)

type prometheusRuleOperator struct {
	BaseOperator
	client               monitoringclient.Interface
	informerFactory      monitoringinformers.SharedInformerFactory
	prometheusRuleLister monitoringlister.PrometheusRuleLister
}

// NewPrometheusRuleOperator 创建 PrometheusRule 操作器（不使用 informer）
func NewPrometheusRuleOperator(ctx context.Context, client monitoringclient.Interface) types.PrometheusRuleOperator {
	return &prometheusRuleOperator{
		BaseOperator: NewBaseOperator(ctx, false),
		client:       client,
	}
}

// NewPrometheusRuleOperatorWithInformer 创建 PrometheusRule 操作器（使用 informer）
func NewPrometheusRuleOperatorWithInformer(
	ctx context.Context,
	client monitoringclient.Interface,
	informerFactory monitoringinformers.SharedInformerFactory,
) types.PrometheusRuleOperator {
	var prometheusRuleLister monitoringlister.PrometheusRuleLister

	if informerFactory != nil {
		prometheusRuleLister = informerFactory.Monitoring().V1().PrometheusRules().Lister()
	}

	return &prometheusRuleOperator{
		BaseOperator:         NewBaseOperator(ctx, informerFactory != nil),
		client:               client,
		informerFactory:      informerFactory,
		prometheusRuleLister: prometheusRuleLister,
	}
}

// Create 创建 PrometheusRule
func (p *prometheusRuleOperator) Create(rule *monitoringv1.PrometheusRule) error {
	if rule == nil || rule.Name == "" {
		return fmt.Errorf("PrometheusRule 不能为空")
	}

	// 判断是否已经存在
	_, err := p.client.MonitoringV1().PrometheusRules(rule.Namespace).Get(p.ctx, rule.Name, metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("PrometheusRule %s/%s 已经存在", rule.Namespace, rule.Name)
	}
	if !errors.IsNotFound(err) {
		return fmt.Errorf("检查 PrometheusRule 是否存在失败: %v", err)
	}

	_, err = p.client.MonitoringV1().PrometheusRules(rule.Namespace).Create(p.ctx, rule, metav1.CreateOptions{})
	return err
}

// Get 获取 PrometheusRule 的 YAML
func (p *prometheusRuleOperator) Get(namespace, name string) (string, error) {
	if namespace == "" || name == "" {
		return "", fmt.Errorf("命名空间和名称不能为空")
	}

	var rule *monitoringv1.PrometheusRule
	var err error

	// 优先使用 informer
	if p.useInformer && p.prometheusRuleLister != nil {
		rule, err = p.prometheusRuleLister.PrometheusRules(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return "", fmt.Errorf("PrometheusRule %s/%s 不存在", namespace, name)
			}
			// informer 出错时回退到 API 调用
			rule, err = p.client.MonitoringV1().PrometheusRules(namespace).Get(p.ctx, name, metav1.GetOptions{})
			if err != nil {
				return "", fmt.Errorf("获取 PrometheusRule 失败: %v", err)
			}
		} else {
			// 返回副本以避免修改缓存
			rule = rule.DeepCopy()
		}
	} else {
		// 使用 API 调用
		rule, err = p.client.MonitoringV1().PrometheusRules(namespace).Get(p.ctx, name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return "", fmt.Errorf("PrometheusRule %s/%s 不存在", namespace, name)
			}
			return "", fmt.Errorf("获取 PrometheusRule 失败: %v", err)
		}
	}

	// 注入 TypeMeta
	p.injectTypeMeta(rule)

	// 清理 ManagedFields
	rule.ManagedFields = nil

	// 转换为 YAML
	yamlBytes, err := yaml.Marshal(rule)
	if err != nil {
		return "", fmt.Errorf("转换为 YAML 失败: %v", err)
	}

	return string(yamlBytes), nil
}

// List 获取 PrometheusRule 列表
func (p *prometheusRuleOperator) List(namespace string, search string) (*types.ListPrometheusRuleResponse, error) {
	var rules []*monitoringv1.PrometheusRule
	var err error

	// 优先使用 informer
	if p.useInformer && p.prometheusRuleLister != nil {
		rules, err = p.prometheusRuleLister.PrometheusRules(namespace).List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("获取 PrometheusRule 列表失败: %v", err)
		}
	} else {
		// 使用 API 调用
		ruleList, err := p.client.MonitoringV1().PrometheusRules(namespace).List(p.ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("获取 PrometheusRule 列表失败: %v", err)
		}
		rules = make([]*monitoringv1.PrometheusRule, len(ruleList.Items))
		for i := range ruleList.Items {
			rules[i] = &ruleList.Items[i]
		}
	}

	// 搜索过滤
	if search != "" {
		filtered := make([]*monitoringv1.PrometheusRule, 0)
		searchLower := strings.ToLower(search)
		for _, rule := range rules {
			if strings.Contains(strings.ToLower(rule.Name), searchLower) {
				filtered = append(filtered, rule)
			}
		}
		rules = filtered
	}

	// 转换为响应格式
	items := make([]types.PrometheusRuleInfo, len(rules))
	for i, rule := range rules {
		items[i] = types.PrometheusRuleInfo{
			Name:              rule.Name,
			Namespace:         rule.Namespace,
			Age:               p.formatAge(rule.CreationTimestamp.Time),
			Labels:            rule.Labels,
			Annotations:       rule.Annotations,
			CreationTimestamp: rule.CreationTimestamp.UnixMilli(),
		}
	}

	return &types.ListPrometheusRuleResponse{
		Total: len(items),
		Items: items,
	}, nil
}

// Update 更新 PrometheusRule
func (p *prometheusRuleOperator) Update(namespace, name string, rule *monitoringv1.PrometheusRule) error {
	if namespace == "" || name == "" || rule == nil {
		return fmt.Errorf("命名空间、名称和 PrometheusRule 不能为空")
	}

	// 验证名称和命名空间
	if rule.Name != name {
		return fmt.Errorf("PrometheusRule 名称(%s)与参数名称(%s)不匹配", rule.Name, name)
	}
	if rule.Namespace != "" && rule.Namespace != namespace {
		return fmt.Errorf("PrometheusRule 命名空间(%s)与参数命名空间(%s)不匹配", rule.Namespace, namespace)
	}
	rule.Namespace = namespace

	// 获取现有的 PrometheusRule 以保留 ResourceVersion
	existing, err := p.client.MonitoringV1().PrometheusRules(namespace).Get(p.ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("获取现有 PrometheusRule 失败: %v", err)
	}

	rule.ResourceVersion = existing.ResourceVersion

	// 更新
	_, err = p.client.MonitoringV1().PrometheusRules(namespace).Update(p.ctx, rule, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("更新 PrometheusRule 失败: %v", err)
	}

	return nil
}

// Delete 删除 PrometheusRule
func (p *prometheusRuleOperator) Delete(namespace, name string) error {
	if namespace == "" || name == "" {
		return fmt.Errorf("命名空间和名称不能为空")
	}

	err := p.client.MonitoringV1().PrometheusRules(namespace).Delete(p.ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("删除 PrometheusRule 失败: %v", err)
	}

	return nil
}

// injectTypeMeta 注入 TypeMeta
func (p *prometheusRuleOperator) injectTypeMeta(rule *monitoringv1.PrometheusRule) {
	rule.TypeMeta = metav1.TypeMeta{
		Kind:       "PrometheusRule",
		APIVersion: "monitoring.coreos.com/v1",
	}
}

// formatAge 格式化时间
func (p *prometheusRuleOperator) formatAge(t time.Time) string {
	duration := time.Since(t)
	if duration.Hours() >= 24 {
		days := int(duration.Hours() / 24)
		return fmt.Sprintf("%dd", days)
	}
	if duration.Hours() >= 1 {
		return fmt.Sprintf("%dh", int(duration.Hours()))
	}
	if duration.Minutes() >= 1 {
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	}
	return fmt.Sprintf("%ds", int(duration.Seconds()))
}
