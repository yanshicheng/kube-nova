package model

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AlertRuleFilesModel = (*customAlertRuleFilesModel)(nil)

type (
	// AlertRuleFilesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAlertRuleFilesModel.
	AlertRuleFilesModel interface {
		alertRuleFilesModel
	}

	customAlertRuleFilesModel struct {
		*defaultAlertRuleFilesModel
	}
)

// NewAlertRuleFilesModel returns a model for the database table.
func NewAlertRuleFilesModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) AlertRuleFilesModel {
	return &customAlertRuleFilesModel{
		defaultAlertRuleFilesModel: newAlertRuleFilesModel(conn, c, opts...),
	}
}
