package operator

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/repositorymanager/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type RetentionOperatorImpl struct {
	log logx.Logger
	ctx context.Context
	*BaseOperator
}

func NewRetentionOperator(ctx context.Context, base *BaseOperator) types.RetentionOperator {
	return &RetentionOperatorImpl{
		log:          logx.WithContext(ctx),
		ctx:          ctx,
		BaseOperator: base,
	}
}

// GetByProject 通过项目获取保留策略
// GetByProject 通过项目获取保留策略
func (r *RetentionOperatorImpl) GetByProject(projectName string) (*types.RetentionPolicy, error) {
	r.log.Infof("获取项目保留策略: project=%s", projectName)

	// 1. 先获取项目信息得到 project_id
	var project types.Project
	path := fmt.Sprintf("/api/v2.0/projects/%s", url.PathEscape(projectName))
	if err := r.doRequest("GET", path, nil, &project); err != nil {
		r.log.Errorf("获取项目失败: %v", err)
		return nil, fmt.Errorf("获取项目失败: %w", err)
	}

	r.log.Infof("项目 ID: %d", project.ProjectID)

	// 2. 方法1：尝试直接获取项目的 metadata，看是否包含 retention_id
	metadataPath := fmt.Sprintf("/api/v2.0/projects/%d/metadatas", project.ProjectID)
	var metadata map[string]string
	if err := r.doRequest("GET", metadataPath, nil, &metadata); err == nil {
		if retentionID, ok := metadata["retention_id"]; ok {
			r.log.Infof("从 metadata 获取到 retention_id: %s", retentionID)
			policyID, _ := strconv.ParseInt(retentionID, 10, 64)
			if policyID > 0 {
				return r.Get(policyID)
			}
		}
	}

	// 3. 方法2：列出所有保留策略，然后过滤出属于该项目的
	var allPolicies []types.RetentionPolicy
	listPath := "/api/v2.0/retentions"

	if err := r.doRequest("GET", listPath, nil, &allPolicies); err != nil {
		r.log.Errorf("获取保留策略列表失败: %v", err)
		return nil, fmt.Errorf("获取保留策略列表失败: %w", err)
	}

	r.log.Infof("获取到 %d 个保留策略", len(allPolicies))

	// 过滤出属于该项目的策略
	for _, policy := range allPolicies {
		if policy.Scope != nil && policy.Scope.Ref == project.ProjectID {
			r.log.Infof("找到项目保留策略: id=%d, rules=%d", policy.ID, len(policy.Rules))
			return &policy, nil
		}
	}

	r.log.Infof("项目 %s 没有配置保留策略", projectName)
	return nil, fmt.Errorf("项目没有配置保留策略")
}

// Get 获取保留策略
func (r *RetentionOperatorImpl) Get(policyID int64) (*types.RetentionPolicy, error) {
	r.log.Infof("获取保留策略: policyID=%d", policyID)

	var policy types.RetentionPolicy
	path := fmt.Sprintf("/api/v2.0/retentions/%d", policyID)

	if err := r.doRequest("GET", path, nil, &policy); err != nil {
		return nil, err
	}

	r.log.Infof("获取保留策略成功: id=%d, rules=%d", policy.ID, len(policy.Rules))
	return &policy, nil
}

// Create 创建保留策略
func (r *RetentionOperatorImpl) Create(projectName string, policy *types.RetentionPolicy) (int64, error) {
	r.log.Infof("创建保留策略: project=%s", projectName)

	// 1. 获取项目 ID
	var project types.Project
	projectPath := fmt.Sprintf("/api/v2.0/projects/%s", url.PathEscape(projectName))
	if err := r.doRequest("GET", projectPath, nil, &project); err != nil {
		r.log.Errorf("获取项目失败: %v", err)
		return 0, fmt.Errorf("获取项目失败: %w", err)
	}

	// 2. 设置作用域
	policy.Scope = &types.RetentionScope{
		Level: 1, // 项目级别
		Ref:   project.ProjectID,
	}

	// 3. 创建策略
	path := "/api/v2.0/retentions"

	var respBody map[string]interface{}
	if err := r.doRequest("POST", path, policy, &respBody); err != nil {
		return 0, err
	}

	// 从 Location header 或响应中提取 ID
	// 这里简化处理，重新查询获取最新创建的策略
	latestPolicy, err := r.GetByProject(projectName)
	if err != nil {
		r.log.Errorf("获取新创建的策略失败: %v", err)
		return 0, nil
	}

	r.log.Infof("创建保留策略成功: id=%d", latestPolicy.ID)
	return latestPolicy.ID, nil
}

// Update 更新保留策略
func (r *RetentionOperatorImpl) Update(policyID int64, policy *types.RetentionPolicy) error {
	r.log.Infof("更新保留策略: policyID=%d", policyID)

	path := fmt.Sprintf("/api/v2.0/retentions/%d", policyID)

	if err := r.doRequest("PUT", path, policy, nil); err != nil {
		return err
	}

	r.log.Infof("更新保留策略成功: policyID=%d", policyID)
	return nil
}

// Execute 执行保留策略
func (r *RetentionOperatorImpl) Execute(policyID int64, dryRun bool) (int64, error) {
	r.log.Infof("执行保留策略: policyID=%d, dryRun=%v", policyID, dryRun)

	body := map[string]interface{}{
		"dry_run": dryRun,
	}

	path := fmt.Sprintf("/api/v2.0/retentions/%d/executions", policyID)

	var respBody map[string]interface{}
	if err := r.doRequest("POST", path, body, &respBody); err != nil {
		return 0, err
	}

	// 获取最新的执行记录
	executions, err := r.ListExecutions(policyID, types.ListRequest{
		Page:     1,
		PageSize: 1,
	})
	if err != nil {
		r.log.Errorf("获取执行记录失败: %v", err)
		return 0, nil
	}

	if len(executions.Items) > 0 {
		executionID := executions.Items[0].ID
		r.log.Infof("执行保留策略成功: executionID=%d", executionID)
		return executionID, nil
	}

	r.log.Info("执行保留策略成功")
	return 0, nil
}

// ListExecutions 列出执行历史
func (r *RetentionOperatorImpl) ListExecutions(policyID int64, req types.ListRequest) (*types.ListRetentionExecutionsResponse, error) {
	r.log.Infof("列出保留策略执行历史: policyID=%d, page=%d, pageSize=%d",
		policyID, req.Page, req.PageSize)

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

	// 获取所有执行记录
	var allExecutions []types.RetentionExecution
	page := 1

	for {
		params := map[string]string{
			"page":      strconv.Itoa(page),
			"page_size": "100",
		}

		query := r.buildQuery(params)
		path := fmt.Sprintf("/api/v2.0/retentions/%d/executions%s", policyID, query)

		var executions []types.RetentionExecution
		if err := r.doRequest("GET", path, nil, &executions); err != nil {
			return nil, err
		}

		if len(executions) == 0 {
			break
		}

		allExecutions = append(allExecutions, executions...)

		if len(executions) < 100 {
			break
		}
		page++
	}

	r.log.Infof("获取到 %d 条执行记录", len(allExecutions))

	// 排序 - 按开始时间倒序
	sort.Slice(allExecutions, func(i, j int) bool {
		return allExecutions[i].StartTime.After(allExecutions[j].StartTime)
	})

	// 计算分页
	total := len(allExecutions)
	totalPages := (total + int(req.PageSize) - 1) / int(req.PageSize)

	start := (int(req.Page) - 1) * int(req.PageSize)
	end := start + int(req.PageSize)

	if start >= total {
		return &types.ListRetentionExecutionsResponse{
			ListResponse: types.ListResponse{
				Total:      total,
				Page:       req.Page,
				PageSize:   req.PageSize,
				TotalPages: totalPages,
			},
			Items: []types.RetentionExecution{},
		}, nil
	}

	if end > total {
		end = total
	}

	pageExecutions := allExecutions[start:end]

	r.log.Infof("返回执行历史: total=%d, page=%d/%d, 返回=%d",
		total, req.Page, totalPages, len(pageExecutions))

	return &types.ListRetentionExecutionsResponse{
		ListResponse: types.ListResponse{
			Total:      total,
			Page:       req.Page,
			PageSize:   req.PageSize,
			TotalPages: totalPages,
		},
		Items: pageExecutions,
	}, nil
}
