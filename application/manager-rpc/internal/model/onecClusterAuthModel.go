package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OnecClusterAuthModel = (*customOnecClusterAuthModel)(nil)

type (
	// OnecClusterAuthModel is an interface to be customized, add more methods here,
	// and implement the added methods in customOnecClusterAuthModel.
	OnecClusterAuthModel interface {
		// DeleteCache 根据ID手动删除集群认证信息的缓存
		DeleteCache(ctx context.Context, id uint64) error

		// DeleteCacheByClusterUuid 根据集群UUID手动删除集群认证信息的缓存
		DeleteCacheByClusterUuid(ctx context.Context, clusterUuid string) error
		onecClusterAuthModel
	}

	customOnecClusterAuthModel struct {
		*defaultOnecClusterAuthModel
	}
)

// NewOnecClusterAuthModel returns a model for the database table.
func NewOnecClusterAuthModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OnecClusterAuthModel {
	return &customOnecClusterAuthModel{
		defaultOnecClusterAuthModel: newOnecClusterAuthModel(conn, c, opts...),
	}
}

// DeleteCache 根据ID手动删除集群认证信息的缓存
func (m *defaultOnecClusterAuthModel) DeleteCache(ctx context.Context, id uint64) error {
	data, err := m.FindOne(ctx, id)
	if err != nil {
		return err
	}

	ikubeopsOnecClusterAuthClusterUuidKey := fmt.Sprintf("%s%v", cacheIkubeopsOnecClusterAuthClusterUuidPrefix, data.ClusterUuid)
	ikubeopsOnecClusterAuthIdKey := fmt.Sprintf("%s%v", cacheIkubeopsOnecClusterAuthIdPrefix, data.Id)

	return m.DelCacheCtx(ctx, ikubeopsOnecClusterAuthClusterUuidKey, ikubeopsOnecClusterAuthIdKey)
}

// DeleteCacheByClusterUuid 根据集群UUID手动删除集群认证信息的缓存
func (m *defaultOnecClusterAuthModel) DeleteCacheByClusterUuid(ctx context.Context, clusterUuid string) error {
	data, err := m.FindOneByClusterUuid(ctx, clusterUuid)
	if err != nil {
		return err
	}

	ikubeopsOnecClusterAuthClusterUuidKey := fmt.Sprintf("%s%v", cacheIkubeopsOnecClusterAuthClusterUuidPrefix, data.ClusterUuid)
	ikubeopsOnecClusterAuthIdKey := fmt.Sprintf("%s%v", cacheIkubeopsOnecClusterAuthIdPrefix, data.Id)

	return m.DelCacheCtx(ctx, ikubeopsOnecClusterAuthClusterUuidKey, ikubeopsOnecClusterAuthIdKey)
}
