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

type clusterRoleBindingOperator struct {
	BaseOperator
	client                   kubernetes.Interface
	informerFactory          informers.SharedInformerFactory
	clusterRoleBindingLister rbacv1lister.ClusterRoleBindingLister
	clusterRoleLister        rbacv1lister.ClusterRoleLister
	roleBindingLister        rbacv1lister.RoleBindingLister
}

// NewClusterRoleBindingOperator 创建 ClusterRoleBinding 操作器（不使用 informer）
func NewClusterRoleBindingOperator(ctx context.Context, client kubernetes.Interface) types.ClusterRoleBindingOperator {
	return &clusterRoleBindingOperator{
		BaseOperator: NewBaseOperator(ctx, false),
		client:       client,
	}
}

// NewClusterRoleBindingOperatorWithInformer 创建 ClusterRoleBinding 操作器（使用 informer）
func NewClusterRoleBindingOperatorWithInformer(
	ctx context.Context,
	client kubernetes.Interface,
	informerFactory informers.SharedInformerFactory,
) types.ClusterRoleBindingOperator {
	var clusterRoleBindingLister rbacv1lister.ClusterRoleBindingLister
	var clusterRoleLister rbacv1lister.ClusterRoleLister
	var roleBindingLister rbacv1lister.RoleBindingLister

	if informerFactory != nil {
		clusterRoleBindingLister = informerFactory.Rbac().V1().ClusterRoleBindings().Lister()
		clusterRoleLister = informerFactory.Rbac().V1().ClusterRoles().Lister()
		roleBindingLister = informerFactory.Rbac().V1().RoleBindings().Lister()
	}

	return &clusterRoleBindingOperator{
		BaseOperator:             NewBaseOperator(ctx, informerFactory != nil),
		client:                   client,
		informerFactory:          informerFactory,
		clusterRoleBindingLister: clusterRoleBindingLister,
		clusterRoleLister:        clusterRoleLister,
		roleBindingLister:        roleBindingLister,
	}
}

// Create 创建 ClusterRoleBinding
func (c *clusterRoleBindingOperator) Create(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
	if crb == nil || crb.Name == "" {
		return nil, fmt.Errorf("ClusterRoleBinding对象和名称不能为空")
	}
	injectCommonAnnotations(crb)
	created, err := c.client.RbacV1().ClusterRoleBindings().Create(c.ctx, crb, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("创建ClusterRoleBinding失败: %v", err)
	}

	return created, nil
}

// Get 获取 ClusterRoleBinding
func (c *clusterRoleBindingOperator) Get(name string) (*rbacv1.ClusterRoleBinding, error) {
	if name == "" {
		return nil, fmt.Errorf("名称不能为空")
	}

	if c.useInformer && c.clusterRoleBindingLister != nil {
		crb, err := c.clusterRoleBindingLister.Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("ClusterRoleBinding %s 不存在", name)
			}
			crb, apiErr := c.client.RbacV1().ClusterRoleBindings().Get(c.ctx, name, metav1.GetOptions{})
			if apiErr != nil {
				return nil, fmt.Errorf("获取ClusterRoleBinding失败: %v", apiErr)
			}
			c.injectTypeMeta(crb)
			return crb, nil
		}
		result := crb.DeepCopy()
		c.injectTypeMeta(result)
		return result, nil
	}

	crb, err := c.client.RbacV1().ClusterRoleBindings().Get(c.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("ClusterRoleBinding %s 不存在", name)
		}
		return nil, fmt.Errorf("获取ClusterRoleBinding失败: %v", err)
	}

	c.injectTypeMeta(crb)
	return crb, nil
}

// injectTypeMeta 注入 TypeMeta
func (c *clusterRoleBindingOperator) injectTypeMeta(crb *rbacv1.ClusterRoleBinding) {
	crb.TypeMeta = metav1.TypeMeta{
		Kind:       "ClusterRoleBinding",
		APIVersion: "rbac.authorization.k8s.io/v1",
	}
}

// Update 更新 ClusterRoleBinding
func (c *clusterRoleBindingOperator) Update(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
	if crb == nil || crb.Name == "" {
		return nil, fmt.Errorf("ClusterRoleBinding对象和名称不能为空")
	}

	updated, err := c.client.RbacV1().ClusterRoleBindings().Update(c.ctx, crb, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("更新ClusterRoleBinding失败: %v", err)
	}

	return updated, nil
}

// Delete 删除 ClusterRoleBinding
func (c *clusterRoleBindingOperator) Delete(name string) error {
	if name == "" {
		return fmt.Errorf("名称不能为空")
	}

	err := c.client.RbacV1().ClusterRoleBindings().Delete(c.ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("删除ClusterRoleBinding失败: %v", err)
	}

	return nil
}

// List 获取 ClusterRoleBinding 列表
func (c *clusterRoleBindingOperator) List(search string, labelSelector string) (*types.ListClusterRoleBindingResponse, error) {
	var selector labels.Selector = labels.Everything()
	if labelSelector != "" {
		parsedSelector, err := labels.Parse(labelSelector)
		if err != nil {
			return nil, fmt.Errorf("解析标签选择器失败: %v", err)
		}
		selector = parsedSelector
	}

	var crbList []*rbacv1.ClusterRoleBinding
	var err error

	if c.useInformer && c.clusterRoleBindingLister != nil {
		crbList, err = c.clusterRoleBindingLister.List(selector)
		if err != nil {
			return nil, fmt.Errorf("获取ClusterRoleBinding列表失败: %v", err)
		}
	} else {
		listOpts := metav1.ListOptions{LabelSelector: selector.String()}
		list, err := c.client.RbacV1().ClusterRoleBindings().List(c.ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("获取ClusterRoleBinding列表失败: %v", err)
		}
		crbList = make([]*rbacv1.ClusterRoleBinding, len(list.Items))
		for i := range list.Items {
			crbList[i] = &list.Items[i]
		}
	}

	// 搜索过滤
	if search != "" {
		filtered := make([]*rbacv1.ClusterRoleBinding, 0)
		searchLower := strings.ToLower(search)
		for _, crb := range crbList {
			if strings.Contains(strings.ToLower(crb.Name), searchLower) ||
				strings.Contains(strings.ToLower(crb.RoleRef.Name), searchLower) {
				filtered = append(filtered, crb)
			}
		}
		crbList = filtered
	}

	// 转换为响应格式
	items := make([]types.ClusterRoleBindingInfo, len(crbList))
	for i, crb := range crbList {
		items[i] = c.toClusterRoleBindingInfo(crb)
	}

	// 按名称排序
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})

	return &types.ListClusterRoleBindingResponse{
		Total: len(items),
		Items: items,
	}, nil
}

// toClusterRoleBindingInfo 转换为 ClusterRoleBindingInfo
func (c *clusterRoleBindingOperator) toClusterRoleBindingInfo(crb *rbacv1.ClusterRoleBinding) types.ClusterRoleBindingInfo {
	// 分类主体
	var users, groups, serviceAccounts []string
	for _, subject := range crb.Subjects {
		switch subject.Kind {
		case "User":
			users = append(users, subject.Name)
		case "Group":
			groups = append(groups, subject.Name)
		case "ServiceAccount":
			if subject.Namespace != "" {
				serviceAccounts = append(serviceAccounts, fmt.Sprintf("%s/%s", subject.Namespace, subject.Name))
			} else {
				serviceAccounts = append(serviceAccounts, subject.Name)
			}
		}
	}

	// 格式化为字符串
	usersStr := ""
	if len(users) > 0 {
		usersStr = strings.Join(users, ", ")
	}

	groupsStr := ""
	if len(groups) > 0 {
		groupsStr = strings.Join(groups, ", ")
	}

	saStr := ""
	if len(serviceAccounts) > 0 {
		saStr = strings.Join(serviceAccounts, ", ")
	}

	return types.ClusterRoleBindingInfo{
		Name:              crb.Name,
		Role:              fmt.Sprintf("%s/%s", crb.RoleRef.Kind, crb.RoleRef.Name),
		Users:             usersStr,
		Groups:            groupsStr,
		ServiceAccounts:   saStr,
		SubjectCount:      len(crb.Subjects),
		Labels:            crb.Labels,
		Annotations:       crb.Annotations,
		Age:               c.formatAge(crb.CreationTimestamp.Time),
		CreationTimestamp: crb.CreationTimestamp.UnixMilli(),
	}
}

// formatAge 格式化时间
func (c *clusterRoleBindingOperator) formatAge(t time.Time) string {
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

// GetYaml 获取 ClusterRoleBinding 的 YAML
func (c *clusterRoleBindingOperator) GetYaml(name string) (string, error) {
	crb, err := c.Get(name)
	if err != nil {
		return "", err
	}

	crb.ManagedFields = nil

	yamlBytes, err := yaml.Marshal(crb)
	if err != nil {
		return "", fmt.Errorf("转换为YAML失败: %v", err)
	}

	return string(yamlBytes), nil
}

// Describe 获取 ClusterRoleBinding 详情
func (c *clusterRoleBindingOperator) Describe(name string) (string, error) {
	crb, err := c.Get(name)
	if err != nil {
		return "", err
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Name:         %s\n", crb.Name))
	sb.WriteString(fmt.Sprintf("Labels:       %s\n", c.formatLabels(crb.Labels)))
	sb.WriteString(fmt.Sprintf("Annotations:  %s\n", c.formatAnnotations(crb.Annotations)))

	// Role
	sb.WriteString(fmt.Sprintf("Role:\n"))
	sb.WriteString(fmt.Sprintf("  Kind:  %s\n", crb.RoleRef.Kind))
	sb.WriteString(fmt.Sprintf("  Name:  %s\n", crb.RoleRef.Name))

	// Subjects
	sb.WriteString("Subjects:\n")
	if len(crb.Subjects) > 0 {
		sb.WriteString("  Kind            Name                           Namespace\n")
		sb.WriteString("  ----            ----                           ---------\n")
		for _, subject := range crb.Subjects {
			namespace := subject.Namespace
			if namespace == "" {
				namespace = ""
			}
			sb.WriteString(fmt.Sprintf("  %-15s %-30s %s\n", subject.Kind, subject.Name, namespace))
		}
	} else {
		sb.WriteString("  <none>\n")
	}

	return sb.String(), nil
}

// formatLabels 格式化标签
func (c *clusterRoleBindingOperator) formatLabels(labelMap map[string]string) string {
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
func (c *clusterRoleBindingOperator) formatAnnotations(annotations map[string]string) string {
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

// Watch 监听 ClusterRoleBinding 变化
func (c *clusterRoleBindingOperator) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return c.client.RbacV1().ClusterRoleBindings().Watch(c.ctx, opts)
}

// UpdateLabels 更新标签
func (c *clusterRoleBindingOperator) UpdateLabels(name string, labelMap map[string]string) error {
	crb, err := c.Get(name)
	if err != nil {
		return err
	}

	if crb.Labels == nil {
		crb.Labels = make(map[string]string)
	}
	for k, v := range labelMap {
		crb.Labels[k] = v
	}

	_, err = c.Update(crb)
	return err
}

// UpdateAnnotations 更新注解
func (c *clusterRoleBindingOperator) UpdateAnnotations(name string, annotations map[string]string) error {
	crb, err := c.Get(name)
	if err != nil {
		return err
	}

	if crb.Annotations == nil {
		crb.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		crb.Annotations[k] = v
	}

	_, err = c.Update(crb)
	return err
}

// AddSubject 添加主体
func (c *clusterRoleBindingOperator) AddSubject(name string, subject rbacv1.Subject) error {
	crb, err := c.Get(name)
	if err != nil {
		return err
	}

	// 检查是否已存在
	for _, s := range crb.Subjects {
		if s.Kind == subject.Kind && s.Name == subject.Name && s.Namespace == subject.Namespace {
			return fmt.Errorf("主体已存在: %s/%s", subject.Kind, subject.Name)
		}
	}

	crb.Subjects = append(crb.Subjects, subject)
	_, err = c.Update(crb)
	return err
}

// RemoveSubject 移除主体
func (c *clusterRoleBindingOperator) RemoveSubject(name string, subjectKind, subjectName, subjectNamespace string) error {
	crb, err := c.Get(name)
	if err != nil {
		return err
	}

	newSubjects := make([]rbacv1.Subject, 0)
	found := false
	for _, s := range crb.Subjects {
		if s.Kind == subjectKind && s.Name == subjectName && s.Namespace == subjectNamespace {
			found = true
			continue
		}
		newSubjects = append(newSubjects, s)
	}

	if !found {
		return fmt.Errorf("主体不存在: %s/%s", subjectKind, subjectName)
	}

	crb.Subjects = newSubjects
	_, err = c.Update(crb)
	return err
}

// UpdateSubjects 更新所有主体
func (c *clusterRoleBindingOperator) UpdateSubjects(name string, subjects []rbacv1.Subject) error {
	crb, err := c.Get(name)
	if err != nil {
		return err
	}

	crb.Subjects = subjects
	_, err = c.Update(crb)
	return err
}

// GetEffectivePermissions 获取实际授予的权限
func (c *clusterRoleBindingOperator) GetEffectivePermissions(name string) (*types.ClusterRoleBindingEffectivePermissions, error) {
	crb, err := c.Get(name)
	if err != nil {
		return nil, err
	}

	result := &types.ClusterRoleBindingEffectivePermissions{
		BindingName:     name,
		RoleName:        crb.RoleRef.Name,
		Subjects:        make([]types.SubjectInfo, 0),
		Rules:           make([]types.ClusterRoleRuleInfo, 0),
		EffectiveScopes: []string{"cluster-wide"},
		RiskLevel:       "low",
	}

	// 转换主体
	for _, s := range crb.Subjects {
		result.Subjects = append(result.Subjects, types.SubjectInfo{
			Kind:      s.Kind,
			Name:      s.Name,
			Namespace: s.Namespace,
			APIGroup:  s.APIGroup,
		})
	}

	// 获取 ClusterRole 的规则
	var cr *rbacv1.ClusterRole
	if c.useInformer && c.clusterRoleLister != nil {
		cr, err = c.clusterRoleLister.Get(crb.RoleRef.Name)
	} else {
		cr, err = c.client.RbacV1().ClusterRoles().Get(c.ctx, crb.RoleRef.Name, metav1.GetOptions{})
	}

	if err != nil {
		if errors.IsNotFound(err) {
			result.RoleExists = false
			return result, nil
		}
		return nil, fmt.Errorf("获取ClusterRole失败: %v", err)
	}

	result.RoleExists = true

	// 转换规则
	for _, rule := range cr.Rules {
		ruleInfo := types.ClusterRoleRuleInfo{
			Verbs:           rule.Verbs,
			APIGroups:       rule.APIGroups,
			Resources:       rule.Resources,
			ResourceNames:   rule.ResourceNames,
			NonResourceURLs: rule.NonResourceURLs,
		}
		result.Rules = append(result.Rules, ruleInfo)

		// 检查是否是超级管理员权限
		for _, verb := range rule.Verbs {
			if verb == "*" {
				for _, resource := range rule.Resources {
					if resource == "*" {
						result.IsSuperAdmin = true
						result.RiskLevel = "critical"
					}
				}
			}
		}
	}

	// 评估风险等级
	if !result.IsSuperAdmin {
		if c.hasHighRiskPermissions(cr.Rules) {
			result.RiskLevel = "high"
		} else if c.hasMediumRiskPermissions(cr.Rules) {
			result.RiskLevel = "medium"
		}
	}

	return result, nil
}

// hasHighRiskPermissions 检查是否有高风险权限
func (c *clusterRoleBindingOperator) hasHighRiskPermissions(rules []rbacv1.PolicyRule) bool {
	for _, rule := range rules {
		for _, verb := range rule.Verbs {
			if verb == "*" || verb == "delete" || verb == "deletecollection" {
				return true
			}
		}
	}
	return false
}

// hasMediumRiskPermissions 检查是否有中风险权限
func (c *clusterRoleBindingOperator) hasMediumRiskPermissions(rules []rbacv1.PolicyRule) bool {
	for _, rule := range rules {
		for _, resource := range rule.Resources {
			if resource == "secrets" || resource == "configmaps" {
				return true
			}
		}
	}
	return false
}

// GetByClusterRole 根据 ClusterRole 名称获取绑定列表
func (c *clusterRoleBindingOperator) GetByClusterRole(clusterRoleName string) (*types.ListClusterRoleBindingResponse, error) {
	response, err := c.List("", "")
	if err != nil {
		return nil, err
	}

	// 过滤
	filtered := make([]types.ClusterRoleBindingInfo, 0)
	for _, item := range response.Items {
		if strings.HasSuffix(item.Role, "/"+clusterRoleName) {
			filtered = append(filtered, item)
		}
	}

	return &types.ListClusterRoleBindingResponse{
		Total: len(filtered),
		Items: filtered,
	}, nil
}

// GetBySubject 根据主体获取绑定列表
func (c *clusterRoleBindingOperator) GetBySubject(subjectKind, subjectName, subjectNamespace string) (*types.SubjectBindingsResponse, error) {
	response := &types.SubjectBindingsResponse{
		SubjectKind:           subjectKind,
		SubjectName:           subjectName,
		SubjectNamespace:      subjectNamespace,
		ClusterRoleBindings:   make([]types.ClusterRoleBindingInfo, 0),
		RoleBindings:          make([]types.RoleBindingInfo, 0),
		EffectiveClusterRoles: make([]string, 0),
		EffectiveRoles:        make([]types.NamespacedRoleInfo, 0),
	}

	// 获取所有 ClusterRoleBinding
	crbList, err := c.List("", "")
	if err != nil {
		return nil, err
	}

	// 过滤匹配的 ClusterRoleBinding
	for _, crb := range crbList.Items {
		if c.subjectMatches(crb, subjectKind, subjectName, subjectNamespace) {
			response.ClusterRoleBindings = append(response.ClusterRoleBindings, crb)
			// 提取角色名称
			parts := strings.Split(crb.Role, "/")
			if len(parts) == 2 {
				response.EffectiveClusterRoles = appendUniqueStr(response.EffectiveClusterRoles, parts[1])
			}
		}
	}

	// 获取所有 RoleBinding
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

	// 过滤匹配的 RoleBinding
	for _, rb := range rbList {
		for _, subject := range rb.Subjects {
			if subject.Kind == subjectKind && subject.Name == subjectName {
				if subjectKind == "ServiceAccount" && subjectNamespace != "" && subject.Namespace != subjectNamespace {
					continue
				}
				response.RoleBindings = append(response.RoleBindings, types.RoleBindingInfo{
					Name:      rb.Name,
					Namespace: rb.Namespace,
					RoleName:  rb.RoleRef.Name,
					RoleKind:  rb.RoleRef.Kind,
					Age:       c.formatAge(rb.CreationTimestamp.Time),
				})
				response.EffectiveRoles = append(response.EffectiveRoles, types.NamespacedRoleInfo{
					RoleName:  rb.RoleRef.Name,
					RoleKind:  rb.RoleRef.Kind,
					Namespace: rb.Namespace,
				})
				break
			}
		}
	}

	response.TotalBindings = len(response.ClusterRoleBindings) + len(response.RoleBindings)

	return response, nil
}

// subjectMatches 检查主体是否匹配
func (c *clusterRoleBindingOperator) subjectMatches(crb types.ClusterRoleBindingInfo, kind, name, namespace string) bool {
	switch kind {
	case "User":
		return strings.Contains(crb.Users, name)
	case "Group":
		return strings.Contains(crb.Groups, name)
	case "ServiceAccount":
		if namespace != "" {
			return strings.Contains(crb.ServiceAccounts, fmt.Sprintf("%s/%s", namespace, name))
		}
		return strings.Contains(crb.ServiceAccounts, name)
	}
	return false
}

// appendUniqueStr 追加不重复的字符串
func appendUniqueStr(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}

// GetServiceAccountBindings 获取 ServiceAccount 的所有绑定
func (c *clusterRoleBindingOperator) GetServiceAccountBindings(namespace, name string) (*types.SubjectBindingsResponse, error) {
	return c.GetBySubject("ServiceAccount", name, namespace)
}

// CanDelete 检查是否可以安全删除
func (c *clusterRoleBindingOperator) CanDelete(name string) (bool, string, error) {
	crb, err := c.Get(name)
	if err != nil {
		return false, "", err
	}

	// 检查是否是系统关键绑定
	if strings.HasPrefix(crb.Name, "system:") {
		return false, "系统关键ClusterRoleBinding不建议删除", nil
	}

	// 检查绑定的主体数量
	if len(crb.Subjects) > 0 {
		return true, fmt.Sprintf("删除将移除%d个主体的集群级别权限", len(crb.Subjects)), nil
	}

	return true, "", nil
}

// ValidateRoleRef 验证角色引用是否有效
func (c *clusterRoleBindingOperator) ValidateRoleRef(name string) (bool, string, error) {
	crb, err := c.Get(name)
	if err != nil {
		return false, "", err
	}

	// 检查 ClusterRole 是否存在
	var cr *rbacv1.ClusterRole
	if c.useInformer && c.clusterRoleLister != nil {
		cr, err = c.clusterRoleLister.Get(crb.RoleRef.Name)
	} else {
		cr, err = c.client.RbacV1().ClusterRoles().Get(c.ctx, crb.RoleRef.Name, metav1.GetOptions{})
	}

	if err != nil {
		if errors.IsNotFound(err) {
			return false, fmt.Sprintf("引用的ClusterRole %s 不存在", crb.RoleRef.Name), nil
		}
		return false, "", err
	}

	return true, fmt.Sprintf("ClusterRole %s 存在，包含 %d 条规则", cr.Name, len(cr.Rules)), nil
}
