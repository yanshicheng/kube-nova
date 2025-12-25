package model

import (
	"context"
	"errors"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OnecProjectApplicationModel = (*customOnecProjectApplicationModel)(nil)

type (
	// OnecProjectApplicationModel is an interface to be customized, add more methods here,
	// and implement the added methods in customOnecProjectApplicationModel.
	OnecProjectApplicationModel interface {
		onecProjectApplicationModel
		// GetVersionsByApplicationId 通过 applicationId 获取所有版本数据
		GetVersionsByApplicationId(ctx context.Context, applicationId uint64) ([]*OnecProjectVersion, error)
	}

	customOnecProjectApplicationModel struct {
		*defaultOnecProjectApplicationModel
	}
)

// NewOnecProjectApplicationModel returns a model for the database table.
func NewOnecProjectApplicationModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OnecProjectApplicationModel {
	return &customOnecProjectApplicationModel{
		defaultOnecProjectApplicationModel: newOnecProjectApplicationModel(conn, c, opts...),
	}
}

// GetVersionsByApplicationId 通过 applicationId 获取所有版本数据
func (m *customOnecProjectApplicationModel) GetVersionsByApplicationId(ctx context.Context, applicationId uint64) ([]*OnecProjectVersion, error) {
	// 首先验证应用是否存在
	_, err := m.FindOne(ctx, applicationId)
	if err != nil {
		return nil, err
	}

	// 查询该应用的所有版本，按创建时间倒序
	query := `
		SELECT id, application_id, version, label, created_by, updated_by, created_at, updated_at, is_deleted
		FROM onec_project_version
		WHERE application_id = ? AND is_deleted = 0
		ORDER BY created_at DESC
	`

	var versions []*OnecProjectVersion
	err = m.QueryRowsNoCacheCtx(ctx, &versions, query, applicationId)
	switch {
	case err == nil:
		return versions, nil
	case errors.Is(err, sqlc.ErrNotFound):
		return nil, ErrNotFound
	default:
		return nil, err
	}
}
