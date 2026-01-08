package crontab

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

// Manager 定时任务管理器
type Manager struct {
	scheduler   *DefaultScheduler
	mu          sync.RWMutex
	started     bool
	jobBuilders []JobBuilder
}

// JobBuilder 任务构建器函数类型
type JobBuilder func() Job

// ManagerConfig 管理器配置
type ManagerConfig struct {
	// NodeID 节点标识
	NodeID string

	// Redis 客户端
	Redis *redis.Redis

	// EnableDistributedLock 是否启用分布式锁
	EnableDistributedLock bool

	// Location 时区，默认 Local
	Location *time.Location

	// RecoverPanic 是否恢复 panic，默认 true
	RecoverPanic bool

	// GlobalHooks 全局回调
	// 所有任务执行时都会触发这些回调
	GlobalHooks JobHooks

	// LockTTLMultiplier 锁过期时间倍数
	// 锁的 TTL = 任务超时时间 * LockTTLMultiplier
	// 默认 1.5
	LockTTLMultiplier float64

	// EnableAutoRenew 是否启用锁自动续期
	EnableAutoRenew bool
}

// NewManager 创建定时任务管理器
func NewManager(cfg ManagerConfig) *Manager {
	// 设置默认值
	if cfg.LockTTLMultiplier <= 0 {
		cfg.LockTTLMultiplier = 1.5
	}

	scheduler := NewScheduler(SchedulerConfig{
		NodeID:                cfg.NodeID,
		Redis:                 cfg.Redis,
		EnableDistributedLock: cfg.EnableDistributedLock,
		Location:              cfg.Location,
		RecoverPanic:          cfg.RecoverPanic,
		GlobalHooks:           cfg.GlobalHooks,
		LockTTLMultiplier:     cfg.LockTTLMultiplier,
		EnableAutoRenew:       cfg.EnableAutoRenew,
	})

	return &Manager{
		scheduler:   scheduler,
		jobBuilders: make([]JobBuilder, 0),
	}
}

// RegisterJob 注册任务
// 在 Start 之前调用
func (m *Manager) RegisterJob(job Job) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		// 如果已经启动，直接添加到调度器
		return m.scheduler.AddJob(job)
	}

	// 未启动时，使用构建器模式延迟注册
	m.jobBuilders = append(m.jobBuilders, func() Job { return job })
	return nil
}

// RegisterJobBuilder 注册任务构建器
// 适用于需要依赖注入的场景
func (m *Manager) RegisterJobBuilder(builder JobBuilder) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.jobBuilders = append(m.jobBuilders, builder)
}

// Start 启动管理器
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return fmt.Errorf("manager already started")
	}

	// 注册所有任务
	for _, builder := range m.jobBuilders {
		job := builder()
		if err := m.scheduler.AddJob(job); err != nil {
			logx.Errorf("[Crontab] 注册任务失败: %v", err)
			// 继续注册其他任务，不中断
		}
	}

	// 启动调度器
	if err := m.scheduler.Start(); err != nil {
		return fmt.Errorf("start scheduler failed: %w", err)
	}

	m.started = true
	logx.Infof("[Crontab] 定时任务管理器启动成功, 任务数=%d, nodeID=%s",
		m.scheduler.GetRegistry().Count(), m.scheduler.GetNodeID())

	return nil
}

// Stop 停止管理器
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return nil
	}

	if err := m.scheduler.Stop(); err != nil {
		return fmt.Errorf("stop scheduler failed: %w", err)
	}

	m.started = false
	logx.Info("[Crontab] 定时任务管理器已停止")

	return nil
}

// TriggerJob 手动触发任务
func (m *Manager) TriggerJob(ctx context.Context, name string) error {
	return m.scheduler.TriggerJob(ctx, name)
}

// RemoveJob 移除任务
func (m *Manager) RemoveJob(name string) error {
	return m.scheduler.RemoveJob(name)
}

// GetJobStatus 获取任务状态
func (m *Manager) GetJobStatus(name string) (*JobStatusSummary, bool) {
	status, lastRun, runCount, failCount, exists := m.scheduler.GetJobStatus(name)
	if !exists {
		return nil, false
	}

	return &JobStatusSummary{
		Name:       name,
		LastRun:    lastRun,
		LastStatus: status.String(),
		RunCount:   runCount,
		FailCount:  failCount,
	}, true
}

// GetAllJobStatus 获取所有任务状态
func (m *Manager) GetAllJobStatus() map[string]*JobStatusSummary {
	return m.scheduler.GetAllJobStatus()
}

// ListJobs 列出所有任务名称
func (m *Manager) ListJobs() []string {
	registry := m.scheduler.GetRegistry()
	if r, ok := registry.(*DefaultRegistry); ok {
		return r.Names()
	}
	return nil
}

// GetNodeID 获取节点ID
func (m *Manager) GetNodeID() string {
	return m.scheduler.GetNodeID()
}

// IsRunning 是否正在运行
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.started
}

// GetScheduler 获取底层调度器（高级用法）
func (m *Manager) GetScheduler() *DefaultScheduler {
	return m.scheduler
}

// GetJob 获取任务
func (m *Manager) GetJob(name string) (Job, bool) {
	return m.scheduler.GetRegistry().Get(name)
}

// HasJob 检查任务是否存在
func (m *Manager) HasJob(name string) bool {
	return m.scheduler.GetRegistry().Has(name)
}

// JobCount 获取任务数量
func (m *Manager) JobCount() int {
	return m.scheduler.GetRegistry().Count()
}
