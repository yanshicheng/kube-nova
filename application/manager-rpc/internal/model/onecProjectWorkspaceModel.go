package model

import (
	"context"
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
