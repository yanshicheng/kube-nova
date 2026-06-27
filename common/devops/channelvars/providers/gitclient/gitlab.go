package gitclient

import (
	"context"
	"fmt"

	"github.com/xanzy/go-gitlab"
)

// GitLabClient GitLab客户端实现
type GitLabClient struct {
	client *gitlab.Client
}

// NewGitLabClient 创建GitLab客户端
func NewGitLabClient(token string, baseURL string) (*GitLabClient, error) {
	opts := []gitlab.ClientOptionFunc{}
	if baseURL != "" && baseURL != "gitlab.com" {
		opts = append(opts, gitlab.WithBaseURL(baseURL))
	}

	client, err := gitlab.NewClient(token, opts...)
	if err != nil {
		return nil, fmt.Errorf("create gitlab client: %w", err)
	}

	return &GitLabClient{client: client}, nil
}

// ListProjects 列出项目
func (c *GitLabClient) ListProjects(ctx context.Context, search string, page, perPage int) ([]Project, error) {
	opts := &gitlab.ListProjectsOptions{
		ListOptions: gitlab.ListOptions{
			Page:    page,
			PerPage: perPage,
		},
		Search: gitlab.Ptr(search),
	}

	projects, _, err := c.client.Projects.ListProjects(opts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("list gitlab projects: %w", err)
	}

	result := make([]Project, 0, len(projects))
	for _, p := range projects {
		result = append(result, Project{
			ID:                p.ID,
			Name:              p.Name,
			Path:              p.Path,
			PathWithNamespace: p.PathWithNamespace,
			FullName:          p.PathWithNamespace,
			Description:       p.Description,
			HTTPURL:           p.HTTPURLToRepo,
			SSHURL:            p.SSHURLToRepo,
		})
	}

	return result, nil
}

// GetProject 获取项目详情
func (c *GitLabClient) GetProject(ctx context.Context, projectID string) (*Project, error) {
	project, _, err := c.client.Projects.GetProject(projectID, nil, gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("get gitlab project: %w", err)
	}

	return &Project{
		ID:                project.ID,
		Name:              project.Name,
		Path:              project.Path,
		PathWithNamespace: project.PathWithNamespace,
		FullName:          project.PathWithNamespace,
		Description:       project.Description,
		HTTPURL:           project.HTTPURLToRepo,
		SSHURL:            project.SSHURLToRepo,
	}, nil
}

// ListBranches 列出分支
func (c *GitLabClient) ListBranches(ctx context.Context, projectID string, search string, page, perPage int) ([]Branch, error) {
	opts := &gitlab.ListBranchesOptions{
		ListOptions: gitlab.ListOptions{
			Page:    page,
			PerPage: perPage,
		},
		Search: gitlab.Ptr(search),
	}

	branches, _, err := c.client.Branches.ListBranches(projectID, opts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("list gitlab branches: %w", err)
	}

	result := make([]Branch, 0, len(branches))
	for _, b := range branches {
		result = append(result, Branch{
			Name:      b.Name,
			Protected: b.Protected,
			Commit: Commit{
				ID:        b.Commit.ID,
				ShortID:   b.Commit.ShortID,
				Message:   b.Commit.Message,
				CreatedAt: b.Commit.CreatedAt.String(),
			},
		})
	}

	return result, nil
}

// ListTags 列出标签
func (c *GitLabClient) ListTags(ctx context.Context, projectID string, search string, page, perPage int) ([]Tag, error) {
	opts := &gitlab.ListTagsOptions{
		ListOptions: gitlab.ListOptions{
			Page:    page,
			PerPage: perPage,
		},
		Search: gitlab.Ptr(search),
	}

	tags, _, err := c.client.Tags.ListTags(projectID, opts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("list gitlab tags: %w", err)
	}

	result := make([]Tag, 0, len(tags))
	for _, t := range tags {
		tag := Tag{
			Name:    t.Name,
			Message: t.Message,
		}
		if t.Commit != nil {
			tag.Commit = Commit{
				ID:        t.Commit.ID,
				ShortID:   t.Commit.ShortID,
				Message:   t.Commit.Message,
				CreatedAt: t.Commit.CreatedAt.String(),
			}
		}
		result = append(result, tag)
	}

	return result, nil
}
