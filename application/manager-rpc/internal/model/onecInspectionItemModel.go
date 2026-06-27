package model

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OnecInspectionItemModel = (*customOnecInspectionItemModel)(nil)

type (
	// OnecInspectionItemModel is an interface to be customized, add more methods here,
	// and implement the added methods in customOnecInspectionItemModel.
	OnecInspectionItemModel interface {
		onecInspectionItemModel
	}

	customOnecInspectionItemModel struct {
		*defaultOnecInspectionItemModel
	}
)

// NewOnecInspectionItemModel returns a model for the database table.
func NewOnecInspectionItemModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OnecInspectionItemModel {
	return &customOnecInspectionItemModel{
		defaultOnecInspectionItemModel: newOnecInspectionItemModel(conn, c, opts...),
	}
}
