package inspection

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
)

const manualFinishDefaultMessage = "手动结束巡检任务"

func FinishRecordManually(ctx context.Context, svcCtx *svc.ServiceContext, recordID uint64, message, operator string) (*model.OnecInspectionRecord, error) {
	if recordID == 0 {
		return nil, fmt.Errorf("巡检记录ID不能为空")
	}
	if ctx == nil || ctx.Err() != nil {
		ctx = context.Background()
	}
	record, err := svcCtx.OnecInspectionRecordModel.FindOne(ctx, recordID)
	if err != nil || record.IsDeleted == 1 {
		return nil, fmt.Errorf("巡检记录不存在")
	}
	if record.Status != statusRunning {
		return record, nil
	}

	message = strings.TrimSpace(message)
	if message == "" {
		message = manualFinishDefaultMessage
	}
	operator = strings.TrimSpace(operator)
	if operator == "" {
		operator = systemOperator
	}

	results, err := svcCtx.OnecInspectionResultModel.SearchNoPage(ctx, "id", true, "`record_id` = ?", recordID)
	if err != nil && err != model.ErrNotFound {
		return nil, fmt.Errorf("查询巡检结果失败: %w", err)
	}

	now := time.Now()
	record.FinishedAt = sql.NullTime{Time: now, Valid: true}
	if record.StartedAt.Valid {
		record.DurationMs = now.Sub(record.StartedAt.Time).Milliseconds()
	}
	record.TotalCount = 0
	record.SuccessCount = 0
	record.WarningCount = 0
	record.CriticalCount = 0
	record.FailedCount = 0
	applyRecordStats(record, results)
	record.Status = statusFailed
	record.HealthLevel = healthUnknown
	record.Score = 0
	record.ErrorMessage = message
	record.UpdatedBy = operator

	summary := buildSummary(results)
	record.SummaryJson = nullableJSON(summary)
	record.ReportJson = nullableJSON(reportPayload{
		GeneratedAt: now.Unix(),
		RecordNo:    record.RecordNo,
		ClusterUuid: record.ClusterUuid,
		ClusterName: record.ClusterName,
		Score:       record.Score,
		HealthLevel: record.HealthLevel,
		Summary:     summary,
	})
	if err := svcCtx.OnecInspectionRecordModel.Update(ctx, record); err != nil {
		return nil, fmt.Errorf("更新巡检记录失败: %w", err)
	}
	appendRecordLog(ctx, svcCtx, record.Id, "warning", message)

	if record.TaskId > 0 {
		updateManualFinishTask(ctx, svcCtx, record, message, operator, now)
	}
	return record, nil
}

func updateManualFinishTask(ctx context.Context, svcCtx *svc.ServiceContext, record *model.OnecInspectionRecord, message, operator string, now time.Time) {
	task, err := svcCtx.OnecInspectionTaskModel.FindOne(ctx, record.TaskId)
	if err != nil || task == nil || task.IsDeleted == 1 {
		return
	}
	task.LastRunAt = sql.NullTime{Time: now, Valid: true}
	task.LastRecordId = record.Id
	task.LastStatus = statusFailed
	task.LastError = message
	task.UpdatedBy = operator
	if strings.EqualFold(task.ScheduleType, "cron") {
		task.NextRunAt = nextRunTime(task.CronExpr, now)
	}
	_ = svcCtx.OnecInspectionTaskModel.Update(ctx, task)
}
