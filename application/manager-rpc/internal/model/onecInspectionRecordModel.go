package model

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OnecInspectionRecordModel = (*customOnecInspectionRecordModel)(nil)

type (
	// OnecInspectionRecordModel is an interface to be customized, add more methods here,
	// and implement the added methods in customOnecInspectionRecordModel.
	OnecInspectionRecordModel interface {
		onecInspectionRecordModel
	}

	customOnecInspectionRecordModel struct {
		*defaultOnecInspectionRecordModel
	}
)

// NewOnecInspectionRecordModel returns a model for the database table.
func NewOnecInspectionRecordModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OnecInspectionRecordModel {
	return &customOnecInspectionRecordModel{
		defaultOnecInspectionRecordModel: newOnecInspectionRecordModel(conn, c, opts...),
	}
}
