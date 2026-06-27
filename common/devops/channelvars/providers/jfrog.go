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

// JFrogProvider JFrog Artifactory Provider
type JFrogProvider struct {
	channelManager channelvars.ChannelManager
}

// NewJFrogProvider 创建 JFrog Provider
func NewJFrogProvider(channelManager channelvars.ChannelManager) *JFrogProvider {
	return &JFrogProvider{
		channelManager: channelManager,
	}
}

// Capabilities 返回能力
func (p *JFrogProvider) Capabilities() channelvars.ProviderCapabilities {
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
				Pattern: `^https?://[^/]+/artifactory/[^/]+/?$`,
				Example: "https://jfrog.example.com/artifactory/libs-release",
			},
			{
				Field:   "address.artifactUrl",
				Pattern: `^https?://[^/]+/artifactory/[^/]+/.+$`,
				Example: "https://jfrog.example.com/artifactory/libs-release/com/example/app/1.0.0/app-1.0.0.jar",
			},
			{
				Field:   "address.artifactVersionUrl",
				Pattern: `^https?://[^/]+/artifactory/[^/]+/.+$`,
				Example: "https://jfrog.example.com/artifactory/libs-release/com/example/app/1.0.0/app-1.0.0.jar",
			},
		},
		SupportsAddressResolve: true,
		RequiresCredential:     true,
	}
}

// ResolveAddress 解析地址
func (p *JFrogProvider) ResolveAddress(ctx context.Context, req *channelvars.ResolveAddressRequest) (*channelvars.ResolveAddressResponse, error) {
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

	// 解析路径：/artifactory/{repo}/{path}
	path := strings.TrimPrefix(u.Path, "/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 || parts[0] != "artifactory" {
		return &channelvars.ResolveAddressResponse{
			Resolved: false,
			Reason:   "invalid JFrog URL format, expected: /artifactory/{repo}/{path}",
		}, nil
	}

	repo := parts[1]
	artifactPath := ""
	if len(parts) > 2 {
		artifactPath = strings.Join(parts[2:], "/")
	}

	// 构建变量
	fields := map[string]string{
		"dynamic.repository": repo,
	}

	if artifactPath != "" {
		fields["dynamic.artifactName"] = artifactPath
	}

	return &channelvars.ResolveAddressResponse{
		Resolved:          true,
		ChannelInstanceID: instances[0].ID,
		Fields:            fields,
	}, nil
}

// QueryOptions 查询选项
func (p *JFrogProvider) QueryOptions(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	// 获取渠道实例
	instance, err := p.channelManager.GetInstance(ctx, req.ChannelInstanceID)
	if err != nil {
		return nil, fmt.Errorf("get channel instance: %w", err)
	}

	switch req.ProviderKey {
	case "artifact.repository":
		return p.queryRepositories(ctx, instance)
	case "artifact.artifact", "artifact.artifactName":
		return p.queryArtifacts(ctx, instance, req.ParentValues)
	case "artifact.tag", "artifact.artifactVersion":
		return p.queryTags(ctx, instance, req.ParentValues)
	case "artifact.artifactTag":
		return p.queryArtifactTags(ctx, instance, req.ParentValues)
	default:
		return &channelvars.QueryOptionsResponse{
			Options: []channelvars.Option{},
		}, nil
	}
}

// queryRepositories 查询仓库列表
func (p *JFrogProvider) queryRepositories(ctx context.Context, instance *channelvars.ChannelInstance) (*channelvars.QueryOptionsResponse, error) {
	// 构建请求
	apiURL := fmt.Sprintf("%s/api/repositories", strings.TrimSuffix(instance.Endpoint, "/"))

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// 设置认证
	if token, ok := instance.Config["token"].(string); ok && token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	} else if username, ok := instance.Config["username"].(string); ok {
		if password, ok := instance.Config["password"].(string); ok {
			req.SetBasicAuth(username, password)
		}
	}

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
	var repos []struct {
		Key         string `json:"key"`
		Type        string `json:"type"`
		Description string `json:"description"`
		PackageType string `json:"packageType"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// 转换为选项
	options := make([]channelvars.Option, 0, len(repos))
	for _, repo := range repos {
		label := repo.Key
		if repo.Description != "" {
			label = fmt.Sprintf("%s (%s)", repo.Key, repo.Description)
		}
		if repo.PackageType != "" {
			label = fmt.Sprintf("%s [%s]", label, repo.PackageType)
		}

		options = append(options, channelvars.Option{
			Label: label,
			Value: repo.Key,
		})
	}

	return &channelvars.QueryOptionsResponse{
		Options: options,
	}, nil
}

// queryArtifacts 查询制品列表
func (p *JFrogProvider) queryArtifacts(ctx context.Context, instance *channelvars.ChannelInstance, parentValues map[string]string) (*channelvars.QueryOptionsResponse, error) {
	repo, ok := parentValues["dynamic.repository"]
	if !ok || repo == "" {
		return &channelvars.QueryOptionsResponse{
			Options: []channelvars.Option{},
		}, nil
	}

	// 构建请求 - 使用 AQL 查询
	apiURL := fmt.Sprintf("%s/api/search/aql", strings.TrimSuffix(instance.Endpoint, "/"))

	// AQL 查询：查询仓库中的所有制品
	aqlQuery := fmt.Sprintf(`items.find({"repo": "%s"}).include("name", "path", "size", "modified")`, repo)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(aqlQuery))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "text/plain")

	// 设置认证
	if token, ok := instance.Config["token"].(string); ok && token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	} else if username, ok := instance.Config["username"].(string); ok {
		if password, ok := instance.Config["password"].(string); ok {
			req.SetBasicAuth(username, password)
		}
	}

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
		Results []struct {
			Name string `json:"name"`
			Path string `json:"path"`
			Size int64  `json:"size"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// 转换为选项
	options := make([]channelvars.Option, 0, len(result.Results))
	seen := make(map[string]bool)

	for _, item := range result.Results {
		fullPath := item.Path
		if fullPath != "" && fullPath != "." {
			fullPath = fullPath + "/" + item.Name
		} else {
			fullPath = item.Name
		}

		// 去重
		if seen[fullPath] {
			continue
		}
		seen[fullPath] = true

		options = append(options, channelvars.Option{
			Label: fullPath,
			Value: fullPath,
		})
	}

	return &channelvars.QueryOptionsResponse{
		Options: options,
	}, nil
}

// queryTags 查询制品标签列表
func (p *JFrogProvider) queryTags(ctx context.Context, instance *channelvars.ChannelInstance, parentValues map[string]string) (*channelvars.QueryOptionsResponse, error) {
	// JFrog 的标签查询与 artifact 相同
	return p.queryArtifacts(ctx, instance, parentValues)
}

// queryArtifactTags 查询制品加标签组合
func (p *JFrogProvider) queryArtifactTags(ctx context.Context, instance *channelvars.ChannelInstance, parentValues map[string]string) (*channelvars.QueryOptionsResponse, error) {
	// JFrog 的制品加标签查询与 artifact 相同
	return p.queryArtifacts(ctx, instance, parentValues)
}

// RenderOutput 渲染输出
func (p *JFrogProvider) RenderOutput(ctx context.Context, req *channelvars.RenderOutputRequest) (*channelvars.RenderOutputResponse, error) {
	return nil, fmt.Errorf("jfrog provider does not support render output")
}

// ValidateValue 校验值
func (p *JFrogProvider) ValidateValue(ctx context.Context, req *channelvars.ValidateValueRequest) error {
	return nil
}
