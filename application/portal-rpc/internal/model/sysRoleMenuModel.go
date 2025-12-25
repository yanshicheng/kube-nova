package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ SysRoleMenuModel = (*customSysRoleMenuModel)(nil)

type (
	// SysRoleMenuModel is an interface to be customized, add more methods here,
	// and implement the added methods in customSysRoleMenuModel.
	SysRoleMenuModel interface {
		sysRoleMenuModel
		FindByRoleId(ctx context.Context, roleId uint64) ([]*SysRoleMenu, error)
		DeleteByRoleIdWithCache(ctx context.Context, session sqlx.Session, roleId uint64) error
		InsertWithSession(ctx context.Context, session sqlx.Session, data *SysRoleMenu) error
	}

	customSysRoleMenuModel struct {
		*defaultSysRoleMenuModel
	}
)

// NewSysRoleMenuModel returns a model for the database table.
func NewSysRoleMenuModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) SysRoleMenuModel {
	return &customSysRoleMenuModel{
		defaultSysRoleMenuModel: newSysRoleMenuModel(conn, c, opts...),
	}
}

// FindByRoleId 根据角色ID查询所有关联的菜单
func (m *customSysRoleMenuModel) FindByRoleId(ctx context.Context, roleId uint64) ([]*SysRoleMenu, error) {
	var resp []*SysRoleMenu
	query := fmt.Sprintf("SELECT %s FROM %s WHERE `role_id` = ? AND `is_deleted` = 0", sysRoleMenuRows, m.table)
	err := m.QueryRowsNoCacheCtx(ctx, &resp, query, roleId)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// DeleteByRoleIdWithCache 根据角色ID删除所有关联并清理缓存
func (m *customSysRoleMenuModel) DeleteByRoleIdWithCache(ctx context.Context, session sqlx.Session, roleId uint64) error {
	// 1. 先查询出所有需要删除的记录，用于清理缓存
	existingRecords, err := m.FindByRoleId(ctx, roleId)
	if err != nil && err != sqlx.ErrNotFound {
		return err
	}

	// 2. 收集所有需要删除的缓存key
	var cacheKeys []string
	for _, record := range existingRecords {
		idKey := fmt.Sprintf("%s%v", cacheIkubeopsSysRoleMenuIdPrefix, record.Id)
		roleIdMenuIdKey := fmt.Sprintf("%s%v:%v", cacheIkubeopsSysRoleMenuRoleIdMenuIdPrefix, record.RoleId, record.MenuId)
		cacheKeys = append(cacheKeys, idKey, roleIdMenuIdKey)
	}

	// 3. 执行删除操作
	query := fmt.Sprintf("DELETE FROM %s WHERE `role_id` = ?", m.table)
	_, err = session.ExecCtx(ctx, query, roleId)
	if err != nil {
		return err
	}

	// 4. 删除缓存
	if len(cacheKeys) > 0 {
		if err := m.DelCacheCtx(ctx, cacheKeys...); err != nil {
			return err
		}
	}

	return nil
}

// InsertWithSession 在事务中插入数据并处理缓存
func (m *customSysRoleMenuModel) InsertWithSession(ctx context.Context, session sqlx.Session, data *SysRoleMenu) error {
	// 构建缓存key（插入后需要删除可能存在的旧缓存）
	roleIdMenuIdKey := fmt.Sprintf("%s%v:%v", cacheIkubeopsSysRoleMenuRoleIdMenuIdPrefix, data.RoleId, data.MenuId)

	// 先删除可能存在的缓存
	_ = m.DelCacheCtx(ctx, roleIdMenuIdKey)

	// 执行插入
	query := fmt.Sprintf("INSERT INTO %s (role_id, menu_id, is_deleted) VALUES (?, ?, ?)", m.table)
	_, err := session.ExecCtx(ctx, query, data.RoleId, data.MenuId, 0)
	return err
}
