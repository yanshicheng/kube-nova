package model

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	ProjectPlatformSyncTaskStatusPending = "pending"
	ProjectPlatformSyncTaskStatusSuccess = "success"
	ProjectPlatformSyncTaskStatusFailed  = "failed"
)

type (
	ProjectPlatformSyncTaskModel interface {
		Upsert(ctx context.Context, task *ProjectPlatformSyncTask) error
		FindPending(ctx context.Context, limit uint64) ([]*ProjectPlatformSyncTask, error)
		MarkSuccess(ctx context.Context, portalProjectUUID, action string) error
		MarkFailed(ctx context.Context, portalProjectUUID, action, lastError string) error
	}

	projectPlatformSyncTaskModel struct {
		conn  sqlx.SqlConn
		table string
	}

	ProjectPlatformSyncTask struct {
		Id                uint64    `db:"id"`
		ProjectId         uint64    `db:"project_id"`
		PortalProjectUuid string    `db:"portal_project_uuid"`
		PlatformCode      string    `db:"platform_code"`
		Action            string    `db:"action"`
		Payload           string    `db:"payload"`
		Status            string    `db:"status"`
		RetryCount        int64     `db:"retry_count"`
		LastError         string    `db:"last_error"`
		LastSyncedAt      time.Time `db:"last_synced_at"`
		CreatedBy         string    `db:"created_by"`
		UpdatedBy         string    `db:"updated_by"`
		CreatedAt         time.Time `db:"created_at"`
		UpdatedAt         time.Time `db:"updated_at"`
		IsDeleted         int64     `db:"is_deleted"`
	}

	ProjectPlatformSyncTaskPayload struct {
		ProjectId         uint64                  `json:"projectId"`
		PortalProjectUuid string                  `json:"portalProjectUuid"`
		Name              string                  `json:"name,omitempty"`
		Description       string                  `json:"description,omitempty"`
		UpdatedBy         string                  `json:"updatedBy,omitempty"`
		Members           []ProjectSyncTaskMember `json:"members,omitempty"`
	}

	ProjectSyncTaskMember struct {
		UserId   uint64 `json:"userId"`
		Username string `json:"username,omitempty"`
		Nickname string `json:"nickname,omitempty"`
		Role     string `json:"role,omitempty"`
	}
)

func NewProjectPlatformSyncTaskModel(conn sqlx.SqlConn, _ cache.CacheConf, _ ...cache.Option) ProjectPlatformSyncTaskModel {
	return &projectPlatformSyncTaskModel{
		conn:  conn,
		table: "`project_platform_sync_task`",
	}
}

func (m *projectPlatformSyncTaskModel) Upsert(ctx context.Context, task *ProjectPlatformSyncTask) error {
	if task == nil {
		return nil
	}
	query := fmt.Sprintf("insert into %s (`project_id`,`portal_project_uuid`,`platform_code`,`action`,`payload`,`status`,`retry_count`,`last_error`,`last_synced_at`,`created_by`,`updated_by`,`is_deleted`) values (?, ?, ?, ?, ?, ?, 0, '', '1970-01-01 00:00:00', ?, ?, 0) on duplicate key update `payload` = values(`payload`), `status` = values(`status`), `retry_count` = 0, `last_error` = '', `updated_by` = values(`updated_by`), `updated_at` = now(), `is_deleted` = 0", m.table)
	_, err := m.conn.ExecCtx(ctx, query, task.ProjectId, task.PortalProjectUuid, task.PlatformCode, task.Action, task.Payload, task.Status, task.CreatedBy, task.UpdatedBy)
	return err
}

func (m *projectPlatformSyncTaskModel) FindPending(ctx context.Context, limit uint64) ([]*ProjectPlatformSyncTask, error) {
	if limit == 0 {
		limit = 20
	}
	query := fmt.Sprintf("select `id`,`project_id`,`portal_project_uuid`,`platform_code`,`action`,`payload`,`status`,`retry_count`,`last_error`,`last_synced_at`,`created_by`,`updated_by`,`created_at`,`updated_at`,`is_deleted` from %s where `platform_code` = 'devops' and `status` in (?, ?) and `retry_count` < 5 and `is_deleted` = 0 order by `updated_at` asc limit %d", m.table, limit)
	var items []*ProjectPlatformSyncTask
	if err := m.conn.QueryRowsCtx(ctx, &items, query, ProjectPlatformSyncTaskStatusPending, ProjectPlatformSyncTaskStatusFailed); err != nil {
		if err == sqlx.ErrNotFound {
			return []*ProjectPlatformSyncTask{}, nil
		}
		return nil, err
	}
	return items, nil
}

func (m *projectPlatformSyncTaskModel) MarkSuccess(ctx context.Context, portalProjectUUID, action string) error {
	query := fmt.Sprintf("update %s set `status` = ?, `last_error` = '', `last_synced_at` = now(), `updated_at` = now() where `portal_project_uuid` = ? and `action` = ? and `is_deleted` = 0", m.table)
	_, err := m.conn.ExecCtx(ctx, query, ProjectPlatformSyncTaskStatusSuccess, portalProjectUUID, action)
	return err
}

func (m *projectPlatformSyncTaskModel) MarkFailed(ctx context.Context, portalProjectUUID, action, lastError string) error {
	query := fmt.Sprintf("update %s set `status` = ?, `retry_count` = `retry_count` + 1, `last_error` = ?, `updated_at` = now() where `portal_project_uuid` = ? and `action` = ? and `is_deleted` = 0", m.table)
	_, err := m.conn.ExecCtx(ctx, query, ProjectPlatformSyncTaskStatusFailed, lastError, portalProjectUUID, action)
	return err
}

func MarshalProjectPlatformSyncTaskPayload(payload *ProjectPlatformSyncTaskPayload) (string, error) {
	if payload == nil {
		return "", nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func UnmarshalProjectPlatformSyncTaskPayload(raw string) (*ProjectPlatformSyncTaskPayload, error) {
	if raw == "" {
		return &ProjectPlatformSyncTaskPayload{}, nil
	}
	var payload ProjectPlatformSyncTaskPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}
