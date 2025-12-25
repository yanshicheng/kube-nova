package operator

import (
	"context"
	"fmt"
	"net/url"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/repositorymanager/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type QuotaOperatorImpl struct {
	log logx.Logger
	ctx context.Context
	*BaseOperator
}

func NewQuotaOperator(ctx context.Context, base *BaseOperator) types.QuotaOperator {
	return &QuotaOperatorImpl{
		log:          logx.WithContext(ctx),
		ctx:          ctx,
		BaseOperator: base,
	}
}

func (q *QuotaOperatorImpl) List(refType, refID string) ([]types.Quota, error) {
	q.log.Infof("列出配额: refType=%s, refID=%s", refType, refID)

	params := map[string]string{}
	if refType != "" {
		params["reference_type"] = refType
	}
	if refID != "" {
		params["reference_id"] = refID
	}

	query := q.buildQuery(params)
	var quotas []types.Quota

	if err := q.doRequest("GET", "/api/v2.0/quotas"+query, nil, &quotas); err != nil {
		return nil, err
	}

	q.log.Infof("获取到 %d 个配额", len(quotas))
	return quotas, nil
}

func (q *QuotaOperatorImpl) Get(id int64) (*types.Quota, error) {
	q.log.Infof("获取配额: id=%d", id)

	var quota types.Quota
	path := fmt.Sprintf("/api/v2.0/quotas/%d", id)

	if err := q.doRequest("GET", path, nil, &quota); err != nil {
		return nil, err
	}

	q.log.Infof("获取配额成功: id=%d", id)
	return &quota, nil
}

func (q *QuotaOperatorImpl) Update(id int64, hard types.ResourceList) error {
	q.log.Infof("更新配额: id=%d, storage=%d, count=%d", id, hard.Storage, hard.Count)

	// 构建 hard 配额对象，只包含需要设置的字段
	hardMap := make(map[string]int64)

	// Storage 配额：-1 表示无限制
	if hard.Storage != 0 {
		hardMap["storage"] = hard.Storage
		q.log.Infof("设置 storage 配额: %d", hard.Storage)
	}

	// Count 配额：某些 Harbor 版本不支持，需要条件性包含
	// 只有在明确设置且不为 0 时才包含
	if hard.Count != 0 {
		// 先尝试获取当前配额，检查是否支持 count
		currentQuota, err := q.Get(id)
		if err == nil && currentQuota.Hard.Count != 0 {
			// 如果当前配额有 count 字段，说明支持
			hardMap["count"] = hard.Count
			q.log.Infof("设置 count 配额: %d", hard.Count)
		} else {
			q.log.Errorf("Harbor 可能不支持 count 配额，跳过设置")
		}
	}

	body := map[string]interface{}{
		"hard": hardMap,
	}

	path := fmt.Sprintf("/api/v2.0/quotas/%d", id)

	if err := q.doRequest("PUT", path, body, nil); err != nil {
		return err
	}

	q.log.Infof("更新配额成功: id=%d", id)
	return nil
}

// GetByProject 通过项目名获取配额
func (q *QuotaOperatorImpl) GetByProject(projectName string) (*types.Quota, error) {
	q.log.Infof("获取项目配额: project=%s", projectName)

	// 1. 先获取项目信息得到 project_id
	var project types.Project
	path := fmt.Sprintf("/api/v2.0/projects/%s", url.PathEscape(projectName))
	if err := q.doRequest("GET", path, nil, &project); err != nil {
		q.log.Errorf("获取项目失败: %v", err)
		return nil, fmt.Errorf("获取项目失败: %w", err)
	}

	q.log.Infof("项目 ID: %d", project.ProjectID)

	// 2. 通过 project 类型和 id 获取配额
	quotas, err := q.List("project", fmt.Sprintf("%d", project.ProjectID))
	if err != nil {
		q.log.Errorf("获取配额列表失败: %v", err)
		return nil, fmt.Errorf("获取配额列表失败: %w", err)
	}

	if len(quotas) == 0 {
		q.log.Errorf("项目配额不存在: project=%s", projectName)
		return nil, fmt.Errorf("项目配额不存在")
	}

	q.log.Infof("获取项目配额成功: storage=%d/%d, count=%d/%d",
		quotas[0].Used.Storage, quotas[0].Hard.Storage,
		quotas[0].Used.Count, quotas[0].Hard.Count)

	return &quotas[0], nil
}

// UpdateByProject 通过项目名更新配额
func (q *QuotaOperatorImpl) UpdateByProject(projectName string, hard types.ResourceList) error {
	q.log.Infof("更新项目配额: project=%s, storage=%d, count=%d",
		projectName, hard.Storage, hard.Count)

	// 1. 先获取配额 ID
	quota, err := q.GetByProject(projectName)
	if err != nil {
		return err
	}

	// 2. 更新配额
	if err := q.Update(quota.ID, hard); err != nil {
		return err
	}

	q.log.Infof("更新项目配额成功: project=%s", projectName)
	return nil
}
