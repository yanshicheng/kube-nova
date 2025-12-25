package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AlertRuleGroupsModel = (*customAlertRuleGroupsModel)(nil)

type (
	// AlertRuleGroupsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAlertRuleGroupsModel.
	AlertRuleGroupsModel interface {
		alertRuleGroupsModel
		DeleteSoftByFileId(ctx context.Context, fileId uint64) error
		CountByFileId(ctx context.Context, fileId uint64) (int64, error)
	}

	customAlertRuleGroupsModel struct {
		*defaultAlertRuleGroupsModel
	}
)

// NewAlertRuleGroupsModel returns a model for the database table.
func NewAlertRuleGroupsModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) AlertRuleGroupsModel {
	return &customAlertRuleGroupsModel{
		defaultAlertRuleGroupsModel: newAlertRuleGroupsModel(conn, c, opts...),
	}
}

// DeleteSoftByFileId 按文件ID软删除所有分组
func (m *customAlertRuleGroupsModel) DeleteSoftByFileId(ctx context.Context, fileId uint64) error {
	// 注意：这里不清除缓存，因为涉及多条记录，需要调用者自行处理或忽略缓存问题
	query := fmt.Sprintf("update %s set `is_deleted` = 1 where `file_id` = ? and `is_deleted` = 0", m.table)
	_, err := m.ExecNoCacheCtx(ctx, query, fileId)
	return err
}

// Search 分页查询
func (m *customAlertRuleGroupsModel) Search(ctx context.Context, orderStr string, isAsc bool, page, pageSize uint64, queryStr string, args ...interface{}) ([]*AlertRuleGroups, uint64, error) {
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
		alertRuleGroupsRows, m.table, baseWhere, orderStr, order)

	queryArgs := append(args, offset, pageSize)
	var resp []*AlertRuleGroups
	err = m.QueryRowsNoCacheCtx(ctx, &resp, query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}

	return resp, total, nil
}

// CountByFileId 按文件ID统计分组数量
func (m *customAlertRuleGroupsModel) CountByFileId(ctx context.Context, fileId uint64) (int64, error) {
	var count int64
	query := fmt.Sprintf("select count(*) from %s where `file_id` = ? and `is_deleted` = 0", m.table)
	err := m.QueryRowNoCacheCtx(ctx, &count, query, fileId)
	if err != nil {
		return 0, err
	}
	return count, nil
}
