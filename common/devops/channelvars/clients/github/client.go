package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v57/github"
	"golang.org/x/oauth2"
)

// Client GitHub API客户端
type Client struct {
	client   *github.Client
	endpoint string
}

// NewClient 创建GitHub客户端
func NewClient(endpoint, token string) (*Client, error) {
	// 规范化endpoint
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = "https://" + endpoint
	}

	// 创建OAuth2客户端
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	var client *github.Client
	var err error

	// 判断是GitHub还是GitHub Enterprise
	if strings.Contains(endpoint, "github.com") {
		client = github.NewClient(tc)
	} else {
		client, err = github.NewEnterpriseClient(endpoint, endpoint, tc)
		if err != nil {
			return nil, fmt.Errorf("create github enterprise client: %w", err)
		}
	}

	return &Client{
		client:   client,
		endpoint: endpoint,
	}, nil
}

// Repository 仓库信息
type Repository struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	FullName    string `json:"fullName"`
	HTMLURL     string `json:"htmlUrl"`
	Description string `json:"description"`
	Owner       string `json:"owner"`
}

// Branch 分支信息
type Branch struct {
	Name      string `json:"name"`
	Protected bool   `json:"protected"`
}

// Tag 标签信息
type Tag struct {
	Name string `json:"name"`
}

// ListRepositories 查询仓库列表
func (c *Client) ListRepositories(ctx context.Context, search string, page, pageSize int) ([]*Repository, error) {
	opts := &github.RepositoryListOptions{
		ListOptions: github.ListOptions{
			Page:    page,
			PerPage: pageSize,
		},
	}

	repos, _, err := c.client.Repositories.List(ctx, "", opts)
	if err != nil {
		return nil, fmt.Errorf("list repositories: %w", err)
	}

	result := make([]*Repository, 0)
	for _, r := range repos {
		// 如果有搜索条件，进行过滤
		if search != "" && !strings.Contains(strings.ToLower(r.GetFullName()), strings.ToLower(search)) {
			continue
		}

		result = append(result, &Repository{
			ID:          r.GetID(),
			Name:        r.GetName(),
			FullName:    r.GetFullName(),
			HTMLURL:     r.GetHTMLURL(),
			Description: r.GetDescription(),
			Owner:       r.GetOwner().GetLogin(),
		})
	}

	return result, nil
}

// GetRepository 获取仓库详情
func (c *Client) GetRepository(ctx context.Context, owner, repo string) (*Repository, error) {
	repository, _, err := c.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("get repository: %w", err)
	}

	return &Repository{
		ID:          repository.GetID(),
		Name:        repository.GetName(),
		FullName:    repository.GetFullName(),
		HTMLURL:     repository.GetHTMLURL(),
		Description: repository.GetDescription(),
		Owner:       repository.GetOwner().GetLogin(),
	}, nil
}

// GetRepositoryByPath 根据路径获取仓库
func (c *Client) GetRepositoryByPath(ctx context.Context, path string) (*Repository, error) {
	// path格式: owner/repo
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repository path: %s", path)
	}

	return c.GetRepository(ctx, parts[0], parts[1])
}

// ListBranches 查询分支列表
func (c *Client) ListBranches(ctx context.Context, owner, repo string) ([]*Branch, error) {
	opts := &github.BranchListOptions{}

	branches, _, err := c.client.Repositories.ListBranches(ctx, owner, repo, opts)
	if err != nil {
		return nil, fmt.Errorf("list branches: %w", err)
	}

	result := make([]*Branch, len(branches))
	for i, b := range branches {
		result[i] = &Branch{
			Name:      b.GetName(),
			Protected: b.GetProtected(),
		}
	}

	return result, nil
}

// ListTags 查询标签列表
func (c *Client) ListTags(ctx context.Context, owner, repo string) ([]*Tag, error) {
	opts := &github.ListOptions{}

	tags, _, err := c.client.Repositories.ListTags(ctx, owner, repo, opts)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}

	result := make([]*Tag, len(tags))
	for i, t := range tags {
		result[i] = &Tag{
			Name: t.GetName(),
		}
	}

	return result, nil
}

// TestConnection 测试连接
func (c *Client) TestConnection(ctx context.Context) error {
	_, _, err := c.client.Users.Get(ctx, "")
	if err != nil {
		return fmt.Errorf("test connection: %w", err)
	}

	return nil
}
