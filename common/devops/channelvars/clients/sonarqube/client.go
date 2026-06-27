package sonarqube

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

// Client SonarQube API客户端
type Client struct {
	endpoint   string
	token      string
	httpClient *http.Client
}

// NewClient 创建SonarQube客户端
func NewClient(endpoint, token string, insecure bool) *Client {
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
		token:      token,
		httpClient: httpClient,
	}
}

// Project 项目信息
type Project struct {
	Key          string `json:"key"`
	Name         string `json:"name"`
	Qualifier    string `json:"qualifier"`
	Visibility   string `json:"visibility"`
	LastAnalysis string `json:"lastAnalysisDate"`
	Revision     string `json:"revision"`
}

// ProjectsResponse 项目列表响应
type ProjectsResponse struct {
	Paging struct {
		PageIndex int `json:"pageIndex"`
		PageSize  int `json:"pageSize"`
		Total     int `json:"total"`
	} `json:"paging"`
	Components []*Project `json:"components"`
}

// Measure 度量信息
type Measure struct {
	Metric    string `json:"metric"`
	Value     string `json:"value"`
	BestValue bool   `json:"bestValue"`
}

// Component 组件信息
type Component struct {
	Key       string     `json:"key"`
	Name      string     `json:"name"`
	Qualifier string     `json:"qualifier"`
	Measures  []*Measure `json:"measures"`
}

// doRequest 执行HTTP请求
func (c *Client) doRequest(ctx context.Context, method, path string, query url.Values) ([]byte, error) {
	// 构建URL
	apiURL := fmt.Sprintf("%s/api%s", c.endpoint, path)
	if query != nil {
		apiURL = fmt.Sprintf("%s?%s", apiURL, query.Encode())
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, method, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// 设置认证（使用token作为用户名，密码为空）
	req.SetBasicAuth(c.token, "")
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
	query.Set("p", fmt.Sprintf("%d", page))
	query.Set("ps", fmt.Sprintf("%d", pageSize))
	if search != "" {
		query.Set("q", search)
	}

	body, err := c.doRequest(ctx, http.MethodGet, "/components/search", query)
	if err != nil {
		return nil, err
	}

	var response ProjectsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("unmarshal projects: %w", err)
	}

	return response.Components, nil
}

// GetProject 获取项目详情
func (c *Client) GetProject(ctx context.Context, projectKey string) (*Project, error) {
	query := url.Values{}
	query.Set("component", projectKey)

	body, err := c.doRequest(ctx, http.MethodGet, "/components/show", query)
	if err != nil {
		return nil, err
	}

	var response struct {
		Component *Project `json:"component"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("unmarshal project: %w", err)
	}

	return response.Component, nil
}

// GetProjectMeasures 获取项目度量
func (c *Client) GetProjectMeasures(ctx context.Context, projectKey string, metrics []string) ([]*Measure, error) {
	query := url.Values{}
	query.Set("component", projectKey)
	query.Set("metricKeys", strings.Join(metrics, ","))

	body, err := c.doRequest(ctx, http.MethodGet, "/measures/component", query)
	if err != nil {
		return nil, err
	}

	var response struct {
		Component *Component `json:"component"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("unmarshal measures: %w", err)
	}

	if response.Component == nil {
		return nil, fmt.Errorf("component not found")
	}

	return response.Component.Measures, nil
}

// TestConnection 测试连接
func (c *Client) TestConnection(ctx context.Context) error {
	_, err := c.doRequest(ctx, http.MethodGet, "/system/status", nil)
	return err
}
