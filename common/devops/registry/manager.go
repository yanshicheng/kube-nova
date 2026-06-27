package registry

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
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

type catalogResp struct {
	Repositories []string `json:"repositories"`
}

type tagsResp struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

func New() devopstypes.Provider {
	return Manager{provider: httpcheck.ServiceProvider{
		ProductName: "Docker Registry",
		Probes: []httpcheck.Probe{
			{Path: "/v2/", APIVersion: "v2", Capability: "registryV2"},
		},
	}}
}

func (m Manager) TestConnection(ctx context.Context, req devopstypes.Request) devopstypes.Result {
	return m.provider.TestConnection(ctx, req)
}

func (m Manager) ListRepositories(ctx context.Context, req devopstypes.Request, keyword string, page, pageSize uint64) ([]devopstypes.DynamicOption, uint64, error) {
	query := url.Values{}
	if pageSize == 0 {
		pageSize = 20
	}
	query.Set("n", strconv.FormatUint(pageSize, 10))
	resp, err := get(ctx, req, "/v2/_catalog?"+query.Encode())
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/registry/manager.go", err)
		return nil, 0, err
	}
	if resp.status < 200 || resp.status >= 400 {
		err := fmt.Errorf("镜像仓库列表 HTTP %d", resp.status)
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/registry/manager.go", err)
		return nil, 0, err
	}
	var data catalogResp
	if err := json.Unmarshal(resp.body, &data); err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/registry/manager.go", err)
		return nil, 0, err
	}
	return optionsFromStrings(data.Repositories, keyword, page, pageSize), uint64(len(data.Repositories)), nil
}

func (m Manager) ListImages(ctx context.Context, req devopstypes.Request, repository, keyword string, page, pageSize uint64) ([]devopstypes.DynamicOption, uint64, error) {
	repository = strings.Trim(strings.TrimSpace(repository), "/")
	if repository == "" {
		return nil, 0, fmt.Errorf("镜像仓库不能为空")
	}
	resp, err := get(ctx, req, "/v2/"+escapeRegistryName(repository)+"/tags/list")
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/registry/manager.go", err)
		return nil, 0, err
	}
	if resp.status < 200 || resp.status >= 400 {
		err := fmt.Errorf("镜像列表 HTTP %d", resp.status)
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/registry/manager.go", err)
		return nil, 0, err
	}
	var data tagsResp
	if err := json.Unmarshal(resp.body, &data); err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/registry/manager.go", err)
		return nil, 0, err
	}
	items := optionsFromStrings([]string{repository}, keyword, page, pageSize)
	if len(items) == 0 && keyword == "" {
		items = []devopstypes.DynamicOption{{Label: repository, Value: repository, Metadata: map[string]any{"tags": data.Tags}}}
	}
	return items, uint64(len(items)), nil
}

func (m Manager) ListTags(ctx context.Context, req devopstypes.Request, repository, image, keyword string, page, pageSize uint64) ([]devopstypes.DynamicOption, uint64, error) {
	name := strings.Trim(strings.TrimSpace(image), "/")
	if name == "" {
		name = strings.Trim(strings.TrimSpace(repository), "/")
	}
	if name == "" {
		return nil, 0, fmt.Errorf("镜像不能为空")
	}
	resp, err := get(ctx, req, "/v2/"+escapeRegistryName(name)+"/tags/list")
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/registry/manager.go", err)
		return nil, 0, err
	}
	if resp.status < 200 || resp.status >= 400 {
		err := fmt.Errorf("镜像 Tag 列表 HTTP %d", resp.status)
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/registry/manager.go", err)
		return nil, 0, err
	}
	var data tagsResp
	if err := json.Unmarshal(resp.body, &data); err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/registry/manager.go", err)
		return nil, 0, err
	}
	items := optionsFromStrings(data.Tags, keyword, page, pageSize)
	return items, uint64(len(data.Tags)), nil
}

type response struct {
	status int
	body   []byte
}

func get(ctx context.Context, req devopstypes.Request, apiPath string) (response, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(req.Channel.Endpoint), "/")
	if endpoint == "" {
		return response{}, fmt.Errorf("镜像仓库访问地址为空")
	}
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: req.Channel.InsecureSkipTLS}, //nolint:gosec
		},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/"+strings.TrimLeft(apiPath, "/"), http.NoBody)
	if err != nil {
		return response{}, err
	}
	httpcheck.ApplyAuth(request, req.Channel, req.Credential)
	resp, err := client.Do(request)
	if err != nil {
		return response{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return response{status: resp.StatusCode}, err
	}
	return response{status: resp.StatusCode, body: body}, nil
}

func optionsFromStrings(values []string, keyword string, page, pageSize uint64) []devopstypes.DynamicOption {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	filtered := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(value), keyword) {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		filtered = append(filtered, value)
	}
	sort.Strings(filtered)
	start, end := pageWindow(page, pageSize, uint64(len(filtered)))
	items := make([]devopstypes.DynamicOption, 0, end-start)
	for _, value := range filtered[start:end] {
		items = append(items, devopstypes.DynamicOption{Label: value, Value: value})
	}
	return items
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
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return int(start), int(end)
}

func escapeRegistryName(name string) string {
	parts := strings.Split(strings.Trim(name, "/"), "/")
	for i, item := range parts {
		parts[i] = url.PathEscape(item)
	}
	return strings.Join(parts, "/")
}
