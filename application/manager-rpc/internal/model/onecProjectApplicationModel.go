package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OnecProjectApplicationModel = (*customOnecProjectApplicationModel)(nil)

type (
	OnecProjectApplicationModel interface {
		onecProjectApplicationModel
		// GetVersionsByApplicationId 通过 applicationId 获取所有版本数据
		GetVersionsByApplicationId(ctx context.Context, applicationId uint64) ([]*OnecProjectVersion, error)
		// FindOneByWorkspaceIdNameEnResourceTypeIncludeDeleted 查询应用（包含软删除的记录）
		FindOneByWorkspaceIdNameEnResourceTypeIncludeDeleted(ctx context.Context, workspaceId uint64, nameEn string, resourceType string) (*OnecProjectApplication, error)
		// FindOneByWorkspaceIdNameCnResourceTypeIncludeDeleted 通过中文名查询应用（包含软删除的记录）
		FindOneByWorkspaceIdNameCnResourceTypeIncludeDeleted(ctx context.Context, workspaceId uint64, nameCn string, resourceType string) (*OnecProjectApplication, error)
		// FindByWorkspaceIdNamesResourceTypeIncludeDeleted 通过中英文名查询应用（包含软删除的记录）
		FindByWorkspaceIdNamesResourceTypeIncludeDeleted(ctx context.Context, workspaceId uint64, nameCn string, nameEn string, resourceType string) (*OnecProjectApplication, error)
		// RestoreSoftDeleted 恢复软删除的应用
		RestoreSoftDeleted(ctx context.Context, id uint64, operator string) error
		// FindAllByWorkspaceId 查询工作空间下的所有应用（不包含软删除）
		FindAllByWorkspaceId(ctx context.Context, workspaceId uint64) ([]*OnecProjectApplication, error)
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
	_, err := m.FindOne(ctx, applicationId)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, application_id, version, version_role, resource_name, parent_app_name, created_by, updated_by, created_at, updated_at, is_deleted, status
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

// FindOneByWorkspaceIdNameEnResourceTypeIncludeDeleted 查询应用（包含软删除的记录）
func (m *customOnecProjectApplicationModel) FindOneByWorkspaceIdNameEnResourceTypeIncludeDeleted(ctx context.Context, workspaceId uint64, nameEn string, resourceType string) (*OnecProjectApplication, error) {
	var resp OnecProjectApplication
	query := fmt.Sprintf("select %s from %s where `workspace_id` = ? and `name_en` = ? and `resource_type` = ? limit 1", onecProjectApplicationRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &resp, query, workspaceId, nameEn, resourceType)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

// RestoreSoftDeleted 恢复软删除的应用
func (m *customOnecProjectApplicationModel) RestoreSoftDeleted(ctx context.Context, id uint64, operator string) error {
	// 先查询获取信息用于清除缓存（不带 is_deleted 条件）
	var data OnecProjectApplication
	query := fmt.Sprintf("select %s from %s where `id` = ? limit 1", onecProjectApplicationRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &data, query, id)
	if err != nil {
		return err
	}

	cacheKeyId := fmt.Sprintf("%s%v", cacheIkubeopsOnecProjectApplicationIdPrefix, id)
	cacheKeyUnique := fmt.Sprintf("%s%v:%v:%v", cacheIkubeopsOnecProjectApplicationWorkspaceIdNameEnResourceTypePrefix, data.WorkspaceId, data.NameEn, data.ResourceType)

	_, err = m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (result sql.Result, err error) {
		updateQuery := fmt.Sprintf("update %s set `is_deleted` = 0, `updated_by` = ?, `updated_at` = NOW() where `id` = ?", m.table)
		return conn.ExecCtx(ctx, updateQuery, operator, id)
	}, cacheKeyId, cacheKeyUnique)

	return err
}

// FindOneByWorkspaceIdNameCnResourceTypeIncludeDeleted 通过中文名查询应用（包含软删除的记录）
func (m *customOnecProjectApplicationModel) FindOneByWorkspaceIdNameCnResourceTypeIncludeDeleted(ctx context.Context, workspaceId uint64, nameCn string, resourceType string) (*OnecProjectApplication, error) {
	var resp OnecProjectApplication
	query := fmt.Sprintf("select %s from %s where `workspace_id` = ? and `name_cn` = ? and `resource_type` = ? limit 1", onecProjectApplicationRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &resp, query, workspaceId, nameCn, resourceType)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

// FindByWorkspaceIdNamesResourceTypeIncludeDeleted 通过中英文名查询应用（包含软删除的记录）
// 优先匹配中文名和英文名都相符的应用，如果没有则分别尝试中文名或英文名匹配
func (m *customOnecProjectApplicationModel) FindByWorkspaceIdNamesResourceTypeIncludeDeleted(ctx context.Context, workspaceId uint64, nameCn string, nameEn string, resourceType string) (*OnecProjectApplication, error) {
	// 1. 优先查找中文名和英文名都匹配的应用
	if nameCn != "" && nameEn != "" {
		var resp OnecProjectApplication
		query := fmt.Sprintf("select %s from %s where `workspace_id` = ? and `name_cn` = ? and `name_en` = ? and `resource_type` = ? limit 1", onecProjectApplicationRows, m.table)
		err := m.QueryRowNoCacheCtx(ctx, &resp, query, workspaceId, nameCn, nameEn, resourceType)
		if err == nil {
			return &resp, nil
		}
		if !errors.Is(err, sqlx.ErrNotFound) {
			return nil, err
		}
	}

	// 2. 尝试通过中文名匹配
	if nameCn != "" {
		app, err := m.FindOneByWorkspaceIdNameCnResourceTypeIncludeDeleted(ctx, workspaceId, nameCn, resourceType)
		if err == nil {
			return app, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}

	// 3. 尝试通过英文名匹配
	if nameEn != "" {
		app, err := m.FindOneByWorkspaceIdNameEnResourceTypeIncludeDeleted(ctx, workspaceId, nameEn, resourceType)
		if err == nil {
			return app, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}

	return nil, ErrNotFound
}

// FindAllByWorkspaceId 查询工作空间下的所有应用（不包含软删除）
func (m *customOnecProjectApplicationModel) FindAllByWorkspaceId(ctx context.Context, workspaceId uint64) ([]*OnecProjectApplication, error) {
	var resp []*OnecProjectApplication
	query := fmt.Sprintf("select %s from %s where `workspace_id` = ? and `is_deleted` = 0", onecProjectApplicationRows, m.table)
	err := m.QueryRowsNoCacheCtx(ctx, &resp, query, workspaceId)
	switch err {
	case nil:
		return resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}
