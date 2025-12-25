package model

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AlertNotificationsModel = (*customAlertNotificationsModel)(nil)

type (
	// AlertNotificationsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAlertNotificationsModel.
	AlertNotificationsModel interface {
		alertNotificationsModel
	}

	customAlertNotificationsModel struct {
		*defaultAlertNotificationsModel
	}
)

// NewAlertNotificationsModel returns a model for the database table.
func NewAlertNotificationsModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) AlertNotificationsModel {
	return &customAlertNotificationsModel{
		defaultAlertNotificationsModel: newAlertNotificationsModel(conn, c, opts...),
	}
}
