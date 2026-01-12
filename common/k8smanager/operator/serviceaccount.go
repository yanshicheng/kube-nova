package operator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1lister "k8s.io/client-go/listers/core/v1"
	"sigs.k8s.io/yaml"
)

type serviceAccountOperator struct {
	BaseOperator
	client          kubernetes.Interface
	informerFactory informers.SharedInformerFactory
	saLister        corev1lister.ServiceAccountLister
	podLister       corev1lister.PodLister
	secretLister    corev1lister.SecretLister
}

// NewServiceAccountOperator 创建 ServiceAccount 操作器（不使用 informer）
func NewServiceAccountOperator(ctx context.Context, client kubernetes.Interface) types.ServiceAccountOperator {
	return &serviceAccountOperator{
		BaseOperator: NewBaseOperator(ctx, false),
		client:       client,
	}
}

// NewServiceAccountOperatorWithInformer 创建 ServiceAccount 操作器（使用 informer）
func NewServiceAccountOperatorWithInformer(
	ctx context.Context,
	client kubernetes.Interface,
	informerFactory informers.SharedInformerFactory,
) types.ServiceAccountOperator {
	var saLister corev1lister.ServiceAccountLister
	var podLister corev1lister.PodLister
	var secretLister corev1lister.SecretLister

	if informerFactory != nil {
		saLister = informerFactory.Core().V1().ServiceAccounts().Lister()
		podLister = informerFactory.Core().V1().Pods().Lister()
		secretLister = informerFactory.Core().V1().Secrets().Lister()
	}

	return &serviceAccountOperator{
		BaseOperator:    NewBaseOperator(ctx, informerFactory != nil),
		client:          client,
		informerFactory: informerFactory,
		saLister:        saLister,
		podLister:       podLister,
		secretLister:    secretLister,
	}
}

// Get 获取 ServiceAccount
func (s *serviceAccountOperator) Get(namespace, name string) (*corev1.ServiceAccount, error) {
	if namespace == "" || name == "" {
		return nil, fmt.Errorf("命名空间和名称不能为空")
	}

	// 优先使用 informer
	if s.useInformer && s.saLister != nil {
		sa, err := s.saLister.ServiceAccounts(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("ServiceAccount %s/%s 不存在", namespace, name)
			}
			// informer 出错时回退到 API 调用
			sa, apiErr := s.client.CoreV1().ServiceAccounts(namespace).Get(s.ctx, name, metav1.GetOptions{})
			if apiErr != nil {
				return nil, fmt.Errorf("获取ServiceAccount失败: %v", apiErr)
			}
			s.injectTypeMeta(sa)
			return sa, nil
		}
		// 返回副本以避免修改缓存
		result := sa.DeepCopy()
		s.injectTypeMeta(result)
		return result, nil
	}

	// 使用 API 调用
	sa, err := s.client.CoreV1().ServiceAccounts(namespace).Get(s.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("ServiceAccount %s/%s 不存在", namespace, name)
		}
		return nil, fmt.Errorf("获取ServiceAccount失败: %v", err)
	}

	s.injectTypeMeta(sa)
	return sa, nil
}

// injectTypeMeta 注入 TypeMeta
func (s *serviceAccountOperator) injectTypeMeta(sa *corev1.ServiceAccount) {
	sa.TypeMeta = metav1.TypeMeta{
		Kind:       "ServiceAccount",
		APIVersion: "v1",
	}
}

// List 获取 ServiceAccount 列表
func (s *serviceAccountOperator) List(namespace string, search string, labelSelector string) (*types.ListServiceAccountResponse, error) {
	var selector labels.Selector = labels.Everything()
	if labelSelector != "" {
		parsedSelector, err := labels.Parse(labelSelector)
		if err != nil {
			return nil, fmt.Errorf("解析标签选择器失败: %v", err)
		}
		selector = parsedSelector
	}

	var serviceAccounts []*corev1.ServiceAccount
	var err error

	// 优先使用 informer
	if s.useInformer && s.saLister != nil {
		serviceAccounts, err = s.saLister.ServiceAccounts(namespace).List(selector)
		if err != nil {
			return nil, fmt.Errorf("获取ServiceAccount列表失败: %v", err)
		}
	} else {
		// 使用 API 调用
		listOpts := metav1.ListOptions{LabelSelector: selector.String()}
		saList, err := s.client.CoreV1().ServiceAccounts(namespace).List(s.ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("获取ServiceAccount列表失败: %v", err)
		}
		serviceAccounts = make([]*corev1.ServiceAccount, len(saList.Items))
		for i := range saList.Items {
			serviceAccounts[i] = &saList.Items[i]
		}
	}

	// 搜索过滤
	if search != "" {
		filtered := make([]*corev1.ServiceAccount, 0)
		searchLower := strings.ToLower(search)
		for _, sa := range serviceAccounts {
			if strings.Contains(strings.ToLower(sa.Name), searchLower) {
				filtered = append(filtered, sa)
			}
		}
		serviceAccounts = filtered
	}

	// 转换为响应格式
	items := make([]types.ServiceAccountInfo, len(serviceAccounts))
	for i, sa := range serviceAccounts {
		items[i] = types.ServiceAccountInfo{
			Name:              sa.Name,
			Namespace:         sa.Namespace,
			Secrets:           len(sa.Secrets),
			Age:               s.formatAge(sa.CreationTimestamp.Time),
			Labels:            sa.Labels,
			Annotations:       sa.Annotations,
			CreationTimestamp: sa.CreationTimestamp.UnixMilli(),
		}
	}

	return &types.ListServiceAccountResponse{
		Total: len(items),
		Items: items,
	}, nil
}

// GetYaml 获取 ServiceAccount 的 YAML
func (s *serviceAccountOperator) GetYaml(namespace, name string) (string, error) {
	sa, err := s.Get(namespace, name)
	if err != nil {
		return "", err
	}

	// 清理 ManagedFields
	sa.ManagedFields = nil

	yamlBytes, err := yaml.Marshal(sa)
	if err != nil {
		return "", fmt.Errorf("转换为YAML失败: %v", err)
	}

	return string(yamlBytes), nil
}

// Create
func (s *serviceAccountOperator) Create(sa *corev1.ServiceAccount) error {
	// 先判断是否已经存在
	if _, err := s.Get(sa.Namespace, sa.Name); err == nil {
		return fmt.Errorf("ServiceAccount %s/%s 已经存在", sa.Namespace, sa.Name)
	}
	injectCommonAnnotations(sa)
	_, err := s.client.CoreV1().ServiceAccounts(sa.Namespace).Create(s.ctx, sa, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("创建ServiceAccount失败: %v", err)
	}
	return nil
}

// Update 更新 ServiceAccount
func (s *serviceAccountOperator) Update(namespace, name string, sa *corev1.ServiceAccount) error {
	// 验证名称和命名空间
	if sa.Name != name {
		return fmt.Errorf("YAML中的名称(%s)与参数名称(%s)不匹配", sa.Name, name)
	}
	if sa.Namespace != "" && sa.Namespace != namespace {
		return fmt.Errorf("YAML中的命名空间(%s)与参数命名空间(%s)不匹配", sa.Namespace, namespace)
	}
	sa.Namespace = namespace

	// 获取现有的 ServiceAccount 以保留 ResourceVersion
	existing, err := s.Get(namespace, name)
	if err != nil {
		return fmt.Errorf("获取现有ServiceAccount失败: %v", err)
	}

	sa.ResourceVersion = existing.ResourceVersion

	// 更新
	_, err = s.client.CoreV1().ServiceAccounts(namespace).Update(s.ctx, sa, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("更新ServiceAccount失败: %v", err)
	}

	return nil
}

// Describe 获取 ServiceAccount 详情
func (s *serviceAccountOperator) Describe(namespace, name string) (string, error) {
	sa, err := s.Get(namespace, name)
	if err != nil {
		return "", err
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Name:                %s\n", sa.Name))
	sb.WriteString(fmt.Sprintf("Namespace:           %s\n", sa.Namespace))

	// Labels
	if len(sa.Labels) > 0 {
		sb.WriteString("Labels:              ")
		labels := make([]string, 0, len(sa.Labels))
		for k, v := range sa.Labels {
			labels = append(labels, fmt.Sprintf("%s=%s", k, v))
		}
		sb.WriteString(strings.Join(labels, "\n                     ") + "\n")
	} else {
		sb.WriteString("Labels:              <none>\n")
	}

	// Annotations
	if len(sa.Annotations) > 0 {
		sb.WriteString("Annotations:         ")
		annotations := make([]string, 0, len(sa.Annotations))
		for k, v := range sa.Annotations {
			if len(v) > 50 {
				v = v[:50] + "..."
			}
			annotations = append(annotations, fmt.Sprintf("%s=%s", k, v))
		}
		sb.WriteString(strings.Join(annotations, "\n                     ") + "\n")
	} else {
		sb.WriteString("Annotations:         <none>\n")
	}

	// Image pull secrets
	if len(sa.ImagePullSecrets) > 0 {
		sb.WriteString("Image pull secrets:  ")
		secrets := make([]string, len(sa.ImagePullSecrets))
		for i, secret := range sa.ImagePullSecrets {
			secrets[i] = secret.Name
		}
		sb.WriteString(strings.Join(secrets, "\n                     ") + "\n")
	} else {
		sb.WriteString("Image pull secrets:  <none>\n")
	}

	// Mountable secrets
	if len(sa.Secrets) > 0 {
		sb.WriteString("Mountable secrets:   ")
		secrets := make([]string, len(sa.Secrets))
		for i, secret := range sa.Secrets {
			secrets[i] = secret.Name
		}
		sb.WriteString(strings.Join(secrets, "\n                     ") + "\n")
	} else {
		sb.WriteString("Mountable secrets:   <none>\n")
	}

	// Tokens
	sb.WriteString(fmt.Sprintf("Tokens:              %d\n", len(sa.Secrets)))

	// Automount Service Account Token
	if sa.AutomountServiceAccountToken != nil {
		sb.WriteString(fmt.Sprintf("Automount token:     %v\n", *sa.AutomountServiceAccountToken))
	} else {
		sb.WriteString("Automount token:     <unset>\n")
	}

	// Events
	events, err := s.getEvents(namespace, name)
	if err == nil && len(events) > 0 {
		sb.WriteString("Events:\n")
		sb.WriteString("  Type    Reason  Age  From  Message\n")
		sb.WriteString("  ----    ------  ---  ----  -------\n")
		for _, event := range events {
			age := s.formatAge(time.UnixMilli(event.LastTimestamp))
			sb.WriteString(fmt.Sprintf("  %s  %s  %s  %s  %s\n",
				event.Type, event.Reason, age, event.Source, event.Message))
		}
	} else {
		sb.WriteString("Events:              <none>\n")
	}

	return sb.String(), nil
}

// Delete 删除 ServiceAccount
func (s *serviceAccountOperator) Delete(namespace, name string) error {
	if namespace == "" || name == "" {
		return fmt.Errorf("命名空间和名称不能为空")
	}

	err := s.client.CoreV1().ServiceAccounts(namespace).Delete(s.ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("删除ServiceAccount失败: %v", err)
	}

	return nil
}

// GetAssociation 获取关联信息
func (s *serviceAccountOperator) GetAssociation(namespace, name string) (*types.ServiceAccountAssociation, error) {
	// 验证 ServiceAccount 存在
	sa, err := s.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	association := &types.ServiceAccountAssociation{
		ServiceAccountName: name,
		Namespace:          namespace,
		Pods:               make([]string, 0),
		Secrets:            make([]string, 0),
		ImagePullSecrets:   make([]string, 0),
	}

	// 获取关联的 Secrets
	for _, secret := range sa.Secrets {
		association.Secrets = append(association.Secrets, secret.Name)
	}

	// 获取 ImagePullSecrets
	for _, secret := range sa.ImagePullSecrets {
		association.ImagePullSecrets = append(association.ImagePullSecrets, secret.Name)
	}

	// 获取使用该 ServiceAccount 的 Pods
	var pods []*corev1.Pod
	if s.useInformer && s.podLister != nil {
		pods, err = s.podLister.Pods(namespace).List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("获取Pod列表失败: %v", err)
		}
	} else {
		podList, err := s.client.CoreV1().Pods(namespace).List(s.ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("获取Pod列表失败: %v", err)
		}
		pods = make([]*corev1.Pod, len(podList.Items))
		for i := range podList.Items {
			pods[i] = &podList.Items[i]
		}
	}

	for _, pod := range pods {
		if pod.Spec.ServiceAccountName == name {
			association.Pods = append(association.Pods, pod.Name)
		}
	}
	association.PodCount = len(association.Pods)

	return association, nil
}

// formatAge 格式化时间
func (s *serviceAccountOperator) formatAge(t time.Time) string {
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
func (s *serviceAccountOperator) getEvents(namespace, name string) ([]types.EventInfo, error) {
	fieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=ServiceAccount", name)
	events, err := s.client.CoreV1().Events(namespace).List(s.ctx, metav1.ListOptions{
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
