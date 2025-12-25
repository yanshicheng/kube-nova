package operator

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/repositorymanager/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ArtifactOperatorImpl struct {
	log logx.Logger
	ctx context.Context
	*BaseOperator
}

func NewArtifactOperator(ctx context.Context, base *BaseOperator) types.ArtifactOperator {
	return &ArtifactOperatorImpl{
		log:          logx.WithContext(ctx),
		ctx:          ctx,
		BaseOperator: base,
	}
}

// cleanRepoName 清理仓库名称，移除项目名前缀
func (a *ArtifactOperatorImpl) cleanRepoName(projectName, repoName string) string {
	// 如果 repoName 以 "projectName/" 开头，去除这个前缀
	prefix := projectName + "/"
	if strings.HasPrefix(repoName, prefix) {
		cleaned := strings.TrimPrefix(repoName, prefix)
		a.log.Infof("清理仓库名: %s -> %s", repoName, cleaned)
		return cleaned
	}
	return repoName
}

func (a *ArtifactOperatorImpl) List(projectName, repoName string, req types.ListRequest) (*types.ListArtifactResponse, error) {
	// 清理仓库名，移除可能包含的项目名前缀
	cleanedRepoName := a.cleanRepoName(projectName, repoName)

	a.log.Infof("列出制品: project=%s, repo=%s (cleaned=%s), search=%s",
		projectName, repoName, cleanedRepoName, req.Search)

	// 设置默认值
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	// 获取所有制品
	var allArtifacts []types.Artifact
	page := 1

	for {
		params := map[string]string{
			"page":      strconv.Itoa(page),
			"page_size": "100",
			"with_tag":  "true",
		}

		query := a.buildQuery(params)
		// 使用清理后的仓库名
		path := fmt.Sprintf("/api/v2.0/projects/%s/repositories/%s/artifacts%s",
			url.PathEscape(projectName), url.PathEscape(cleanedRepoName), query)

		a.log.Debugf("请求路径: %s", path)

		var artifacts []types.Artifact
		if err := a.doRequest("GET", path, nil, &artifacts); err != nil {
			return nil, err
		}

		if len(artifacts) == 0 {
			break
		}

		allArtifacts = append(allArtifacts, artifacts...)

		if len(artifacts) < 100 {
			break
		}
		page++
	}

	a.log.Infof("获取到 %d 个制品", len(allArtifacts))

	// 应用搜索过滤
	if req.Search != "" {
		filtered := make([]types.Artifact, 0)
		searchLower := strings.ToLower(req.Search)

		for _, artifact := range allArtifacts {
			// 搜索 Digest
			if strings.Contains(strings.ToLower(artifact.Digest), searchLower) {
				filtered = append(filtered, artifact)
				continue
			}

			// 搜索 Tags
			for _, tag := range artifact.Tags {
				if strings.Contains(strings.ToLower(tag.Name), searchLower) {
					filtered = append(filtered, artifact)
					break
				}
			}
		}
		allArtifacts = filtered
		a.log.Infof("搜索过滤后剩余 %d 个制品", len(allArtifacts))
	}

	// 排序
	sortBy := req.SortBy
	if sortBy == "" {
		sortBy = "pushTime"
	}

	sort.Slice(allArtifacts, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "size":
			less = allArtifacts[i].Size < allArtifacts[j].Size
		case "pullTime":
			less = allArtifacts[i].PullTime.Before(allArtifacts[j].PullTime)
		default: // pushTime
			less = allArtifacts[i].PushTime.Before(allArtifacts[j].PushTime)
		}

		if req.SortDesc {
			return !less
		}
		return less
	})

	// 计算分页
	total := len(allArtifacts)
	totalPages := (total + int(req.PageSize) - 1) / int(req.PageSize)

	start := (int(req.Page) - 1) * int(req.PageSize)
	end := start + int(req.PageSize)

	if start >= total {
		return &types.ListArtifactResponse{
			ListResponse: types.ListResponse{
				Total:      total,
				Page:       req.Page,
				PageSize:   req.PageSize,
				TotalPages: totalPages,
			},
			Items: []types.Artifact{},
		}, nil
	}

	if end > total {
		end = total
	}

	pageArtifacts := allArtifacts[start:end]

	a.log.Infof("返回制品列表: total=%d, page=%d/%d, 返回=%d",
		total, req.Page, totalPages, len(pageArtifacts))

	return &types.ListArtifactResponse{
		ListResponse: types.ListResponse{
			Total:      total,
			Page:       req.Page,
			PageSize:   req.PageSize,
			TotalPages: totalPages,
		},
		Items: pageArtifacts,
	}, nil
}

func (a *ArtifactOperatorImpl) Get(projectName, repoName, reference string) (*types.Artifact, error) {
	// 清理仓库名
	cleanedRepoName := a.cleanRepoName(projectName, repoName)

	a.log.Infof("获取制品: %s/%s:%s", projectName, cleanedRepoName, reference)

	var artifact types.Artifact
	path := fmt.Sprintf("/api/v2.0/projects/%s/repositories/%s/artifacts/%s",
		url.PathEscape(projectName), url.PathEscape(cleanedRepoName), url.PathEscape(reference))

	if err := a.doRequest("GET", path, nil, &artifact); err != nil {
		return nil, err
	}

	a.log.Infof("获取制品成功: digest=%s", artifact.Digest)
	return &artifact, nil
}

func (a *ArtifactOperatorImpl) Delete(projectName, repoName, reference string) error {
	// 清理仓库名
	cleanedRepoName := a.cleanRepoName(projectName, repoName)

	a.log.Infof("删除制品: %s/%s:%s", projectName, cleanedRepoName, reference)

	path := fmt.Sprintf("/api/v2.0/projects/%s/repositories/%s/artifacts/%s",
		url.PathEscape(projectName), url.PathEscape(cleanedRepoName), url.PathEscape(reference))

	if err := a.doRequest("DELETE", path, nil, nil); err != nil {
		return err
	}

	a.log.Infof("删除制品成功: %s/%s:%s", projectName, cleanedRepoName, reference)
	return nil
}

func (a *ArtifactOperatorImpl) ListTags(projectName, repoName, reference string) ([]types.Tag, error) {
	// 清理仓库名
	cleanedRepoName := a.cleanRepoName(projectName, repoName)

	a.log.Infof("列出标签: %s/%s:%s", projectName, cleanedRepoName, reference)

	path := fmt.Sprintf("/api/v2.0/projects/%s/repositories/%s/artifacts/%s/tags",
		url.PathEscape(projectName), url.PathEscape(cleanedRepoName), url.PathEscape(reference))

	var tags []types.Tag
	if err := a.doRequest("GET", path, nil, &tags); err != nil {
		return nil, err
	}

	a.log.Infof("获取到 %d 个标签", len(tags))
	return tags, nil
}

func (a *ArtifactOperatorImpl) CreateTag(projectName, repoName, reference, tagName string) error {
	// 清理仓库名
	cleanedRepoName := a.cleanRepoName(projectName, repoName)

	a.log.Infof("创建标签: %s/%s:%s -> %s", projectName, cleanedRepoName, reference, tagName)

	body := map[string]string{"name": tagName}
	path := fmt.Sprintf("/api/v2.0/projects/%s/repositories/%s/artifacts/%s/tags",
		url.PathEscape(projectName), url.PathEscape(cleanedRepoName), url.PathEscape(reference))

	if err := a.doRequest("POST", path, body, nil); err != nil {
		return err
	}

	a.log.Infof("创建标签成功: %s", tagName)
	return nil
}

func (a *ArtifactOperatorImpl) DeleteTag(projectName, repoName, tagName string) error {
	// 清理仓库名
	cleanedRepoName := a.cleanRepoName(projectName, repoName)

	a.log.Infof("删除标签: %s/%s:%s", projectName, cleanedRepoName, tagName)

	// 需要先获取对应的 artifact reference
	path := fmt.Sprintf("/api/v2.0/projects/%s/repositories/%s/artifacts/latest/tags/%s",
		url.PathEscape(projectName), url.PathEscape(cleanedRepoName), url.PathEscape(tagName))

	if err := a.doRequest("DELETE", path, nil, nil); err != nil {
		return err
	}

	a.log.Infof("删除标签成功: %s", tagName)
	return nil
}
