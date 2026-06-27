package model

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OnecLogAlertEvalTaskModel = (*customOnecLogAlertEvalTaskModel)(nil)

type (
	// OnecLogAlertEvalTaskModel is an interface to be customized, add more methods here,
	// and implement the added methods in customOnecLogAlertEvalTaskModel.
	OnecLogAlertEvalTaskModel interface {
		onecLogAlertEvalTaskModel
	}

	customOnecLogAlertEvalTaskModel struct {
		*defaultOnecLogAlertEvalTaskModel
	}
)

// NewOnecLogAlertEvalTaskModel returns a model for the database table.
func NewOnecLogAlertEvalTaskModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OnecLogAlertEvalTaskModel {
	return &customOnecLogAlertEvalTaskModel{
		defaultOnecLogAlertEvalTaskModel: newOnecLogAlertEvalTaskModel(conn, c, opts...),
	}
}
