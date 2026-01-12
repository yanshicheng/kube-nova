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

type roleBindingOperator struct {
	BaseOperator
	client            kubernetes.Interface
	informerFactory   informers.SharedInformerFactory
	roleBindingLister rbacv1lister.RoleBindingLister
	roleLister        rbacv1lister.RoleLister
	clusterRoleLister rbacv1lister.ClusterRoleLister
}

// NewRoleBindingOperator 创建 RoleBinding 操作器（不使用 informer）
func NewRoleBindingOperator(ctx context.Context, client kubernetes.Interface) types.RoleBindingOperator {
	return &roleBindingOperator{
		BaseOperator: NewBaseOperator(ctx, false),
		client:       client,
	}
}

// NewRoleBindingOperatorWithInformer 创建 RoleBinding 操作器（使用 informer）
func NewRoleBindingOperatorWithInformer(
	ctx context.Context,
	client kubernetes.Interface,
	informerFactory informers.SharedInformerFactory,
) types.RoleBindingOperator {
	var roleBindingLister rbacv1lister.RoleBindingLister
	var roleLister rbacv1lister.RoleLister
	var clusterRoleLister rbacv1lister.ClusterRoleLister

	if informerFactory != nil {
		roleBindingLister = informerFactory.Rbac().V1().RoleBindings().Lister()
		roleLister = informerFactory.Rbac().V1().Roles().Lister()
		clusterRoleLister = informerFactory.Rbac().V1().ClusterRoles().Lister()
	}

	return &roleBindingOperator{
		BaseOperator:      NewBaseOperator(ctx, informerFactory != nil),
		client:            client,
		informerFactory:   informerFactory,
		roleBindingLister: roleBindingLister,
		roleLister:        roleLister,
		clusterRoleLister: clusterRoleLister,
	}
}

// Create 创建 RoleBinding
func (r *roleBindingOperator) Create(rolebind *rbacv1.RoleBinding) error {
	// 判断
	if rolebind == nil || rolebind.Name == "" {
		return fmt.Errorf("RoleBinding 不能为空")
	}
	// 判断是否已经存在
	if _, err := r.Get(rolebind.Namespace, rolebind.Name); err == nil {
		return fmt.Errorf("RoleBinding %s/%s 已经存在", rolebind.Namespace, rolebind.Name)
	}
	injectCommonAnnotations(rolebind)
	_, err := r.client.RbacV1().RoleBindings(rolebind.Namespace).Create(r.ctx, rolebind, metav1.CreateOptions{})
	return err
}

// Get 获取 RoleBinding
func (r *roleBindingOperator) Get(namespace, name string) (*rbacv1.RoleBinding, error) {
	if namespace == "" || name == "" {
		return nil, fmt.Errorf("命名空间和名称不能为空")
	}

	// 优先使用 informer
	if r.useInformer && r.roleBindingLister != nil {
		rb, err := r.roleBindingLister.RoleBindings(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("RoleBinding %s/%s 不存在", namespace, name)
			}
			// informer 出错时回退到 API 调用
			rb, apiErr := r.client.RbacV1().RoleBindings(namespace).Get(r.ctx, name, metav1.GetOptions{})
			if apiErr != nil {
				return nil, fmt.Errorf("获取RoleBinding失败: %v", apiErr)
			}
			r.injectTypeMeta(rb)
			return rb, nil
		}
		// 返回副本以避免修改缓存
		result := rb.DeepCopy()
		r.injectTypeMeta(result)
		return result, nil
	}

	// 使用 API 调用
	rb, err := r.client.RbacV1().RoleBindings(namespace).Get(r.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("RoleBinding %s/%s 不存在", namespace, name)
		}
		return nil, fmt.Errorf("获取RoleBinding失败: %v", err)
	}

	r.injectTypeMeta(rb)
	return rb, nil
}

// injectTypeMeta 注入 TypeMeta
func (r *roleBindingOperator) injectTypeMeta(rb *rbacv1.RoleBinding) {
	rb.TypeMeta = metav1.TypeMeta{
		Kind:       "RoleBinding",
		APIVersion: "rbac.authorization.k8s.io/v1",
	}
}

// List 获取 RoleBinding 列表
func (r *roleBindingOperator) List(namespace string, search string, labelSelector string) (*types.ListRoleBindingResponse, error) {
	var selector labels.Selector = labels.Everything()
	if labelSelector != "" {
		parsedSelector, err := labels.Parse(labelSelector)
		if err != nil {
			return nil, fmt.Errorf("解析标签选择器失败: %v", err)
		}
		selector = parsedSelector
	}

	var roleBindings []*rbacv1.RoleBinding
	var err error

	// 优先使用 informer
	if r.useInformer && r.roleBindingLister != nil {
		roleBindings, err = r.roleBindingLister.RoleBindings(namespace).List(selector)
		if err != nil {
			return nil, fmt.Errorf("获取RoleBinding列表失败: %v", err)
		}
	} else {
		// 使用 API 调用
		listOpts := metav1.ListOptions{LabelSelector: selector.String()}
		rbList, err := r.client.RbacV1().RoleBindings(namespace).List(r.ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("获取RoleBinding列表失败: %v", err)
		}
		roleBindings = make([]*rbacv1.RoleBinding, len(rbList.Items))
		for i := range rbList.Items {
			roleBindings[i] = &rbList.Items[i]
		}
	}

	// 搜索过滤
	if search != "" {
		filtered := make([]*rbacv1.RoleBinding, 0)
		searchLower := strings.ToLower(search)
		for _, rb := range roleBindings {
			if strings.Contains(strings.ToLower(rb.Name), searchLower) ||
				strings.Contains(strings.ToLower(rb.RoleRef.Name), searchLower) {
				filtered = append(filtered, rb)
			}
		}
		roleBindings = filtered
	}

	// 转换为响应格式
	items := make([]types.RoleBindingInfoo, len(roleBindings))
	for i, rb := range roleBindings {
		roleStr := fmt.Sprintf("%s/%s", rb.RoleRef.Kind, rb.RoleRef.Name)
		items[i] = types.RoleBindingInfoo{
			Name:              rb.Name,
			Namespace:         rb.Namespace,
			Role:              roleStr,
			Age:               r.formatAge(rb.CreationTimestamp.Time),
			Labels:            rb.Labels,
			Annotations:       rb.Annotations,
			CreationTimestamp: rb.CreationTimestamp.UnixMilli(),
		}
	}

	return &types.ListRoleBindingResponse{
		Total: len(items),
		Items: items,
	}, nil
}

// GetYaml 获取 RoleBinding 的 YAML
func (r *roleBindingOperator) GetYaml(namespace, name string) (string, error) {
	rb, err := r.Get(namespace, name)
	if err != nil {
		return "", err
	}

	// 清理 ManagedFields
	rb.ManagedFields = nil

	yamlBytes, err := yaml.Marshal(rb)
	if err != nil {
		return "", fmt.Errorf("转换为YAML失败: %v", err)
	}

	return string(yamlBytes), nil
}

// Update 更新 RoleBinding
func (r *roleBindingOperator) Update(namespace, name string, rb *rbacv1.RoleBinding) error {

	// 验证名称和命名空间
	if rb.Name != name {
		return fmt.Errorf("YAML中的名称(%s)与参数名称(%s)不匹配", rb.Name, name)
	}
	if rb.Namespace != "" && rb.Namespace != namespace {
		return fmt.Errorf("YAML中的命名空间(%s)与参数命名空间(%s)不匹配", rb.Namespace, namespace)
	}
	rb.Namespace = namespace

	// 获取现有的 RoleBinding 以保留 ResourceVersion
	existing, err := r.Get(namespace, name)
	if err != nil {
		return fmt.Errorf("获取现有RoleBinding失败: %v", err)
	}

	rb.ResourceVersion = existing.ResourceVersion

	// 更新
	_, err = r.client.RbacV1().RoleBindings(namespace).Update(r.ctx, rb, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("更新RoleBinding失败: %v", err)
	}

	return nil
}

// Describe 获取 RoleBinding 详情
func (r *roleBindingOperator) Describe(namespace, name string) (string, error) {
	rb, err := r.Get(namespace, name)
	if err != nil {
		return "", err
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Name:         %s\n", rb.Name))
	sb.WriteString(fmt.Sprintf("Namespace:    %s\n", rb.Namespace))

	// Labels
	if len(rb.Labels) > 0 {
		sb.WriteString("Labels:       ")
		labels := make([]string, 0, len(rb.Labels))
		for k, v := range rb.Labels {
			labels = append(labels, fmt.Sprintf("%s=%s", k, v))
		}
		sb.WriteString(strings.Join(labels, "\n              ") + "\n")
	} else {
		sb.WriteString("Labels:       <none>\n")
	}

	// Annotations
	if len(rb.Annotations) > 0 {
		sb.WriteString("Annotations:  ")
		annotations := make([]string, 0, len(rb.Annotations))
		for k, v := range rb.Annotations {
			if len(v) > 50 {
				v = v[:50] + "..."
			}
			annotations = append(annotations, fmt.Sprintf("%s=%s", k, v))
		}
		sb.WriteString(strings.Join(annotations, "\n              ") + "\n")
	} else {
		sb.WriteString("Annotations:  <none>\n")
	}

	// Role
	sb.WriteString("Role:\n")
	sb.WriteString(fmt.Sprintf("  Kind:       %s\n", rb.RoleRef.Kind))
	sb.WriteString(fmt.Sprintf("  Name:       %s\n", rb.RoleRef.Name))
	sb.WriteString(fmt.Sprintf("  APIGroup:   %s\n", rb.RoleRef.APIGroup))

	// Subjects
	if len(rb.Subjects) > 0 {
		sb.WriteString("Subjects:\n")
		sb.WriteString("  Kind            Name               Namespace\n")
		sb.WriteString("  ----            ----               ---------\n")
		for _, subject := range rb.Subjects {
			ns := subject.Namespace
			if ns == "" {
				ns = "-"
			}
			sb.WriteString(fmt.Sprintf("  %-15s %-18s %s\n", subject.Kind, subject.Name, ns))
		}
	} else {
		sb.WriteString("Subjects:     <none>\n")
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

// Delete 删除 RoleBinding
func (r *roleBindingOperator) Delete(namespace, name string) error {
	if namespace == "" || name == "" {
		return fmt.Errorf("命名空间和名称不能为空")
	}

	err := r.client.RbacV1().RoleBindings(namespace).Delete(r.ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("删除RoleBinding失败: %v", err)
	}

	return nil
}

// GetAssociation 获取关联信息
func (r *roleBindingOperator) GetAssociation(namespace, name string) (*types.RoleBindingAssociation, error) {
	// 验证 RoleBinding 存在
	rb, err := r.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	association := &types.RoleBindingAssociation{
		RoleBindingName: name,
		Namespace:       namespace,
		RoleName:        rb.RoleRef.Name,
		RoleKind:        rb.RoleRef.Kind,
		Subjects:        make([]types.SubjectInfo, 0),
	}

	// 转换 Subjects
	for _, subject := range rb.Subjects {
		subjectInfo := types.SubjectInfo{
			Kind:      subject.Kind,
			Name:      subject.Name,
			Namespace: subject.Namespace,
			APIGroup:  subject.APIGroup,
		}
		association.Subjects = append(association.Subjects, subjectInfo)
	}

	association.SubjectCount = len(association.Subjects)

	return association, nil
}

// formatAge 格式化时间
func (r *roleBindingOperator) formatAge(t time.Time) string {
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
func (r *roleBindingOperator) getEvents(namespace, name string) ([]types.EventInfo, error) {
	fieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=RoleBinding", name)
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
