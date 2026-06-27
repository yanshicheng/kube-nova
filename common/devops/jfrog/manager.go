package jfrog

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
	Key         string `json:"key"`
	Type        string `json:"type"`
	Description string `json:"description"`
	URL         string `json:"url"`
	PackageType string `json:"packageType"`
}

type storageListResp struct {
	Files []storageFileItem `json:"files"`
}

type storageFileItem struct {
	URI          string `json:"uri"`
	Size         int64  `json:"size"`
	Folder       bool   `json:"folder"`
	LastModified string `json:"lastModified"`
}

func New() devopstypes.Provider {
	return Manager{provider: httpcheck.ServiceProvider{
		ProductName: "JFrog",
		Probes: []httpcheck.Probe{
			{Path: "/artifactory/api/system/version", Capability: "systemApi"},
			{Path: "/artifactory/api/system/ping", Capability: "ping"},
		},
	}}
}

func (m Manager) TestConnection(ctx context.Context, req devopstypes.Request) devopstypes.Result {
	return m.provider.TestConnection(ctx, req)
}

func (m Manager) ListRepositories(ctx context.Context, req devopstypes.Request, keyword string, page, pageSize uint64) ([]devopstypes.DynamicOption, uint64, error) {
	resp, err := get(ctx, req, "/artifactory/api/repositories")
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jfrog/manager.go", err)
		return nil, 0, err
	}
	if resp.status < 200 || resp.status >= 400 {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jfrog/manager.go", fmt.Errorf("JFrog 仓库列表 HTTP %d", resp.status))
		return nil, 0, fmt.Errorf("JFrog 仓库列表 HTTP %d", resp.status)
	}
	var data []repositoryItem
	if err := json.Unmarshal(resp.body, &data); err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jfrog/manager.go", err)
		return nil, 0, err
	}
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	filtered := make([]repositoryItem, 0, len(data))
	for _, item := range data {
		if strings.TrimSpace(item.Key) == "" {
			continue
		}
		if keyword == "" || strings.Contains(strings.ToLower(item.Key), keyword) {
			filtered = append(filtered, item)
		}
	}
	start, end := pageWindow(page, pageSize, uint64(len(filtered)))
	items := make([]devopstypes.DynamicOption, 0, end-start)
	for _, item := range filtered[start:end] {
		items = append(items, devopstypes.DynamicOption{
			Label: item.Key,
			Value: item.Key,
			Metadata: map[string]any{
				"type":        item.Type,
				"packageType": item.PackageType,
				"url":         item.URL,
			},
		})
	}
	return items, uint64(len(filtered)), nil
}

func (m Manager) ListArtifacts(ctx context.Context, req devopstypes.Request, repository, keyword string, page, pageSize uint64) ([]devopstypes.DynamicOption, uint64, error) {
	repository = strings.TrimSpace(repository)
	if repository == "" {
		return nil, 0, fmt.Errorf("JFrog 仓库不能为空")
	}
	query := url.Values{}
	query.Set("list", "")
	query.Set("deep", "1")
	query.Set("listFolders", "0")
	resp, err := get(ctx, req, "/artifactory/api/storage/"+url.PathEscape(repository)+"?"+query.Encode())
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jfrog/manager.go", err)
		return nil, 0, err
	}
	if resp.status < 200 || resp.status >= 400 {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jfrog/manager.go", fmt.Errorf("JFrog 制品列表 HTTP %d", resp.status))
		return nil, 0, fmt.Errorf("JFrog 制品列表 HTTP %d", resp.status)
	}
	var data storageListResp
	if err := json.Unmarshal(resp.body, &data); err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jfrog/manager.go", err)
		return nil, 0, err
	}
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	filtered := make([]storageFileItem, 0, len(data.Files))
	for _, item := range data.Files {
		value := strings.Trim(strings.TrimSpace(item.URI), "/")
		if value == "" || item.Folder {
			continue
		}
		if keyword == "" || strings.Contains(strings.ToLower(value), keyword) {
			item.URI = value
			filtered = append(filtered, item)
		}
	}
	start, end := pageWindow(page, pageSize, uint64(len(filtered)))
	items := make([]devopstypes.DynamicOption, 0, end-start)
	for _, item := range filtered[start:end] {
		items = append(items, devopstypes.DynamicOption{
			Label: item.URI,
			Value: item.URI,
			Metadata: map[string]any{
				"repository":   repository,
				"size":         item.Size,
				"lastModified": item.LastModified,
			},
		})
	}
	return items, uint64(len(filtered)), nil
}

func (m Manager) ListVersions(ctx context.Context, req devopstypes.Request, repository, artifactName, keyword string, page, pageSize uint64) ([]devopstypes.DynamicOption, uint64, error) {
	items, _, err := m.ListArtifacts(ctx, req, repository, artifactName, 1, 200)
	if err != nil {
		return nil, 0, err
	}
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	artifactName = strings.Trim(strings.TrimSpace(artifactName), "/")
	result := make([]devopstypes.DynamicOption, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		value := strings.Trim(strings.TrimSpace(item.Value), "/")
		if artifactName != "" && value != artifactName && !strings.HasPrefix(value, artifactName+"/") {
			continue
		}
		version := strings.TrimPrefix(value, artifactName)
		version = strings.Trim(version, "/")
		if version == "" {
			version = value
		}
		if keyword != "" && !strings.Contains(strings.ToLower(version), keyword) {
			continue
		}
		if _, ok := seen[version]; ok {
			continue
		}
		seen[version] = struct{}{}
		result = append(result, devopstypes.DynamicOption{
			Label:    version,
			Value:    version,
			Metadata: item.Metadata,
		})
	}
	start, end := pageWindow(page, pageSize, uint64(len(result)))
	return result[start:end], uint64(len(result)), nil
}

type response struct {
	status int
	body   []byte
}

func get(ctx context.Context, req devopstypes.Request, apiPath string) (response, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(req.Channel.Endpoint), "/")
	if endpoint == "" {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jfrog/manager.go", fmt.Errorf("JFrog 访问地址为空"))
		return response{}, fmt.Errorf("JFrog 访问地址为空")
	}
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: req.Channel.InsecureSkipTLS}, //nolint:gosec
		},
	}
	target := endpoint + "/" + strings.TrimLeft(apiPath, "/")
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jfrog/manager.go", err)
		return response{}, err
	}
	httpcheck.ApplyAuth(httpReq, req.Channel, req.Credential)
	httpResp, err := client.Do(httpReq)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jfrog/manager.go", err)
		return response{}, err
	}
	defer httpResp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(httpResp.Body, 1024*1024))
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jfrog/manager.go", err)
		return response{status: httpResp.StatusCode}, err
	}
	return response{status: httpResp.StatusCode, body: body}, nil
}

func pageWindow(page, pageSize, total uint64) (int, int) {
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start >= total {
		return int(total), int(total)
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return int(start), int(end)
}
