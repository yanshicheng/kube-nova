package model

import (
	"context"
	"errors"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OnecProjectVersionModel = (*customOnecProjectVersionModel)(nil)

// VersionDetailInfo 版本详细信息
type VersionDetailInfo struct {
	ClusterUuid       string `db:"cluster_uuid"`        // 集群UUID
	Namespace         string `db:"namespace"`           // 命名空间
	ResourceName      string `db:"resource_name"`       // 资源名称
	ResourceType      string `db:"resource_type"`       // 资源类型
	ApplicationId     uint64 `db:"application_id"`      // 应用ID
	WorkspaceId       uint64 `db:"workspace_id"`        // 工作空间ID
	ResourceClusterId uint64 `db:"resource_cluster_id"` // 资源集群ID
}

type (
	// OnecProjectVersionModel is an interface to be customized, add more methods here,
	// and implement the added methods in customOnecProjectVersionModel.
	OnecProjectVersionModel interface {
		onecProjectVersionModel
		// GetVersionResourceName 通过 versionId 获取版本的资源名称
		GetVersionResourceName(ctx context.Context, versionId uint64) (string, error)
		// GetVersionDetailInfo 通过 versionId 获取版本的详细信息（集群UUID、命名空间、资源名称、资源类型）
		GetVersionDetailInfo(ctx context.Context, versionId uint64) (*VersionDetailInfo, error)
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

// GetVersionDetailInfo 通过 versionId 获取版本的详细信息（集群UUID、命名空间、资源名称、资源类型）
func (m *customOnecProjectVersionModel) GetVersionDetailInfo(ctx context.Context, versionId uint64) (*VersionDetailInfo, error) {
	// 关联查询：版本 -> 应用 -> 工作空间
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
