package logservicelogic

import (
	"context"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
)

const platformEvalTaskOperator = "log-alert-engine"

func upsertPlatformEvalTask(ctx context.Context, svcCtx *svc.ServiceContext, rule *model.OnecLogAlertRule) error {
	if svcCtx == nil || rule == nil || !isPlatformManagedLogBackend(svcCtx, rule.BackendType) {
		return nil
	}

	ensureLogAlertRuleDefaults(rule)
	now := time.Now()
	interval := time.Minute
	if parsed, err := time.ParseDuration(rule.EvalInterval); err == nil && parsed > 0 {
		interval = parsed
	}

	task, err := svcCtx.OnecLogAlertEvalTaskModel.FindOneByRuleId(ctx, rule.Id)
	if err != nil && err != model.ErrNotFound {
		return err
	}

	status := "pending"
	nextEvalAt := now
	lastErr := ""
	if rule.Enabled != 1 {
		status = "disabled"
		nextEvalAt = now.Add(interval)
	}

	if task == nil {
		_, err = svcCtx.OnecLogAlertEvalTaskModel.Insert(ctx, &model.OnecLogAlertEvalTask{
			RuleId:          rule.Id,
			ClusterUuid:     rule.ClusterUuid,
			BackendType:     strings.ToLower(strings.TrimSpace(rule.BackendType)),
			Priority:        100,
			CostScore:       100,
			EvalIntervalSec: int64(interval / time.Second),
			NextEvalAt:      nextEvalAt,
			LastEvalStatus:  status,
			LastEvalError:   lastErr,
			Version:         rule.RuleVersion,
			CreatedBy:       platformEvalTaskOperator,
			UpdatedBy:       platformEvalTaskOperator,
			IsDeleted:       0,
		})
		return err
	}

	task.ClusterUuid = rule.ClusterUuid
	task.BackendType = strings.ToLower(strings.TrimSpace(rule.BackendType))
	task.EvalIntervalSec = int64(interval / time.Second)
	task.NextEvalAt = nextEvalAt
	task.LastEvalStatus = status
	task.LastEvalError = lastErr
	task.Version = rule.RuleVersion
	task.UpdatedBy = platformEvalTaskOperator
	task.IsDeleted = 0
	return svcCtx.OnecLogAlertEvalTaskModel.Update(ctx, task)
}

func deletePlatformEvalTask(ctx context.Context, svcCtx *svc.ServiceContext, ruleID uint64) error {
	if svcCtx == nil || ruleID == 0 {
		return nil
	}
	task, err := svcCtx.OnecLogAlertEvalTaskModel.FindOneByRuleId(ctx, ruleID)
	if err != nil {
		if err == model.ErrNotFound {
			goto cleanup
		}
		return err
	}
	if err := svcCtx.OnecLogAlertEvalTaskModel.DeleteSoft(ctx, task.Id); err != nil {
		return err
	}

cleanup:
	states, err := svcCtx.OnecLogAlertEvalStateModel.SearchNoPage(ctx, "id", true, "`rule_id` = ?", ruleID)
	if err != nil && err != model.ErrNotFound {
		return err
	}
	for _, state := range states {
		_ = svcCtx.OnecLogAlertEvalStateModel.DeleteSoft(ctx, state.Id)
	}

	events, err := svcCtx.OnecLogAlertFireEventModel.SearchNoPage(ctx, "id", true, "`rule_id` = ?", ruleID)
	if err != nil && err != model.ErrNotFound {
		return err
	}
	for _, event := range events {
		_ = svcCtx.OnecLogAlertFireEventModel.DeleteSoft(ctx, event.Id)
	}
	return nil
}
