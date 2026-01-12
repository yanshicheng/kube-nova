package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1lister "k8s.io/client-go/listers/core/v1"
	"sigs.k8s.io/yaml"
)

type secretOperator struct {
	BaseOperator
	client          kubernetes.Interface
	informerFactory informers.SharedInformerFactory
	secretLister    corev1lister.SecretLister
}

func NewSecretOperator(ctx context.Context, client kubernetes.Interface) types.SecretOperator {
	return &secretOperator{
		BaseOperator: NewBaseOperator(ctx, false),
		client:       client,
	}
}

func NewSecretOperatorWithInformer(
	ctx context.Context,
	client kubernetes.Interface,
	informerFactory informers.SharedInformerFactory,
) types.SecretOperator {
	var secretLister corev1lister.SecretLister

	if informerFactory != nil {
		secretLister = informerFactory.Core().V1().Secrets().Lister()
	}

	return &secretOperator{
		BaseOperator:    NewBaseOperator(ctx, informerFactory != nil),
		client:          client,
		informerFactory: informerFactory,
		secretLister:    secretLister,
	}
}

func (s *secretOperator) Create(secret *corev1.Secret) (*corev1.Secret, error) {
	if secret == nil || secret.Name == "" || secret.Namespace == "" {
		return nil, fmt.Errorf("Secret对象、名称和命名空间不能为空")
	}
	injectCommonAnnotations(secret)
	created, err := s.client.CoreV1().Secrets(secret.Namespace).Create(s.ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("创建Secret失败: %v", err)
	}

	return created, nil
}

func (s *secretOperator) Get(namespace, name string) (*corev1.Secret, error) {
	if namespace == "" || name == "" {
		return nil, fmt.Errorf("命名空间和名称不能为空")
	}

	// 优先使用 informer
	if s.useInformer && s.secretLister != nil {
		secret, err := s.secretLister.Secrets(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("Secret %s/%s 不存在", namespace, name)
			}
			// informer 出错时回退到 API 调用
			secret, apiErr := s.client.CoreV1().Secrets(namespace).Get(s.ctx, name, metav1.GetOptions{})
			if apiErr != nil {
				return nil, fmt.Errorf("获取Secret失败: %v", apiErr)
			}
			secret.TypeMeta = metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			}
			return secret, nil
		}
		secret.TypeMeta = metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		}
		return secret, nil
	}

	// 使用 API 调用
	secret, err := s.client.CoreV1().Secrets(namespace).Get(s.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("Secret %s/%s 不存在", namespace, name)
		}
		return nil, fmt.Errorf("获取Secret失败: %v", err)
	}

	secret.TypeMeta = metav1.TypeMeta{
		Kind:       "Secret",
		APIVersion: "v1",
	}

	return secret, nil
}

func (s *secretOperator) Update(secret *corev1.Secret) (*corev1.Secret, error) {
	if secret == nil || secret.Name == "" || secret.Namespace == "" {
		return nil, fmt.Errorf("Secret对象、名称和命名空间不能为空")
	}

	updated, err := s.client.CoreV1().Secrets(secret.Namespace).Update(s.ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("更新Secret失败: %v", err)
	}

	return updated, nil
}

func (s *secretOperator) Delete(namespace, name string) error {
	if namespace == "" || name == "" {
		return fmt.Errorf("命名空间和名称不能为空")
	}

	err := s.client.CoreV1().Secrets(namespace).Delete(s.ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("删除Secret失败: %v", err)
	}

	return nil
}

// List 获取 Secret 列表，支持 search、labelSelector 和 secretType 过滤
func (s *secretOperator) List(namespace string, search string, labelSelector string, secretType string) (*types.ListSecretResponse, error) {
	var selector labels.Selector = labels.Everything()
	if labelSelector != "" {
		parsedSelector, err := labels.Parse(labelSelector)
		if err != nil {
			return nil, fmt.Errorf("解析标签选择器失败: %v", err)
		}
		selector = parsedSelector
	}

	var secrets []*corev1.Secret
	var err error

	// 优先使用 informer
	if s.useInformer && s.secretLister != nil {
		secrets, err = s.secretLister.Secrets(namespace).List(selector)
		if err != nil {
			return nil, fmt.Errorf("获取Secret列表失败: %v", err)
		}
	} else {
		// 使用 API 调用
		listOpts := metav1.ListOptions{LabelSelector: selector.String()}
		secretList, err := s.client.CoreV1().Secrets(namespace).List(s.ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("获取Secret列表失败: %v", err)
		}
		secrets = make([]*corev1.Secret, len(secretList.Items))
		for i := range secretList.Items {
			secrets[i] = &secretList.Items[i]
		}
	}

	// 类型过滤
	if secretType != "" {
		filtered := make([]*corev1.Secret, 0)
		for _, secret := range secrets {
			if string(secret.Type) == secretType {
				filtered = append(filtered, secret)
			}
		}
		secrets = filtered
	}

	// 搜索过滤
	if search != "" {
		filtered := make([]*corev1.Secret, 0)
		searchLower := strings.ToLower(search)
		for _, secret := range secrets {
			if strings.Contains(strings.ToLower(secret.Name), searchLower) {
				filtered = append(filtered, secret)
			}
		}
		secrets = filtered
	}

	// 转换为响应格式
	items := make([]types.SecretInfo, len(secrets))
	for i, secret := range secrets {
		items[i] = types.SecretInfo{
			Name:              secret.Name,
			Namespace:         secret.Namespace,
			Type:              string(secret.Type),
			Labels:            secret.Labels,
			Annotations:       secret.Annotations,
			DataCount:         len(secret.Data),
			Age:               time.Since(secret.CreationTimestamp.Time).String(),
			CreationTimestamp: secret.CreationTimestamp.UnixMilli(),
		}
	}

	return &types.ListSecretResponse{
		Total: len(items),
		Items: items,
	}, nil
}

func (s *secretOperator) GetData(namespace, name string) (*types.GetSecretDataResponse, error) {
	secret, err := s.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	data := make([]types.SecretDataItem, 0, len(secret.Data))
	for key, value := range secret.Data {
		data = append(data, types.SecretDataItem{
			Key:   key,
			Value: string(value),
		})
	}

	// 按 key 排序，保证顺序一致
	sort.Slice(data, func(i, j int) bool {
		return data[i].Key < data[j].Key
	})

	return &types.GetSecretDataResponse{
		Name:      name,
		Namespace: namespace,
		Type:      string(secret.Type),
		Data:      data,
	}, nil
}

func (s *secretOperator) Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return s.client.CoreV1().Secrets(namespace).Watch(s.ctx, opts)
}

func (s *secretOperator) UpdateLabels(namespace, name string, labels map[string]string) error {
	secret, err := s.Get(namespace, name)
	if err != nil {
		return err
	}

	if secret.Labels == nil {
		secret.Labels = make(map[string]string)
	}
	for k, v := range labels {
		secret.Labels[k] = v
	}

	_, err = s.Update(secret)
	return err
}

func (s *secretOperator) UpdateAnnotations(namespace, name string, annotations map[string]string) error {
	secret, err := s.Get(namespace, name)
	if err != nil {
		return err
	}

	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		secret.Annotations[k] = v
	}

	_, err = s.Update(secret)
	return err
}

func (s *secretOperator) GetYaml(namespace, name string) (string, error) {
	secret, err := s.Get(namespace, name)
	if err != nil {
		return "", err
	}

	secret.TypeMeta = metav1.TypeMeta{
		Kind:       "Secret",
		APIVersion: "v1",
	}

	secret.ManagedFields = nil
	yamlBytes, err := yaml.Marshal(secret)
	if err != nil {
		return "", fmt.Errorf("转换为YAML失败: %v", err)
	}

	return string(yamlBytes), nil
}

func (s *secretOperator) UpdateKey(namespace, name, key, value string) error {
	secret, err := s.Get(namespace, name)
	if err != nil {
		return err
	}

	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	secret.Data[key] = []byte(value)

	_, err = s.Update(secret)
	return err
}

func (s *secretOperator) DeleteKey(namespace, name, key string) error {
	secret, err := s.Get(namespace, name)
	if err != nil {
		return err
	}

	if secret.Data != nil {
		delete(secret.Data, key)
	}

	_, err = s.Update(secret)
	return err
}

func (s *secretOperator) UpdateData(namespace, name string, data map[string]string) error {
	secret, err := s.Get(namespace, name)
	if err != nil {
		return err
	}

	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}

	for k, v := range data {
		secret.Data[k] = []byte(v)
	}

	_, err = s.Update(secret)
	return err
}

func (s *secretOperator) CreateTLSSecret(namespace, name string, cert, key []byte, labels, annotations map[string]string) (*corev1.Secret, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			corev1.TLSCertKey:       cert,
			corev1.TLSPrivateKeyKey: key,
		},
	}

	return s.Create(secret)
}

func (s *secretOperator) CreateDockerRegistrySecret(namespace, name, server, username, password, email string, labels, annotations map[string]string) (*corev1.Secret, error) {
	dockerConfig := map[string]interface{}{
		"auths": map[string]interface{}{
			server: map[string]string{
				"username": username,
				"password": password,
				"email":    email,
			},
		},
	}

	dockerConfigJSON, err := json.Marshal(dockerConfig)
	if err != nil {
		return nil, fmt.Errorf("生成docker配置失败: %v", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			corev1.DockerConfigJsonKey: dockerConfigJSON,
		},
	}

	return s.Create(secret)
}

func (s *secretOperator) CreateBasicAuthSecret(namespace, name, username, password string, labels, annotations map[string]string) (*corev1.Secret, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Type: corev1.SecretTypeBasicAuth,
		Data: map[string][]byte{
			corev1.BasicAuthUsernameKey: []byte(username),
			corev1.BasicAuthPasswordKey: []byte(password),
		},
	}

	return s.Create(secret)
}

// GetUsage 获取 Secret 的使用情况
func (s *secretOperator) GetUsage(namespace, name string) (*types.SecretUsageResponse, error) {
	// 首先获取 Secret 本身，使用 informer（如果可用）
	secret, err := s.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	response := &types.SecretUsageResponse{
		SecretName:      name,
		SecretNamespace: namespace,
		SecretType:      string(secret.Type),
		UsedBy:          make([]types.SecretUsageReference, 0),
	}

	// 用于存储已经扫描过的工作负载创建的 Pod
	managedPods := make(map[string]bool)

	// 1. 扫描 Deployment
	deployments, err := s.client.AppsV1().Deployments(namespace).List(s.ctx, metav1.ListOptions{})
	if err == nil {
		for _, deploy := range deployments.Items {
			if ref := s.checkPodTemplateForSecret(name, &deploy.Spec.Template); ref != nil {
				ref.ResourceType = "Deployment"
				ref.ResourceName = deploy.Name
				ref.Namespace = deploy.Namespace
				response.UsedBy = append(response.UsedBy, *ref)

				s.markManagedPods(namespace, deploy.Spec.Selector, managedPods)
			}
		}
	}

	// 2. 扫描 StatefulSet
	statefulSets, err := s.client.AppsV1().StatefulSets(namespace).List(s.ctx, metav1.ListOptions{})
	if err == nil {
		for _, sts := range statefulSets.Items {
			if ref := s.checkPodTemplateForSecret(name, &sts.Spec.Template); ref != nil {
				ref.ResourceType = "StatefulSet"
				ref.ResourceName = sts.Name
				ref.Namespace = sts.Namespace
				response.UsedBy = append(response.UsedBy, *ref)

				s.markManagedPods(namespace, sts.Spec.Selector, managedPods)
			}
		}
	}

	// 3. 扫描 DaemonSet
	daemonSets, err := s.client.AppsV1().DaemonSets(namespace).List(s.ctx, metav1.ListOptions{})
	if err == nil {
		for _, ds := range daemonSets.Items {
			if ref := s.checkPodTemplateForSecret(name, &ds.Spec.Template); ref != nil {
				ref.ResourceType = "DaemonSet"
				ref.ResourceName = ds.Name
				ref.Namespace = ds.Namespace
				response.UsedBy = append(response.UsedBy, *ref)

				s.markManagedPods(namespace, ds.Spec.Selector, managedPods)
			}
		}
	}

	// 4. 扫描 Job
	jobs, err := s.client.BatchV1().Jobs(namespace).List(s.ctx, metav1.ListOptions{})
	if err == nil {
		for _, job := range jobs.Items {
			if ref := s.checkPodTemplateForSecret(name, &job.Spec.Template); ref != nil {
				ref.ResourceType = "Job"
				ref.ResourceName = job.Name
				ref.Namespace = job.Namespace
				response.UsedBy = append(response.UsedBy, *ref)

				s.markManagedPods(namespace, job.Spec.Selector, managedPods)
			}
		}
	}

	// 5. 扫描 CronJob
	cronJobs, err := s.client.BatchV1().CronJobs(namespace).List(s.ctx, metav1.ListOptions{})
	if err == nil {
		for _, cronJob := range cronJobs.Items {
			if ref := s.checkPodTemplateForSecret(name, &cronJob.Spec.JobTemplate.Spec.Template); ref != nil {
				ref.ResourceType = "CronJob"
				ref.ResourceName = cronJob.Name
				ref.Namespace = cronJob.Namespace
				response.UsedBy = append(response.UsedBy, *ref)
			}
		}
	}

	// 6. 扫描独立的 Pod（不由上述工作负载管理的 Pod）
	pods, err := s.client.CoreV1().Pods(namespace).List(s.ctx, metav1.ListOptions{})
	if err == nil {
		for _, pod := range pods.Items {
			podKey := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)

			// 跳过已被工作负载管理的 Pod
			if managedPods[podKey] {
				continue
			}

			if ref := s.checkPodForSecret(name, &pod); ref != nil {
				ref.ResourceType = "Pod"
				ref.ResourceName = pod.Name
				ref.Namespace = pod.Namespace
				response.UsedBy = append(response.UsedBy, *ref)
			}
		}
	}

	// 7. 扫描 ServiceAccounts
	saList, err := s.client.CoreV1().ServiceAccounts(namespace).List(s.ctx, metav1.ListOptions{})
	if err == nil {
		for _, sa := range saList.Items {
			usageTypes := make([]string, 0)

			// 检查 Secrets
			for _, secret := range sa.Secrets {
				if secret.Name == name {
					usageTypes = append(usageTypes, "serviceAccountToken")
					break
				}
			}

			// 检查 ImagePullSecrets
			for _, ips := range sa.ImagePullSecrets {
				if ips.Name == name {
					usageTypes = append(usageTypes, "imagePullSecret")
					break
				}
			}

			if len(usageTypes) > 0 {
				response.UsedBy = append(response.UsedBy, types.SecretUsageReference{
					ResourceType: "ServiceAccount",
					ResourceName: sa.Name,
					Namespace:    sa.Namespace,
					UsageType:    usageTypes,
				})
			}
		}
	}

	response.TotalUsageCount = len(response.UsedBy)
	response.CanDelete = response.TotalUsageCount == 0
	if !response.CanDelete {
		response.DeleteWarning = fmt.Sprintf("Secret被%d个资源引用，删除可能导致这些资源异常", response.TotalUsageCount)
	}

	return response, nil
}

// checkPodTemplateForSecret 检查 Pod 模板是否引用了指定的 Secret
func (s *secretOperator) checkPodTemplateForSecret(secretName string, template *corev1.PodTemplateSpec) *types.SecretUsageReference {
	usageTypes := make([]string, 0)
	usedKeys := make([]string, 0)
	containerNames := make([]string, 0)

	// 检查 Volume
	for _, vol := range template.Spec.Volumes {
		if vol.Secret != nil && vol.Secret.SecretName == secretName {
			usageTypes = append(usageTypes, "volume")
			for _, item := range vol.Secret.Items {
				usedKeys = append(usedKeys, item.Key)
			}
		}
	}

	// 检查 ImagePullSecrets
	for _, ips := range template.Spec.ImagePullSecrets {
		if ips.Name == secretName {
			usageTypes = append(usageTypes, "imagePullSecret")
		}
	}

	// 检查所有容器（包括 InitContainers 和普通 Containers）
	allContainers := append(template.Spec.InitContainers, template.Spec.Containers...)
	for _, container := range allContainers {
		found := false

		// 检查环境变量
		for _, env := range container.Env {
			if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil &&
				env.ValueFrom.SecretKeyRef.Name == secretName {
				if !found {
					usageTypes = append(usageTypes, "env")
					containerNames = append(containerNames, container.Name)
					found = true
				}
				usedKeys = append(usedKeys, env.ValueFrom.SecretKeyRef.Key)
			}
		}

		// 检查 EnvFrom
		for _, envFrom := range container.EnvFrom {
			if envFrom.SecretRef != nil && envFrom.SecretRef.Name == secretName {
				if !found {
					usageTypes = append(usageTypes, "envFrom")
					containerNames = append(containerNames, container.Name)
					found = true
				}
			}
		}
	}

	if len(usageTypes) > 0 {
		return &types.SecretUsageReference{
			UsageType:      usageTypes,
			UsedKeys:       usedKeys,
			ContainerNames: containerNames,
		}
	}

	return nil
}

// checkPodForSecret 检查 Pod 是否引用了指定的 Secret
func (s *secretOperator) checkPodForSecret(secretName string, pod *corev1.Pod) *types.SecretUsageReference {
	usageTypes := make([]string, 0)
	usedKeys := make([]string, 0)
	containerNames := make([]string, 0)

	// 检查 Volume
	for _, vol := range pod.Spec.Volumes {
		if vol.Secret != nil && vol.Secret.SecretName == secretName {
			usageTypes = append(usageTypes, "volume")
			for _, item := range vol.Secret.Items {
				usedKeys = append(usedKeys, item.Key)
			}
		}
	}

	// 检查 ImagePullSecrets
	for _, ips := range pod.Spec.ImagePullSecrets {
		if ips.Name == secretName {
			usageTypes = append(usageTypes, "imagePullSecret")
		}
	}

	// 检查所有容器
	allContainers := append(pod.Spec.InitContainers, pod.Spec.Containers...)
	for _, container := range allContainers {
		found := false

		// 检查环境变量
		for _, env := range container.Env {
			if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil &&
				env.ValueFrom.SecretKeyRef.Name == secretName {
				if !found {
					usageTypes = append(usageTypes, "env")
					containerNames = append(containerNames, container.Name)
					found = true
				}
				usedKeys = append(usedKeys, env.ValueFrom.SecretKeyRef.Key)
			}
		}

		// 检查 EnvFrom
		for _, envFrom := range container.EnvFrom {
			if envFrom.SecretRef != nil && envFrom.SecretRef.Name == secretName {
				if !found {
					usageTypes = append(usageTypes, "envFrom")
					containerNames = append(containerNames, container.Name)
					found = true
				}
			}
		}
	}

	if len(usageTypes) > 0 {
		return &types.SecretUsageReference{
			UsageType:      usageTypes,
			UsedKeys:       usedKeys,
			ContainerNames: containerNames,
		}
	}

	return nil
}

// markManagedPods 标记由指定 selector 管理的 Pod
func (s *secretOperator) markManagedPods(namespace string, selector *metav1.LabelSelector, managedPods map[string]bool) {
	if selector == nil {
		return
	}

	selectorStr := labels.Set(selector.MatchLabels).String()
	pods, err := s.client.CoreV1().Pods(namespace).List(s.ctx, metav1.ListOptions{
		LabelSelector: selectorStr,
	})

	if err == nil {
		for _, pod := range pods.Items {
			podKey := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
			managedPods[podKey] = true
		}
	}
}

func (s *secretOperator) CanDelete(namespace, name string) (bool, string, error) {
	usage, err := s.GetUsage(namespace, name)
	if err != nil {
		return false, "", err
	}

	return usage.CanDelete, usage.DeleteWarning, nil
}
