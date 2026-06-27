package model

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OnecLogAlertFireEventModel = (*customOnecLogAlertFireEventModel)(nil)

type (
	// OnecLogAlertFireEventModel is an interface to be customized, add more methods here,
	// and implement the added methods in customOnecLogAlertFireEventModel.
	OnecLogAlertFireEventModel interface {
		onecLogAlertFireEventModel
	}

	customOnecLogAlertFireEventModel struct {
		*defaultOnecLogAlertFireEventModel
	}
)

// NewOnecLogAlertFireEventModel returns a model for the database table.
func NewOnecLogAlertFireEventModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OnecLogAlertFireEventModel {
	return &customOnecLogAlertFireEventModel{
		defaultOnecLogAlertFireEventModel: newOnecLogAlertFireEventModel(conn, c, opts...),
	}
}
