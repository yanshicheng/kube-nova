package repository

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ ContainerRegistryModel = (*customContainerRegistryModel)(nil)

type (
	// ContainerRegistryModel is an interface to be customized, add more methods here,
	// and implement the added methods in customContainerRegistryModel.
	ContainerRegistryModel interface {
		containerRegistryModel
	}

	customContainerRegistryModel struct {
		*defaultContainerRegistryModel
	}
)

// NewContainerRegistryModel returns a model for the database table.
func NewContainerRegistryModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) ContainerRegistryModel {
	return &customContainerRegistryModel{
		defaultContainerRegistryModel: newContainerRegistryModel(conn, c, opts...),
	}
}
