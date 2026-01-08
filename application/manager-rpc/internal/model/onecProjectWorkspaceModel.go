package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OnecProjectWorkspaceModel = (*customOnecProjectWorkspaceModel)(nil)

type (
	// OnecProjectWorkspaceModel is an interface to be customized, add more methods here,
	// and implement the added methods in customOnecProjectWorkspaceModel.
	OnecProjectWorkspaceModel interface {
		// DeleteCache 根据ID手动删除工作空间的缓存
		DeleteCache(ctx context.Context, id uint64) error

		// DeleteCacheByProjectClusterIdNamespace 根据项目集群ID和命名空间手动删除工作空间的缓存
		DeleteCacheByProjectClusterIdNamespace(ctx context.Context, projectClusterId uint64, namespace string) error

		// FindOneByClusterUuidNamespaceIncludeDeleted 根据集群UUID和命名空间查询（包含软删除，可能返回多条）
		FindAllByClusterUuidNamespaceIncludeDeleted(ctx context.Context, clusterUuid string, namespace string) ([]*OnecProjectWorkspace, error)
		// FindOneByProjectClusterIdNamespaceIncludeDeleted 根据项目集群ID和命名空间查询（包含软删除）
		FindOneByProjectClusterIdNamespaceIncludeDeleted(ctx context.Context, projectClusterId uint64, namespace string) (*OnecProjectWorkspace, error)
		// RestoreSoftDeleted 恢复软删除的工作空间
		RestoreSoftDeleted(ctx context.Context, id uint64, operator string) error
		// RestoreAndUpdateStatus 恢复软删除并更新状态
		RestoreAndUpdateStatus(ctx context.Context, id uint64, status int64, operator string) error
		onecProjectWorkspaceModel
	}

	customOnecProjectWorkspaceModel struct {
		*defaultOnecProjectWorkspaceModel
	}
)

// NewOnecProjectWorkspaceModel returns a model for the database table.
func NewOnecProjectWorkspaceModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OnecProjectWorkspaceModel {
	return &customOnecProjectWorkspaceModel{
		defaultOnecProjectWorkspaceModel: newOnecProjectWorkspaceModel(conn, c, opts...),
	}
}

// FindAllByClusterUuidNamespaceIncludeDeleted 根据集群UUID和命名空间查询所有记录（包含软删除）
func (m *customOnecProjectWorkspaceModel) FindAllByClusterUuidNamespaceIncludeDeleted(ctx context.Context, clusterUuid string, namespace string) ([]*OnecProjectWorkspace, error) {
	var resp []*OnecProjectWorkspace
	query := fmt.Sprintf("select %s from %s where `cluster_uuid` = ? and `namespace` = ?", onecProjectWorkspaceRows, m.table)
	err := m.QueryRowsNoCacheCtx(ctx, &resp, query, clusterUuid, namespace)
	switch err {
	case nil:
		return resp, nil
	case sqlx.ErrNotFound:
		return nil, nil // 返回空切片而不是错误
	default:
		return nil, err
	}
}

// FindOneByProjectClusterIdNamespaceIncludeDeleted 根据项目集群ID和命名空间查询（包含软删除）
func (m *customOnecProjectWorkspaceModel) FindOneByProjectClusterIdNamespaceIncludeDeleted(ctx context.Context, projectClusterId uint64, namespace string) (*OnecProjectWorkspace, error) {
	var resp OnecProjectWorkspace
	query := fmt.Sprintf("select %s from %s where `project_cluster_id` = ? and `namespace` = ? limit 1", onecProjectWorkspaceRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &resp, query, projectClusterId, namespace)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

// RestoreSoftDeleted 恢复软删除的工作空间
func (m *customOnecProjectWorkspaceModel) RestoreSoftDeleted(ctx context.Context, id uint64, operator string) error {
	// 先查询获取信息用于清除缓存（不带 is_deleted 条件）
	var data OnecProjectWorkspace
	query := fmt.Sprintf("select %s from %s where `id` = ? limit 1", onecProjectWorkspaceRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &data, query, id)
	if err != nil {
		return err
	}

	cacheKeyId := fmt.Sprintf("%s%v", cacheIkubeopsOnecProjectWorkspaceIdPrefix, id)
	cacheKeyProjectClusterIdNamespace := fmt.Sprintf("%s%v:%v", cacheIkubeopsOnecProjectWorkspaceProjectClusterIdNamespacePrefix, data.ProjectClusterId, data.Namespace)

	_, err = m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (result sql.Result, err error) {
		updateQuery := fmt.Sprintf("update %s set `is_deleted` = 0, `updated_by` = ?, `updated_at` = NOW() where `id` = ?", m.table)
		return conn.ExecCtx(ctx, updateQuery, operator, id)
	}, cacheKeyId, cacheKeyProjectClusterIdNamespace)

	return err
}

// RestoreAndUpdateStatus 恢复软删除并更新状态
func (m *customOnecProjectWorkspaceModel) RestoreAndUpdateStatus(ctx context.Context, id uint64, status int64, operator string) error {
	// 先查询获取信息用于清除缓存（不带 is_deleted 条件）
	var data OnecProjectWorkspace
	query := fmt.Sprintf("select %s from %s where `id` = ? limit 1", onecProjectWorkspaceRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &data, query, id)
	if err != nil {
		return err
	}

	cacheKeyId := fmt.Sprintf("%s%v", cacheIkubeopsOnecProjectWorkspaceIdPrefix, id)
	cacheKeyProjectClusterIdNamespace := fmt.Sprintf("%s%v:%v", cacheIkubeopsOnecProjectWorkspaceProjectClusterIdNamespacePrefix, data.ProjectClusterId, data.Namespace)

	_, err = m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (result sql.Result, err error) {
		updateQuery := fmt.Sprintf("update %s set `is_deleted` = 0, `status` = ?, `updated_by` = ?, `updated_at` = NOW() where `id` = ?", m.table)
		return conn.ExecCtx(ctx, updateQuery, status, operator, id)
	}, cacheKeyId, cacheKeyProjectClusterIdNamespace)

	return err
}

// DeleteCache 根据ID手动删除工作空间的缓存
func (m *defaultOnecProjectWorkspaceModel) DeleteCache(ctx context.Context, id uint64) error {
	data, err := m.FindOne(ctx, id)
	if err != nil {
		return err
	}

	ikubeopsOnecProjectWorkspaceIdKey := fmt.Sprintf("%s%v",
		cacheIkubeopsOnecProjectWorkspaceIdPrefix,
		data.Id)
	ikubeopsOnecProjectWorkspaceProjectClusterIdNamespaceKey := fmt.Sprintf("%s%v:%v",
		cacheIkubeopsOnecProjectWorkspaceProjectClusterIdNamespacePrefix,
		data.ProjectClusterId,
		data.Namespace)

	return m.DelCacheCtx(ctx, ikubeopsOnecProjectWorkspaceIdKey, ikubeopsOnecProjectWorkspaceProjectClusterIdNamespaceKey)
}

// DeleteCacheByProjectClusterIdNamespace 根据项目集群ID和命名空间手动删除工作空间的缓存
func (m *defaultOnecProjectWorkspaceModel) DeleteCacheByProjectClusterIdNamespace(ctx context.Context, projectClusterId uint64, namespace string) error {
	data, err := m.FindOneByProjectClusterIdNamespace(ctx, projectClusterId, namespace)
	if err != nil {
		return err
	}

	ikubeopsOnecProjectWorkspaceIdKey := fmt.Sprintf("%s%v",
		cacheIkubeopsOnecProjectWorkspaceIdPrefix,
		data.Id)
	ikubeopsOnecProjectWorkspaceProjectClusterIdNamespaceKey := fmt.Sprintf("%s%v:%v",
		cacheIkubeopsOnecProjectWorkspaceProjectClusterIdNamespacePrefix,
		data.ProjectClusterId,
		data.Namespace)

	return m.DelCacheCtx(ctx, ikubeopsOnecProjectWorkspaceIdKey, ikubeopsOnecProjectWorkspaceProjectClusterIdNamespaceKey)
}
