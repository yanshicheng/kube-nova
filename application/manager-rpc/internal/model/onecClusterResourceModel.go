package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OnecClusterResourceModel = (*customOnecClusterResourceModel)(nil)

type (
	// OnecClusterResourceModel is an interface to be customized, add more methods here,
	// and implement the added methods in customOnecClusterResourceModel.
	OnecClusterResourceModel interface {
		// DeleteCache 根据ID手动删除集群资源的缓存
		DeleteCache(ctx context.Context, id uint64) error

		// DeleteCacheByClusterUuid 根据集群UUID手动删除集群资源的缓存
		DeleteCacheByClusterUuid(ctx context.Context, clusterUuid string) error
		onecClusterResourceModel
	}

	customOnecClusterResourceModel struct {
		*defaultOnecClusterResourceModel
	}
)

// NewOnecClusterResourceModel returns a model for the database table.
func NewOnecClusterResourceModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OnecClusterResourceModel {
	return &customOnecClusterResourceModel{
		defaultOnecClusterResourceModel: newOnecClusterResourceModel(conn, c, opts...),
	}
}

// DeleteCache 根据ID手动删除集群资源的缓存
func (m *defaultOnecClusterResourceModel) DeleteCache(ctx context.Context, id uint64) error {
	data, err := m.FindOne(ctx, id)
	if err != nil {
		return err
	}

	ikubeopsOnecClusterResourceClusterUuidKey := fmt.Sprintf("%s%v",
		cacheIkubeopsOnecClusterResourceClusterUuidPrefix,
		data.ClusterUuid)
	ikubeopsOnecClusterResourceIdKey := fmt.Sprintf("%s%v",
		cacheIkubeopsOnecClusterResourceIdPrefix,
		data.Id)

	return m.DelCacheCtx(ctx, ikubeopsOnecClusterResourceClusterUuidKey, ikubeopsOnecClusterResourceIdKey)
}

// DeleteCacheByClusterUuid 根据集群UUID手动删除集群资源的缓存
func (m *defaultOnecClusterResourceModel) DeleteCacheByClusterUuid(ctx context.Context, clusterUuid string) error {
	data, err := m.FindOneByClusterUuid(ctx, clusterUuid)
	if err != nil {
		return err
	}

	ikubeopsOnecClusterResourceClusterUuidKey := fmt.Sprintf("%s%v",
		cacheIkubeopsOnecClusterResourceClusterUuidPrefix,
		data.ClusterUuid)
	ikubeopsOnecClusterResourceIdKey := fmt.Sprintf("%s%v",
		cacheIkubeopsOnecClusterResourceIdPrefix,
		data.Id)

	return m.DelCacheCtx(ctx, ikubeopsOnecClusterResourceClusterUuidKey, ikubeopsOnecClusterResourceIdKey)
}
