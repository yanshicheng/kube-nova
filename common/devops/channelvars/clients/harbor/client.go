package harbor

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

// Client Harbor API客户端
type Client struct {
	endpoint   string
	username   string
	password   string
	httpClient *http.Client
}

// NewClient 创建Harbor客户端
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

// Project 项目信息
type Project struct {
	ProjectID    int64  `json:"project_id"`
	Name         string `json:"name"`
	RepoCount    int64  `json:"repo_count"`
	ChartCount   int64  `json:"chart_count"`
	Metadata     string `json:"metadata"`
	CreationTime string `json:"creation_time"`
}

// Repository 仓库信息
type Repository struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	ProjectID     int64  `json:"project_id"`
	Description   string `json:"description"`
	PullCount     int64  `json:"pull_count"`
	ArtifactCount int64  `json:"artifact_count"`
	CreationTime  string `json:"creation_time"`
	UpdateTime    string `json:"update_time"`
}

// Artifact 制品信息
type Artifact struct {
	ID           int64  `json:"id"`
	Type         string `json:"type"`
	Digest       string `json:"digest"`
	Tags         []*Tag `json:"tags"`
	PushTime     string `json:"push_time"`
	PullTime     string `json:"pull_time"`
	Size         int64  `json:"size"`
	ScanOverview string `json:"scan_overview"`
}

// Tag 标签信息
type Tag struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	PushTime     string `json:"push_time"`
	PullTime     string `json:"pull_time"`
	RepositoryID int64  `json:"repository_id"`
	ArtifactID   int64  `json:"artifact_id"`
	Immutable    bool   `json:"immutable"`
	Signed       bool   `json:"signed"`
}

// doRequest 执行HTTP请求
func (c *Client) doRequest(ctx context.Context, method, path string, query url.Values) ([]byte, error) {
	// 构建URL
	apiURL := fmt.Sprintf("%s/api/v2.0%s", c.endpoint, path)
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

// ListProjects 查询项目列表
func (c *Client) ListProjects(ctx context.Context, search string, page, pageSize int) ([]*Project, error) {
	query := url.Values{}
	query.Set("page", fmt.Sprintf("%d", page))
	query.Set("page_size", fmt.Sprintf("%d", pageSize))
	if search != "" {
		query.Set("q", fmt.Sprintf("name=~%s", search))
	}

	body, err := c.doRequest(ctx, http.MethodGet, "/projects", query)
	if err != nil {
		return nil, err
	}

	var projects []*Project
	if err := json.Unmarshal(body, &projects); err != nil {
		return nil, fmt.Errorf("unmarshal projects: %w", err)
	}

	return projects, nil
}

// GetProject 获取项目详情
func (c *Client) GetProject(ctx context.Context, projectName string) (*Project, error) {
	path := fmt.Sprintf("/projects/%s", url.PathEscape(projectName))

	body, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var project Project
	if err := json.Unmarshal(body, &project); err != nil {
		return nil, fmt.Errorf("unmarshal project: %w", err)
	}

	return &project, nil
}

// ListRepositories 查询仓库列表
func (c *Client) ListRepositories(ctx context.Context, projectName, search string, page, pageSize int) ([]*Repository, error) {
	path := fmt.Sprintf("/projects/%s/repositories", url.PathEscape(projectName))

	query := url.Values{}
	query.Set("page", fmt.Sprintf("%d", page))
	query.Set("page_size", fmt.Sprintf("%d", pageSize))
	if search != "" {
		query.Set("q", fmt.Sprintf("name=~%s", search))
	}

	body, err := c.doRequest(ctx, http.MethodGet, path, query)
	if err != nil {
		return nil, err
	}

	var repos []*Repository
	if err := json.Unmarshal(body, &repos); err != nil {
		return nil, fmt.Errorf("unmarshal repositories: %w", err)
	}

	return repos, nil
}

// ListArtifacts 查询制品列表
func (c *Client) ListArtifacts(ctx context.Context, projectName, repoName string, page, pageSize int) ([]*Artifact, error) {
	path := fmt.Sprintf("/projects/%s/repositories/%s/artifacts",
		url.PathEscape(projectName),
		url.PathEscape(repoName))

	query := url.Values{}
	query.Set("page", fmt.Sprintf("%d", page))
	query.Set("page_size", fmt.Sprintf("%d", pageSize))
	query.Set("with_tag", "true")

	body, err := c.doRequest(ctx, http.MethodGet, path, query)
	if err != nil {
		return nil, err
	}

	var artifacts []*Artifact
	if err := json.Unmarshal(body, &artifacts); err != nil {
		return nil, fmt.Errorf("unmarshal artifacts: %w", err)
	}

	return artifacts, nil
}

// ListTags 查询标签列表
func (c *Client) ListTags(ctx context.Context, projectName, repoName string, page, pageSize int) ([]*Tag, error) {
	// 先获取制品列表
	artifacts, err := c.ListArtifacts(ctx, projectName, repoName, page, pageSize)
	if err != nil {
		return nil, err
	}

	// 提取所有标签
	var tags []*Tag
	for _, artifact := range artifacts {
		if artifact.Tags != nil {
			tags = append(tags, artifact.Tags...)
		}
	}

	return tags, nil
}

// TestConnection 测试连接
func (c *Client) TestConnection(ctx context.Context) error {
	_, err := c.doRequest(ctx, http.MethodGet, "/systeminfo", nil)
	return err
}
