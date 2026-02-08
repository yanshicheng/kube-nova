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
func (r *RetentionOperatorImpl) GetByProject(projectName string) (*types.RetentionPolicy, error) {
	r.log.Infof("获取项目保留策略: project=%s", projectName)

	// 1. 获取项目信息
	var project types.Project
	path := fmt.Sprintf("/api/v2.0/projects/%s", url.PathEscape(projectName))
	if err := r.doRequest("GET", path, nil, &project); err != nil {
		r.log.Errorf("获取项目失败: %v", err)
		return nil, fmt.Errorf("获取项目失败: %w", err)
	}

	r.log.Infof("项目 ID: %d", project.ProjectID)

	// 2. 获取项目元数据，查找 retention_id
	metadataPath := fmt.Sprintf("/api/v2.0/projects/%d/metadatas", project.ProjectID)
	var metadata map[string]string
	if err := r.doRequest("GET", metadataPath, nil, &metadata); err != nil {
		// 获取元数据失败，可能是项目没有配置任何元数据
		r.log.Errorf("获取项目元数据失败，项目可能没有配置保留策略: %v", err)
		return nil, nil // 返回 nil, nil 表示没有策略
	}

	r.log.Infof("项目元数据: %+v", metadata)

	// 3. 检查是否存在 retention_id
	retentionIDStr, ok := metadata["retention_id"]
	if !ok || retentionIDStr == "" {
		r.log.Infof("项目 %s 没有配置保留策略 (metadata 中无 retention_id)", projectName)
		return nil, nil // 【关键】返回 nil, nil 而不是错误
	}

	// 4. 解析 retention_id
	policyID, err := strconv.ParseInt(retentionIDStr, 10, 64)
	if err != nil {
		r.log.Errorf("解析 retention_id 失败: %v", err)
		return nil, fmt.Errorf("解析保留策略 ID 失败: %w", err)
	}

	if policyID <= 0 {
		r.log.Infof("项目 %s 的 retention_id 无效 (policyId=%d)", projectName, policyID)
		return nil, nil
	}

	r.log.Infof("从 metadata 获取到 retention_id: %d", policyID)

	// 5. 获取策略详情
	return r.Get(policyID)
}

// Get 获取保留策略
func (r *RetentionOperatorImpl) Get(policyID int64) (*types.RetentionPolicy, error) {
	r.log.Infof("获取保留策略: policyID=%d", policyID)

	if policyID <= 0 {
		r.log.Errorf("无效的 policyID: %d", policyID)
		return nil, nil
	}

	var policy types.RetentionPolicy
	path := fmt.Sprintf("/api/v2.0/retentions/%d", policyID)

	if err := r.doRequest("GET", path, nil, &policy); err != nil {
		r.log.Errorf("获取保留策略失败: %v", err)
		return nil, err
	}

	r.log.Infof("获取保留策略成功: id=%d, rules=%d", policy.ID, len(policy.Rules))
	return &policy, nil
}

// normalizeCron 确保 cron 表达式是 6 位（Harbor 需要秒字段）
func (r *RetentionOperatorImpl) normalizeCron(cron string) string {
	cron = strings.TrimSpace(cron)
	if cron == "" {
		return ""
	}
	fields := strings.Fields(cron)
	// Harbor 需要 6 位 cron（秒 分 时 日 月 周）
	// 如果只有 5 位，前面补 "0 "
	if len(fields) == 5 {
		return "0 " + cron
	}
	return cron
}

// Create 创建保留策略
func (r *RetentionOperatorImpl) Create(projectName string, policy *types.RetentionPolicy) (int64, error) {
	r.log.Infof("创建保留策略: project=%s, algorithm=%s, rulesCount=%d",
		projectName, policy.Algorithm, len(policy.Rules))

	// 1. 获取项目 ID
	var project types.Project
	projectPath := fmt.Sprintf("/api/v2.0/projects/%s", url.PathEscape(projectName))
	if err := r.doRequest("GET", projectPath, nil, &project); err != nil {
		r.log.Errorf("获取项目失败: %v", err)
		return 0, fmt.Errorf("获取项目失败: %w", err)
	}

	// 2. 检查是否已有策略
	metadataPath := fmt.Sprintf("/api/v2.0/projects/%d/metadatas", project.ProjectID)
	var metadata map[string]string
	if err := r.doRequest("GET", metadataPath, nil, &metadata); err == nil {
		if retentionIDStr, ok := metadata["retention_id"]; ok && retentionIDStr != "" {
			policyID, _ := strconv.ParseInt(retentionIDStr, 10, 64)
			if policyID > 0 {
				r.log.Infof("项目已有保留策略 ID: %d，将执行更新操作", policyID)
				// 设置作用域
				policy.Scope = &types.RetentionScope{
					Level: "project",
					Ref:   project.ProjectID,
				}
				policy.ID = policyID
				if err := r.Update(policyID, policy); err != nil {
					return 0, err
				}
				return policyID, nil
			}
		}
	}

	// 3. 设置作用域
	policy.Scope = &types.RetentionScope{
		Level: "project", // 项目级别，必须是字符串
		Ref:   project.ProjectID,
	}

	// 4. 设置默认算法
	if policy.Algorithm == "" {
		policy.Algorithm = "or"
	}

	// 5. 处理 Trigger 中的 cron 表达式
	if policy.Trigger != nil && policy.Trigger.Settings != nil {
		if cron, ok := policy.Trigger.Settings["cron"]; ok {
			policy.Trigger.Settings["cron"] = r.normalizeCron(cron)
		}
	}

	// 6. 创建策略
	path := "/api/v2.0/retentions"
	if err := r.doRequest("POST", path, policy, nil); err != nil {
		r.log.Errorf("创建保留策略失败: %v", err)
		return 0, fmt.Errorf("创建保留策略失败: %w", err)
	}

	// 7. 重新获取项目元数据以获取新创建的策略 ID
	var newMetadata map[string]string
	if err := r.doRequest("GET", metadataPath, nil, &newMetadata); err != nil {
		r.log.Errorf("获取新创建的策略 ID 失败: %v", err)
		return 0, nil
	}

	if retentionIDStr, ok := newMetadata["retention_id"]; ok {
		policyID, _ := strconv.ParseInt(retentionIDStr, 10, 64)
		r.log.Infof("创建保留策略成功: id=%d", policyID)
		return policyID, nil
	}

	r.log.Info("创建保留策略成功，但无法获取策略 ID")
	return 0, nil
}

// Update 更新保留策略
func (r *RetentionOperatorImpl) Update(policyID int64, policy *types.RetentionPolicy) error {
	r.log.Infof("更新保留策略: policyID=%d, algorithm=%s, rulesCount=%d",
		policyID, policy.Algorithm, len(policy.Rules))

	if policyID <= 0 {
		r.log.Errorf("无效的 policyID: %d", policyID)
		return fmt.Errorf("无效的策略ID: %d", policyID)
	}

	// 处理 Trigger 中的 cron 表达式
	if policy.Trigger != nil && policy.Trigger.Settings != nil {
		if cron, ok := policy.Trigger.Settings["cron"]; ok {
			policy.Trigger.Settings["cron"] = r.normalizeCron(cron)
		}
	}

	path := fmt.Sprintf("/api/v2.0/retentions/%d", policyID)

	if err := r.doRequest("PUT", path, policy, nil); err != nil {
		r.log.Errorf("更新保留策略失败: %v", err)
		return fmt.Errorf("更新保留策略失败: %w", err)
	}

	r.log.Infof("更新保留策略成功: policyID=%d", policyID)
	return nil
}

// Execute 执行保留策略
func (r *RetentionOperatorImpl) Execute(policyID int64, dryRun bool) (int64, error) {
	r.log.Infof("执行保留策略: policyID=%d, dryRun=%v", policyID, dryRun)

	if policyID <= 0 {
		r.log.Errorf("无效的 policyID: %d", policyID)
		return 0, fmt.Errorf("无效的策略ID: %d，请先配置保留策略", policyID)
	}

	body := map[string]interface{}{
		"dry_run": dryRun,
	}

	path := fmt.Sprintf("/api/v2.0/retentions/%d/executions", policyID)

	if err := r.doRequest("POST", path, body, nil); err != nil {
		r.log.Errorf("执行保留策略失败: %v", err)
		return 0, fmt.Errorf("执行保留策略失败: %w", err)
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

	if policyID <= 0 {
		r.log.Errorf("无效的 policyID: %d，返回空列表", policyID)
		return &types.ListRetentionExecutionsResponse{
			ListResponse: types.ListResponse{
				Total:      0,
				Page:       req.Page,
				PageSize:   req.PageSize,
				TotalPages: 0,
			},
			Items: []types.RetentionExecution{},
		}, nil
	}

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
			r.log.Errorf("获取执行历史失败: %v", err)
			return nil, fmt.Errorf("获取执行历史失败: %w", err)
		}

		if len(executions) == 0 {
			break
		}

		allExecutions = append(allExecutions, executions...)

		if len(executions) < 100 {
			break
		}
		page++

		// 防止无限循环
		if page > 100 {
			break
		}
	}

	r.log.Infof("获取到 %d 条执行记录", len(allExecutions))

	// 排序 - 按开始时间倒序
	sort.Slice(allExecutions, func(i, j int) bool {
		return allExecutions[i].StartTime.After(allExecutions[j].StartTime)
	})

	// 计算分页
	total := len(allExecutions)
	totalPages := (total + int(req.PageSize) - 1) / int(req.PageSize)
	if totalPages == 0 {
		totalPages = 1
	}

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
