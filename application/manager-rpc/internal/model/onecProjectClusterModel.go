package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OnecProjectClusterModel = (*customOnecProjectClusterModel)(nil)

type (
	// OnecProjectClusterModel is an interface to be customized, add more methods here,
	// and implement the added methods in customOnecProjectClusterModel.
	OnecProjectClusterModel interface {
		// DeleteCache 根据ID手动删除项目集群的缓存
		DeleteCache(ctx context.Context, id uint64) error

		// DeleteCacheByClusterUuidProjectId 根据集群UUID和项目ID手动删除项目集群的缓存
		DeleteCacheByClusterUuidProjectId(ctx context.Context, clusterUuid string, projectId uint64) error
		// FindOneByClusterUuidProjectIdIncludeDeleted 查询项目集群绑定（包含软删除的记录）
		FindOneByClusterUuidProjectIdIncludeDeleted(ctx context.Context, clusterUuid string, projectId uint64) (*OnecProjectCluster, error)
		// RestoreSoftDeleted 恢复软删除的项目集群绑定
		RestoreSoftDeleted(ctx context.Context, id uint64, operator string) error
		onecProjectClusterModel
	}

	customOnecProjectClusterModel struct {
		*defaultOnecProjectClusterModel
	}
)

// NewOnecProjectClusterModel returns a model for the database table.
func NewOnecProjectClusterModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OnecProjectClusterModel {
	return &customOnecProjectClusterModel{
		defaultOnecProjectClusterModel: newOnecProjectClusterModel(conn, c, opts...),
	}
}

// FindOneByClusterUuidProjectIdIncludeDeleted 查询项目集群绑定（包含软删除的记录）
func (m *customOnecProjectClusterModel) FindOneByClusterUuidProjectIdIncludeDeleted(ctx context.Context, clusterUuid string, projectId uint64) (*OnecProjectCluster, error) {
	var resp OnecProjectCluster
	query := fmt.Sprintf("select %s from %s where `cluster_uuid` = ? and `project_id` = ? limit 1", onecProjectClusterRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &resp, query, clusterUuid, projectId)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

// RestoreSoftDeleted 恢复软删除的项目集群绑定
func (m *customOnecProjectClusterModel) RestoreSoftDeleted(ctx context.Context, id uint64, operator string) error {
	// 先查询获取信息用于清除缓存（不带 is_deleted 条件）
	var data OnecProjectCluster
	query := fmt.Sprintf("select %s from %s where `id` = ? limit 1", onecProjectClusterRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &data, query, id)
	if err != nil {
		return err
	}

	cacheKeyClusterUuidProjectId := fmt.Sprintf("%s%v:%v", cacheIkubeopsOnecProjectClusterClusterUuidProjectIdPrefix, data.ClusterUuid, data.ProjectId)
	cacheKeyId := fmt.Sprintf("%s%v", cacheIkubeopsOnecProjectClusterIdPrefix, id)

	_, err = m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (result sql.Result, err error) {
		updateQuery := fmt.Sprintf("update %s set `is_deleted` = 0, `updated_by` = ?, `updated_at` = NOW() where `id` = ?", m.table)
		return conn.ExecCtx(ctx, updateQuery, operator, id)
	}, cacheKeyClusterUuidProjectId, cacheKeyId)

	return err
}

// DeleteCache 根据ID手动删除项目集群的缓存
func (m *defaultOnecProjectClusterModel) DeleteCache(ctx context.Context, id uint64) error {
	data, err := m.FindOne(ctx, id)
	if err != nil {
		return err
	}

	ikubeopsOnecProjectClusterClusterUuidProjectIdKey := fmt.Sprintf("%s%v:%v",
		cacheIkubeopsOnecProjectClusterClusterUuidProjectIdPrefix,
		data.ClusterUuid,
		data.ProjectId)
	ikubeopsOnecProjectClusterIdKey := fmt.Sprintf("%s%v",
		cacheIkubeopsOnecProjectClusterIdPrefix,
		data.Id)

	return m.DelCacheCtx(ctx, ikubeopsOnecProjectClusterClusterUuidProjectIdKey, ikubeopsOnecProjectClusterIdKey)
}

// DeleteCacheByClusterUuidProjectId 根据集群UUID和项目ID手动删除项目集群的缓存
func (m *defaultOnecProjectClusterModel) DeleteCacheByClusterUuidProjectId(ctx context.Context, clusterUuid string, projectId uint64) error {
	data, err := m.FindOneByClusterUuidProjectId(ctx, clusterUuid, projectId)
	if err != nil {
		return err
	}

	ikubeopsOnecProjectClusterClusterUuidProjectIdKey := fmt.Sprintf("%s%v:%v",
		cacheIkubeopsOnecProjectClusterClusterUuidProjectIdPrefix,
		data.ClusterUuid,
		data.ProjectId)
	ikubeopsOnecProjectClusterIdKey := fmt.Sprintf("%s%v",
		cacheIkubeopsOnecProjectClusterIdPrefix,
		data.Id)

	return m.DelCacheCtx(ctx, ikubeopsOnecProjectClusterClusterUuidProjectIdKey, ikubeopsOnecProjectClusterIdKey)
}
