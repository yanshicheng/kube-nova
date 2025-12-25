package model

import (
	"context"
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
