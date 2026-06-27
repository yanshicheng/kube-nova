package model

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ ProjectMemberBindingModel = (*customProjectMemberBindingModel)(nil)

const projectMemberBindingWithUserRows = "pmb.`id`,pmb.`project_id`,pmb.`user_id`,su.`username`,su.`nickname`,pmb.`role`,pmb.`created_by`,pmb.`updated_by`,pmb.`created_at`,pmb.`updated_at`,pmb.`is_deleted`"

type (
	ProjectMemberBindingModel interface {
		projectMemberBindingModel
		SetProjectMembers(ctx context.Context, projectId uint64, members []*ProjectMemberBinding, updatedBy string) error
		ListByProject(ctx context.Context, projectId uint64) ([]*ProjectMemberBindingWithUser, error)
		ListProjectIdsByUser(ctx context.Context, userId uint64) ([]uint64, error)
		ListUserIdsByPlatform(ctx context.Context, platformId, page, pageSize uint64) ([]uint64, uint64, error)
	}

	customProjectMemberBindingModel struct {
		*defaultProjectMemberBindingModel
	}

	ProjectMemberBindingWithUser struct {
		Id        uint64    `db:"id"`
		ProjectId uint64    `db:"project_id"`
		UserId    uint64    `db:"user_id"`
		Username  string    `db:"username"`
		Nickname  string    `db:"nickname"`
		Role      string    `db:"role"`
		CreatedBy string    `db:"created_by"`
		UpdatedBy string    `db:"updated_by"`
		CreatedAt time.Time `db:"created_at"`
		UpdatedAt time.Time `db:"updated_at"`
		IsDeleted int64     `db:"is_deleted"`
	}
)

func NewProjectMemberBindingModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) ProjectMemberBindingModel {
	return &customProjectMemberBindingModel{
		defaultProjectMemberBindingModel: newProjectMemberBindingModel(conn, c, opts...),
	}
}

func (m *customProjectMemberBindingModel) SetProjectMembers(ctx context.Context, projectId uint64, members []*ProjectMemberBinding, updatedBy string) error {
	if updatedBy == "" {
		updatedBy = "system"
	}
	return m.TransCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		deleteQuery := fmt.Sprintf("update %s set `is_deleted` = 1, `updated_by` = ?, `updated_at` = now() where `project_id` = ? and `is_deleted` = 0", m.table)
		if _, err := session.ExecCtx(ctx, deleteQuery, updatedBy, projectId); err != nil {
			return err
		}
		insertQuery := fmt.Sprintf("insert into %s (`project_id`,`user_id`,`role`,`created_by`,`updated_by`,`is_deleted`) values (?, ?, ?, ?, ?, 0) on duplicate key update `role` = values(`role`), `updated_by` = values(`updated_by`), `updated_at` = now(), `is_deleted` = 0", m.table)
		for _, member := range members {
			if member == nil {
				continue
			}
			role := member.Role
			if role == "" {
				role = "member"
			}
			if _, err := session.ExecCtx(ctx, insertQuery, projectId, member.UserId, role, updatedBy, updatedBy); err != nil {
				return err
			}
		}
		return nil
	})
}

func (m *customProjectMemberBindingModel) ListByProject(ctx context.Context, projectId uint64) ([]*ProjectMemberBindingWithUser, error) {
	query := fmt.Sprintf("select %s from %s pmb inner join `sys_user` su on su.`id` = pmb.`user_id` and su.`is_deleted` = 0 where pmb.`project_id` = ? and pmb.`is_deleted` = 0 order by pmb.`id` asc", projectMemberBindingWithUserRows, m.table)
	var resp []*ProjectMemberBindingWithUser
	if err := m.QueryRowsNoCacheCtx(ctx, &resp, query, projectId); err != nil {
		if err == sqlx.ErrNotFound {
			return []*ProjectMemberBindingWithUser{}, nil
		}
		return nil, err
	}
	return resp, nil
}

func (m *customProjectMemberBindingModel) ListProjectIdsByUser(ctx context.Context, userId uint64) ([]uint64, error) {
	query := fmt.Sprintf("select `project_id` from %s where `user_id` = ? and `is_deleted` = 0 order by `project_id` asc", m.table)
	var ids []uint64
	if err := m.QueryRowsNoCacheCtx(ctx, &ids, query, userId); err != nil {
		if err == sqlx.ErrNotFound {
			return []uint64{}, nil
		}
		return nil, err
	}
	return ids, nil
}

func (m *customProjectMemberBindingModel) ListUserIdsByPlatform(ctx context.Context, platformId, page, pageSize uint64) ([]uint64, uint64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}

	from := fmt.Sprintf("from %s pmb inner join `project_platform_binding` ppb on ppb.`project_id` = pmb.`project_id` and ppb.`platform_id` = ? and ppb.`is_deleted` = 0 where pmb.`is_deleted` = 0", m.table)
	countQuery := "select count(distinct pmb.`user_id`) " + from
	var total uint64
	if err := m.QueryRowNoCacheCtx(ctx, &total, countQuery, platformId); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []uint64{}, 0, nil
	}

	query := "select distinct pmb.`user_id` " + from + " order by pmb.`user_id` asc limit ?, ?"
	offset := (page - 1) * pageSize
	var ids []uint64
	if err := m.QueryRowsNoCacheCtx(ctx, &ids, query, platformId, offset, pageSize); err != nil {
		if err == sqlx.ErrNotFound {
			return []uint64{}, total, nil
		}
		return nil, 0, err
	}
	return ids, total, nil
}

func projectIdsCondition(field string, ids []uint64) (string, []any) {
	if len(ids) == 0 {
		return "1 = 0", nil
	}
	placeholders := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	return fmt.Sprintf("%s in (%s)", field, strings.Join(placeholders, ",")), args
}
