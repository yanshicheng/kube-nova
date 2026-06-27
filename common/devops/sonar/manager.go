package sonar

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

const sonarResponseMaxBytes = 20 << 20

type projectSearchResp struct {
	Components []projectItem `json:"components"`
	Paging     struct {
		Total uint64 `json:"total"`
	} `json:"paging"`
}

type projectItem struct {
	Key              string `json:"key"`
	Name             string `json:"name"`
	Qualifier        string `json:"qualifier"`
	Visibility       string `json:"visibility"`
	LastAnalysisDate string `json:"lastAnalysisDate"`
}

type issuesSearchResp struct {
	Issues []json.RawMessage `json:"issues"`
	Paging struct {
		PageIndex int `json:"pageIndex"`
		PageSize  int `json:"pageSize"`
		Total     int `json:"total"`
	} `json:"paging"`
}

func New() devopstypes.Provider {
	return Manager{provider: httpcheck.ServiceProvider{
		ProductName: "SonarQube",
		Probes: []httpcheck.Probe{
			{Path: "/api/server/version", PlainVersion: true, Capability: "serverApi"},
			{Path: "/api/system/status", Capability: "systemStatus"},
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
	query.Set("p", fmt.Sprintf("%d", page))
	query.Set("ps", fmt.Sprintf("%d", pageSize))
	if strings.TrimSpace(keyword) != "" {
		query.Set("q", strings.TrimSpace(keyword))
	}
	resp, err := get(ctx, req, "/api/projects/search?"+query.Encode())
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/sonar/manager.go", err)
		return nil, 0, err
	}
	if resp.status < 200 || resp.status >= 400 {
		err := fmt.Errorf("SonarQube 项目列表 HTTP %d", resp.status)
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/sonar/manager.go", err)
		return nil, 0, err
	}
	var data projectSearchResp
	if err := json.Unmarshal(resp.body, &data); err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/sonar/manager.go", err)
		return nil, 0, err
	}
	items := make([]devopstypes.DynamicOption, 0, len(data.Components))
	for _, item := range data.Components {
		key := strings.TrimSpace(item.Key)
		if key == "" {
			continue
		}
		label := strings.TrimSpace(item.Name)
		if label == "" {
			label = key
		} else if label != key {
			label = label + "（" + key + "）"
		}
		items = append(items, devopstypes.DynamicOption{
			Label: label,
			Value: key,
			Metadata: map[string]any{
				"name":             item.Name,
				"qualifier":        item.Qualifier,
				"visibility":       item.Visibility,
				"lastAnalysisDate": item.LastAnalysisDate,
			},
		})
	}
	total := data.Paging.Total
	if total == 0 {
		total = uint64(len(items))
	}
	return items, total, nil
}

func (m Manager) FetchIssuesJSON(ctx context.Context, req devopstypes.Request, projectKey string) ([]byte, error) {
	projectKey = strings.TrimSpace(projectKey)
	if projectKey == "" {
		err := fmt.Errorf("SonarQube 项目 Key 为空")
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/sonar/manager.go", err)
		return nil, err
	}
	const pageSize = 200
	allIssues := make([]json.RawMessage, 0)
	for page := 1; ; page++ {
		query := url.Values{}
		query.Set("componentKeys", projectKey)
		query.Set("resolved", "false")
		query.Set("p", fmt.Sprintf("%d", page))
		query.Set("ps", fmt.Sprintf("%d", pageSize))
		resp, err := get(ctx, req, "/api/issues/search?"+query.Encode())
		if err != nil {
			logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/sonar/manager.go", err)
			return nil, err
		}
		if resp.status < 200 || resp.status >= 400 {
			err := fmt.Errorf("SonarQube 问题查询 HTTP %d", resp.status)
			logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/sonar/manager.go", err)
			return nil, err
		}
		var data issuesSearchResp
		if err := json.Unmarshal(resp.body, &data); err != nil {
			logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/sonar/manager.go", err)
			return nil, err
		}
		allIssues = append(allIssues, data.Issues...)
		if data.Paging.Total <= len(allIssues) || len(data.Issues) == 0 {
			break
		}
	}
	out, err := json.Marshal(map[string]any{"issues": allIssues})
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/sonar/manager.go", err)
		return nil, err
	}
	return out, nil
}

type response struct {
	status int
	body   []byte
}

func get(ctx context.Context, req devopstypes.Request, apiPath string) (response, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(req.Channel.Endpoint), "/")
	if endpoint == "" {
		err := fmt.Errorf("SonarQube 访问地址为空")
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/sonar/manager.go", err)
		return response{}, err
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
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/sonar/manager.go", err)
		return response{}, err
	}
	httpcheck.ApplyAuth(httpReq, req.Channel, req.Credential)
	httpResp, err := client.Do(httpReq)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/sonar/manager.go", err)
		return response{}, err
	}
	defer httpResp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(httpResp.Body, sonarResponseMaxBytes+1))
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/sonar/manager.go", err)
		return response{status: httpResp.StatusCode}, err
	}
	if len(body) > sonarResponseMaxBytes {
		err := fmt.Errorf("SonarQube 响应超过大小限制")
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/sonar/manager.go", err)
		return response{status: httpResp.StatusCode}, err
	}
	return response{status: httpResp.StatusCode, body: body}, nil
}
