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

	"github.com/yanshicheng/kube-nova/common/devops/internal/httpcheck"
	devopstypes "github.com/yanshicheng/kube-nova/common/devops/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type Manager struct {
	provider httpcheck.ServiceProvider
}

type repositoryItem struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Format string `json:"format"`
	URL    string `json:"url"`
}

type componentResp struct {
	Items             []componentItem `json:"items"`
	ContinuationToken string          `json:"continuationToken"`
}

type componentItem struct {
	ID         string      `json:"id"`
	Repository string      `json:"repository"`
	Group      string      `json:"group"`
	Name       string      `json:"name"`
	Version    string      `json:"version"`
	Assets     []assetItem `json:"assets"`
}

type assetItem struct {
	Path string `json:"path"`
}

func New() devopstypes.Provider {
	return Manager{provider: httpcheck.ServiceProvider{
		ProductName: "Nexus",
		Probes: []httpcheck.Probe{
			{Path: "/service/rest/v1/status", Capability: "statusApi"},
			{Path: "/service/rest/v1/repositories", Capability: "repositoryApi"},
		},
	}}
}

func (m Manager) TestConnection(ctx context.Context, req devopstypes.Request) devopstypes.Result {
	return m.provider.TestConnection(ctx, req)
}

func (m Manager) ListRepositories(ctx context.Context, req devopstypes.Request, keyword string, page, pageSize uint64) ([]devopstypes.DynamicOption, uint64, error) {
	resp, err := m.get(ctx, req, "/service/rest/v1/repositories")
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/nexus/manager.go", err)
		return nil, 0, err
	}
	if resp.status < 200 || resp.status >= 400 {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/nexus/manager.go", fmt.Errorf("Nexus 仓库列表 HTTP %d", resp.status))
		return nil, 0, fmt.Errorf("Nexus 仓库列表 HTTP %d", resp.status)
	}
	var data []repositoryItem
	if err := json.Unmarshal(resp.body, &data); err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/nexus/manager.go", err)
		return nil, 0, err
	}
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	filtered := make([]repositoryItem, 0, len(data))
	for _, item := range data {
		if keyword == "" || strings.Contains(strings.ToLower(item.Name), keyword) {
			filtered = append(filtered, item)
		}
	}
	start, end := pageWindow(page, pageSize, uint64(len(filtered)))
	items := make([]devopstypes.DynamicOption, 0, end-start)
	for _, item := range filtered[start:end] {
		items = append(items, devopstypes.DynamicOption{
			Label: item.Name,
			Value: item.Name,
			Metadata: map[string]any{
				"type":   item.Type,
				"format": item.Format,
				"url":    item.URL,
			},
		})
	}
	return items, uint64(len(filtered)), nil
}

func (m Manager) ListComponents(ctx context.Context, req devopstypes.Request, repository, keyword string, pageSize uint64) ([]devopstypes.DynamicOption, uint64, error) {
	repository = strings.TrimSpace(repository)
	if repository == "" {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/nexus/manager.go", fmt.Errorf("Nexus 仓库不能为空"))
		return nil, 0, fmt.Errorf("Nexus 仓库不能为空")
	}
	query := url.Values{}
	query.Set("repository", repository)
	if strings.TrimSpace(keyword) != "" {
		query.Set("name", strings.TrimSpace(keyword))
	}
	query.Set("continuationToken", "")
	resp, err := m.get(ctx, req, "/service/rest/v1/components?"+query.Encode())
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/nexus/manager.go", err)
		return nil, 0, err
	}
	if resp.status < 200 || resp.status >= 400 {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/nexus/manager.go", fmt.Errorf("Nexus 组件列表 HTTP %d", resp.status))
		return nil, 0, fmt.Errorf("Nexus 组件列表 HTTP %d", resp.status)
	}
	var data componentResp
	if err := json.Unmarshal(resp.body, &data); err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/nexus/manager.go", err)
		return nil, 0, err
	}
	if pageSize == 0 {
		pageSize = 20
	}
	items := make([]devopstypes.DynamicOption, 0, len(data.Items))
	seen := map[string]struct{}{}
	for _, item := range data.Items {
		value := strings.Trim(strings.Join([]string{item.Group, item.Name}, "/"), "/")
		if value == "" {
			value = item.Name
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		items = append(items, devopstypes.DynamicOption{
			Label: value,
			Value: value,
			Metadata: map[string]any{
				"repository": item.Repository,
				"version":    item.Version,
			},
		})
		if uint64(len(items)) >= pageSize {
			break
		}
	}
	return items, uint64(len(items)), nil
}

func (m Manager) ListVersions(ctx context.Context, req devopstypes.Request, repository, component, keyword string, pageSize uint64) ([]devopstypes.DynamicOption, uint64, error) {
	repository = strings.TrimSpace(repository)
	component = strings.TrimSpace(component)
	if repository == "" || component == "" {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/nexus/manager.go", fmt.Errorf("Nexus 仓库和组件不能为空"))
		return nil, 0, fmt.Errorf("Nexus 仓库和组件不能为空")
	}
	query := url.Values{}
	query.Set("repository", repository)
	group, name := splitComponentValue(component)
	if group != "" {
		query.Set("group", group)
	}
	query.Set("name", name)
	resp, err := m.get(ctx, req, "/service/rest/v1/components?"+query.Encode())
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/nexus/manager.go", err)
		return nil, 0, err
	}
	if resp.status < 200 || resp.status >= 400 {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/nexus/manager.go", fmt.Errorf("Nexus 版本列表 HTTP %d", resp.status))
		return nil, 0, fmt.Errorf("Nexus 版本列表 HTTP %d", resp.status)
	}
	var data componentResp
	if err := json.Unmarshal(resp.body, &data); err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/nexus/manager.go", err)
		return nil, 0, err
	}
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if pageSize == 0 {
		pageSize = 20
	}
	items := make([]devopstypes.DynamicOption, 0, len(data.Items))
	for _, item := range data.Items {
		if item.Version == "" {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(item.Version), keyword) {
			continue
		}
		items = append(items, devopstypes.DynamicOption{Label: item.Version, Value: item.Version})
		if uint64(len(items)) >= pageSize {
			break
		}
	}
	return items, uint64(len(items)), nil
}

func splitComponentValue(component string) (string, string) {
	component = strings.Trim(strings.TrimSpace(component), "/")
	if component == "" {
		return "", ""
	}
	index := strings.LastIndex(component, "/")
	if index <= 0 || index >= len(component)-1 {
		return "", component
	}
	return component[:index], component[index+1:]
}

type response struct {
	status int
	body   []byte
}

func (m Manager) get(ctx context.Context, req devopstypes.Request, apiPath string) (response, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(req.Channel.Endpoint), "/")
	if endpoint == "" {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/nexus/manager.go", fmt.Errorf("Nexus 访问地址为空"))
		return response{}, fmt.Errorf("Nexus 访问地址为空")
	}
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: req.Channel.InsecureSkipTLS}, //nolint:gosec
		},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/"+strings.TrimLeft(apiPath, "/"), http.NoBody)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/nexus/manager.go", err)
		return response{}, err
	}
	httpcheck.ApplyAuth(request, req.Channel, req.Credential)
	resp, err := client.Do(request)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/nexus/manager.go", err)
		return response{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/nexus/manager.go", err)
		return response{status: resp.StatusCode}, err
	}
	return response{status: resp.StatusCode, body: body}, nil
}

func pageWindow(page, pageSize, total uint64) (int, int) {
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start > total {
		return int(total), int(total)
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return int(start), int(end)
}
