package inspection

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
)

func DeleteRecordHard(ctx context.Context, svcCtx *svc.ServiceContext, recordID uint64, operator string) error {
	if recordID == 0 {
		return fmt.Errorf("巡检记录ID不能为空")
	}
	if ctx == nil || ctx.Err() != nil {
		ctx = context.Background()
	}
	record, err := svcCtx.OnecInspectionRecordModel.FindOne(ctx, recordID)
	if err != nil || record.IsDeleted == 1 {
		return fmt.Errorf("巡检记录不存在")
	}
	if _, err := svcCtx.OnecInspectionResultModel.ExecSql(ctx, 0, "delete from onec_inspection_result where `record_id` = ?", recordID); err != nil {
		return fmt.Errorf("删除巡检结果明细失败: %w", err)
	}
	ClearRecordLog(ctx, svcCtx, recordID)
	if err := svcCtx.OnecInspectionRecordModel.Delete(ctx, recordID); err != nil {
		return fmt.Errorf("删除巡检记录失败: %w", err)
	}
	clearTaskLastRecord(ctx, svcCtx, record, operator)
	return nil
}

func clearTaskLastRecord(ctx context.Context, svcCtx *svc.ServiceContext, record *model.OnecInspectionRecord, operator string) {
	if record == nil || record.TaskId == 0 {
		return
	}
	task, err := svcCtx.OnecInspectionTaskModel.FindOne(ctx, record.TaskId)
	if err != nil || task == nil || task.IsDeleted == 1 || task.LastRecordId != record.Id {
		return
	}
	operator = strings.TrimSpace(operator)
	if operator == "" {
		operator = systemOperator
	}
	task.LastRecordId = 0
	task.LastStatus = ""
	task.LastError = ""
	task.UpdatedBy = operator
	_ = svcCtx.OnecInspectionTaskModel.Update(ctx, task)
}
