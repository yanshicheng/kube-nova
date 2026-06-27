package inspection

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/watch/incremental"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	promoperator "github.com/yanshicheng/kube-nova/common/prometheusmanager/operator"
	promtypes "github.com/yanshicheng/kube-nova/common/prometheusmanager/types"
	kubeutils "github.com/yanshicheng/kube-nova/common/utils"
	"github.com/zeromicro/go-zero/core/logx"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	systemOperator = "inspection-engine"

	statusSuccess  = "success"
	statusWarning  = "warning"
	statusCritical = "critical"
	statusFailed   = "failed"
	statusPartial  = "partial"
	statusRunning  = "running"

	healthHealthy  = "healthy"
	healthWarning  = "warning"
	healthCritical = "critical"
	healthUnknown  = "unknown"
)

type Service struct {
	svcCtx *svc.ServiceContext
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	locker incremental.DistributedLocker
}

type Runner struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

type runTarget struct {
	cluster *model.OnecCluster
	record  *model.OnecInspectionRecord
}

type clusterRunResult struct {
	record *model.OnecInspectionRecord
	err    error
}

type summaryPayload struct {
	TotalCount    int64            `json:"totalCount"`
	SuccessCount  int64            `json:"successCount"`
	WarningCount  int64            `json:"warningCount"`
	CriticalCount int64            `json:"criticalCount"`
	FailedCount   int64            `json:"failedCount"`
	CategoryStats map[string]int64 `json:"categoryStats"`
}

type reportPayload struct {
	GeneratedAt int64          `json:"generatedAt"`
	RecordNo    string         `json:"recordNo"`
	ClusterUuid string         `json:"clusterUuid"`
	ClusterName string         `json:"clusterName"`
	Score       float64        `json:"score"`
	HealthLevel string         `json:"healthLevel"`
	Summary     summaryPayload `json:"summary"`
}

type resourceScope struct {
	ClusterUuid     string
	ClusterName     string
	ProjectId       uint64
	ProjectUuid     string
	ProjectName     string
	WorkspaceId     uint64
	WorkspaceName   string
	Namespace       string
	ApplicationId   uint64
	ApplicationName string
	ApplicationEn   string
	ResourceType    string
	VersionId       uint64
	Version         string
	ResourceName    string
}

type inspectionItemConfig struct {
	DetailPromQL    string   `json:"detailPromql"`
	TargetLabels    []string `json:"targetLabels"`
	TargetKind      string   `json:"targetKind"`
	DetailMode      string   `json:"detailMode"`
	DetailOperator  string   `json:"detailOperator"`
	DetailThreshold string   `json:"detailThreshold"`
	DetailExpected  string   `json:"detailExpected"`
}

type inspectionTemplateConfig struct {
	InspectionMode  string `json:"inspectionMode"`
	RecordAllResult *bool  `json:"recordAllResults"`
}

type inspectionTaskConfig struct {
	Instances []string `json:"instances"`
}

type inspectionExecutionConfig struct {
	InspectionMode   string
	RecordAllResults bool
	Instances        []string
}

type inspectionTargetCandidate struct {
	Name    string
	Aliases []string
}

type metricCondition struct {
	Operator  string
	Threshold string
	Expected  string
	Source    string
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	var locker incremental.DistributedLocker = incremental.NewNoopLocker("inspection-engine")
	if svcCtx != nil && svcCtx.Cache != nil && svcCtx.Config.InspectionEngine.LockEnabled {
		locker = incremental.NewLockWithAutoRenew(svcCtx.Cache, "inspection-engine")
	}
	return &Service{
		svcCtx: svcCtx,
		ctx:    ctx,
		cancel: cancel,
		locker: locker,
	}
}

func NewRunner(ctx context.Context, svcCtx *svc.ServiceContext) *Runner {
	return &Runner{ctx: ctx, svcCtx: svcCtx}
}

func (s *Service) Start() {
	if s.svcCtx == nil || !s.svcCtx.Config.InspectionEngine.Enabled {
		logx.Info("[InspectionEngine] 巡检引擎未启用，跳过启动")
		<-s.ctx.Done()
		return
	}

	interval := s.svcCtx.Config.InspectionEngine.ScanInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}

	logx.Info("[InspectionEngine] 自动化集群巡检引擎启动")
	s.wg.Add(1)
	go s.scanLoop(interval)
	if err := s.recoverStaleRecords(); err != nil {
		logx.Errorf("[InspectionEngine] 兜底巡检运行中记录失败: %v", err)
	}
	<-s.ctx.Done()
	logx.Info("[InspectionEngine] 收到停止信号")
}

func (s *Service) Stop() {
	s.cancel()
	s.wg.Wait()
	logx.Info("[InspectionEngine] 自动化集群巡检引擎已停止")
}

func (s *Service) scanLoop(interval time.Duration) {
	defer s.wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if err := s.recoverStaleRecords(); err != nil {
				logx.Errorf("[InspectionEngine] 兜底巡检运行中记录失败: %v", err)
			}
			if err := s.runDueTasks(); err != nil {
				logx.Errorf("[InspectionEngine] 扫描巡检任务失败: %v", err)
			}
		}
	}
}

func (s *Service) recoverStaleRecords() error {
	if s.svcCtx == nil {
		return nil
	}
	acquired, release, err := s.locker.TryLock(s.ctx, "cluster-inspection:recover", s.svcCtx.Config.InspectionEngine.LockTTL)
	if err != nil {
		return err
	}
	if !acquired {
		return nil
	}
	defer release()

	records, err := s.svcCtx.OnecInspectionRecordModel.SearchNoPage(s.ctx, "id", true, "`status` = ?", "running")
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil
		}
		return err
	}
	now := time.Now()
	runner := NewRunner(s.ctx, s.svcCtx)
	for _, record := range records {
		if record == nil {
			continue
		}
		timeout := 30 * time.Minute
		task, err := s.svcCtx.OnecInspectionTaskModel.FindOne(s.ctx, record.TaskId)
		if err == nil && task.TimeoutSec > 0 {
			timeout = time.Duration(task.TimeoutSec+60) * time.Second
		}
		baseTime := record.CreatedAt
		if record.StartedAt.Valid {
			baseTime = record.StartedAt.Time
		}
		if baseTime.IsZero() || now.Sub(baseTime) <= timeout {
			continue
		}
		message := "巡检执行超时或执行实例异常退出，系统已自动兜底结束"
		record.ErrorMessage = message
		if err := runner.finishRecord(record, nil, statusFailed, message, systemOperator); err != nil {
			logx.Errorf("[InspectionEngine] 兜底结束巡检记录失败: recordId=%d, err=%v", record.Id, err)
			continue
		}
		appendRecordLog(s.ctx, s.svcCtx, record.Id, "error", message)
		if err == nil && task != nil {
			runner.updateTaskAfterRun(task, record.Id, statusFailed, message)
		}
	}
	return nil
}

func (s *Service) runDueTasks() error {
	acquired, release, err := s.locker.TryLock(s.ctx, "cluster-inspection:scan", s.svcCtx.Config.InspectionEngine.LockTTL)
	if err != nil {
		return err
	}
	if !acquired {
		return nil
	}
	defer release()

	tasks, err := s.svcCtx.OnecInspectionTaskModel.SearchNoPage(
		s.ctx,
		"next_run_at",
		true,
		"`enabled` = 1 AND `schedule_type` = ? AND (`next_run_at` IS NULL OR `next_run_at` <= ?)",
		"cron",
		time.Now(),
	)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil
		}
		return err
	}

	maxBatch := s.svcCtx.Config.InspectionEngine.MaxBatchTasks
	if maxBatch <= 0 {
		maxBatch = 50
	}
	if len(tasks) > maxBatch {
		tasks = tasks[:maxBatch]
	}

	for _, task := range tasks {
		lockKey := fmt.Sprintf("cluster-inspection:task:%d", task.Id)
		acquired, release, err := s.locker.TryLock(s.ctx, lockKey, s.svcCtx.Config.InspectionEngine.LockTTL)
		if err != nil {
			logx.Errorf("[InspectionEngine] 获取任务锁失败: taskId=%d, err=%v", task.Id, err)
			continue
		}
		if !acquired {
			continue
		}
		_, runErr := NewRunner(s.ctx, s.svcCtx).RunTaskAsync(task.Id, "", "schedule", systemOperator, release)
		if runErr != nil {
			release()
			logx.Errorf("[InspectionEngine] 执行定时巡检失败: taskId=%d, err=%v", task.Id, runErr)
		}
	}

	return nil
}

func (r *Runner) RunTask(taskID uint64, overrideClusterUuid, triggerType, operator string) ([]*pb.InspectionRecord, error) {
	if taskID == 0 {
		return nil, fmt.Errorf("巡检任务ID不能为空")
	}
	task, err := r.svcCtx.OnecInspectionTaskModel.FindOne(r.ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("查询巡检任务失败: %w", err)
	}
	if task.IsDeleted == 1 {
		return nil, fmt.Errorf("巡检任务不存在")
	}
	if operator == "" {
		operator = systemOperator
	}
	if triggerType == "" {
		triggerType = "manual"
	}
	if err := r.ensureTemplateEnabled(task.TemplateId); err != nil {
		r.updateTaskAfterRun(task, 0, statusFailed, err.Error())
		return nil, err
	}

	clusters, err := r.resolveClusters(task, strings.TrimSpace(overrideClusterUuid))
	if err != nil {
		r.updateTaskAfterRun(task, 0, statusFailed, err.Error())
		return nil, err
	}
	if len(clusters) == 0 {
		err = fmt.Errorf("未找到可巡检的集群")
		r.updateTaskAfterRun(task, 0, statusFailed, err.Error())
		return nil, err
	}

	records, lastRecordID, lastStatus, lastError := r.runClusters(task, clusters, triggerType, operator)
	r.updateTaskAfterRun(task, lastRecordID, lastStatus, lastError)
	return records, nil
}

func (r *Runner) RunTaskAsync(taskID uint64, overrideClusterUuid, triggerType, operator string, release func()) ([]*pb.InspectionRecord, error) {
	if taskID == 0 {
		return nil, fmt.Errorf("巡检任务ID不能为空")
	}
	task, err := r.svcCtx.OnecInspectionTaskModel.FindOne(r.ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("查询巡检任务失败: %w", err)
	}
	if task.IsDeleted == 1 {
		return nil, fmt.Errorf("巡检任务不存在")
	}
	if operator == "" {
		operator = systemOperator
	}
	if triggerType == "" {
		triggerType = "manual"
	}
	if err := r.ensureTemplateEnabled(task.TemplateId); err != nil {
		r.updateTaskAfterRun(task, 0, statusFailed, err.Error())
		return nil, err
	}

	clusters, err := r.resolveClusters(task, strings.TrimSpace(overrideClusterUuid))
	if err != nil {
		r.updateTaskAfterRun(task, 0, statusFailed, err.Error())
		return nil, err
	}
	if len(clusters) == 0 {
		err = fmt.Errorf("未找到可巡检的集群")
		r.updateTaskAfterRun(task, 0, statusFailed, err.Error())
		return nil, err
	}

	targets := make([]runTarget, 0, len(clusters))
	var records []*pb.InspectionRecord
	var lastRecordID uint64
	lastError := ""
	for _, cluster := range clusters {
		record, err := r.createRecord(task, cluster, triggerType, operator)
		if err != nil {
			lastError = err.Error()
			logx.WithContext(r.ctx).Errorf("创建巡检记录失败: taskId=%d, clusterUuid=%s, err=%v", task.Id, cluster.Uuid, err)
			continue
		}
		lastRecordID = record.Id
		records = append(records, ToPBRecord(record))
		targets = append(targets, runTarget{cluster: cluster, record: record})
		appendRecordLog(r.ctx, r.svcCtx, record.Id, "info", "巡检任务已提交，等待执行")
	}
	if len(targets) == 0 {
		if lastError == "" {
			lastError = "创建巡检记录失败"
		}
		r.updateTaskAfterRun(task, 0, statusFailed, lastError)
		return nil, errors.New(lastError)
	}
	r.updateTaskAfterRun(task, lastRecordID, statusRunning, "")

	go func() {
		if release != nil {
			defer release()
		}
		bgCtx, cancel := context.WithCancel(context.Background())
		if timeout := taskTimeout(task); timeout > 0 {
			bgCtx, cancel = context.WithTimeout(context.Background(), timeout)
		}
		defer cancel()

		bgRunner := NewRunner(bgCtx, r.svcCtx)
		defer func() {
			if recovered := recover(); recovered != nil {
				message := fmt.Sprintf("巡检执行异常退出: %v", recovered)
				logx.Errorf("[InspectionEngine] %s\n%s", message, string(debug.Stack()))
				recordID, status := bgRunner.failRunningTargets(task, targets, message, operator)
				bgRunner.updateTaskAfterRun(task, recordID, status, message)
			}
		}()
		_, recordID, status, runErr := bgRunner.runPreparedTargets(task, targets, operator)
		if errors.Is(bgCtx.Err(), context.DeadlineExceeded) {
			message := "巡检执行超时，系统已自动结束未完成记录"
			timeoutRecordID, timeoutStatus := bgRunner.failRunningTargets(task, targets, message, operator)
			if timeoutRecordID > 0 {
				recordID = timeoutRecordID
			}
			status = timeoutStatus
			if runErr == "" {
				runErr = message
			}
		}
		bgRunner.updateTaskAfterRun(task, recordID, status, runErr)
	}()

	return records, nil
}

func (r *Runner) runClusters(task *model.OnecInspectionTask, clusters []*model.OnecCluster, triggerType, operator string) ([]*pb.InspectionRecord, uint64, string, string) {
	targets := make([]runTarget, 0, len(clusters))
	for _, cluster := range clusters {
		targets = append(targets, runTarget{cluster: cluster})
	}
	return r.runPreparedTargets(task, targets, operator, func(target runTarget) (*model.OnecInspectionRecord, error) {
		return r.runCluster(task, target.cluster, triggerType, operator)
	})
}

func (r *Runner) runPreparedTargets(task *model.OnecInspectionTask, targets []runTarget, operator string, customRun ...func(runTarget) (*model.OnecInspectionRecord, error)) ([]*pb.InspectionRecord, uint64, string, string) {
	records := make([]*pb.InspectionRecord, 0, len(targets))
	lastRecordID := uint64(0)
	lastStatus := statusSuccess
	lastError := ""
	nonSuccessRecords := 0
	failedRecords := 0
	if len(targets) == 0 {
		return records, 0, statusFailed, "未找到可巡检的集群"
	}

	workers := r.clusterWorkers(task, len(targets))
	jobs := make(chan runTarget)
	results := make(chan clusterRunResult, len(targets))
	var wg sync.WaitGroup
	runFunc := func(target runTarget) (*model.OnecInspectionRecord, error) {
		if len(customRun) > 0 && customRun[0] != nil {
			return customRun[0](target)
		}
		return r.executeClusterRecord(task, target.cluster, target.record, operator)
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for target := range jobs {
				func() {
					defer func() {
						if recovered := recover(); recovered != nil {
							message := fmt.Sprintf("集群巡检执行异常: %v", recovered)
							logx.Errorf("[InspectionEngine] %s\n%s", message, string(debug.Stack()))
							record := target.record
							if record != nil {
								_ = r.finishRecord(record, nil, statusFailed, message, operator)
								appendRecordLog(r.ctx, r.svcCtx, record.Id, "error", message)
							}
							results <- clusterRunResult{record: record, err: errors.New(message)}
						}
					}()
					record, err := runFunc(target)
					results <- clusterRunResult{record: record, err: err}
				}()
			}
		}()
	}

	for _, target := range targets {
		jobs <- target
	}
	close(jobs)
	wg.Wait()
	close(results)

	for result := range results {
		if result.err != nil {
			lastStatus = statusPartial
			lastError = result.err.Error()
			logx.WithContext(r.ctx).Errorf("执行集群巡检失败: taskId=%d, err=%v", task.Id, result.err)
		}
		if result.record == nil {
			continue
		}
		lastRecordID = result.record.Id
		switch result.record.Status {
		case statusFailed:
			nonSuccessRecords++
			failedRecords++
		case statusPartial:
			nonSuccessRecords++
		}
		records = append(records, ToPBRecord(result.record))
	}
	if len(records) == 0 && lastError != "" {
		lastStatus = statusFailed
	} else if len(records) > 0 && failedRecords == len(records) {
		lastStatus = statusFailed
	} else if nonSuccessRecords > 0 || lastError != "" {
		lastStatus = statusPartial
	}
	return records, lastRecordID, lastStatus, lastError
}

func (r *Runner) clusterWorkers(task *model.OnecInspectionTask, total int) int {
	workers := 1
	if task != nil && task.MaxConcurrency > 0 {
		workers = int(task.MaxConcurrency)
	} else if r.svcCtx != nil && r.svcCtx.Config.InspectionEngine.ClusterWorkers > 0 {
		workers = r.svcCtx.Config.InspectionEngine.ClusterWorkers
	}
	if workers < 1 {
		workers = 1
	}
	if workers > 20 {
		workers = 20
	}
	if total > 0 && workers > total {
		workers = total
	}
	return workers
}

func (r *Runner) resolveClusters(task *model.OnecInspectionTask, overrideClusterUuid string) ([]*model.OnecCluster, error) {
	clusterUuid := strings.TrimSpace(overrideClusterUuid)
	if clusterUuid == "" && !strings.EqualFold(task.ScopeType, "global") {
		clusterUuid = strings.TrimSpace(task.ClusterUuid)
	}
	if clusterUuid != "" {
		cluster, err := r.svcCtx.OnecClusterModel.FindOneByUuid(r.ctx, clusterUuid)
		if err != nil {
			return nil, fmt.Errorf("查询集群失败: %w", err)
		}
		if cluster.IsDeleted == 1 {
			return nil, fmt.Errorf("集群不存在")
		}
		return []*model.OnecCluster{cluster}, nil
	}
	return r.svcCtx.OnecClusterModel.GetAllClusters(r.ctx)
}

func (r *Runner) ensureTemplateEnabled(templateID uint64) error {
	if templateID == 0 {
		return nil
	}
	template, err := r.svcCtx.OnecInspectionTemplateModel.FindOne(r.ctx, templateID)
	if err != nil {
		return fmt.Errorf("查询巡检规则库失败: %w", err)
	}
	if template.IsDeleted == 1 {
		return fmt.Errorf("巡检规则库不存在")
	}
	if template.Enabled != 1 {
		return fmt.Errorf("巡检规则库已禁用")
	}
	return nil
}

func (r *Runner) runCluster(task *model.OnecInspectionTask, cluster *model.OnecCluster, triggerType, operator string) (record *model.OnecInspectionRecord, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			message := fmt.Sprintf("集群巡检执行异常: %v", recovered)
			logx.Errorf("[InspectionEngine] %s\n%s", message, string(debug.Stack()))
			if record != nil {
				_ = r.finishRecord(record, nil, statusFailed, message, operator)
				appendRecordLog(r.ctx, r.svcCtx, record.Id, "error", message)
			}
			err = errors.New(message)
		}
	}()
	record, err = r.createRecord(task, cluster, triggerType, operator)
	if err != nil {
		return nil, err
	}
	return r.executeClusterRecord(task, cluster, record, operator)
}

func (r *Runner) createRecord(task *model.OnecInspectionTask, cluster *model.OnecCluster, triggerType, operator string) (*model.OnecInspectionRecord, error) {
	start := time.Now()
	record := &model.OnecInspectionRecord{
		RecordNo:    fmt.Sprintf("INSP-%s-%d-%d-%06d", start.Format("20060102150405"), task.Id, cluster.Id, start.Nanosecond()/1000),
		TaskId:      task.Id,
		TemplateId:  task.TemplateId,
		TriggerType: triggerType,
		ScopeType:   task.ScopeType,
		ClusterUuid: cluster.Uuid,
		ClusterName: cluster.Name,
		Status:      statusRunning,
		HealthLevel: healthUnknown,
		StartedAt:   sql.NullTime{Time: start, Valid: true},
		CreatedBy:   operator,
		UpdatedBy:   operator,
	}
	result, err := r.svcCtx.OnecInspectionRecordModel.Insert(r.ctx, record)
	if err != nil {
		return nil, fmt.Errorf("创建巡检记录失败: %w", err)
	}
	recordID, _ := result.LastInsertId()
	record.Id = uint64(recordID)
	return record, nil
}

func (r *Runner) executeClusterRecord(task *model.OnecInspectionTask, cluster *model.OnecCluster, record *model.OnecInspectionRecord, operator string) (finished *model.OnecInspectionRecord, err error) {
	if record == nil {
		return nil, fmt.Errorf("巡检记录为空")
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			message := fmt.Sprintf("巡检记录执行异常: %v", recovered)
			logx.Errorf("[InspectionEngine] %s\n%s", message, string(debug.Stack()))
			_ = r.finishRecord(record, nil, statusFailed, message, operator)
			appendRecordLog(r.ctx, r.svcCtx, record.Id, "error", message)
			finished = record
			err = errors.New(message)
		}
	}()

	appendRecordLog(r.ctx, r.svcCtx, record.Id, "info", fmt.Sprintf("开始巡检集群 %s", cluster.Name))

	items, err := r.loadItems(task.TemplateId)
	if err != nil {
		r.finishRecord(record, nil, statusFailed, err.Error(), operator)
		appendRecordLog(r.ctx, r.svcCtx, record.Id, "error", "加载巡检规则失败: "+err.Error())
		return record, err
	}
	appendRecordLog(r.ctx, r.svcCtx, record.Id, "info", fmt.Sprintf("加载巡检规则 %d 条", len(items)))

	execCfg := r.loadExecutionConfig(task)
	appendRecordLog(r.ctx, r.svcCtx, record.Id, "info", fmt.Sprintf("巡检模式 %s，保存%s结果", execCfg.InspectionMode, recordAllText(execCfg.RecordAllResults)))

	results := make([]*model.OnecInspectionResult, 0, len(items))
	for _, item := range items {
		if current, stopped := r.finishedRecordSnapshot(record.Id); stopped {
			if current != nil {
				*record = *current
				appendRecordLog(r.ctx, r.svcCtx, record.Id, "warning", "巡检记录已结束，停止后续巡检项")
			}
			return record, nil
		}
		if r.ctx.Err() != nil {
			message := "巡检上下文已结束: " + r.ctx.Err().Error()
			if errors.Is(r.ctx.Err(), context.DeadlineExceeded) {
				message = "巡检执行超时，停止执行后续巡检项"
			}
			r.finishRecord(record, results, statusFailed, message, operator)
			appendRecordLog(r.ctx, r.svcCtx, record.Id, "error", message)
			return record, r.ctx.Err()
		}
		appendRecordLog(r.ctx, r.svcCtx, record.Id, "info", "执行巡检项: "+item.ItemName)
		checkRunner, cancel := r.withItemTimeout(item)
		checkResults := checkRunner.runItem(task, cluster, item, operator, execCfg)
		cancel()
		if current, stopped := r.finishedRecordSnapshot(record.Id); stopped {
			if current != nil {
				*record = *current
				appendRecordLog(r.ctx, r.svcCtx, record.Id, "warning", "巡检记录已结束，停止写入后续结果")
			}
			return record, nil
		}
		if len(checkResults) == 0 {
			checkResults = []*model.OnecInspectionResult{failedResult(&model.OnecInspectionResult{
				ItemId:     item.Id,
				ItemCode:   item.ItemCode,
				ItemName:   item.ItemName,
				Category:   item.Category,
				TargetType: item.TargetType,
				TargetName: cluster.Name,
				Severity:   item.Severity,
				CreatedBy:  operator,
				UpdatedBy:  operator,
			}, "巡检项未返回结果")}
		}
		itemStatus := statusSuccess
		for _, checkResult := range checkResults {
			if current, stopped := r.finishedRecordSnapshot(record.Id); stopped {
				if current != nil {
					*record = *current
				}
				return record, nil
			}
			if checkResult == nil {
				continue
			}
			checkResult.RecordId = record.Id
			if execCfg.RecordAllResults || checkResult.Status != statusSuccess {
				if _, err := r.svcCtx.OnecInspectionResultModel.Insert(r.ctx, checkResult); err != nil {
					logx.WithContext(r.ctx).Errorf("写入巡检结果失败: recordId=%d, item=%s, target=%s, err=%v", record.Id, item.ItemCode, checkResult.TargetName, err)
					checkResult.Status = statusFailed
					checkResult.Score = 0
					checkResult.Message = "写入巡检结果失败"
					appendRecordLog(r.ctx, r.svcCtx, record.Id, "error", fmt.Sprintf("写入巡检结果失败: %s / %s", item.ItemName, checkResult.TargetName))
				}
			}
			if statusRank(checkResult.Status) < statusRank(itemStatus) {
				itemStatus = checkResult.Status
			}
			results = append(results, checkResult)
		}
		appendRecordLog(r.ctx, r.svcCtx, record.Id, logLevelByStatus(itemStatus), fmt.Sprintf("巡检项完成: %s，生成明细 %d 条，结果 %s", item.ItemName, len(checkResults), itemStatus))
	}

	if err := r.finishRecord(record, results, "", "", operator); err != nil {
		appendRecordLog(r.ctx, r.svcCtx, record.Id, "error", "更新巡检报告失败: "+err.Error())
		return nil, err
	}
	appendRecordLog(r.ctx, r.svcCtx, record.Id, logLevelByStatus(record.Status), fmt.Sprintf("巡检完成，状态 %s，得分 %.2f", record.Status, record.Score))
	return record, nil
}

func (r *Runner) loadItems(templateID uint64) ([]*model.OnecInspectionItem, error) {
	if templateID == 0 {
		return defaultItems(), nil
	}
	if err := r.ensureTemplateEnabled(templateID); err != nil {
		return nil, err
	}
	items, err := r.svcCtx.OnecInspectionItemModel.SearchNoPage(
		r.ctx,
		"order_num",
		true,
		"`template_id` = ? AND `enabled` = 1",
		templateID,
	)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, fmt.Errorf("巡检规则库没有启用的巡检规则")
		}
		return nil, fmt.Errorf("查询巡检项失败: %w", err)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("巡检规则库没有启用的巡检规则")
	}
	items, err = r.filterItemsByEnabledGroups(templateID, items)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("巡检规则库没有启用的巡检分组或巡检规则")
	}
	return items, nil
}

func (r *Runner) filterItemsByEnabledGroups(templateID uint64, items []*model.OnecInspectionItem) ([]*model.OnecInspectionItem, error) {
	groups, err := r.svcCtx.OnecInspectionGroupModel.SearchNoPage(r.ctx, "order_num", true, "`template_id` = ?", templateID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return items, nil
		}
		return nil, fmt.Errorf("查询巡检分组失败: %w", err)
	}
	if len(groups) == 0 {
		return items, nil
	}
	enabledGroupIDs := make(map[uint64]struct{}, len(groups))
	enabledGroupNames := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		if group == nil || group.IsDeleted == 1 || group.Enabled != 1 {
			continue
		}
		enabledGroupIDs[group.Id] = struct{}{}
		enabledGroupNames[group.GroupName] = struct{}{}
	}
	filtered := make([]*model.OnecInspectionItem, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		if item.GroupId > 0 {
			if _, ok := enabledGroupIDs[item.GroupId]; ok {
				filtered = append(filtered, item)
			}
			continue
		}
		if _, ok := enabledGroupNames[item.Category]; ok {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

func (r *Runner) loadExecutionConfig(task *model.OnecInspectionTask) inspectionExecutionConfig {
	cfg := inspectionExecutionConfig{
		InspectionMode:   "cluster",
		RecordAllResults: true,
	}
	if task == nil {
		return cfg
	}
	if task.TemplateId > 0 {
		template, err := r.svcCtx.OnecInspectionTemplateModel.FindOne(r.ctx, task.TemplateId)
		if err == nil && template != nil && template.ConfigJson.Valid {
			templateCfg := parseInspectionTemplateConfig(template.ConfigJson.String)
			if templateCfg.InspectionMode != "" {
				cfg.InspectionMode = templateCfg.InspectionMode
			}
			if templateCfg.RecordAllResult != nil {
				cfg.RecordAllResults = *templateCfg.RecordAllResult
			}
		}
	}
	if task.ConfigJson.Valid {
		taskCfg := parseInspectionTaskConfig(task.ConfigJson.String)
		cfg.Instances = taskCfg.Instances
	}
	return cfg
}

func parseInspectionTemplateConfig(raw string) inspectionTemplateConfig {
	var cfg inspectionTemplateConfig
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return cfg
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return inspectionTemplateConfig{}
	}
	cfg.InspectionMode = strings.ToLower(strings.TrimSpace(cfg.InspectionMode))
	if cfg.InspectionMode != "instance" {
		cfg.InspectionMode = "cluster"
	}
	return cfg
}

func parseInspectionTaskConfig(raw string) inspectionTaskConfig {
	var cfg inspectionTaskConfig
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return cfg
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return inspectionTaskConfig{}
	}
	cfg.Instances = trimStringList(cfg.Instances)
	return cfg
}

func recordAllText(enabled bool) string {
	if enabled {
		return "全部"
	}
	return "异常"
}

func (r *Runner) runItem(task *model.OnecInspectionTask, cluster *model.OnecCluster, item *model.OnecInspectionItem, operator string, execCfg inspectionExecutionConfig) (out []*model.OnecInspectionResult) {
	if item == nil {
		return nil
	}
	result := &model.OnecInspectionResult{
		ItemId:     item.Id,
		ItemCode:   item.ItemCode,
		ItemName:   item.ItemName,
		Category:   item.Category,
		TargetType: item.TargetType,
		TargetName: cluster.Name,
		Severity:   item.Severity,
		Status:     statusSuccess,
		Score:      100,
		Expected:   item.Threshold,
		Unit:       item.Unit,
		Suggestion: item.Advice,
		CreatedBy:  operator,
		UpdatedBy:  operator,
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			message := fmt.Sprintf("巡检项执行异常: %v", recovered)
			logx.Errorf("[InspectionEngine] %s\n%s", message, string(debug.Stack()))
			out = []*model.OnecInspectionResult{failedResult(result, message)}
		}
	}()

	if err := r.ctx.Err(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return []*model.OnecInspectionResult{failedResult(result, "巡检项执行超时")}
		}
		return []*model.OnecInspectionResult{failedResult(result, "巡检项上下文已结束: "+err.Error())}
	}

	switch strings.ToLower(strings.TrimSpace(item.CheckType)) {
	case "promql":
		return r.runPromQLItem(task, cluster, item, result, execCfg)
	default:
		return r.runBuiltinItem(task, cluster, item, result)
	}
}

func (r *Runner) runBuiltinItem(task *model.OnecInspectionTask, cluster *model.OnecCluster, item *model.OnecInspectionItem, result *model.OnecInspectionResult) []*model.OnecInspectionResult {
	if item.ItemCode == "prometheus_available" {
		prom, cleanup, err := r.prometheusClientForTask(task, cluster)
		if err != nil {
			return []*model.OnecInspectionResult{warningResult(result, fmt.Sprintf("未找到可用 Prometheus: %v", err), "请检查计划级 Prometheus 或集群默认 Prometheus 配置")}
		}
		defer cleanup()
		if err := prom.Ping(); err != nil {
			return []*model.OnecInspectionResult{warningResult(result, "Prometheus 健康检查失败", "请检查 Prometheus 地址、认证和网络连通性")}
		}
		result.Message = "Prometheus 可访问"
		return []*model.OnecInspectionResult{result}
	}

	client, err := r.svcCtx.K8sManager.GetCluster(r.ctx, cluster.Uuid)
	if err != nil {
		return []*model.OnecInspectionResult{failedResult(result, fmt.Sprintf("获取集群客户端失败: %v", err))}
	}

	switch item.ItemCode {
	case "cluster_connectivity":
		version, err := client.GetVersionInfo()
		if err != nil {
			return []*model.OnecInspectionResult{criticalResult(result, "集群 API 不可访问", "请检查集群认证、网络连通性和 apiserver 状态")}
		}
		result.Actual = version.Version
		result.Value = version.Version
		result.Message = "集群 API 访问正常"
		return []*model.OnecInspectionResult{result}
	case "node_ready":
		nodes, err := client.GetKubeClient().CoreV1().Nodes().List(r.ctx, metav1.ListOptions{})
		if err != nil {
			return []*model.OnecInspectionResult{failedResult(result, fmt.Sprintf("查询节点失败: %v", err))}
		}
		return buildNodeReadyResults(cluster, result, nodes.Items)
	case "node_pressure":
		nodes, err := client.GetKubeClient().CoreV1().Nodes().List(r.ctx, metav1.ListOptions{})
		if err != nil {
			return []*model.OnecInspectionResult{failedResult(result, fmt.Sprintf("查询节点压力失败: %v", err))}
		}
		return buildNodePressureResults(cluster, result, nodes.Items)
	case "pod_abnormal":
		pods, err := client.GetKubeClient().CoreV1().Pods(metav1.NamespaceAll).List(r.ctx, metav1.ListOptions{})
		if err != nil {
			return []*model.OnecInspectionResult{failedResult(result, fmt.Sprintf("查询 Pod 失败: %v", err))}
		}
		return r.buildPodAbnormalResults(cluster, result, pods.Items)
	default:
		return []*model.OnecInspectionResult{failedResult(result, "不支持的内置巡检项: "+item.ItemCode)}
	}
}

func (r *Runner) prometheusClientForTask(task *model.OnecInspectionTask, cluster *model.OnecCluster) (promtypes.PrometheusClient, func(), error) {
	if task != nil && task.PrometheusEnabled == 1 {
		endpoint := strings.TrimSpace(task.PrometheusEndpoint)
		if endpoint == "" {
			return nil, func() {}, fmt.Errorf("计划级 Prometheus 地址不能为空")
		}
		authType := ""
		username := ""
		password := ""
		token := ""
		if task.PrometheusAuthEnabled == 1 {
			authType = strings.ToLower(strings.TrimSpace(task.PrometheusAuthType))
			username = strings.TrimSpace(task.PrometheusUsername)
			password = strings.TrimSpace(task.PrometheusPassword)
			token = nullString(task.PrometheusToken)
		}
		caCert := ""
		clientCert := ""
		clientKey := ""
		if task.PrometheusTlsEnabled == 1 {
			caCert = nullString(task.PrometheusCaCert)
			clientCert = nullString(task.PrometheusClientCert)
			clientKey = nullString(task.PrometheusClientKey)
		}
		name := fmt.Sprintf("inspection-task-%d", task.Id)
		if cluster != nil && strings.TrimSpace(cluster.Name) != "" {
			name = fmt.Sprintf("%s-%s", name, cluster.Name)
		}
		client, err := promoperator.NewPrometheusClient(r.ctx, &promtypes.PrometheusConfig{
			UUID:       fmt.Sprintf("inspection-task-%d", task.Id),
			Name:       name,
			Endpoint:   endpoint,
			AuthType:   authType,
			Username:   username,
			Password:   password,
			Token:      token,
			Insecure:   task.PrometheusInsecureSkipVerify == 1,
			CACert:     caCert,
			ClientCert: clientCert,
			ClientKey:  clientKey,
			Timeout:    30,
		})
		if err != nil {
			return nil, func() {}, err
		}
		return client, func() { _ = client.Close() }, nil
	}
	if r.svcCtx.PrometheusManager == nil {
		return nil, func() {}, fmt.Errorf("Prometheus 管理器未初始化")
	}
	prom, err := r.svcCtx.PrometheusManager.Get(cluster.Uuid)
	if err != nil {
		return nil, func() {}, err
	}
	return prom, func() {}, nil
}

func (r *Runner) runPromQLItem(task *model.OnecInspectionTask, cluster *model.OnecCluster, item *model.OnecInspectionItem, result *model.OnecInspectionResult, execCfg inspectionExecutionConfig) []*model.OnecInspectionResult {
	if strings.TrimSpace(item.Promql.String) == "" {
		return []*model.OnecInspectionResult{failedResult(result, "PromQL 不能为空")}
	}
	prom, cleanup, err := r.prometheusClientForTask(task, cluster)
	if err != nil {
		return []*model.OnecInspectionResult{failedResult(result, fmt.Sprintf("获取 Prometheus 失败: %v", err))}
	}
	defer cleanup()
	itemConfig := parseInspectionItemConfig(item.ConfigJson.String)
	query := firstString(itemConfig.DetailPromQL, deriveDetailPromQL(item), item.Promql.String)
	values, err := prom.Query(query, nil)
	if err != nil {
		return []*model.OnecInspectionResult{failedResult(result, fmt.Sprintf("执行 PromQL 失败: %v", err))}
	}
	result.Expected = strings.TrimSpace(item.Operator + " " + item.Threshold)
	expectedTargets := r.expectedTargetsForItem(cluster, item, itemConfig, execCfg)
	if len(values) == 0 {
		if len(expectedTargets) > 0 {
			return buildPromQLMissingTargetResults(cluster, item, itemConfig, result, expectedTargets, query)
		}
		result.Actual = "无数据"
		result.Value = "0"
		return []*model.OnecInspectionResult{warningResult(result, "PromQL 未返回数据", "请检查指标采集、表达式和时间窗口")}
	}
	return buildPromQLResults(cluster, item, itemConfig, result, values, query, expectedTargets)
}

func (r *Runner) expectedTargetsForItem(cluster *model.OnecCluster, item *model.OnecInspectionItem, cfg inspectionItemConfig, execCfg inspectionExecutionConfig) []inspectionTargetCandidate {
	if strings.ToLower(strings.TrimSpace(execCfg.InspectionMode)) != "instance" {
		return nil
	}
	if len(execCfg.Instances) > 0 {
		targets := make([]inspectionTargetCandidate, 0, len(execCfg.Instances))
		for _, instance := range execCfg.Instances {
			targets = append(targets, inspectionTargetCandidate{Name: instance, Aliases: []string{instance}})
		}
		return targets
	}
	targetType := canonicalTargetType(item.TargetType, cfg.TargetKind)
	if targetType != "host" {
		return nil
	}
	client, err := r.svcCtx.K8sManager.GetCluster(r.ctx, cluster.Uuid)
	if err != nil {
		return nil
	}
	nodes, err := client.GetKubeClient().CoreV1().Nodes().List(r.ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	}
	targets := make([]inspectionTargetCandidate, 0, len(nodes.Items))
	for _, node := range nodes.Items {
		if node.Name != "" {
			aliases := []string{node.Name}
			for _, address := range node.Status.Addresses {
				if strings.TrimSpace(address.Address) != "" {
					aliases = append(aliases, address.Address)
				}
			}
			targets = append(targets, inspectionTargetCandidate{Name: node.Name, Aliases: trimStringList(aliases)})
		}
	}
	return targets
}

func parseInspectionItemConfig(raw string) inspectionItemConfig {
	var cfg inspectionItemConfig
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return cfg
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return inspectionItemConfig{}
	}
	cfg.DetailPromQL = strings.TrimSpace(cfg.DetailPromQL)
	cfg.TargetKind = strings.TrimSpace(cfg.TargetKind)
	cfg.DetailMode = strings.TrimSpace(cfg.DetailMode)
	cfg.DetailOperator = strings.TrimSpace(cfg.DetailOperator)
	cfg.DetailThreshold = strings.TrimSpace(cfg.DetailThreshold)
	cfg.DetailExpected = strings.TrimSpace(cfg.DetailExpected)
	cfg.TargetLabels = trimStringList(cfg.TargetLabels)
	return cfg
}

func resolveDetailMetricCondition(item *model.OnecInspectionItem, cfg inspectionItemConfig) metricCondition {
	if cfg.DetailOperator != "" && cfg.DetailThreshold != "" {
		return newMetricCondition(cfg.DetailOperator, cfg.DetailThreshold, cfg.DetailExpected, "config")
	}
	if item != nil {
		if operator, threshold, ok := inferDetailConditionFromPromQL(item.Promql.String, item.Operator, item.Threshold); ok {
			return newMetricCondition(operator, threshold, "", "inferred")
		}
		return newMetricCondition(item.Operator, item.Threshold, "", "rule")
	}
	return metricCondition{}
}

func newMetricCondition(operator, threshold, expected, source string) metricCondition {
	condition := metricCondition{
		Operator:  normalizeMetricOperator(operator),
		Threshold: strings.TrimSpace(threshold),
		Expected:  strings.TrimSpace(expected),
		Source:    strings.TrimSpace(source),
	}
	if condition.Expected == "" {
		condition.Expected = strings.TrimSpace(condition.Operator + " " + condition.Threshold)
	}
	return condition
}

func inferDetailConditionFromPromQL(promql, operator, threshold string) (string, string, bool) {
	if !isZeroAnomalyAggregate(operator, threshold) {
		return "", "", false
	}
	query := stripPromQLVectorFallback(promql)
	inner := stripTopLevelPromQLAggregate(query)
	if strings.TrimSpace(inner) == strings.TrimSpace(query) {
		return "", "", false
	}
	_, innerOperator, innerThreshold, ok := splitNumericComparisonOutsideLabels(inner)
	if !ok {
		return "", "", false
	}
	return inverseMetricOperator(innerOperator), innerThreshold, true
}

func isZeroAnomalyAggregate(operator, threshold string) bool {
	thresholdValue, err := strconv.ParseFloat(strings.TrimSpace(threshold), 64)
	if err != nil {
		return false
	}
	switch normalizeMetricOperator(operator) {
	case "==", "<=":
		return thresholdValue == 0
	case "<":
		return thresholdValue <= 1
	default:
		return false
	}
}

func findNumericComparisonOutsideLabels(expr string) (string, string, bool) {
	_, operator, threshold, ok := splitNumericComparisonOutsideLabels(expr)
	return operator, threshold, ok
}

func splitNumericComparisonOutsideLabels(expr string) (string, string, string, bool) {
	inString := false
	braceDepth := 0
	bracketDepth := 0
	for i := 0; i < len(expr); i++ {
		ch := expr[i]
		if ch == '"' && (i == 0 || expr[i-1] != '\\') {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch ch {
		case '{':
			braceDepth++
			continue
		case '}':
			if braceDepth > 0 {
				braceDepth--
			}
			continue
		case '[':
			bracketDepth++
			continue
		case ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
			continue
		}
		if braceDepth > 0 || bracketDepth > 0 {
			continue
		}
		for _, operator := range []string{"==", "!=", ">=", "<=", ">", "<"} {
			if !strings.HasPrefix(expr[i:], operator) {
				continue
			}
			threshold, ok := readNumericLiteral(expr[i+len(operator):])
			if ok {
				return strings.TrimSpace(expr[:i]), operator, threshold, true
			}
		}
	}
	return "", "", "", false
}

func readNumericLiteral(value string) (string, bool) {
	value = strings.TrimLeft(value, " \t\r\n")
	if strings.HasPrefix(strings.ToLower(value), "bool ") {
		value = strings.TrimLeft(value[4:], " \t\r\n")
	}
	if value == "" {
		return "", false
	}
	end := 0
	for end < len(value) {
		ch := value[end]
		if (ch >= '0' && ch <= '9') || ch == '.' || ch == '-' || ch == '+' {
			end++
			continue
		}
		break
	}
	if end == 0 {
		return "", false
	}
	literal := strings.TrimSpace(value[:end])
	if _, err := strconv.ParseFloat(literal, 64); err != nil {
		return "", false
	}
	return literal, true
}

func inverseMetricOperator(operator string) string {
	switch normalizeMetricOperator(operator) {
	case "==":
		return "!="
	case "!=":
		return "=="
	case ">":
		return "<="
	case ">=":
		return "<"
	case "<":
		return ">="
	case "<=":
		return ">"
	default:
		return operator
	}
}

func buildPromQLResults(cluster *model.OnecCluster, item *model.OnecInspectionItem, cfg inspectionItemConfig, base *model.OnecInspectionResult, values []promtypes.InstantQueryResult, query string, expectedTargets []inspectionTargetCandidate) []*model.OnecInspectionResult {
	condition := resolveDetailMetricCondition(item, cfg)
	targetType := canonicalTargetType(item.TargetType, cfg.TargetKind)
	results := make([]*model.OnecInspectionResult, 0, len(values))
	seenTargets := make([]string, 0, len(values))
	for _, series := range values {
		result := cloneInspectionResult(base)
		result.TargetType = targetType
		targetName, hasTargetLabel := prometheusTargetNameByConfig(series.Metric, firstString(cfg.TargetKind, targetType), cfg.TargetLabels)
		if len(expectedTargets) > 0 {
			expected, ok := matchedInspectionTargetCandidate(targetName, expectedTargets)
			if !hasTargetLabel || !ok {
				continue
			}
			targetName = expected.Name
		}
		if targetName != "" {
			seenTargets = append(seenTargets, targetName)
		}
		result.TargetName = firstString(targetName, base.TargetName)
		result.Expected = condition.Expected
		result.Value = formatMetricValue(series.Value)
		result.Actual = formatSeriesActual(result.Value, item.Unit, result.Expected)
		result.DetailJson = nullableJSON(buildPromQLResultDetail(cluster, item, cfg, condition, series, query, result.TargetName, hasTargetLabel))
		if compareMetric(series.Value, condition.Operator, condition.Threshold) {
			result.Status = statusSuccess
			result.Score = 100
			if hasTargetLabel {
				result.Message = "PromQL 巡检通过"
			} else {
				result.Message = "PromQL 巡检通过，未识别到执行目标标签"
			}
		} else {
			statusResultBySeverity(result, "PromQL 阈值不满足", item.Advice)
			if !hasTargetLabel {
				result.Message = "PromQL 阈值不满足，未识别到执行目标标签"
			}
		}
		results = append(results, result)
	}

	for _, target := range expectedTargets {
		if targetCandidateWasSeen(target, seenTargets) {
			continue
		}
		results = append(results, buildPromQLMissingTargetResult(cluster, item, cfg, base, target.Name, query))
	}
	return results
}

func buildPromQLMissingTargetResults(cluster *model.OnecCluster, item *model.OnecInspectionItem, cfg inspectionItemConfig, base *model.OnecInspectionResult, targets []inspectionTargetCandidate, query string) []*model.OnecInspectionResult {
	results := make([]*model.OnecInspectionResult, 0, len(targets))
	for _, target := range targets {
		results = append(results, buildPromQLMissingTargetResult(cluster, item, cfg, base, target.Name, query))
	}
	return results
}

func buildPromQLMissingTargetResult(cluster *model.OnecCluster, item *model.OnecInspectionItem, cfg inspectionItemConfig, base *model.OnecInspectionResult, targetName, query string) *model.OnecInspectionResult {
	condition := resolveDetailMetricCondition(item, cfg)
	result := cloneInspectionResult(base)
	result.TargetType = canonicalTargetType(item.TargetType, cfg.TargetKind)
	result.TargetName = targetName
	result.Expected = condition.Expected
	result.Value = "0"
	result.Actual = "无数据"
	detail := map[string]string{
		"noData": "true",
	}
	if cluster != nil {
		setDetailValue(detail, "clusterUuid", cluster.Uuid)
		setDetailValue(detail, "clusterName", cluster.Name)
	}
	result.DetailJson = nullableJSON(detail)
	return warningResult(result, "实例未返回指标数据", "请检查该实例 exporter、Prometheus target 或标签配置")
}

func buildPromQLResultDetail(cluster *model.OnecCluster, item *model.OnecInspectionItem, cfg inspectionItemConfig, condition metricCondition, series promtypes.InstantQueryResult, query, targetName string, hasTargetLabel bool) map[string]string {
	row := map[string]string{}
	if cluster != nil {
		setDetailValue(row, "clusterUuid", cluster.Uuid)
		setDetailValue(row, "clusterName", cluster.Name)
	}
	for key, value := range series.Metric {
		setDetailValue(row, normalizePrometheusLabel(key), value)
	}
	targetName = strings.TrimSpace(targetName)
	if targetName != "" {
		itemTargetType := ""
		if item != nil {
			itemTargetType = item.TargetType
		}
		targetType := canonicalTargetType(itemTargetType, cfg.TargetKind)
		if targetType == "host" {
			if rawInstance := strings.TrimSpace(row["instance"]); rawInstance != "" && rawInstance != targetName {
				forceDetailValue(row, "prometheusInstance", rawInstance)
			}
			if strings.TrimSpace(row["instance"]) == targetName {
				delete(row, "instance")
			}
			setDetailValue(row, "node", targetName)
		}
	}
	return row
}

func prometheusTargetNameByConfig(metric map[string]string, targetType string, labels []string) (string, bool) {
	for _, label := range labels {
		if value := firstString(metric[label], metric[normalizePrometheusLabel(label)]); value != "" {
			return value, true
		}
	}
	value := prometheusTargetName(metric, targetType)
	return value, value != ""
}

func canonicalTargetType(targetType, targetKind string) string {
	targetType = strings.ToLower(strings.TrimSpace(targetType))
	targetKind = strings.ToLower(strings.TrimSpace(targetKind))
	switch targetType {
	case "node":
		return "host"
	case "cluster":
		if targetKind != "" && targetKind != "cluster" {
			return "middleware"
		}
		return "cluster"
	case "":
		return "cluster"
	default:
		return targetType
	}
}

func cloneInspectionResult(source *model.OnecInspectionResult) *model.OnecInspectionResult {
	if source == nil {
		return &model.OnecInspectionResult{}
	}
	next := *source
	next.DetailJson = sql.NullString{}
	return &next
}

func trimStringList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func deriveDetailPromQL(item *model.OnecInspectionItem) string {
	if item == nil {
		return ""
	}
	targetType := strings.ToLower(strings.TrimSpace(item.TargetType))
	category := strings.ToLower(strings.TrimSpace(item.Category))
	code := strings.ToLower(strings.TrimSpace(item.ItemCode))
	if targetType == "cluster" && !strings.Contains(category, "etcd") && !strings.HasPrefix(code, "etcd_") {
		return ""
	}
	query := stripPromQLVectorFallback(item.Promql.String)
	if strings.Contains(query, "histogram_quantile") {
		query = ensureHistogramInstanceGroup(query)
		return query
	}
	if detail := deriveCountSelectorPromQL(query); detail != "" {
		return detail
	}
	if inner := stripTopLevelPromQLAggregate(query); strings.TrimSpace(inner) != strings.TrimSpace(query) {
		if left, _, _, ok := splitNumericComparisonOutsideLabels(inner); ok && left != "" {
			return left
		}
		query = inner
	}
	query = stripPromQLAggregateCalls(query)
	if strings.TrimSpace(query) == strings.TrimSpace(stripPromQLVectorFallback(item.Promql.String)) {
		return ""
	}
	return query
}

func deriveCountSelectorPromQL(query string) string {
	query = strings.TrimSpace(query)
	const prefix = "count("
	if !strings.HasPrefix(query, prefix) || !strings.HasSuffix(query, ")") {
		return ""
	}
	inner := strings.TrimSpace(query[len(prefix) : len(query)-1])
	if !strings.Contains(inner, "{") || !strings.Contains(inner, "}") {
		return ""
	}
	return fmt.Sprintf("count by(instance, job, namespace, pod, service, endpoint)(%s)", inner)
}

func stripPromQLVectorFallback(query string) string {
	query = strings.TrimSpace(query)
	lower := strings.ToLower(query)
	idx := strings.LastIndex(lower, " or vector(")
	if idx < 0 {
		return query
	}
	return strings.TrimSpace(query[:idx])
}

func ensureHistogramInstanceGroup(query string) string {
	query = strings.ReplaceAll(query, "by (le)", "by (instance, le)")
	query = strings.ReplaceAll(query, "by(le)", "by(instance, le)")
	return query
}

func stripTopLevelPromQLAggregate(query string) string {
	query = strings.TrimSpace(query)
	for _, fn := range []string{"sum", "max", "min", "count", "avg"} {
		prefix := fn + "("
		if !strings.HasPrefix(query, prefix) || !strings.HasSuffix(query, ")") {
			if inner, ok := stripTopLevelPromQLAggregateWithGrouping(query, fn); ok {
				return inner
			}
			continue
		}
		inner := query[len(prefix) : len(query)-1]
		if isBalancedPromQL(inner) {
			return strings.TrimSpace(inner)
		}
	}
	return query
}

func stripTopLevelPromQLAggregateWithGrouping(query, fn string) (string, bool) {
	prefixes := []string{fn + " by", fn + " without"}
	matched := false
	for _, prefix := range prefixes {
		if strings.HasPrefix(query, prefix) {
			matched = true
			break
		}
	}
	if !matched || !strings.HasSuffix(query, ")") {
		return "", false
	}
	groupOpen := strings.Index(query, "(")
	if groupOpen < 0 {
		return "", false
	}
	groupClose := matchingParenIndex(query, groupOpen)
	if groupClose < 0 {
		return "", false
	}
	rest := strings.TrimSpace(query[groupClose+1:])
	if !strings.HasPrefix(rest, "(") || !strings.HasSuffix(rest, ")") {
		return "", false
	}
	inner := rest[1 : len(rest)-1]
	if !isBalancedPromQL(inner) {
		return "", false
	}
	return strings.TrimSpace(inner), true
}

func matchingParenIndex(value string, open int) int {
	depth := 0
	for i := open; i < len(value); i++ {
		switch value[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func stripPromQLAggregateCalls(query string) string {
	for {
		changed := false
		for _, fn := range []string{"sum", "max", "min", "count", "avg"} {
			next, ok := stripFirstPromQLAggregateCall(query, fn)
			if ok {
				query = next
				changed = true
				break
			}
		}
		if !changed {
			return query
		}
	}
}

func stripFirstPromQLAggregateCall(query, fn string) (string, bool) {
	token := fn + "("
	start := strings.Index(query, token)
	if start < 0 {
		return query, false
	}
	open := start + len(fn)
	depth := 0
	for i := open; i < len(query); i++ {
		switch query[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth != 0 {
				continue
			}
			if strings.HasPrefix(strings.TrimSpace(query[i+1:]), "by") {
				return query, false
			}
			inner := query[open+1 : i]
			return query[:start] + inner + query[i+1:], true
		}
	}
	return query, false
}

func isBalancedPromQL(value string) bool {
	depth := 0
	for _, ch := range value {
		switch ch {
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

func representativeMetricValue(current, next float64, operator string) float64 {
	switch strings.TrimSpace(operator) {
	case "<", "<=", "lt", "lte":
		if next > current {
			return next
		}
	case ">", ">=", "gt", "gte":
		if next < current {
			return next
		}
	case "=", "==", "eq":
		if math.Abs(next) > math.Abs(current) {
			return next
		}
	}
	return current
}

func formatPromQLActual(total, failed int, value float64, unit string) string {
	valueText := formatMetricValue(value)
	unit = strings.TrimSpace(unit)
	if total <= 1 {
		return valueText + unit
	}
	return fmt.Sprintf("返回 %d 条指标，异常 %d 条，代表值 %s%s", total, failed, valueText, unit)
}

func formatMetricValue(value float64) string {
	return strconv.FormatFloat(math.Round(value*10000)/10000, 'f', -1, 64)
}

func formatSeriesActual(value, unit, expected string) string {
	valueText := strings.TrimSpace(value)
	if strings.TrimSpace(unit) != "" {
		valueText = valueText + " " + strings.TrimSpace(unit)
	}
	parts := []string{"指标值 " + valueText}
	if strings.TrimSpace(expected) != "" {
		parts = append(parts, "期望 "+strings.TrimSpace(expected))
	}
	return strings.Join(parts, "，")
}

func buildPromQLDetailRows(cluster *model.OnecCluster, item *model.OnecInspectionItem, values []promtypes.InstantQueryResult) []map[string]string {
	rows := make([]map[string]string, 0, len(values))
	for _, series := range values {
		row := map[string]string{}
		if cluster != nil {
			setDetailValue(row, "clusterUuid", cluster.Uuid)
			setDetailValue(row, "clusterName", cluster.Name)
		}
		for key, value := range series.Metric {
			setDetailValue(row, normalizePrometheusLabel(key), value)
		}
		if item != nil {
			targetName := prometheusTargetName(series.Metric, item.TargetType)
			switch item.TargetType {
			case "node":
				setDetailValue(row, "node", targetName)
			case "pod":
				setDetailValue(row, "podName", targetName)
			case "workload":
				setDetailValue(row, "resourceName", targetName)
			case "cluster", "prometheus", "middleware":
				setDetailValue(row, "instance", targetName)
			default:
				setDetailValue(row, "resourceName", targetName)
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func normalizePrometheusLabel(key string) string {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "__name__":
		return "metricName"
	case "pod_name":
		return "podName"
	case "node_name", "nodename", "kubernetes_node":
		return "nodeName"
	case "host", "hostname":
		return "hostName"
	case "namespace":
		return "namespace"
	default:
		return key
	}
}

func prometheusTargetName(metric map[string]string, targetType string) string {
	if len(metric) == 0 {
		return ""
	}
	switch targetType {
	case "node", "host":
		return firstString(metric["node"], metric["nodeName"], metric["node_name"], metric["nodename"], metric["kubernetes_node"], metric["host"], metric["hostName"], metric["hostname"], metric["instance"])
	case "pod":
		return firstString(metric["pod"], metric["pod_name"], metric["podName"], metric["instance"])
	case "workload":
		return firstString(metric["deployment"], metric["statefulset"], metric["daemonset"], metric["job"], metric["resourceName"], metric["pod"], metric["instance"])
	case "cluster", "prometheus", "middleware", "database", "mysql", "redis", "mongodb", "postgresql", "kafka", "rabbitmq", "zookeeper", "etcd":
		return firstString(metric["instance"], metric["endpoint"], metric["pod"], metric["service"], metric["job"])
	default:
		return firstString(metric["instance"], metric["endpoint"], metric["job"], metric["service"], metric["pod"], metric["node"])
	}
}

func setDetailValue(row map[string]string, key, value string) {
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" || value == "" {
		return
	}
	if row[key] == "" {
		row[key] = value
	}
}

func forceDetailValue(row map[string]string, key, value string) {
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" || value == "" {
		return
	}
	row[key] = value
}

func (r *Runner) finishRecord(record *model.OnecInspectionRecord, results []*model.OnecInspectionResult, forceStatus, forceError, operator string) error {
	if record == nil {
		return fmt.Errorf("巡检记录不能为空")
	}
	ctx := r.ctx
	if ctx == nil || ctx.Err() != nil {
		ctx = context.Background()
	}
	if current, stopped := r.finishedRecordSnapshot(record.Id); stopped {
		if current != nil {
			*record = *current
		} else {
			record.Id = 0
		}
		return nil
	}
	now := time.Now()
	record.FinishedAt = sql.NullTime{Time: now, Valid: true}
	if record.StartedAt.Valid {
		record.DurationMs = now.Sub(record.StartedAt.Time).Milliseconds()
	}
	record.UpdatedBy = operator
	record.ErrorMessage = forceError

	if forceStatus != "" {
		record.Status = forceStatus
		record.HealthLevel = healthUnknown
		record.Score = 0
	} else {
		applyRecordStats(record, results)
	}

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
	if err := r.svcCtx.OnecInspectionRecordModel.Update(ctx, record); err != nil {
		return fmt.Errorf("更新巡检记录失败: %w", err)
	}
	return nil
}

func (r *Runner) finishedRecordSnapshot(recordID uint64) (*model.OnecInspectionRecord, bool) {
	if recordID == 0 {
		return nil, false
	}
	ctx := r.ctx
	if ctx == nil || ctx.Err() != nil {
		ctx = context.Background()
	}
	current, err := r.svcCtx.OnecInspectionRecordModel.FindOne(ctx, recordID)
	if err != nil || current == nil || current.IsDeleted == 1 {
		return nil, true
	}
	return current, current.Status != statusRunning && current.FinishedAt.Valid
}

func (r *Runner) updateTaskAfterRun(task *model.OnecInspectionTask, recordID uint64, status, lastError string) {
	now := time.Now()
	task.LastRunAt = sql.NullTime{Time: now, Valid: true}
	task.LastRecordId = recordID
	task.LastStatus = status
	task.LastError = lastError
	task.UpdatedBy = systemOperator
	if strings.EqualFold(task.ScheduleType, "cron") {
		task.NextRunAt = nextRunTime(task.CronExpr, now)
	}
	ctx := r.ctx
	if ctx == nil || ctx.Err() != nil {
		ctx = context.Background()
	}
	if err := r.svcCtx.OnecInspectionTaskModel.Update(ctx, task); err != nil {
		logx.WithContext(ctx).Errorf("更新巡检任务状态失败: taskId=%d, err=%v", task.Id, err)
	}
}

func (r *Runner) withItemTimeout(item *model.OnecInspectionItem) (*Runner, context.CancelFunc) {
	if item == nil || item.TimeoutSec <= 0 {
		return r, func() {}
	}
	ctx, cancel := context.WithTimeout(r.ctx, time.Duration(item.TimeoutSec)*time.Second)
	return NewRunner(ctx, r.svcCtx), cancel
}

func taskTimeout(task *model.OnecInspectionTask) time.Duration {
	if task == nil || task.TimeoutSec <= 0 {
		return 0
	}
	return time.Duration(task.TimeoutSec) * time.Second
}

func (r *Runner) failRunningTargets(task *model.OnecInspectionTask, targets []runTarget, message, operator string) (uint64, string) {
	lastRecordID := uint64(0)
	failedCount := 0
	nonSuccessCount := 0
	totalCount := 0
	for _, target := range targets {
		record := target.record
		if record == nil {
			continue
		}
		totalCount++
		current, err := r.svcCtx.OnecInspectionRecordModel.FindOne(context.Background(), record.Id)
		if err == nil && current != nil {
			record = current
		}
		lastRecordID = record.Id
		if record.Status != statusRunning {
			switch record.Status {
			case statusFailed:
				failedCount++
				nonSuccessCount++
			case statusPartial:
				nonSuccessCount++
			}
			continue
		}
		if err := r.finishRecord(record, nil, statusFailed, message, operator); err != nil {
			logx.Errorf("[InspectionEngine] 兜底结束巡检记录失败: recordId=%d, err=%v", record.Id, err)
			continue
		}
		failedCount++
		nonSuccessCount++
		appendRecordLog(context.Background(), r.svcCtx, record.Id, "error", message)
	}
	if totalCount == 0 || failedCount == totalCount {
		return lastRecordID, statusFailed
	}
	if nonSuccessCount > 0 {
		return lastRecordID, statusPartial
	}
	return lastRecordID, statusSuccess
}

func defaultItems() []*model.OnecInspectionItem {
	return []*model.OnecInspectionItem{
		{
			ItemCode:   "cluster_connectivity",
			ItemName:   "集群 API 连通性",
			Category:   "集群基础",
			CheckType:  "builtin",
			TargetType: "cluster",
			Severity:   "critical",
			Weight:     30,
			Enabled:    1,
			Threshold:  "API 可访问",
			Advice:     "请检查集群认证、网络连通性和 apiserver 状态",
		},
		{
			ItemCode:   "node_ready",
			ItemName:   "节点 Ready 状态",
			Category:   "节点健康",
			CheckType:  "builtin",
			TargetType: "node",
			Severity:   "critical",
			Weight:     25,
			Enabled:    1,
			Threshold:  "异常节点数 = 0",
			Advice:     "请优先检查节点 kubelet、网络和资源压力",
		},
		{
			ItemCode:   "node_pressure",
			ItemName:   "节点资源压力",
			Category:   "节点健康",
			CheckType:  "builtin",
			TargetType: "node",
			Severity:   "warning",
			Weight:     15,
			Enabled:    1,
			Threshold:  "资源压力节点数 = 0",
			Advice:     "请检查节点磁盘、内存、PID 使用率并清理异常负载",
		},
		{
			ItemCode:   "pod_abnormal",
			ItemName:   "Pod 异常状态",
			Category:   "工作负载",
			CheckType:  "builtin",
			TargetType: "pod",
			Severity:   "warning",
			Weight:     20,
			Enabled:    1,
			Threshold:  "异常 Pod 数 = 0",
			Advice:     "请检查异常 Pod 事件、镜像拉取、调度和探针配置",
		},
		{
			ItemCode:   "prometheus_available",
			ItemName:   "Prometheus 可用性",
			Category:   "监控体系",
			CheckType:  "builtin",
			TargetType: "prometheus",
			Severity:   "info",
			Weight:     10,
			Enabled:    1,
			Threshold:  "Prometheus 可访问",
			Advice:     "请在集群应用配置中设置默认 Prometheus",
		},
	}
}

func buildNodeReadyResults(cluster *model.OnecCluster, base *model.OnecInspectionResult, nodes []corev1.Node) []*model.OnecInspectionResult {
	results := make([]*model.OnecInspectionResult, 0, len(nodes))
	for _, node := range nodes {
		ready, reason, message := nodeConditionStatus(node, corev1.NodeReady)
		result := cloneInspectionResult(base)
		result.TargetType = "host"
		result.TargetName = node.Name
		result.Expected = "Ready=True"
		result.Value = string(ready)
		result.Actual = "Ready=" + string(ready)
		detail := map[string]string{
			"node":    node.Name,
			"status":  string(ready),
			"reason":  reason,
			"message": message,
		}
		if cluster != nil {
			setDetailValue(detail, "clusterUuid", cluster.Uuid)
			setDetailValue(detail, "clusterName", cluster.Name)
		}
		result.DetailJson = nullableJSON(detail)
		if ready == corev1.ConditionTrue {
			result.Status = statusSuccess
			result.Score = 100
			result.Message = "节点 Ready"
		} else {
			criticalResult(result, "节点 NotReady", "请优先检查节点 kubelet、网络和资源压力")
		}
		results = append(results, result)
	}
	if len(results) == 0 {
		result := cloneInspectionResult(base)
		result.Actual = "无节点"
		result.Value = "0"
		return []*model.OnecInspectionResult{warningResult(result, "未查询到节点", "请检查集群状态和访问权限")}
	}
	return results
}

func buildNodePressureResults(cluster *model.OnecCluster, base *model.OnecInspectionResult, nodes []corev1.Node) []*model.OnecInspectionResult {
	results := make([]*model.OnecInspectionResult, 0, len(nodes))
	for _, node := range nodes {
		pressureTypes, reason, message := nodePressureSummary(node)
		result := cloneInspectionResult(base)
		result.TargetType = "host"
		result.TargetName = node.Name
		result.Expected = "无资源压力"
		result.Value = strconv.Itoa(len(pressureTypes))
		result.Actual = firstString(strings.Join(pressureTypes, ","), "无资源压力")
		detail := map[string]string{
			"node":          node.Name,
			"pressureTypes": strings.Join(pressureTypes, ","),
			"reason":        reason,
			"message":       message,
		}
		if cluster != nil {
			setDetailValue(detail, "clusterUuid", cluster.Uuid)
			setDetailValue(detail, "clusterName", cluster.Name)
		}
		result.DetailJson = nullableJSON(detail)
		if len(pressureTypes) == 0 {
			result.Status = statusSuccess
			result.Score = 100
			result.Message = "节点无资源压力"
		} else {
			warningResult(result, "节点存在资源压力", "请检查节点磁盘、内存、PID 使用率并清理异常负载")
		}
		results = append(results, result)
	}
	if len(results) == 0 {
		result := cloneInspectionResult(base)
		result.Actual = "无节点"
		result.Value = "0"
		return []*model.OnecInspectionResult{warningResult(result, "未查询到节点", "请检查集群状态和访问权限")}
	}
	return results
}

func (r *Runner) buildPodAbnormalResults(cluster *model.OnecCluster, base *model.OnecInspectionResult, pods []corev1.Pod) []*model.OnecInspectionResult {
	results := make([]*model.OnecInspectionResult, 0, len(pods))
	nsCache := make(map[string]resourceScope)
	appCache := make(map[string]resourceScope)
	for _, pod := range pods {
		reason := podAbnormalReason(pod)
		scope := r.resolvePodScope(cluster, pod, nsCache, appCache)
		result := cloneInspectionResult(base)
		result.TargetType = "pod"
		result.TargetName = pod.Namespace + "/" + pod.Name
		result.Expected = "Pod Running"
		result.Value = firstString(reason, string(pod.Status.Phase))
		result.Actual = fmt.Sprintf("Phase=%s", pod.Status.Phase)
		detail := map[string]string{
			"clusterUuid":     scope.ClusterUuid,
			"clusterName":     scope.ClusterName,
			"projectId":       formatUint(scope.ProjectId),
			"projectUuid":     scope.ProjectUuid,
			"projectName":     scope.ProjectName,
			"workspaceId":     formatUint(scope.WorkspaceId),
			"workspaceName":   scope.WorkspaceName,
			"namespace":       pod.Namespace,
			"applicationId":   formatUint(scope.ApplicationId),
			"applicationName": scope.ApplicationName,
			"applicationEn":   scope.ApplicationEn,
			"resourceType":    scope.ResourceType,
			"resourceName":    scope.ResourceName,
			"versionId":       formatUint(scope.VersionId),
			"version":         scope.Version,
			"podName":         pod.Name,
			"phase":           string(pod.Status.Phase),
			"reason":          reason,
		}
		result.DetailJson = nullableJSON(detail)
		if reason == "" {
			result.Status = statusSuccess
			result.Score = 100
			result.Message = "Pod 运行正常"
		} else {
			warningResult(result, "Pod 状态异常", "请检查异常 Pod 事件、镜像拉取、调度和探针配置")
		}
		results = append(results, result)
	}
	if len(results) == 0 {
		result := cloneInspectionResult(base)
		result.Actual = "无 Pod"
		result.Value = "0"
		return []*model.OnecInspectionResult{warningResult(result, "未查询到 Pod", "请检查命名空间、权限或业务部署状态")}
	}
	return results
}

func nodeConditionStatus(node corev1.Node, conditionType corev1.NodeConditionType) (corev1.ConditionStatus, string, string) {
	for _, condition := range node.Status.Conditions {
		if condition.Type == conditionType {
			return condition.Status, condition.Reason, condition.Message
		}
	}
	return corev1.ConditionUnknown, "", ""
}

func nodePressureSummary(node corev1.Node) ([]string, string, string) {
	types := make([]string, 0, 3)
	reasons := make([]string, 0, 3)
	messages := make([]string, 0, 3)
	for _, condition := range node.Status.Conditions {
		if condition.Status != corev1.ConditionTrue {
			continue
		}
		if condition.Type == corev1.NodeMemoryPressure || condition.Type == corev1.NodeDiskPressure || condition.Type == corev1.NodePIDPressure {
			types = append(types, string(condition.Type))
			if condition.Reason != "" {
				reasons = append(reasons, condition.Reason)
			}
			if condition.Message != "" {
				messages = append(messages, condition.Message)
			}
		}
	}
	return types, strings.Join(reasons, ","), strings.Join(messages, "; ")
}

func podAbnormalReason(pod corev1.Pod) string {
	if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodUnknown || pod.Status.Phase == corev1.PodPending {
		return string(pod.Status.Phase)
	}
	for _, status := range pod.Status.ContainerStatuses {
		if status.State.Waiting != nil && status.State.Waiting.Reason != "" {
			return status.State.Waiting.Reason
		}
		if status.RestartCount >= 5 {
			return "HighRestartCount"
		}
	}
	return ""
}

func collectNotReadyNodes(nodes []corev1.Node) []map[string]string {
	result := make([]map[string]string, 0)
	for _, node := range nodes {
		ready := corev1.ConditionUnknown
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady {
				ready = condition.Status
				break
			}
		}
		if ready != corev1.ConditionTrue {
			result = append(result, map[string]string{
				"name":   node.Name,
				"status": string(ready),
			})
		}
	}
	return result
}

func collectNodePressures(nodes []corev1.Node) []map[string]string {
	result := make([]map[string]string, 0)
	for _, node := range nodes {
		for _, condition := range node.Status.Conditions {
			if condition.Status != corev1.ConditionTrue {
				continue
			}
			if condition.Type == corev1.NodeMemoryPressure || condition.Type == corev1.NodeDiskPressure || condition.Type == corev1.NodePIDPressure {
				result = append(result, map[string]string{
					"name":    node.Name,
					"type":    string(condition.Type),
					"reason":  condition.Reason,
					"message": condition.Message,
				})
			}
		}
	}
	return result
}

func (r *Runner) collectAbnormalPods(cluster *model.OnecCluster, pods []corev1.Pod) []map[string]string {
	result := make([]map[string]string, 0)
	nsCache := make(map[string]resourceScope)
	appCache := make(map[string]resourceScope)
	for _, pod := range pods {
		reason := ""
		if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodUnknown || pod.Status.Phase == corev1.PodPending {
			reason = string(pod.Status.Phase)
		}
		for _, status := range pod.Status.ContainerStatuses {
			if status.State.Waiting != nil {
				reason = status.State.Waiting.Reason
			}
			if status.RestartCount >= 5 && reason == "" {
				reason = "HighRestartCount"
			}
		}
		if reason == "" {
			continue
		}
		scope := r.resolvePodScope(cluster, pod, nsCache, appCache)
		result = append(result, map[string]string{
			"clusterUuid":     scope.ClusterUuid,
			"clusterName":     scope.ClusterName,
			"projectId":       formatUint(scope.ProjectId),
			"projectUuid":     scope.ProjectUuid,
			"projectName":     scope.ProjectName,
			"workspaceId":     formatUint(scope.WorkspaceId),
			"workspaceName":   scope.WorkspaceName,
			"namespace":       pod.Namespace,
			"applicationId":   formatUint(scope.ApplicationId),
			"applicationName": scope.ApplicationName,
			"applicationEn":   scope.ApplicationEn,
			"resourceType":    scope.ResourceType,
			"versionId":       formatUint(scope.VersionId),
			"version":         scope.Version,
			"resourceName":    scope.ResourceName,
			"podName":         pod.Name,
			"phase":           string(pod.Status.Phase),
			"reason":          reason,
		})
	}
	return result
}

func (r *Runner) resolvePodScope(cluster *model.OnecCluster, pod corev1.Pod, nsCache, appCache map[string]resourceScope) resourceScope {
	scope := resourceScope{
		ClusterUuid: cluster.Uuid,
		ClusterName: cluster.Name,
		Namespace:   pod.Namespace,
	}
	if cached, ok := nsCache[pod.Namespace]; ok {
		scope = cached
	}
	if _, ok := nsCache[pod.Namespace]; !ok {
		scope = r.resolveNamespaceScope(scope)
		nsCache[pod.Namespace] = scope
	}

	applyPodAnnotations(&scope, pod.Annotations)
	resourceType, resourceName := podOwnerResource(pod)
	if scope.ResourceType == "" {
		scope.ResourceType = resourceType
	}
	if scope.ResourceName == "" {
		scope.ResourceName = resourceName
	}
	if scope.WorkspaceId > 0 && scope.ResourceName != "" {
		scope = r.resolveApplicationScope(scope, appCache)
	}
	return scope
}

func (r *Runner) resolveNamespaceScope(scope resourceScope) resourceScope {
	workspaces, err := r.svcCtx.OnecProjectWorkspaceModel.SearchNoPage(
		r.ctx,
		"id",
		true,
		"`cluster_uuid` = ? AND `namespace` = ?",
		scope.ClusterUuid,
		scope.Namespace,
	)
	if err != nil || len(workspaces) == 0 {
		return scope
	}
	workspace := workspaces[0]
	scope.WorkspaceId = workspace.Id
	scope.WorkspaceName = workspace.Name

	projectCluster, err := r.svcCtx.OnecProjectClusterModel.FindOne(r.ctx, workspace.ProjectClusterId)
	if err != nil {
		return scope
	}
	project, err := r.svcCtx.OnecProjectModel.FindOne(r.ctx, projectCluster.ProjectId)
	if err != nil {
		return scope
	}
	scope.ProjectId = project.Id
	scope.ProjectUuid = project.Uuid
	scope.ProjectName = project.Name
	return scope
}

func (r *Runner) resolveApplicationScope(scope resourceScope, appCache map[string]resourceScope) resourceScope {
	cacheKey := fmt.Sprintf("%d:%s:%s", scope.WorkspaceId, strings.ToLower(scope.ResourceType), scope.ResourceName)
	if cached, ok := appCache[cacheKey]; ok {
		scope.ApplicationId = cached.ApplicationId
		scope.ApplicationName = firstString(scope.ApplicationName, cached.ApplicationName)
		scope.ApplicationEn = firstString(scope.ApplicationEn, cached.ApplicationEn)
		scope.ResourceType = firstString(scope.ResourceType, cached.ResourceType)
		scope.VersionId = cached.VersionId
		scope.Version = firstString(scope.Version, cached.Version)
		return scope
	}

	apps, err := r.svcCtx.OnecProjectApplication.SearchNoPage(r.ctx, "id", true, "`workspace_id` = ?", scope.WorkspaceId)
	if err != nil {
		appCache[cacheKey] = resourceScope{}
		return scope
	}
	for _, app := range apps {
		if app == nil {
			continue
		}
		if scope.ApplicationEn != "" && !strings.EqualFold(app.NameEn, scope.ApplicationEn) {
			continue
		}
		if scope.ApplicationName != "" && app.NameCn != scope.ApplicationName && !strings.EqualFold(app.NameEn, scope.ApplicationName) {
			continue
		}
		if scope.ResourceType != "" && !strings.EqualFold(app.ResourceType, scope.ResourceType) {
			continue
		}
		version, err := r.svcCtx.OnecProjectVersion.FindOneByApplicationIdResourceName(r.ctx, app.Id, scope.ResourceName)
		if err != nil {
			continue
		}
		scope.ApplicationId = app.Id
		scope.ApplicationName = firstString(scope.ApplicationName, app.NameCn)
		scope.ApplicationEn = firstString(scope.ApplicationEn, app.NameEn)
		scope.ResourceType = firstString(scope.ResourceType, app.ResourceType)
		scope.VersionId = version.Id
		scope.Version = firstString(scope.Version, version.Version)
		appCache[cacheKey] = scope
		return scope
	}
	appCache[cacheKey] = resourceScope{}
	return scope
}

func applyPodAnnotations(scope *resourceScope, annotations map[string]string) {
	if scope == nil || len(annotations) == 0 {
		return
	}
	scope.ProjectUuid = firstString(scope.ProjectUuid, annotations[kubeutils.AnnotationProjectUuid])
	scope.ProjectName = firstString(scope.ProjectName, annotations[kubeutils.AnnotationProject])
	scope.WorkspaceName = firstString(scope.WorkspaceName, annotations[kubeutils.AnnotationWorkspaceName])
	scope.ApplicationName = firstString(scope.ApplicationName, annotations[kubeutils.AnnotationApplication])
	scope.ApplicationEn = firstString(scope.ApplicationEn, annotations[kubeutils.AnnotationApplicationEn])
	scope.ResourceName = firstString(scope.ResourceName, annotations[kubeutils.AnnotationServiceName])
	scope.Version = firstString(scope.Version, annotations[kubeutils.AnnotationVersion])
}

func podOwnerResource(pod corev1.Pod) (string, string) {
	for _, owner := range pod.OwnerReferences {
		if owner.Controller != nil && !*owner.Controller {
			continue
		}
		switch owner.Kind {
		case "ReplicaSet":
			return "deployment", deploymentNameFromReplicaSet(owner.Name)
		case "StatefulSet":
			return "statefulset", owner.Name
		case "DaemonSet":
			return "daemonset", owner.Name
		case "Job":
			return "job", owner.Name
		}
	}
	return "pod", pod.Name
}

func deploymentNameFromReplicaSet(name string) string {
	idx := strings.LastIndex(name, "-")
	if idx <= 0 {
		return name
	}
	suffix := name[idx+1:]
	if len(suffix) < 5 {
		return name
	}
	return name[:idx]
}

func firstString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func nullString(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return strings.TrimSpace(value.String)
}

func targetInCandidates(target string, targets []inspectionTargetCandidate) bool {
	_, ok := matchedInspectionTargetCandidate(target, targets)
	return ok
}

func matchedInspectionTargetCandidate(target string, targets []inspectionTargetCandidate) (inspectionTargetCandidate, bool) {
	for _, expected := range targets {
		if inspectionTargetCandidateMatches(expected, target) {
			return expected, true
		}
	}
	return inspectionTargetCandidate{}, false
}

func targetCandidateWasSeen(expected inspectionTargetCandidate, seen []string) bool {
	for _, actual := range seen {
		if inspectionTargetCandidateMatches(expected, actual) {
			return true
		}
	}
	return false
}

func inspectionTargetCandidateMatches(expected inspectionTargetCandidate, actual string) bool {
	if inspectionTargetMatches(expected.Name, actual) {
		return true
	}
	for _, alias := range expected.Aliases {
		if inspectionTargetMatches(alias, actual) {
			return true
		}
	}
	return false
}

func inspectionTargetMatches(expected, actual string) bool {
	expected = strings.ToLower(strings.TrimSpace(expected))
	actual = strings.ToLower(strings.TrimSpace(actual))
	if expected == "" || actual == "" {
		return false
	}
	if expected == actual {
		return true
	}
	return stripTargetPort(expected) == actual || stripTargetPort(actual) == expected || stripTargetPort(expected) == stripTargetPort(actual)
}

func stripTargetPort(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.Contains(value, "/") {
		return value
	}
	if idx := strings.LastIndex(value, ":"); idx > 0 && idx < len(value)-1 {
		return value[:idx]
	}
	return value
}

func formatUint(value uint64) string {
	if value == 0 {
		return ""
	}
	return strconv.FormatUint(value, 10)
}

func applyRecordStats(record *model.OnecInspectionRecord, results []*model.OnecInspectionResult) {
	var totalWeight float64
	var weightedScore float64
	for _, result := range results {
		if result == nil {
			continue
		}
		record.TotalCount++
		weight := itemWeight(result)
		totalWeight += weight
		weightedScore += result.Score * weight
		switch result.Status {
		case statusCritical:
			record.CriticalCount++
		case statusWarning:
			record.WarningCount++
		case statusFailed:
			record.FailedCount++
		default:
			record.SuccessCount++
		}
	}
	if totalWeight == 0 {
		record.Score = 0
		record.HealthLevel = healthUnknown
		record.Status = statusFailed
		return
	}
	record.Score = math.Round((weightedScore/totalWeight)*100) / 100
	record.Status = statusSuccess
	if record.FailedCount > 0 {
		record.Status = statusPartial
	}
	switch {
	case record.CriticalCount > 0:
		record.HealthLevel = healthCritical
	case record.WarningCount > 0 || record.FailedCount > 0:
		record.HealthLevel = healthWarning
	default:
		record.HealthLevel = healthHealthy
	}
}

func buildSummary(results []*model.OnecInspectionResult) summaryPayload {
	summary := summaryPayload{CategoryStats: make(map[string]int64)}
	for _, result := range results {
		if result == nil {
			continue
		}
		summary.TotalCount++
		summary.CategoryStats[result.Category]++
		switch result.Status {
		case statusCritical:
			summary.CriticalCount++
		case statusWarning:
			summary.WarningCount++
		case statusFailed:
			summary.FailedCount++
		default:
			summary.SuccessCount++
		}
	}
	return summary
}

func itemWeight(result *model.OnecInspectionResult) float64 {
	switch result.Severity {
	case "critical":
		return 30
	case "warning":
		return 20
	default:
		return 10
	}
}

func statusResultBySeverity(result *model.OnecInspectionResult, message, suggestion string) *model.OnecInspectionResult {
	switch strings.ToLower(strings.TrimSpace(result.Severity)) {
	case "critical":
		return criticalResult(result, message, suggestion)
	case "warning":
		return warningResult(result, message, suggestion)
	default:
		result.Status = statusWarning
		result.Score = 80
		result.Message = message
		if suggestion != "" {
			result.Suggestion = suggestion
		}
		return result
	}
}

func criticalResult(result *model.OnecInspectionResult, message, suggestion string) *model.OnecInspectionResult {
	result.Status = statusCritical
	result.Score = 0
	result.Message = message
	if suggestion != "" {
		result.Suggestion = suggestion
	}
	return result
}

func warningResult(result *model.OnecInspectionResult, message, suggestion string) *model.OnecInspectionResult {
	result.Status = statusWarning
	result.Score = 60
	result.Message = message
	if suggestion != "" {
		result.Suggestion = suggestion
	}
	return result
}

func failedResult(result *model.OnecInspectionResult, message string) *model.OnecInspectionResult {
	result.Status = statusFailed
	result.Score = 0
	result.Message = message
	return result
}

func compareMetric(value float64, operator, threshold string) bool {
	thresholdValue, err := strconv.ParseFloat(strings.TrimSpace(threshold), 64)
	if err != nil {
		return false
	}
	switch normalizeMetricOperator(operator) {
	case ">", "gt":
		return value > thresholdValue
	case ">=", "gte":
		return value >= thresholdValue
	case "<", "lt":
		return value < thresholdValue
	case "<=", "lte":
		return value <= thresholdValue
	case "=", "==", "eq":
		return value == thresholdValue
	case "!=", "ne":
		return value != thresholdValue
	default:
		return false
	}
}

func normalizeMetricOperator(operator string) string {
	switch strings.ToLower(strings.TrimSpace(operator)) {
	case "=", "==", "eq":
		return "=="
	case "!=", "ne":
		return "!="
	case ">", "gt":
		return ">"
	case ">=", "gte":
		return ">="
	case "<", "lt":
		return "<"
	case "<=", "lte":
		return "<="
	default:
		return strings.TrimSpace(operator)
	}
}

func nullableJSON(v any) sql.NullString {
	b, err := json.Marshal(v)
	if err != nil {
		return sql.NullString{}
	}
	return sql.NullString{String: string(b), Valid: true}
}

func nextRunTime(expr string, base time.Time) sql.NullTime {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return sql.NullTime{}
	}
	parser := cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	schedule, err := parser.Parse(expr)
	if err != nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: schedule.Next(base), Valid: true}
}

func ToPBRecord(item *model.OnecInspectionRecord) *pb.InspectionRecord {
	if item == nil {
		return nil
	}
	return &pb.InspectionRecord{
		Id:            item.Id,
		RecordNo:      item.RecordNo,
		TaskId:        item.TaskId,
		TemplateId:    item.TemplateId,
		TriggerType:   item.TriggerType,
		ScopeType:     item.ScopeType,
		ClusterUuid:   item.ClusterUuid,
		ClusterName:   item.ClusterName,
		Status:        item.Status,
		Score:         item.Score,
		HealthLevel:   item.HealthLevel,
		TotalCount:    item.TotalCount,
		SuccessCount:  item.SuccessCount,
		WarningCount:  item.WarningCount,
		CriticalCount: item.CriticalCount,
		FailedCount:   item.FailedCount,
		SummaryJson:   item.SummaryJson.String,
		ReportJson:    item.ReportJson.String,
		StartedAt:     timeToUnix(item.StartedAt),
		FinishedAt:    timeToUnix(item.FinishedAt),
		DurationMs:    item.DurationMs,
		ErrorMessage:  item.ErrorMessage,
		CreatedBy:     item.CreatedBy,
		UpdatedBy:     item.UpdatedBy,
		CreatedAt:     item.CreatedAt.Unix(),
		UpdatedAt:     item.UpdatedAt.Unix(),
	}
}

func ToPBResult(item *model.OnecInspectionResult) *pb.InspectionResult {
	if item == nil {
		return nil
	}
	return &pb.InspectionResult{
		Id:         item.Id,
		RecordId:   item.RecordId,
		ItemId:     item.ItemId,
		ItemCode:   item.ItemCode,
		ItemName:   item.ItemName,
		Category:   item.Category,
		TargetType: item.TargetType,
		TargetName: item.TargetName,
		Severity:   item.Severity,
		Status:     item.Status,
		Score:      item.Score,
		Expected:   item.Expected,
		Actual:     item.Actual,
		Value:      item.Value,
		Unit:       item.Unit,
		Message:    item.Message,
		Suggestion: item.Suggestion,
		DetailJson: item.DetailJson.String,
		CreatedBy:  item.CreatedBy,
		UpdatedBy:  item.UpdatedBy,
		CreatedAt:  item.CreatedAt.Unix(),
		UpdatedAt:  item.UpdatedAt.Unix(),
	}
}

func timeToUnix(t sql.NullTime) int64 {
	if !t.Valid {
		return 0
	}
	return t.Time.Unix()
}
