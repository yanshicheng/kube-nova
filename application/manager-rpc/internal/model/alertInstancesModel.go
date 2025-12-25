package model

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AlertInstancesModel = (*customAlertInstancesModel)(nil)

type (
	// AlertInstancesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAlertInstancesModel.
	AlertInstancesModel interface {
		alertInstancesModel
	}

	customAlertInstancesModel struct {
		*defaultAlertInstancesModel
	}
)

// NewAlertInstancesModel returns a model for the database table.
func NewAlertInstancesModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) AlertInstancesModel {
	return &customAlertInstancesModel{
		defaultAlertInstancesModel: newAlertInstancesModel(conn, c, opts...),
	}
}
