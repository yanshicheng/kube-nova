package gitee

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client Gitee API客户端
type Client struct {
	endpoint   string
	token      string
	httpClient *http.Client
}

// NewClient 创建Gitee客户端
func NewClient(endpoint, token string, insecure bool) *Client {
	// 规范化endpoint
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = "https://" + endpoint
	}
	endpoint = strings.TrimSuffix(endpoint, "/")

	// 如果是公共Gitee，使用默认endpoint
	if endpoint == "https://gitee.com" || endpoint == "" {
		endpoint = "https://gitee.com"
	}

	// 创建HTTP客户端
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: insecure,
			},
		},
	}

	return &Client{
		endpoint:   endpoint,
		token:      token,
		httpClient: httpClient,
	}
}

// Repository 仓库信息
type Repository struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Path          string `json:"path"`
	HTMLURL       string `json:"html_url"`
	Description   string `json:"description"`
	Private       bool   `json:"private"`
	Owner         *Owner `json:"owner"`
	DefaultBranch string `json:"default_branch"`
}

// Owner 所有者信息
type Owner struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Name  string `json:"name"`
	Type  string `json:"type"`
}

// Branch 分支信息
type Branch struct {
	Name      string `json:"name"`
	Protected bool   `json:"protected"`
}

// Tag 标签信息
type Tag struct {
	Name    string `json:"name"`
	Message string `json:"message"`
	Commit  struct {
		SHA string `json:"sha"`
	} `json:"commit"`
}

// doRequest 执行HTTP请求
func (c *Client) doRequest(ctx context.Context, method, path string, query url.Values) ([]byte, error) {
	// 构建URL
	apiURL := fmt.Sprintf("%s/api/v5%s", c.endpoint, path)

	// 添加access_token
	if query == nil {
		query = url.Values{}
	}
	query.Set("access_token", c.token)
	apiURL = fmt.Sprintf("%s?%s", apiURL, query.Encode())

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, method, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	// 执行请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// 检查状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// ListRepositories 查询仓库列表
func (c *Client) ListRepositories(ctx context.Context, search string, page, pageSize int) ([]*Repository, error) {
	query := url.Values{}
	query.Set("page", fmt.Sprintf("%d", page))
	query.Set("per_page", fmt.Sprintf("%d", pageSize))
	query.Set("type", "all")
	if search != "" {
		query.Set("q", search)
	}

	body, err := c.doRequest(ctx, http.MethodGet, "/user/repos", query)
	if err != nil {
		return nil, err
	}

	var repos []*Repository
	if err := json.Unmarshal(body, &repos); err != nil {
		return nil, fmt.Errorf("unmarshal repositories: %w", err)
	}

	return repos, nil
}

// GetRepository 获取仓库详情
func (c *Client) GetRepository(ctx context.Context, owner, repo string) (*Repository, error) {
	path := fmt.Sprintf("/repos/%s/%s", url.PathEscape(owner), url.PathEscape(repo))

	body, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var repository Repository
	if err := json.Unmarshal(body, &repository); err != nil {
		return nil, fmt.Errorf("unmarshal repository: %w", err)
	}

	return &repository, nil
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
	path := fmt.Sprintf("/repos/%s/%s/branches", url.PathEscape(owner), url.PathEscape(repo))

	body, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var branches []*Branch
	if err := json.Unmarshal(body, &branches); err != nil {
		return nil, fmt.Errorf("unmarshal branches: %w", err)
	}

	return branches, nil
}

// ListTags 查询标签列表
func (c *Client) ListTags(ctx context.Context, owner, repo string) ([]*Tag, error) {
	path := fmt.Sprintf("/repos/%s/%s/tags", url.PathEscape(owner), url.PathEscape(repo))

	body, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var tags []*Tag
	if err := json.Unmarshal(body, &tags); err != nil {
		return nil, fmt.Errorf("unmarshal tags: %w", err)
	}

	return tags, nil
}

// TestConnection 测试连接
func (c *Client) TestConnection(ctx context.Context) error {
	_, err := c.doRequest(ctx, http.MethodGet, "/user", nil)
	return err
}
