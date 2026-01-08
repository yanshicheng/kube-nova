package crontab

import (
	"fmt"
	"sync"
)

// Registry 任务注册中心接口
type Registry interface {
	// Register 注册任务
	Register(job Job) error

	// Unregister 取消注册任务
	Unregister(name string) error

	// Get 获取指定任务
	Get(name string) (Job, bool)

	// GetAll 获取所有任务
	GetAll() []Job

	// Count 获取任务数量
	Count() int

	// Has 检查任务是否存在
	Has(name string) bool
}

// DefaultRegistry 默认任务注册中心实现
// 使用读写锁保证并发安全
type DefaultRegistry struct {
	mu   sync.RWMutex
	jobs map[string]Job
}

// NewRegistry 创建任务注册中心
func NewRegistry() *DefaultRegistry {
	return &DefaultRegistry{
		jobs: make(map[string]Job),
	}
}

// Register 注册任务
func (r *DefaultRegistry) Register(job Job) error {
	if job == nil {
		return fmt.Errorf("job cannot be nil")
	}

	name := job.Name()
	if name == "" {
		return fmt.Errorf("job name cannot be empty")
	}

	spec := job.Spec()
	if spec == "" {
		return fmt.Errorf("job spec cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.jobs[name]; exists {
		return fmt.Errorf("job '%s' already registered", name)
	}

	r.jobs[name] = job
	return nil
}

// Unregister 取消注册任务
func (r *DefaultRegistry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.jobs[name]; !exists {
		return fmt.Errorf("job '%s' not found", name)
	}

	delete(r.jobs, name)
	return nil
}

// Get 获取指定任务
func (r *DefaultRegistry) Get(name string) (Job, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	job, exists := r.jobs[name]
	return job, exists
}

// GetAll 获取所有任务
func (r *DefaultRegistry) GetAll() []Job {
	r.mu.RLock()
	defer r.mu.RUnlock()

	jobs := make([]Job, 0, len(r.jobs))
	for _, job := range r.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

// Count 获取任务数量
func (r *DefaultRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.jobs)
}

// Has 检查任务是否存在
func (r *DefaultRegistry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.jobs[name]
	return exists
}

// Clear 清空所有任务
func (r *DefaultRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.jobs = make(map[string]Job)
}

// Names 获取所有任务名称
func (r *DefaultRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.jobs))
	for name := range r.jobs {
		names = append(names, name)
	}
	return names
}

// Update 更新任务（如果存在则替换，不存在则添加）
func (r *DefaultRegistry) Update(job Job) error {
	if job == nil {
		return fmt.Errorf("job cannot be nil")
	}

	name := job.Name()
	if name == "" {
		return fmt.Errorf("job name cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.jobs[name] = job
	return nil
}
