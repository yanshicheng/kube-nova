package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ SysUserRoleModel = (*customSysUserRoleModel)(nil)

type (
	// SysUserRoleModel is an interface to be customized, add more methods here,
	// and implement the added methods in customSysUserRoleModel.
	SysUserRoleModel interface {
		sysUserRoleModel
		FindByUserId(ctx context.Context, userId uint64) ([]*SysUserRole, error)
		DeleteByUserIdWithCache(ctx context.Context, session sqlx.Session, userId uint64) error
		InsertWithSession(ctx context.Context, session sqlx.Session, data *SysUserRole) error
	}

	customSysUserRoleModel struct {
		*defaultSysUserRoleModel
	}
)

// NewSysUserRoleModel returns a model for the database table.
func NewSysUserRoleModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) SysUserRoleModel {
	return &customSysUserRoleModel{
		defaultSysUserRoleModel: newSysUserRoleModel(conn, c, opts...),
	}
}

// FindByUserId 根据用户ID查询所有关联的角色
func (m *customSysUserRoleModel) FindByUserId(ctx context.Context, userId uint64) ([]*SysUserRole, error) {
	var resp []*SysUserRole
	query := fmt.Sprintf("SELECT %s FROM %s WHERE `user_id` = ? AND `is_deleted` = 0", sysUserRoleRows, m.table)
	err := m.QueryRowsNoCacheCtx(ctx, &resp, query, userId)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// DeleteByUserIdWithCache 根据用户ID删除所有关联并清理缓存
func (m *customSysUserRoleModel) DeleteByUserIdWithCache(ctx context.Context, session sqlx.Session, userId uint64) error {
	// 1. 先查询出所有需要删除的记录，用于清理缓存
	existingRecords, err := m.FindByUserId(ctx, userId)
	if err != nil && err != sqlx.ErrNotFound {
		return err
	}

	// 2. 收集所有需要删除的缓存key
	var cacheKeys []string
	for _, record := range existingRecords {
		idKey := fmt.Sprintf("%s%v", cacheIkubeopsSysUserRoleIdPrefix, record.Id)
		userIdRoleIdKey := fmt.Sprintf("%s%v:%v", cacheIkubeopsSysUserRoleUserIdRoleIdPrefix, record.UserId, record.RoleId)
		cacheKeys = append(cacheKeys, idKey, userIdRoleIdKey)
	}

	// 3. 执行删除操作
	query := fmt.Sprintf("DELETE FROM %s WHERE `user_id` = ?", m.table)
	_, err = session.ExecCtx(ctx, query, userId)
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
func (m *customSysUserRoleModel) InsertWithSession(ctx context.Context, session sqlx.Session, data *SysUserRole) error {
	// 构建缓存key（插入后需要删除可能存在的旧缓存）
	userIdRoleIdKey := fmt.Sprintf("%s%v:%v", cacheIkubeopsSysUserRoleUserIdRoleIdPrefix, data.UserId, data.RoleId)

	// 先删除可能存在的缓存
	_ = m.DelCacheCtx(ctx, userIdRoleIdKey)

	// 执行插入
	query := fmt.Sprintf("INSERT INTO %s (user_id, role_id, is_deleted) VALUES (?, ?, ?)", m.table)
	_, err := session.ExecCtx(ctx, query, data.UserId, data.RoleId, 0)
	return err
}
