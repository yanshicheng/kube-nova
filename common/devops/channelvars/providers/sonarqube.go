package providers

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
	"github.com/yanshicheng/kube-nova/common/devops/channelvars/clients/sonarqube"
)

// SonarQubeProvider SonarQube Provider实现
type SonarQubeProvider struct {
	channelManager channelvars.ChannelManager
}

// NewSonarQubeProvider 创建SonarQube Provider
func NewSonarQubeProvider(channelManager channelvars.ChannelManager) *SonarQubeProvider {
	return &SonarQubeProvider{
		channelManager: channelManager,
	}
}

// Capabilities 声明能力
func (p *SonarQubeProvider) Capabilities() channelvars.ProviderCapabilities {
	return channelvars.ProviderCapabilities{
		SupportedQueries: []string{
			"scan.projectName",
			"scan.project",
			"scan.projectKey",
		},
		SupportedAddressFormats: []channelvars.AddressFormat{
			{
				Field:   "address.projectUrl",
				Pattern: `^https?://[^/]+/dashboard\?id=.+$`,
				Example: "https://sonarqube.example.com/dashboard?id=my-project",
			},
		},
		SupportsAddressResolve: true,
		RequiresCredential:     true,
	}
}

// QueryOptions 查询选项
func (p *SonarQubeProvider) QueryOptions(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	switch req.ProviderKey {
	case "scan.projectName", "scan.project":
		return p.queryProjects(ctx, req)
	case "scan.projectKey":
		return p.queryProjectKeys(ctx, req)
	default:
		return nil, fmt.Errorf("unsupported provider key: %s", req.ProviderKey)
	}
}

// queryProjects 查询扫描项目列表
func (p *SonarQubeProvider) queryProjects(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	instance, err := p.channelManager.GetInstance(ctx, req.ChannelInstanceID)
	if err != nil {
		return nil, fmt.Errorf("get channel instance: %w", err)
	}

	// 创建 SonarQube 客户端
	client, err := p.getSonarQubeClient(instance)
	if err != nil {
		return nil, err
	}

	// 获取搜索关键词
	search := ""
	if req.Search != nil {
		search = *req.Search
	}

	// 调用 SonarQube API 查询项目列表
	projects, err := client.ListProjects(ctx, search, 1, 50)
	if err != nil {
		return nil, fmt.Errorf("list sonarqube projects: %w", err)
	}

	// 转换为选项列表
	options := make([]channelvars.Option, 0, len(projects))
	for _, project := range projects {
		label := project.Name
		if project.Key != "" {
			label = fmt.Sprintf("%s (%s)", project.Name, project.Key)
		}
		options = append(options, channelvars.Option{
			Label: label,
			Value: project.Key,
		})
	}

	return &channelvars.QueryOptionsResponse{
		Options: options,
	}, nil
}

// queryProjectKeys 查询项目Key列表
func (p *SonarQubeProvider) queryProjectKeys(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	// 项目Key和项目是一对一的关系，返回相同的列表
	return p.queryProjects(ctx, req)
}

// ResolveAddress 解析地址
func (p *SonarQubeProvider) ResolveAddress(ctx context.Context, req *channelvars.ResolveAddressRequest) (*channelvars.ResolveAddressResponse, error) {
	// 解析 https://sonarqube.example.com/dashboard?id=my-project

	// 提取host
	host, err := channelvars.ExtractHost(req.Address)
	if err != nil {
		return &channelvars.ResolveAddressResponse{
			Resolved: false,
			Reason:   fmt.Sprintf("invalid URL: %v", err),
		}, nil
	}

	// 提取项目Key
	projectKey := p.extractProjectKey(req.Address)
	if projectKey == "" {
		return &channelvars.ResolveAddressResponse{
			Resolved: false,
			Reason:   "project key not found in URL",
		}, nil
	}

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

	return &channelvars.ResolveAddressResponse{
		Resolved:          true,
		ChannelInstanceID: instances[0].ID,
		Fields: map[string]string{
			channelvars.FieldDynamicProjectName: projectKey,
			channelvars.FieldDynamicProjectKey:  projectKey,
			channelvars.FieldDynamicProject:     projectKey,
		},
	}, nil
}

// extractProjectKey 从URL提取项目Key
func (p *SonarQubeProvider) extractProjectKey(address string) string {
	// 从 ?id=xxx 提取项目Key
	if idx := strings.Index(address, "?id="); idx != -1 {
		projectKey := address[idx+4:]
		// 移除可能的其他查询参数
		if idx := strings.Index(projectKey, "&"); idx != -1 {
			projectKey = projectKey[:idx]
		}
		return projectKey
	}
	return ""
}

// RenderOutput 渲染输出
func (p *SonarQubeProvider) RenderOutput(ctx context.Context, req *channelvars.RenderOutputRequest) (*channelvars.RenderOutputResponse, error) {
	return nil, fmt.Errorf("sonarqube provider does not support render output")
}

// ValidateValue 校验值
func (p *SonarQubeProvider) ValidateValue(ctx context.Context, req *channelvars.ValidateValueRequest) error {
	return nil
}

// getSonarQubeClient 获取 SonarQube 客户端
func (p *SonarQubeProvider) getSonarQubeClient(instance *channelvars.ChannelInstance) (*sonarqube.Client, error) {
	// 从配置中获取 token
	token, _ := instance.Config["token"].(string)

	// 获取 insecureSkipTls 配置
	insecure := false
	if val, ok := instance.Config["insecureSkipTls"].(bool); ok {
		insecure = val
	}

	// 创建 SonarQube 客户端
	client := sonarqube.NewClient(instance.Endpoint, token, insecure)

	return client, nil
}
