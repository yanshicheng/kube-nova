package model

import (
	"context"
	"fmt"
	"strings"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OnecInspectionResultModel = (*customOnecInspectionResultModel)(nil)

type (
	// OnecInspectionResultModel is an interface to be customized, add more methods here,
	// and implement the added methods in customOnecInspectionResultModel.
	OnecInspectionResultModel interface {
		onecInspectionResultModel
		InsertBatch(ctx context.Context, data []*OnecInspectionResult) error
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

func (m *customOnecInspectionResultModel) InsertBatch(ctx context.Context, data []*OnecInspectionResult) error {
	if len(data) == 0 {
		return nil
	}
	const batchSize = 500
	for start := 0; start < len(data); start += batchSize {
		end := start + batchSize
		if end > len(data) {
			end = len(data)
		}
		if err := m.insertBatch(ctx, data[start:end]); err != nil {
			return err
		}
	}
	return nil
}

func (m *customOnecInspectionResultModel) insertBatch(ctx context.Context, data []*OnecInspectionResult) error {
	rows := make([]string, 0, len(data))
	args := make([]any, 0, len(data)*20)
	for _, item := range data {
		if item == nil {
			continue
		}
		rows = append(rows, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
		args = append(args,
			item.RecordId,
			item.ItemId,
			item.ItemCode,
			item.ItemName,
			item.Category,
			item.TargetType,
			item.TargetName,
			item.Severity,
			item.Status,
			item.Score,
			item.Expected,
			item.Actual,
			item.Value,
			item.Unit,
			item.Message,
			item.Suggestion,
			item.DetailJson,
			item.CreatedBy,
			item.UpdatedBy,
			item.IsDeleted,
		)
	}
	if len(rows) == 0 {
		return nil
	}
	query := fmt.Sprintf("insert into %s (%s) values %s", m.table, onecInspectionResultRowsExpectAutoSet, strings.Join(rows, ","))
	_, err := m.ExecNoCacheCtx(ctx, query, args...)
	return err
}
