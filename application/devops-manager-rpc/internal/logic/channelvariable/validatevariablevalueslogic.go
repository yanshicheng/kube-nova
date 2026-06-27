package logic

import (
	"context"
	"encoding/json"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/devops/channelvars"

	"github.com/zeromicro/go-zero/core/logx"
)

type ValidateVariableValuesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewValidateVariableValuesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ValidateVariableValuesLogic {
	return &ValidateVariableValuesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ValidateVariableValues 校验变量值
func (l *ValidateVariableValuesLogic) ValidateVariableValues(in *pb.ValidateVariableValuesReq) (*pb.ValidateVariableValuesResp, error) {
	// 1. 加载规格
	dbSpecs, err := l.svcCtx.ChannelVariableSpecModel.FindBySpecProfile(l.ctx, in.SpecProfile)
	if err != nil {
		l.Errorf("查询渠道变量规格失败: %v", err)
		return nil, err
	}

	// 2. 转换为channelvars.VariableSpec
	specs := make([]channelvars.VariableSpec, len(dbSpecs))
	for i, dbSpec := range dbSpecs {
		var deps *channelvars.DependencyRule
		if dbSpec.Dependencies != nil {
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

	// 3. 转换values为map[string]interface{}
	values := make(map[string]interface{})
	for k, v := range in.Values {
		values[k] = v
	}

	// 4. 创建依赖检查器
	checker := channelvars.NewDependencyChecker(specs, values)

	// 5. 校验所有字段
	var errors []string
	for _, spec := range specs {
		// 跳过非必填字段
		if !spec.IsRequired {
			continue
		}

		// 检查依赖
		result := checker.Check(spec.FieldKey)
		if !result.Satisfied {
			continue // 依赖不满足，跳过
		}

		// 检查值是否存在
		val, ok := values[spec.FieldKey]
		if !ok || val == "" {
			errors = append(errors, spec.FieldName+"不能为空")
		}
	}

	return &pb.ValidateVariableValuesResp{
		Valid:  len(errors) == 0,
		Errors: errors,
	}, nil
}
