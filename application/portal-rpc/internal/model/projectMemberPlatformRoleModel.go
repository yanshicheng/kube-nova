package model

import (
	"context"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ ProjectMemberPlatformRoleModel = (*customProjectMemberPlatformRoleModel)(nil)

type (
	// ProjectMemberPlatformRoleModel is an interface to be customized, add more methods here,
	// and implement the added methods in customProjectMemberPlatformRoleModel.
	ProjectMemberPlatformRoleModel interface {
		projectMemberPlatformRoleModel
		ReplaceProjectMembersAndRoles(ctx context.Context, projectId uint64, members []*ProjectMemberBinding, roles []*ProjectMemberPlatformRole, updatedBy string) error
		ListByProject(ctx context.Context, projectId uint64) ([]*ProjectMemberPlatformRoleWithPlatform, error)
		ListPlatformIdsByUser(ctx context.Context, userId uint64) ([]uint64, error)
		ListPlatformIdsByUserAndProject(ctx context.Context, userId, projectId uint64) ([]uint64, error)
		ListUserIdsByPlatform(ctx context.Context, platformId, page, pageSize uint64) ([]uint64, uint64, error)
		ListMembersByProjectAndPlatform(ctx context.Context, projectId, platformId uint64) ([]*ProjectMemberBindingWithUser, error)
		GrantPlatformToProjectMembers(ctx context.Context, projectId, platformId uint64, operator string) error
		RevokeProjectPlatform(ctx context.Context, projectId, platformId uint64, operator string) error
	}

	customProjectMemberPlatformRoleModel struct {
		*defaultProjectMemberPlatformRoleModel
	}

	ProjectMemberPlatformRoleWithPlatform struct {
		Id           uint64    `db:"id"`
		ProjectId    uint64    `db:"project_id"`
		UserId       uint64    `db:"user_id"`
		PlatformId   uint64    `db:"platform_id"`
		PlatformCode string    `db:"platform_code"`
		PlatformName string    `db:"platform_name"`
		Role         string    `db:"role"`
		CreatedBy    string    `db:"created_by"`
		UpdatedBy    string    `db:"updated_by"`
		CreatedAt    time.Time `db:"created_at"`
		UpdatedAt    time.Time `db:"updated_at"`
		IsDeleted    int64     `db:"is_deleted"`
	}
)

// NewProjectMemberPlatformRoleModel returns a model for the database table.
func NewProjectMemberPlatformRoleModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) ProjectMemberPlatformRoleModel {
	return &customProjectMemberPlatformRoleModel{
		defaultProjectMemberPlatformRoleModel: newProjectMemberPlatformRoleModel(conn, c, opts...),
	}
}

func (m *customProjectMemberPlatformRoleModel) ReplaceProjectMembersAndRoles(ctx context.Context, projectId uint64, members []*ProjectMemberBinding, roles []*ProjectMemberPlatformRole, updatedBy string) error {
	if updatedBy == "" {
		updatedBy = "system"
	}
	return m.TransCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		if _, err := session.ExecCtx(ctx, "update `project_member_binding` set `is_deleted` = 1, `updated_by` = ?, `updated_at` = now() where `project_id` = ? and `is_deleted` = 0", updatedBy, projectId); err != nil {
			return err
		}
		memberInsert := "insert into `project_member_binding` (`project_id`,`user_id`,`role`,`created_by`,`updated_by`,`is_deleted`) values (?, ?, ?, ?, ?, 0) on duplicate key update `role` = values(`role`), `updated_by` = values(`updated_by`), `updated_at` = now(), `is_deleted` = 0"
		for _, member := range members {
			if member == nil {
				continue
			}
			role := member.Role
			if role == "" {
				role = "member"
			}
			if _, err := session.ExecCtx(ctx, memberInsert, projectId, member.UserId, role, updatedBy, updatedBy); err != nil {
				return err
			}
		}

		if _, err := session.ExecCtx(ctx, fmt.Sprintf("update %s set `is_deleted` = 1, `updated_by` = ?, `updated_at` = now() where `project_id` = ? and `is_deleted` = 0", m.table), updatedBy, projectId); err != nil {
			return err
		}
		roleInsert := fmt.Sprintf("insert into %s (`project_id`,`user_id`,`platform_id`,`role`,`created_by`,`updated_by`,`is_deleted`) values (?, ?, ?, ?, ?, ?, 0) on duplicate key update `role` = values(`role`), `updated_by` = values(`updated_by`), `updated_at` = now(), `is_deleted` = 0", m.table)
		for _, item := range roles {
			if item == nil {
				continue
			}
			role := item.Role
			if role == "" {
				role = "member"
			}
			if _, err := session.ExecCtx(ctx, roleInsert, projectId, item.UserId, item.PlatformId, role, updatedBy, updatedBy); err != nil {
				return err
			}
		}
		return nil
	})
}

func (m *customProjectMemberPlatformRoleModel) ListByProject(ctx context.Context, projectId uint64) ([]*ProjectMemberPlatformRoleWithPlatform, error) {
	query := fmt.Sprintf("select pmpr.`id`,pmpr.`project_id`,pmpr.`user_id`,pmpr.`platform_id`,sp.`platform_code`,sp.`platform_name`,pmpr.`role`,pmpr.`created_by`,pmpr.`updated_by`,pmpr.`created_at`,pmpr.`updated_at`,pmpr.`is_deleted` from %s pmpr inner join `sys_platform` sp on sp.`id` = pmpr.`platform_id` and sp.`is_deleted` = 0 where pmpr.`project_id` = ? and pmpr.`is_deleted` = 0 order by pmpr.`user_id` asc, sp.`sort` asc, pmpr.`platform_id` asc", m.table)
	var resp []*ProjectMemberPlatformRoleWithPlatform
	if err := m.QueryRowsNoCacheCtx(ctx, &resp, query, projectId); err != nil {
		if err == sqlx.ErrNotFound {
			return []*ProjectMemberPlatformRoleWithPlatform{}, nil
		}
		return nil, err
	}
	return resp, nil
}

func (m *customProjectMemberPlatformRoleModel) ListPlatformIdsByUser(ctx context.Context, userId uint64) ([]uint64, error) {
	query := fmt.Sprintf("select distinct pmpr.`platform_id` from %s pmpr inner join `project_platform_binding` ppb on ppb.`project_id` = pmpr.`project_id` and ppb.`platform_id` = pmpr.`platform_id` and ppb.`is_deleted` = 0 where pmpr.`user_id` = ? and pmpr.`is_deleted` = 0 order by pmpr.`platform_id` asc", m.table)
	var ids []uint64
	if err := m.QueryRowsNoCacheCtx(ctx, &ids, query, userId); err != nil {
		if err == sqlx.ErrNotFound {
			return []uint64{}, nil
		}
		return nil, err
	}
	return ids, nil
}

func (m *customProjectMemberPlatformRoleModel) ListPlatformIdsByUserAndProject(ctx context.Context, userId, projectId uint64) ([]uint64, error) {
	query := fmt.Sprintf("select distinct pmpr.`platform_id` from %s pmpr inner join `project_platform_binding` ppb on ppb.`project_id` = pmpr.`project_id` and ppb.`platform_id` = pmpr.`platform_id` and ppb.`is_deleted` = 0 where pmpr.`user_id` = ? and pmpr.`project_id` = ? and pmpr.`is_deleted` = 0 order by pmpr.`platform_id` asc", m.table)
	var ids []uint64
	if err := m.QueryRowsNoCacheCtx(ctx, &ids, query, userId, projectId); err != nil {
		if err == sqlx.ErrNotFound {
			return []uint64{}, nil
		}
		return nil, err
	}
	return ids, nil
}

func (m *customProjectMemberPlatformRoleModel) ListUserIdsByPlatform(ctx context.Context, platformId, page, pageSize uint64) ([]uint64, uint64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	from := fmt.Sprintf("from %s pmpr inner join `project_platform_binding` ppb on ppb.`project_id` = pmpr.`project_id` and ppb.`platform_id` = pmpr.`platform_id` and ppb.`is_deleted` = 0 where pmpr.`platform_id` = ? and pmpr.`is_deleted` = 0", m.table)
	var total uint64
	if err := m.QueryRowNoCacheCtx(ctx, &total, "select count(distinct pmpr.`user_id`) "+from, platformId); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []uint64{}, 0, nil
	}
	offset := (page - 1) * pageSize
	var ids []uint64
	if err := m.QueryRowsNoCacheCtx(ctx, &ids, "select distinct pmpr.`user_id` "+from+" order by pmpr.`user_id` asc limit ?, ?", platformId, offset, pageSize); err != nil {
		if err == sqlx.ErrNotFound {
			return []uint64{}, total, nil
		}
		return nil, 0, err
	}
	return ids, total, nil
}

func (m *customProjectMemberPlatformRoleModel) ListMembersByProjectAndPlatform(ctx context.Context, projectId, platformId uint64) ([]*ProjectMemberBindingWithUser, error) {
	query := fmt.Sprintf("select pmb.`id`,pmb.`project_id`,pmb.`user_id`,su.`username`,su.`nickname`,pmpr.`role`,pmb.`created_by`,pmb.`updated_by`,pmb.`created_at`,pmb.`updated_at`,pmb.`is_deleted` from `project_member_binding` pmb inner join %s pmpr on pmpr.`project_id` = pmb.`project_id` and pmpr.`user_id` = pmb.`user_id` and pmpr.`platform_id` = ? and pmpr.`is_deleted` = 0 inner join `sys_user` su on su.`id` = pmb.`user_id` and su.`is_deleted` = 0 where pmb.`project_id` = ? and pmb.`is_deleted` = 0 order by pmb.`id` asc", m.table)
	var resp []*ProjectMemberBindingWithUser
	if err := m.QueryRowsNoCacheCtx(ctx, &resp, query, platformId, projectId); err != nil {
		if err == sqlx.ErrNotFound {
			return []*ProjectMemberBindingWithUser{}, nil
		}
		return nil, err
	}
	return resp, nil
}

func (m *customProjectMemberPlatformRoleModel) GrantPlatformToProjectMembers(ctx context.Context, projectId, platformId uint64, operator string) error {
	if operator == "" {
		operator = "system"
	}
	query := fmt.Sprintf("insert into %s (`project_id`,`user_id`,`platform_id`,`role`,`created_by`,`updated_by`,`is_deleted`) select pmb.`project_id`,pmb.`user_id`,?,pmb.`role`,?, ?,0 from `project_member_binding` pmb where pmb.`project_id` = ? and pmb.`is_deleted` = 0 on duplicate key update `role` = values(`role`), `updated_by` = values(`updated_by`), `updated_at` = now(), `is_deleted` = 0", m.table)
	_, err := m.ExecSql(ctx, 0, query, platformId, operator, operator, projectId)
	return err
}

func (m *customProjectMemberPlatformRoleModel) RevokeProjectPlatform(ctx context.Context, projectId, platformId uint64, operator string) error {
	if operator == "" {
		operator = "system"
	}
	query := fmt.Sprintf("update %s set `is_deleted` = 1, `updated_by` = ?, `updated_at` = now() where `project_id` = ? and `platform_id` = ? and `is_deleted` = 0", m.table)
	_, err := m.ExecSql(ctx, 0, query, operator, projectId, platformId)
	return err
}
