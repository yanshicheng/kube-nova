package providers

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
	_ "github.com/yanshicheng/kube-nova/common/devops/channelvars/clients/gitee"
	"github.com/yanshicheng/kube-nova/common/devops/channelvars/clients/github"
	"github.com/yanshicheng/kube-nova/common/devops/channelvars/clients/gitlab"
)

// GitProvider Git Provider实现
type GitProvider struct {
	channelManager channelvars.ChannelManager
}

// NewGitProvider 创建Git Provider
func NewGitProvider(channelManager channelvars.ChannelManager) *GitProvider {
	return &GitProvider{
		channelManager: channelManager,
	}
}

// Capabilities 声明能力
func (p *GitProvider) Capabilities() channelvars.ProviderCapabilities {
	return channelvars.ProviderCapabilities{
		SupportedQueries: []string{
			"git.project",
			"git.branch",
			"git.tag",
			"git.projectUrl",
		},
		SupportedAddressFormats: []channelvars.AddressFormat{
			{
				Field:   "address.projectUrl",
				Pattern: `^https?://[^/]+/[^/]+/[^/]+(\.git)?$`,
				Example: "https://gitlab.com/group/project.git",
			},
		},
		SupportsAddressResolve: true,
		RequiresCredential:     true,
	}
}

// QueryOptions 查询选项
func (p *GitProvider) QueryOptions(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	switch req.ProviderKey {
	case "git.project":
		return p.queryProjects(ctx, req)
	case "git.branch":
		return p.queryBranches(ctx, req)
	case "git.tag":
		return p.queryTags(ctx, req)
	case "git.projectUrl":
		return p.queryCascaderProjects(ctx, req)
	default:
		return nil, fmt.Errorf("unsupported provider key: %s", req.ProviderKey)
	}
}

// queryProjects 查询项目列表
func (p *GitProvider) queryProjects(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	// 获取渠道实例
	instance, err := p.channelManager.GetInstance(ctx, req.ChannelInstanceID)
	if err != nil {
		return nil, fmt.Errorf("get channel instance: %w", err)
	}

	// 获取搜索关键词
	search := ""
	if req.Search != nil {
		search = *req.Search
	}

	// 根据不同的Git类型调用不同的API
	var options []channelvars.Option

	switch instance.ChannelType {
	case "gitlab":
		options, err = p.queryGitLabProjects(ctx, instance, search)
	case "github":
		options, err = p.queryGitHubProjects(ctx, instance, search)
	case "gitee":
		options, err = p.queryGiteeProjects(ctx, instance, search)
	default:
		return nil, fmt.Errorf("unsupported git type: %s", instance.ChannelType)
	}

	if err != nil {
		return nil, err
	}

	return &channelvars.QueryOptionsResponse{
		Options: options,
	}, nil
}

// queryBranches 查询分支列表
func (p *GitProvider) queryBranches(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	// 优先从 dynamic.project 获取项目ID
	projectID := req.ParentValues["dynamic.project"]

	// Fallback: 如果没有 dynamic.project，尝试从 address.projectUrl 解析
	if projectID == "" {
		projectURL := req.ParentValues["address.projectUrl"]
		if projectURL != "" {
			// 解析地址获取 endpoint 和 project
			resolvedProject, resolvedEndpoint, err := p.resolveProjectFromURL(ctx, projectURL, req.ProjectID)
			if err != nil {
				return nil, fmt.Errorf("resolve project from URL: %w", err)
			}
			projectID = resolvedProject

			// 如果没有传 channelInstanceId，需要根据 endpoint 查找实例
			if req.ChannelInstanceID == 0 {
				instances, err := p.channelManager.FindInstancesByEndpoint(ctx, resolvedEndpoint, req.ProjectID)
				if err != nil || len(instances) == 0 {
					return nil, fmt.Errorf("no channel instance found for endpoint: %s", resolvedEndpoint)
				}
				req.ChannelInstanceID = instances[0].ID
			}
		}
	}

	if projectID == "" {
		return nil, fmt.Errorf("dynamic.project or address.projectUrl is required")
	}

	// 获取渠道实例
	instance, err := p.channelManager.GetInstance(ctx, req.ChannelInstanceID)
	if err != nil {
		return nil, fmt.Errorf("get channel instance: %w", err)
	}

	// 获取搜索关键词
	search := ""
	if req.Search != nil {
		search = *req.Search
	}

	// 根据不同的Git类型调用不同的API
	var options []channelvars.Option

	switch instance.ChannelType {
	case "gitlab":
		options, err = p.queryGitLabBranches(ctx, instance, projectID, search)
	case "github":
		options, err = p.queryGitHubBranches(ctx, instance, projectID, search)
	case "gitee":
		options, err = p.queryGiteeBranches(ctx, instance, projectID, search)
	default:
		return nil, fmt.Errorf("unsupported git type: %s", instance.ChannelType)
	}

	if err != nil {
		return nil, err
	}

	return &channelvars.QueryOptionsResponse{
		Options: options,
	}, nil
}

// queryTags 查询标签列表
func (p *GitProvider) queryTags(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	// 优先从 dynamic.project 获取项目ID
	projectID := req.ParentValues["dynamic.project"]

	// Fallback: 如果没有 dynamic.project，尝试从 address.projectUrl 解析
	if projectID == "" {
		projectURL := req.ParentValues["address.projectUrl"]
		if projectURL != "" {
			// 解析地址获取 endpoint 和 project
			resolvedProject, resolvedEndpoint, err := p.resolveProjectFromURL(ctx, projectURL, req.ProjectID)
			if err != nil {
				return nil, fmt.Errorf("resolve project from URL: %w", err)
			}
			projectID = resolvedProject

			// 如果没有传 channelInstanceId，需要根据 endpoint 查找实例
			if req.ChannelInstanceID == 0 {
				instances, err := p.channelManager.FindInstancesByEndpoint(ctx, resolvedEndpoint, req.ProjectID)
				if err != nil || len(instances) == 0 {
					return nil, fmt.Errorf("no channel instance found for endpoint: %s", resolvedEndpoint)
				}
				req.ChannelInstanceID = instances[0].ID
			}
		}
	}

	if projectID == "" {
		return nil, fmt.Errorf("dynamic.project or address.projectUrl is required")
	}

	// 获取渠道实例
	instance, err := p.channelManager.GetInstance(ctx, req.ChannelInstanceID)
	if err != nil {
		return nil, fmt.Errorf("get channel instance: %w", err)
	}

	// 获取搜索关键词
	search := ""
	if req.Search != nil {
		search = *req.Search
	}

	// 根据不同的Git类型调用不同的API
	var options []channelvars.Option

	switch instance.ChannelType {
	case "gitlab":
		options, err = p.queryGitLabTags(ctx, instance, projectID, search)
	case "github":
		options, err = p.queryGitHubTags(ctx, instance, projectID, search)
	case "gitee":
		options, err = p.queryGiteeTags(ctx, instance, projectID, search)
	default:
		return nil, fmt.Errorf("unsupported git type: %s", instance.ChannelType)
	}

	if err != nil {
		return nil, err
	}

	return &channelvars.QueryOptionsResponse{
		Options: options,
	}, nil
}

// queryCascaderProjects 查询级联选择器项目列表
func (p *GitProvider) queryCascaderProjects(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	// 级联选择器：Git实例 -> Git仓库
	// 第一级已经通过endpoint选择，这里返回第二级（项目列表）
	return p.queryProjects(ctx, req)
}

// ResolveAddress 解析地址
func (p *GitProvider) ResolveAddress(ctx context.Context, req *channelvars.ResolveAddressRequest) (*channelvars.ResolveAddressResponse, error) {
	// 解析 https://gitlab.com/group/project.git

	// 1. 规范化URL
	address := channelvars.NormalizeGitURL(req.Address)

	// 2. 提取域名和路径
	u, err := url.Parse(address)
	if err != nil {
		return &channelvars.ResolveAddressResponse{
			Resolved: false,
			Reason:   fmt.Sprintf("invalid URL: %v", err),
		}, nil
	}

	host := u.Host
	path := strings.TrimPrefix(u.Path, "/")

	// 3. 查找匹配的渠道实例
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

	// 4. 查询项目（通过API）
	for _, instance := range instances {
		// TODO: 实际调用Git API查询项目
		// 这里需要根据path查询项目ID
		// 暂时返回模拟数据
		projectID := p.extractProjectIDFromPath(path)

		if projectID != "" {
			return &channelvars.ResolveAddressResponse{
				Resolved:          true,
				ChannelInstanceID: instance.ID,
				Fields: map[string]string{
					"dynamic.project": projectID,
				},
			}, nil
		}
	}

	return &channelvars.ResolveAddressResponse{
		Resolved: false,
		Reason:   fmt.Sprintf("project not found in any instance: %s", path),
	}, nil
}

// extractProjectIDFromPath 从路径提取项目ID
func (p *GitProvider) extractProjectIDFromPath(path string) string {
	// TODO: 实际实现需要调用Git API根据路径查询项目ID
	// 这里返回模拟数据
	return "1"
}

// RenderOutput 渲染输出
func (p *GitProvider) RenderOutput(ctx context.Context, req *channelvars.RenderOutputRequest) (*channelvars.RenderOutputResponse, error) {
	// Git Provider不需要渲染输出
	return nil, fmt.Errorf("git provider does not support render output")
}

// ValidateValue 校验值
func (p *GitProvider) ValidateValue(ctx context.Context, req *channelvars.ValidateValueRequest) error {
	// TODO: 实现值校验逻辑
	return nil
}

// resolveProjectFromURL 从 URL 解析项目信息
func (p *GitProvider) resolveProjectFromURL(ctx context.Context, projectURL string, projectID int64) (project string, endpoint string, error error) {
	// 规范化 URL
	projectURL = channelvars.NormalizeGitURL(projectURL)

	// 解析 URL
	u, err := url.Parse(projectURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid URL: %w", err)
	}

	// 提取 endpoint (host)
	endpoint = u.Host

	// 提取 project (path)
	path := strings.TrimPrefix(u.Path, "/")
	path = strings.TrimSuffix(path, ".git")

	// 对于 GitLab/GitHub/Gitee，project 就是路径
	// 例如：https://gitlab.com/group/project -> group/project
	project = path

	return project, endpoint, nil
}

// BuildProjectURL 构建项目URL
func (p *GitProvider) BuildProjectURL(instance *channelvars.ChannelInstance, projectID string) (string, error) {
	// TODO: 根据不同的Git类型构建URL
	// GitLab: https://gitlab.com/group/project
	// GitHub: https://github.com/owner/repo
	// Gitee: https://gitee.com/owner/repo

	endpoint := instance.Endpoint
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = "https://" + endpoint
	}

	// 这里需要根据projectID查询项目路径
	// 暂时返回模拟数据
	projectPath := "group/project"

	return fmt.Sprintf("%s/%s", strings.TrimSuffix(endpoint, "/"), projectPath), nil
}

// ParseProjectURL 解析项目URL
func (p *GitProvider) ParseProjectURL(projectURL string) (host string, path string, err error) {
	u, err := url.Parse(projectURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid URL: %w", err)
	}

	host = u.Host
	path = strings.TrimPrefix(u.Path, "/")
	path = strings.TrimSuffix(path, ".git")

	return host, path, nil
}

// queryGitLabProjects 查询GitLab项目列表
func (p *GitProvider) queryGitLabProjects(ctx context.Context, instance *channelvars.ChannelInstance, search string) ([]channelvars.Option, error) {
	client, err := p.getGitLabClient(instance)
	if err != nil {
		return nil, err
	}

	// 调用GitLab API
	projects, err := client.ListProjects(ctx, search, 1, 50)
	if err != nil {
		return nil, fmt.Errorf("list gitlab projects: %w", err)
	}

	// 转换为选项列表
	options := make([]channelvars.Option, 0, len(projects))
	for _, project := range projects {
		options = append(options, channelvars.Option{
			Label: project.PathWithNamespace,
			Value: strconv.FormatInt(project.ID, 10),
		})
	}

	return options, nil
}

// queryGitHubProjects 查询GitHub项目列表
func (p *GitProvider) queryGitHubProjects(ctx context.Context, instance *channelvars.ChannelInstance, search string) ([]channelvars.Option, error) {
	client, err := p.getGitHubClient(instance)
	if err != nil {
		return nil, err
	}

	// 调用GitHub API
	repos, err := client.ListRepositories(ctx, search, 1, 50)
	if err != nil {
		return nil, fmt.Errorf("list github repositories: %w", err)
	}

	// 转换为选项列表
	options := make([]channelvars.Option, 0, len(repos))
	for _, repo := range repos {
		options = append(options, channelvars.Option{
			Label: repo.FullName,
			Value: strconv.FormatInt(repo.ID, 10),
		})
	}

	return options, nil
}

// queryGiteeProjects 查询Gitee项目列表
func (p *GitProvider) queryGiteeProjects(ctx context.Context, instance *channelvars.ChannelInstance, search string) ([]channelvars.Option, error) {
	// TODO: 实现Gitee API调用
	// Gitee API文档: https://gitee.com/api/v5/swagger
	return nil, fmt.Errorf("gitee provider not implemented yet")
}

// queryGitLabBranches 查询GitLab分支列表
func (p *GitProvider) queryGitLabBranches(ctx context.Context, instance *channelvars.ChannelInstance, projectID string, search string) ([]channelvars.Option, error) {
	client, err := p.getGitLabClient(instance)
	if err != nil {
		return nil, err
	}

	// 调用GitLab API
	branches, err := client.ListBranches(ctx, projectID, search)
	if err != nil {
		return nil, fmt.Errorf("list gitlab branches: %w", err)
	}

	// 转换为选项列表
	options := make([]channelvars.Option, 0, len(branches))
	for _, branch := range branches {
		options = append(options, channelvars.Option{
			Label: branch.Name,
			Value: branch.Name,
		})
	}

	return options, nil
}

// queryGitHubBranches 查询GitHub分支列表
func (p *GitProvider) queryGitHubBranches(ctx context.Context, instance *channelvars.ChannelInstance, projectID string, search string) ([]channelvars.Option, error) {
	client, err := p.getGitHubClient(instance)
	if err != nil {
		return nil, err
	}

	// GitHub的projectID格式为 "owner/repo"
	parts := strings.Split(projectID, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid github project id format: %s", projectID)
	}
	owner, repo := parts[0], parts[1]

	// 调用GitHub API
	branches, err := client.ListBranches(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("list github branches: %w", err)
	}

	// 如果有搜索条件，过滤分支
	if search != "" {
		filtered := make([]*github.Branch, 0)
		for _, branch := range branches {
			if strings.Contains(branch.Name, search) {
				filtered = append(filtered, branch)
			}
		}
		branches = filtered
	}

	// 转换为选项列表
	options := make([]channelvars.Option, 0, len(branches))
	for _, branch := range branches {
		options = append(options, channelvars.Option{
			Label: branch.Name,
			Value: branch.Name,
		})
	}

	return options, nil
}

// queryGiteeBranches 查询Gitee分支列表
func (p *GitProvider) queryGiteeBranches(ctx context.Context, instance *channelvars.ChannelInstance, projectID string, search string) ([]channelvars.Option, error) {
	// TODO: 实现Gitee API调用
	return nil, fmt.Errorf("gitee provider not implemented yet")
}

// queryGitLabTags 查询GitLab标签列表
func (p *GitProvider) queryGitLabTags(ctx context.Context, instance *channelvars.ChannelInstance, projectID string, search string) ([]channelvars.Option, error) {
	client, err := p.getGitLabClient(instance)
	if err != nil {
		return nil, err
	}

	// 调用GitLab API
	tags, err := client.ListTags(ctx, projectID, search)
	if err != nil {
		return nil, fmt.Errorf("list gitlab tags: %w", err)
	}

	// 转换为选项列表
	options := make([]channelvars.Option, 0, len(tags))
	for _, tag := range tags {
		options = append(options, channelvars.Option{
			Label: tag.Name,
			Value: tag.Name,
		})
	}

	return options, nil
}

// queryGitHubTags 查询GitHub标签列表
func (p *GitProvider) queryGitHubTags(ctx context.Context, instance *channelvars.ChannelInstance, projectID string, search string) ([]channelvars.Option, error) {
	client, err := p.getGitHubClient(instance)
	if err != nil {
		return nil, err
	}

	// GitHub的projectID格式为 "owner/repo"
	parts := strings.Split(projectID, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid github project id format: %s", projectID)
	}
	owner, repo := parts[0], parts[1]

	// 调用GitHub API
	tags, err := client.ListTags(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("list github tags: %w", err)
	}

	// 如果有搜索条件，过滤标签
	if search != "" {
		filtered := make([]*github.Tag, 0)
		for _, tag := range tags {
			if strings.Contains(tag.Name, search) {
				filtered = append(filtered, tag)
			}
		}
		tags = filtered
	}

	// 转换为选项列表
	options := make([]channelvars.Option, 0, len(tags))
	for _, tag := range tags {
		options = append(options, channelvars.Option{
			Label: tag.Name,
			Value: tag.Name,
		})
	}

	return options, nil
}

// queryGiteeTags 查询Gitee标签列表
func (p *GitProvider) queryGiteeTags(ctx context.Context, instance *channelvars.ChannelInstance, projectID string, search string) ([]channelvars.Option, error) {
	// TODO: 实现Gitee API调用
	return nil, fmt.Errorf("gitee provider not implemented yet")
}

// getGitLabClient 获取GitLab客户端
func (p *GitProvider) getGitLabClient(instance *channelvars.ChannelInstance) (*gitlab.Client, error) {
	// 从配置中获取token
	token, ok := instance.Config["token"].(string)
	if !ok || token == "" {
		return nil, fmt.Errorf("gitlab token not found in config")
	}

	// 创建GitLab客户端
	baseURL := "https://gitlab.com"
	if instance.Endpoint != "" && instance.Endpoint != "gitlab.com" {
		baseURL = instance.Endpoint
		if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
			baseURL = "https://" + baseURL
		}
	}

	client, err := gitlab.NewClient(token, baseURL)
	if err != nil {
		return nil, fmt.Errorf("create gitlab client: %w", err)
	}

	return client, nil
}

// getGitHubClient 获取GitHub客户端
func (p *GitProvider) getGitHubClient(instance *channelvars.ChannelInstance) (*github.Client, error) {
	// 从配置中获取token
	token, ok := instance.Config["token"].(string)
	if !ok || token == "" {
		return nil, fmt.Errorf("github token not found in config")
	}

	// 创建GitHub客户端
	var endpoint string
	if instance.Endpoint != "" && instance.Endpoint != "github.com" {
		// GitHub Enterprise
		endpoint = instance.Endpoint
		if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
			endpoint = "https://" + endpoint
		}
	} else {
		// 使用默认的 github.com API endpoint
		endpoint = "https://api.github.com"
	}

	client, err := github.NewClient(endpoint, token)
	if err != nil {
		return nil, fmt.Errorf("create github client: %w", err)
	}

	return client, nil
}

// GetProjectID 根据路径获取项目ID
func (p *GitProvider) GetProjectID(ctx context.Context, instance *channelvars.ChannelInstance, projectPath string) (string, error) {
	// TODO: 实际调用Git API根据路径查询项目ID
	// GitLab: GET /api/v4/projects/:path_encoded
	// GitHub: GET /repos/:owner/:repo
	// Gitee: GET /api/v5/repos/:owner/:repo

	return "1", nil
}

// init 注册Provider
func init() {
	// 注册到全局Provider注册表
	// 实际使用时需要在应用启动时注册
	// channelvars.GetProviderRegistry().Register("gitlab", NewGitProvider(channelManager))
	// channelvars.GetProviderRegistry().Register("github", NewGitProvider(channelManager))
	// channelvars.GetProviderRegistry().Register("gitee", NewGitProvider(channelManager))
}
