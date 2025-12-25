package operator

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/repositorymanager/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ReplicationOperatorImpl struct {
	log logx.Logger
	ctx context.Context
	*BaseOperator
}

func NewReplicationOperator(ctx context.Context, base *BaseOperator) types.ReplicationOperator {
	return &ReplicationOperatorImpl{
		log:          logx.WithContext(ctx),
		ctx:          ctx,
		BaseOperator: base,
	}
}

// List 列出复制策略
func (r *ReplicationOperatorImpl) List(req types.ListRequest) (*types.ListReplicationPoliciesResponse, error) {
	r.log.Infof("列出复制策略: search=%s, page=%d, pageSize=%d",
		req.Search, req.Page, req.PageSize)

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

	// 获取所有复制策略
	var allPolicies []types.ReplicationPolicy
	page := 1

	for {
		params := map[string]string{
			"page":      strconv.Itoa(page),
			"page_size": "100",
		}

		// 如果有搜索关键词，添加到参数中
		if req.Search != "" {
			params["name"] = req.Search
		}

		query := r.buildQuery(params)
		path := "/api/v2.0/replication/policies" + query

		var policies []types.ReplicationPolicy
		if err := r.doRequest("GET", path, nil, &policies); err != nil {
			return nil, err
		}

		if len(policies) == 0 {
			break
		}

		allPolicies = append(allPolicies, policies...)

		if len(policies) < 100 {
			break
		}
		page++
	}

	r.log.Infof("获取到 %d 个复制策略", len(allPolicies))

	// 应用额外的搜索过滤
	if req.Search != "" {
		filtered := make([]types.ReplicationPolicy, 0)
		searchLower := strings.ToLower(req.Search)

		for _, policy := range allPolicies {
			if strings.Contains(strings.ToLower(policy.Name), searchLower) ||
				strings.Contains(strings.ToLower(policy.Description), searchLower) {
				filtered = append(filtered, policy)
			}
		}
		allPolicies = filtered
		r.log.Infof("搜索过滤后剩余 %d 个策略", len(allPolicies))
	}

	// 排序 - 按创建时间倒序
	sort.Slice(allPolicies, func(i, j int) bool {
		return allPolicies[i].CreationTime.After(allPolicies[j].CreationTime)
	})

	// 计算分页
	total := len(allPolicies)
	totalPages := (total + int(req.PageSize) - 1) / int(req.PageSize)

	start := (int(req.Page) - 1) * int(req.PageSize)
	end := start + int(req.PageSize)

	if start >= total {
		return &types.ListReplicationPoliciesResponse{
			ListResponse: types.ListResponse{
				Total:      total,
				Page:       req.Page,
				PageSize:   req.PageSize,
				TotalPages: totalPages,
			},
			Items: []types.ReplicationPolicy{},
		}, nil
	}

	if end > total {
		end = total
	}

	pagePolicies := allPolicies[start:end]

	r.log.Infof("返回复制策略列表: total=%d, page=%d/%d, 返回=%d",
		total, req.Page, totalPages, len(pagePolicies))

	return &types.ListReplicationPoliciesResponse{
		ListResponse: types.ListResponse{
			Total:      total,
			Page:       req.Page,
			PageSize:   req.PageSize,
			TotalPages: totalPages,
		},
		Items: pagePolicies,
	}, nil
}

// Get 获取复制策略
func (r *ReplicationOperatorImpl) Get(policyID int64) (*types.ReplicationPolicy, error) {
	r.log.Infof("获取复制策略: policyID=%d", policyID)

	var policy types.ReplicationPolicy
	path := fmt.Sprintf("/api/v2.0/replication/policies/%d", policyID)

	if err := r.doRequest("GET", path, nil, &policy); err != nil {
		return nil, err
	}

	r.log.Infof("获取复制策略成功: name=%s, enabled=%v", policy.Name, policy.Enabled)
	return &policy, nil
}

// Create 创建复制策略
func (r *ReplicationOperatorImpl) Create(policy *types.ReplicationPolicy) (int64, error) {
	r.log.Infof("创建复制策略: name=%s", policy.Name)

	// 验证必填字段
	if policy.Name == "" {
		return 0, fmt.Errorf("策略名称不能为空")
	}

	if policy.Trigger == nil {
		return 0, fmt.Errorf("触发器配置不能为空")
	}

	path := "/api/v2.0/replication/policies"

	var respBody map[string]interface{}
	if err := r.doRequest("POST", path, policy, &respBody); err != nil {
		return 0, err
	}

	// 从 Location header 或响应中提取 ID
	// 这里简化处理，重新查询获取最新创建的策略
	listResp, err := r.List(types.ListRequest{
		Page:     1,
		PageSize: 1,
		Search:   policy.Name,
	})
	if err != nil {
		r.log.Errorf("获取新创建的策略失败: %v", err)
		return 0, nil
	}

	if len(listResp.Items) > 0 {
		policyID := listResp.Items[0].ID
		r.log.Infof("创建复制策略成功: id=%d", policyID)
		return policyID, nil
	}

	r.log.Info("创建复制策略成功")
	return 0, nil
}

// Update 更新复制策略
func (r *ReplicationOperatorImpl) Update(policyID int64, policy *types.ReplicationPolicy) error {
	r.log.Infof("更新复制策略: policyID=%d", policyID)

	path := fmt.Sprintf("/api/v2.0/replication/policies/%d", policyID)

	if err := r.doRequest("PUT", path, policy, nil); err != nil {
		return err
	}

	r.log.Infof("更新复制策略成功: policyID=%d", policyID)
	return nil
}

// Delete 删除复制策略
func (r *ReplicationOperatorImpl) Delete(policyID int64) error {
	r.log.Infof("删除复制策略: policyID=%d", policyID)

	path := fmt.Sprintf("/api/v2.0/replication/policies/%d", policyID)

	if err := r.doRequest("DELETE", path, nil, nil); err != nil {
		return err
	}

	r.log.Infof("删除复制策略成功: policyID=%d", policyID)
	return nil
}

// Execute 执行复制策略
func (r *ReplicationOperatorImpl) Execute(policyID int64) (int64, error) {
	r.log.Infof("执行复制策略: policyID=%d", policyID)

	body := map[string]interface{}{
		"policy_id": policyID,
	}

	path := "/api/v2.0/replication/executions"

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
		r.log.Infof("执行复制策略成功: executionID=%d", executionID)
		return executionID, nil
	}

	r.log.Info("执行复制策略成功")
	return 0, nil
}

// ListExecutions 列出执行历史
func (r *ReplicationOperatorImpl) ListExecutions(policyID int64, req types.ListRequest) (*types.ListReplicationExecutionsResponse, error) {
	r.log.Infof("列出复制执行历史: policyID=%d, page=%d, pageSize=%d",
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
	var allExecutions []types.ReplicationExecution
	page := 1

	for {
		params := map[string]string{
			"page":      strconv.Itoa(page),
			"page_size": "100",
			"policy_id": strconv.FormatInt(policyID, 10),
		}

		query := r.buildQuery(params)
		path := "/api/v2.0/replication/executions" + query

		var executions []types.ReplicationExecution
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
		return &types.ListReplicationExecutionsResponse{
			ListResponse: types.ListResponse{
				Total:      total,
				Page:       req.Page,
				PageSize:   req.PageSize,
				TotalPages: totalPages,
			},
			Items: []types.ReplicationExecution{},
		}, nil
	}

	if end > total {
		end = total
	}

	pageExecutions := allExecutions[start:end]

	r.log.Infof("返回执行历史: total=%d, page=%d/%d, 返回=%d",
		total, req.Page, totalPages, len(pageExecutions))

	return &types.ListReplicationExecutionsResponse{
		ListResponse: types.ListResponse{
			Total:      total,
			Page:       req.Page,
			PageSize:   req.PageSize,
			TotalPages: totalPages,
		},
		Items: pageExecutions,
	}, nil
}
