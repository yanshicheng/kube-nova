package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OnecProjectVersionModel = (*customOnecProjectVersionModel)(nil)

// VersionDetailInfo 版本详细信息
type VersionDetailInfo struct {
	ClusterUuid       string `db:"cluster_uuid"`
	Namespace         string `db:"namespace"`
	ResourceName      string `db:"resource_name"`
	ResourceType      string `db:"resource_type"`
	ApplicationId     uint64 `db:"application_id"`
	WorkspaceId       uint64 `db:"workspace_id"`
	ResourceClusterId uint64 `db:"resource_cluster_id"`
}

type (
	OnecProjectVersionModel interface {
		onecProjectVersionModel
		// GetVersionResourceName 通过 versionId 获取版本的资源名称
		GetVersionResourceName(ctx context.Context, versionId uint64) (string, error)
		// GetVersionDetailInfo 通过 versionId 获取版本的详细信息
		GetVersionDetailInfo(ctx context.Context, versionId uint64) (*VersionDetailInfo, error)
		// FindOneByApplicationIdResourceNameIncludeDeleted 查询版本（包含软删除的记录）
		FindOneByApplicationIdResourceNameIncludeDeleted(ctx context.Context, applicationId uint64, resourceName string) (*OnecProjectVersion, error)
		// RestoreSoftDeleted 恢复软删除的版本
		RestoreSoftDeleted(ctx context.Context, id uint64, operator string) error
		// FindAllByApplicationId 查询应用下的所有版本（不包含软删除）
		FindAllByApplicationId(ctx context.Context, applicationId uint64) ([]*OnecProjectVersion, error)
	}

	customOnecProjectVersionModel struct {
		*defaultOnecProjectVersionModel
	}
)

// NewOnecProjectVersionModel returns a model for the database table.
func NewOnecProjectVersionModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OnecProjectVersionModel {
	return &customOnecProjectVersionModel{
		defaultOnecProjectVersionModel: newOnecProjectVersionModel(conn, c, opts...),
	}
}

// GetVersionResourceName 通过 versionId 获取版本的资源名称
func (m *customOnecProjectVersionModel) GetVersionResourceName(ctx context.Context, versionId uint64) (string, error) {
	query := `
        SELECT opv.resource_name
        FROM onec_project_version opv
        WHERE opv.id = ? AND opv.is_deleted = 0
        LIMIT 1
    `

	var resourceName string
	err := m.QueryRowNoCacheCtx(ctx, &resourceName, query, versionId)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return "", errors.New("version not found")
		}
		return "", err
	}

	return resourceName, nil
}

// GetVersionDetailInfo 通过 versionId 获取版本的详细信息
func (m *customOnecProjectVersionModel) GetVersionDetailInfo(ctx context.Context, versionId uint64) (*VersionDetailInfo, error) {
	query := `
        SELECT
            opw.cluster_uuid,
            opw.namespace,
            opv.resource_name,
            opa.resource_type,
            opa.id as application_id,
            opa.workspace_id,
            opw.project_cluster_id as resource_cluster_id
        FROM onec_project_version opv
        INNER JOIN onec_project_application opa ON opv.application_id = opa.id
        INNER JOIN onec_project_workspace opw ON opa.workspace_id = opw.id
        WHERE opv.id = ?
            AND opv.is_deleted = 0
            AND opa.is_deleted = 0
            AND opw.is_deleted = 0
        LIMIT 1
    `

	var info VersionDetailInfo
	err := m.QueryRowNoCacheCtx(ctx, &info, query, versionId)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, errors.New("version, application or workspace not found")
		}
		return nil, err
	}

	return &info, nil
}

// FindOneByApplicationIdResourceNameIncludeDeleted 查询版本（包含软删除的记录）
func (m *customOnecProjectVersionModel) FindOneByApplicationIdResourceNameIncludeDeleted(ctx context.Context, applicationId uint64, resourceName string) (*OnecProjectVersion, error) {
	var resp OnecProjectVersion
	query := fmt.Sprintf("select %s from %s where `application_id` = ? and `resource_name` = ? limit 1", onecProjectVersionRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &resp, query, applicationId, resourceName)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

// RestoreSoftDeleted 恢复软删除的版本
func (m *customOnecProjectVersionModel) RestoreSoftDeleted(ctx context.Context, id uint64, operator string) error {
	// 先查询获取信息用于清除缓存（不带 is_deleted 条件）
	var data OnecProjectVersion
	query := fmt.Sprintf("select %s from %s where `id` = ? limit 1", onecProjectVersionRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &data, query, id)
	if err != nil {
		return err
	}

	cacheKeyId := fmt.Sprintf("%s%v", cacheIkubeopsOnecProjectVersionIdPrefix, id)
	cacheKeyResourceName := fmt.Sprintf("%s%v:%v", cacheIkubeopsOnecProjectVersionApplicationIdResourceNamePrefix, data.ApplicationId, data.ResourceName)
	cacheKeyVersion := fmt.Sprintf("%s%v:%v", cacheIkubeopsOnecProjectVersionApplicationIdVersionPrefix, data.ApplicationId, data.Version)

	_, err = m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (result sql.Result, err error) {
		updateQuery := fmt.Sprintf("update %s set `is_deleted` = 0, `status` = 1, `updated_by` = ?, `updated_at` = NOW() where `id` = ?", m.table)
		return conn.ExecCtx(ctx, updateQuery, operator, id)
	}, cacheKeyId, cacheKeyResourceName, cacheKeyVersion)

	return err
}

// FindAllByApplicationId 查询应用下的所有版本（不包含软删除）
func (m *customOnecProjectVersionModel) FindAllByApplicationId(ctx context.Context, applicationId uint64) ([]*OnecProjectVersion, error) {
	var resp []*OnecProjectVersion
	query := fmt.Sprintf("select %s from %s where `application_id` = ? and `is_deleted` = 0 order by created_at desc", onecProjectVersionRows, m.table)
	err := m.QueryRowsNoCacheCtx(ctx, &resp, query, applicationId)
	switch err {
	case nil:
		return resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}
