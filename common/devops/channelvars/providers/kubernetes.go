package providers

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
)

// KubernetesProvider Kubernetes Provider实现
type KubernetesProvider struct {
	channelManager channelvars.ChannelManager
	k8sManager     K8sManager
}

// NewKubernetesProvider 创建Kubernetes Provider
func NewKubernetesProvider(channelManager channelvars.ChannelManager, k8sManager K8sManager) *KubernetesProvider {
	return &KubernetesProvider{
		channelManager: channelManager,
		k8sManager:     k8sManager,
	}
}

// Capabilities 声明能力
func (p *KubernetesProvider) Capabilities() channelvars.ProviderCapabilities {
	return channelvars.ProviderCapabilities{
		SupportedQueries: []string{
			"kubernetes.namespace",
			"kubernetes.workloadType",
			"kubernetes.resourceName",
			"kubernetes.containerName",
			"kubernetes.imageName",
			"k8s.namespace",
			"k8s.workloadType",
			"k8s.resource",
			"k8s.container",
			"k8s.image",
		},
		SupportedAddressFormats: nil,
		SupportsAddressResolve:  false,
		RequiresCredential:      true,
	}
}

// QueryOptions 查询选项
func (p *KubernetesProvider) QueryOptions(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	switch req.ProviderKey {
	case "kubernetes.namespace", "k8s.namespace":
		return p.queryNamespaces(ctx, req)
	case "kubernetes.workloadType", "k8s.workloadType":
		return p.queryWorkloadTypes(ctx, req)
	case "kubernetes.resourceName", "k8s.resource":
		return p.queryResources(ctx, req)
	case "kubernetes.containerName", "k8s.container":
		return p.queryContainers(ctx, req)
	case "kubernetes.imageName", "k8s.image":
		return p.queryImage(ctx, req)
	default:
		return nil, fmt.Errorf("unsupported provider key: %s", req.ProviderKey)
	}
}

// queryNamespaces 查询命名空间列表
func (p *KubernetesProvider) queryNamespaces(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	instance, err := p.channelManager.GetInstance(ctx, req.ChannelInstanceID)
	if err != nil {
		return nil, fmt.Errorf("get channel instance: %w", err)
	}

	// 调用K8s API查询命名空间
	namespaces, err := p.k8sManager.ListNamespaces(ctx, instance.ID)
	if err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}

	options := make([]channelvars.Option, len(namespaces))
	for i, ns := range namespaces {
		options[i] = channelvars.Option{
			Label: ns,
			Value: ns,
		}
	}

	return &channelvars.QueryOptionsResponse{
		Options: options,
	}, nil
}

// queryWorkloadTypes 查询工作负载类型
func (p *KubernetesProvider) queryWorkloadTypes(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	// 工作负载类型是固定的
	options := []channelvars.Option{
		{Label: "Deployment", Value: "deployment"},
		{Label: "StatefulSet", Value: "statefulset"},
		{Label: "DaemonSet", Value: "daemonset"},
		{Label: "CronJob", Value: "cronjob"},
	}

	return &channelvars.QueryOptionsResponse{
		Options: options,
	}, nil
}

// queryResources 查询资源列表
func (p *KubernetesProvider) queryResources(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	namespace := req.ParentValues[channelvars.FieldDynamicNamespace]
	workloadType := req.ParentValues[channelvars.FieldDynamicWorkloadType]

	if namespace == "" || workloadType == "" {
		return nil, fmt.Errorf("%s and %s are required", channelvars.FieldDynamicNamespace, channelvars.FieldDynamicWorkloadType)
	}

	instance, err := p.channelManager.GetInstance(ctx, req.ChannelInstanceID)
	if err != nil {
		return nil, fmt.Errorf("get channel instance: %w", err)
	}

	// 调用K8s API查询资源列表
	resources, err := p.k8sManager.ListResources(ctx, instance.ID, namespace, workloadType)
	if err != nil {
		return nil, fmt.Errorf("list resources: %w", err)
	}

	options := make([]channelvars.Option, len(resources))
	for i, res := range resources {
		options[i] = channelvars.Option{
			Label: res,
			Value: res,
		}
	}

	return &channelvars.QueryOptionsResponse{
		Options: options,
	}, nil
}

// queryContainers 查询容器列表
func (p *KubernetesProvider) queryContainers(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	namespace := req.ParentValues[channelvars.FieldDynamicNamespace]
	workloadType := req.ParentValues[channelvars.FieldDynamicWorkloadType]
	resource := firstValue(req.ParentValues, channelvars.FieldDynamicResourceName, channelvars.FieldDynamicResource)

	if namespace == "" || workloadType == "" || resource == "" {
		return nil, fmt.Errorf("%s, %s and %s are required", channelvars.FieldDynamicNamespace, channelvars.FieldDynamicWorkloadType, channelvars.FieldDynamicResourceName)
	}

	instance, err := p.channelManager.GetInstance(ctx, req.ChannelInstanceID)
	if err != nil {
		return nil, fmt.Errorf("get channel instance: %w", err)
	}

	// 调用K8s API查询容器列表
	containers, err := p.k8sManager.ListContainers(ctx, instance.ID, namespace, workloadType, resource)
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	options := make([]channelvars.Option, len(containers))
	for i, container := range containers {
		options[i] = channelvars.Option{
			Label: container,
			Value: container,
		}
	}

	return &channelvars.QueryOptionsResponse{
		Options: options,
	}, nil
}

// queryImage 查询当前镜像
func (p *KubernetesProvider) queryImage(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	namespace := req.ParentValues[channelvars.FieldDynamicNamespace]
	workloadType := req.ParentValues[channelvars.FieldDynamicWorkloadType]
	resource := firstValue(req.ParentValues, channelvars.FieldDynamicResourceName, channelvars.FieldDynamicResource)
	container := firstValue(req.ParentValues, channelvars.FieldDynamicContainerName, channelvars.FieldDynamicContainer)

	if namespace == "" || workloadType == "" || resource == "" || container == "" {
		return nil, fmt.Errorf("all parent values are required")
	}

	instance, err := p.channelManager.GetInstance(ctx, req.ChannelInstanceID)
	if err != nil {
		return nil, fmt.Errorf("get channel instance: %w", err)
	}

	// 调用K8s API查询容器镜像
	image, err := p.k8sManager.GetContainerImage(ctx, instance.ID, namespace, workloadType, resource, container)
	if err != nil {
		return nil, fmt.Errorf("get container image: %w", err)
	}

	options := []channelvars.Option{
		{Label: image, Value: image},
	}

	return &channelvars.QueryOptionsResponse{
		Options: options,
	}, nil
}

// ResolveAddress 解析地址
func (p *KubernetesProvider) ResolveAddress(ctx context.Context, req *channelvars.ResolveAddressRequest) (*channelvars.ResolveAddressResponse, error) {
	return &channelvars.ResolveAddressResponse{
		Resolved: false,
		Reason:   "kubernetes provider does not support address resolve",
	}, nil
}

// RenderOutput 渲染输出
func (p *KubernetesProvider) RenderOutput(ctx context.Context, req *channelvars.RenderOutputRequest) (*channelvars.RenderOutputResponse, error) {
	return nil, fmt.Errorf("kubernetes provider does not support render output")
}

// ValidateValue 校验值
func (p *KubernetesProvider) ValidateValue(ctx context.Context, req *channelvars.ValidateValueRequest) error {
	return nil
}

// K8sManager Kubernetes管理器接口
type K8sManager interface {
	// ListNamespaces 查询命名空间列表
	ListNamespaces(ctx context.Context, clusterID int64) ([]string, error)

	// ListResources 查询资源列表
	ListResources(ctx context.Context, clusterID int64, namespace, workloadType string) ([]string, error)

	// ListContainers 查询容器列表
	ListContainers(ctx context.Context, clusterID int64, namespace, workloadType, resource string) ([]string, error)

	// GetContainerImage 获取容器镜像
	GetContainerImage(ctx context.Context, clusterID int64, namespace, workloadType, resource, container string) (string, error)
}
