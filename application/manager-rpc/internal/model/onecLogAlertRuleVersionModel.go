package model

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OnecLogAlertRuleVersionModel = (*customOnecLogAlertRuleVersionModel)(nil)

type (
	// OnecLogAlertRuleVersionModel is an interface to be customized, add more methods here,
	// and implement the added methods in customOnecLogAlertRuleVersionModel.
	OnecLogAlertRuleVersionModel interface {
		onecLogAlertRuleVersionModel
	}

	customOnecLogAlertRuleVersionModel struct {
		*defaultOnecLogAlertRuleVersionModel
	}
)

// NewOnecLogAlertRuleVersionModel returns a model for the database table.
func NewOnecLogAlertRuleVersionModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OnecLogAlertRuleVersionModel {
	return &customOnecLogAlertRuleVersionModel{
		defaultOnecLogAlertRuleVersionModel: newOnecLogAlertRuleVersionModel(conn, c, opts...),
	}
}
