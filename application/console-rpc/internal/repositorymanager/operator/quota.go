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

// List 列出配额
// refType: 引用类型，如 "project"
// refID: 引用 ID，如项目 ID
func (q *QuotaOperatorImpl) List(refType, refID string) ([]types.Quota, error) {
	q.log.Infof("列出配额: refType=%s, refID=%s", refType, refID)

	params := map[string]string{}

	// Harbor 2.0 API 使用 "reference" 作为过滤参数名
	if refType != "" {
		params["reference"] = refType
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

// Get 获取单个配额详情
func (q *QuotaOperatorImpl) Get(id int64) (*types.Quota, error) {
	q.log.Infof("获取配额: id=%d", id)

	var quota types.Quota
	path := fmt.Sprintf("/api/v2.0/quotas/%d", id)

	if err := q.doRequest("GET", path, nil, &quota); err != nil {
		return nil, err
	}

	q.log.Infof("获取配额成功: id=%d, storage=%d/%d", id, quota.Used.Storage, quota.Hard.Storage)
	return &quota, nil
}

// Update 更新配额
func (q *QuotaOperatorImpl) Update(id int64, hard types.ResourceList) error {
	q.log.Infof("更新配额: id=%d, storage=%d, count=%d", id, hard.Storage, hard.Count)

	// 构建 hard 配额对象
	// 使用 interface{} 类型以支持不同的值类型
	hardMap := make(map[string]interface{})

	// Storage 配额处理：
	// -1 表示无限制，正数表示具体限制值
	// 只要不是默认的 0 值就应该更新
	if hard.Storage != 0 {
		hardMap["storage"] = hard.Storage
		if hard.Storage == -1 {
			q.log.Infof("设置 storage 配额为无限制")
		} else {
			q.log.Infof("设置 storage 配额: %d 字节 (%s)", hard.Storage, FormatBytes(hard.Storage))
		}
	}

	// Count 配额处理：
	// 某些 Harbor 版本可能不支持 count 配额
	// -1 表示无限制，正数表示具体限制值
	if hard.Count != 0 {
		hardMap["count"] = hard.Count
		if hard.Count == -1 {
			q.log.Infof("设置 count 配额为无限制")
		} else {
			q.log.Infof("设置 count 配额: %d", hard.Count)
		}
	}

	// 如果没有要更新的字段，直接返回
	if len(hardMap) == 0 {
		q.log.Infof("没有需要更新的配额字段")
		return nil
	}

	body := map[string]interface{}{
		"hard": hardMap,
	}

	path := fmt.Sprintf("/api/v2.0/quotas/%d", id)

	q.log.Debugf("更新配额请求: path=%s, body=%+v", path, body)

	if err := q.doRequest("PUT", path, body, nil); err != nil {
		q.log.Errorf("更新配额失败: %v", err)
		return fmt.Errorf("更新配额失败: %w", err)
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

	// 遍历查找匹配的配额（以防 API 返回多个）
	for _, quota := range quotas {
		if quota.Ref != nil && quota.Ref.ID == project.ProjectID {
			q.log.Infof("获取项目配额成功: quotaID=%d, storage=%d/%d, count=%d/%d",
				quota.ID, quota.Used.Storage, quota.Hard.Storage,
				quota.Used.Count, quota.Hard.Count)
			return &quota, nil
		}
	}

	// 如果没找到精确匹配的，返回第一个（保持向后兼容）
	q.log.Infof("获取项目配额成功（使用首个结果）: quotaID=%d, storage=%d/%d",
		quotas[0].ID, quotas[0].Used.Storage, quotas[0].Hard.Storage)

	return &quotas[0], nil
}

// UpdateByProject 通过项目名更新配额
func (q *QuotaOperatorImpl) UpdateByProject(projectName string, hard types.ResourceList) error {
	q.log.Infof("更新项目配额: project=%s, storage=%d, count=%d",
		projectName, hard.Storage, hard.Count)

	// 1. 先获取配额信息（包含配额 ID）
	quota, err := q.GetByProject(projectName)
	if err != nil {
		return err
	}

	q.log.Infof("找到项目配额: quotaID=%d, 当前 storage=%d/%d",
		quota.ID, quota.Used.Storage, quota.Hard.Storage)

	// 2. 更新配额
	if err := q.Update(quota.ID, hard); err != nil {
		return err
	}

	q.log.Infof("更新项目配额成功: project=%s", projectName)
	return nil
}
