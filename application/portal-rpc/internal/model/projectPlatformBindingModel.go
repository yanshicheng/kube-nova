package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ ProjectPlatformBindingModel = (*customProjectPlatformBindingModel)(nil)

type (
	// ProjectPlatformBindingModel is an interface to be customized
	ProjectPlatformBindingModel interface {
		projectPlatformBindingModel
		BindProjectPlatform(ctx context.Context, projectId, platformId uint64, operator string) error
		UnbindProjectPlatform(ctx context.Context, projectId, platformId uint64, operator string) error
		IsProjectBoundToPlatform(ctx context.Context, projectId, platformId uint64) (bool, error)
		ListPlatformIdsByUser(ctx context.Context, userId uint64) ([]uint64, error)
	}

	customProjectPlatformBindingModel struct {
		*defaultProjectPlatformBindingModel
	}
)

func NewProjectPlatformBindingModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) ProjectPlatformBindingModel {
	return &customProjectPlatformBindingModel{
		defaultProjectPlatformBindingModel: newProjectPlatformBindingModel(conn, c, opts...),
	}
}

func (m *customProjectPlatformBindingModel) BindProjectPlatform(ctx context.Context, projectId, platformId uint64, operator string) error {
	if operator == "" {
		operator = "system"
	}
	query := fmt.Sprintf("insert into %s (`project_id`,`platform_id`,`created_by`,`updated_by`,`is_deleted`) values (?, ?, ?, ?, 0) on duplicate key update `updated_by` = values(`updated_by`), `updated_at` = now(), `is_deleted` = 0", m.table)
	_, err := m.ExecSql(ctx, 0, query, projectId, platformId, operator, operator)
	return err
}

func (m *customProjectPlatformBindingModel) UnbindProjectPlatform(ctx context.Context, projectId, platformId uint64, operator string) error {
	if operator == "" {
		operator = "system"
	}
	query := fmt.Sprintf("update %s set `is_deleted` = 1, `updated_by` = ?, `updated_at` = now() where `project_id` = ? and `platform_id` = ? and `is_deleted` = 0", m.table)
	_, err := m.ExecSql(ctx, 0, query, operator, projectId, platformId)
	return err
}

func (m *customProjectPlatformBindingModel) IsProjectBoundToPlatform(ctx context.Context, projectId, platformId uint64) (bool, error) {
	query := fmt.Sprintf("select count(1) from %s where `project_id` = ? and `platform_id` = ? and `is_deleted` = 0", m.table)
	var count int64
	if err := m.QueryRowNoCacheCtx(ctx, &count, query, projectId, platformId); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (m *customProjectPlatformBindingModel) ListPlatformIdsByUser(ctx context.Context, userId uint64) ([]uint64, error) {
	query := fmt.Sprintf("select distinct ppb.`platform_id` from %s ppb inner join `project_member_binding` pmb on pmb.`project_id` = ppb.`project_id` and pmb.`user_id` = ? and pmb.`is_deleted` = 0 where ppb.`is_deleted` = 0 order by ppb.`platform_id` asc", m.table)
	var ids []uint64
	if err := m.QueryRowsNoCacheCtx(ctx, &ids, query, userId); err != nil {
		if err == sqlx.ErrNotFound {
			return []uint64{}, nil
		}
		return nil, err
	}
	return ids, nil
}
