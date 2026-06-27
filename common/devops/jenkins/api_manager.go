package jenkins

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/zeromicro/go-zero/core/logx"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ClientConfig struct {
	Endpoint        string
	Username        string
	Password        string
	Token           string
	InsecureSkipTLS bool
}

type Manager struct {
	endpoint         string
	username         string
	password         string
	token            string
	client           *http.Client
	crumbMu          sync.Mutex
	crumbField       string
	crumbValue       string
	crumbUnsupported bool
	crumbFetchedAt   time.Time
}

type BuildResult struct {
	QueueID    string
	BuildURL   string
	Location   string
	StatusCode int
}

type BuildInfo struct {
	Number    int64  `json:"number"`
	Building  bool   `json:"building"`
	Result    string `json:"result"`
	Url       string `json:"url"`
	Timestamp int64  `json:"timestamp"`
	Duration  int64  `json:"duration"`
}

type BuildArtifact struct {
	FileName     string `json:"fileName"`
	RelativePath string `json:"relativePath"`
}

type QueueInfo struct {
	ID         int64            `json:"id"`
	Cancelled  bool             `json:"cancelled"`
	Why        string           `json:"why"`
	Executable *QueueExecutable `json:"executable"`
}

type QueueExecutable struct {
	Number int64  `json:"number"`
	URL    string `json:"url"`
}

type CredentialConfig struct {
	ID          string
	Description string
	Type        string
	Username    string
	Password    string
	Token       string
	SecretText  string
	FileName    string
	PrivateKey  string
	Passphrase  string
	Kubeconfig  string
	Certificate string
	JsonData    string
}

type jobInfo struct {
	LastBuild *QueueExecutable `json:"lastBuild"`
}

type StageStatus struct {
	ID       string `json:"id"`
	ParentID string `json:"parentId,omitempty"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Type     string `json:"type"`
	Duration int64  `json:"durationMillis"`
	Start    int64  `json:"startTimeMillis"`
}

type stageLogResp struct {
	Text    string `json:"text"`
	HasMore bool   `json:"hasMore"`
	Length  int64  `json:"length"`
}

type stageNodeDesc struct {
	StageFlowNodes []stageFlowNode `json:"stageFlowNodes"`
}

type stageFlowNode struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Type     string `json:"type"`
	Duration int64  `json:"durationMillis"`
	Start    int64  `json:"startTimeMillis"`
}

type AgentNode struct {
	Name               string
	DisplayName        string
	Label              string
	Labels             []string
	Offline            bool
	TemporarilyOffline bool
}

type InputAction struct {
	ID         string
	Name       string
	Message    string
	ProceedURL string
	AbortURL   string
	Parameters []InputParameter
}

type InputParameter struct {
	Name         string
	Type         string
	Description  string
	DefaultValue string
	Choices      []string
}

type rawInputParameter struct {
	Name                  string   `json:"name"`
	Type                  string   `json:"type"`
	Description           string   `json:"description"`
	DefaultValue          any      `json:"defaultValue"`
	Value                 any      `json:"value"`
	Choices               []string `json:"choices"`
	DefaultParameterValue struct {
		Value any `json:"value"`
	} `json:"defaultParameterValue"`
}

type rawInputAction struct {
	ID         string              `json:"id"`
	Name       string              `json:"name"`
	Message    string              `json:"message"`
	ProceedURL string              `json:"proceedUrl"`
	AbortURL   string              `json:"abortUrl"`
	Parameters []rawInputParameter `json:"parameters"`
	Inputs     []rawInputParameter `json:"inputs"`
}

func NewManager(config ClientConfig) *Manager {
	client := &http.Client{Timeout: 30 * time.Second}
	if jar, err := cookiejar.New(nil); err == nil {
		client.Jar = jar
	}
	if config.InsecureSkipTLS {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // 用户显式配置跳过 Jenkins TLS 校验。
		}
	}
	return &Manager{
		endpoint: strings.TrimRight(config.Endpoint, "/"),
		username: config.Username,
		password: config.Password,
		token:    config.Token,
		client:   client,
	}
}

func (m *Manager) EnsureFolder(ctx context.Context, folder string) error {
	folder = strings.Trim(folder, "/")
	if folder == "" {
		return nil
	}
	parts := strings.Split(folder, "/")
	current := ""
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		parentPath := folderURLPath(current)
		form := url.Values{}
		form.Set("name", part)
		form.Set("mode", "com.cloudbees.hudson.plugins.folder.Folder")
		form.Set("from", "")
		if err := m.postForm(ctx, parentPath+"/createItem", form); err != nil && !strings.Contains(err.Error(), "400") {
			logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
			return err
		}
		if current == "" {
			current = part
		} else {
			current += "/" + part
		}
	}
	return nil
}

func (m *Manager) UpsertPipelineJob(ctx context.Context, folder, jobName, displayName, pipeline string) error {
	return m.UpsertPipelineJobWithParams(ctx, folder, jobName, displayName, pipeline, nil)
}

func (m *Manager) CreatePipelineJobWithParams(ctx context.Context, folder, jobName, displayName, pipeline string, params []JenkinsRenderParam) error {
	folder = strings.Trim(folder, "/")
	jobName = strings.TrimSpace(jobName)
	if jobName == "" {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", fmt.Errorf("jenkins job name is empty"))
		return fmt.Errorf("jenkins job name is empty")
	}
	if err := m.EnsureFolder(ctx, folder); err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return err
	}
	configXML := pipelineJobXMLWithParams(displayName, pipeline, params)
	formPath := folderURLPath(folder) + "/createItem?name=" + url.QueryEscape(jobName)
	req, err := m.newRequest(ctx, http.MethodPost, formPath, bytes.NewBufferString(configXML))
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return err
	}
	req.Header.Set("Content-Type", "application/xml; charset=UTF-8")
	resp, err := m.do(req)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	createStatus := resp.Status
	createBody, _ := io.ReadAll(io.LimitReader(resp.Body, 65536))
	if !isJenkinsJobAlreadyExists(resp.StatusCode, createBody) {
		return jenkinsHTTPError("jenkins create job failed", createStatus, createBody)
	}

	jobPath := path.Join(folderURLPath(folder), "job", url.PathEscape(jobName), "config.xml")
	req, err = m.newRequest(ctx, http.MethodPost, jobPath, bytes.NewBufferString(configXML))
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return err
	}
	req.Header.Set("Content-Type", "application/xml; charset=UTF-8")
	resp, err = m.do(req)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		return nil
	}
	updateBody, _ := io.ReadAll(io.LimitReader(resp.Body, 65536))
	if resp.StatusCode == http.StatusNotFound {
		return jenkinsHTTPError("jenkins create job failed", createStatus, createBody)
	}
	return jenkinsHTTPError("jenkins update job failed", resp.Status, updateBody)
}

func isJenkinsJobAlreadyExists(statusCode int, body []byte) bool {
	if statusCode == http.StatusConflict {
		return true
	}
	if statusCode != http.StatusBadRequest {
		return false
	}
	message := strings.ToLower(cleanJenkinsErrorBody(string(body)))
	return strings.Contains(message, "already exists") || strings.Contains(message, "already exist")
}

func (m *Manager) UpsertFolderCredential(ctx context.Context, folder string, credential CredentialConfig) error {
	folder = strings.Trim(folder, "/")
	credential.ID = strings.TrimSpace(credential.ID)
	if credential.ID == "" {
		err := fmt.Errorf("jenkins credential id is empty")
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return err
	}
	if err := m.EnsureFolder(ctx, folder); err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return err
	}
	configXML, err := credentialConfigXML(credential)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return err
	}
	updatePath := folderURLPath(folder) + "/credentials/store/folder/domain/_/credential/" + url.PathEscape(credential.ID) + "/config.xml"
	req, err := m.newRequest(ctx, http.MethodPost, updatePath, bytes.NewBufferString(configXML))
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return err
	}
	req.Header.Set("Content-Type", "application/xml; charset=UTF-8")
	resp, err := m.do(req)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	if resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 65536))
		return jenkinsHTTPError("jenkins update credential failed", resp.Status, body)
	}

	createForm, err := credentialCreateForm(credential)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return err
	}
	createPath := folderURLPath(folder) + "/credentials/store/folder/domain/_/createCredentials"
	req, err = m.newRequest(ctx, http.MethodPost, createPath, strings.NewReader(createForm.Encode()))
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err = m.do(req)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 65536))
	return jenkinsHTTPError("jenkins create credential failed", resp.Status, body)
}

func (m *Manager) DeleteFolderCredential(ctx context.Context, folder, credentialID string) error {
	folder = strings.Trim(folder, "/")
	credentialID = strings.TrimSpace(credentialID)
	if credentialID == "" {
		err := fmt.Errorf("jenkins credential id is empty")
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return err
	}
	deletePath := folderURLPath(folder) + "/credentials/store/folder/domain/_/credential/" + url.PathEscape(credentialID) + "/doDelete"
	req, err := m.newRequest(ctx, http.MethodPost, deletePath, nil)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return err
	}
	resp, err := m.do(req)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 65536))
	return jenkinsHTTPError("jenkins delete credential failed", resp.Status, body)
}

func (m *Manager) UpsertPipelineJobWithParams(ctx context.Context, folder, jobName, displayName, pipeline string, params []JenkinsRenderParam) error {
	folder = strings.Trim(folder, "/")
	jobName = strings.TrimSpace(jobName)
	if jobName == "" {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", fmt.Errorf("jenkins job name is empty"))
		return fmt.Errorf("jenkins job name is empty")
	}
	if err := m.EnsureFolder(ctx, folder); err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return err
	}
	configXML := pipelineJobXMLWithParams(displayName, pipeline, params)
	jobPath := path.Join(folderURLPath(folder), "job", url.PathEscape(jobName), "config.xml")
	req, err := m.newRequest(ctx, http.MethodPost, jobPath, bytes.NewBufferString(configXML))
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return err
	}
	req.Header.Set("Content-Type", "application/xml; charset=UTF-8")
	resp, err := m.do(req)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		return nil
	}
	if resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 65536))
		return jenkinsHTTPError("jenkins update job failed", resp.Status, body)
	}

	formPath := folderURLPath(folder) + "/createItem?name=" + url.QueryEscape(jobName)
	req, err = m.newRequest(ctx, http.MethodPost, formPath, bytes.NewBufferString(configXML))
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return err
	}
	req.Header.Set("Content-Type", "application/xml; charset=UTF-8")
	resp, err = m.do(req)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 65536))
	return jenkinsHTTPError("jenkins create job failed", resp.Status, body)
}

func (m *Manager) BuildWithParameters(ctx context.Context, jobFullName string, params map[string]string) (*BuildResult, error) {
	form := url.Values{}
	for key, value := range params {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		form.Set(key, value)
	}
	if len(form) == 0 {
		return m.Build(ctx, jobFullName)
	}
	endpoint := jobURLPath(jobFullName) + "/buildWithParameters"
	req, err := m.newRequest(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := m.do(req)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 65536))
		return nil, jenkinsHTTPError("jenkins trigger build failed", resp.Status, body)
	}
	location := resp.Header.Get("Location")
	return &BuildResult{
		QueueID:    queueIDFromLocation(location),
		BuildURL:   "",
		Location:   m.externalURL(location),
		StatusCode: resp.StatusCode,
	}, nil
}

func (m *Manager) Build(ctx context.Context, jobFullName string) (*BuildResult, error) {
	endpoint := jobURLPath(jobFullName) + "/build"
	req, err := m.newRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return nil, err
	}
	resp, err := m.do(req)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 65536))
		return nil, jenkinsHTTPError("jenkins trigger build failed", resp.Status, body)
	}
	location := resp.Header.Get("Location")
	return &BuildResult{
		QueueID:    queueIDFromLocation(location),
		BuildURL:   "",
		Location:   m.externalURL(location),
		StatusCode: resp.StatusCode,
	}, nil
}

func (m *Manager) QueueInfo(ctx context.Context, queueID string) (*QueueInfo, error) {
	queueID = strings.TrimSpace(queueID)
	if queueID == "" {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", fmt.Errorf("jenkins queue id is empty"))
		return nil, fmt.Errorf("jenkins queue id is empty")
	}
	var info QueueInfo
	if err := m.getJSON(ctx, "/queue/item/"+url.PathEscape(queueID)+"/api/json", &info); err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return nil, err
	}
	return &info, nil
}

func (m *Manager) QueueBuild(ctx context.Context, queueID string) (int64, string, bool, error) {
	info, err := m.QueueInfo(ctx, queueID)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return 0, "", false, err
	}
	if info.Cancelled {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", fmt.Errorf("jenkins queue item cancelled"))
		return 0, "", false, fmt.Errorf("jenkins queue item cancelled")
	}
	if info.Executable == nil || info.Executable.Number <= 0 {
		return 0, "", false, nil
	}
	return info.Executable.Number, m.externalURL(info.Executable.URL), true, nil
}

func (m *Manager) StopBuild(ctx context.Context, jobFullName string, buildNumber int64) error {
	return m.postForm(ctx, fmt.Sprintf("%s/%d/stop", jobURLPath(jobFullName), buildNumber), url.Values{})
}

func (m *Manager) DeleteBuild(ctx context.Context, jobFullName string, buildNumber int64) error {
	if buildNumber <= 0 {
		err := fmt.Errorf("jenkins build number is empty")
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return err
	}
	return m.postForm(ctx, fmt.Sprintf("%s/%d/doDelete", jobURLPath(jobFullName), buildNumber), url.Values{})
}

func (m *Manager) DisableJob(ctx context.Context, jobFullName string) error {
	jobFullName = strings.Trim(jobFullName, "/")
	if jobFullName == "" {
		err := fmt.Errorf("jenkins job full name is empty")
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return err
	}
	return m.postForm(ctx, jobURLPath(jobFullName)+"/disable", url.Values{})
}

func (m *Manager) BuildInfo(ctx context.Context, jobFullName string, buildNumber int64) (*BuildInfo, error) {
	var info BuildInfo
	if err := m.getJSON(ctx, fmt.Sprintf("%s/%d/api/json", jobURLPath(jobFullName), buildNumber), &info); err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return nil, err
	}
	info.Url = m.externalURL(info.Url)
	return &info, nil
}

func (m *Manager) LastBuildInfo(ctx context.Context, jobFullName string) (*BuildInfo, bool, error) {
	var info jobInfo
	if err := m.getJSON(ctx, jobURLPath(jobFullName)+"/api/json?tree=lastBuild[number,url]", &info); err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return nil, false, err
	}
	if info.LastBuild == nil || info.LastBuild.Number <= 0 {
		return nil, false, nil
	}
	build, err := m.BuildInfo(ctx, jobFullName, info.LastBuild.Number)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return nil, false, err
	}
	if strings.TrimSpace(build.Url) == "" {
		build.Url = m.externalURL(info.LastBuild.URL)
	}
	return build, true, nil
}

func (m *Manager) BuildArtifacts(ctx context.Context, jobFullName string, buildNumber int64) ([]BuildArtifact, error) {
	var resp struct {
		Artifacts []BuildArtifact `json:"artifacts"`
	}
	tree := url.QueryEscape("artifacts[fileName,relativePath]")
	if err := m.getJSON(ctx, fmt.Sprintf("%s/%d/api/json?tree=%s", jobURLPath(jobFullName), buildNumber, tree), &resp); err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return nil, err
	}
	return resp.Artifacts, nil
}

func (m *Manager) DownloadArtifact(ctx context.Context, jobFullName string, buildNumber int64, relativePath string) ([]byte, string, error) {
	relativePath = strings.Trim(strings.TrimSpace(relativePath), "/")
	if relativePath == "" {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", fmt.Errorf("jenkins artifact relative path is empty"))
		return nil, "", fmt.Errorf("jenkins artifact relative path is empty")
	}
	endpoint := fmt.Sprintf("%s/%d/artifact/%s", jobURLPath(jobFullName), buildNumber, escapePathSegments(relativePath))
	req, err := m.newRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return nil, "", err
	}
	resp, err := m.do(req)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 65536))
		return nil, "", jenkinsHTTPError("jenkins artifact download failed", resp.Status, body)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return nil, "", err
	}
	return body, strings.TrimSpace(resp.Header.Get("Content-Type")), nil
}

func (m *Manager) ConsoleText(ctx context.Context, jobFullName string, buildNumber int64) (string, error) {
	req, err := m.newRequest(ctx, http.MethodGet, fmt.Sprintf("%s/%d/consoleText", jobURLPath(jobFullName), buildNumber), nil)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return "", err
	}
	resp, err := m.do(req)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", fmt.Errorf("jenkins console log failed: %s", resp.Status))
		return "", fmt.Errorf("jenkins console log failed: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return "", err
	}
	return cleanJenkinsLogText(string(body)), nil
}

func (m *Manager) ProgressiveLog(ctx context.Context, jobFullName string, buildNumber, start int64) (string, int64, bool, error) {
	req, err := m.newRequest(ctx, http.MethodGet, fmt.Sprintf("%s/%d/logText/progressiveText?start=%d", jobURLPath(jobFullName), buildNumber, start), nil)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return "", start, false, err
	}
	resp, err := m.do(req)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return "", start, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", fmt.Errorf("jenkins progressive log failed: %s", resp.Status))
		return "", start, false, fmt.Errorf("jenkins progressive log failed: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return "", start, false, err
	}
	next := start
	if size := resp.Header.Get("X-Text-Size"); size != "" {
		if value, convErr := strconv.ParseInt(size, 10, 64); convErr == nil {
			next = value
		}
	}
	more := strings.EqualFold(resp.Header.Get("X-More-Data"), "true")
	return cleanJenkinsLogText(string(body)), next, more, nil
}

func (m *Manager) StageStatus(ctx context.Context, jobFullName string, buildNumber int64) ([]StageStatus, error) {
	var resp struct {
		Stages []StageStatus `json:"stages"`
	}
	if err := m.getJSON(ctx, fmt.Sprintf("%s/%d/wfapi/describe", jobURLPath(jobFullName), buildNumber), &resp); err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return nil, err
	}
	return resp.Stages, nil
}

func (m *Manager) PendingInputs(ctx context.Context, jobFullName string, buildNumber int64) ([]InputAction, error) {
	var raw json.RawMessage
	wfErr := m.getJSON(ctx, fmt.Sprintf("%s/%d/wfapi/pendingInputActions", jobURLPath(jobFullName), buildNumber), &raw)
	if wfErr == nil {
		inputs, err := parsePendingInputActions(raw)
		if err != nil {
			logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
			return nil, err
		}
		if len(inputs) > 0 {
			return rawInputActionsToInputActions(inputs), nil
		}
	}

	var inputRaw json.RawMessage
	inputErr := m.getJSON(ctx, fmt.Sprintf("%s/%d/input/api/json", jobURLPath(jobFullName), buildNumber), &inputRaw)
	if inputErr == nil {
		inputs, err := parsePendingInputActions(inputRaw)
		if err != nil {
			logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
			return nil, err
		}
		return rawInputActionsToInputActions(inputs), nil
	}

	if wfErr != nil {
		return nil, wfErr
	}
	return []InputAction{}, nil
}

func parsePendingInputActions(raw json.RawMessage) ([]rawInputAction, error) {
	var inputs []rawInputAction
	if err := json.Unmarshal(raw, &inputs); err == nil {
		return inputs, nil
	}
	var resp struct {
		PendingInputActions []rawInputAction `json:"pendingInputActions"`
		Inputs              []rawInputAction `json:"inputs"`
		Actions             []rawInputAction `json:"actions"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	switch {
	case len(resp.PendingInputActions) > 0:
		return resp.PendingInputActions, nil
	case len(resp.Inputs) > 0:
		return resp.Inputs, nil
	case len(resp.Actions) > 0:
		return resp.Actions, nil
	default:
		var single rawInputAction
		if err := json.Unmarshal(raw, &single); err != nil {
			return nil, err
		}
		if single.ID != "" || single.Name != "" || single.Message != "" || single.ProceedURL != "" || len(single.Parameters) > 0 || len(single.Inputs) > 0 {
			return []rawInputAction{single}, nil
		}
		return []rawInputAction{}, nil
	}
}

func rawInputActionsToInputActions(inputs []rawInputAction) []InputAction {
	items := make([]InputAction, 0, len(inputs))
	for _, item := range inputs {
		rawParams := append([]rawInputParameter{}, item.Parameters...)
		rawParams = append(rawParams, item.Inputs...)
		params := make([]InputParameter, 0, len(rawParams))
		for _, param := range rawParams {
			params = append(params, InputParameter{
				Name:         strings.TrimSpace(param.Name),
				Type:         strings.TrimSpace(param.Type),
				Description:  strings.TrimSpace(param.Description),
				DefaultValue: rawInputDefaultValue(param),
				Choices:      param.Choices,
			})
		}
		items = append(items, InputAction{
			ID:         strings.TrimSpace(item.ID),
			Name:       strings.TrimSpace(item.Name),
			Message:    strings.TrimSpace(item.Message),
			ProceedURL: strings.TrimSpace(item.ProceedURL),
			AbortURL:   strings.TrimSpace(item.AbortURL),
			Parameters: params,
		})
	}
	return items
}

func rawInputDefaultValue(param rawInputParameter) string {
	if param.DefaultValue != nil {
		return stringifyInputDefaultValue(param.DefaultValue)
	}
	if param.DefaultParameterValue.Value != nil {
		return stringifyInputDefaultValue(param.DefaultParameterValue.Value)
	}
	return stringifyInputDefaultValue(param.Value)
}

func (m *Manager) ProceedInput(ctx context.Context, jobFullName string, buildNumber int64, inputID string, params map[string]string) error {
	input, err := m.findInput(ctx, jobFullName, buildNumber, inputID)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return err
	}
	endpoint := firstNonEmpty(input.ProceedURL, input.ID)
	if endpoint == "" {
		endpoint = fmt.Sprintf("%s/%d/input/%s/proceed", jobURLPath(jobFullName), buildNumber, url.PathEscape(input.ID))
	} else if !strings.Contains(endpoint, "/") {
		endpoint = fmt.Sprintf("%s/%d/input/%s/proceed", jobURLPath(jobFullName), buildNumber, url.PathEscape(endpoint))
	}
	form := url.Values{}
	if len(input.Parameters) > 0 {
		form.Set("json", inputParametersJSON(input.Parameters, params))
	} else {
		form.Set("json", `{"parameter":[]}`)
	}
	return m.postForm(ctx, normalizeJenkinsEndpoint(endpoint), form)
}

func (m *Manager) AbortInput(ctx context.Context, jobFullName string, buildNumber int64, inputID string) error {
	input, err := m.findInput(ctx, jobFullName, buildNumber, inputID)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return err
	}
	endpoint := firstNonEmpty(input.AbortURL, input.ID)
	if endpoint == "" {
		endpoint = fmt.Sprintf("%s/%d/input/%s/abort", jobURLPath(jobFullName), buildNumber, url.PathEscape(input.ID))
	} else if !strings.Contains(endpoint, "/") {
		endpoint = fmt.Sprintf("%s/%d/input/%s/abort", jobURLPath(jobFullName), buildNumber, url.PathEscape(endpoint))
	}
	return m.postForm(ctx, normalizeJenkinsEndpoint(endpoint), url.Values{})
}

func (m *Manager) StageLog(ctx context.Context, jobFullName string, buildNumber int64, nodeID string) (string, error) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", fmt.Errorf("jenkins stage node id is empty"))
		return "", fmt.Errorf("jenkins stage node id is empty")
	}
	text, err := m.nodeLog(ctx, jobFullName, buildNumber, nodeID)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return "", err
	}
	childText, err := m.stageChildLogs(ctx, jobFullName, buildNumber, nodeID)
	if err != nil || strings.TrimSpace(childText) == "" {
		return text, nil
	}
	switch {
	case strings.TrimSpace(text) == "":
		return childText, nil
	case stageLogLooksLikeScaffold(text):
		return childText, nil
	case strings.Contains(childText, text):
		return childText, nil
	case strings.Contains(text, childText):
		return text, nil
	case len([]byte(childText)) > len([]byte(text)):
		return childText, nil
	default:
		return text, nil
	}
}

func (m *Manager) StageChildStatuses(ctx context.Context, jobFullName string, buildNumber int64, nodeID string) ([]StageStatus, error) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", fmt.Errorf("jenkins stage node id is empty"))
		return nil, fmt.Errorf("jenkins stage node id is empty")
	}
	var desc stageNodeDesc
	endpoint := fmt.Sprintf("%s/%d/execution/node/%s/wfapi/describe", jobURLPath(jobFullName), buildNumber, url.PathEscape(nodeID))
	if err := m.getJSON(ctx, endpoint, &desc); err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return nil, err
	}
	items := make([]StageStatus, 0, len(desc.StageFlowNodes))
	for _, child := range desc.StageFlowNodes {
		childID := strings.TrimSpace(child.ID)
		if childID == "" || childID == nodeID {
			continue
		}
		items = append(items, StageStatus{
			ID:       childID,
			Name:     strings.TrimSpace(child.Name),
			Status:   strings.TrimSpace(child.Status),
			Type:     strings.TrimSpace(child.Type),
			Duration: child.Duration,
			Start:    child.Start,
		})
	}
	return items, nil
}

func (m *Manager) NodeProgressiveLog(ctx context.Context, jobFullName string, buildNumber int64, nodeID string, start int64) (string, int64, bool, error) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", fmt.Errorf("jenkins stage node id is empty"))
		return "", start, false, fmt.Errorf("jenkins stage node id is empty")
	}
	var resp stageLogResp
	endpoint := fmt.Sprintf("%s/%d/execution/node/%s/wfapi/log?start=%d", jobURLPath(jobFullName), buildNumber, url.PathEscape(nodeID), start)
	if err := m.getJSON(ctx, endpoint, &resp); err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return "", start, false, err
	}
	text := cleanJenkinsLogText(resp.Text)
	next := start + int64(len([]byte(resp.Text)))
	if resp.Length > start {
		next = resp.Length
	}
	if next < start {
		next = start
	}
	return text, next, resp.HasMore, nil
}

func (m *Manager) nodeLog(ctx context.Context, jobFullName string, buildNumber int64, nodeID string) (string, error) {
	var builder strings.Builder
	var start int64
	for attempt := 0; attempt < 20; attempt++ {
		text, next, more, err := m.NodeProgressiveLog(ctx, jobFullName, buildNumber, nodeID, start)
		if err != nil {
			logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
			return "", err
		}
		if text != "" {
			builder.WriteString(text)
		}
		if !more || next <= start {
			break
		}
		start = next
	}
	return builder.String(), nil
}

func (m *Manager) stageChildLogs(ctx context.Context, jobFullName string, buildNumber int64, nodeID string) (string, error) {
	children, err := m.StageChildStatuses(ctx, jobFullName, buildNumber, nodeID)
	if err != nil {
		return "", err
	}
	var builder strings.Builder
	for _, child := range children {
		childID := strings.TrimSpace(child.ID)
		text, err := m.nodeLog(ctx, jobFullName, buildNumber, childID)
		if err != nil || strings.TrimSpace(text) == "" {
			continue
		}
		name := strings.TrimSpace(child.Name)
		if name != "" {
			if builder.Len() > 0 {
				builder.WriteString("\n")
			}
			builder.WriteString("[" + name + "]\n")
		}
		builder.WriteString(text)
		if !strings.HasSuffix(text, "\n") {
			builder.WriteString("\n")
		}
	}
	return builder.String(), nil
}

func stageLogLooksLikeScaffold(text string) bool {
	lines := strings.Split(strings.ReplaceAll(strings.TrimSpace(text), "\r\n", "\n"), "\n")
	meaningful := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		meaningful = append(meaningful, line)
		if len(meaningful) > 6 {
			return false
		}
	}
	if len(meaningful) == 0 {
		return false
	}
	for _, line := range meaningful {
		if !strings.Contains(line, "[Pipeline]") {
			return false
		}
	}
	return true
}

func (m *Manager) ListAgents(ctx context.Context) ([]AgentNode, error) {
	var resp struct {
		Computer []struct {
			DisplayName        string `json:"displayName"`
			Offline            bool   `json:"offline"`
			TemporarilyOffline bool   `json:"temporarilyOffline"`
			AssignedLabels     []struct {
				Name string `json:"name"`
			} `json:"assignedLabels"`
		} `json:"computer"`
	}
	tree := "computer[displayName,offline,temporarilyOffline,assignedLabels[name]]"
	if err := m.getJSON(ctx, "/computer/api/json?tree="+url.QueryEscape(tree), &resp); err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return nil, err
	}
	items := make([]AgentNode, 0, len(resp.Computer)+2)
	seen := make(map[string]struct{})
	addAgent := func(item AgentNode) {
		key := strings.TrimSpace(item.Label)
		if key == "" {
			key = strings.TrimSpace(item.Name)
		}
		if key == "" {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		items = append(items, item)
	}
	addAgent(AgentNode{
		Name:        "any",
		DisplayName: "任意可用节点",
		Label:       "",
		Labels:      []string{"any"},
	})
	for _, item := range resp.Computer {
		name := strings.TrimSpace(item.DisplayName)
		if name == "" {
			name = "built-in"
		}
		labels := make([]string, 0, len(item.AssignedLabels))
		for _, label := range item.AssignedLabels {
			labelName := strings.TrimSpace(label.Name)
			if labelName != "" && !containsString(labels, labelName) {
				labels = append(labels, labelName)
			}
		}
		if isBuiltInNode(name, labels) {
			addAgent(AgentNode{
				Name:               "master",
				DisplayName:        "master",
				Label:              "master",
				Labels:             []string{"master"},
				Offline:            item.Offline,
				TemporarilyOffline: item.TemporarilyOffline,
			})
			addAgent(AgentNode{
				Name:               "built-in",
				DisplayName:        "Built-In Node",
				Label:              "built-in",
				Labels:             []string{"built-in"},
				Offline:            item.Offline,
				TemporarilyOffline: item.TemporarilyOffline,
			})
			continue
		}
		labelText := strings.Join(labels, " && ")
		if labelText == "" {
			labelText = name
		}
		addAgent(AgentNode{
			Name:               name,
			DisplayName:        item.DisplayName,
			Label:              labelText,
			Labels:             labels,
			Offline:            item.Offline,
			TemporarilyOffline: item.TemporarilyOffline,
		})
	}
	return items, nil
}

func (m *Manager) findInput(ctx context.Context, jobFullName string, buildNumber int64, inputID string) (InputAction, error) {
	inputID = strings.TrimSpace(inputID)
	if inputID == "" {
		return InputAction{}, fmt.Errorf("jenkins input id is empty")
	}
	items, err := m.PendingInputs(ctx, jobFullName, buildNumber)
	if err != nil {
		return InputAction{}, err
	}
	for _, item := range items {
		if item.ID == inputID || item.Name == inputID {
			return item, nil
		}
	}
	return InputAction{}, fmt.Errorf("jenkins input not found")
}

func inputParametersJSON(defs []InputParameter, values map[string]string) string {
	parameters := make([]map[string]string, 0, len(defs))
	for _, item := range defs {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		value := ""
		if values != nil {
			value = values[name]
		}
		if value == "" {
			value = item.DefaultValue
		}
		parameters = append(parameters, map[string]string{"name": name, "value": value})
	}
	raw, _ := json.Marshal(map[string]any{"parameter": parameters})
	return string(raw)
}

func stringifyInputDefaultValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case bool:
		return strconv.FormatBool(v)
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case json.Number:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}

func normalizeJenkinsEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return endpoint
	}
	if parsed, err := url.Parse(endpoint); err == nil && parsed.IsAbs() {
		if parsed.RawQuery != "" {
			return parsed.Path + "?" + parsed.RawQuery
		}
		return parsed.Path
	}
	if !strings.HasPrefix(endpoint, "/") {
		return "/" + endpoint
	}
	return endpoint
}

func isBuiltInNode(name string, labels []string) bool {
	if strings.EqualFold(name, "Built-In Node") || strings.EqualFold(name, "built-in") || strings.EqualFold(name, "master") {
		return true
	}
	return containsStringFold(labels, "built-in") || containsStringFold(labels, "master")
}

func containsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func containsStringFold(items []string, value string) bool {
	for _, item := range items {
		if strings.EqualFold(item, value) {
			return true
		}
	}
	return false
}

func (m *Manager) getJSON(ctx context.Context, endpoint string, out any) error {
	req, err := m.newRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return err
	}
	resp, err := m.do(req)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", fmt.Errorf("jenkins request failed: %s", resp.Status))
		return fmt.Errorf("jenkins request failed: %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (m *Manager) postForm(ctx context.Context, endpoint string, form url.Values) error {
	req, err := m.newRequest(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := m.do(req)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 65536))
	return jenkinsHTTPError("jenkins request failed", resp.Status, body)
}

func (m *Manager) newRequest(ctx context.Context, method, endpoint string, body io.Reader) (*http.Request, error) {
	fullURL := m.endpoint + endpoint
	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return nil, err
	}
	m.applyAuth(req)
	if needsCrumb(method) {
		if err := m.applyCrumb(ctx, req); err != nil {
			logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
			return nil, err
		}
	}
	return req, nil
}

func (m *Manager) do(req *http.Request) (*http.Response, error) {
	return m.client.Do(req)
}

func (m *Manager) applyAuth(req *http.Request) {
	if m.username != "" && (m.token != "" || m.password != "") {
		secret := m.token
		if secret == "" {
			secret = m.password
		}
		req.SetBasicAuth(m.username, secret)
		return
	}
	if m.token != "" {
		req.Header.Set("Authorization", "Bearer "+m.token)
	}
}

func needsCrumb(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}

func (m *Manager) applyCrumb(ctx context.Context, req *http.Request) error {
	field, crumb, err := m.getCrumb(ctx)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return err
	}
	if field != "" && crumb != "" {
		req.Header.Set(field, crumb)
	}
	return nil
}

func (m *Manager) getCrumb(ctx context.Context) (string, string, error) {
	m.crumbMu.Lock()
	defer m.crumbMu.Unlock()

	if m.crumbUnsupported {
		return "", "", nil
	}
	if m.crumbField != "" && m.crumbValue != "" && time.Since(m.crumbFetchedAt) < 30*time.Minute {
		return m.crumbField, m.crumbValue, nil
	}

	var resp *http.Response
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.endpoint+"/crumbIssuer/api/json", nil)
		if err != nil {
			logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
			return "", "", err
		}
		m.applyAuth(req)
		resp, lastErr = m.client.Do(req)
		if lastErr == nil {
			break
		}
		if attempt == 2 {
			break
		}
		// Jenkins 重启时 NodePort 可能短暂拒绝连接，crumb 获取做轻量重试。
		select {
		case <-ctx.Done():
			return "", "", ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 500 * time.Millisecond):
		}
	}
	if lastErr != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", lastErr)
		return "", "", fmt.Errorf("jenkins crumb request failed: %w", lastErr)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		m.crumbUnsupported = true
		return "", "", nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 65536))
		return "", "", jenkinsHTTPError("jenkins crumb request failed", resp.Status, body)
	}

	var result struct {
		CrumbRequestField string `json:"crumbRequestField"`
		Crumb             string `json:"crumb"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", err)
		return "", "", err
	}
	field := strings.TrimSpace(result.CrumbRequestField)
	if field == "" {
		field = "Jenkins-Crumb"
	}
	crumb := strings.TrimSpace(result.Crumb)
	if crumb == "" {
		return "", "", nil
	}
	m.crumbField = field
	m.crumbValue = crumb
	m.crumbFetchedAt = time.Now()
	return m.crumbField, m.crumbValue, nil
}

func folderURLPath(folder string) string {
	folder = strings.Trim(folder, "/")
	if folder == "" {
		return ""
	}
	parts := strings.Split(folder, "/")
	out := ""
	for _, part := range parts {
		if part == "" {
			continue
		}
		out += "/job/" + url.PathEscape(part)
	}
	return out
}

func jobURLPath(fullName string) string {
	return folderURLPath(fullName)
}

func escapePathSegments(value string) string {
	parts := strings.Split(value, "/")
	for index := range parts {
		parts[index] = url.PathEscape(parts[index])
	}
	return strings.Join(parts, "/")
}

func queueIDFromLocation(location string) string {
	location = strings.TrimRight(location, "/")
	idx := strings.LastIndex(location, "/")
	if idx < 0 {
		return ""
	}
	return location[idx+1:]
}

func (m *Manager) externalURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	base, err := url.Parse(m.endpoint)
	if err != nil || strings.TrimSpace(base.Scheme) == "" || strings.TrimSpace(base.Host) == "" {
		return raw
	}
	target, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	target.Scheme = base.Scheme
	target.Host = base.Host
	target.RawPath = ""
	target.Path = mergeEndpointPath(base.Path, target.Path)
	return target.String()
}

func mergeEndpointPath(basePath, targetPath string) string {
	basePath = strings.TrimRight(strings.TrimSpace(basePath), "/")
	targetPath = strings.TrimSpace(targetPath)
	if targetPath == "" {
		if basePath == "" {
			return ""
		}
		return basePath
	}
	if !strings.HasPrefix(targetPath, "/") {
		targetPath = "/" + targetPath
	}
	if basePath == "" || targetPath == basePath || strings.HasPrefix(targetPath, basePath+"/") {
		return targetPath
	}
	return basePath + targetPath
}

func pipelineJobXML(displayName, script string) string {
	return pipelineJobXMLWithParams(displayName, script, nil)
}

func credentialConfigXML(credential CredentialConfig) (string, error) {
	credential.Type = strings.TrimSpace(credential.Type)
	switch credential.Type {
	case "username_password":
		secret := firstNonEmpty(credential.Password, credential.Token, credential.SecretText)
		return fmt.Sprintf(`<?xml version='1.1' encoding='UTF-8'?>
<com.cloudbees.plugins.credentials.impl.UsernamePasswordCredentialsImpl>
  <scope>GLOBAL</scope>
  <id>%s</id>
  <description>%s</description>
  <username>%s</username>
  <password>%s</password>
</com.cloudbees.plugins.credentials.impl.UsernamePasswordCredentialsImpl>`,
			xmlEscape(credential.ID),
			xmlEscape(credential.Description),
			xmlEscape(credential.Username),
			xmlEscape(secret),
		), nil
	case "token", "secret_text", "kubeconfig", "certificate", "json":
		secret := firstNonEmpty(credential.Token, credential.SecretText, credential.Kubeconfig, credential.Certificate, credential.JsonData, credential.Password)
		return fmt.Sprintf(`<?xml version='1.1' encoding='UTF-8'?>
<org.jenkinsci.plugins.plaincredentials.impl.StringCredentialsImpl>
  <scope>GLOBAL</scope>
  <id>%s</id>
  <description>%s</description>
  <secret>%s</secret>
</org.jenkinsci.plugins.plaincredentials.impl.StringCredentialsImpl>`,
			xmlEscape(credential.ID),
			xmlEscape(credential.Description),
			xmlEscape(secret),
		), nil
	case "secret_file":
		secret := firstNonEmpty(credential.SecretText, credential.Kubeconfig, credential.Certificate, credential.Password, credential.Token)
		return fmt.Sprintf(`<?xml version='1.1' encoding='UTF-8'?>
<org.jenkinsci.plugins.plaincredentials.impl.FileCredentialsImpl>
  <scope>GLOBAL</scope>
  <id>%s</id>
  <description>%s</description>
  <fileName>%s</fileName>
  <secretBytes>%s</secretBytes>
</org.jenkinsci.plugins.plaincredentials.impl.FileCredentialsImpl>`,
			xmlEscape(credential.ID),
			xmlEscape(credential.Description),
			xmlEscape(firstNonEmpty(credential.FileName, credential.ID)),
			xmlEscape(secret),
		), nil
	case "ssh_key":
		return fmt.Sprintf(`<?xml version='1.1' encoding='UTF-8'?>
<com.cloudbees.jenkins.plugins.sshcredentials.impl.BasicSSHUserPrivateKey>
  <scope>GLOBAL</scope>
  <id>%s</id>
  <description>%s</description>
  <username>%s</username>
  <privateKeySource class="com.cloudbees.jenkins.plugins.sshcredentials.impl.BasicSSHUserPrivateKey$DirectEntryPrivateKeySource">
    <privateKey>%s</privateKey>
  </privateKeySource>
  <passphrase>%s</passphrase>
</com.cloudbees.jenkins.plugins.sshcredentials.impl.BasicSSHUserPrivateKey>`,
			xmlEscape(credential.ID),
			xmlEscape(credential.Description),
			xmlEscape(credential.Username),
			xmlEscape(credential.PrivateKey),
			xmlEscape(credential.Passphrase),
		), nil
	default:
		return "", fmt.Errorf("unsupported jenkins credential type: %s", credential.Type)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func credentialCreateForm(credential CredentialConfig) (url.Values, error) {
	credentials, err := credentialCreatePayload(credential)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(map[string]any{
		"":            "0",
		"credentials": credentials,
	})
	if err != nil {
		return nil, err
	}
	form := url.Values{}
	form.Set("json", string(data))
	return form, nil
}

func credentialCreatePayload(credential CredentialConfig) (map[string]any, error) {
	credential.Type = strings.TrimSpace(credential.Type)
	base := map[string]any{
		"scope":       "GLOBAL",
		"id":          credential.ID,
		"description": credential.Description,
		"$redact":     "secret",
	}
	switch credential.Type {
	case "username_password":
		base["stapler-class"] = "com.cloudbees.plugins.credentials.impl.UsernamePasswordCredentialsImpl"
		base["$class"] = "com.cloudbees.plugins.credentials.impl.UsernamePasswordCredentialsImpl"
		base["username"] = credential.Username
		base["password"] = firstNonEmpty(credential.Password, credential.Token, credential.SecretText)
		return base, nil
	case "token", "secret_text", "kubeconfig", "certificate", "json":
		base["stapler-class"] = "org.jenkinsci.plugins.plaincredentials.impl.StringCredentialsImpl"
		base["$class"] = "org.jenkinsci.plugins.plaincredentials.impl.StringCredentialsImpl"
		base["secret"] = firstNonEmpty(credential.Token, credential.SecretText, credential.Kubeconfig, credential.Certificate, credential.JsonData, credential.Password)
		return base, nil
	case "secret_file":
		base["stapler-class"] = "org.jenkinsci.plugins.plaincredentials.impl.FileCredentialsImpl"
		base["$class"] = "org.jenkinsci.plugins.plaincredentials.impl.FileCredentialsImpl"
		base["fileName"] = firstNonEmpty(credential.FileName, credential.ID)
		base["secretBytes"] = firstNonEmpty(credential.SecretText, credential.Kubeconfig, credential.Certificate, credential.Password, credential.Token)
		return base, nil
	case "ssh_key":
		base["stapler-class"] = "com.cloudbees.jenkins.plugins.sshcredentials.impl.BasicSSHUserPrivateKey"
		base["$class"] = "com.cloudbees.jenkins.plugins.sshcredentials.impl.BasicSSHUserPrivateKey"
		base["username"] = credential.Username
		base["privateKeySource"] = map[string]any{
			"stapler-class": "com.cloudbees.jenkins.plugins.sshcredentials.impl.BasicSSHUserPrivateKey$DirectEntryPrivateKeySource",
			"$class":        "com.cloudbees.jenkins.plugins.sshcredentials.impl.BasicSSHUserPrivateKey$DirectEntryPrivateKeySource",
			"privateKey":    credential.PrivateKey,
		}
		base["passphrase"] = credential.Passphrase
		return base, nil
	default:
		return nil, fmt.Errorf("unsupported jenkins credential type: %s", credential.Type)
	}
}

func pipelineJobXMLWithParams(displayName, script string, params []JenkinsRenderParam) string {
	escapedDisplay := xmlEscape(displayName)
	escapedScript := xmlEscape(script)
	properties := "<properties/>"
	if paramXML := pipelineJobParameterXML(params); paramXML != "" {
		properties = "<properties>\n" + paramXML + "\n  </properties>"
	}
	return fmt.Sprintf(`<?xml version='1.1' encoding='UTF-8'?>
<flow-definition plugin="workflow-job">
  <actions/>
  <description>%s</description>
  <keepDependencies>false</keepDependencies>
  %s
  <definition class="org.jenkinsci.plugins.workflow.cps.CpsFlowDefinition" plugin="workflow-cps">
    <script>%s</script>
    <sandbox>true</sandbox>
  </definition>
  <triggers/>
  <disabled>false</disabled>
</flow-definition>`, escapedDisplay, properties, escapedScript)
}

func pipelineJobParameterXML(params []JenkinsRenderParam) string {
	defs := make([]string, 0, len(params))
	for _, item := range params {
		if effectiveParamRuntimeMode(item) != "params" || strings.TrimSpace(item.Code) == "" {
			continue
		}
		value := item.CurrentValue
		if value == "" {
			value = item.DefaultValue
		}
		name := xmlEscape(item.Code)
		desc := xmlEscape(item.Description)
		defaultValue := xmlEscape(value)
		switch item.ParamType {
		case "booleanParam":
			if value != "true" {
				defaultValue = "false"
			}
			defs = append(defs, fmt.Sprintf(`      <hudson.model.BooleanParameterDefinition>
        <name>%s</name>
        <description>%s</description>
        <defaultValue>%s</defaultValue>
      </hudson.model.BooleanParameterDefinition>`, name, desc, defaultValue))
		case "number":
			if strings.TrimSpace(value) != "" {
				if _, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err != nil {
					logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", fmt.Errorf("number 参数「%s」默认值必须是合法数字", item.Code))
				}
			}
			defs = append(defs, fmt.Sprintf(`      <hudson.model.StringParameterDefinition>
        <name>%s</name>
        <description>%s</description>
        <defaultValue>%s</defaultValue>
        <trim>true</trim>
      </hudson.model.StringParameterDefinition>`, name, desc, defaultValue))
		case "text":
			defs = append(defs, fmt.Sprintf(`      <hudson.model.TextParameterDefinition>
        <name>%s</name>
        <description>%s</description>
        <defaultValue>%s</defaultValue>
      </hudson.model.TextParameterDefinition>`, name, desc, defaultValue))
		case "password":
			defs = append(defs, fmt.Sprintf(`      <hudson.model.PasswordParameterDefinition>
        <name>%s</name>
        <description>%s</description>
        <defaultValue>%s</defaultValue>
      </hudson.model.PasswordParameterDefinition>`, name, desc, defaultValue))
		case "file":
			defs = append(defs, fmt.Sprintf(`      <hudson.model.FileParameterDefinition>
        <name>%s</name>
        <description>%s</description>
      </hudson.model.FileParameterDefinition>`, name, desc))
		case "choice":
			choices := renderParamChoices(item)
			if len(choices) == 0 {
				choices = []string{value}
			}
			defs = append(defs, fmt.Sprintf(`      <hudson.model.ChoiceParameterDefinition>
        <name>%s</name>
        <description>%s</description>
        <choices class="java.util.Arrays$ArrayList">
          <a class="string-array">
%s
          </a>
        </choices>
      </hudson.model.ChoiceParameterDefinition>`, name, desc, choiceXMLValues(choices)))
		default:
			defs = append(defs, fmt.Sprintf(`      <hudson.model.StringParameterDefinition>
        <name>%s</name>
        <description>%s</description>
        <defaultValue>%s</defaultValue>
        <trim>true</trim>
      </hudson.model.StringParameterDefinition>`, name, desc, defaultValue))
		}
	}
	if len(defs) == 0 {
		return ""
	}
	return "    <hudson.model.ParametersDefinitionProperty>\n      <parameterDefinitions>\n" + strings.Join(defs, "\n") + "\n      </parameterDefinitions>\n    </hudson.model.ParametersDefinitionProperty>"
}

func choiceXMLValues(items []string) string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, "            <string>"+xmlEscape(item)+"</string>")
	}
	return strings.Join(lines, "\n")
}

func xmlEscape(value string) string {
	value = sanitizeXMLText(value)
	value = strings.ReplaceAll(value, "&", "&amp;")
	value = strings.ReplaceAll(value, "<", "&lt;")
	value = strings.ReplaceAll(value, ">", "&gt;")
	value = strings.ReplaceAll(value, `"`, "&quot;")
	value = strings.ReplaceAll(value, "'", "&apos;")
	return value
}

func sanitizeXMLText(value string) string {
	value = strings.ToValidUTF8(value, "")
	return strings.Map(func(r rune) rune {
		switch r {
		case '\t', '\n', '\r':
			return r
		}
		if r < 0x20 || (r >= 0x7f && r <= 0x9f) {
			return -1
		}
		return r
	}, value)
}

var (
	htmlTagPattern            = regexp.MustCompile(`(?s)<[^>]+>`)
	hiddenHTMLPattern         = regexp.MustCompile(`(?is)<span[^>]*style=["'][^"']*display\s*:\s*none[^"']*["'][^>]*>.*?</span>`)
	htmlLineBreakPattern      = regexp.MustCompile(`(?i)<br\s*/?>`)
	ansiConcealedPattern      = regexp.MustCompile(`(?s)\x1b\[8m.*?(?:\x1b\[0m|$)`)
	ansiOSCSequencePattern    = regexp.MustCompile(`\x1b\][^\x07]*(?:\x07|\x1b\\)`)
	ansiCSISequencePattern    = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)
	jenkinsConsoleNotePattern = regexp.MustCompile(`ha:////[A-Za-z0-9+/=\r\n]+`)
	logSpacePattern           = regexp.MustCompile(`[ \t]{2,}`)
	stageLogHeaderPattern     = regexp.MustCompile(`^\[(Print Message|Shell Script|Pipeline Script)\]$`)
)

// CleanLogText 清理 Jenkins 日志中的隐藏标记和控制字符。
func CleanLogText(raw string) string {
	return cleanJenkinsLogText(raw)
}

func cleanJenkinsLogText(raw string) string {
	raw = strings.ToValidUTF8(raw, "")
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	raw = html.UnescapeString(raw)
	raw = ansiConcealedPattern.ReplaceAllString(raw, "")
	raw = ansiOSCSequencePattern.ReplaceAllString(raw, "")
	raw = ansiCSISequencePattern.ReplaceAllString(raw, "")
	raw = jenkinsConsoleNotePattern.ReplaceAllString(raw, "")
	raw = cleanLogControlChars(raw)
	raw = hiddenHTMLPattern.ReplaceAllString(raw, "")
	raw = htmlLineBreakPattern.ReplaceAllString(raw, "\n")
	raw = htmlTagPattern.ReplaceAllString(raw, "")
	raw = strings.ReplaceAll(raw, "\u00a0", " ")

	lines := strings.Split(raw, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(logSpacePattern.ReplaceAllString(line, " "))
		if line == "" {
			if len(cleaned) > 0 && cleaned[len(cleaned)-1] != "" {
				cleaned = append(cleaned, "")
			}
			continue
		}
		if stageLogHeaderPattern.MatchString(line) {
			continue
		}
		cleaned = append(cleaned, line)
	}
	text := strings.TrimSpace(strings.Join(cleaned, "\n"))
	if text == "" {
		return ""
	}
	return text + "\n"
}

func cleanLogControlChars(value string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\t':
			return r
		}
		if r < 0x20 || (r >= 0x7f && r <= 0x9f) {
			return -1
		}
		return r
	}, value)
}

func jenkinsHTTPError(prefix, status string, body []byte) error {
	message := cleanJenkinsErrorBody(string(body))
	if message == "" {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", fmt.Errorf("%s: %s", prefix, status))
		return fmt.Errorf("%s: %s", prefix, status)
	}
	logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/api_manager.go", fmt.Errorf("%s: %s %s", prefix, status, message))
	return fmt.Errorf("%s: %s %s", prefix, status, message)
}

func cleanJenkinsErrorBody(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	candidates := make([]string, 0, 5)
	for _, tag := range []string{"pre", "h2", "h1", "title"} {
		if text := extractHTMLTag(raw, tag); text != "" {
			candidates = append(candidates, text)
		}
	}
	text := htmlTagPattern.ReplaceAllString(raw, " ")
	text = html.UnescapeString(text)
	text = strings.Join(strings.Fields(text), " ")
	if text != "" {
		candidates = append(candidates, text)
	}
	for _, item := range candidates {
		if !isGenericJenkinsErrorText(item) {
			return truncateErrorText(item, 500)
		}
	}
	return ""
}

func isGenericJenkinsErrorText(text string) bool {
	normalized := strings.ToLower(strings.TrimSpace(text))
	switch normalized {
	case "", "oops!", "jenkins", "jenkins - jenkins":
		return true
	}
	return normalized == "server error" || strings.HasPrefix(normalized, "error 500")
}

func extractHTMLTag(raw, tag string) string {
	pattern := regexp.MustCompile(`(?is)<` + tag + `[^>]*>(.*?)</` + tag + `>`)
	matches := pattern.FindStringSubmatch(raw)
	if len(matches) < 2 {
		return ""
	}
	text := htmlTagPattern.ReplaceAllString(matches[1], " ")
	text = html.UnescapeString(text)
	return strings.Join(strings.Fields(text), " ")
}

func truncateErrorText(text string, max int) string {
	text = strings.TrimSpace(text)
	if len([]rune(text)) <= max {
		return text
	}
	runes := []rune(text)
	return string(runes[:max]) + "..."
}
