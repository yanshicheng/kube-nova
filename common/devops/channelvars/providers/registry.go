package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
)

// RegistryProvider Docker Registry Provider (支持 Docker Registry 和阿里云镜像仓库)
type RegistryProvider struct {
	channelManager channelvars.ChannelManager
}

// NewRegistryProvider 创建 Registry Provider
func NewRegistryProvider(channelManager channelvars.ChannelManager) *RegistryProvider {
	return &RegistryProvider{
		channelManager: channelManager,
	}
}

// Capabilities 返回能力
func (p *RegistryProvider) Capabilities() channelvars.ProviderCapabilities {
	return channelvars.ProviderCapabilities{
		SupportedQueries: []string{
			"image.project",
			"image.registry",
			"image.image",
			"image.tag",
			"image.imageTag",
		},
		SupportedAddressFormats: []channelvars.AddressFormat{
			{
				Field:   "address.projectUrl",
				Pattern: `^[^/]+/.+$`,
				Example: "registry.example.com/namespace",
			},
			{
				Field:   "address.imageUrl",
				Pattern: `^[^/]+/.+/.+$`,
				Example: "registry.example.com/namespace/nginx",
			},
			{
				Field:   "address.imageTagUrl",
				Pattern: `^[^/]+/.+/.+:[^:]+$`,
				Example: "registry.example.com/namespace/nginx:latest",
			},
		},
		SupportsAddressResolve: true,
		RequiresCredential:     true,
	}
}

// ResolveAddress 解析地址
func (p *RegistryProvider) ResolveAddress(ctx context.Context, req *channelvars.ResolveAddressRequest) (*channelvars.ResolveAddressResponse, error) {
	address := strings.TrimSpace(req.Address)
	if !strings.Contains(address, "://") {
		address = "https://" + address
	}
	u, err := url.Parse(address)
	if err != nil || u.Host == "" {
		return &channelvars.ResolveAddressResponse{
			Resolved: false,
			Reason:   fmt.Sprintf("invalid URL: %v", err),
		}, nil
	}

	// 获取渠道实例
	instances, err := p.channelManager.FindInstancesByEndpoint(ctx, fmt.Sprintf("%s://%s", u.Scheme, u.Host), req.ProjectID)
	if err != nil || len(instances) == 0 {
		return &channelvars.ResolveAddressResponse{
			Resolved: false,
			Reason:   "no matching channel instance found",
		}, nil
	}

	// 解析路径：/{repository}:{tag} 或 /{namespace}/{repository}:{tag}
	path := strings.TrimPrefix(u.Path, "/")

	// 分离镜像和标签
	var imagePath, tag string
	if idx := strings.LastIndex(path, ":"); idx != -1 {
		imagePath = path[:idx]
		tag = path[idx+1:]
	} else {
		imagePath = path
	}

	// 分离命名空间和仓库
	var namespace, repository string
	parts := strings.Split(imagePath, "/")
	if len(parts) >= 2 {
		namespace = strings.Join(parts[:len(parts)-1], "/")
		repository = parts[len(parts)-1]
	} else if len(parts) == 1 {
		repository = parts[0]
	}

	// 构建变量
	fields := map[string]string{}

	if namespace != "" {
		fields[channelvars.FieldDynamicProject] = namespace
		fields[channelvars.FieldDynamicRegistry] = namespace
		fields["dynamic.namespace"] = namespace
		fields[channelvars.FieldAddressProjectURL] = u.Host + "/" + namespace
	}
	if repository != "" {
		fields[channelvars.FieldDynamicImage] = repository
		fields[channelvars.FieldDynamicRepository] = repository
		if namespace != "" {
			fields[channelvars.FieldAddressImageURL] = u.Host + "/" + namespace + "/" + repository
		}
	}
	if tag != "" {
		fields[channelvars.FieldDynamicTag] = tag
		fields[channelvars.FieldDynamicImageTag] = repository + ":" + tag
		if namespace != "" && repository != "" {
			fields[channelvars.FieldAddressImageTagURL] = u.Host + "/" + namespace + "/" + repository + ":" + tag
		}
	}

	return &channelvars.ResolveAddressResponse{
		Resolved:          true,
		ChannelInstanceID: instances[0].ID,
		Fields:            fields,
	}, nil
}

// QueryOptions 查询选项
func (p *RegistryProvider) QueryOptions(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	// 获取渠道实例
	instance, err := p.channelManager.GetInstance(ctx, req.ChannelInstanceID)
	if err != nil {
		return nil, fmt.Errorf("get channel instance: %w", err)
	}

	switch req.ProviderKey {
	case "image.project", "image.registry":
		return p.queryNamespaces(ctx, instance)
	case "image.image":
		return p.queryRepositories(ctx, instance, req.ParentValues)
	case "image.tag":
		return p.queryTags(ctx, instance, req.ParentValues)
	case "image.imageTag":
		return p.queryImageTags(ctx, instance, req.ParentValues)
	default:
		return &channelvars.QueryOptionsResponse{
			Options: []channelvars.Option{},
		}, nil
	}
}

// queryNamespaces 查询命名空间列表
func (p *RegistryProvider) queryNamespaces(ctx context.Context, instance *channelvars.ChannelInstance) (*channelvars.QueryOptionsResponse, error) {
	// Docker Registry v2 API 不直接支持命名空间查询
	// 我们通过 _catalog 接口获取所有仓库，然后提取命名空间
	apiURL := fmt.Sprintf("%s/v2/_catalog", strings.TrimSuffix(instance.Endpoint, "/"))

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// 设置认证
	p.setAuth(req, instance)

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s, body: %s", resp.Status, string(body))
	}

	// 解析响应
	var result struct {
		Repositories []string `json:"repositories"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// 提取命名空间
	namespaceSet := make(map[string]bool)
	for _, repo := range result.Repositories {
		if idx := strings.LastIndex(repo, "/"); idx != -1 {
			namespace := repo[:idx]
			namespaceSet[namespace] = true
		}
	}

	// 转换为选项
	options := make([]channelvars.Option, 0, len(namespaceSet))
	for namespace := range namespaceSet {
		options = append(options, channelvars.Option{
			Label: namespace,
			Value: namespace,
		})
	}

	return &channelvars.QueryOptionsResponse{
		Options: options,
	}, nil
}

// queryRepositories 查询仓库列表
func (p *RegistryProvider) queryRepositories(ctx context.Context, instance *channelvars.ChannelInstance, parentValues map[string]string) (*channelvars.QueryOptionsResponse, error) {
	apiURL := fmt.Sprintf("%s/v2/_catalog", strings.TrimSuffix(instance.Endpoint, "/"))

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// 设置认证
	p.setAuth(req, instance)

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s, body: %s", resp.Status, string(body))
	}

	// 解析响应
	var result struct {
		Repositories []string `json:"repositories"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// 过滤命名空间
	namespace := registryNamespaceValue(parentValues)
	options := make([]channelvars.Option, 0)

	for _, repo := range result.Repositories {
		// 如果指定了命名空间，只返回该命名空间下的仓库
		if namespace != "" {
			if !strings.HasPrefix(repo, namespace+"/") {
				continue
			}
			// 去掉命名空间前缀
			repo = strings.TrimPrefix(repo, namespace+"/")
		}

		// 如果仓库名包含 /，说明还有子命名空间，跳过
		if namespace != "" && strings.Contains(repo, "/") {
			continue
		}

		options = append(options, channelvars.Option{
			Label: repo,
			Value: repo,
		})
	}

	return &channelvars.QueryOptionsResponse{
		Options: options,
	}, nil
}

// queryTags 查询标签列表
func (p *RegistryProvider) queryTags(ctx context.Context, instance *channelvars.ChannelInstance, parentValues map[string]string) (*channelvars.QueryOptionsResponse, error) {
	namespace := registryNamespaceValue(parentValues)
	repository := registryImageValue(parentValues)

	if repository == "" {
		return &channelvars.QueryOptionsResponse{
			Options: []channelvars.Option{},
		}, nil
	}

	// 构建完整的仓库名
	fullRepo := repository
	if namespace != "" {
		fullRepo = namespace + "/" + repository
	}

	apiURL := fmt.Sprintf("%s/v2/%s/tags/list", strings.TrimSuffix(instance.Endpoint, "/"), fullRepo)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// 设置认证
	p.setAuth(req, instance)

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s, body: %s", resp.Status, string(body))
	}

	// 解析响应
	var result struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// 转换为选项
	options := make([]channelvars.Option, 0, len(result.Tags))
	for _, tag := range result.Tags {
		options = append(options, channelvars.Option{
			Label: tag,
			Value: tag,
		})
	}

	return &channelvars.QueryOptionsResponse{
		Options: options,
	}, nil
}

// queryImageTags 查询镜像加标签组合
func (p *RegistryProvider) queryImageTags(ctx context.Context, instance *channelvars.ChannelInstance, parentValues map[string]string) (*channelvars.QueryOptionsResponse, error) {
	namespace := registryNamespaceValue(parentValues)
	repository := registryImageValue(parentValues)
	tagsResp, err := p.queryTags(ctx, instance, parentValues)
	if err != nil {
		return nil, err
	}
	options := make([]channelvars.Option, 0, len(tagsResp.Options))
	for _, tag := range tagsResp.Options {
		value := repository + ":" + tag.Value
		label := repository + ":" + tag.Label
		if namespace != "" && repository == "" {
			value = namespace + ":" + tag.Value
			label = namespace + ":" + tag.Label
		}
		options = append(options, channelvars.Option{Label: label, Value: value})
	}
	return &channelvars.QueryOptionsResponse{Options: options}, nil
}

func registryNamespaceValue(parentValues map[string]string) string {
	if parentValues == nil {
		return ""
	}
	for _, field := range []string{
		channelvars.FieldDynamicProject,
		channelvars.FieldDynamicRegistry,
		"dynamic.namespace",
	} {
		if value := strings.TrimSpace(parentValues[field]); value != "" {
			return value
		}
	}
	for _, field := range []string{
		channelvars.FieldAddressProjectURL,
		channelvars.FieldAddressRegistryURL,
		channelvars.FieldAddressImageURL,
		channelvars.FieldAddressImageTagURL,
	} {
		if project, _, _ := splitImageAddress(parentValues[field]); project != "" {
			return project
		}
	}
	return ""
}

func registryImageValue(parentValues map[string]string) string {
	if parentValues == nil {
		return ""
	}
	for _, field := range []string{channelvars.FieldDynamicImage, channelvars.FieldDynamicRepository} {
		if value := strings.TrimSpace(parentValues[field]); value != "" {
			return value
		}
	}
	for _, field := range []string{channelvars.FieldAddressImageURL, channelvars.FieldAddressImageTagURL} {
		if _, image, _ := splitImageAddress(parentValues[field]); image != "" {
			return image
		}
	}
	return ""
}

// setAuth 设置认证信息
func (p *RegistryProvider) setAuth(req *http.Request, instance *channelvars.ChannelInstance) {
	if token, ok := instance.Config["token"].(string); ok && token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	} else if username, ok := instance.Config["username"].(string); ok {
		if password, ok := instance.Config["password"].(string); ok {
			req.SetBasicAuth(username, password)
		}
	}
}

// RenderOutput 渲染输出
func (p *RegistryProvider) RenderOutput(ctx context.Context, req *channelvars.RenderOutputRequest) (*channelvars.RenderOutputResponse, error) {
	return nil, fmt.Errorf("registry provider does not support render output")
}

// ValidateValue 校验值
func (p *RegistryProvider) ValidateValue(ctx context.Context, req *channelvars.ValidateValueRequest) error {
	return nil
}
