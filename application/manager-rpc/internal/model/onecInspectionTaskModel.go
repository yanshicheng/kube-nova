package model

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OnecInspectionTaskModel = (*customOnecInspectionTaskModel)(nil)

type (
	// OnecInspectionTaskModel is an interface to be customized, add more methods here,
	// and implement the added methods in customOnecInspectionTaskModel.
	OnecInspectionTaskModel interface {
		onecInspectionTaskModel
	}

	customOnecInspectionTaskModel struct {
		*defaultOnecInspectionTaskModel
	}
)

// NewOnecInspectionTaskModel returns a model for the database table.
func NewOnecInspectionTaskModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OnecInspectionTaskModel {
	return &customOnecInspectionTaskModel{
		defaultOnecInspectionTaskModel: newOnecInspectionTaskModel(conn, c, opts...),
	}
}
