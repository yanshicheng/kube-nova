package model

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OnecInspectionTemplateModel = (*customOnecInspectionTemplateModel)(nil)

type (
	// OnecInspectionTemplateModel is an interface to be customized, add more methods here,
	// and implement the added methods in customOnecInspectionTemplateModel.
	OnecInspectionTemplateModel interface {
		onecInspectionTemplateModel
	}

	customOnecInspectionTemplateModel struct {
		*defaultOnecInspectionTemplateModel
	}
)

// NewOnecInspectionTemplateModel returns a model for the database table.
func NewOnecInspectionTemplateModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OnecInspectionTemplateModel {
	return &customOnecInspectionTemplateModel{
		defaultOnecInspectionTemplateModel: newOnecInspectionTemplateModel(conn, c, opts...),
	}
}
