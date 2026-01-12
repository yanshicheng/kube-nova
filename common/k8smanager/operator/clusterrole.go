package operator

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	rbacv1lister "k8s.io/client-go/listers/rbac/v1"
	"sigs.k8s.io/yaml"
)

type clusterRoleOperator struct {
	BaseOperator
	client                   kubernetes.Interface
	informerFactory          informers.SharedInformerFactory
	clusterRoleLister        rbacv1lister.ClusterRoleLister
	clusterRoleBindingLister rbacv1lister.ClusterRoleBindingLister
	roleBindingLister        rbacv1lister.RoleBindingLister
}

// NewClusterRoleOperator 创建 ClusterRole 操作器（不使用 informer）
func NewClusterRoleOperator(ctx context.Context, client kubernetes.Interface) types.ClusterRoleOperator {
	return &clusterRoleOperator{
		BaseOperator: NewBaseOperator(ctx, false),
		client:       client,
	}
}

// NewClusterRoleOperatorWithInformer 创建 ClusterRole 操作器（使用 informer）
func NewClusterRoleOperatorWithInformer(
	ctx context.Context,
	client kubernetes.Interface,
	informerFactory informers.SharedInformerFactory,
) types.ClusterRoleOperator {
	var clusterRoleLister rbacv1lister.ClusterRoleLister
	var clusterRoleBindingLister rbacv1lister.ClusterRoleBindingLister
	var roleBindingLister rbacv1lister.RoleBindingLister

	if informerFactory != nil {
		clusterRoleLister = informerFactory.Rbac().V1().ClusterRoles().Lister()
		clusterRoleBindingLister = informerFactory.Rbac().V1().ClusterRoleBindings().Lister()
		roleBindingLister = informerFactory.Rbac().V1().RoleBindings().Lister()
	}

	return &clusterRoleOperator{
		BaseOperator:             NewBaseOperator(ctx, informerFactory != nil),
		client:                   client,
		informerFactory:          informerFactory,
		clusterRoleLister:        clusterRoleLister,
		clusterRoleBindingLister: clusterRoleBindingLister,
		roleBindingLister:        roleBindingLister,
	}
}

// Create 创建 ClusterRole
func (c *clusterRoleOperator) Create(cr *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
	if cr == nil || cr.Name == "" {
		return nil, fmt.Errorf("ClusterRole对象和名称不能为空")
	}
	injectCommonAnnotations(cr)
	created, err := c.client.RbacV1().ClusterRoles().Create(c.ctx, cr, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("创建ClusterRole失败: %v", err)
	}

	return created, nil
}

// Get 获取 ClusterRole
func (c *clusterRoleOperator) Get(name string) (*rbacv1.ClusterRole, error) {
	if name == "" {
		return nil, fmt.Errorf("名称不能为空")
	}

	// 优先使用 informer
	if c.useInformer && c.clusterRoleLister != nil {
		cr, err := c.clusterRoleLister.Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("ClusterRole %s 不存在", name)
			}
			cr, apiErr := c.client.RbacV1().ClusterRoles().Get(c.ctx, name, metav1.GetOptions{})
			if apiErr != nil {
				return nil, fmt.Errorf("获取ClusterRole失败: %v", apiErr)
			}
			c.injectTypeMeta(cr)
			return cr, nil
		}
		result := cr.DeepCopy()
		c.injectTypeMeta(result)
		return result, nil
	}

	cr, err := c.client.RbacV1().ClusterRoles().Get(c.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("ClusterRole %s 不存在", name)
		}
		return nil, fmt.Errorf("获取ClusterRole失败: %v", err)
	}

	c.injectTypeMeta(cr)
	return cr, nil
}

// injectTypeMeta 注入 TypeMeta
func (c *clusterRoleOperator) injectTypeMeta(cr *rbacv1.ClusterRole) {
	cr.TypeMeta = metav1.TypeMeta{
		Kind:       "ClusterRole",
		APIVersion: "rbac.authorization.k8s.io/v1",
	}
}

// Update 更新 ClusterRole
func (c *clusterRoleOperator) Update(cr *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
	if cr == nil || cr.Name == "" {
		return nil, fmt.Errorf("ClusterRole对象和名称不能为空")
	}

	updated, err := c.client.RbacV1().ClusterRoles().Update(c.ctx, cr, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("更新ClusterRole失败: %v", err)
	}

	return updated, nil
}

// Delete 删除 ClusterRole
func (c *clusterRoleOperator) Delete(name string) error {
	if name == "" {
		return fmt.Errorf("名称不能为空")
	}

	err := c.client.RbacV1().ClusterRoles().Delete(c.ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("删除ClusterRole失败: %v", err)
	}

	return nil
}

// List 获取 ClusterRole 列表
func (c *clusterRoleOperator) List(search string, labelSelector string) (*types.ListClusterRoleResponse, error) {
	var selector labels.Selector = labels.Everything()
	if labelSelector != "" {
		parsedSelector, err := labels.Parse(labelSelector)
		if err != nil {
			return nil, fmt.Errorf("解析标签选择器失败: %v", err)
		}
		selector = parsedSelector
	}

	var clusterRoles []*rbacv1.ClusterRole
	var err error

	if c.useInformer && c.clusterRoleLister != nil {
		clusterRoles, err = c.clusterRoleLister.List(selector)
		if err != nil {
			return nil, fmt.Errorf("获取ClusterRole列表失败: %v", err)
		}
	} else {
		listOpts := metav1.ListOptions{LabelSelector: selector.String()}
		crList, err := c.client.RbacV1().ClusterRoles().List(c.ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("获取ClusterRole列表失败: %v", err)
		}
		clusterRoles = make([]*rbacv1.ClusterRole, len(crList.Items))
		for i := range crList.Items {
			clusterRoles[i] = &crList.Items[i]
		}
	}

	// 搜索过滤
	if search != "" {
		filtered := make([]*rbacv1.ClusterRole, 0)
		searchLower := strings.ToLower(search)
		for _, cr := range clusterRoles {
			if strings.Contains(strings.ToLower(cr.Name), searchLower) {
				filtered = append(filtered, cr)
			}
		}
		clusterRoles = filtered
	}

	// 转换为响应格式
	items := make([]types.ClusterRoleInfo, len(clusterRoles))
	for i, cr := range clusterRoles {
		items[i] = c.toClusterRoleInfo(cr)
	}

	// 按名称排序
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})

	return &types.ListClusterRoleResponse{
		Total: len(items),
		Items: items,
	}, nil
}

// toClusterRoleInfo 转换为 ClusterRoleInfo
func (c *clusterRoleOperator) toClusterRoleInfo(cr *rbacv1.ClusterRole) types.ClusterRoleInfo {
	// 提取聚合标签
	var aggregationLabels []string
	if cr.AggregationRule != nil && len(cr.AggregationRule.ClusterRoleSelectors) > 0 {
		for _, selector := range cr.AggregationRule.ClusterRoleSelectors {
			for k, v := range selector.MatchLabels {
				aggregationLabels = append(aggregationLabels, fmt.Sprintf("%s=%s", k, v))
			}
		}
	}

	return types.ClusterRoleInfo{
		Name:              cr.Name,
		CreatedAt:         cr.CreationTimestamp.Format(time.RFC3339),
		RuleCount:         len(cr.Rules),
		AggregationLabels: aggregationLabels,
		Labels:            cr.Labels,
		Annotations:       cr.Annotations,
		Age:               c.formatAge(cr.CreationTimestamp.Time),
		CreationTimestamp: cr.CreationTimestamp.UnixMilli(),
	}
}

// formatAge 格式化时间
func (c *clusterRoleOperator) formatAge(t time.Time) string {
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

// GetYaml 获取 ClusterRole 的 YAML
func (c *clusterRoleOperator) GetYaml(name string) (string, error) {
	cr, err := c.Get(name)
	if err != nil {
		return "", err
	}

	cr.ManagedFields = nil

	yamlBytes, err := yaml.Marshal(cr)
	if err != nil {
		return "", fmt.Errorf("转换为YAML失败: %v", err)
	}

	return string(yamlBytes), nil
}

// Describe 获取 ClusterRole 详情
func (c *clusterRoleOperator) Describe(name string) (string, error) {
	cr, err := c.Get(name)
	if err != nil {
		return "", err
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Name:         %s\n", cr.Name))
	sb.WriteString(fmt.Sprintf("Labels:       %s\n", c.formatLabels(cr.Labels)))
	sb.WriteString(fmt.Sprintf("Annotations:  %s\n", c.formatAnnotations(cr.Annotations)))

	// PolicyRule
	sb.WriteString("PolicyRule:\n")
	if len(cr.Rules) > 0 {
		sb.WriteString("  Resources                                    Non-Resource URLs  Resource Names  Verbs\n")
		sb.WriteString("  ---------                                    -----------------  --------------  -----\n")
		for _, rule := range cr.Rules {
			resources := c.formatResources(rule.APIGroups, rule.Resources)
			nonResourceURLs := strings.Join(rule.NonResourceURLs, ", ")
			if nonResourceURLs == "" {
				nonResourceURLs = "[]"
			}
			resourceNames := strings.Join(rule.ResourceNames, ", ")
			if resourceNames == "" {
				resourceNames = "[]"
			}
			verbs := strings.Join(rule.Verbs, ", ")
			if verbs == "" {
				verbs = "[]"
			}

			sb.WriteString(fmt.Sprintf("  %-44s %-18s %-15s [%s]\n",
				resources, nonResourceURLs, resourceNames, verbs))
		}
	} else {
		sb.WriteString("  <none>\n")
	}

	// AggregationRule
	if cr.AggregationRule != nil && len(cr.AggregationRule.ClusterRoleSelectors) > 0 {
		sb.WriteString("AggregationRule:\n")
		for _, selector := range cr.AggregationRule.ClusterRoleSelectors {
			sb.WriteString(fmt.Sprintf("  ClusterRoleSelector: %s\n", c.formatLabelSelector(&selector)))
		}
	}

	return sb.String(), nil
}

// formatResources 格式化资源
func (c *clusterRoleOperator) formatResources(apiGroups, resources []string) string {
	if len(resources) == 0 {
		return "[]"
	}

	var result []string
	for _, r := range resources {
		for _, g := range apiGroups {
			if g == "" {
				result = append(result, r)
			} else {
				result = append(result, fmt.Sprintf("%s.%s", r, g))
			}
		}
	}

	if len(result) == 0 {
		return strings.Join(resources, ", ")
	}
	return strings.Join(result, ", ")
}

// formatLabels 格式化标签
func (c *clusterRoleOperator) formatLabels(labelMap map[string]string) string {
	if len(labelMap) == 0 {
		return "<none>"
	}
	parts := make([]string, 0, len(labelMap))
	for k, v := range labelMap {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(parts, ", ")
}

// formatAnnotations 格式化注解
func (c *clusterRoleOperator) formatAnnotations(annotations map[string]string) string {
	if len(annotations) == 0 {
		return "<none>"
	}
	parts := make([]string, 0, len(annotations))
	for k, v := range annotations {
		if len(v) > 50 {
			v = v[:50] + "..."
		}
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(parts, ", ")
}

// formatLabelSelector 格式化标签选择器
func (c *clusterRoleOperator) formatLabelSelector(selector *metav1.LabelSelector) string {
	if selector == nil {
		return "<none>"
	}
	var parts []string
	for k, v := range selector.MatchLabels {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	for _, expr := range selector.MatchExpressions {
		parts = append(parts, fmt.Sprintf("%s %s %v", expr.Key, expr.Operator, expr.Values))
	}
	if len(parts) == 0 {
		return "<none>"
	}
	return strings.Join(parts, ", ")
}

// Watch 监听 ClusterRole 变化
func (c *clusterRoleOperator) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return c.client.RbacV1().ClusterRoles().Watch(c.ctx, opts)
}

// UpdateLabels 更新标签
func (c *clusterRoleOperator) UpdateLabels(name string, labelMap map[string]string) error {
	cr, err := c.Get(name)
	if err != nil {
		return err
	}

	if cr.Labels == nil {
		cr.Labels = make(map[string]string)
	}
	for k, v := range labelMap {
		cr.Labels[k] = v
	}

	_, err = c.Update(cr)
	return err
}

// UpdateAnnotations 更新注解
func (c *clusterRoleOperator) UpdateAnnotations(name string, annotations map[string]string) error {
	cr, err := c.Get(name)
	if err != nil {
		return err
	}

	if cr.Annotations == nil {
		cr.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		cr.Annotations[k] = v
	}

	_, err = c.Update(cr)
	return err
}

// AddRule 添加规则
func (c *clusterRoleOperator) AddRule(name string, rule rbacv1.PolicyRule) error {
	cr, err := c.Get(name)
	if err != nil {
		return err
	}

	cr.Rules = append(cr.Rules, rule)
	_, err = c.Update(cr)
	return err
}

// RemoveRule 移除规则（按索引）
func (c *clusterRoleOperator) RemoveRule(name string, ruleIndex int) error {
	cr, err := c.Get(name)
	if err != nil {
		return err
	}

	if ruleIndex < 0 || ruleIndex >= len(cr.Rules) {
		return fmt.Errorf("规则索引超出范围: %d", ruleIndex)
	}

	cr.Rules = append(cr.Rules[:ruleIndex], cr.Rules[ruleIndex+1:]...)
	_, err = c.Update(cr)
	return err
}

// UpdateRules 更新所有规则
func (c *clusterRoleOperator) UpdateRules(name string, rules []rbacv1.PolicyRule) error {
	cr, err := c.Get(name)
	if err != nil {
		return err
	}

	cr.Rules = rules
	_, err = c.Update(cr)
	return err
}

// GetUsage 获取 ClusterRole 被哪些 ClusterRoleBinding/RoleBinding 引用
func (c *clusterRoleOperator) GetUsage(name string) (*types.ClusterRoleUsageResponse, error) {
	// 验证 ClusterRole 存在
	_, err := c.Get(name)
	if err != nil {
		return nil, err
	}

	response := &types.ClusterRoleUsageResponse{
		ClusterRoleName:     name,
		ClusterRoleBindings: make([]types.ClusterRoleBindingRef, 0),
		RoleBindings:        make([]types.RoleBindingRef, 0),
	}

	// 获取所有 ClusterRoleBinding
	var crbList []*rbacv1.ClusterRoleBinding
	if c.useInformer && c.clusterRoleBindingLister != nil {
		crbList, _ = c.clusterRoleBindingLister.List(labels.Everything())
	} else {
		list, err := c.client.RbacV1().ClusterRoleBindings().List(c.ctx, metav1.ListOptions{})
		if err == nil {
			crbList = make([]*rbacv1.ClusterRoleBinding, len(list.Items))
			for i := range list.Items {
				crbList[i] = &list.Items[i]
			}
		}
	}

	// 过滤引用该 ClusterRole 的 ClusterRoleBinding
	for _, crb := range crbList {
		if crb.RoleRef.Kind == "ClusterRole" && crb.RoleRef.Name == name {
			subjects := c.convertSubjects(crb.Subjects)
			response.ClusterRoleBindings = append(response.ClusterRoleBindings, types.ClusterRoleBindingRef{
				Name:     crb.Name,
				Subjects: subjects,
				Age:      c.formatAge(crb.CreationTimestamp.Time),
			})

			// 统计主体
			for _, s := range subjects {
				switch s.Kind {
				case "ServiceAccount":
					response.ServiceAccountCount++
				case "User":
					response.UserCount++
				case "Group":
					response.GroupCount++
				}
			}
		}
	}

	// 获取所有 RoleBinding（跨所有命名空间）
	var rbList []*rbacv1.RoleBinding
	if c.useInformer && c.roleBindingLister != nil {
		rbList, _ = c.roleBindingLister.List(labels.Everything())
	} else {
		list, err := c.client.RbacV1().RoleBindings("").List(c.ctx, metav1.ListOptions{})
		if err == nil {
			rbList = make([]*rbacv1.RoleBinding, len(list.Items))
			for i := range list.Items {
				rbList[i] = &list.Items[i]
			}
		}
	}

	// 过滤引用该 ClusterRole 的 RoleBinding
	for _, rb := range rbList {
		if rb.RoleRef.Kind == "ClusterRole" && rb.RoleRef.Name == name {
			subjects := c.convertSubjects(rb.Subjects)
			response.RoleBindings = append(response.RoleBindings, types.RoleBindingRef{
				Name:      rb.Name,
				Namespace: rb.Namespace,
				Subjects:  subjects,
				Age:       c.formatAge(rb.CreationTimestamp.Time),
			})

			// 统计主体（避免重复计算）
			for _, s := range subjects {
				switch s.Kind {
				case "ServiceAccount":
					response.ServiceAccountCount++
				case "User":
					response.UserCount++
				case "Group":
					response.GroupCount++
				}
			}
		}
	}

	response.BindingCount = len(response.ClusterRoleBindings)
	response.RoleBindingCount = len(response.RoleBindings)
	response.TotalSubjects = response.ServiceAccountCount + response.UserCount + response.GroupCount

	response.CanDelete = response.BindingCount == 0 && response.RoleBindingCount == 0
	if !response.CanDelete {
		response.DeleteWarning = fmt.Sprintf("ClusterRole被%d个ClusterRoleBinding和%d个RoleBinding引用，删除可能影响这些绑定的权限",
			response.BindingCount, response.RoleBindingCount)
	}

	return response, nil
}

// convertSubjects 转换主体列表
func (c *clusterRoleOperator) convertSubjects(subjects []rbacv1.Subject) []types.SubjectInfo {
	result := make([]types.SubjectInfo, len(subjects))
	for i, s := range subjects {
		result[i] = types.SubjectInfo{
			Kind:      s.Kind,
			Name:      s.Name,
			Namespace: s.Namespace,
			APIGroup:  s.APIGroup,
		}
	}
	return result
}

// GetPermissionSummary 获取权限摘要分析
func (c *clusterRoleOperator) GetPermissionSummary(name string) (*types.ClusterRolePermissionSummary, error) {
	cr, err := c.Get(name)
	if err != nil {
		return nil, err
	}

	summary := &types.ClusterRolePermissionSummary{
		ClusterRoleName: name,
		VerbSummary:     make(map[string][]string),
		ResourceSummary: make(map[string][]string),
		RiskLevel:       "low",
	}

	for _, rule := range cr.Rules {
		// 检查是否有 * 权限
		for _, verb := range rule.Verbs {
			if verb == "*" {
				summary.HasAllVerbs = true
			}
		}
		for _, resource := range rule.Resources {
			if resource == "*" {
				summary.HasAllResources = true
			}
		}

		// 按动作分组
		for _, verb := range rule.Verbs {
			for _, resource := range rule.Resources {
				for _, apiGroup := range rule.APIGroups {
					fullResource := resource
					if apiGroup != "" {
						fullResource = fmt.Sprintf("%s.%s", resource, apiGroup)
					}
					summary.VerbSummary[verb] = appendUnique(summary.VerbSummary[verb], fullResource)
					summary.ResourceSummary[fullResource] = appendUnique(summary.ResourceSummary[fullResource], verb)
				}
			}
		}
	}

	// 判断风险等级
	if summary.HasAllVerbs && summary.HasAllResources {
		summary.IsSuperAdmin = true
		summary.RiskLevel = "critical"
	} else if summary.HasAllVerbs || summary.HasAllResources {
		summary.RiskLevel = "high"
	} else if c.hasSecretsAccess(cr.Rules) || c.hasPodsExecAccess(cr.Rules) {
		summary.RiskLevel = "medium"
	}

	return summary, nil
}

// hasSecretsAccess 检查是否有 secrets 访问权限
func (c *clusterRoleOperator) hasSecretsAccess(rules []rbacv1.PolicyRule) bool {
	for _, rule := range rules {
		for _, resource := range rule.Resources {
			if resource == "secrets" || resource == "*" {
				for _, verb := range rule.Verbs {
					if verb == "get" || verb == "list" || verb == "*" {
						return true
					}
				}
			}
		}
	}
	return false
}

// hasPodsExecAccess 检查是否有 pods/exec 权限
func (c *clusterRoleOperator) hasPodsExecAccess(rules []rbacv1.PolicyRule) bool {
	for _, rule := range rules {
		for _, resource := range rule.Resources {
			if resource == "pods/exec" || resource == "*" {
				for _, verb := range rule.Verbs {
					if verb == "create" || verb == "*" {
						return true
					}
				}
			}
		}
	}
	return false
}

// appendUnique 追加不重复的元素
func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}

// CanDelete 检查是否可以安全删除
func (c *clusterRoleOperator) CanDelete(name string) (bool, string, error) {
	usage, err := c.GetUsage(name)
	if err != nil {
		return false, "", err
	}

	return usage.CanDelete, usage.DeleteWarning, nil
}

// GetByAggregationLabel 根据聚合标签获取 ClusterRole 列表
func (c *clusterRoleOperator) GetByAggregationLabel(labelSelector string) (*types.ListClusterRoleResponse, error) {
	return c.List("", labelSelector)
}
