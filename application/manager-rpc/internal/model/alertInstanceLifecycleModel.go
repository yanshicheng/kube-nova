package model

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AlertInstanceLifecycleModel = (*customAlertInstanceLifecycleModel)(nil)

type (
	// AlertInstanceLifecycleModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAlertInstanceLifecycleModel.
	AlertInstanceLifecycleModel interface {
		alertInstanceLifecycleModel
	}

	customAlertInstanceLifecycleModel struct {
		*defaultAlertInstanceLifecycleModel
	}
)

// NewAlertInstanceLifecycleModel returns a model for the database table.
func NewAlertInstanceLifecycleModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) AlertInstanceLifecycleModel {
	return &customAlertInstanceLifecycleModel{
		defaultAlertInstanceLifecycleModel: newAlertInstanceLifecycleModel(conn, c, opts...),
	}
}
