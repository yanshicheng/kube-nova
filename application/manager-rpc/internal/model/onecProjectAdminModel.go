package model

import (
	"context"
	"errors"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OnecProjectAdminModel = (*customOnecProjectAdminModel)(nil)

type (
	// OnecProjectAdminModel is an interface to be customized, add more methods here,
	// and implement the added methods in customOnecProjectAdminModel.
	OnecProjectAdminModel interface {
		// DeleteCache 根据ID手动删除项目管理员的缓存
		DeleteCache(ctx context.Context, id uint64) error

		// DeleteCacheByProjectIdUserId 根据项目ID和用户ID手动删除项目管理员的缓存
		DeleteCacheByProjectIdUserId(ctx context.Context, projectId uint64, userId uint64) error

		// BatchDeleteCacheByProjectId 根据项目ID批量删除该项目所有管理员的缓存
		BatchDeleteCacheByProjectId(ctx context.Context, projectId uint64) error
		onecProjectAdminModel
	}

	customOnecProjectAdminModel struct {
		*defaultOnecProjectAdminModel
	}
)

// NewOnecProjectAdminModel returns a model for the database table.
func NewOnecProjectAdminModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OnecProjectAdminModel {
	return &customOnecProjectAdminModel{
		defaultOnecProjectAdminModel: newOnecProjectAdminModel(conn, c, opts...),
	}
}

// DeleteCache 根据ID手动删除项目管理员的缓存
func (m *defaultOnecProjectAdminModel) DeleteCache(ctx context.Context, id uint64) error {
	data, err := m.FindOne(ctx, id)
	if err != nil {
		return err
	}

	ikubeopsOnecProjectAdminIdKey := fmt.Sprintf("%s%v",
		cacheIkubeopsOnecProjectAdminIdPrefix,
		data.Id)
	ikubeopsOnecProjectAdminProjectIdUserIdKey := fmt.Sprintf("%s%v:%v",
		cacheIkubeopsOnecProjectAdminProjectIdUserIdPrefix,
		data.ProjectId,
		data.UserId)

	return m.DelCacheCtx(ctx, ikubeopsOnecProjectAdminIdKey, ikubeopsOnecProjectAdminProjectIdUserIdKey)
}

// DeleteCacheByProjectIdUserId 根据项目ID和用户ID手动删除项目管理员的缓存
func (m *defaultOnecProjectAdminModel) DeleteCacheByProjectIdUserId(ctx context.Context, projectId uint64, userId uint64) error {
	data, err := m.FindOneByProjectIdUserId(ctx, projectId, userId)
	if err != nil {
		// 如果记录不存在，说明缓存也不存在，直接返回成功
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		return err
	}

	ikubeopsOnecProjectAdminIdKey := fmt.Sprintf("%s%v",
		cacheIkubeopsOnecProjectAdminIdPrefix,
		data.Id)
	ikubeopsOnecProjectAdminProjectIdUserIdKey := fmt.Sprintf("%s%v:%v",
		cacheIkubeopsOnecProjectAdminProjectIdUserIdPrefix,
		data.ProjectId,
		data.UserId)

	return m.DelCacheCtx(ctx, ikubeopsOnecProjectAdminIdKey, ikubeopsOnecProjectAdminProjectIdUserIdKey)
}

// BatchDeleteCacheByProjectId 根据项目ID批量删除该项目所有管理员的缓存
func (m *defaultOnecProjectAdminModel) BatchDeleteCacheByProjectId(ctx context.Context, projectId uint64) error {
	// 查询该项目的所有管理员记录
	query := fmt.Sprintf("SELECT %s FROM %s WHERE `project_id` = ? AND `is_deleted` = 0",
		onecProjectAdminRows, m.table)

	var admins []*OnecProjectAdmin
	err := m.QueryRowsNoCacheCtx(ctx, &admins, query, projectId)
	if err != nil {
		// 如果没有找到记录，说明没有缓存需要删除
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("查询项目管理员失败: %v", err)
	}

	// 批量删除所有记录的缓存
	for _, admin := range admins {
		ikubeopsOnecProjectAdminIdKey := fmt.Sprintf("%s%v",
			cacheIkubeopsOnecProjectAdminIdPrefix,
			admin.Id)
		ikubeopsOnecProjectAdminProjectIdUserIdKey := fmt.Sprintf("%s%v:%v",
			cacheIkubeopsOnecProjectAdminProjectIdUserIdPrefix,
			admin.ProjectId,
			admin.UserId)

		if err := m.DelCacheCtx(ctx, ikubeopsOnecProjectAdminIdKey, ikubeopsOnecProjectAdminProjectIdUserIdKey); err != nil {
			return fmt.Errorf("删除管理员缓存失败 [id=%d]: %v", admin.Id, err)
		}
	}

	return nil
}
