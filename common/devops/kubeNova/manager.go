package kubeNova

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
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

type Manager struct{}

func New() Manager {
	return Manager{}
}

func (Manager) TestConnection(ctx context.Context, req devopstypes.Request) devopstypes.Result {
	return serviceProvider().TestConnection(ctx, req)
}

func (Manager) ListProjects(ctx context.Context, req devopstypes.Request, keyword string, page, pageSize uint64) ([]devopstypes.DynamicOption, uint64, error) {
	data, err := getData(ctx, req, "/manager/v1/project/user", map[string]string{"name": keyword})
	if err != nil {
		return nil, 0, err
	}
	options := make([]devopstypes.DynamicOption, 0)
	for _, item := range objectList(data) {
		name := firstString(item, "name", "Name", "uuid", "Uuid", "id", "Id")
		id := firstString(item, "id", "Id")
		if id == "" {
			continue
		}
		options = appendIfMatch(options, devopstypes.DynamicOption{
			Label:    name,
			Value:    id,
			Metadata: item,
		}, keyword)
	}
	return paginateOptions(options, page, pageSize), uint64(len(options)), nil
}

func (Manager) ListClusters(ctx context.Context, req devopstypes.Request, projectID, keyword string, page, pageSize uint64) ([]devopstypes.DynamicOption, uint64, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, 0, fmt.Errorf("请选择项目")
	}
	data, err := getData(ctx, req, "/manager/v1/project/cluster/search", map[string]string{"projectId": projectID})
	if err != nil {
		return nil, 0, err
	}
	options := make([]devopstypes.DynamicOption, 0)
	for _, item := range objectList(data) {
		id := firstString(item, "id", "Id")
		if id == "" {
			continue
		}
		label := firstString(item, "clusterName", "ClusterName", "name", "Name", "clusterUuid", "ClusterUuid", "id", "Id")
		options = appendIfMatch(options, devopstypes.DynamicOption{
			Label:    label,
			Value:    id,
			Metadata: item,
		}, keyword)
	}
	return paginateOptions(options, page, pageSize), uint64(len(options)), nil
}

func (Manager) ListWorkspaces(ctx context.Context, req devopstypes.Request, projectClusterID, keyword string, page, pageSize uint64) ([]devopstypes.DynamicOption, uint64, error) {
	projectClusterID = strings.TrimSpace(projectClusterID)
	if projectClusterID == "" {
		return nil, 0, fmt.Errorf("请选择集群")
	}
	data, err := getData(ctx, req, "/manager/v1/project/workspace/search", map[string]string{"projectClusterId": projectClusterID})
	if err != nil {
		return nil, 0, err
	}
	options := make([]devopstypes.DynamicOption, 0)
	for _, item := range objectList(data) {
		id := firstString(item, "id", "Id")
		if id == "" {
			continue
		}
		label := firstString(item, "name", "Name", "namespace", "Namespace", "id", "Id")
		options = appendIfMatch(options, devopstypes.DynamicOption{
			Label:    label,
			Value:    id,
			Metadata: item,
		}, keyword)
	}
	sortOptions(options)
	return paginateOptions(options, page, pageSize), uint64(len(options)), nil
}

func (Manager) ListApplications(ctx context.Context, req devopstypes.Request, workspaceID, keyword string, page, pageSize uint64) ([]devopstypes.DynamicOption, uint64, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil, 0, fmt.Errorf("请选择工作空间")
	}
	data, err := getData(ctx, req, "/workload/v1/application/search", map[string]string{"workspaceId": workspaceID, "nameCn": keyword})
	if err != nil {
		return nil, 0, err
	}
	options := make([]devopstypes.DynamicOption, 0)
	for _, item := range objectList(data) {
		id := firstString(item, "id", "Id")
		if id == "" {
			continue
		}
		label := firstString(item, "nameCn", "NameCn", "nameEn", "NameEn", "id", "Id")
		options = appendIfMatch(options, devopstypes.DynamicOption{
			Label:    label,
			Value:    id,
			Metadata: item,
		}, keyword)
	}
	return paginateOptions(options, page, pageSize), uint64(len(options)), nil
}

func (Manager) ListVersions(ctx context.Context, req devopstypes.Request, applicationID, keyword string, page, pageSize uint64) ([]devopstypes.DynamicOption, uint64, error) {
	applicationID = strings.TrimSpace(applicationID)
	if applicationID == "" {
		return nil, 0, fmt.Errorf("请选择应用")
	}
	data, err := getData(ctx, req, "/workload/v1/application/version/search", map[string]string{"applicationId": applicationID})
	if err != nil {
		return nil, 0, err
	}
	options := make([]devopstypes.DynamicOption, 0)
	for _, item := range objectList(data) {
		id := firstString(item, "id", "Id")
		if id == "" {
			continue
		}
		label := firstString(item, "version", "Version", "resourceName", "ResourceName", "id", "Id")
		options = appendIfMatch(options, devopstypes.DynamicOption{
			Label:    label,
			Value:    id,
			Metadata: item,
		}, keyword)
	}
	return paginateOptions(options, page, pageSize), uint64(len(options)), nil
}

func (Manager) ListContainers(ctx context.Context, req devopstypes.Request, versionID, keyword string) ([]devopstypes.DynamicOption, uint64, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, 0, fmt.Errorf("请选择版本")
	}
	data, err := getData(ctx, req, "/workload/v1/resource/"+url.PathEscape(versionID)+"/images", nil)
	if err != nil {
		return nil, 0, err
	}
	options := make([]devopstypes.DynamicOption, 0)
	for _, item := range containerList(data) {
		name := firstString(item, "containerName", "ContainerName", "name", "Name")
		if name == "" {
			continue
		}
		containerType := firstString(item, "containerType", "ContainerType")
		if containerType == "" {
			containerType = "main"
			item["containerType"] = containerType
		}
		label := name + "（" + containerType + "）"
		options = appendIfMatch(options, devopstypes.DynamicOption{
			Label:    label,
			Value:    name,
			Metadata: item,
		}, keyword)
	}
	return options, uint64(len(options)), nil
}

func serviceProvider() httpcheck.ServiceProvider {
	return httpcheck.ServiceProvider{
		ProductName: "kube-nova",
		Probes: []httpcheck.Probe{
			{Path: "/version", PlainVersion: true, Capability: "versionApi"},
			{Path: "/healthz", Capability: "healthApi"},
		},
	}
}

func getData(ctx context.Context, req devopstypes.Request, apiPath string, params map[string]string) (any, error) {
	paths := []string{apiPath}
	if strings.HasPrefix(apiPath, "/manager/") || strings.HasPrefix(apiPath, "/workload/") {
		paths = append(paths, "/api"+apiPath)
	}
	var lastErr error
	for _, path := range paths {
		data, err := getDataByPath(ctx, req, path, params)
		if err == nil {
			return data, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func getDataByPath(ctx context.Context, req devopstypes.Request, apiPath string, params map[string]string) (any, error) {
	resp, err := get(ctx, req, apiPath, params)
	if err != nil {
		return nil, err
	}
	if resp.status < 200 || resp.status >= 300 {
		return nil, fmt.Errorf("kube-nova HTTP %d", resp.status)
	}
	var body any
	if err := json.Unmarshal(resp.body, &body); err != nil {
		return nil, fmt.Errorf("解析 kube-nova 响应失败: %v", err)
	}
	obj, ok := body.(map[string]any)
	if !ok {
		return body, nil
	}
	if code, exists := obj["code"]; exists && stringValue(code) != "0" {
		message := firstString(obj, "message", "msg", "data")
		if message == "" {
			message = "kube-nova 接口返回失败"
		}
		return nil, errors.New(message)
	}
	if data, exists := obj["data"]; exists {
		return data, nil
	}
	return body, nil
}

type response struct {
	status int
	body   []byte
	header http.Header
}

func get(ctx context.Context, req devopstypes.Request, apiPath string, params map[string]string) (response, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(req.Channel.Endpoint), "/")
	if endpoint == "" {
		return response{}, fmt.Errorf("kube-nova 访问地址为空")
	}
	target, err := url.Parse(endpoint + "/" + strings.TrimLeft(apiPath, "/"))
	if err != nil {
		return response{}, err
	}
	query := target.Query()
	for key, value := range params {
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			query.Set(key, value)
		}
	}
	target.RawQuery = query.Encode()
	client := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: req.Channel.InsecureSkipTLS}, //nolint:gosec
		},
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), http.NoBody)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/kubeNova/manager.go", err)
		return response{}, err
	}
	httpcheck.ApplyAuth(httpReq, req.Channel, req.Credential)
	resp, err := client.Do(httpReq)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/kubeNova/manager.go", err)
		return response{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/kubeNova/manager.go", err)
		return response{status: resp.StatusCode, header: resp.Header}, err
	}
	return response{status: resp.StatusCode, body: body, header: resp.Header}, nil
}

func objectList(value any) []map[string]any {
	rawItems, ok := value.([]any)
	if !ok {
		return nil
	}
	items := make([]map[string]any, 0, len(rawItems))
	for _, raw := range rawItems {
		if item, ok := raw.(map[string]any); ok {
			items = append(items, item)
		}
	}
	return items
}

func containerList(value any) []map[string]any {
	if obj, ok := value.(map[string]any); ok {
		if containers, ok := obj["containers"]; ok {
			return objectList(containers)
		}
	}
	return objectList(value)
}

func appendIfMatch(options []devopstypes.DynamicOption, option devopstypes.DynamicOption, keyword string) []devopstypes.DynamicOption {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return append(options, option)
	}
	if strings.Contains(strings.ToLower(option.Label), keyword) || strings.Contains(strings.ToLower(option.Value), keyword) {
		return append(options, option)
	}
	for _, value := range option.Metadata {
		if strings.Contains(strings.ToLower(stringValue(value)), keyword) {
			return append(options, option)
		}
	}
	return options
}

func firstString(item map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := stringValue(item[key]); value != "" {
			return value
		}
	}
	return ""
}

func stringValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return strings.TrimSpace(v.String())
	case float64:
		if math.Trunc(v) == v {
			return strconv.FormatInt(int64(v), 10)
		}
		return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(v, 'f', 6, 64), "0"), ".")
	case float32:
		value := float64(v)
		if math.Trunc(value) == value {
			return strconv.FormatInt(int64(value), 10)
		}
		return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(value, 'f', 6, 64), "0"), ".")
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func cloneMap(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func mergeMissing(target, source map[string]any) {
	for key, value := range source {
		if _, exists := target[key]; !exists {
			target[key] = value
		}
	}
}

func paginateOptions(options []devopstypes.DynamicOption, page, pageSize uint64) []devopstypes.DynamicOption {
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}
	start := int((page - 1) * pageSize)
	if start >= len(options) {
		return []devopstypes.DynamicOption{}
	}
	end := start + int(pageSize)
	if end > len(options) {
		end = len(options)
	}
	return options[start:end]
}

func sortOptions(options []devopstypes.DynamicOption) {
	sort.SliceStable(options, func(i, j int) bool {
		return strings.ToLower(options[i].Label) < strings.ToLower(options[j].Label)
	})
}

var _ devopstypes.Provider = Manager{}
