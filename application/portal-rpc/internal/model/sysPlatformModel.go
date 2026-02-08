package model

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ SysPlatformModel = (*customSysPlatformModel)(nil)

type (
	// SysPlatformModel is an interface to be customized, add more methods here,
	// and implement the added methods in customSysPlatformModel.
	SysPlatformModel interface {
		sysPlatformModel
	}

	customSysPlatformModel struct {
		*defaultSysPlatformModel
	}
)

// NewSysPlatformModel returns a model for the database table.
func NewSysPlatformModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) SysPlatformModel {
	return &customSysPlatformModel{
		defaultSysPlatformModel: newSysPlatformModel(conn, c, opts...),
	}
}
