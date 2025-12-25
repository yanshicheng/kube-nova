package model

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ SysApiModel = (*customSysApiModel)(nil)

type (
	// SysApiModel is an interface to be customized, add more methods here,
	// and implement the added methods in customSysApiModel.
	SysApiModel interface {
		sysApiModel
		// FindOneByPathMethod 根据路径和方法查找API
		FindOneByPathMethod(ctx context.Context, path string, method string) (*SysApi, error)
	}

	customSysApiModel struct {
		*defaultSysApiModel
	}
)

// NewSysApiModel returns a model for the database table.
func NewSysApiModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) SysApiModel {
	return &customSysApiModel{
		defaultSysApiModel: newSysApiModel(conn, c, opts...),
	}
}

// FindOneByPathMethod 根据路径和方法查找API
func (m *customSysApiModel) FindOneByPathMethod(ctx context.Context, path string, method string) (*SysApi, error) {
	var resp SysApi
	query := `
		SELECT ` + sysApiRows + ` 
		FROM ` + m.table + ` 
		WHERE path = ? AND method = ? AND is_deleted = 0 
		LIMIT 1
	`

	err := m.QueryRowNoCacheCtx(ctx, &resp, query, path, method)
	switch err {
	case nil:
		return &resp, nil
	case sqlc.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}
