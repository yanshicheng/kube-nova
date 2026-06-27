package providers

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
	"github.com/yanshicheng/kube-nova/common/devops/channelvars/clients/nexus"
)

// NexusProvider Nexus Provider实现
type NexusProvider struct {
	channelManager channelvars.ChannelManager
}

// NewNexusProvider 创建Nexus Provider
func NewNexusProvider(channelManager channelvars.ChannelManager) *NexusProvider {
	return &NexusProvider{
		channelManager: channelManager,
	}
}

// Capabilities 声明能力
func (p *NexusProvider) Capabilities() channelvars.ProviderCapabilities {
	return channelvars.ProviderCapabilities{
		SupportedQueries: []string{
			"artifact.repository",
			"artifact.artifact",
			"artifact.artifactName",
			"artifact.tag",
			"artifact.artifactVersion",
			"artifact.artifactTag",
		},
		SupportedAddressFormats: []channelvars.AddressFormat{
			{
				Field:   "address.repositoryUrl",
				Pattern: `^[^/]+/repository/[^/]+$`,
				Example: "nexus.example.com/repository/maven-releases",
			},
			{
				Field:   "address.artifactUrl",
				Pattern: `^[^/]+/repository/[^/]+/.+$`,
				Example: "nexus.example.com/repository/maven-releases/com/example/app",
			},
			{
				Field:   "address.artifactVersionUrl",
				Pattern: `^[^/]+/repository/[^/]+/.+:[^/]+$`,
				Example: "nexus.example.com/repository/maven-releases/com/example/app:1.0.0",
			},
		},
		SupportsAddressResolve: true,
		RequiresCredential:     true,
	}
}

// QueryOptions 查询选项
func (p *NexusProvider) QueryOptions(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	switch req.ProviderKey {
	case "artifact.repository":
		return p.queryRepositories(ctx, req)
	case "artifact.artifact", "artifact.artifactName":
		return p.queryArtifacts(ctx, req)
	case "artifact.tag", "artifact.artifactVersion":
		return p.queryTags(ctx, req)
	case "artifact.artifactTag":
		return p.queryArtifactTags(ctx, req)
	default:
		return nil, fmt.Errorf("unsupported provider key: %s", req.ProviderKey)
	}
}

// queryRepositories 查询制品仓库列表
func (p *NexusProvider) queryRepositories(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	instance, err := p.channelManager.GetInstance(ctx, req.ChannelInstanceID)
	if err != nil {
		return nil, fmt.Errorf("get channel instance: %w", err)
	}

	// 创建 Nexus 客户端
	client, err := p.getNexusClient(instance)
	if err != nil {
		return nil, err
	}

	// 调用 Nexus API 查询仓库列表
	repos, err := client.ListRepositories(ctx)
	if err != nil {
		return nil, fmt.Errorf("list nexus repositories: %w", err)
	}

	// 转换为选项列表
	options := make([]channelvars.Option, 0, len(repos))
	for _, repo := range repos {
		options = append(options, channelvars.Option{
			Label: repo.Name,
			Value: repo.Name,
		})
	}

	return &channelvars.QueryOptionsResponse{
		Options: options,
	}, nil
}

// queryArtifacts 查询制品列表
func (p *NexusProvider) queryArtifacts(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	// 优先从 dynamic.repository 获取
	repository := req.ParentValues["dynamic.repository"]

	// Fallback: 从 address.repositoryUrl 解析
	if repository == "" {
		repositoryURL := req.ParentValues["address.repositoryUrl"]
		if repositoryURL != "" {
			// 解析地址：nexus.example.com/repository/maven-releases -> maven-releases
			parts := strings.Split(repositoryURL, "/repository/")
			if len(parts) > 1 {
				repository = strings.Split(parts[1], "/")[0]
			}
		}
	}

	if repository == "" {
		return nil, fmt.Errorf("dynamic.repository or address.repositoryUrl is required")
	}

	instance, err := p.channelManager.GetInstance(ctx, req.ChannelInstanceID)
	if err != nil {
		return nil, fmt.Errorf("get channel instance: %w", err)
	}

	// 创建 Nexus 客户端
	client, err := p.getNexusClient(instance)
	if err != nil {
		return nil, err
	}

	// 获取搜索关键词
	search := ""
	if req.Search != nil {
		search = *req.Search
	}

	// 调用 Nexus API 查询制品列表
	components, err := client.ListComponentsByRepository(ctx, repository)
	if err != nil {
		return nil, fmt.Errorf("list nexus components: %w", err)
	}

	// 转换为选项列表，去重
	seen := make(map[string]bool)
	options := make([]channelvars.Option, 0)

	for _, comp := range components {
		if search != "" && !strings.Contains(comp.Name, search) {
			continue
		}

		if !seen[comp.Name] {
			seen[comp.Name] = true
			options = append(options, channelvars.Option{
				Label: comp.Name,
				Value: comp.Name,
			})
		}
	}

	return &channelvars.QueryOptionsResponse{
		Options: options,
	}, nil
}

// queryTags 查询制品版本列表
func (p *NexusProvider) queryTags(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	// 优先从 dynamic 字段获取
	repository := req.ParentValues["dynamic.repository"]
	artifact := firstParentValue(req.ParentValues, "dynamic.artifactName", "dynamic.artifact")

	// Fallback: 从 address.artifactUrl 解析
	if repository == "" || artifact == "" {
		artifactURL := req.ParentValues["address.artifactUrl"]
		if artifactURL != "" {
			// 解析地址：nexus.example.com/repository/maven-releases/com/example/app
			parts := strings.Split(artifactURL, "/repository/")
			if len(parts) > 1 {
				subParts := strings.Split(parts[1], "/")
				if len(subParts) > 1 {
					repository = subParts[0]
					artifact = strings.Join(subParts[1:], "/")
				}
			}
		}
	}

	if repository == "" || artifact == "" {
		return nil, fmt.Errorf("dynamic.repository+dynamic.artifactName or address.artifactUrl is required")
	}

	instance, err := p.channelManager.GetInstance(ctx, req.ChannelInstanceID)
	if err != nil {
		return nil, fmt.Errorf("get channel instance: %w", err)
	}

	// 创建 Nexus 客户端
	client, err := p.getNexusClient(instance)
	if err != nil {
		return nil, err
	}

	// 调用 Nexus API 查询版本列表
	versions, err := client.ListVersions(ctx, repository, artifact)
	if err != nil {
		return nil, fmt.Errorf("list nexus versions: %w", err)
	}

	// 转换为选项列表
	options := make([]channelvars.Option, 0, len(versions))
	for _, version := range versions {
		options = append(options, channelvars.Option{
			Label: version,
			Value: version,
		})
	}

	return &channelvars.QueryOptionsResponse{
		Options: options,
	}, nil
}

// queryArtifactTags 查询制品:版本组合
func (p *NexusProvider) queryArtifactTags(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	repository := req.ParentValues["dynamic.repository"]
	artifact := firstParentValue(req.ParentValues, "dynamic.artifactName", "dynamic.artifact")

	if repository == "" || artifact == "" {
		return nil, fmt.Errorf("dynamic.repository and dynamic.artifactName are required")
	}

	// 查询版本列表
	tagsResp, err := p.queryTags(ctx, req)
	if err != nil {
		return nil, err
	}

	// 组合成 artifact:version 格式
	options := make([]channelvars.Option, len(tagsResp.Options))
	for i, tag := range tagsResp.Options {
		options[i] = channelvars.Option{
			Label: fmt.Sprintf("%s:%s", artifact, tag.Label),
			Value: fmt.Sprintf("%s:%s", artifact, tag.Value),
		}
	}

	return &channelvars.QueryOptionsResponse{
		Options: options,
	}, nil
}

// ResolveAddress 解析地址
func (p *NexusProvider) ResolveAddress(ctx context.Context, req *channelvars.ResolveAddressRequest) (*channelvars.ResolveAddressResponse, error) {
	// 解析 nexus.example.com/repository/maven-releases/com/example/app/1.0.0

	parts := strings.Split(req.Address, "/")
	if len(parts) < 3 {
		return &channelvars.ResolveAddressResponse{
			Resolved: false,
			Reason:   "invalid artifact address format",
		}, nil
	}

	host := parts[0]
	repository := parts[2]

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
		"dynamic.repository": repository,
	}
	if len(parts) > 3 {
		artifactParts := parts[3:]
		if last := artifactParts[len(artifactParts)-1]; strings.Contains(last, ":") {
			name, version, _ := strings.Cut(last, ":")
			artifactParts[len(artifactParts)-1] = name
			fields["dynamic.artifactVersion"] = version
		}
		if artifact := strings.Trim(strings.Join(artifactParts, "/"), "/"); artifact != "" {
			fields["dynamic.artifactName"] = artifact
		}
	}

	return &channelvars.ResolveAddressResponse{
		Resolved:          true,
		ChannelInstanceID: instances[0].ID,
		Fields:            fields,
	}, nil
}

func firstParentValue(values map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(values[key]); value != "" {
			return value
		}
	}
	return ""
}

// RenderOutput 渲染输出
func (p *NexusProvider) RenderOutput(ctx context.Context, req *channelvars.RenderOutputRequest) (*channelvars.RenderOutputResponse, error) {
	return nil, fmt.Errorf("nexus provider does not support render output")
}

// ValidateValue 校验值
func (p *NexusProvider) ValidateValue(ctx context.Context, req *channelvars.ValidateValueRequest) error {
	return nil
}

// getNexusClient 获取 Nexus 客户端
func (p *NexusProvider) getNexusClient(instance *channelvars.ChannelInstance) (*nexus.Client, error) {
	// 从配置中获取认证信息
	username, _ := instance.Config["username"].(string)
	password, _ := instance.Config["password"].(string)

	// 获取 insecureSkipTls 配置
	insecure := false
	if val, ok := instance.Config["insecureSkipTls"].(bool); ok {
		insecure = val
	}

	// 创建 Nexus 客户端
	client := nexus.NewClient(instance.Endpoint, username, password, insecure)

	return client, nil
}
