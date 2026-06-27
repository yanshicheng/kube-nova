package model

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OnecInspectionResultModel = (*customOnecInspectionResultModel)(nil)

type (
	// OnecInspectionResultModel is an interface to be customized, add more methods here,
	// and implement the added methods in customOnecInspectionResultModel.
	OnecInspectionResultModel interface {
		onecInspectionResultModel
	}

	customOnecInspectionResultModel struct {
		*defaultOnecInspectionResultModel
	}
)

// NewOnecInspectionResultModel returns a model for the database table.
func NewOnecInspectionResultModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OnecInspectionResultModel {
	return &customOnecInspectionResultModel{
		defaultOnecInspectionResultModel: newOnecInspectionResultModel(conn, c, opts...),
	}
}
