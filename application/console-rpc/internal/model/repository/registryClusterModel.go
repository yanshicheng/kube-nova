package repository

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ RegistryClusterModel = (*customRegistryClusterModel)(nil)

type (
	// RegistryClusterModel is an interface to be customized, add more methods here,
	// and implement the added methods in customRegistryClusterModel.
	RegistryClusterModel interface {
		registryClusterModel
	}

	customRegistryClusterModel struct {
		*defaultRegistryClusterModel
	}
)

// NewRegistryClusterModel returns a model for the database table.
func NewRegistryClusterModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) RegistryClusterModel {
	return &customRegistryClusterModel{
		defaultRegistryClusterModel: newRegistryClusterModel(conn, c, opts...),
	}
}
