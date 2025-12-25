package operator

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/repositorymanager/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type RepositoryOperatorImpl struct {
	log logx.Logger
	ctx context.Context
	*BaseOperator
}

func NewRepositoryOperator(ctx context.Context, base *BaseOperator) types.RepositoryOperator {
	return &RepositoryOperatorImpl{
		log:          logx.WithContext(ctx),
		ctx:          ctx,
		BaseOperator: base,
	}
}

// List 列出仓库 - 优化版本：使用 Harbor API 的原生分页和排序
func (r *RepositoryOperatorImpl) List(projectName string, req types.ListRequest) (*types.ListRepositoryResponse, error) {
	r.log.Infof("列出仓库: project=%s, search=%s, page=%d, pageSize=%d",
		projectName, req.Search, req.Page, req.PageSize)

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

	// 构建查询参数
	params := map[string]string{
		"page":      strconv.FormatInt(req.Page, 10),
		"page_size": strconv.FormatInt(req.PageSize, 10),
	}

	// 搜索参数 - Harbor 支持 q 参数
	if req.Search != "" {
		params["q"] = fmt.Sprintf("name=~%s", req.Search)
	}

	// 排序参数
	sortBy := req.SortBy
	if sortBy == "" {
		sortBy = "name"
	}

	// 转换排序字段名
	harborSortField := sortBy
	switch sortBy {
	case "pullCount":
		harborSortField = "pull_count"
	case "artifactCount":
		harborSortField = "artifact_count"
	case "creationTime":
		harborSortField = "creation_time"
	case "updateTime":
		harborSortField = "update_time"
	}

	if req.SortDesc {
		params["sort"] = "-" + harborSortField
	} else {
		params["sort"] = harborSortField
	}

	query := r.buildQuery(params)
	path := fmt.Sprintf("/api/v2.0/projects/%s/repositories%s", url.PathEscape(projectName), query)

	r.log.Debugf("请求路径: %s", path)

	var repos []types.Repository
	if err := r.doRequest("GET", path, nil, &repos); err != nil {
		return nil, err
	}

	r.log.Infof("API 返回 %d 个仓库", len(repos))

	// 估算总数
	total := len(repos)
	if len(repos) == int(req.PageSize) {
		total = int(req.Page * req.PageSize)
	}

	totalPages := (total + int(req.PageSize) - 1) / int(req.PageSize)

	r.log.Infof("返回仓库列表: total=%d, page=%d/%d, 返回=%d",
		total, req.Page, totalPages, len(repos))

	return &types.ListRepositoryResponse{
		ListResponse: types.ListResponse{
			Total:      total,
			Page:       req.Page,
			PageSize:   req.PageSize,
			TotalPages: totalPages,
		},
		Items: repos,
	}, nil
}

// Get 获取仓库 - 保持不变
func (r *RepositoryOperatorImpl) Get(projectName, repoName string) (*types.Repository, error) {
	r.log.Infof("获取仓库: %s/%s", projectName, repoName)

	var repo types.Repository
	path := fmt.Sprintf("/api/v2.0/projects/%s/repositories/%s",
		url.PathEscape(projectName), url.PathEscape(repoName))

	if err := r.doRequest("GET", path, nil, &repo); err != nil {
		return nil, err
	}

	r.log.Infof("获取仓库成功: %s", repo.Name)
	return &repo, nil
}

// Delete 删除仓库 - 保持不变
func (r *RepositoryOperatorImpl) Delete(projectName, repoName string) error {
	r.log.Infof("删除仓库: %s/%s", projectName, repoName)

	path := fmt.Sprintf("/api/v2.0/projects/%s/repositories/%s",
		url.PathEscape(projectName), url.PathEscape(repoName))

	if err := r.doRequest("DELETE", path, nil, nil); err != nil {
		return err
	}

	r.log.Infof("删除仓库成功: %s/%s", projectName, repoName)
	return nil
}

// Update 更新仓库 - 保持不变
func (r *RepositoryOperatorImpl) Update(projectName, repoName, description string) error {
	r.log.Infof("更新仓库: %s/%s", projectName, repoName)

	body := map[string]string{"description": description}
	path := fmt.Sprintf("/api/v2.0/projects/%s/repositories/%s",
		url.PathEscape(projectName), url.PathEscape(repoName))

	if err := r.doRequest("PUT", path, body, nil); err != nil {
		return err
	}

	r.log.Infof("更新仓库成功: %s/%s", projectName, repoName)
	return nil
}
