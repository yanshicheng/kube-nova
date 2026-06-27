package logalertengine

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/consumer"
	logservicelogic "github.com/yanshicheng/kube-nova/application/manager-rpc/internal/logic/logservice"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/watch/incremental"
	portalpb "github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	esop "github.com/yanshicheng/kube-nova/common/logmanager/operator/elasticsearch"
	logtypes "github.com/yanshicheng/kube-nova/common/logmanager/types"
	"github.com/zeromicro/go-zero/core/logx"
)

const (
	defaultScanInterval    = 5 * time.Second
	systemOperator         = "log-alert-engine"
	esContainerDataStream  = "logs-container-default"
	esSystemDataStream     = "logs-system-default"
	esContainerIndexStream = "logs-container-*"
	esSystemIndexStream    = "logs-system-*"
)

type Service struct {
	svcCtx *svc.ServiceContext
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	locker incremental.DistributedLocker
}

type ruleRuntimeState struct {
	Active               bool   `json:"active"`
	LastEventFingerprint string `json:"lastEventFingerprint,omitempty"`
	SilenceUntilMs       int64  `json:"silenceUntilMs,omitempty"`
}

type evalItem struct {
	task        *model.OnecLogAlertEvalTask
	rule        *model.OnecLogAlertRule
	windowStart time.Time
	windowEnd   time.Time
	release     func()
}

type Stats struct {
	ClusterUuid        string
	TotalRules         uint64
	EnabledRules       uint64
	DisabledRules      uint64
	SuccessRules       uint64
	FailedRules        uint64
	PendingRules       uint64
	ActiveFiringEvents uint64
	FailedNotifyEvents uint64
	DeadNotifyEvents   uint64
	CriticalRules      uint64
	WarningRules       uint64
	InfoRules          uint64
	HotRules           uint64
	WarmRules          uint64
	ColdRules          uint64
}

type scheduleDecision struct {
	Interval  time.Duration
	Priority  int64
	CostScore int64
	Bucket    string
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	var locker incremental.DistributedLocker = incremental.NewNoopLocker("log-alert-engine")
	if svcCtx != nil && svcCtx.Cache != nil && svcCtx.Config.LogAlertEngine.LockEnabled {
		locker = incremental.NewLockWithAutoRenew(svcCtx.Cache, "log-alert-engine")
	}
	return &Service{
		svcCtx: svcCtx,
		ctx:    ctx,
		cancel: cancel,
		locker: locker,
	}
}

func (s *Service) Start() {
	if !s.enabled() {
		logx.Info("[LogAlertEngine] 引擎未启用，跳过启动")
		<-s.ctx.Done()
		return
	}

	logx.Info("[LogAlertEngine] 平台日志告警引擎启动")

	s.wg.Add(3)
	go s.syncLoop()
	go s.evalLoop()
	go s.retryLoop()

	<-s.ctx.Done()
	logx.Info("[LogAlertEngine] 收到停止信号")
}

func (s *Service) Stop() {
	s.cancel()
	s.wg.Wait()
	logx.Info("[LogAlertEngine] 平台日志告警引擎已停止")
}

func (s *Service) enabled() bool {
	return s.svcCtx != nil &&
		s.svcCtx.Config.LogAlertEngine.Enabled &&
		strings.EqualFold(strings.TrimSpace(s.svcCtx.Config.LogAlertEngine.Mode), "platform")
}

func (s *Service) syncLoop() {
	defer s.wg.Done()

	interval := s.svcCtx.Config.LogAlertEngine.RuleSyncInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}

	if err := s.runSyncRuleTasks(); err != nil {
		logx.Errorf("[LogAlertEngine] 初次同步规则失败: %v", err)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if err := s.runSyncRuleTasks(); err != nil {
				logx.Errorf("[LogAlertEngine] 同步规则失败: %v", err)
			}
		}
	}
}

func (s *Service) evalLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(defaultScanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if err := s.evalDueTasks(); err != nil {
				logx.Errorf("[LogAlertEngine] 执行评估任务失败: %v", err)
			}
		}
	}
}

func (s *Service) retryLoop() {
	defer s.wg.Done()

	interval := s.svcCtx.Config.LogAlertEngine.NotifyRetryScanInterval
	if interval <= 0 {
		interval = 15 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if err := s.runRetryFailedNotifications(); err != nil {
				logx.Errorf("[LogAlertEngine] 重试失败通知异常: %v", err)
			}
		}
	}
}

func (s *Service) runSyncRuleTasks() error {
	acquired, release, err := s.tryAcquireGlobalLock("log-alert-sync-rules")
	if err != nil {
		return err
	}
	if !acquired {
		return nil
	}
	defer release()
	return s.syncRuleTasks()
}

func (s *Service) runRetryFailedNotifications() error {
	acquired, release, err := s.tryAcquireGlobalLock("log-alert-retry-scan")
	if err != nil {
		return err
	}
	if !acquired {
		return nil
	}
	defer release()
	return s.retryFailedNotifications()
}

func (s *Service) syncRuleTasks() error {
	rules, err := s.svcCtx.OnecLogAlertRuleModel.SearchNoPage(s.ctx, "id", true, "")
	if err != nil && err != model.ErrNotFound {
		return err
	}

	tasks, err := s.svcCtx.OnecLogAlertEvalTaskModel.SearchNoPage(s.ctx, "id", true, "")
	if err != nil && err != model.ErrNotFound {
		return err
	}

	ruleMap := make(map[uint64]*model.OnecLogAlertRule, len(rules))
	for _, rule := range rules {
		if !isPlatformManagedBackend(rule.BackendType) {
			continue
		}
		ruleMap[rule.Id] = rule
		if err := s.upsertTask(rule); err != nil {
			logx.Errorf("[LogAlertEngine] 同步任务失败: ruleId=%d, err=%v", rule.Id, err)
		}
	}

	for _, task := range tasks {
		if _, ok := ruleMap[task.RuleId]; ok {
			continue
		}
		if task.IsDeleted == 1 {
			continue
		}
		if err := s.svcCtx.OnecLogAlertEvalTaskModel.DeleteSoft(s.ctx, task.Id); err != nil {
			logx.Errorf("[LogAlertEngine] 软删除过期任务失败: taskId=%d, err=%v", task.Id, err)
		}
	}

	return nil
}

func (s *Service) upsertTask(rule *model.OnecLogAlertRule) error {
	if rule == nil {
		return nil
	}

	now := time.Now()
	interval := s.ruleEvalInterval(rule)
	decision := s.deriveSchedule(rule, nil, 0)
	status := "pending"
	lastErr := ""
	if rule.Enabled != 1 {
		status = "disabled"
	}

	exist, err := s.svcCtx.OnecLogAlertEvalTaskModel.FindOneByRuleId(s.ctx, rule.Id)
	if err != nil && err != model.ErrNotFound {
		return err
	}

	if exist == nil {
		_, err = s.svcCtx.OnecLogAlertEvalTaskModel.Insert(s.ctx, &model.OnecLogAlertEvalTask{
			RuleId:          rule.Id,
			ClusterUuid:     rule.ClusterUuid,
			BackendType:     strings.ToLower(strings.TrimSpace(rule.BackendType)),
			Priority:        decision.Priority,
			CostScore:       decision.CostScore,
			EvalIntervalSec: int64(interval / time.Second),
			NextEvalAt:      now,
			LastEvalStatus:  status,
			LastEvalError:   lastErr,
			Version:         rule.RuleVersion,
			CreatedBy:       systemOperator,
			UpdatedBy:       systemOperator,
			IsDeleted:       0,
		})
		return err
	}

	exist.ClusterUuid = rule.ClusterUuid
	exist.BackendType = strings.ToLower(strings.TrimSpace(rule.BackendType))
	exist.Priority = decision.Priority
	exist.CostScore = decision.CostScore
	exist.EvalIntervalSec = int64(interval / time.Second)
	exist.Version = rule.RuleVersion
	exist.UpdatedBy = systemOperator
	exist.IsDeleted = 0
	if rule.Enabled != 1 {
		exist.LastEvalStatus = "disabled"
		exist.LastEvalError = ""
		exist.NextEvalAt = now.Add(interval)
	} else if exist.LastEvalStatus == "disabled" || exist.NextEvalAt.Before(now.Add(-interval)) {
		exist.LastEvalStatus = "pending"
		exist.LastEvalError = ""
		exist.NextEvalAt = now
	}

	return s.svcCtx.OnecLogAlertEvalTaskModel.Update(s.ctx, exist)
}

func (s *Service) evalDueTasks() error {
	now := time.Now()
	tasks, err := s.svcCtx.OnecLogAlertEvalTaskModel.SearchNoPage(s.ctx, "next_eval_at", true, "`next_eval_at` <= ?", now)
	if err != nil {
		if err == model.ErrNotFound {
			return nil
		}
		return err
	}

	maxBatch := s.svcCtx.Config.LogAlertEngine.MaxBatchRules
	if maxBatch <= 0 {
		maxBatch = 100
	}
	if len(tasks) > maxBatch {
		tasks = tasks[:maxBatch]
	}

	clusterBuckets := make(map[string][]*evalItem)
	for _, task := range tasks {
		rule, findErr := s.svcCtx.OnecLogAlertRuleModel.FindOne(s.ctx, task.RuleId)
		if findErr != nil {
			if err := s.markTaskFailed(task, nil, now, "规则不存在"); err != nil {
				logx.Errorf("[LogAlertEngine] 标记任务失败异常: taskId=%d, err=%v", task.Id, err)
			}
			continue
		}
		if rule.Enabled != 1 {
			if err := s.markTaskDisabled(task, rule, now); err != nil {
				logx.Errorf("[LogAlertEngine] 标记任务禁用异常: taskId=%d, ruleId=%d, err=%v", task.Id, task.RuleId, err)
			}
			continue
		}
		acquired, release, lockErr := s.tryAcquireEvalLock(task, rule)
		if lockErr != nil {
			logx.Errorf("[LogAlertEngine] 获取评估锁失败: taskId=%d, ruleId=%d, err=%v", task.Id, task.RuleId, lockErr)
			continue
		}
		if !acquired {
			continue
		}
		bucketKey := s.evalBucketKey(rule)
		clusterBuckets[bucketKey] = append(clusterBuckets[bucketKey], &evalItem{task: task, rule: rule, release: release})
	}

	workerCount := s.svcCtx.Config.LogAlertEngine.ClusterWorkers
	if workerCount <= 0 {
		workerCount = 2
	}
	sem := make(chan struct{}, workerCount)
	var wg sync.WaitGroup
	for bucketKey, items := range clusterBuckets {
		bucketKey := bucketKey
		items := items
		sortEvalItems(items)
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case <-s.ctx.Done():
				return
			case sem <- struct{}{}:
			}
			defer func() { <-sem }()
			if err := s.evalClusterTasks(bucketKey, items, now); err != nil {
				logx.Errorf("[LogAlertEngine] 集群批量评估失败: bucket=%s, err=%v", bucketKey, err)
			}
		}()
	}
	wg.Wait()
	return nil
}

func (s *Service) evalClusterTasks(bucketKey string, items []*evalItem, now time.Time) error {
	if len(items) == 0 {
		return nil
	}
	defer s.releaseEvalItems(items)

	clusterUUID, backendType := splitEvalBucketKey(bucketKey)
	app, err := s.findClusterBackendApp(clusterUUID, backendType)
	if err != nil || app == nil {
		for _, item := range items {
			_ = s.markTaskFailed(item.task, item.rule, now, "未找到默认日志组件配置")
		}
		return nil
	}

	client, err := logservicelogic.BuildLogClientForEngine(s.ctx, app, s.svcCtx.Config.LogSearch)
	if err != nil {
		for _, item := range items {
			_ = s.markTaskFailed(item.task, item.rule, now, err.Error())
		}
		return nil
	}
	switch typed := client.(type) {
	case *esop.Client:
		return s.evalElasticsearchTasks(clusterUUID, items, now, typed)
	default:
		return s.evalLogClientTasks(clusterUUID, backendType, items, now, client)
	}
}

func (s *Service) markTaskSuccess(task *model.OnecLogAlertEvalTask, rule *model.OnecLogAlertRule, now time.Time, decision scheduleDecision) error {
	task.LastEvalStatus = "success"
	task.LastEvalError = ""
	task.LastEvalAt = sql.NullTime{Time: now, Valid: true}
	task.UpdatedBy = systemOperator
	task.Priority = decision.Priority
	task.CostScore = decision.CostScore
	task.EvalIntervalSec = int64(decision.Interval / time.Second)
	task.NextEvalAt = now.Add(decision.Interval)
	if err := s.svcCtx.OnecLogAlertEvalTaskModel.Update(s.ctx, task); err != nil {
		return err
	}
	return s.updateRuleEvalMeta(rule, task.LastEvalStatus, task.LastEvalError, task.LastEvalAt, task.NextEvalAt)
}

func (s *Service) evalElasticsearchTasks(clusterUUID string, items []*evalItem, now time.Time, client *esop.Client) error {
	searchReqs := make([]*logtypes.SearchRequest, 0, len(items))
	reqItems := make([]*evalItem, 0, len(items))
	for _, item := range items {
		searchReq, windowStart, windowEnd, buildErr := s.buildEvalSearchRequest(item.rule, now)
		if buildErr != nil {
			_ = s.markTaskFailed(item.task, item.rule, now, buildErr.Error())
			continue
		}
		item.windowStart = windowStart
		item.windowEnd = windowEnd
		searchReqs = append(searchReqs, searchReq)
		reqItems = append(reqItems, item)
	}
	if len(searchReqs) == 0 {
		return nil
	}

	hot, warm, cold := countBuckets(reqItems)
	batchStart := time.Now()
	logx.Infof("[LogAlertEngine] 批量评估: cluster=%s backend=elasticsearch rules=%d hot=%d warm=%d cold=%d", clusterUUID, len(searchReqs), hot, warm, cold)
	results, err := client.MultiCount(searchReqs)
	if err != nil {
		for _, item := range reqItems {
			_ = s.markTaskFailed(item.task, item.rule, now, err.Error())
		}
		return nil
	}
	perRuleLatencyMs := maxInt64(1, time.Since(batchStart).Milliseconds()/int64(len(reqItems)))
	for idx, result := range results {
		item := reqItems[idx]
		if result.Err != nil {
			_ = s.markTaskFailed(item.task, item.rule, now, result.Err.Error())
			continue
		}
		var sample *logtypes.LogRecord
		if result.Count > 0 {
			sample = fetchAlertSample(client, searchReqs[idx])
		}
		state, handleErr := s.handleEvalResult(item.rule, result.Count, item.windowStart, item.windowEnd, sample)
		if handleErr != nil {
			logx.Errorf("[LogAlertEngine] 处理评估结果失败: taskId=%d, ruleId=%d, err=%v", item.task.Id, item.rule.Id, handleErr)
			_ = s.markTaskFailed(item.task, item.rule, now, handleErr.Error())
			continue
		}
		decision := s.deriveSchedule(item.rule, state, perRuleLatencyMs)
		if err := s.markTaskSuccess(item.task, item.rule, now, decision); err != nil {
			logx.Errorf("[LogAlertEngine] 标记任务成功失败: taskId=%d, ruleId=%d, err=%v", item.task.Id, item.rule.Id, err)
		}
	}
	return nil
}

func (s *Service) evalLogClientTasks(clusterUUID, backendType string, items []*evalItem, now time.Time, client logtypes.LogClient) error {
	hot, warm, cold := countBuckets(items)
	logx.Infof("[LogAlertEngine] 平台评估: cluster=%s backend=%s rules=%d hot=%d warm=%d cold=%d", clusterUUID, backendType, len(items), hot, warm, cold)
	for _, item := range items {
		searchReq, windowStart, windowEnd, buildErr := s.buildEvalSearchRequest(item.rule, now)
		if buildErr != nil {
			_ = s.markTaskFailed(item.task, item.rule, now, buildErr.Error())
			continue
		}
		item.windowStart = windowStart
		item.windowEnd = windowEnd
		startAt := time.Now()
		resp, err := client.Search(searchReq)
		if err != nil {
			_ = s.markTaskFailed(item.task, item.rule, now, err.Error())
			continue
		}
		hitCount := int64(len(resp.List))
		var sample *logtypes.LogRecord
		if len(resp.List) > 0 {
			sample = &resp.List[0]
		}
		state, handleErr := s.handleEvalResult(item.rule, hitCount, item.windowStart, item.windowEnd, sample)
		if handleErr != nil {
			logx.Errorf("[LogAlertEngine] 处理评估结果失败: taskId=%d, ruleId=%d, err=%v", item.task.Id, item.rule.Id, handleErr)
			_ = s.markTaskFailed(item.task, item.rule, now, handleErr.Error())
			continue
		}
		decision := s.deriveSchedule(item.rule, state, maxInt64(1, time.Since(startAt).Milliseconds()))
		if err := s.markTaskSuccess(item.task, item.rule, now, decision); err != nil {
			logx.Errorf("[LogAlertEngine] 标记任务成功失败: taskId=%d, ruleId=%d, err=%v", item.task.Id, item.rule.Id, err)
		}
	}
	return nil
}

func (s *Service) markTaskFailed(task *model.OnecLogAlertEvalTask, rule *model.OnecLogAlertRule, now time.Time, errMsg string) error {
	task.LastEvalStatus = "failed"
	task.LastEvalError = trimErr(errMsg)
	task.UpdatedBy = systemOperator
	task.LastEvalAt = sql.NullTime{Time: now, Valid: true}
	task.NextEvalAt = now.Add(time.Duration(task.EvalIntervalSec) * time.Second)
	if err := s.svcCtx.OnecLogAlertEvalTaskModel.Update(s.ctx, task); err != nil {
		return err
	}
	if rule != nil {
		return s.updateRuleEvalMeta(rule, task.LastEvalStatus, task.LastEvalError, task.LastEvalAt, task.NextEvalAt)
	}
	return nil
}

func (s *Service) markTaskDisabled(task *model.OnecLogAlertEvalTask, rule *model.OnecLogAlertRule, now time.Time) error {
	task.LastEvalStatus = "disabled"
	task.LastEvalError = ""
	task.UpdatedBy = systemOperator
	task.NextEvalAt = now.Add(time.Duration(task.EvalIntervalSec) * time.Second)
	if err := s.svcCtx.OnecLogAlertEvalTaskModel.Update(s.ctx, task); err != nil {
		return err
	}
	return s.updateRuleEvalMeta(rule, task.LastEvalStatus, task.LastEvalError, task.LastEvalAt, task.NextEvalAt)
}

func (s *Service) tryAcquireEvalLock(task *model.OnecLogAlertEvalTask, rule *model.OnecLogAlertRule) (bool, func(), error) {
	if s.locker == nil || task == nil || rule == nil {
		return true, func() {}, nil
	}
	ttl := s.evalLockTTL()
	key := fmt.Sprintf("log-alert-eval:%s:%s:%d", rule.ClusterUuid, strings.ToLower(strings.TrimSpace(rule.BackendType)), rule.Id)
	return s.locker.TryLock(s.ctx, key, ttl)
}

func (s *Service) tryAcquireGlobalLock(key string) (bool, func(), error) {
	if s.locker == nil {
		return true, func() {}, nil
	}
	return s.locker.TryLock(s.ctx, key, s.evalLockTTL())
}

func (s *Service) tryAcquireRetryLock(event *model.OnecLogAlertFireEvent) (bool, func(), error) {
	if s.locker == nil || event == nil {
		return true, func() {}, nil
	}
	return s.locker.TryLock(s.ctx, fmt.Sprintf("log-alert-retry:%d", event.Id), s.evalLockTTL())
}

func (s *Service) releaseEvalItems(items []*evalItem) {
	for _, item := range items {
		if item == nil || item.release == nil {
			continue
		}
		item.release()
		item.release = nil
	}
}

func (s *Service) evalBucketKey(rule *model.OnecLogAlertRule) string {
	if rule == nil {
		return ""
	}
	return fmt.Sprintf("%s|%s", rule.ClusterUuid, strings.ToLower(strings.TrimSpace(rule.BackendType)))
}

func splitEvalBucketKey(value string) (clusterUUID, backendType string) {
	parts := strings.SplitN(value, "|", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return value, ""
}

func (s *Service) findClusterBackendApp(clusterUUID, backendType string) (*model.OnecClusterApp, error) {
	apps, err := s.svcCtx.OnecClusterAppModel.SearchNoPage(s.ctx, "created_at", false, "`cluster_uuid` = ? AND `app_type` = ?", clusterUUID, 2)
	if err != nil || len(apps) == 0 {
		return nil, err
	}
	normalizedBackend := strings.ToLower(strings.TrimSpace(backendType))
	for _, item := range apps {
		if strings.EqualFold(strings.TrimSpace(item.AppCode), normalizedBackend) {
			return item, nil
		}
	}
	for _, item := range apps {
		if item.IsDefault == 1 {
			return item, nil
		}
	}
	return apps[0], nil
}

func (s *Service) buildEvalSearchRequest(rule *model.OnecLogAlertRule, now time.Time) (*logtypes.SearchRequest, time.Time, time.Time, error) {
	windowDuration := s.ruleWindow(rule)
	maxQueryLimit := s.svcCtx.Config.LogAlertEngine.MaxQueryLimit
	if maxQueryLimit <= 0 {
		maxQueryLimit = 500
	}

	windowEnd := now
	windowStart := now.Add(-windowDuration)
	searchReq := &logtypes.SearchRequest{
		ClusterUuid:   rule.ClusterUuid,
		ProjectUuid:   rule.ProjectUuid,
		NamespaceName: rule.Namespace,
		Namespace:     rule.Namespace,
		Application:   rule.Application,
		ResourceName:  rule.ResourceName,
		QueryText:     effectiveRuleQuery(rule),
		QueryMode:     evalQueryMode(rule.BackendType),
		SearchMode:    normalizedSearchMode(rule),
		QueryExpr:     nullStringValue(rule.Expr),
		LogType:       normalizedLogType(rule),
		Start:         windowStart,
		End:           windowEnd,
		Limit:         maxQueryLimit,
		Direction:     "backward",
	}
	if searchReq.QueryMode == logtypes.QueryModeES {
		searchReq.DataStream = resolveESDataStream(normalizedLogType(rule), s.svcCtx.Config.LogSearch.Elasticsearch.DataStream)
		searchReq.IndexPattern = resolveESIndexPattern(normalizedLogType(rule), s.svcCtx.Config.LogSearch.Elasticsearch.IndexPattern)
	}
	if err := searchReq.Normalize(logtypes.QueryDefaults{QueryMode: searchReq.QueryMode}); err != nil {
		return nil, time.Time{}, time.Time{}, err
	}
	return searchReq, windowStart, windowEnd, nil
}

func effectiveRuleQuery(rule *model.OnecLogAlertRule) string {
	if rule == nil {
		return ""
	}
	if strings.EqualFold(strings.TrimSpace(rule.ConditionType), "flatline") {
		return ""
	}
	return rule.QueryText
}

func evalQueryMode(backendType string) string {
	switch strings.ToLower(strings.TrimSpace(backendType)) {
	case "loki":
		return logtypes.QueryModeLoki
	default:
		return logtypes.QueryModeES
	}
}

func isPlatformManagedBackend(backendType string) bool {
	switch strings.ToLower(strings.TrimSpace(backendType)) {
	case "elasticsearch", "es", "loki":
		return true
	default:
		return false
	}
}

func (s *Service) handleEvalResult(rule *model.OnecLogAlertRule, hitCount int64, windowStart, windowEnd time.Time, sample *logtypes.LogRecord) (*model.OnecLogAlertEvalState, error) {
	state, err := s.loadOrInitState(rule)
	if err != nil {
		return nil, err
	}

	runtimeState := decodeRuntimeState(state.StateJson)
	matched := s.matchCondition(rule, hitCount)
	nowMs := windowEnd.UnixMilli()

	if matched {
		state.ConsecutiveHits++
		state.ConsecutiveMiss = 0
		state.LastHitCount = hitCount
		state.CheckpointAt = nowMs
		if !runtimeState.Active && s.reachForDuration(rule, state) {
			if runtimeState.SilenceUntilMs > nowMs {
				state.StateJson = toNullString(mustJSON(runtimeState))
				state.UpdatedBy = systemOperator
				if state.Id == 0 {
					_, err = s.svcCtx.OnecLogAlertEvalStateModel.Insert(s.ctx, state)
				} else {
					state.IsDeleted = 0
					err = s.svcCtx.OnecLogAlertEvalStateModel.Update(s.ctx, state)
				}
				return state, err
			}
			fingerprint := buildEventFingerprint(rule.Id, rule.ClusterUuid, "firing", windowEnd.UnixMilli())
			if err := s.createEventAndNotify(rule, fingerprint, "firing", hitCount, windowStart, windowEnd, sample); err != nil {
				logx.Errorf("[LogAlertEngine] 发送 firing 事件失败: ruleId=%d, err=%v", rule.Id, err)
			} else {
				runtimeState.Active = true
				runtimeState.LastEventFingerprint = fingerprint
				runtimeState.SilenceUntilMs = windowEnd.Add(s.ruleSilenceWindow(rule)).UnixMilli()
			}
		}
	} else {
		if runtimeState.Active {
			resolvedFingerprint := buildEventFingerprint(rule.Id, rule.ClusterUuid, "resolved", windowEnd.UnixMilli())
			if err := s.createEventAndNotify(rule, resolvedFingerprint, "resolved", hitCount, windowStart, windowEnd, sample); err != nil {
				logx.Errorf("[LogAlertEngine] 发送 resolved 事件失败: ruleId=%d, err=%v", rule.Id, err)
			}
		}
		state.ConsecutiveHits = 0
		state.ConsecutiveMiss++
		state.LastHitCount = hitCount
		state.CheckpointAt = nowMs
		runtimeState.Active = false
		runtimeState.LastEventFingerprint = ""
	}

	state.StateJson = toNullString(mustJSON(runtimeState))
	state.UpdatedBy = systemOperator
	if state.Id == 0 {
		_, err = s.svcCtx.OnecLogAlertEvalStateModel.Insert(s.ctx, state)
	} else {
		state.IsDeleted = 0
		err = s.svcCtx.OnecLogAlertEvalStateModel.Update(s.ctx, state)
	}
	return state, err
}

func (s *Service) createEventAndNotify(rule *model.OnecLogAlertRule, fingerprint, status string, hitCount int64, windowStart, windowEnd time.Time, sample *logtypes.LogRecord) error {
	exist, err := s.svcCtx.OnecLogAlertFireEventModel.FindOneByEventFingerprint(s.ctx, fingerprint)
	if err != nil && err != model.ErrNotFound {
		return err
	}
	if exist != nil {
		return nil
	}

	clusterName, projectID, projectName, workspaceID, workspaceName := s.resolveRuleContext(rule)
	_ = projectID
	event := &model.OnecLogAlertFireEvent{
		RuleId:           rule.Id,
		ClusterUuid:      rule.ClusterUuid,
		EventFingerprint: fingerprint,
		WindowStartMs:    windowStart.UnixMilli(),
		WindowEndMs:      windowEnd.UnixMilli(),
		HitCount:         hitCount,
		Severity:         rule.Severity,
		EventStatus:      status,
		NotifyStatus:     "pending",
		NotifyError:      "",
		PayloadJson:      toNullString(mustJSON(buildEventPayload(rule, clusterName, projectName, workspaceID, workspaceName, status, hitCount, windowStart, windowEnd, sample))),
		CreatedBy:        systemOperator,
		UpdatedBy:        systemOperator,
		IsDeleted:        0,
	}

	result, err := s.svcCtx.OnecLogAlertFireEventModel.Insert(s.ctx, event)
	if err != nil {
		return err
	}
	id, _ := result.LastInsertId()
	event.Id = uint64(id)

	notifyErr := s.notify(rule, fingerprint, status, hitCount, windowStart, windowEnd)
	if notifyErr != nil {
		event.NotifyStatus = "failed"
		event.NotifyError = trimErr(notifyErr.Error())
		event.RetryCount = 1
		event.NextRetryAt = sql.NullTime{Time: time.Now().Add(s.retryDelay(1)), Valid: true}
	} else {
		event.NotifyStatus = "success"
		event.NotifyError = ""
		event.RetryCount = 0
		event.NextRetryAt = sql.NullTime{}
	}
	event.UpdatedBy = systemOperator
	return s.svcCtx.OnecLogAlertFireEventModel.Update(s.ctx, event)
}

func (s *Service) retryFailedNotifications() error {
	now := time.Now()
	events, err := s.svcCtx.OnecLogAlertFireEventModel.SearchNoPage(s.ctx, "next_retry_at", true, "`notify_status` = ? AND `next_retry_at` <= ?", "failed", now)
	if err != nil {
		if err == model.ErrNotFound {
			return nil
		}
		return err
	}
	for _, event := range events {
		acquired, release, lockErr := s.tryAcquireRetryLock(event)
		if lockErr != nil {
			logx.Errorf("[LogAlertEngine] 获取补偿锁失败: eventId=%d, err=%v", event.Id, lockErr)
			continue
		}
		if !acquired {
			continue
		}
		if err := s.retryEventNotify(event, now); err != nil {
			logx.Errorf("[LogAlertEngine] 补偿通知失败: eventId=%d, err=%v", event.Id, err)
		}
		release()
	}
	return nil
}

func (s *Service) retryEventNotify(event *model.OnecLogAlertFireEvent, now time.Time) error {
	if event == nil {
		return nil
	}
	rule, err := s.svcCtx.OnecLogAlertRuleModel.FindOne(s.ctx, event.RuleId)
	if err != nil {
		event.NotifyStatus = "dead"
		event.NotifyError = "规则不存在，停止重试"
		event.NextRetryAt = sql.NullTime{}
		event.UpdatedBy = systemOperator
		return s.svcCtx.OnecLogAlertFireEventModel.Update(s.ctx, event)
	}

	payload, err := parseEventPayload(event.PayloadJson)
	if err != nil {
		event.NotifyStatus = "dead"
		event.NotifyError = trimErr(err.Error())
		event.NextRetryAt = sql.NullTime{}
		event.UpdatedBy = systemOperator
		return s.svcCtx.OnecLogAlertFireEventModel.Update(s.ctx, event)
	}

	notifyErr := s.notify(rule, event.EventFingerprint, payload.Status, payload.HitCount, payload.WindowStart, payload.WindowEnd)
	if notifyErr == nil {
		event.NotifyStatus = "success"
		event.NotifyError = ""
		event.NextRetryAt = sql.NullTime{}
		event.UpdatedBy = systemOperator
		return s.svcCtx.OnecLogAlertFireEventModel.Update(s.ctx, event)
	}

	event.RetryCount++
	event.NotifyError = trimErr(notifyErr.Error())
	event.UpdatedBy = systemOperator
	if int(event.RetryCount) >= s.maxNotifyRetry() {
		event.NotifyStatus = "dead"
		event.NextRetryAt = sql.NullTime{}
	} else {
		event.NotifyStatus = "failed"
		event.NextRetryAt = sql.NullTime{Time: now.Add(s.retryDelay(event.RetryCount)), Valid: true}
	}
	return s.svcCtx.OnecLogAlertFireEventModel.Update(s.ctx, event)
}

func (s *Service) notify(rule *model.OnecLogAlertRule, fingerprint, status string, hitCount int64, windowStart, windowEnd time.Time) error {
	clusterName, projectID, projectName, workspaceID, workspaceName := s.resolveRuleContext(rule)
	title := fmt.Sprintf("日志告警[%s] %s", strings.ToUpper(status), rule.Name)

	alertData, err := json.Marshal([]*consumer.AlertInstance{{
		ID:            rule.Id,
		Instance:      rule.ClusterUuid,
		Fingerprint:   fingerprint,
		ClusterUUID:   rule.ClusterUuid,
		ClusterName:   clusterName,
		ProjectID:     projectID,
		ProjectName:   projectName,
		WorkspaceID:   workspaceID,
		WorkspaceName: workspaceName,
		AlertName:     rule.Name,
		Severity:      rule.Severity,
		Status:        status,
		Labels: map[string]string{
			"alert_source": "logging",
			"backend_type": rule.BackendType,
			"cluster_uuid": rule.ClusterUuid,
			"project_uuid": rule.ProjectUuid,
			"namespace":    rule.Namespace,
			"rule_id":      strconv.FormatUint(rule.Id, 10),
		},
		Annotations: map[string]string{
			"description": rule.Description,
			"queryText":   rule.QueryText,
			"hitCount":    strconv.FormatInt(hitCount, 10),
		},
		GeneratorURL: "platform-log-alert-engine",
		StartsAt:     windowStart,
		EndsAt:       nil,
		ResolvedAt:   nil,
		Duration:     uint(windowEnd.Sub(windowStart).Seconds()),
		RepeatCount:  0,
	}})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()

	_, err = s.svcCtx.AlertRpc.AlertNotify(ctx, &portalpb.AlertNotifyReq{
		AlertType: "prometheus",
		AlertData: string(alertData),
		Title:     title,
	})
	return err
}

func (s *Service) resolveRuleContext(rule *model.OnecLogAlertRule) (clusterName string, projectID uint64, projectName string, workspaceID uint64, workspaceName string) {
	if cluster, err := s.svcCtx.OnecClusterModel.FindOneByUuid(s.ctx, rule.ClusterUuid); err == nil {
		clusterName = cluster.Name
	}
	if project, err := s.svcCtx.OnecProjectModel.FindOneByUuid(s.ctx, rule.ProjectUuid); err == nil {
		projectID = project.Id
		projectName = project.Name
	}
	if rule.WorkspaceId > 0 {
		if workspace, err := s.svcCtx.OnecProjectWorkspaceModel.FindOne(s.ctx, rule.WorkspaceId); err == nil {
			workspaceID = workspace.Id
			workspaceName = workspace.Name
		}
	}
	return
}

func (s *Service) loadOrInitState(rule *model.OnecLogAlertRule) (*model.OnecLogAlertEvalState, error) {
	state, err := s.svcCtx.OnecLogAlertEvalStateModel.FindOneByRuleIdClusterUuid(s.ctx, rule.Id, rule.ClusterUuid)
	if err != nil && err != model.ErrNotFound {
		return nil, err
	}
	if state != nil {
		return state, nil
	}
	return &model.OnecLogAlertEvalState{
		RuleId:      rule.Id,
		ClusterUuid: rule.ClusterUuid,
		CreatedBy:   systemOperator,
		UpdatedBy:   systemOperator,
		IsDeleted:   0,
	}, nil
}

func (s *Service) matchCondition(rule *model.OnecLogAlertRule, hitCount int64) bool {
	threshold, err := strconv.ParseFloat(strings.TrimSpace(rule.Threshold), 64)
	if err != nil {
		threshold = 0
	}

	switch strings.ToLower(strings.TrimSpace(rule.ConditionType)) {
	case "count_gt":
		return float64(hitCount) > threshold
	case "count_gte", "match_any":
		return float64(hitCount) >= threshold
	case "count_lt":
		return float64(hitCount) < threshold
	case "count_lte":
		return float64(hitCount) <= threshold
	case "no_data", "flatline":
		return hitCount == 0
	default:
		return hitCount > 0
	}
}

func (s *Service) reachForDuration(rule *model.OnecLogAlertRule, state *model.OnecLogAlertEvalState) bool {
	required := s.ruleForDuration(rule)
	if required <= 0 {
		return true
	}
	interval := s.ruleEvalInterval(rule)
	return time.Duration(state.ConsecutiveHits)*interval >= required
}

func (s *Service) ruleEvalInterval(rule *model.OnecLogAlertRule) time.Duration {
	if rule == nil {
		return s.defaultEvalInterval()
	}
	interval := strings.TrimSpace(rule.EvalInterval)
	if interval == "" {
		return s.defaultEvalInterval()
	}
	parsed, err := time.ParseDuration(interval)
	if err != nil || parsed <= 0 {
		return s.defaultEvalInterval()
	}
	return parsed
}

func (s *Service) minEvalInterval() time.Duration {
	if s.svcCtx.Config.LogAlertEngine.MinEvalInterval <= 0 {
		return 15 * time.Second
	}
	return s.svcCtx.Config.LogAlertEngine.MinEvalInterval
}

func (s *Service) maxEvalInterval() time.Duration {
	if s.svcCtx.Config.LogAlertEngine.MaxEvalInterval <= 0 {
		return 5 * time.Minute
	}
	return s.svcCtx.Config.LogAlertEngine.MaxEvalInterval
}

func (s *Service) silenceWindow() time.Duration {
	if s.svcCtx.Config.LogAlertEngine.SilenceWindow <= 0 {
		return 10 * time.Minute
	}
	return s.svcCtx.Config.LogAlertEngine.SilenceWindow
}

func (s *Service) ruleSilenceWindow(rule *model.OnecLogAlertRule) time.Duration {
	if rule == nil || strings.TrimSpace(rule.SilenceWindow) == "" {
		return s.silenceWindow()
	}
	parsed, err := time.ParseDuration(strings.TrimSpace(rule.SilenceWindow))
	if err != nil || parsed <= 0 {
		return s.silenceWindow()
	}
	return parsed
}

func (s *Service) evalLockTTL() time.Duration {
	if s.svcCtx == nil || s.svcCtx.Config.LogAlertEngine.LockTTL <= 0 {
		return 45 * time.Second
	}
	return s.svcCtx.Config.LogAlertEngine.LockTTL
}

func (s *Service) maxNotifyRetry() int {
	if s.svcCtx.Config.LogAlertEngine.MaxNotifyRetry <= 0 {
		return 5
	}
	return s.svcCtx.Config.LogAlertEngine.MaxNotifyRetry
}

func (s *Service) retryDelay(retryCount int64) time.Duration {
	base := s.svcCtx.Config.LogAlertEngine.NotifyRetryBase
	if base <= 0 {
		base = 30 * time.Second
	}
	delay := base
	for i := int64(1); i < retryCount; i++ {
		delay *= 2
		if delay > 10*time.Minute {
			return 10 * time.Minute
		}
	}
	return delay
}

func (s *Service) ruleWindow(rule *model.OnecLogAlertRule) time.Duration {
	if rule == nil {
		return 5 * time.Minute
	}
	window := strings.TrimSpace(rule.Window)
	if window == "" {
		return 5 * time.Minute
	}
	parsed, err := time.ParseDuration(window)
	if err != nil || parsed <= 0 {
		return 5 * time.Minute
	}
	return parsed
}

func (s *Service) ruleForDuration(rule *model.OnecLogAlertRule) time.Duration {
	if rule == nil {
		return 0
	}
	value := strings.TrimSpace(rule.ForDuration)
	if value == "" {
		return 0
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed < 0 {
		return 0
	}
	return parsed
}

func (s *Service) defaultEvalInterval() time.Duration {
	if s.svcCtx.Config.LogAlertEngine.EvalInterval <= 0 {
		return 60 * time.Second
	}
	return s.svcCtx.Config.LogAlertEngine.EvalInterval
}

func (s *Service) deriveSchedule(rule *model.OnecLogAlertRule, state *model.OnecLogAlertEvalState, latencyMs int64) scheduleDecision {
	baseInterval := s.ruleEvalInterval(rule)
	interval := baseInterval
	active := false
	consecutiveHits := int64(0)
	consecutiveMiss := int64(0)
	lastHitCount := int64(0)
	if state != nil {
		active = decodeRuntimeState(state.StateJson).Active
		consecutiveHits = state.ConsecutiveHits
		consecutiveMiss = state.ConsecutiveMiss
		lastHitCount = state.LastHitCount
	}

	switch {
	case active:
		interval = maxDuration(s.minEvalInterval(), baseInterval/4)
	case consecutiveHits >= 3:
		interval = maxDuration(s.minEvalInterval(), baseInterval/2)
	case consecutiveMiss >= 20:
		interval = minDuration(s.maxEvalInterval(), baseInterval*5)
	case consecutiveMiss >= 10:
		interval = minDuration(s.maxEvalInterval(), baseInterval*3)
	case consecutiveMiss >= 5:
		interval = minDuration(s.maxEvalInterval(), baseInterval*2)
	}

	priority := basePriority(rule)
	if active {
		priority -= 20
	} else if consecutiveHits >= 3 {
		priority -= 10
	}
	priority = clampInt64(priority, 1, 200)

	windowMinutes := maxInt64(1, int64(s.ruleWindow(rule)/time.Minute))
	costScore := windowMinutes*10 + maxInt64(1, latencyMs/20)
	if strings.EqualFold(strings.TrimSpace(rule.SearchMode), logtypes.SearchModeCode) {
		costScore += 30
	}
	if lastHitCount > 1000 {
		costScore += 20
	} else if lastHitCount > 100 {
		costScore += 10
	}
	costScore = clampInt64(costScore, 10, 999)

	return scheduleDecision{
		Interval:  interval,
		Priority:  priority,
		CostScore: costScore,
		Bucket:    classifyBucket(priority),
	}
}

func (s *Service) updateRuleEvalMeta(rule *model.OnecLogAlertRule, status, lastErr string, lastEvalAt sql.NullTime, nextEvalAt time.Time) error {
	rule.LastSyncStatus = status
	rule.LastSyncError = toNullString(lastErr)
	rule.LastEvalAt = lastEvalAt
	rule.NextEvalAt = sql.NullTime{Time: nextEvalAt, Valid: true}
	rule.UpdatedBy = systemOperator
	return s.svcCtx.OnecLogAlertRuleModel.Update(s.ctx, rule)
}

func buildEventPayload(rule *model.OnecLogAlertRule, clusterName, projectName string, workspaceID uint64, workspaceName, status string, hitCount int64, windowStart, windowEnd time.Time, sample *logtypes.LogRecord) map[string]any {
	payload := map[string]any{
		"ruleId":          rule.Id,
		"ruleName":        rule.Name,
		"ruleDescription": rule.Description,
		"backendType":     rule.BackendType,
		"clusterUuid":     rule.ClusterUuid,
		"clusterName":     clusterName,
		"projectUuid":     rule.ProjectUuid,
		"projectName":     projectName,
		"workspaceId":     workspaceID,
		"workspaceName":   workspaceName,
		"namespace":       rule.Namespace,
		"application":     rule.Application,
		"resourceName":    rule.ResourceName,
		"status":          status,
		"hitCount":        hitCount,
		"windowStart":     windowStart.UnixMilli(),
		"windowEnd":       windowEnd.UnixMilli(),
	}
	if sample != nil {
		payload["sample"] = map[string]any{
			"timestamp": sample.Timestamp,
			"message":   sample.Message,
			"raw":       sample.Raw,
			"labels":    sample.Labels,
		}
	}
	return payload
}

func fetchAlertSample(client logtypes.LogClient, req *logtypes.SearchRequest) *logtypes.LogRecord {
	if client == nil || req == nil {
		return nil
	}
	sampleReq := *req
	sampleReq.Limit = 1
	resp, err := client.Search(&sampleReq)
	if err != nil || resp == nil || len(resp.List) == 0 {
		return nil
	}
	return &resp.List[0]
}

func buildEventFingerprint(ruleID uint64, clusterUUID, status string, windowEndMs int64) string {
	raw := fmt.Sprintf("%d:%s:%s:%d", ruleID, clusterUUID, status, windowEndMs)
	sum := sha1.Sum([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func decodeRuntimeState(state sql.NullString) ruleRuntimeState {
	if !state.Valid || strings.TrimSpace(state.String) == "" {
		return ruleRuntimeState{}
	}
	var result ruleRuntimeState
	if err := json.Unmarshal([]byte(state.String), &result); err != nil {
		return ruleRuntimeState{}
	}
	return result
}

func normalizedSearchMode(rule *model.OnecLogAlertRule) string {
	value := strings.TrimSpace(rule.SearchMode)
	if value == "" {
		return logtypes.SearchModeForm
	}
	return value
}

func normalizedLogType(rule *model.OnecLogAlertRule) string {
	value := strings.TrimSpace(rule.LogType)
	if value == "" {
		return "container"
	}
	return value
}

func resolveESDataStream(logType, fallback string) string {
	switch strings.TrimSpace(logType) {
	case "container":
		return esContainerDataStream
	case "system":
		return esSystemDataStream
	default:
		return strings.TrimSpace(fallback)
	}
}

func resolveESIndexPattern(logType, fallback string) string {
	switch strings.TrimSpace(logType) {
	case "container":
		return esContainerIndexStream
	case "system":
		return esSystemIndexStream
	default:
		return strings.TrimSpace(fallback)
	}
}

func nullStringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func toNullString(value string) sql.NullString {
	if strings.TrimSpace(value) == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

func mustJSON(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func trimErr(value string) string {
	if len(value) <= 1000 {
		return value
	}
	return value[:1000]
}

type eventPayload struct {
	Status      string
	HitCount    int64
	WindowStart time.Time
	WindowEnd   time.Time
}

func parseEventPayload(payload sql.NullString) (eventPayload, error) {
	if !payload.Valid || strings.TrimSpace(payload.String) == "" {
		return eventPayload{}, fmt.Errorf("事件载荷为空")
	}
	var raw struct {
		Status      string `json:"status"`
		HitCount    int64  `json:"hitCount"`
		WindowStart int64  `json:"windowStart"`
		WindowEnd   int64  `json:"windowEnd"`
	}
	if err := json.Unmarshal([]byte(payload.String), &raw); err != nil {
		return eventPayload{}, err
	}
	return eventPayload{
		Status:      raw.Status,
		HitCount:    raw.HitCount,
		WindowStart: time.UnixMilli(raw.WindowStart),
		WindowEnd:   time.UnixMilli(raw.WindowEnd),
	}, nil
}

func CollectStats(ctx context.Context, svcCtx *svc.ServiceContext, clusterUUID string) (*Stats, error) {
	engine := &Service{svcCtx: svcCtx, ctx: ctx}
	return engine.collectStats(clusterUUID)
}

func (s *Service) collectStats(clusterUUID string) (*Stats, error) {
	stats := &Stats{ClusterUuid: strings.TrimSpace(clusterUUID)}

	ruleQuery := ""
	taskQuery := ""
	eventQuery := ""
	var ruleArgs []any
	var taskArgs []any
	var eventArgs []any
	if stats.ClusterUuid != "" {
		ruleQuery = "`cluster_uuid` = ?"
		taskQuery = "`cluster_uuid` = ?"
		eventQuery = "`cluster_uuid` = ?"
		ruleArgs = append(ruleArgs, stats.ClusterUuid)
		taskArgs = append(taskArgs, stats.ClusterUuid)
		eventArgs = append(eventArgs, stats.ClusterUuid)
	}

	rules, err := s.svcCtx.OnecLogAlertRuleModel.SearchNoPage(s.ctx, "id", true, ruleQuery, ruleArgs...)
	if err != nil && err != model.ErrNotFound {
		return nil, err
	}
	for _, rule := range rules {
		stats.TotalRules++
		if rule.Enabled == 1 {
			stats.EnabledRules++
		} else {
			stats.DisabledRules++
		}
		switch strings.ToLower(strings.TrimSpace(rule.Severity)) {
		case "critical":
			stats.CriticalRules++
		case "warning":
			stats.WarningRules++
		case "info":
			stats.InfoRules++
		}
	}

	tasks, err := s.svcCtx.OnecLogAlertEvalTaskModel.SearchNoPage(s.ctx, "id", true, taskQuery, taskArgs...)
	if err != nil && err != model.ErrNotFound {
		return nil, err
	}
	for _, task := range tasks {
		switch strings.ToLower(strings.TrimSpace(task.LastEvalStatus)) {
		case "success":
			stats.SuccessRules++
		case "failed":
			stats.FailedRules++
		case "pending", "syncing":
			stats.PendingRules++
		}
		switch classifyBucket(task.Priority) {
		case "hot":
			stats.HotRules++
		case "warm":
			stats.WarmRules++
		default:
			stats.ColdRules++
		}
	}

	events, err := s.svcCtx.OnecLogAlertFireEventModel.SearchNoPage(s.ctx, "id", true, eventQuery, eventArgs...)
	if err != nil && err != model.ErrNotFound {
		return nil, err
	}
	for _, event := range events {
		if strings.EqualFold(strings.TrimSpace(event.EventStatus), "firing") {
			stats.ActiveFiringEvents++
		}
		switch strings.ToLower(strings.TrimSpace(event.NotifyStatus)) {
		case "failed":
			stats.FailedNotifyEvents++
		case "dead":
			stats.DeadNotifyEvents++
		}
	}

	return stats, nil
}

func ManualRetryEvent(ctx context.Context, svcCtx *svc.ServiceContext, eventID uint64, operator string) (*model.OnecLogAlertFireEvent, error) {
	engine := &Service{svcCtx: svcCtx, ctx: ctx}
	return engine.manualRetryEvent(eventID, operator)
}

func (s *Service) manualRetryEvent(eventID uint64, operator string) (*model.OnecLogAlertFireEvent, error) {
	event, err := s.svcCtx.OnecLogAlertFireEventModel.FindOne(s.ctx, eventID)
	if err != nil {
		return nil, err
	}
	if event.IsDeleted == 1 {
		return nil, model.ErrNotFound
	}
	event.UpdatedBy = operator
	event.RetryCount++
	if event.RetryCount < 0 {
		event.RetryCount = 1
	}
	err = s.retryEventNotify(event, time.Now())
	if err != nil {
		return nil, err
	}
	return event, nil
}

func sortEvalItems(items []*evalItem) {
	sort.Slice(items, func(i, j int) bool {
		left := items[i].task
		right := items[j].task
		leftBucket := classifyBucket(left.Priority)
		rightBucket := classifyBucket(right.Priority)
		if leftBucket != rightBucket {
			return bucketRank(leftBucket) < bucketRank(rightBucket)
		}
		if left.Priority != right.Priority {
			return left.Priority < right.Priority
		}
		if left.CostScore != right.CostScore {
			return left.CostScore < right.CostScore
		}
		if !left.NextEvalAt.Equal(right.NextEvalAt) {
			return left.NextEvalAt.Before(right.NextEvalAt)
		}
		return left.Id < right.Id
	})
}

func countBuckets(items []*evalItem) (hot, warm, cold int) {
	for _, item := range items {
		switch classifyBucket(item.task.Priority) {
		case "hot":
			hot++
		case "warm":
			warm++
		default:
			cold++
		}
	}
	return
}

func classifyBucket(priority int64) string {
	switch {
	case priority <= 20:
		return "hot"
	case priority <= 60:
		return "warm"
	default:
		return "cold"
	}
}

func bucketRank(bucket string) int {
	switch bucket {
	case "hot":
		return 0
	case "warm":
		return 1
	default:
		return 2
	}
}

func basePriority(rule *model.OnecLogAlertRule) int64 {
	switch strings.ToLower(strings.TrimSpace(rule.Severity)) {
	case "critical":
		return 10
	case "warning":
		return 40
	case "info":
		return 80
	default:
		return 60
	}
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}

func clampInt64(v, minV, maxV int64) int64 {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
