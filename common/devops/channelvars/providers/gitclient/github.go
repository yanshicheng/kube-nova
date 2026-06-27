package gitclient

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v57/github"
	"golang.org/x/oauth2"
)

// GitHubClient GitHub客户端实现
type GitHubClient struct {
	client *github.Client
}

// NewGitHubClient 创建GitHub客户端
func NewGitHubClient(token string, baseURL string) (*GitHubClient, error) {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	var client *github.Client
	if baseURL != "" && baseURL != "github.com" {
		// GitHub Enterprise
		if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
			baseURL = "https://" + baseURL
		}
		var err error
		client, err = github.NewEnterpriseClient(baseURL, baseURL, tc)
		if err != nil {
			return nil, fmt.Errorf("create github enterprise client: %w", err)
		}
	} else {
		client = github.NewClient(tc)
	}

	return &GitHubClient{client: client}, nil
}

// ListProjects 列出项目（GitHub中为仓库）
func (c *GitHubClient) ListProjects(ctx context.Context, search string, page, perPage int) ([]Project, error) {
	// GitHub使用搜索API
	query := search
	if query == "" {
		query = "stars:>0" // 默认查询有星标的仓库
	}

	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{
			Page:    page,
			PerPage: perPage,
		},
	}

	result, _, err := c.client.Search.Repositories(ctx, query, opts)
	if err != nil {
		return nil, fmt.Errorf("search github repositories: %w", err)
	}

	projects := make([]Project, 0, len(result.Repositories))
	for _, repo := range result.Repositories {
		project := Project{
			ID:                int(repo.GetID()),
			Name:              repo.GetName(),
			Path:              repo.GetName(),
			PathWithNamespace: repo.GetFullName(),
			FullName:          repo.GetFullName(),
			Description:       repo.GetDescription(),
			HTTPURL:           repo.GetCloneURL(),
			SSHURL:            repo.GetSSHURL(),
		}
		projects = append(projects, project)
	}

	return projects, nil
}

// GetProject 获取项目详情
func (c *GitHubClient) GetProject(ctx context.Context, projectID string) (*Project, error) {
	// GitHub的projectID格式为 "owner/repo"
	parts := strings.Split(projectID, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid github project id format: %s", projectID)
	}
	owner, repo := parts[0], parts[1]

	repository, _, err := c.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("get github repository: %w", err)
	}

	return &Project{
		ID:                int(repository.GetID()),
		Name:              repository.GetName(),
		Path:              repository.GetName(),
		PathWithNamespace: repository.GetFullName(),
		FullName:          repository.GetFullName(),
		Description:       repository.GetDescription(),
		HTTPURL:           repository.GetCloneURL(),
		SSHURL:            repository.GetSSHURL(),
	}, nil
}

// ListBranches 列出分支
func (c *GitHubClient) ListBranches(ctx context.Context, projectID string, search string, page, perPage int) ([]Branch, error) {
	// GitHub的projectID格式为 "owner/repo"
	parts := strings.Split(projectID, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid github project id format: %s", projectID)
	}
	owner, repo := parts[0], parts[1]

	opts := &github.BranchListOptions{
		ListOptions: github.ListOptions{
			Page:    page,
			PerPage: perPage,
		},
	}

	branches, _, err := c.client.Repositories.ListBranches(ctx, owner, repo, opts)
	if err != nil {
		return nil, fmt.Errorf("list github branches: %w", err)
	}

	result := make([]Branch, 0, len(branches))
	for _, b := range branches {
		branch := Branch{
			Name:      b.GetName(),
			Protected: b.GetProtected(),
		}
		if commit := b.GetCommit(); commit != nil {
			branch.Commit = Commit{
				ID:      commit.GetSHA(),
				ShortID: commit.GetSHA()[:7],
			}
		}
		result = append(result, branch)
	}

	// 如果有搜索条件，过滤分支
	if search != "" {
		filtered := make([]Branch, 0)
		for _, b := range result {
			if strings.Contains(b.Name, search) {
				filtered = append(filtered, b)
			}
		}
		result = filtered
	}

	return result, nil
}

// ListTags 列出标签
func (c *GitHubClient) ListTags(ctx context.Context, projectID string, search string, page, perPage int) ([]Tag, error) {
	// GitHub的projectID格式为 "owner/repo"
	parts := strings.Split(projectID, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid github project id format: %s", projectID)
	}
	owner, repo := parts[0], parts[1]

	opts := &github.ListOptions{
		Page:    page,
		PerPage: perPage,
	}

	tags, _, err := c.client.Repositories.ListTags(ctx, owner, repo, opts)
	if err != nil {
		return nil, fmt.Errorf("list github tags: %w", err)
	}

	result := make([]Tag, 0, len(tags))
	for _, t := range tags {
		tag := Tag{
			Name: t.GetName(),
		}
		if commit := t.GetCommit(); commit != nil {
			tag.Commit = Commit{
				ID:      commit.GetSHA(),
				ShortID: commit.GetSHA()[:7],
			}
		}
		result = append(result, tag)
	}

	// 如果有搜索条件，过滤标签
	if search != "" {
		filtered := make([]Tag, 0)
		for _, t := range result {
			if strings.Contains(t.Name, search) {
				filtered = append(filtered, t)
			}
		}
		result = filtered
	}

	return result, nil
}
