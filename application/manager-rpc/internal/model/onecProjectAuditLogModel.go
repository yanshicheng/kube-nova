package model

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OnecProjectAuditLogModel = (*customOnecProjectAuditLogModel)(nil)

type (
	// OnecProjectAuditLogModel is an interface to be customized, add more methods here,
	// and implement the added methods in customOnecProjectAuditLogModel.
	OnecProjectAuditLogModel interface {
		onecProjectAuditLogModel
	}

	customOnecProjectAuditLogModel struct {
		*defaultOnecProjectAuditLogModel
	}
)

// NewOnecProjectAuditLogModel returns a model for the database table.
func NewOnecProjectAuditLogModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OnecProjectAuditLogModel {
	return &customOnecProjectAuditLogModel{
		defaultOnecProjectAuditLogModel: newOnecProjectAuditLogModel(conn, c, opts...),
	}
}
