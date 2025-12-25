package model

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ SysUserDeptModel = (*customSysUserDeptModel)(nil)

type (
	// SysUserDeptModel is an interface to be customized, add more methods here,
	// and implement the added methods in customSysUserDeptModel.
	SysUserDeptModel interface {
		sysUserDeptModel
	}

	customSysUserDeptModel struct {
		*defaultSysUserDeptModel
	}
)

// NewSysUserDeptModel returns a model for the database table.
func NewSysUserDeptModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) SysUserDeptModel {
	return &customSysUserDeptModel{
		defaultSysUserDeptModel: newSysUserDeptModel(conn, c, opts...),
	}
}
