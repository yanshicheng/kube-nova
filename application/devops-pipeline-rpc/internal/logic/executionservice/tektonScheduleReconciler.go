package executionservicelogic

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

const tektonScheduleReconcileInterval = 30 * time.Second
const tektonScheduleMissedRunLookback = 24 * time.Hour
const tektonScheduleMaxScanSteps = 2000

type tektonScheduleConfig struct {
	Enabled           bool              `json:"enabled"`
	Cron              string            `json:"cron"`
	Timezone          string            `json:"timezone"`
	ConcurrencyPolicy string            `json:"concurrencyPolicy"`
	ParameterSnapshot bool              `json:"parameterSnapshot"`
	ParamsSnapshot    map[string]string `json:"paramsSnapshot"`
}

type tektonScheduleState struct {
	Signature string
	NextRun   time.Time
}

var tektonScheduleStates sync.Map

// StartTektonScheduleReconciler 按 Cron 配置触发 Tekton 定时流水线。
func StartTektonScheduleReconciler(svcCtx *svc.ServiceContext) {
	go func() {
		time.Sleep(5 * time.Second)
		ticker := time.NewTicker(tektonScheduleReconcileInterval)
		defer ticker.Stop()
		for {
			reconcileTektonSchedules(context.Background(), svcCtx)
			<-ticker.C
		}
	}()
}

func reconcileTektonSchedules(ctx context.Context, svcCtx *svc.ServiceContext) {
	items, err := svcCtx.PipelineModel.ListTektonScheduled(ctx, 500)
	if err != nil {
		logx.WithContext(ctx).Errorf("扫描 Tekton 定时流水线失败: %v", err)
		return
	}
	seen := make(map[string]struct{}, len(items))
	now := time.Now()
	for _, pipeline := range items {
		if pipeline == nil || pipeline.EngineType != engineTekton {
			continue
		}
		pipelineID := pipeline.ID.Hex()
		if !isRunnableTektonScheduledPipeline(pipeline) {
			tektonScheduleStates.Delete(pipelineID)
			continue
		}
		seen[pipelineID] = struct{}{}
		cfg, ok := tektonPipelineScheduleConfig(pipeline)
		if !ok {
			tektonScheduleStates.Delete(pipelineID)
			continue
		}
		reconcileTektonPipelineSchedule(ctx, svcCtx, pipeline, cfg, now)
	}
	tektonScheduleStates.Range(func(key, value any) bool {
		if id, ok := key.(string); ok {
			if _, exists := seen[id]; !exists {
				tektonScheduleStates.Delete(id)
			}
		}
		return true
	})
}

func isRunnableTektonScheduledPipeline(pipeline *model.DevopsPipeline) bool {
	return pipeline != nil &&
		pipeline.EngineType == engineTekton &&
		pipeline.Status == 1 &&
		pipeline.SyncStatus == "synced"
}

func reconcileTektonPipelineSchedule(ctx context.Context, svcCtx *svc.ServiceContext, pipeline *model.DevopsPipeline, cfg tektonScheduleConfig, now time.Time) {
	loc, err := time.LoadLocation(strings.TrimSpace(cfg.Timezone))
	if err != nil {
		logx.WithContext(ctx).Errorf("Tekton 定时流水线时区无效: pipelineId=%s timezone=%s err=%v", pipeline.ID.Hex(), cfg.Timezone, err)
		return
	}
	schedule, err := cron.ParseStandard("CRON_TZ=" + strings.TrimSpace(cfg.Timezone) + " " + strings.TrimSpace(cfg.Cron))
	if err != nil {
		logx.WithContext(ctx).Errorf("Tekton 定时流水线 Cron 无效: pipelineId=%s cron=%s err=%v", pipeline.ID.Hex(), cfg.Cron, err)
		return
	}
	signature := tektonScheduleSignature(cfg)
	state := loadTektonScheduleState(pipeline.ID.Hex())
	nowInLoc := now.In(loc)
	if state.Signature != signature || state.NextRun.IsZero() {
		state = tektonScheduleState{
			Signature: signature,
			NextRun:   schedule.Next(nowInLoc),
		}
		tektonScheduleStates.Store(pipeline.ID.Hex(), state)
		if dueAt, ok := recoverableTektonScheduleDueTime(ctx, svcCtx, pipeline, schedule, nowInLoc); ok {
			triggerTektonScheduledPipeline(ctx, svcCtx, pipeline, cfg, signature, dueAt)
		}
		return
	}
	if nowInLoc.Before(state.NextRun) {
		return
	}
	dueAt := state.NextRun
	nextRun := schedule.Next(dueAt)
	for !nextRun.After(nowInLoc) {
		nextRun = schedule.Next(nextRun)
	}
	state.NextRun = nextRun
	tektonScheduleStates.Store(pipeline.ID.Hex(), state)
	triggerTektonScheduledPipeline(ctx, svcCtx, pipeline, cfg, signature, dueAt)
}

func loadTektonScheduleState(pipelineID string) tektonScheduleState {
	value, ok := tektonScheduleStates.Load(pipelineID)
	if !ok {
		return tektonScheduleState{}
	}
	state, _ := value.(tektonScheduleState)
	return state
}

func recoverableTektonScheduleDueTime(ctx context.Context, svcCtx *svc.ServiceContext, pipeline *model.DevopsPipeline, schedule cron.Schedule, now time.Time) (time.Time, bool) {
	lastDue, ok := latestTektonScheduleDueTime(schedule, now, tektonScheduleMissedRunLookback)
	if !ok {
		return time.Time{}, false
	}
	latestRun, err := svcCtx.PipelineRunModel.FindLatestScheduledByPipeline(ctx, pipeline.ID.Hex())
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		logx.WithContext(ctx).Errorf("查询 Tekton 定时流水线最近运行失败: pipelineId=%s err=%v", pipeline.ID.Hex(), err)
		return time.Time{}, false
	}
	if err != nil {
		latestRun = nil
	}
	return lastDue, shouldTriggerTektonMissedSchedule(pipeline, latestRun, lastDue)
}

func latestTektonScheduleDueTime(schedule cron.Schedule, now time.Time, lookback time.Duration) (time.Time, bool) {
	if schedule == nil || lookback <= 0 {
		return time.Time{}, false
	}
	cursor := now.Add(-lookback)
	var latest time.Time
	for i := 0; i < tektonScheduleMaxScanSteps; i++ {
		next := schedule.Next(cursor)
		if next.IsZero() || next.After(now) {
			break
		}
		latest = next
		cursor = next
	}
	if latest.IsZero() {
		return time.Time{}, false
	}
	return latest, true
}

func shouldTriggerTektonMissedSchedule(pipeline *model.DevopsPipeline, latestRun *model.DevopsPipelineRun, lastDue time.Time) bool {
	if pipeline == nil || lastDue.IsZero() {
		return false
	}
	if !pipeline.UpdateAt.IsZero() && pipeline.UpdateAt.After(lastDue) {
		return false
	}
	if latestRun == nil || latestRun.CreateAt.IsZero() {
		return true
	}
	return latestRun.CreateAt.Before(lastDue)
}

func triggerTektonScheduledPipeline(ctx context.Context, svcCtx *svc.ServiceContext, pipeline *model.DevopsPipeline, cfg tektonScheduleConfig, signature string, dueAt time.Time) {
	if !acquireTektonScheduleRunLock(ctx, svcCtx, pipeline.ID.Hex(), signature, dueAt) {
		return
	}
	if err := applyTektonRunConcurrencyPolicy(ctx, svcCtx, pipeline.ID.Hex(), cfg.ConcurrencyPolicy, "", "system", 0, []string{"SUPER_ADMIN"}); err != nil {
		logx.WithContext(ctx).Infof("Tekton 定时流水线并发策略阻止本次触发: pipelineId=%s err=%v", pipeline.ID.Hex(), err)
		return
	}
	params, err := tektonScheduledParams(pipeline, cfg)
	if err != nil {
		logx.WithContext(ctx).Errorf("生成 Tekton 定时流水线参数失败: pipelineId=%s err=%v", pipeline.ID.Hex(), err)
		return
	}
	paramsBytes, _ := json.Marshal(params)
	if _, err := NewPipelineRunLogic(ctx, svcCtx).PipelineRun(&pb.RunPipelineReq{
		Id:              pipeline.ID.Hex(),
		Params:          string(paramsBytes),
		TriggerType:     "scheduled",
		CurrentUserId:   0,
		CurrentUsername: "system",
		CurrentRoles:    []string{"SUPER_ADMIN"},
	}); err != nil {
		logx.WithContext(ctx).Errorf("触发 Tekton 定时流水线失败: pipelineId=%s err=%v", pipeline.ID.Hex(), err)
	}
}

func acquireTektonScheduleRunLock(ctx context.Context, svcCtx *svc.ServiceContext, pipelineID, signature string, dueAt time.Time) bool {
	if svcCtx == nil || svcCtx.Cache == nil || dueAt.IsZero() {
		return true
	}
	sum := sha1.Sum([]byte(signature))
	key := fmt.Sprintf("devops:tekton:schedule:%s:%x:%d", pipelineID, sum, dueAt.Unix())
	ok, err := svcCtx.Cache.SetnxExCtx(ctx, key, "1", int((tektonScheduleMissedRunLookback + time.Hour).Seconds()))
	if err != nil {
		logx.WithContext(ctx).Errorf("获取 Tekton 定时流水线触发锁失败: pipelineId=%s err=%v", pipelineID, err)
		return false
	}
	if !ok {
		logx.WithContext(ctx).Infof("Tekton 定时流水线已由其他调度器处理: pipelineId=%s dueAt=%s", pipelineID, dueAt.Format(time.RFC3339))
	}
	return ok
}

func activePipelineRuns(ctx context.Context, svcCtx *svc.ServiceContext, pipelineID string) []*model.DevopsPipelineRun {
	result := make([]*model.DevopsPipelineRun, 0)
	for _, status := range activeRunStatuses() {
		items, _, err := svcCtx.PipelineRunModel.List(ctx, model.DevopsPipelineRunListFilter{
			PipelineID: pipelineID,
			Status:     status,
			Page:       1,
			PageSize:   50,
			Limit:      50,
		})
		if err != nil {
			logx.WithContext(ctx).Errorf("查询流水线运行状态失败: pipelineId=%s status=%s err=%v", pipelineID, status, err)
			continue
		}
		for _, item := range items {
			if item != nil && item.EngineType == engineTekton {
				result = append(result, item)
			}
		}
	}
	return result
}

func applyTektonRunConcurrencyPolicy(ctx context.Context, svcCtx *svc.ServiceContext, pipelineID, policy, currentRunID, operator string, userID uint64, roles []string) error {
	switch normalizeTektonRunConcurrency(policy) {
	case "allow":
		return nil
	case "skip":
		for _, run := range activePipelineRuns(ctx, svcCtx, pipelineID) {
			if run != nil && run.ID.Hex() != currentRunID {
				return errorx.Msg("已有 Tekton PipelineRun 正在运行")
			}
		}
	case "replace":
		for _, run := range activePipelineRuns(ctx, svcCtx, pipelineID) {
			if run == nil || run.ID.Hex() == currentRunID {
				continue
			}
			if _, err := NewPipelineRunStopLogic(ctx, svcCtx).PipelineRunStop(&pb.StopPipelineRunReq{
				RunId:         run.ID.Hex(),
				CurrentUserId: userID,
				CurrentRoles:  roles,
				Operator:      operator,
			}); err != nil {
				logx.WithContext(ctx).Errorf("停止 Tekton 流水线旧运行失败: runId=%s err=%v", run.ID.Hex(), err)
				return err
			}
		}
	}
	return nil
}

func tektonScheduledParams(pipeline *model.DevopsPipeline, cfg tektonScheduleConfig) (map[string]string, error) {
	if cfg.ParameterSnapshot && len(cfg.ParamsSnapshot) > 0 {
		raw, _ := json.Marshal(cfg.ParamsSnapshot)
		return pipelineParamsMap(pipeline.Params, string(raw))
	}
	return pipelineParamsMap(pipeline.Params, "")
}

func tektonPipelineScheduleConfig(pipeline *model.DevopsPipeline) (tektonScheduleConfig, bool) {
	cfg, ok := tektonScheduleConfigFromJSON(pipeline.TektonTriggerConfig)
	if !ok {
		cfg, ok = tektonScheduleConfigFromJSON(pipeline.TektonDagConfig)
	}
	if !ok || !cfg.Enabled || strings.TrimSpace(cfg.Cron) == "" {
		return tektonScheduleConfig{}, false
	}
	if strings.TrimSpace(cfg.Timezone) == "" {
		cfg.Timezone = "Asia/Shanghai"
	}
	cfg.ConcurrencyPolicy = normalizeTektonScheduleConcurrency(cfg.ConcurrencyPolicy)
	return cfg, true
}

func tektonScheduleConfigFromJSON(content string) (tektonScheduleConfig, bool) {
	var doc struct {
		ScheduleConfig tektonScheduleConfig `json:"scheduleConfig"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &doc); err != nil {
		return tektonScheduleConfig{}, false
	}
	if !doc.ScheduleConfig.Enabled {
		return tektonScheduleConfig{}, false
	}
	return doc.ScheduleConfig, true
}

func normalizeTektonScheduleConcurrency(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "allow", "replace":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "skip"
	}
}

func normalizeTektonRunConcurrency(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "skip", "replace":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "allow"
	}
}

func tektonScheduleSignature(cfg tektonScheduleConfig) string {
	bytes, _ := json.Marshal(cfg)
	return string(bytes)
}
