package gitlab

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/xanzy/go-gitlab"
)

// Client GitLab API客户端
type Client struct {
	client   *gitlab.Client
	endpoint string
}

// NewClient 创建GitLab客户端
func NewClient(endpoint, token string) (*Client, error) {
	// 规范化endpoint
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = "https://" + endpoint
	}

	// 创建GitLab客户端
	client, err := gitlab.NewClient(token, gitlab.WithBaseURL(endpoint))
	if err != nil {
		return nil, fmt.Errorf("create gitlab client: %w", err)
	}

	return &Client{
		client:   client,
		endpoint: endpoint,
	}, nil
}

// Project 项目信息
type Project struct {
	ID                int64  `json:"id"`
	Name              string `json:"name"`
	Path              string `json:"path"`
	PathWithNamespace string `json:"pathWithNamespace"`
	WebURL            string `json:"webUrl"`
	Description       string `json:"description"`
}

// Branch 分支信息
type Branch struct {
	Name      string `json:"name"`
	Protected bool   `json:"protected"`
	Default   bool   `json:"default"`
}

// Tag 标签信息
type Tag struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}

// ListProjects 查询项目列表
func (c *Client) ListProjects(ctx context.Context, search string, page, pageSize int) ([]*Project, error) {
	opts := &gitlab.ListProjectsOptions{
		ListOptions: gitlab.ListOptions{
			Page:    page,
			PerPage: pageSize,
		},
		Search: gitlab.Ptr(search),
	}

	projects, _, err := c.client.Projects.ListProjects(opts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}

	result := make([]*Project, len(projects))
	for i, p := range projects {
		result[i] = &Project{
			ID:                int64(p.ID),
			Name:              p.Name,
			Path:              p.Path,
			PathWithNamespace: p.PathWithNamespace,
			WebURL:            p.WebURL,
			Description:       p.Description,
		}
	}

	return result, nil
}

// GetProject 获取项目详情
func (c *Client) GetProject(ctx context.Context, projectID interface{}) (*Project, error) {
	project, _, err := c.client.Projects.GetProject(projectID, nil, gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}

	return &Project{
		ID:                int64(project.ID),
		Name:              project.Name,
		Path:              project.Path,
		PathWithNamespace: project.PathWithNamespace,
		WebURL:            project.WebURL,
		Description:       project.Description,
	}, nil
}

// GetProjectByPath 根据路径获取项目
func (c *Client) GetProjectByPath(ctx context.Context, path string) (*Project, error) {
	// URL编码路径
	encodedPath := url.PathEscape(path)
	return c.GetProject(ctx, encodedPath)
}

// ListBranches 查询分支列表
func (c *Client) ListBranches(ctx context.Context, projectID interface{}, search string) ([]*Branch, error) {
	opts := &gitlab.ListBranchesOptions{
		Search: gitlab.Ptr(search),
	}

	branches, _, err := c.client.Branches.ListBranches(projectID, opts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("list branches: %w", err)
	}

	result := make([]*Branch, len(branches))
	for i, b := range branches {
		result[i] = &Branch{
			Name:      b.Name,
			Protected: b.Protected,
			Default:   b.Default,
		}
	}

	return result, nil
}

// ListTags 查询标签列表
func (c *Client) ListTags(ctx context.Context, projectID interface{}, search string) ([]*Tag, error) {
	opts := &gitlab.ListTagsOptions{
		Search: gitlab.Ptr(search),
	}

	tags, _, err := c.client.Tags.ListTags(projectID, opts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}

	result := make([]*Tag, len(tags))
	for i, t := range tags {
		result[i] = &Tag{
			Name:    t.Name,
			Message: t.Message,
		}
	}

	return result, nil
}

// TestConnection 测试连接
func (c *Client) TestConnection(ctx context.Context) error {
	_, resp, err := c.client.Version.GetVersion(gitlab.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("test connection: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
