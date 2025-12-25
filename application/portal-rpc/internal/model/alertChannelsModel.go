package model

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AlertChannelsModel = (*customAlertChannelsModel)(nil)

type (
	// AlertChannelsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAlertChannelsModel.
	AlertChannelsModel interface {
		alertChannelsModel
	}

	customAlertChannelsModel struct {
		*defaultAlertChannelsModel
	}
)

// NewAlertChannelsModel returns a model for the database table.
func NewAlertChannelsModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) AlertChannelsModel {
	return &customAlertChannelsModel{
		defaultAlertChannelsModel: newAlertChannelsModel(conn, c, opts...),
	}
}
