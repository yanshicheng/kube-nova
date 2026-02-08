package model

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ SysUserPlatformModel = (*customSysUserPlatformModel)(nil)

type (
	// SysUserPlatformModel is an interface to be customized, add more methods here,
	// and implement the added methods in customSysUserPlatformModel.
	SysUserPlatformModel interface {
		sysUserPlatformModel
	}

	customSysUserPlatformModel struct {
		*defaultSysUserPlatformModel
	}
)

// NewSysUserPlatformModel returns a model for the database table.
func NewSysUserPlatformModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) SysUserPlatformModel {
	return &customSysUserPlatformModel{
		defaultSysUserPlatformModel: newSysUserPlatformModel(conn, c, opts...),
	}
}
