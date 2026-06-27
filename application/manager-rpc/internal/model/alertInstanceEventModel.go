package model

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AlertInstanceEventModel = (*customAlertInstanceEventModel)(nil)

type (
	// AlertInstanceEventModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAlertInstanceEventModel.
	AlertInstanceEventModel interface {
		alertInstanceEventModel
	}

	customAlertInstanceEventModel struct {
		*defaultAlertInstanceEventModel
	}
)

// NewAlertInstanceEventModel returns a model for the database table.
func NewAlertInstanceEventModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) AlertInstanceEventModel {
	return &customAlertInstanceEventModel{
		defaultAlertInstanceEventModel: newAlertInstanceEventModel(conn, c, opts...),
	}
}
