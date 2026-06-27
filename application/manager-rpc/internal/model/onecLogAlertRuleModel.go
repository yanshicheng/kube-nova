package model

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OnecLogAlertRuleModel = (*customOnecLogAlertRuleModel)(nil)

type (
	// OnecLogAlertRuleModel is an interface to be customized, add more methods here,
	// and implement the added methods in customOnecLogAlertRuleModel.
	OnecLogAlertRuleModel interface {
		onecLogAlertRuleModel
	}

	customOnecLogAlertRuleModel struct {
		*defaultOnecLogAlertRuleModel
	}
)

// NewOnecLogAlertRuleModel returns a model for the database table.
func NewOnecLogAlertRuleModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OnecLogAlertRuleModel {
	return &customOnecLogAlertRuleModel{
		defaultOnecLogAlertRuleModel: newOnecLogAlertRuleModel(conn, c, opts...),
	}
}
