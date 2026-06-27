package adapter

import (
	"context"
	"encoding/json"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
)

// SpecLoaderAdapter 规格加载器适配器
type SpecLoaderAdapter struct {
	specModel model.ChannelVariableSpecModel
}

// NewSpecLoaderAdapter 创建规格加载器适配器
func NewSpecLoaderAdapter(specModel model.ChannelVariableSpecModel) *SpecLoaderAdapter {
	return &SpecLoaderAdapter{
		specModel: specModel,
	}
}

// LoadSpecs 从数据库加载规格
func (a *SpecLoaderAdapter) LoadSpecs(ctx context.Context) ([]channelvars.VariableSpec, error) {
	dbSpecs, err := a.specModel.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	specs := make([]channelvars.VariableSpec, len(dbSpecs))
	for i, dbSpec := range dbSpecs {
		var deps *channelvars.DependencyRule
		if dbSpec.Dependencies != nil {
			// MongoDB中dependencies存储为interface{}，需要转换
			depsBytes, _ := json.Marshal(dbSpec.Dependencies)
			deps, _ = channelvars.ParseDependencies(string(depsBytes))
		}

		specs[i] = channelvars.VariableSpec{
			ID:               dbSpec.ID.Hex(),
			SpecProfile:      dbSpec.SpecProfile,
			FieldKey:         dbSpec.FieldKey,
			FieldName:        dbSpec.FieldName,
			FieldKind:        dbSpec.FieldKind,
			ProviderKey:      dbSpec.ProviderKey,
			UIControl:        dbSpec.UIControl,
			Dependencies:     deps,
			AllowManualInput: dbSpec.AllowManualInput,
			IsRequired:       dbSpec.IsRequired,
			DefaultValue:     dbSpec.DefaultValue,
			Placeholder:      dbSpec.Placeholder,
			HelpText:         dbSpec.HelpText,
			SortOrder:        int(dbSpec.SortOrder),
		}
	}

	return specs, nil
}
