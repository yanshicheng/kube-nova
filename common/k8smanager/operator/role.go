package operator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	rbacv1lister "k8s.io/client-go/listers/rbac/v1"
	"sigs.k8s.io/yaml"
)

type roleOperator struct {
	BaseOperator
	client            kubernetes.Interface
	informerFactory   informers.SharedInformerFactory
	roleLister        rbacv1lister.RoleLister
	roleBindingLister rbacv1lister.RoleBindingLister
}

// NewRoleOperator 创建 Role 操作器（不使用 informer）
func NewRoleOperator(ctx context.Context, client kubernetes.Interface) types.RoleOperator {
	return &roleOperator{
		BaseOperator: NewBaseOperator(ctx, false),
		client:       client,
	}
}

// NewRoleOperatorWithInformer 创建 Role 操作器（使用 informer）
func NewRoleOperatorWithInformer(
	ctx context.Context,
	client kubernetes.Interface,
	informerFactory informers.SharedInformerFactory,
) types.RoleOperator {
	var roleLister rbacv1lister.RoleLister
	var roleBindingLister rbacv1lister.RoleBindingLister

	if informerFactory != nil {
		roleLister = informerFactory.Rbac().V1().Roles().Lister()
		roleBindingLister = informerFactory.Rbac().V1().RoleBindings().Lister()
	}

	return &roleOperator{
		BaseOperator:      NewBaseOperator(ctx, informerFactory != nil),
		client:            client,
		informerFactory:   informerFactory,
		roleLister:        roleLister,
		roleBindingLister: roleBindingLister,
	}
}

// Create
func (r *roleOperator) Create(role *rbacv1.Role) error {
	if role == nil || role.Name == "" {
		return fmt.Errorf("Role 不能为空")
	}
	// 判断是否已经存在
	if _, err := r.Get(role.Namespace, role.Name); err == nil {
		return fmt.Errorf("Role %s/%s 已经存在", role.Namespace, role.Name)
	}
	_, err := r.client.RbacV1().Roles(role.Namespace).Create(r.ctx, role, metav1.CreateOptions{})
	return err

}

// Get 获取 Role
func (r *roleOperator) Get(namespace, name string) (*rbacv1.Role, error) {
	if namespace == "" || name == "" {
		return nil, fmt.Errorf("命名空间和名称不能为空")
	}

	// 优先使用 informer
	if r.useInformer && r.roleLister != nil {
		role, err := r.roleLister.Roles(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("Role %s/%s 不存在", namespace, name)
			}
			// informer 出错时回退到 API 调用
			role, apiErr := r.client.RbacV1().Roles(namespace).Get(r.ctx, name, metav1.GetOptions{})
			if apiErr != nil {
				return nil, fmt.Errorf("获取Role失败: %v", apiErr)
			}
			r.injectTypeMeta(role)
			return role, nil
		}
		// 返回副本以避免修改缓存
		result := role.DeepCopy()
		r.injectTypeMeta(result)
		return result, nil
	}

	// 使用 API 调用
	role, err := r.client.RbacV1().Roles(namespace).Get(r.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("Role %s/%s 不存在", namespace, name)
		}
		return nil, fmt.Errorf("获取Role失败: %v", err)
	}

	r.injectTypeMeta(role)
	return role, nil
}

// injectTypeMeta 注入 TypeMeta
func (r *roleOperator) injectTypeMeta(role *rbacv1.Role) {
	role.TypeMeta = metav1.TypeMeta{
		Kind:       "Role",
		APIVersion: "rbac.authorization.k8s.io/v1",
	}
}

// List 获取 Role 列表
func (r *roleOperator) List(namespace string, search string, labelSelector string) (*types.ListRoleResponse, error) {
	var selector labels.Selector = labels.Everything()
	if labelSelector != "" {
		parsedSelector, err := labels.Parse(labelSelector)
		if err != nil {
			return nil, fmt.Errorf("解析标签选择器失败: %v", err)
		}
		selector = parsedSelector
	}

	var roles []*rbacv1.Role
	var err error

	// 优先使用 informer
	if r.useInformer && r.roleLister != nil {
		roles, err = r.roleLister.Roles(namespace).List(selector)
		if err != nil {
			return nil, fmt.Errorf("获取Role列表失败: %v", err)
		}
	} else {
		// 使用 API 调用
		listOpts := metav1.ListOptions{LabelSelector: selector.String()}
		roleList, err := r.client.RbacV1().Roles(namespace).List(r.ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("获取Role列表失败: %v", err)
		}
		roles = make([]*rbacv1.Role, len(roleList.Items))
		for i := range roleList.Items {
			roles[i] = &roleList.Items[i]
		}
	}

	// 搜索过滤
	if search != "" {
		filtered := make([]*rbacv1.Role, 0)
		searchLower := strings.ToLower(search)
		for _, role := range roles {
			if strings.Contains(strings.ToLower(role.Name), searchLower) {
				filtered = append(filtered, role)
			}
		}
		roles = filtered
	}

	// 转换为响应格式
	items := make([]types.RoleInfo, len(roles))
	for i, role := range roles {
		items[i] = types.RoleInfo{
			Name:              role.Name,
			Namespace:         role.Namespace,
			Age:               r.formatAge(role.CreationTimestamp.Time),
			Labels:            role.Labels,
			Annotations:       role.Annotations,
			CreationTimestamp: role.CreationTimestamp.UnixMilli(),
		}
	}

	return &types.ListRoleResponse{
		Total: len(items),
		Items: items,
	}, nil
}

// GetYaml 获取 Role 的 YAML
func (r *roleOperator) GetYaml(namespace, name string) (string, error) {
	role, err := r.Get(namespace, name)
	if err != nil {
		return "", err
	}

	// 清理 ManagedFields
	role.ManagedFields = nil

	yamlBytes, err := yaml.Marshal(role)
	if err != nil {
		return "", fmt.Errorf("转换为YAML失败: %v", err)
	}

	return string(yamlBytes), nil
}

// Update 更新 Role
func (r *roleOperator) Update(namespace, name string, role *rbacv1.Role) error {
	// 验证名称和命名空间
	if role.Name != name {
		return fmt.Errorf("YAML中的名称(%s)与参数名称(%s)不匹配", role.Name, name)
	}
	if role.Namespace != "" && role.Namespace != namespace {
		return fmt.Errorf("YAML中的命名空间(%s)与参数命名空间(%s)不匹配", role.Namespace, namespace)
	}
	role.Namespace = namespace

	// 获取现有的 Role 以保留 ResourceVersion
	existing, err := r.Get(namespace, name)
	if err != nil {
		return fmt.Errorf("获取现有Role失败: %v", err)
	}

	role.ResourceVersion = existing.ResourceVersion

	// 更新
	_, err = r.client.RbacV1().Roles(namespace).Update(r.ctx, role, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("更新Role失败: %v", err)
	}

	return nil
}

// Describe 获取 Role 详情
func (r *roleOperator) Describe(namespace, name string) (string, error) {
	role, err := r.Get(namespace, name)
	if err != nil {
		return "", err
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Name:         %s\n", role.Name))
	sb.WriteString(fmt.Sprintf("Namespace:    %s\n", role.Namespace))

	// Labels
	if len(role.Labels) > 0 {
		sb.WriteString("Labels:       ")
		labels := make([]string, 0, len(role.Labels))
		for k, v := range role.Labels {
			labels = append(labels, fmt.Sprintf("%s=%s", k, v))
		}
		sb.WriteString(strings.Join(labels, "\n              ") + "\n")
	} else {
		sb.WriteString("Labels:       <none>\n")
	}

	// Annotations
	if len(role.Annotations) > 0 {
		sb.WriteString("Annotations:  ")
		annotations := make([]string, 0, len(role.Annotations))
		for k, v := range role.Annotations {
			if len(v) > 50 {
				v = v[:50] + "..."
			}
			annotations = append(annotations, fmt.Sprintf("%s=%s", k, v))
		}
		sb.WriteString(strings.Join(annotations, "\n              ") + "\n")
	} else {
		sb.WriteString("Annotations:  <none>\n")
	}

	// PolicyRules
	if len(role.Rules) > 0 {
		sb.WriteString("PolicyRule:\n")
		for i, rule := range role.Rules {
			sb.WriteString(fmt.Sprintf("  Rule %d:\n", i+1))

			// Resources
			if len(rule.Resources) > 0 {
				sb.WriteString(fmt.Sprintf("    Resources:  %s\n", strings.Join(rule.Resources, ", ")))
			}

			// Resource Names
			if len(rule.ResourceNames) > 0 {
				sb.WriteString(fmt.Sprintf("    Resource Names:  %s\n", strings.Join(rule.ResourceNames, ", ")))
			}

			// Non Resource URLs
			if len(rule.NonResourceURLs) > 0 {
				sb.WriteString(fmt.Sprintf("    Non-Resource URLs:  %s\n", strings.Join(rule.NonResourceURLs, ", ")))
			}

			// API Groups
			if len(rule.APIGroups) > 0 {
				apiGroups := make([]string, len(rule.APIGroups))
				for j, group := range rule.APIGroups {
					if group == "" {
						apiGroups[j] = `""`
					} else {
						apiGroups[j] = group
					}
				}
				sb.WriteString(fmt.Sprintf("    API Groups:  %s\n", strings.Join(apiGroups, ", ")))
			}

			// Verbs
			if len(rule.Verbs) > 0 {
				sb.WriteString(fmt.Sprintf("    Verbs:  %s\n", strings.Join(rule.Verbs, ", ")))
			}
		}
	} else {
		sb.WriteString("PolicyRule:   <none>\n")
	}

	// Events
	events, err := r.getEvents(namespace, name)
	if err == nil && len(events) > 0 {
		sb.WriteString("Events:\n")
		sb.WriteString("  Type    Reason  Age  From  Message\n")
		sb.WriteString("  ----    ------  ---  ----  -------\n")
		for _, event := range events {
			age := r.formatAge(time.UnixMilli(event.LastTimestamp))
			sb.WriteString(fmt.Sprintf("  %s  %s  %s  %s  %s\n",
				event.Type, event.Reason, age, event.Source, event.Message))
		}
	} else {
		sb.WriteString("Events:       <none>\n")
	}

	return sb.String(), nil
}

// Delete 删除 Role
func (r *roleOperator) Delete(namespace, name string) error {
	if namespace == "" || name == "" {
		return fmt.Errorf("命名空间和名称不能为空")
	}

	err := r.client.RbacV1().Roles(namespace).Delete(r.ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("删除Role失败: %v", err)
	}

	return nil
}

// GetAssociation 获取关联信息
func (r *roleOperator) GetAssociation(namespace, name string) (*types.RoleAssociation, error) {
	// 验证 Role 存在
	_, err := r.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	association := &types.RoleAssociation{
		RoleName:     name,
		Namespace:    namespace,
		RoleBindings: make([]string, 0),
		Subjects:     make([]string, 0),
	}

	// 获取所有 RoleBindings
	var roleBindings []*rbacv1.RoleBinding
	if r.useInformer && r.roleBindingLister != nil {
		roleBindings, err = r.roleBindingLister.RoleBindings(namespace).List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("获取RoleBinding列表失败: %v", err)
		}
	} else {
		rbList, err := r.client.RbacV1().RoleBindings(namespace).List(r.ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("获取RoleBinding列表失败: %v", err)
		}
		roleBindings = make([]*rbacv1.RoleBinding, len(rbList.Items))
		for i := range rbList.Items {
			roleBindings[i] = &rbList.Items[i]
		}
	}

	// 查找引用该 Role 的 RoleBinding
	subjectSet := make(map[string]bool)
	for _, rb := range roleBindings {
		if rb.RoleRef.Kind == "Role" && rb.RoleRef.Name == name {
			association.RoleBindings = append(association.RoleBindings, rb.Name)

			// 收集所有主体
			for _, subject := range rb.Subjects {
				subjectStr := fmt.Sprintf("%s/%s", subject.Kind, subject.Name)
				if subject.Namespace != "" {
					subjectStr = fmt.Sprintf("%s/%s/%s", subject.Kind, subject.Namespace, subject.Name)
				}
				if !subjectSet[subjectStr] {
					subjectSet[subjectStr] = true
					association.Subjects = append(association.Subjects, subjectStr)
				}
			}
		}
	}

	association.BindingCount = len(association.RoleBindings)

	return association, nil
}

// formatAge 格式化时间
func (r *roleOperator) formatAge(t time.Time) string {
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

// getEvents 获取事件
func (r *roleOperator) getEvents(namespace, name string) ([]types.EventInfo, error) {
	fieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Role", name)
	events, err := r.client.CoreV1().Events(namespace).List(r.ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return nil, err
	}

	result := make([]types.EventInfo, 0, len(events.Items))
	for _, event := range events.Items {
		result = append(result, types.ConvertK8sEventToEventInfo(&event))
	}

	return result, nil
}
