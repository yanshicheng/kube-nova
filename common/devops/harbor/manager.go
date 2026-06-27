package harbor

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/common/devops/internal/httpcheck"
	devopstypes "github.com/yanshicheng/kube-nova/common/devops/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type Manager struct {
	provider httpcheck.ServiceProvider
}

type projectItem struct {
	ID        int64  `json:"project_id"`
	Name      string `json:"name"`
	RepoCount int64  `json:"repo_count"`
}

type repositoryItem struct {
	Name          string `json:"name"`
	ArtifactCount int64  `json:"artifact_count"`
	PullCount     int64  `json:"pull_count"`
}

type artifactItem struct {
	Digest     string    `json:"digest"`
	Tags       []tagItem `json:"tags"`
	PushTime   string    `json:"push_time"`
	PullTime   string    `json:"pull_time"`
	Size       int64     `json:"size"`
	MediaType  string    `json:"media_type"`
	Repository string    `json:"repository_name"`
}

type tagItem struct {
	Name     string `json:"name"`
	PushTime string `json:"push_time"`
	PullTime string `json:"pull_time"`
}

func New() devopstypes.Provider {
	return Manager{provider: httpcheck.ServiceProvider{
		ProductName: "Harbor",
		Probes: []httpcheck.Probe{
			{Path: "/api/v2.0/systeminfo", APIVersion: "v2.0", Capability: "systemInfo"},
			{Path: "/v2/", APIVersion: "v2", Capability: "registryV2"},
		},
	}}
}

func (m Manager) TestConnection(ctx context.Context, req devopstypes.Request) devopstypes.Result {
	return m.provider.TestConnection(ctx, req)
}

func (m Manager) ListProjects(ctx context.Context, req devopstypes.Request, keyword string, page, pageSize uint64) ([]devopstypes.DynamicOption, uint64, error) {
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}
	query := url.Values{}
	query.Set("page", strconv.FormatUint(page, 10))
	query.Set("page_size", strconv.FormatUint(pageSize, 10))
	if strings.TrimSpace(keyword) != "" {
		query.Set("name", strings.TrimSpace(keyword))
	}
	resp, err := get(ctx, req, "/api/v2.0/projects?"+query.Encode())
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/harbor/manager.go", err)
		return nil, 0, err
	}
	if resp.status < 200 || resp.status >= 400 {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/harbor/manager.go", fmt.Errorf("Harbor 项目列表 HTTP %d", resp.status))
		return nil, 0, fmt.Errorf("Harbor 项目列表 HTTP %d", resp.status)
	}
	var data []projectItem
	if err := json.Unmarshal(resp.body, &data); err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/harbor/manager.go", err)
		return nil, 0, err
	}
	items := make([]devopstypes.DynamicOption, 0, len(data))
	for _, item := range data {
		if strings.TrimSpace(item.Name) == "" {
			continue
		}
		items = append(items, devopstypes.DynamicOption{
			Label: item.Name,
			Value: item.Name,
			Metadata: map[string]any{
				"id":        item.ID,
				"repoCount": item.RepoCount,
			},
		})
	}
	total := uint64(len(items))
	if value := strings.TrimSpace(resp.header.Get("X-Total-Count")); value != "" {
		if parsed, err := strconv.ParseUint(value, 10, 64); err == nil {
			total = parsed
		}
	}
	return items, total, nil
}

func (m Manager) ListRepositories(ctx context.Context, req devopstypes.Request, project, keyword string, page, pageSize uint64) ([]devopstypes.DynamicOption, uint64, error) {
	project = strings.TrimSpace(project)
	if project == "" {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/harbor/manager.go", fmt.Errorf("Harbor 项目不能为空"))
		return nil, 0, fmt.Errorf("Harbor 项目不能为空")
	}
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}
	query := url.Values{}
	query.Set("page", strconv.FormatUint(page, 10))
	query.Set("page_size", strconv.FormatUint(pageSize, 10))
	if strings.TrimSpace(keyword) != "" {
		query.Set("q", "name=~"+strings.TrimSpace(keyword))
	}
	resp, err := get(ctx, req, "/api/v2.0/projects/"+url.PathEscape(project)+"/repositories?"+query.Encode())
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/harbor/manager.go", err)
		return nil, 0, err
	}
	if resp.status < 200 || resp.status >= 400 {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/harbor/manager.go", fmt.Errorf("Harbor 镜像列表 HTTP %d", resp.status))
		return nil, 0, fmt.Errorf("Harbor 镜像列表 HTTP %d", resp.status)
	}
	var data []repositoryItem
	if err := json.Unmarshal(resp.body, &data); err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/harbor/manager.go", err)
		return nil, 0, err
	}
	items := make([]devopstypes.DynamicOption, 0, len(data))
	projectPrefix := strings.Trim(project, "/") + "/"
	for _, item := range data {
		fullName := strings.TrimSpace(item.Name)
		if fullName == "" {
			continue
		}
		value := strings.TrimPrefix(fullName, projectPrefix)
		items = append(items, devopstypes.DynamicOption{
			Label: value,
			Value: value,
			Metadata: map[string]any{
				"fullName":      fullName,
				"artifactCount": item.ArtifactCount,
				"pullCount":     item.PullCount,
			},
		})
	}
	total := uint64(len(items))
	if value := strings.TrimSpace(resp.header.Get("X-Total-Count")); value != "" {
		if parsed, err := strconv.ParseUint(value, 10, 64); err == nil {
			total = parsed
		}
	}
	return items, total, nil
}

func (m Manager) ListTags(ctx context.Context, req devopstypes.Request, project, repository, keyword string, page, pageSize uint64) ([]devopstypes.DynamicOption, uint64, error) {
	project = strings.TrimSpace(project)
	repository = strings.TrimSpace(repository)
	if project == "" || repository == "" {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/harbor/manager.go", fmt.Errorf("Harbor 项目和镜像不能为空"))
		return nil, 0, fmt.Errorf("Harbor 项目和镜像不能为空")
	}
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}
	query := url.Values{}
	query.Set("page", strconv.FormatUint(page, 10))
	query.Set("page_size", strconv.FormatUint(pageSize, 10))
	resp, err := get(ctx, req, "/api/v2.0/projects/"+url.PathEscape(project)+"/repositories/"+escapeHarborRepository(repository)+"/artifacts?"+query.Encode())
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/harbor/manager.go", err)
		return nil, 0, err
	}
	if resp.status < 200 || resp.status >= 400 {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/harbor/manager.go", fmt.Errorf("Harbor 镜像 Tag 列表 HTTP %d", resp.status))
		return nil, 0, fmt.Errorf("Harbor 镜像 Tag 列表 HTTP %d", resp.status)
	}
	var data []artifactItem
	if err := json.Unmarshal(resp.body, &data); err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/harbor/manager.go", err)
		return nil, 0, err
	}
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	items := make([]devopstypes.DynamicOption, 0)
	for _, artifact := range data {
		for _, tag := range artifact.Tags {
			name := strings.TrimSpace(tag.Name)
			if name == "" {
				continue
			}
			if keyword != "" && !strings.Contains(strings.ToLower(name), keyword) {
				continue
			}
			items = append(items, devopstypes.DynamicOption{
				Label: name,
				Value: name,
				Metadata: map[string]any{
					"digest":   artifact.Digest,
					"pushTime": tag.PushTime,
					"pullTime": tag.PullTime,
					"size":     artifact.Size,
				},
			})
		}
	}
	total := uint64(len(items))
	if value := strings.TrimSpace(resp.header.Get("X-Total-Count")); value != "" {
		if parsed, err := strconv.ParseUint(value, 10, 64); err == nil && parsed > total {
			total = parsed
		}
	}
	return items, total, nil
}

type response struct {
	status int
	body   []byte
	header http.Header
}

func get(ctx context.Context, req devopstypes.Request, apiPath string) (response, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(req.Channel.Endpoint), "/")
	if endpoint == "" {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/harbor/manager.go", fmt.Errorf("Harbor 访问地址为空"))
		return response{}, fmt.Errorf("Harbor 访问地址为空")
	}
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: req.Channel.InsecureSkipTLS}, //nolint:gosec
		},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/"+strings.TrimLeft(apiPath, "/"), http.NoBody)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/harbor/manager.go", err)
		return response{}, err
	}
	httpcheck.ApplyAuth(request, req.Channel, req.Credential)
	resp, err := client.Do(request)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/harbor/manager.go", err)
		return response{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/harbor/manager.go", err)
		return response{status: resp.StatusCode, header: resp.Header}, err
	}
	return response{status: resp.StatusCode, body: body, header: resp.Header}, nil
}

func escapeHarborRepository(repository string) string {
	parts := strings.Split(strings.Trim(repository, "/"), "/")
	for i, item := range parts {
		parts[i] = url.PathEscape(item)
	}
	return strings.Join(parts, "%2F")
}
