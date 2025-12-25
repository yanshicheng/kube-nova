package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AlertRulesModel = (*customAlertRulesModel)(nil)

type (
	// AlertRulesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAlertRulesModel.
	AlertRulesModel interface {
		alertRulesModel
		DeleteSoftByGroupId(ctx context.Context, groupId uint64) error
		CountByGroupId(ctx context.Context, groupId uint64) (int64, error)
		CountByFileId(ctx context.Context, fileId uint64) (int64, error)
	}

	customAlertRulesModel struct {
		*defaultAlertRulesModel
	}
)

// NewAlertRulesModel returns a model for the database table.
func NewAlertRulesModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) AlertRulesModel {
	return &customAlertRulesModel{
		defaultAlertRulesModel: newAlertRulesModel(conn, c, opts...),
	}
}

// DeleteSoftByGroupId 按分组ID软删除所有规则
func (m *customAlertRulesModel) DeleteSoftByGroupId(ctx context.Context, groupId uint64) error {
	// 注意：这里不清除缓存，因为涉及多条记录
	query := fmt.Sprintf("update %s set `is_deleted` = 1 where `group_id` = ? and `is_deleted` = 0", m.table)
	_, err := m.ExecNoCacheCtx(ctx, query, groupId)
	return err
}

// Search 分页查询
func (m *customAlertRulesModel) Search(ctx context.Context, orderStr string, isAsc bool, page, pageSize uint64, queryStr string, args ...interface{}) ([]*AlertRules, uint64, error) {
	// 构建基础查询条件
	baseWhere := "`is_deleted` = 0"
	if queryStr != "" {
		baseWhere = baseWhere + " AND " + queryStr
	}

	// 查询总数
	var total uint64
	countQuery := fmt.Sprintf("select count(*) from %s where %s", m.table, baseWhere)
	err := m.QueryRowNoCacheCtx(ctx, &total, countQuery, args...)
	if err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return nil, 0, nil
	}

	// 构建排序
	order := "ASC"
	if !isAsc {
		order = "DESC"
	}
	if orderStr == "" {
		orderStr = "id"
	}

	// 计算偏移量
	offset := (page - 1) * pageSize

	// 查询数据
	query := fmt.Sprintf("select %s from %s where %s order by `%s` %s limit ?, ?",
		alertRulesRows, m.table, baseWhere, orderStr, order)

	queryArgs := append(args, offset, pageSize)
	var resp []*AlertRules
	err = m.QueryRowsNoCacheCtx(ctx, &resp, query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}

	return resp, total, nil
}

// CountByGroupId 按分组ID统计规则数量
func (m *customAlertRulesModel) CountByGroupId(ctx context.Context, groupId uint64) (int64, error) {
	var count int64
	query := fmt.Sprintf("select count(*) from %s where `group_id` = ? and `is_deleted` = 0", m.table)
	err := m.QueryRowNoCacheCtx(ctx, &count, query, groupId)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// CountByFileId 按文件ID统计规则数量（通过 group 关联）
func (m *customAlertRulesModel) CountByFileId(ctx context.Context, fileId uint64) (int64, error) {
	var count int64
	query := fmt.Sprintf("select count(*) from %s r inner join `alert_rule_groups` g on r.group_id = g.id where g.file_id = ? and r.is_deleted = 0 and g.is_deleted = 0", m.table)
	err := m.QueryRowNoCacheCtx(ctx, &count, query, fileId)
	if err != nil {
		return 0, err
	}
	return count, nil
}
