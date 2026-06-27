package nexus

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

// Client Nexus API客户端
type Client struct {
	endpoint   string
	username   string
	password   string
	httpClient *http.Client
}

// NewClient 创建Nexus客户端
func NewClient(endpoint, username, password string, insecure bool) *Client {
	// 规范化endpoint
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = "https://" + endpoint
	}
	endpoint = strings.TrimSuffix(endpoint, "/")

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
		username:   username,
		password:   password,
		httpClient: httpClient,
	}
}

// Repository 仓库信息
type Repository struct {
	Name   string `json:"name"`
	Format string `json:"format"`
	Type   string `json:"type"`
	URL    string `json:"url"`
}

// Component 组件信息
type Component struct {
	ID         string            `json:"id"`
	Repository string            `json:"repository"`
	Format     string            `json:"format"`
	Group      string            `json:"group"`
	Name       string            `json:"name"`
	Version    string            `json:"version"`
	Assets     []*Asset          `json:"assets"`
	Tags       map[string]string `json:"tags"`
}

// Asset 资产信息
type Asset struct {
	ID           string `json:"id"`
	Path         string `json:"path"`
	DownloadURL  string `json:"downloadUrl"`
	Repository   string `json:"repository"`
	Format       string `json:"format"`
	ContentType  string `json:"contentType"`
	LastModified string `json:"lastModified"`
}

// SearchResult 搜索结果
type SearchResult struct {
	Items             []*Component `json:"items"`
	ContinuationToken string       `json:"continuationToken"`
}

// doRequest 执行HTTP请求
func (c *Client) doRequest(ctx context.Context, method, path string, query url.Values) ([]byte, error) {
	// 构建URL
	apiURL := fmt.Sprintf("%s/service/rest%s", c.endpoint, path)
	if query != nil {
		apiURL = fmt.Sprintf("%s?%s", apiURL, query.Encode())
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, method, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// 设置认证
	req.SetBasicAuth(c.username, c.password)
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
func (c *Client) ListRepositories(ctx context.Context) ([]*Repository, error) {
	body, err := c.doRequest(ctx, http.MethodGet, "/v1/repositories", nil)
	if err != nil {
		return nil, err
	}

	var repos []*Repository
	if err := json.Unmarshal(body, &repos); err != nil {
		return nil, fmt.Errorf("unmarshal repositories: %w", err)
	}

	return repos, nil
}

// SearchComponents 搜索组件
func (c *Client) SearchComponents(ctx context.Context, repository, name, version string) ([]*Component, error) {
	query := url.Values{}
	if repository != "" {
		query.Set("repository", repository)
	}
	if name != "" {
		query.Set("name", name)
	}
	if version != "" {
		query.Set("version", version)
	}

	body, err := c.doRequest(ctx, http.MethodGet, "/v1/search", query)
	if err != nil {
		return nil, err
	}

	var result SearchResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal search result: %w", err)
	}

	return result.Items, nil
}

// ListComponentsByRepository 查询仓库中的组件
func (c *Client) ListComponentsByRepository(ctx context.Context, repository string) ([]*Component, error) {
	return c.SearchComponents(ctx, repository, "", "")
}

// ListVersions 查询组件的版本列表
func (c *Client) ListVersions(ctx context.Context, repository, name string) ([]string, error) {
	components, err := c.SearchComponents(ctx, repository, name, "")
	if err != nil {
		return nil, err
	}

	versions := make([]string, 0, len(components))
	for _, comp := range components {
		if comp.Version != "" {
			versions = append(versions, comp.Version)
		}
	}

	return versions, nil
}

// GetComponent 获取组件详情
func (c *Client) GetComponent(ctx context.Context, componentID string) (*Component, error) {
	path := fmt.Sprintf("/v1/components/%s", url.PathEscape(componentID))

	body, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var component Component
	if err := json.Unmarshal(body, &component); err != nil {
		return nil, fmt.Errorf("unmarshal component: %w", err)
	}

	return &component, nil
}

// TestConnection 测试连接
func (c *Client) TestConnection(ctx context.Context) error {
	_, err := c.doRequest(ctx, http.MethodGet, "/v1/status", nil)
	return err
}
