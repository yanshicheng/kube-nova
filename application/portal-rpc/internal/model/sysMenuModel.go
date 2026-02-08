package model

import (
	"context"
	"fmt"
	"strings"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ SysMenuModel = (*customSysMenuModel)(nil)

type (
	// SysMenuModel is an interface to be customized, add more methods here,
	// and implement the added methods in customSysMenuModel.
	SysMenuModel interface {
		sysMenuModel
		// FindByIds 批量查询菜单，避免 N+1 问题
		FindByIds(ctx context.Context, ids []uint64) ([]*SysMenu, error)
		// FindExistingIds 批量检查菜单是否存在，返回存在的ID列表
		FindExistingIds(ctx context.Context, ids []uint64) ([]uint64, error)
	}

	customSysMenuModel struct {
		*defaultSysMenuModel
	}
)

// NewSysMenuModel returns a model for the database table.
func NewSysMenuModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) SysMenuModel {
	return &customSysMenuModel{
		defaultSysMenuModel: newSysMenuModel(conn, c, opts...),
	}
}

// FindByIds 批量查询菜单
func (m *customSysMenuModel) FindByIds(ctx context.Context, ids []uint64) ([]*SysMenu, error) {
	if len(ids) == 0 {
		return []*SysMenu{}, nil
	}

	// 构建 IN 查询的占位符
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf("SELECT %s FROM %s WHERE `id` IN (%s) AND `is_deleted` = 0",
		sysMenuRows, m.table, strings.Join(placeholders, ","))

	var resp []*SysMenu
	err := m.QueryRowsNoCacheCtx(ctx, &resp, query, args...)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// FindExistingIds 批量检查菜单是否存在，返回存在的ID列表
func (m *customSysMenuModel) FindExistingIds(ctx context.Context, ids []uint64) ([]uint64, error) {
	if len(ids) == 0 {
		return []uint64{}, nil
	}

	// 构建 IN 查询的占位符
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf("SELECT `id` FROM %s WHERE `id` IN (%s) AND `is_deleted` = 0",
		m.table, strings.Join(placeholders, ","))

	var existingIds []uint64
	err := m.QueryRowsNoCacheCtx(ctx, &existingIds, query, args...)
	if err != nil {
		return nil, err
	}

	return existingIds, nil
}
