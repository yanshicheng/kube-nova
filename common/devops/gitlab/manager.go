package gitlab

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/common/devops/internal/httpcheck"
	devopstypes "github.com/yanshicheng/kube-nova/common/devops/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type Manager struct{}

func New() devopstypes.Provider {
	return Manager{}
}

func NewGitHub() devopstypes.Provider {
	return Manager{}
}

func NewGitee() devopstypes.Provider {
	return Manager{}
}

func NewSVN() devopstypes.Provider {
	return Manager{}
}

type response struct {
	status int
	body   []byte
	header http.Header
}

type projectItem struct {
	ID                int64  `json:"id"`
	Name              string `json:"name"`
	Path              string `json:"path"`
	PathWithNamespace string `json:"path_with_namespace"`
	WebURL            string `json:"web_url"`
	HTTPURLToRepo     string `json:"http_url_to_repo"`
	SSHURLToRepo      string `json:"ssh_url_to_repo"`
}

type refItem struct {
	Name string `json:"name"`
}

var gitlabVersionPattern = regexp.MustCompile(`(?i)GitLab(?:\s+(?:Community|Enterprise)\s+Edition)?[^0-9]{0,80}([0-9]+\.[0-9]+(?:\.[0-9]+)?[0-9A-Za-z.+~_-]*)`)
var gitlabClientFactory = gitlabHTTPClient

func (Manager) TestConnection(ctx context.Context, req devopstypes.Request) devopstypes.Result {
	switch strings.TrimSpace(req.Channel.Type) {
	case "github":
		return testGitHTTPConnection(ctx, req, "GitHub", "v3")
	case "gitee":
		return testGitHTTPConnection(ctx, req, "Gitee", "v5")
	case "svn":
		return testGitHTTPConnection(ctx, req, "SVN", "")
	}
	endpoint := strings.TrimRight(strings.TrimSpace(req.Channel.Endpoint), "/")
	metadata := devopstypes.Metadata{
		ProductName:  "GitLab",
		APIVersion:   "v4",
		Capabilities: map[string]any{},
	}
	if endpoint == "" || strings.HasPrefix(endpoint, "system://") {
		return devopstypes.Unhealthy("访问地址为空", metadata)
	}
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		return devopstypes.Unhealthy("访问地址必须是 HTTP(S)", metadata)
	}

	client := &http.Client{
		Timeout: 4 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: req.Channel.InsecureSkipTLS}, //nolint:gosec
		},
	}

	connectStatus := 0
	apiStatus := 0
	lastMessage := ""
	credentialConfigured := hasGitLabCredential(req)

	if resp, err := get(ctx, client, endpoint, "/-/readiness", req); err == nil {
		metadata.Capabilities["readinessStatus"] = resp.status
		if isSuccess(resp.status) {
			connectStatus = firstStatus(connectStatus, resp.status)
			metadata.Capabilities["readiness"] = true
			metadata.Capabilities["httpStatus"] = resp.status
		} else if lastMessage == "" {
			logGitLabHTTPStatus("readiness", resp.status)
			lastMessage = fmt.Sprintf("GitLab readiness HTTP %d", resp.status)
		}
	} else if lastMessage == "" {
		lastMessage = fmt.Sprintf("GitLab readiness 连接失败: %s", devopstypes.TrimMessage(err.Error()))
		return devopstypes.Unhealthy(lastMessage, metadata)
	}

	if credentialConfigured {
		for _, path := range []string{"/api/v4/version", "/api/v4/metadata"} {
			resp, err := get(ctx, client, endpoint, path, req)
			if err != nil {
				lastMessage = fmt.Sprintf("GitLab version API 连接失败: %s", devopstypes.TrimMessage(err.Error()))
				continue
			}
			metadata.Capabilities["versionStatus"] = resp.status
			if isSuccess(resp.status) {
				apiStatus = firstStatus(apiStatus, resp.status)
				metadata.Capabilities["api"] = true
				metadata.Capabilities["versionApi"] = true
				metadata.Capabilities["httpStatus"] = resp.status
				version, revision := parseVersion(resp.body)
				if version == "" {
					version = parseVersionHeader(resp.header)
				}
				if version == "" {
					version = parseVersionHTML(resp.body)
				}
				metadata.ProductVersion = version
				metadata.Version = version
				if revision == "" {
					revision = parseRevisionHeader(resp.header)
				}
				if revision != "" {
					metadata.Capabilities["revision"] = revision
				}
				if version != "" {
					break
				}
			} else {
				metadata.Capabilities["versionAuthRequired"] = resp.status == http.StatusUnauthorized || resp.status == http.StatusForbidden
				logGitLabHTTPStatus("version API", resp.status)
				lastMessage = fmt.Sprintf("GitLab version API HTTP %d", resp.status)
			}
		}
	} else {
		metadata.Capabilities["authRequired"] = true
		metadata.Capabilities["versionSkipped"] = "GitLab 未配置凭据"
		lastMessage = "请配置 GitLab 凭据"
	}

	if credentialConfigured {
		if resp, err := get(ctx, client, endpoint, "/api/v4/user", req); err == nil {
			metadata.Capabilities["authStatus"] = resp.status
			if isSuccess(resp.status) {
				apiStatus = firstStatus(apiStatus, resp.status)
				metadata.Capabilities["auth"] = true
				metadata.Capabilities["httpStatus"] = resp.status
				metadata.AuthUser = parseUser(resp.body)
				if metadata.ProductVersion == "" {
					metadata.ProductVersion = parseVersionHeader(resp.header)
					metadata.Version = metadata.ProductVersion
				}
			} else {
				logGitLabHTTPStatus("user API", resp.status)
				lastMessage = fmt.Sprintf("GitLab user API HTTP %d", resp.status)
			}
		} else if lastMessage == "" {
			lastMessage = fmt.Sprintf("GitLab user API 连接失败: %s", devopstypes.TrimMessage(err.Error()))
		}
	}

	if credentialConfigured && apiStatus == 0 && connectStatus == 0 {
		lastMessage = "GitLab API 凭据不可用"
	}

	if credentialConfigured && apiStatus > 0 {
		return devopstypes.Healthy(fmt.Sprintf("GitLab HTTP %d", apiStatus), metadata)
	}
	if lastMessage == "" {
		lastMessage = "GitLab 连接失败"
	}
	return devopstypes.Unhealthy(lastMessage, metadata)
}

func testGitHTTPConnection(ctx context.Context, req devopstypes.Request, productName, apiVersion string) devopstypes.Result {
	endpoint := strings.TrimRight(strings.TrimSpace(req.Channel.Endpoint), "/")
	metadata := devopstypes.Metadata{
		ProductName:  productName,
		APIVersion:   apiVersion,
		Capabilities: map[string]any{},
	}
	if endpoint == "" || strings.HasPrefix(endpoint, "system://") {
		return devopstypes.Unhealthy("访问地址为空", metadata)
	}
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		return devopstypes.Unhealthy("访问地址必须是 HTTP(S)", metadata)
	}
	client := gitlabHTTPClient(req)
	resp, err := doGet(ctx, client, endpoint, "/", req, false)
	if err != nil {
		return devopstypes.Unhealthy(productName+" 连接失败: "+devopstypes.TrimMessage(err.Error()), metadata)
	}
	metadata.Capabilities["httpStatus"] = resp.status
	if isSuccess(resp.status) || resp.status == http.StatusUnauthorized || resp.status == http.StatusForbidden {
		return devopstypes.Healthy(fmt.Sprintf("%s HTTP %d", productName, resp.status), metadata)
	}
	return devopstypes.Unhealthy(fmt.Sprintf("%s HTTP %d", productName, resp.status), metadata)
}

func hasGitLabCredential(req devopstypes.Request) bool {
	if req.Credential != nil {
		switch req.Credential.Type {
		case "token":
			return strings.TrimSpace(req.Credential.Token) != ""
		case "username_password":
			return strings.TrimSpace(req.Credential.Username) != "" && strings.TrimSpace(req.Credential.Password) != ""
		default:
			return false
		}
	}
	switch req.Channel.AuthType {
	case "token":
		return strings.TrimSpace(req.Channel.Token) != ""
	case "username_password":
		return strings.TrimSpace(req.Channel.Username) != "" && strings.TrimSpace(req.Channel.Password) != ""
	default:
		return false
	}
}

func get(ctx context.Context, client *http.Client, endpoint, path string, req devopstypes.Request) (response, error) {
	return doGet(ctx, client, endpoint, path, req, true)
}

func doGet(ctx context.Context, client *http.Client, endpoint, path string, req devopstypes.Request, applyCredential bool) (response, error) {
	target := endpoint + "/" + strings.TrimLeft(path, "/")
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/gitlab/manager.go", err)
		return response{}, err
	}
	if applyCredential {
		applyGitLabAPIAuth(request, req)
	}
	resp, err := client.Do(request)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/gitlab/manager.go", err)
		return response{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/gitlab/manager.go", err)
		return response{status: resp.StatusCode, header: resp.Header}, err
	}
	return response{status: resp.StatusCode, body: body, header: resp.Header}, nil
}

func applyGitLabAPIAuth(request *http.Request, req devopstypes.Request) {
	if req.Credential != nil {
		switch req.Credential.Type {
		case "token":
			setGitLabPrivateToken(request, req.Credential.Token)
		case "secret_text":
			setGitLabPrivateToken(request, req.Credential.SecretText)
		case "username_password":
			setGitLabPrivateToken(request, req.Credential.Password)
		}
		return
	}
	switch req.Channel.AuthType {
	case "token":
		setGitLabPrivateToken(request, req.Channel.Token)
	case "username_password":
		setGitLabPrivateToken(request, req.Channel.Password)
	}
}

func setGitLabPrivateToken(request *http.Request, token string) {
	token = strings.TrimSpace(token)
	if token == "" {
		return
	}
	request.Header.Set("PRIVATE-TOKEN", token)
}

func logGitLabHTTPStatus(action string, status int) {
	logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/gitlab/manager.go", fmt.Errorf("GitLab %s HTTP %d", action, status))
}

func (Manager) ListProjects(ctx context.Context, req devopstypes.Request, keyword string, page, pageSize uint64) ([]devopstypes.DynamicOption, uint64, error) {
	if strings.TrimSpace(req.Channel.Type) == "svn" {
		options, total := svnDynamicOptions(req, keyword, page, pageSize, "project")
		return options, total, nil
	}
	endpoint := strings.TrimRight(strings.TrimSpace(req.Channel.Endpoint), "/")
	client := gitlabClientFactory(req)
	query := url.Values{}
	if strings.TrimSpace(keyword) != "" {
		query.Set("search", strings.TrimSpace(keyword))
	}
	query.Set("simple", "true")
	query.Set("membership", "true")
	query.Set("order_by", "last_activity_at")
	query.Set("sort", "desc")
	query.Set("page", pageString(page, 1))
	query.Set("per_page", pageString(pageSize, 20))
	resp, err := get(ctx, client, endpoint, "/api/v4/projects?"+query.Encode(), req)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/gitlab/manager.go", err)
		return nil, 0, err
	}
	if !isSuccess(resp.status) {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/gitlab/manager.go", fmt.Errorf("GitLab 项目列表 HTTP %d", resp.status))
		return nil, 0, fmt.Errorf("GitLab 项目列表 HTTP %d", resp.status)
	}
	var data []projectItem
	if err := json.Unmarshal(resp.body, &data); err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/gitlab/manager.go", err)
		return nil, 0, err
	}
	items := make([]devopstypes.DynamicOption, 0, len(data))
	for _, item := range data {
		value := item.PathWithNamespace
		if value == "" {
			value = strconv.FormatInt(item.ID, 10)
		}
		label := value
		if item.Name != "" && item.Name != value {
			label = item.Name + " / " + value
		}
		items = append(items, devopstypes.DynamicOption{
			Label: label,
			Value: value,
			Metadata: map[string]any{
				"id":                item.ID,
				"path":              item.Path,
				"pathWithNamespace": item.PathWithNamespace,
				"webUrl":            item.WebURL,
				"httpUrlToRepo":     item.HTTPURLToRepo,
				"sshUrlToRepo":      item.SSHURLToRepo,
			},
		})
	}
	return items, totalFromHeader(resp.header, uint64(len(items))), nil
}

func (Manager) ListBranches(ctx context.Context, req devopstypes.Request, project, keyword string, page, pageSize uint64) ([]devopstypes.DynamicOption, uint64, error) {
	if strings.TrimSpace(req.Channel.Type) == "svn" {
		options, total := svnDynamicOptions(req, keyword, page, pageSize, "branch")
		return options, total, nil
	}
	return listProjectRefs(ctx, req, project, keyword, page, pageSize, "branches")
}

func (Manager) ListTags(ctx context.Context, req devopstypes.Request, project, keyword string, page, pageSize uint64) ([]devopstypes.DynamicOption, uint64, error) {
	if strings.TrimSpace(req.Channel.Type) == "svn" {
		options, total := svnDynamicOptions(req, keyword, page, pageSize, "tag")
		return options, total, nil
	}
	return listProjectRefs(ctx, req, project, keyword, page, pageSize, "tags")
}

func svnDynamicOptions(req devopstypes.Request, keyword string, page, pageSize uint64, kind string) ([]devopstypes.DynamicOption, uint64) {
	values := svnCandidateValues(req, kind)
	options := make([]devopstypes.DynamicOption, 0, len(values))
	seen := map[string]struct{}{}
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	for _, value := range values {
		value = strings.Trim(strings.TrimSpace(value), "/")
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
		options = append(options, devopstypes.DynamicOption{Label: value, Value: value})
	}
	total := uint64(len(options))
	page = uint64Value(page, 1)
	pageSize = uint64Value(pageSize, 20)
	start := (page - 1) * pageSize
	if start >= total {
		return []devopstypes.DynamicOption{}, total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return options[start:end], total
}

func svnCandidateValues(req devopstypes.Request, kind string) []string {
	config := parseStringMap(req.Channel.Config)
	read := func(keys ...string) []string {
		result := make([]string, 0, len(keys))
		for _, key := range keys {
			if value, ok := config[key]; ok {
				result = append(result, fmt.Sprint(value))
			}
		}
		return result
	}
	switch kind {
	case "branch":
		return append(read("branch", "defaultBranch", "branches"), "trunk")
	case "tag":
		return read("tag", "defaultTag", "tags")
	default:
		values := read("project", "repository", "repositoryPath", "path")
		if len(values) == 0 {
			if endpointPath := svnProjectFromEndpoint(req.Channel.Endpoint); endpointPath != "" {
				values = append(values, endpointPath)
			}
		}
		return values
	}
}

func parseStringMap(raw string) map[string]any {
	var data map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &data); err != nil {
		return nil
	}
	return data
}

func svnProjectFromEndpoint(endpoint string) string {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || parsed.Host == "" {
		return ""
	}
	path := strings.Trim(parsed.EscapedPath(), "/")
	if decoded, err := url.PathUnescape(path); err == nil {
		path = decoded
	}
	return path
}

func uint64Value(value, fallback uint64) uint64 {
	if value == 0 {
		return fallback
	}
	return value
}

func listProjectRefs(ctx context.Context, req devopstypes.Request, project, keyword string, page, pageSize uint64, refType string) ([]devopstypes.DynamicOption, uint64, error) {
	project = strings.TrimSpace(project)
	if project == "" {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/gitlab/manager.go", fmt.Errorf("GitLab 项目不能为空"))
		return nil, 0, fmt.Errorf("GitLab 项目不能为空")
	}
	endpoint := strings.TrimRight(strings.TrimSpace(req.Channel.Endpoint), "/")
	client := gitlabClientFactory(req)
	query := url.Values{}
	if strings.TrimSpace(keyword) != "" {
		query.Set("search", strings.TrimSpace(keyword))
	}
	query.Set("page", pageString(page, 1))
	query.Set("per_page", pageString(pageSize, 20))
	resp, err := get(ctx, client, endpoint, gitlabRefAPI(project, refType, query.Encode()), req)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/gitlab/manager.go", err)
		return nil, 0, err
	}
	if !isSuccess(resp.status) {
		directErr := gitlabHTTPError(fmt.Sprintf("%s 列表", refType), resp)
		apiErr := directErr
		if shouldRetryProjectRefWithID(resp.status, project) {
			if projectID, lookupErr := findProjectID(ctx, req, project); lookupErr == nil && projectID != "" && projectID != project {
				resp, err = get(ctx, client, endpoint, gitlabRefAPI(projectID, refType, query.Encode()), req)
				if err != nil {
					logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/gitlab/manager.go", err)
					return nil, 0, err
				}
				if !isSuccess(resp.status) {
					apiErr = fmt.Errorf("%s；回退项目 ID %s 后仍失败：%s", directErr.Error(), projectID, gitlabHTTPError(fmt.Sprintf("%s 列表", refType), resp).Error())
				}
			} else if lookupErr != nil {
				apiErr = fmt.Errorf("%s；项目定位失败：%s", directErr.Error(), devopstypes.TrimMessage(lookupErr.Error()))
			} else {
				apiErr = fmt.Errorf("%s；未找到项目 ID", directErr.Error())
			}
		} else if shouldRetryProjectRefWithPath(resp.status, project) {
			if projectPath, lookupErr := findProjectPathByID(ctx, req, project); lookupErr == nil && projectPath != "" && projectPath != project {
				resp, err = get(ctx, client, endpoint, gitlabRefAPI(projectPath, refType, query.Encode()), req)
				if err != nil {
					logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/gitlab/manager.go", err)
					return nil, 0, err
				}
				if !isSuccess(resp.status) {
					apiErr = fmt.Errorf("%s；回退项目路径 %s 后仍失败：%s", directErr.Error(), projectPath, gitlabHTTPError(fmt.Sprintf("%s 列表", refType), resp).Error())
				}
			} else if lookupErr != nil {
				apiErr = fmt.Errorf("%s；项目路径定位失败：%s", directErr.Error(), devopstypes.TrimMessage(lookupErr.Error()))
			} else {
				apiErr = fmt.Errorf("%s；未找到项目路径", directErr.Error())
			}
		}
		if isSuccess(resp.status) {
			return refsFromGitlabResponse(resp)
		}
		items, total, fallbackErr := listProjectRefsByGitHTTP(ctx, req, project, keyword, page, pageSize, refType)
		if fallbackErr == nil {
			return items, total, nil
		}
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/gitlab/manager.go", fallbackErr)
		return nil, 0, fmt.Errorf("%s；Git HTTP 兜底失败：%s", apiErr.Error(), devopstypes.TrimMessage(fallbackErr.Error()))
	}
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/gitlab/manager.go", err)
		return nil, 0, err
	}
	return refsFromGitlabResponse(resp)
}

func refsFromGitlabResponse(resp response) ([]devopstypes.DynamicOption, uint64, error) {
	var data []refItem
	if err := json.Unmarshal(resp.body, &data); err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/gitlab/manager.go", err)
		return nil, 0, err
	}
	items := make([]devopstypes.DynamicOption, 0, len(data))
	for _, item := range data {
		if strings.TrimSpace(item.Name) == "" {
			continue
		}
		items = append(items, devopstypes.DynamicOption{Label: item.Name, Value: item.Name})
	}
	return items, totalFromHeader(resp.header, uint64(len(items))), nil
}

func shouldRetryProjectRefWithID(status int, project string) bool {
	if isSuccess(status) {
		return false
	}
	project = strings.TrimSpace(project)
	if project == "" || !strings.Contains(project, "/") {
		return false
	}
	return true
}

func shouldRetryProjectRefWithPath(status int, project string) bool {
	if isSuccess(status) {
		return false
	}
	_, err := strconv.ParseInt(strings.TrimSpace(project), 10, 64)
	return err == nil
}

func gitlabRefAPI(project, refType, query string) string {
	return fmt.Sprintf("/api/v4/projects/%s/repository/%s?%s", url.PathEscape(project), refType, query)
}

func gitlabProjectAPI(project string) string {
	return fmt.Sprintf("/api/v4/projects/%s", url.PathEscape(project))
}

func listProjectRefsByGitHTTP(ctx context.Context, req devopstypes.Request, project, keyword string, page, pageSize uint64, refType string) ([]devopstypes.DynamicOption, uint64, error) {
	info, err := findProjectInfo(ctx, req, project)
	if err != nil && !strings.Contains(project, "/") {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/gitlab/manager.go", err)
		return nil, 0, err
	}
	projectPath := strings.TrimSpace(info.PathWithNamespace)
	if projectPath == "" {
		projectPath = strings.TrimSpace(project)
	}
	cloneURL := gitHTTPCloneURL(req.Channel.Endpoint, projectPath)
	if cloneURL == "" {
		cloneURL = strings.TrimSpace(info.HTTPURLToRepo)
	}
	client := gitlabClientFactory(req)
	resp, err := getGitUploadPackRefs(ctx, client, cloneURL, req)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/gitlab/manager.go", err)
		return nil, 0, err
	}
	if !isSuccess(resp.status) {
		return nil, 0, gitlabHTTPError("Git refs", resp)
	}
	names, err := parseGitUploadPackRefs(resp.body, refType)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/gitlab/manager.go", err)
		return nil, 0, err
	}
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	items := make([]devopstypes.DynamicOption, 0, len(names))
	for _, name := range names {
		if keyword != "" && !strings.Contains(strings.ToLower(name), keyword) {
			continue
		}
		items = append(items, devopstypes.DynamicOption{Label: name, Value: name})
	}
	total := uint64(len(items))
	return paginateDynamicOptions(items, page, pageSize), total, nil
}

func getGitUploadPackRefs(ctx context.Context, client *http.Client, cloneURL string, req devopstypes.Request) (response, error) {
	if strings.TrimSpace(cloneURL) == "" {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/gitlab/manager.go", fmt.Errorf("Git clone 地址为空"))
		return response{}, fmt.Errorf("Git clone 地址为空")
	}
	target := strings.TrimRight(cloneURL, "/") + "/info/refs?service=git-upload-pack"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/gitlab/manager.go", err)
		return response{}, err
	}
	httpcheck.ApplyAuth(request, req.Channel, req.Credential)
	applyGitHTTPBasicAuth(request, req)
	resp, err := client.Do(request)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/gitlab/manager.go", err)
		return response{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/gitlab/manager.go", err)
		return response{status: resp.StatusCode, header: resp.Header}, err
	}
	return response{status: resp.StatusCode, body: body, header: resp.Header}, nil
}

func applyGitHTTPBasicAuth(request *http.Request, req devopstypes.Request) {
	if req.Credential != nil {
		switch req.Credential.Type {
		case "username_password":
			request.SetBasicAuth(req.Credential.Username, req.Credential.Password)
		case "token":
			request.SetBasicAuth("oauth2", req.Credential.Token)
		case "secret_text":
			request.SetBasicAuth("oauth2", req.Credential.SecretText)
		}
		return
	}
	switch req.Channel.AuthType {
	case "username_password":
		request.SetBasicAuth(req.Channel.Username, req.Channel.Password)
	case "token":
		request.SetBasicAuth("oauth2", req.Channel.Token)
	}
}

func gitHTTPCloneURL(endpoint, projectPath string) string {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	projectPath = strings.Trim(strings.TrimSpace(projectPath), "/")
	projectPath = strings.TrimSuffix(projectPath, ".git")
	if endpoint == "" || projectPath == "" {
		return ""
	}
	return endpoint + "/" + projectPath + ".git"
}

func parseGitUploadPackRefs(body []byte, refType string) ([]string, error) {
	prefix := "refs/heads/"
	if refType == "tags" {
		prefix = "refs/tags/"
	}
	result := make([]string, 0)
	seen := make(map[string]struct{})
	for len(body) >= 4 {
		size, err := strconv.ParseInt(string(body[:4]), 16, 64)
		if err != nil {
			logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/gitlab/manager.go", fmt.Errorf("Git refs 数据格式错误"))
			return nil, fmt.Errorf("Git refs 数据格式错误")
		}
		body = body[4:]
		if size == 0 {
			continue
		}
		payloadLen := int(size) - 4
		if payloadLen < 0 || payloadLen > len(body) {
			logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/gitlab/manager.go", fmt.Errorf("Git refs 数据长度错误"))
			return nil, fmt.Errorf("Git refs 数据长度错误")
		}
		payload := string(body[:payloadLen])
		body = body[payloadLen:]
		if strings.HasPrefix(payload, "#") || !strings.Contains(payload, " ") {
			continue
		}
		fields := strings.SplitN(payload, " ", 2)
		refName := strings.TrimSpace(strings.SplitN(fields[1], "\x00", 2)[0])
		if !strings.HasPrefix(refName, prefix) || strings.HasSuffix(refName, "^{}") {
			continue
		}
		name := strings.TrimPrefix(refName, prefix)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}
	return result, nil
}

func paginateDynamicOptions(items []devopstypes.DynamicOption, page, pageSize uint64) []devopstypes.DynamicOption {
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = uint64(len(items))
	}
	start := (page - 1) * pageSize
	if start >= uint64(len(items)) {
		return []devopstypes.DynamicOption{}
	}
	end := start + pageSize
	if end > uint64(len(items)) {
		end = uint64(len(items))
	}
	return items[start:end]
}

func findProjectID(ctx context.Context, req devopstypes.Request, project string) (string, error) {
	project = strings.TrimSpace(project)
	if project == "" {
		return "", nil
	}
	if _, err := strconv.ParseInt(project, 10, 64); err == nil {
		return project, nil
	}
	search := project
	if strings.Contains(project, "/") {
		parts := strings.Split(project, "/")
		search = parts[len(parts)-1]
	}
	endpoint := strings.TrimRight(strings.TrimSpace(req.Channel.Endpoint), "/")
	client := gitlabClientFactory(req)
	if projectID, err := findProjectIDBySearch(ctx, client, endpoint, req, project, search, true); err != nil || projectID != "" {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/gitlab/manager.go", err)
		return projectID, err
	}
	return findProjectIDBySearch(ctx, client, endpoint, req, project, search, false)
}

func findProjectPathByID(ctx context.Context, req devopstypes.Request, projectID string) (string, error) {
	item, err := findProjectInfo(ctx, req, projectID)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/gitlab/manager.go", err)
		return "", err
	}
	return strings.TrimSpace(item.PathWithNamespace), nil
}

func findProjectInfo(ctx context.Context, req devopstypes.Request, project string) (projectItem, error) {
	project = strings.TrimSpace(project)
	if project == "" {
		return projectItem{}, nil
	}
	endpoint := strings.TrimRight(strings.TrimSpace(req.Channel.Endpoint), "/")
	client := gitlabClientFactory(req)
	resp, err := get(ctx, client, endpoint, gitlabProjectAPI(project), req)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/gitlab/manager.go", err)
		return projectItem{}, err
	}
	if !isSuccess(resp.status) {
		return projectItem{}, gitlabHTTPError("项目详情", resp)
	}
	var item projectItem
	if err := json.Unmarshal(resp.body, &item); err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/gitlab/manager.go", err)
		return projectItem{}, err
	}
	return item, nil
}

func findProjectIDBySearch(ctx context.Context, client *http.Client, endpoint string, req devopstypes.Request, project, search string, membership bool) (string, error) {
	query := url.Values{}
	query.Set("search", search)
	query.Set("simple", "true")
	query.Set("per_page", "100")
	if membership {
		query.Set("membership", "true")
	}
	resp, err := get(ctx, client, endpoint, "/api/v4/projects?"+query.Encode(), req)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/gitlab/manager.go", err)
		return "", err
	}
	if !isSuccess(resp.status) {
		return "", gitlabHTTPError("项目列表", resp)
	}
	var data []projectItem
	if err := json.Unmarshal(resp.body, &data); err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/gitlab/manager.go", err)
		return "", err
	}
	basename := project
	if strings.Contains(project, "/") {
		parts := strings.Split(project, "/")
		basename = parts[len(parts)-1]
	}
	var nameMatchedID string
	for _, item := range data {
		if strings.EqualFold(strings.TrimSpace(item.PathWithNamespace), project) {
			return strconv.FormatInt(item.ID, 10), nil
		}
		if strings.EqualFold(strings.TrimSpace(item.Path), basename) ||
			strings.EqualFold(strings.TrimSpace(item.Name), basename) {
			if nameMatchedID != "" {
				nameMatchedID = ""
				break
			}
			nameMatchedID = strconv.FormatInt(item.ID, 10)
		}
	}
	return nameMatchedID, nil
}

func gitlabHTTPError(action string, resp response) error {
	if message := gitlabErrorMessage(resp.body); message != "" {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/gitlab/manager.go", fmt.Errorf("GitLab %s HTTP %d: %s", action, resp.status, message))
		return fmt.Errorf("GitLab %s HTTP %d: %s", action, resp.status, message)
	}
	logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/gitlab/manager.go", fmt.Errorf("GitLab %s HTTP %d", action, resp.status))
	return fmt.Errorf("GitLab %s HTTP %d", action, resp.status)
}

func gitlabErrorMessage(body []byte) string {
	body = []byte(strings.TrimSpace(string(body)))
	if len(body) == 0 {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err == nil {
		for _, key := range []string{"message", "error", "error_description"} {
			if value, ok := payload[key]; ok {
				return devopstypes.TrimMessage(fmt.Sprint(value))
			}
		}
	}
	return devopstypes.TrimMessage(string(body))
}

func gitlabHTTPClient(req devopstypes.Request) *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: req.Channel.InsecureSkipTLS}, //nolint:gosec
		},
	}
}

func pageString(value, fallback uint64) string {
	if value == 0 {
		value = fallback
	}
	return strconv.FormatUint(value, 10)
}

func totalFromHeader(header http.Header, fallback uint64) uint64 {
	value := strings.TrimSpace(header.Get("X-Total"))
	if value == "" {
		return fallback
	}
	total, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return fallback
	}
	return total
}

func isSuccess(status int) bool {
	return status >= http.StatusOK && status < http.StatusBadRequest
}

func firstStatus(current, next int) int {
	if current > 0 {
		return current
	}
	return next
}

func parseVersion(body []byte) (string, string) {
	var data any
	if err := json.Unmarshal(body, &data); err != nil {
		return "", ""
	}
	return findStringValue(data, "version", "git_version"), findStringValue(data, "revision", "git_revision")
}

func parseVersionHTML(body []byte) string {
	match := gitlabVersionPattern.FindStringSubmatch(string(body))
	if len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	return ""
}

func findStringValue(data any, keys ...string) string {
	switch value := data.(type) {
	case map[string]any:
		for _, key := range keys {
			if item, ok := value[key].(string); ok && strings.TrimSpace(item) != "" {
				return strings.TrimSpace(item)
			}
		}
		for _, item := range value {
			if found := findStringValue(item, keys...); found != "" {
				return found
			}
		}
	case []any:
		for _, item := range value {
			if found := findStringValue(item, keys...); found != "" {
				return found
			}
		}
	}
	return ""
}

func parseVersionHeader(header http.Header) string {
	for _, key := range []string{"X-GitLab-Version", "X-Gitlab-Version"} {
		if value := strings.TrimSpace(header.Get(key)); value != "" {
			return value
		}
	}
	return ""
}

func parseRevisionHeader(header http.Header) string {
	for _, key := range []string{"X-GitLab-Revision", "X-Gitlab-Revision"} {
		if value := strings.TrimSpace(header.Get(key)); value != "" {
			return value
		}
	}
	return ""
}

func parseUser(body []byte) string {
	var data struct {
		Username string `json:"username"`
		Name     string `json:"name"`
		Email    string `json:"email"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return ""
	}
	for _, value := range []string{data.Username, data.Name, data.Email} {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
