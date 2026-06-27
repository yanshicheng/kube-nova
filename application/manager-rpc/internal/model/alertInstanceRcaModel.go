package model

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AlertInstanceRcaModel = (*customAlertInstanceRcaModel)(nil)

type (
	// AlertInstanceRcaModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAlertInstanceRcaModel.
	AlertInstanceRcaModel interface {
		alertInstanceRcaModel
	}

	customAlertInstanceRcaModel struct {
		*defaultAlertInstanceRcaModel
	}
)

// NewAlertInstanceRcaModel returns a model for the database table.
func NewAlertInstanceRcaModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) AlertInstanceRcaModel {
	return &customAlertInstanceRcaModel{
		defaultAlertInstanceRcaModel: newAlertInstanceRcaModel(conn, c, opts...),
	}
}
