package model

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OnecLogAlertEvalStateModel = (*customOnecLogAlertEvalStateModel)(nil)

type (
	// OnecLogAlertEvalStateModel is an interface to be customized, add more methods here,
	// and implement the added methods in customOnecLogAlertEvalStateModel.
	OnecLogAlertEvalStateModel interface {
		onecLogAlertEvalStateModel
	}

	customOnecLogAlertEvalStateModel struct {
		*defaultOnecLogAlertEvalStateModel
	}
)

// NewOnecLogAlertEvalStateModel returns a model for the database table.
func NewOnecLogAlertEvalStateModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OnecLogAlertEvalStateModel {
	return &customOnecLogAlertEvalStateModel{
		defaultOnecLogAlertEvalStateModel: newOnecLogAlertEvalStateModel(conn, c, opts...),
	}
}
