package gitclient

import "context"

// Client 统一的Git客户端接口
type Client interface {
	// ListProjects 列出项目
	ListProjects(ctx context.Context, search string, page, perPage int) ([]Project, error)

	// GetProject 获取项目详情
	GetProject(ctx context.Context, projectID string) (*Project, error)

	// ListBranches 列出分支
	ListBranches(ctx context.Context, projectID string, search string, page, perPage int) ([]Branch, error)

	// ListTags 列出标签
	ListTags(ctx context.Context, projectID string, search string, page, perPage int) ([]Tag, error)
}

// Project 项目信息
type Project struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	Path              string `json:"path"`
	PathWithNamespace string `json:"path_with_namespace"` // GitLab: group/project, GitHub: owner/repo
	FullName          string `json:"full_name"`           // 同PathWithNamespace
	Description       string `json:"description"`
	HTTPURL           string `json:"http_url"`
	SSHURL            string `json:"ssh_url"`
}

// Branch 分支信息
type Branch struct {
	Name      string `json:"name"`
	Protected bool   `json:"protected"`
	Commit    Commit `json:"commit"`
}

// Tag 标签信息
type Tag struct {
	Name    string `json:"name"`
	Message string `json:"message"`
	Commit  Commit `json:"commit"`
}

// Commit 提交信息
type Commit struct {
	ID        string `json:"id"`
	ShortID   string `json:"short_id"`
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
}
