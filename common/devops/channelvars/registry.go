package channelvars

import (
	"context"
	"fmt"
	"sync"
)

var (
	globalRegistry *Registry
	once           sync.Once
)

// Registry 规格注册表
type Registry struct {
	mu    sync.RWMutex
	specs map[string][]VariableSpec // key: specProfile, value: specs
}

// GetRegistry 获取全局注册表
func GetRegistry() *Registry {
	once.Do(func() {
		globalRegistry = &Registry{
			specs: make(map[string][]VariableSpec),
		}
	})
	return globalRegistry
}

// Register 注册规格
func (r *Registry) Register(spec VariableSpec) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.specs[spec.SpecProfile] == nil {
		r.specs[spec.SpecProfile] = make([]VariableSpec, 0)
	}

	r.specs[spec.SpecProfile] = append(r.specs[spec.SpecProfile], spec)
}

// RegisterBatch 批量注册规格
func (r *Registry) RegisterBatch(specs []VariableSpec) {
	for _, spec := range specs {
		r.Register(spec)
	}
}

// GetSpecsByProfile 根据规格模板获取规格列表
func (r *Registry) GetSpecsByProfile(profile string) []VariableSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()

	specs, ok := r.specs[profile]
	if !ok {
		return nil
	}

	// 返回副本，避免外部修改
	result := make([]VariableSpec, len(specs))
	copy(result, specs)
	return result
}

// GetSpecByKey 根据规格模板和字段key获取规格
func (r *Registry) GetSpecByKey(profile, fieldKey string) *VariableSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()

	specs, ok := r.specs[profile]
	if !ok {
		return nil
	}

	for _, spec := range specs {
		if spec.FieldKey == fieldKey {
			return &spec
		}
	}

	return nil
}

// GetAllProfiles 获取所有规格模板
func (r *Registry) GetAllProfiles() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	profiles := make([]string, 0, len(r.specs))
	for profile := range r.specs {
		profiles = append(profiles, profile)
	}

	return profiles
}

// Clear 清空注册表
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.specs = make(map[string][]VariableSpec)
}

// LoadFromDB 从数据库加载规格
func (r *Registry) LoadFromDB(ctx context.Context, loader SpecLoader) error {
	specs, err := loader.LoadSpecs(ctx)
	if err != nil {
		return fmt.Errorf("load specs from db: %w", err)
	}

	r.Clear()
	r.RegisterBatch(specs)

	return nil
}

// SpecLoader 规格加载器接口
type SpecLoader interface {
	LoadSpecs(ctx context.Context) ([]VariableSpec, error)
}
