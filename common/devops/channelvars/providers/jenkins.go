package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
)

// JenkinsProvider Jenkins CI/CD Provider
type JenkinsProvider struct {
	channelManager channelvars.ChannelManager
}

// NewJenkinsProvider 创建 Jenkins Provider
func NewJenkinsProvider(channelManager channelvars.ChannelManager) *JenkinsProvider {
	return &JenkinsProvider{
		channelManager: channelManager,
	}
}

// Capabilities 返回能力
func (p *JenkinsProvider) Capabilities() channelvars.ProviderCapabilities {
	return channelvars.ProviderCapabilities{
		SupportedQueries: []string{
			"build.folder",
			"build.job",
			"build.number",
		},
		SupportedAddressFormats: []channelvars.AddressFormat{
			{
				Field:   "address.jobUrl",
				Pattern: `^https?://[^/]+/job/.+$`,
				Example: "https://jenkins.example.com/job/my-job",
			},
		},
		SupportsAddressResolve: true,
		RequiresCredential:     true,
	}
}

// ResolveAddress 解析地址
func (p *JenkinsProvider) ResolveAddress(ctx context.Context, req *channelvars.ResolveAddressRequest) (*channelvars.ResolveAddressResponse, error) {
	// 解析 URL
	u, err := url.Parse(req.Address)
	if err != nil {
		return &channelvars.ResolveAddressResponse{
			Resolved: false,
			Reason:   fmt.Sprintf("invalid URL: %v", err),
		}, nil
	}

	// 获取渠道实例
	instances, err := p.channelManager.FindInstancesByEndpoint(ctx, fmt.Sprintf("%s://%s", u.Scheme, u.Host), req.ProjectID)
	if err != nil || len(instances) == 0 {
		return &channelvars.ResolveAddressResponse{
			Resolved: false,
			Reason:   "no matching channel instance found",
		}, nil
	}

	// 解析路径：/job/{job_name} 或 /job/{folder}/job/{job_name}
	path := strings.TrimPrefix(u.Path, "/")
	parts := strings.Split(path, "/")

	fields := map[string]string{}

	// 解析 job 路径
	var jobPath []string
	for i := 0; i < len(parts); i++ {
		if parts[i] == "job" && i+1 < len(parts) {
			jobPath = append(jobPath, parts[i+1])
			i++ // 跳过下一个元素
		}
	}

	if len(jobPath) > 0 {
		// 最后一个是 job 名称
		fields["dynamic.job"] = jobPath[len(jobPath)-1]

		// 如果有文件夹路径
		if len(jobPath) > 1 {
			fields["dynamic.folder"] = strings.Join(jobPath[:len(jobPath)-1], "/")
		}
	}

	// 解析构建号
	if strings.Contains(path, "/build/") {
		for i, part := range parts {
			if part == "build" && i+1 < len(parts) {
				fields["dynamic.buildNumber"] = parts[i+1]
				break
			}
		}
	}

	return &channelvars.ResolveAddressResponse{
		Resolved:          true,
		ChannelInstanceID: instances[0].ID,
		Fields:            fields,
	}, nil
}

// QueryOptions 查询选项
func (p *JenkinsProvider) QueryOptions(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	// 获取渠道实例
	instance, err := p.channelManager.GetInstance(ctx, req.ChannelInstanceID)
	if err != nil {
		return nil, fmt.Errorf("get channel instance: %w", err)
	}

	switch req.ProviderKey {
	case "build.folder":
		return p.queryFolders(ctx, instance)
	case "build.job":
		return p.queryJobs(ctx, instance, req.ParentValues)
	case "build.number":
		return p.queryBuilds(ctx, instance, req.ParentValues)
	default:
		return &channelvars.QueryOptionsResponse{
			Options: []channelvars.Option{},
		}, nil
	}
}

// queryFolders 查询文件夹列表
func (p *JenkinsProvider) queryFolders(ctx context.Context, instance *channelvars.ChannelInstance) (*channelvars.QueryOptionsResponse, error) {
	apiURL := fmt.Sprintf("%s/api/json?tree=jobs[name,_class]", strings.TrimSuffix(instance.Endpoint, "/"))

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// 设置认证
	p.setAuth(req, instance)

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s, body: %s", resp.Status, string(body))
	}

	// 解析响应
	var result struct {
		Jobs []struct {
			Name  string `json:"name"`
			Class string `json:"_class"`
		} `json:"jobs"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// 过滤出文件夹
	options := make([]channelvars.Option, 0)
	for _, job := range result.Jobs {
		// Jenkins 文件夹的 class 包含 "Folder"
		if strings.Contains(job.Class, "Folder") {
			options = append(options, channelvars.Option{
				Label: job.Name,
				Value: job.Name,
			})
		}
	}

	return &channelvars.QueryOptionsResponse{
		Options: options,
	}, nil
}

// queryJobs 查询 Job 列表
func (p *JenkinsProvider) queryJobs(ctx context.Context, instance *channelvars.ChannelInstance, parentValues map[string]string) (*channelvars.QueryOptionsResponse, error) {
	folder := parentValues["dynamic.folder"]

	// 构建 API URL
	apiURL := strings.TrimSuffix(instance.Endpoint, "/")
	if folder != "" {
		// 处理文件夹路径
		folderParts := strings.Split(folder, "/")
		for _, part := range folderParts {
			apiURL += "/job/" + part
		}
	}
	apiURL += "/api/json?tree=jobs[name,_class]"

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// 设置认证
	p.setAuth(req, instance)

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s, body: %s", resp.Status, string(body))
	}

	// 解析响应
	var result struct {
		Jobs []struct {
			Name  string `json:"name"`
			Class string `json:"_class"`
		} `json:"jobs"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// 过滤出 Job（排除文件夹）
	options := make([]channelvars.Option, 0)
	for _, job := range result.Jobs {
		// 排除文件夹
		if !strings.Contains(job.Class, "Folder") {
			options = append(options, channelvars.Option{
				Label: job.Name,
				Value: job.Name,
			})
		}
	}

	return &channelvars.QueryOptionsResponse{
		Options: options,
	}, nil
}

// queryBuilds 查询构建列表
func (p *JenkinsProvider) queryBuilds(ctx context.Context, instance *channelvars.ChannelInstance, parentValues map[string]string) (*channelvars.QueryOptionsResponse, error) {
	folder := parentValues["dynamic.folder"]
	job := parentValues["dynamic.job"]

	if job == "" {
		return &channelvars.QueryOptionsResponse{
			Options: []channelvars.Option{},
		}, nil
	}

	// 构建 API URL
	apiURL := strings.TrimSuffix(instance.Endpoint, "/")
	if folder != "" {
		folderParts := strings.Split(folder, "/")
		for _, part := range folderParts {
			apiURL += "/job/" + part
		}
	}
	apiURL += "/job/" + job + "/api/json?tree=builds[number,result,timestamp,displayName]"

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// 设置认证
	p.setAuth(req, instance)

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s, body: %s", resp.Status, string(body))
	}

	// 解析响应
	var result struct {
		Builds []struct {
			Number      int    `json:"number"`
			Result      string `json:"result"`
			DisplayName string `json:"displayName"`
		} `json:"builds"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// 转换为选项
	options := make([]channelvars.Option, 0, len(result.Builds))
	for _, build := range result.Builds {
		label := fmt.Sprintf("#%d", build.Number)
		if build.DisplayName != "" && build.DisplayName != label {
			label = build.DisplayName
		}
		if build.Result != "" {
			label = fmt.Sprintf("%s [%s]", label, build.Result)
		}

		options = append(options, channelvars.Option{
			Label: label,
			Value: fmt.Sprintf("%d", build.Number),
		})
	}

	return &channelvars.QueryOptionsResponse{
		Options: options,
	}, nil
}

// setAuth 设置认证信息
func (p *JenkinsProvider) setAuth(req *http.Request, instance *channelvars.ChannelInstance) {
	if token, ok := instance.Config["token"].(string); ok && token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	} else if username, ok := instance.Config["username"].(string); ok {
		if password, ok := instance.Config["password"].(string); ok {
			req.SetBasicAuth(username, password)
		}
	}
}

// RenderOutput 渲染输出
func (p *JenkinsProvider) RenderOutput(ctx context.Context, req *channelvars.RenderOutputRequest) (*channelvars.RenderOutputResponse, error) {
	return nil, fmt.Errorf("jenkins provider does not support render output")
}

// ValidateValue 校验值
func (p *JenkinsProvider) ValidateValue(ctx context.Context, req *channelvars.ValidateValueRequest) error {
	return nil
}
