package model

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OnecClusterNetworkModel = (*customOnecClusterNetworkModel)(nil)

type (
	// OnecClusterNetworkModel is an interface to be customized, add more methods here,
	// and implement the added methods in customOnecClusterNetworkModel.
	OnecClusterNetworkModel interface {
		onecClusterNetworkModel
	}

	customOnecClusterNetworkModel struct {
		*defaultOnecClusterNetworkModel
	}
)

// NewOnecClusterNetworkModel returns a model for the database table.
func NewOnecClusterNetworkModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OnecClusterNetworkModel {
	return &customOnecClusterNetworkModel{
		defaultOnecClusterNetworkModel: newOnecClusterNetworkModel(conn, c, opts...),
	}
}
