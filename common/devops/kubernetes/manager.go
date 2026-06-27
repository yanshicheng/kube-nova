package kubernetes

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	devopstypes "github.com/yanshicheng/kube-nova/common/devops/types"
	"github.com/zeromicro/go-zero/core/logx"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Manager struct{}

func New() devopstypes.Provider {
	return Manager{}
}

func (Manager) TestConnection(ctx context.Context, req devopstypes.Request) devopstypes.Result {
	endpoint, token := resolveEndpointAndToken(req)
	metadata := devopstypes.Metadata{
		ProductName:  productName(req.Channel.Type),
		Capabilities: map[string]any{},
	}
	if endpoint == "" || strings.HasPrefix(endpoint, "system://") {
		return devopstypes.Unhealthy("Kubernetes API 地址为空", metadata)
	}
	versionURL := strings.TrimRight(endpoint, "/") + "/version"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, versionURL, http.NoBody)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/kubernetes/manager.go", err)
		return devopstypes.Unhealthy(err.Error(), metadata)
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{
		Timeout: 6 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: req.Channel.InsecureSkipTLS}, //nolint:gosec
		},
	}
	resp, err := client.Do(request)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/kubernetes/manager.go", err)
		return devopstypes.Unhealthy(fmt.Sprintf("连接失败: %s", devopstypes.TrimMessage(err.Error())), metadata)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return devopstypes.Unhealthy(fmt.Sprintf("Kubernetes API %d", resp.StatusCode), metadata)
	}
	var version struct {
		GitVersion string `json:"gitVersion"`
		Major      string `json:"major"`
		Minor      string `json:"minor"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&version)
	metadata.ProductVersion = version.GitVersion
	metadata.APIVersion = "v1"
	metadata.Capabilities["versionApi"] = true
	if req.Channel.Type == "tekton" {
		metadata.Capabilities["tektonTarget"] = true
	}
	if req.Channel.Type == "kube_bench" {
		metadata.BenchmarkProfile = "cis"
	}
	return devopstypes.Healthy(fmt.Sprintf("%s API %d", metadata.ProductName, resp.StatusCode), metadata)
}

func (Manager) ListNamespaces(ctx context.Context, req devopstypes.Request, keyword string, page, pageSize uint64) ([]devopstypes.DynamicOption, uint64, error) {
	client, err := kubeClient(req)
	if err != nil {
		logx.Errorf("查询 Kubernetes 命名空间失败: %v", err)
		return nil, 0, err
	}
	list, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		logx.Errorf("查询 Kubernetes 命名空间失败: %v", err)
		return nil, 0, formatKubernetesAPIError(err, "读取命名空间列表", "core/namespaces 的 list", "")
	}
	options := make([]devopstypes.DynamicOption, 0, len(list.Items))
	for _, item := range list.Items {
		name := strings.TrimSpace(item.Name)
		if !matchKeyword(name, keyword) {
			continue
		}
		options = append(options, devopstypes.DynamicOption{
			Label: name,
			Value: name,
			Metadata: map[string]any{
				"namespace": name,
				"status":    string(item.Status.Phase),
			},
		})
	}
	sortOptions(options)
	return paginateOptions(options, page, pageSize)
}

func (Manager) ListResources(ctx context.Context, req devopstypes.Request, namespace, resourceType, keyword string, page, pageSize uint64) ([]devopstypes.DynamicOption, uint64, error) {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return nil, 0, fmt.Errorf("请选择命名空间")
	}
	resourceType, err := normalizeResourceType(resourceType)
	if err != nil {
		return nil, 0, err
	}
	client, err := kubeClient(req)
	if err != nil {
		logx.Errorf("查询 Kubernetes 资源失败: %v", err)
		return nil, 0, err
	}
	var options []devopstypes.DynamicOption
	switch resourceType {
	case "deployment":
		list, err := client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			logx.Errorf("查询 Kubernetes Deployment 失败: %v", err)
			return nil, 0, formatKubernetesAPIError(err, "读取 Deployment 列表", "apps/deployments 的 list", namespace)
		}
		options = make([]devopstypes.DynamicOption, 0, len(list.Items))
		for _, item := range list.Items {
			options = appendResourceOption(options, namespace, resourceType, "Deployment", item.Name, item.Spec.Replicas, keyword)
		}
	case "statefulset":
		list, err := client.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			logx.Errorf("查询 Kubernetes StatefulSet 失败: %v", err)
			return nil, 0, formatKubernetesAPIError(err, "读取 StatefulSet 列表", "apps/statefulsets 的 list", namespace)
		}
		options = make([]devopstypes.DynamicOption, 0, len(list.Items))
		for _, item := range list.Items {
			options = appendResourceOption(options, namespace, resourceType, "StatefulSet", item.Name, item.Spec.Replicas, keyword)
		}
	case "daemonset":
		list, err := client.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			logx.Errorf("查询 Kubernetes DaemonSet 失败: %v", err)
			return nil, 0, formatKubernetesAPIError(err, "读取 DaemonSet 列表", "apps/daemonsets 的 list", namespace)
		}
		options = make([]devopstypes.DynamicOption, 0, len(list.Items))
		for _, item := range list.Items {
			options = appendResourceOption(options, namespace, resourceType, "DaemonSet", item.Name, nil, keyword)
		}
	case "cronjob":
		list, err := client.BatchV1().CronJobs(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			logx.Errorf("查询 Kubernetes CronJob 失败: %v", err)
			return nil, 0, formatKubernetesAPIError(err, "读取 CronJob 列表", "batch/cronjobs 的 list", namespace)
		}
		options = make([]devopstypes.DynamicOption, 0, len(list.Items))
		for _, item := range list.Items {
			options = appendResourceOption(options, namespace, resourceType, "CronJob", item.Name, nil, keyword)
		}
	}
	sortOptions(options)
	return paginateOptions(options, page, pageSize)
}

func (Manager) ListContainers(ctx context.Context, req devopstypes.Request, namespace, resourceType, resourceName, keyword string) ([]devopstypes.DynamicOption, uint64, error) {
	namespace = strings.TrimSpace(namespace)
	resourceName = strings.TrimSpace(resourceName)
	if namespace == "" {
		return nil, 0, fmt.Errorf("请选择命名空间")
	}
	if resourceName == "" {
		return nil, 0, fmt.Errorf("请选择 Kubernetes 资源")
	}
	resourceType, err := normalizeResourceType(resourceType)
	if err != nil {
		return nil, 0, err
	}
	client, err := kubeClient(req)
	if err != nil {
		logx.Errorf("查询 Kubernetes 容器失败: %v", err)
		return nil, 0, err
	}
	var initContainers []corev1.Container
	var containers []corev1.Container
	switch resourceType {
	case "deployment":
		item, err := client.AppsV1().Deployments(namespace).Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			logx.Errorf("查询 Kubernetes Deployment 容器失败: %v", err)
			return nil, 0, formatKubernetesAPIError(err, "读取 Deployment 容器", "apps/deployments 的 get", namespace)
		}
		initContainers = item.Spec.Template.Spec.InitContainers
		containers = item.Spec.Template.Spec.Containers
	case "statefulset":
		item, err := client.AppsV1().StatefulSets(namespace).Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			logx.Errorf("查询 Kubernetes StatefulSet 容器失败: %v", err)
			return nil, 0, formatKubernetesAPIError(err, "读取 StatefulSet 容器", "apps/statefulsets 的 get", namespace)
		}
		initContainers = item.Spec.Template.Spec.InitContainers
		containers = item.Spec.Template.Spec.Containers
	case "daemonset":
		item, err := client.AppsV1().DaemonSets(namespace).Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			logx.Errorf("查询 Kubernetes DaemonSet 容器失败: %v", err)
			return nil, 0, formatKubernetesAPIError(err, "读取 DaemonSet 容器", "apps/daemonsets 的 get", namespace)
		}
		initContainers = item.Spec.Template.Spec.InitContainers
		containers = item.Spec.Template.Spec.Containers
	case "cronjob":
		item, err := client.BatchV1().CronJobs(namespace).Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			logx.Errorf("查询 Kubernetes CronJob 容器失败: %v", err)
			return nil, 0, formatKubernetesAPIError(err, "读取 CronJob 容器", "batch/cronjobs 的 get", namespace)
		}
		initContainers = item.Spec.JobTemplate.Spec.Template.Spec.InitContainers
		containers = item.Spec.JobTemplate.Spec.Template.Spec.Containers
	}
	options := containerOptions(initContainers, containers, keyword)
	sortOptions(options)
	return options, uint64(len(options)), nil
}

func (m Manager) ListImages(ctx context.Context, req devopstypes.Request, namespace, resourceType, resourceName, containerName, keyword string) ([]devopstypes.DynamicOption, uint64, error) {
	options, _, err := m.ListContainers(ctx, req, namespace, resourceType, resourceName, containerName)
	if err != nil {
		return nil, 0, err
	}
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	result := make([]devopstypes.DynamicOption, 0, len(options))
	seen := map[string]struct{}{}
	for _, item := range options {
		if strings.TrimSpace(item.Value) != strings.TrimSpace(containerName) {
			continue
		}
		image, _ := item.Metadata["image"].(string)
		image = strings.TrimSpace(image)
		if image == "" {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(image), keyword) {
			continue
		}
		if _, ok := seen[image]; ok {
			continue
		}
		seen[image] = struct{}{}
		result = append(result, devopstypes.DynamicOption{
			Label:    image,
			Value:    image,
			Metadata: item.Metadata,
		})
	}
	return result, uint64(len(result)), nil
}

func productName(channelType string) string {
	switch channelType {
	case "tekton":
		return "Tekton"
	case "kube_bench":
		return "kube-bench"
	default:
		return "Kubernetes"
	}
}

func resolveEndpointAndToken(req devopstypes.Request) (string, string) {
	endpoint := strings.TrimSpace(req.Channel.Endpoint)
	token := strings.TrimSpace(req.Channel.Token)
	if req.Credential == nil {
		return endpoint, token
	}
	switch req.Credential.Type {
	case "token":
		token = strings.TrimSpace(req.Credential.Token)
	case "secret_text":
		content := strings.TrimSpace(req.Credential.SecretText)
		if looksLikeKubeconfig(content) {
			if parsed := kubeconfigValue(content, "server"); parsed != "" && (endpoint == "" || strings.HasPrefix(endpoint, "system://")) {
				endpoint = parsed
			}
			if token == "" {
				token = kubeconfigValue(content, "token")
			}
		} else if content != "" {
			token = content
		}
	case "kubeconfig":
		if parsed := kubeconfigValue(req.Credential.Kubeconfig, "server"); parsed != "" && (endpoint == "" || strings.HasPrefix(endpoint, "system://")) {
			endpoint = parsed
		}
		if token == "" {
			token = kubeconfigValue(req.Credential.Kubeconfig, "token")
		}
	}
	return endpoint, token
}

func kubeClient(req devopstypes.Request) (*kubernetes.Clientset, error) {
	config, err := restConfigFromRequest(req)
	if err != nil {
		return nil, err
	}
	config.Timeout = 10 * time.Second
	return kubernetes.NewForConfig(config)
}

func restConfigFromRequest(req devopstypes.Request) (*rest.Config, error) {
	if req.Credential != nil {
		switch strings.TrimSpace(req.Credential.Type) {
		case "kubeconfig":
			return restConfigFromKubeconfig(req.Credential.Kubeconfig, req.Channel.InsecureSkipTLS)
		case "secret_text":
			content := strings.TrimSpace(req.Credential.SecretText)
			if looksLikeKubeconfig(content) {
				return restConfigFromKubeconfig(content, req.Channel.InsecureSkipTLS)
			}
		}
	}
	endpoint, token := resolveEndpointAndToken(req)
	if endpoint == "" || strings.HasPrefix(endpoint, "system://") {
		return nil, fmt.Errorf("Kubernetes API 地址为空")
	}
	if token == "" {
		return nil, fmt.Errorf("Kubernetes 凭据未包含可用 token")
	}
	return &rest.Config{
		Host:        endpoint,
		BearerToken: token,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: req.Channel.InsecureSkipTLS,
		},
	}, nil
}

func restConfigFromKubeconfig(content string, insecureSkipTLS bool) (*rest.Config, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("Kubeconfig 凭据为空")
	}
	config, err := clientcmd.RESTConfigFromKubeConfig([]byte(content))
	if err != nil {
		return nil, fmt.Errorf("解析 kubeconfig 失败: %w", err)
	}
	if insecureSkipTLS {
		config.TLSClientConfig.Insecure = true
	}
	return config, nil
}

func appendResourceOption(options []devopstypes.DynamicOption, namespace, resourceType, kind, name string, replicas *int32, keyword string) []devopstypes.DynamicOption {
	name = strings.TrimSpace(name)
	if !matchKeyword(name, keyword) {
		return options
	}
	metadata := map[string]any{
		"namespace":    namespace,
		"resourceType": resourceType,
		"kind":         kind,
	}
	if replicas != nil {
		metadata["replicas"] = *replicas
	}
	return append(options, devopstypes.DynamicOption{
		Label:    name,
		Value:    name,
		Metadata: metadata,
	})
}

func containerOptions(initContainers, containers []corev1.Container, keyword string) []devopstypes.DynamicOption {
	options := make([]devopstypes.DynamicOption, 0, len(initContainers)+len(containers))
	for _, item := range initContainers {
		options = appendContainerOption(options, item, "initContainer", keyword)
	}
	for _, item := range containers {
		options = appendContainerOption(options, item, "container", keyword)
	}
	return options
}

func appendContainerOption(options []devopstypes.DynamicOption, item corev1.Container, containerType, keyword string) []devopstypes.DynamicOption {
	name := strings.TrimSpace(item.Name)
	if !matchKeyword(name, keyword) {
		return options
	}
	return append(options, devopstypes.DynamicOption{
		Label: name,
		Value: name,
		Metadata: map[string]any{
			"containerType": containerType,
			"image":         item.Image,
		},
	})
}

func formatKubernetesAPIError(err error, action, resource, namespace string) error {
	if err == nil {
		return nil
	}
	if apierrors.IsForbidden(err) {
		return fmt.Errorf("当前 Kubernetes 凭据没有%s权限。%s请在集群 RBAC 中授予 %s 权限，或更换具备权限的凭据", action, namespaceScopeText(namespace), resource)
	}
	if apierrors.IsUnauthorized(err) {
		return fmt.Errorf("当前 Kubernetes 凭据认证失败，请检查 token 或 kubeconfig 是否正确")
	}
	if apierrors.IsNotFound(err) {
		return fmt.Errorf("Kubernetes 资源不存在。%s请确认资源是否已创建或命名空间是否正确", namespaceScopeText(namespace))
	}
	return err
}

func namespaceScopeText(namespace string) string {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return ""
	}
	return fmt.Sprintf("目标命名空间：%s。", namespace)
}

func paginateOptions(options []devopstypes.DynamicOption, page, pageSize uint64) ([]devopstypes.DynamicOption, uint64, error) {
	total := uint64(len(options))
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start >= total {
		return []devopstypes.DynamicOption{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return options[start:end], total, nil
}

func sortOptions(options []devopstypes.DynamicOption) {
	sort.SliceStable(options, func(i, j int) bool {
		return strings.ToLower(options[i].Label) < strings.ToLower(options[j].Label)
	})
}

func matchKeyword(value, keyword string) bool {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return true
	}
	return strings.Contains(strings.ToLower(value), keyword)
}

func normalizeResourceType(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "deployment", "deploy", "deployments":
		return "deployment", nil
	case "statefulset", "statefulsets":
		return "statefulset", nil
	case "daemonset", "daemonsets":
		return "daemonset", nil
	case "cronjob", "cronjobs":
		return "cronjob", nil
	default:
		return "", fmt.Errorf("Kubernetes 资源类型不支持")
	}
}

func looksLikeKubeconfig(content string) bool {
	content = strings.TrimSpace(content)
	return strings.Contains(content, "apiVersion:") && strings.Contains(content, "clusters:")
}

func kubeconfigValue(content, key string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, key+":") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, key+":"))
		return strings.Trim(value, `"'`)
	}
	return ""
}
