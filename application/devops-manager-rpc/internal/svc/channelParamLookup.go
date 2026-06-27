package svc

import (
	"context"
	"sync"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
	"github.com/zeromicro/go-zero/core/logx"
)

// stepChannelParamLookup 实现 channelvars.StepChannelParamLookup，使用内存缓存查询步骤参数到渠道分组的映射。
type stepChannelParamLookup struct {
	model  *model.DevopsStepChannelParamModel
	mu     sync.RWMutex
	specs  map[string]channelvars.StepChannelParamSpec
	loaded bool
}

func newStepChannelParamLookup(m *model.DevopsStepChannelParamModel) *stepChannelParamLookup {
	return &stepChannelParamLookup{
		model: m,
	}
}

// Load 从数据库加载所有启用的映射到内存缓存，并注册到 channelvars。
func (l *stepChannelParamLookup) Load(ctx context.Context) error {
	dbSpecs, err := l.model.LoadActiveSpecs(ctx)
	if err != nil {
		return err
	}
	specs := make(map[string]channelvars.StepChannelParamSpec, len(dbSpecs))
	paramTypes := make([]channelvars.StepChannelParamSpec, 0, len(dbSpecs))
	for _, spec := range dbSpecs {
		if _, ok := specs[spec.ParamType]; !ok {
			specs[spec.ParamType] = channelvars.StepChannelParamSpec{
				ParamType:         spec.ParamType,
				GroupCode:         spec.GroupCode,
				ChannelTypeFilter: spec.ChannelTypeFilter,
			}
		}
		paramTypes = append(paramTypes, channelvars.StepChannelParamSpec{
			ParamType:         spec.ParamType,
			GroupCode:         spec.GroupCode,
			ChannelTypeFilter: spec.ChannelTypeFilter,
		})
	}

	l.mu.Lock()
	l.specs = specs
	l.loaded = true
	l.mu.Unlock()

	channelvars.SetStepChannelParamLookup(l)
	channelvars.SetChannelParamTypes(paramTypes)
	logx.Infof("步骤参数映射加载完成，共 %d 条", len(specs))
	return nil
}

// Refresh 重新加载缓存。
func (l *stepChannelParamLookup) Refresh(ctx context.Context) error {
	return l.Load(ctx)
}

// LookupGroupCode 实现 channelvars.StepChannelParamLookup 接口。
func (l *stepChannelParamLookup) LookupGroupCode(paramType string) (groupCode string, channelTypeFilter string, found bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	spec, ok := l.specs[paramType]
	if !ok {
		return "", "", false
	}
	return spec.GroupCode, spec.ChannelTypeFilter, true
}
