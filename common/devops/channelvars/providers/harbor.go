package providers

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
	"github.com/yanshicheng/kube-nova/common/devops/channelvars/clients/harbor"
)

// HarborProvider Harbor Provider实现
type HarborProvider struct {
	channelManager channelvars.ChannelManager
}

// NewHarborProvider 创建Harbor Provider
func NewHarborProvider(channelManager channelvars.ChannelManager) *HarborProvider {
	return &HarborProvider{
		channelManager: channelManager,
	}
}

// Capabilities 声明能力
func (p *HarborProvider) Capabilities() channelvars.ProviderCapabilities {
	return channelvars.ProviderCapabilities{
		SupportedQueries: []string{
			"image.project",
			"image.registry",
			"image.image",
			"image.tag",
			"image.imageTag",
			"image.projectUrl",
			"image.registryUrl",
			"image.imageUrl",
			"image.imageTagUrl",
		},
		SupportedAddressFormats: []channelvars.AddressFormat{
			{
				Field:   "address.projectUrl",
				Pattern: `^[^/]+/[^/]+$`,
				Example: "harbor.example.com/library",
			},
			{
				Field:   "address.imageUrl",
				Pattern: `^[^/]+/[^/]+/[^:]+$`,
				Example: "harbor.example.com/library/nginx",
			},
			{
				Field:   "address.imageTagUrl",
				Pattern: `^[^/]+/[^/]+/[^:]+:[^/]+$`,
				Example: "harbor.example.com/library/nginx:latest",
			},
		},
		SupportsAddressResolve: true,
		RequiresCredential:     true,
	}
}

// QueryOptions 查询选项
func (p *HarborProvider) QueryOptions(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	switch req.ProviderKey {
	case "image.project", "image.registry":
		return p.queryProjects(ctx, req)
	case "image.image":
		return p.queryImages(ctx, req)
	case "image.tag":
		return p.queryTags(ctx, req)
	case "image.imageTag":
		return p.queryImageTags(ctx, req)
	default:
		return nil, fmt.Errorf("unsupported provider key: %s", req.ProviderKey)
	}
}

// queryProjects 查询 Harbor 项目列表。
func (p *HarborProvider) queryProjects(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	instance, err := p.channelManager.GetInstance(ctx, req.ChannelInstanceID)
	if err != nil {
		return nil, fmt.Errorf("get channel instance: %w", err)
	}

	// 创建 Harbor 客户端
	client, err := p.getHarborClient(instance)
	if err != nil {
		return nil, err
	}

	// 获取搜索关键词
	search := ""
	if req.Search != nil {
		search = *req.Search
	}

	// 调用 Harbor API 查询项目列表
	projects, err := client.ListProjects(ctx, search, 1, 50)
	if err != nil {
		return nil, fmt.Errorf("list harbor projects: %w", err)
	}

	// 转换为选项列表
	options := make([]channelvars.Option, 0, len(projects))
	for _, project := range projects {
		options = append(options, channelvars.Option{
			Label: project.Name,
			Value: project.Name,
		})
	}

	return &channelvars.QueryOptionsResponse{
		Options: options,
	}, nil
}

// queryImages 查询镜像列表
func (p *HarborProvider) queryImages(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	project := imageProjectValue(req.ParentValues)
	if project == "" {
		return nil, fmt.Errorf("dynamic.project or address.projectUrl is required")
	}

	instance, err := p.channelManager.GetInstance(ctx, req.ChannelInstanceID)
	if err != nil {
		return nil, fmt.Errorf("get channel instance: %w", err)
	}

	// 创建 Harbor 客户端
	client, err := p.getHarborClient(instance)
	if err != nil {
		return nil, err
	}

	// 获取搜索关键词
	search := ""
	if req.Search != nil {
		search = *req.Search
	}

	// 调用 Harbor API 查询镜像列表
	repos, err := client.ListRepositories(ctx, project, search, 1, 50)
	if err != nil {
		return nil, fmt.Errorf("list harbor repositories: %w", err)
	}

	// 转换为选项列表
	options := make([]channelvars.Option, 0, len(repos))
	for _, repo := range repos {
		// 去掉项目名前缀
		name := strings.TrimPrefix(repo.Name, project+"/")
		options = append(options, channelvars.Option{
			Label: name,
			Value: name,
		})
	}

	return &channelvars.QueryOptionsResponse{
		Options: options,
	}, nil
}

// queryTags 查询镜像标签列表
func (p *HarborProvider) queryTags(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	project := imageProjectValue(req.ParentValues)
	image := imageNameValue(req.ParentValues)
	if project == "" || image == "" {
		return nil, fmt.Errorf("dynamic.project+dynamic.image or address.imageUrl is required")
	}

	instance, err := p.channelManager.GetInstance(ctx, req.ChannelInstanceID)
	if err != nil {
		return nil, fmt.Errorf("get channel instance: %w", err)
	}

	// 创建 Harbor 客户端
	client, err := p.getHarborClient(instance)
	if err != nil {
		return nil, err
	}

	// 调用 Harbor API 查询标签列表
	tags, err := client.ListTags(ctx, project, image, 1, 50)
	if err != nil {
		return nil, fmt.Errorf("list harbor tags: %w", err)
	}

	// 转换为选项列表
	options := make([]channelvars.Option, 0, len(tags))
	for _, tag := range tags {
		options = append(options, channelvars.Option{
			Label: tag.Name,
			Value: tag.Name,
		})
	}

	return &channelvars.QueryOptionsResponse{
		Options: options,
	}, nil
}

// queryImageTags 查询镜像:标签组合
func (p *HarborProvider) queryImageTags(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	project := imageProjectValue(req.ParentValues)
	image := imageNameValue(req.ParentValues)

	if project == "" || image == "" {
		return nil, fmt.Errorf("dynamic.project and dynamic.image are required")
	}

	// 查询标签列表
	tagsResp, err := p.queryTags(ctx, req)
	if err != nil {
		return nil, err
	}

	// 组合成 image:tag 格式
	options := make([]channelvars.Option, len(tagsResp.Options))
	for i, tag := range tagsResp.Options {
		options[i] = channelvars.Option{
			Label: fmt.Sprintf("%s:%s", image, tag.Label),
			Value: fmt.Sprintf("%s:%s", image, tag.Value),
		}
	}

	return &channelvars.QueryOptionsResponse{
		Options: options,
	}, nil
}

// ResolveAddress 解析地址
func (p *HarborProvider) ResolveAddress(ctx context.Context, req *channelvars.ResolveAddressRequest) (*channelvars.ResolveAddressResponse, error) {
	// 规范化镜像URL
	address := channelvars.NormalizeImageURL(req.Address)

	// 解析地址格式
	parts := strings.Split(address, "/")
	if len(parts) < 2 {
		return &channelvars.ResolveAddressResponse{
			Resolved: false,
			Reason:   "invalid image address format",
		}, nil
	}

	host := parts[0]
	project := parts[1]

	// 查找匹配的渠道实例
	instances, err := p.channelManager.FindInstancesByEndpoint(ctx, host, req.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("find instances by endpoint: %w", err)
	}

	if len(instances) == 0 {
		return &channelvars.ResolveAddressResponse{
			Resolved: false,
			Reason:   fmt.Sprintf("no channel instance found for host: %s", host),
		}, nil
	}

	fields := map[string]string{
		"dynamic.project":    project,
		"dynamic.registry":   project,
		"address.projectUrl": host + "/" + project,
	}

	// 如果有镜像名称
	if len(parts) >= 3 {
		imagePart := parts[2]

		// 分离镜像名和标签
		imageTag := strings.Split(imagePart, ":")
		image := imageTag[0]
		fields["dynamic.image"] = image
		fields["address.imageUrl"] = host + "/" + project + "/" + image

		if len(imageTag) > 1 {
			tag := imageTag[1]
			fields["dynamic.tag"] = tag
			fields["dynamic.imageTag"] = image + ":" + tag
			fields["address.imageTagUrl"] = host + "/" + project + "/" + image + ":" + tag
		}
	}

	return &channelvars.ResolveAddressResponse{
		Resolved:          true,
		ChannelInstanceID: instances[0].ID,
		Fields:            fields,
	}, nil
}

// RenderOutput 渲染输出
func (p *HarborProvider) RenderOutput(ctx context.Context, req *channelvars.RenderOutputRequest) (*channelvars.RenderOutputResponse, error) {
	return nil, fmt.Errorf("harbor provider does not support render output")
}

// ValidateValue 校验值
func (p *HarborProvider) ValidateValue(ctx context.Context, req *channelvars.ValidateValueRequest) error {
	return nil
}

func imageProjectValue(parentValues map[string]string) string {
	if parentValues == nil {
		return ""
	}
	if value := strings.TrimSpace(parentValues[channelvars.FieldDynamicProject]); value != "" {
		return value
	}
	if value := strings.TrimSpace(parentValues[channelvars.FieldDynamicRegistry]); value != "" {
		return value
	}
	if value := strings.TrimSpace(parentValues[channelvars.FieldDynamicRepository]); value != "" {
		return value
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

func imageNameValue(parentValues map[string]string) string {
	if parentValues == nil {
		return ""
	}
	if value := strings.TrimSpace(parentValues[channelvars.FieldDynamicImage]); value != "" {
		return value
	}
	for _, field := range []string{channelvars.FieldAddressImageURL, channelvars.FieldAddressImageTagURL} {
		if _, image, _ := splitImageAddress(parentValues[field]); image != "" {
			return image
		}
	}
	return ""
}

func splitImageAddress(address string) (string, string, string) {
	address = channelvars.NormalizeImageURL(strings.TrimSpace(address))
	parts := strings.Split(address, "/")
	if len(parts) < 2 {
		return "", "", ""
	}
	project := strings.TrimSpace(parts[1])
	if len(parts) < 3 {
		return project, "", ""
	}
	imagePart := strings.Join(parts[2:], "/")
	tag := ""
	if index := strings.LastIndex(imagePart, ":"); index >= 0 {
		tag = imagePart[index+1:]
		imagePart = imagePart[:index]
	}
	return project, strings.TrimSpace(imagePart), strings.TrimSpace(tag)
}

// getHarborClient 获取 Harbor 客户端
func (p *HarborProvider) getHarborClient(instance *channelvars.ChannelInstance) (*harbor.Client, error) {
	// 从配置中获取认证信息
	username, _ := instance.Config["username"].(string)
	password, _ := instance.Config["password"].(string)

	// 获取 insecureSkipTls 配置
	insecure := false
	if val, ok := instance.Config["insecureSkipTls"].(bool); ok {
		insecure = val
	}

	// 创建 Harbor 客户端
	client := harbor.NewClient(instance.Endpoint, username, password, insecure)

	return client, nil
}
