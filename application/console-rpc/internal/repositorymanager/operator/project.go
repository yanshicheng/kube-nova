package operator

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/repositorymanager/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectOperatorImpl struct {
	log logx.Logger
	ctx context.Context
	*BaseOperator
}

func NewProjectOperator(ctx context.Context, base *BaseOperator) types.ProjectOperator {
	return &ProjectOperatorImpl{
		log:          logx.WithContext(ctx),
		ctx:          ctx,
		BaseOperator: base,
	}
}

// parsePublicStatus 从 metadata 中解析公开状态
func parsePublicStatus(metadata map[string]string) bool {
	if metadata == nil {
		return false
	}
	publicStr, exists := metadata["public"]
	if !exists {
		return false
	}
	return publicStr == "true" || publicStr == "1"
}

func (p *ProjectOperatorImpl) List(req types.ListRequest) (*types.ListProjectResponse, error) {
	p.log.Infof("列出项目: search=%s, page=%d, pageSize=%d, sortBy=%s, sortDesc=%v",
		req.Search, req.Page, req.PageSize, req.SortBy, req.SortDesc)

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

	// 构建查询参数 - 使用 Harbor 2.0 API 原生支持
	params := map[string]string{
		"page":      strconv.FormatInt(req.Page, 10),
		"page_size": strconv.FormatInt(req.PageSize, 10),
	}

	// 搜索参数 - 使用 Harbor 的 q 参数（支持模糊搜索）
	if req.Search != "" {
		params["q"] = fmt.Sprintf("name=~%s", req.Search)
	}

	// 排序参数 - Harbor 支持的排序字段
	sortBy := req.SortBy
	if sortBy == "" {
		sortBy = "name"
	}

	// 转换排序字段名
	harborSortField := sortBy
	switch sortBy {
	case "creationTime":
		harborSortField = "creation_time"
	case "repoCount":
		harborSortField = "repo_count"
	case "storageUsed":
		// Harbor API 不直接支持按存储排序，这种情况需要客户端排序
		harborSortField = "name"
	}

	// 构建排序参数：-字段名表示降序，字段名表示升序
	if req.SortDesc {
		params["sort"] = "-" + harborSortField
	} else {
		params["sort"] = harborSortField
	}

	query := p.buildQuery(params)
	path := "/api/v2.0/projects" + query

	p.log.Debugf("请求路径: %s", path)

	var projects []types.Project
	if err := p.doRequest("GET", path, nil, &projects); err != nil {
		return nil, err
	}

	p.log.Infof("API 返回 %d 个项目", len(projects))

	// 获取总数 - Harbor 2.0 通过 X-Total-Count header 返回，这里简化处理
	// 如果需要精确总数，需要解析响应头
	total := len(projects)

	// 如果返回的数量等于 page_size，说明可能还有更多数据
	// 这时需要再请求一次来获取准确的总数
	if len(projects) == int(req.PageSize) {
		// 请求一个大的 page_size 只获取总数
		countParams := map[string]string{
			"page":      "1",
			"page_size": "1",
		}
		if req.Search != "" {
			countParams["q"] = fmt.Sprintf("name=~%s", req.Search)
		}
		countQuery := p.buildQuery(countParams)
		countPath := "/api/v2.0/projects" + countQuery

		var tempProjects []types.Project
		if err := p.doRequest("GET", countPath, nil, &tempProjects); err == nil {
			// 从响应头获取总数的逻辑在 doRequest 中实现
			// 这里简化处理，估算总数
			total = int(req.Page * req.PageSize) // 至少有这么多
		}
	}

	// 为每个项目填充存储信息和修正公开状态
	quotaOp := NewQuotaOperator(p.ctx, p.BaseOperator)
	for i := range projects {
		proj := &projects[i]

		// 修正公开状态：从 metadata 中读取
		proj.Public = parsePublicStatus(proj.Metadata)
		p.log.Debugf("项目 %s 公开状态: %v", proj.Name, proj.Public)

		// 获取项目配额信息（失败不影响项目列表返回）
		quota, err := quotaOp.GetByProject(proj.Name)
		if err != nil {
			p.log.Debugf("获取项目 %s 的配额失败: %v", proj.Name, err)
			proj.StorageLimit = 0
			proj.StorageUsed = 0
			proj.StorageLimitDisplay = "未知"
			proj.StorageUsedDisplay = "0B"
		} else {
			proj.StorageLimit = quota.Hard.Storage
			proj.StorageUsed = quota.Used.Storage
			proj.StorageLimitDisplay = formatBytes(quota.Hard.Storage)
			proj.StorageUsedDisplay = formatBytes(quota.Used.Storage)
		}
	}

	totalPages := (total + int(req.PageSize) - 1) / int(req.PageSize)

	p.log.Infof("返回项目列表: total=%d, page=%d/%d, 返回=%d",
		total, req.Page, totalPages, len(projects))

	return &types.ListProjectResponse{
		ListResponse: types.ListResponse{
			Total:      total,
			Page:       req.Page,
			PageSize:   req.PageSize,
			TotalPages: totalPages,
		},
		Items: projects,
	}, nil
}

// Get 获取项目 - 保持不变
func (p *ProjectOperatorImpl) Get(projectNameOrID string) (*types.Project, error) {
	p.log.Infof("获取项目: %s", projectNameOrID)

	var project types.Project
	path := fmt.Sprintf("/api/v2.0/projects/%s", url.PathEscape(projectNameOrID))

	if err := p.doRequest("GET", path, nil, &project); err != nil {
		return nil, err
	}

	// 修正公开状态
	project.Public = parsePublicStatus(project.Metadata)

	// 获取项目配额信息
	quotaOp := NewQuotaOperator(p.ctx, p.BaseOperator)
	quota, err := quotaOp.GetByProject(project.Name)
	if err != nil {
		p.log.Debugf("获取项目 %s 的配额失败: %v", project.Name, err)
		project.StorageLimit = 0
		project.StorageUsed = 0
		project.StorageLimitDisplay = "未知"
		project.StorageUsedDisplay = "0B"
	} else {
		project.StorageLimit = quota.Hard.Storage
		project.StorageUsed = quota.Used.Storage
		project.StorageLimitDisplay = formatBytes(quota.Hard.Storage)
		project.StorageUsedDisplay = formatBytes(quota.Used.Storage)
	}

	p.log.Infof("获取项目成功: %s (公开: %v, 存储: %s/%s)",
		project.Name, project.Public, project.StorageUsedDisplay, project.StorageLimitDisplay)
	return &project, nil
}

// Create 创建项目 - 保持不变
func (p *ProjectOperatorImpl) Create(req *types.ProjectReq) error {
	p.log.Infof("创建项目: %s", req.ProjectName)

	createBody := make(map[string]interface{})
	createBody["project_name"] = req.ProjectName

	if req.Metadata == nil {
		req.Metadata = make(map[string]string)
	}

	if req.Public {
		req.Metadata["public"] = "true"
	} else {
		req.Metadata["public"] = "false"
	}

	createBody["metadata"] = req.Metadata

	if req.StorageLimit != 0 {
		createBody["storage_limit"] = req.StorageLimit
		p.log.Infof("设置项目存储配额: %d", req.StorageLimit)
	}

	if err := p.doRequest("POST", "/api/v2.0/projects", createBody, nil); err != nil {
		return err
	}

	p.log.Infof("创建项目成功: %s", req.ProjectName)
	return nil
}

// Update 更新项目 - 保持不变
func (p *ProjectOperatorImpl) Update(projectNameOrID string, req *types.ProjectReq) error {
	p.log.Infof("更新项目: %s", projectNameOrID)

	updateBody := make(map[string]interface{})

	if req.Metadata == nil {
		req.Metadata = make(map[string]string)
	}

	if req.Public {
		req.Metadata["public"] = "true"
	} else {
		req.Metadata["public"] = "false"
	}

	updateBody["metadata"] = req.Metadata

	path := fmt.Sprintf("/api/v2.0/projects/%s", url.PathEscape(projectNameOrID))
	if err := p.doRequest("PUT", path, updateBody, nil); err != nil {
		return err
	}

	if req.StorageLimit != 0 {
		quotaOp := NewQuotaOperator(p.ctx, p.BaseOperator)
		err := quotaOp.UpdateByProject(projectNameOrID, types.ResourceList{
			Storage: req.StorageLimit,
			Count:   -1,
		})

		if err != nil {
			p.log.Errorf("更新项目配额失败: %v", err)
		}
	}

	p.log.Infof("更新项目成功: %s", projectNameOrID)
	return nil
}

// Delete 删除项目 - 保持不变
func (p *ProjectOperatorImpl) Delete(projectNameOrID string) error {
	p.log.Infof("删除项目: %s", projectNameOrID)

	path := fmt.Sprintf("/api/v2.0/projects/%s", url.PathEscape(projectNameOrID))
	if err := p.doRequest("DELETE", path, nil, nil); err != nil {
		return err
	}

	p.log.Infof("删除项目成功: %s", projectNameOrID)
	return nil
}

// Summary 获取项目摘要 - 保持不变
func (p *ProjectOperatorImpl) Summary(projectNameOrID string) (*types.SystemInfo, error) {
	p.log.Infof("获取项目摘要: %s", projectNameOrID)

	var summary struct {
		QuotaUsage struct {
			Storage int64 `json:"storage"`
		} `json:"quota"`
		RepoCount int64 `json:"repo_count"`
	}

	path := fmt.Sprintf("/api/v2.0/projects/%s/summary", url.PathEscape(projectNameOrID))
	if err := p.doRequest("GET", path, nil, &summary); err != nil {
		return nil, err
	}

	info := &types.SystemInfo{
		StorageTotal:      summary.QuotaUsage.Storage,
		TotalRepositories: summary.RepoCount,
	}

	p.log.Infof("获取项目摘要成功: 存储=%d, 仓库数=%d", info.StorageTotal, info.TotalRepositories)
	return info, nil
}

// formatBytes 格式化字节数为可读格式
func formatBytes(bytes int64) string {
	if bytes < 0 {
		return "无限制"
	}
	if bytes == 0 {
		return "0B"
	}

	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	units := []string{"KB", "MB", "GB", "TB", "PB", "EB"}
	if exp >= len(units) {
		exp = len(units) - 1
	}

	return fmt.Sprintf("%.2f%s", float64(bytes)/float64(div), units[exp])
}

func (p *ProjectOperatorImpl) ListPublic(req types.ListRequest) (*types.ListProjectResponse, error) {
	p.log.Infof("获取所有公开项目: search=%s, page=%d, pageSize=%d", req.Search, req.Page, req.PageSize)

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
		"public":    "true", // Harbor API 过滤公开项目
	}

	// 搜索参数
	if req.Search != "" {
		params["q"] = fmt.Sprintf("name=~%s", req.Search)
	}

	// 排序参数
	sortBy := req.SortBy
	if sortBy == "" {
		sortBy = "name"
	}

	harborSortField := sortBy
	switch sortBy {
	case "creationTime":
		harborSortField = "creation_time"
	case "repoCount":
		harborSortField = "repo_count"
	case "storageUsed":
		harborSortField = "name"
	}

	if req.SortDesc {
		params["sort"] = "-" + harborSortField
	} else {
		params["sort"] = harborSortField
	}

	query := p.buildQuery(params)
	path := "/api/v2.0/projects" + query

	p.log.Debugf("请求路径: %s", path)

	var projects []types.Project
	if err := p.doRequest("GET", path, nil, &projects); err != nil {
		return nil, err
	}

	p.log.Infof("API 返回 %d 个项目", len(projects))

	// 过滤并处理公开项目
	publicProjects := make([]types.Project, 0)
	quotaOp := NewQuotaOperator(p.ctx, p.BaseOperator)

	for i := range projects {
		proj := &projects[i]

		// 修正并验证公开状态
		proj.Public = parsePublicStatus(proj.Metadata)

		// 只保留公开项目（二次过滤确保准确性）
		if !proj.Public {
			p.log.Debugf("跳过非公开项目: %s", proj.Name)
			continue
		}

		// 获取项目配额信息
		quota, err := quotaOp.GetByProject(proj.Name)
		if err != nil {
			p.log.Debugf("获取项目 %s 的配额失败: %v", proj.Name, err)
			proj.StorageLimit = 0
			proj.StorageUsed = 0
			proj.StorageLimitDisplay = "未知"
			proj.StorageUsedDisplay = "0B"
		} else {
			proj.StorageLimit = quota.Hard.Storage
			proj.StorageUsed = quota.Used.Storage
			proj.StorageLimitDisplay = formatBytes(quota.Hard.Storage)
			proj.StorageUsedDisplay = formatBytes(quota.Used.Storage)
		}

		publicProjects = append(publicProjects, *proj)
	}

	total := len(publicProjects)
	totalPages := (total + int(req.PageSize) - 1) / int(req.PageSize)

	p.log.Infof("返回公开项目列表: total=%d, page=%d/%d, 返回=%d",
		total, req.Page, totalPages, len(publicProjects))

	return &types.ListProjectResponse{
		ListResponse: types.ListResponse{
			Total:      total,
			Page:       req.Page,
			PageSize:   req.PageSize,
			TotalPages: totalPages,
		},
		Items: publicProjects,
	}, nil
}
