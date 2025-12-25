package model

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ SiteMessagesModel = (*customSiteMessagesModel)(nil)

type (
	// SiteMessagesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customSiteMessagesModel.
	SiteMessagesModel interface {
		siteMessagesModel
	}

	customSiteMessagesModel struct {
		*defaultSiteMessagesModel
	}
)

// NewSiteMessagesModel returns a model for the database table.
func NewSiteMessagesModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) SiteMessagesModel {
	return &customSiteMessagesModel{
		defaultSiteMessagesModel: newSiteMessagesModel(conn, c, opts...),
	}
}
