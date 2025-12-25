package model

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OnecClusterNodeModel = (*customOnecClusterNodeModel)(nil)

type (
	// OnecClusterNodeModel is an interface to be customized, add more methods here,
	// and implement the added methods in customOnecClusterNodeModel.
	OnecClusterNodeModel interface {
		onecClusterNodeModel
	}

	customOnecClusterNodeModel struct {
		*defaultOnecClusterNodeModel
	}
)

// NewOnecClusterNodeModel returns a model for the database table.
func NewOnecClusterNodeModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OnecClusterNodeModel {
	return &customOnecClusterNodeModel{
		defaultOnecClusterNodeModel: newOnecClusterNodeModel(conn, c, opts...),
	}
}
