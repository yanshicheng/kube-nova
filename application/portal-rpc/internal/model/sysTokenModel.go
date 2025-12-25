package model

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ SysTokenModel = (*customSysTokenModel)(nil)

type (
	// SysTokenModel is an interface to be customized, add more methods here,
	// and implement the added methods in customSysTokenModel.
	SysTokenModel interface {
		sysTokenModel
	}

	customSysTokenModel struct {
		*defaultSysTokenModel
	}
)

// NewSysTokenModel returns a model for the database table.
func NewSysTokenModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) SysTokenModel {
	return &customSysTokenModel{
		defaultSysTokenModel: newSysTokenModel(conn, c, opts...),
	}
}
