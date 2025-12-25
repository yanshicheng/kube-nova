package model

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OnecClusterAppModel = (*customOnecClusterAppModel)(nil)

type (
	// OnecClusterAppModel is an interface to be customized, add more methods here,
	// and implement the added methods in customOnecClusterAppModel.
	OnecClusterAppModel interface {
		onecClusterAppModel
	}

	customOnecClusterAppModel struct {
		*defaultOnecClusterAppModel
	}
)

// NewOnecClusterAppModel returns a model for the database table.
func NewOnecClusterAppModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OnecClusterAppModel {
	return &customOnecClusterAppModel{
		defaultOnecClusterAppModel: newOnecClusterAppModel(conn, c, opts...),
	}
}
