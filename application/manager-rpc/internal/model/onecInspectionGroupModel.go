package model

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OnecInspectionGroupModel = (*customOnecInspectionGroupModel)(nil)

type (
	// OnecInspectionGroupModel is an interface to be customized, add more methods here,
	// and implement the added methods in customOnecInspectionGroupModel.
	OnecInspectionGroupModel interface {
		onecInspectionGroupModel
	}

	customOnecInspectionGroupModel struct {
		*defaultOnecInspectionGroupModel
	}
)

// NewOnecInspectionGroupModel returns a model for the database table.
func NewOnecInspectionGroupModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OnecInspectionGroupModel {
	return &customOnecInspectionGroupModel{
		defaultOnecInspectionGroupModel: newOnecInspectionGroupModel(conn, c, opts...),
	}
}
