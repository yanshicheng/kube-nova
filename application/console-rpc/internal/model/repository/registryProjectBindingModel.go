package repository

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ RegistryProjectBindingModel = (*customRegistryProjectBindingModel)(nil)

type (
	// RegistryProjectBindingModel is an interface to be customized, add more methods here,
	// and implement the added methods in customRegistryProjectBindingModel.
	RegistryProjectBindingModel interface {
		registryProjectBindingModel
	}

	customRegistryProjectBindingModel struct {
		*defaultRegistryProjectBindingModel
	}
)

// NewRegistryProjectBindingModel returns a model for the database table.
func NewRegistryProjectBindingModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) RegistryProjectBindingModel {
	return &customRegistryProjectBindingModel{
		defaultRegistryProjectBindingModel: newRegistryProjectBindingModel(conn, c, opts...),
	}
}
