package model

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OnecProjectModel = (*customOnecProjectModel)(nil)

type (
	OnecProjectModel interface {
		onecProjectModel
		// InsertWithUuid 创建项目（自动生成 UUID）
		InsertWithUuid(ctx context.Context, name string, isSystem int64, description string, createdBy string) (*OnecProject, error)
		// FindOneByUuidIncludeDeleted 根据 UUID 查询项目（包含软删除的记录）
		FindOneByUuidIncludeDeleted(ctx context.Context, uuid string) (*OnecProject, error)
		// BatchFindByIds 批量根据 ID 查询项目
		BatchFindByIds(ctx context.Context, ids []uint64) ([]*OnecProject, error)
	}

	customOnecProjectModel struct {
		*defaultOnecProjectModel
	}
)

func NewOnecProjectModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OnecProjectModel {
	return &customOnecProjectModel{
		defaultOnecProjectModel: newOnecProjectModel(conn, c, opts...),
	}
}

// InsertWithUuid 创建项目（自动生成 UUID）
func (m *customOnecProjectModel) InsertWithUuid(ctx context.Context, name string, isSystem int64, description string, createdBy string) (*OnecProject, error) {
	projectUuid := uuid.New().String()
	project := &OnecProject{
		Name:        name,
		Uuid:        projectUuid,
		IsSystem:    isSystem,
		Description: description,
		CreatedBy:   createdBy,
		UpdatedBy:   createdBy,
		IsDeleted:   0,
	}

	result, err := m.Insert(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("创建项目失败: %v", err)
	}

	insertId, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("获取插入ID失败: %v", err)
	}
	project.Id = uint64(insertId)

	return project, nil
}

// FindOneByUuidIncludeDeleted 根据 UUID 查询项目（包含软删除的记录）
func (m *customOnecProjectModel) FindOneByUuidIncludeDeleted(ctx context.Context, uuid string) (*OnecProject, error) {
	var resp OnecProject
	query := fmt.Sprintf("select %s from %s where `uuid` = ? limit 1", onecProjectRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &resp, query, uuid)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// BatchFindByIds 批量根据 ID 查询项目
func (m *customOnecProjectModel) BatchFindByIds(ctx context.Context, ids []uint64) ([]*OnecProject, error) {
	if len(ids) == 0 {
		return []*OnecProject{}, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	query := fmt.Sprintf("select %s from %s where `id` in (%s) and `is_deleted` = 0", onecProjectRows, m.table, placeholders)
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	var resp []*OnecProject
	err := m.QueryRowsNoCacheCtx(ctx, &resp, query, args...)
	return resp, err
}
