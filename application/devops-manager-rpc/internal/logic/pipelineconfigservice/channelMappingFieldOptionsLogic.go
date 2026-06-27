package pipelineconfigservicelogic

import (
	"context"
	"errors"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/devops/channelvars"

	"github.com/zeromicro/go-zero/core/logx"
)

type ChannelMappingFieldOptionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewChannelMappingFieldOptionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChannelMappingFieldOptionsLogic {
	return &ChannelMappingFieldOptionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ChannelMappingFieldOptionsLogic) ChannelMappingFieldOptions(in *pb.ChannelMappingFieldOptionsReq) (*pb.ChannelMappingFieldOptionsResp, error) {
	groupCode := strings.TrimSpace(in.ChannelGroupCode)
	fields := channelMappingFields(groupCode)
	if item, ok, err := l.findChannelType(in); err != nil {
		l.Errorf("查询渠道类型映射字段失败: %v", err)
		return nil, err
	} else if ok {
		return &pb.ChannelMappingFieldOptionsResp{Data: mappingFieldsForType(item)}, nil
	}
	if groupCode == "" {
		return &pb.ChannelMappingFieldOptionsResp{Data: dedupeMappingFields(fields)}, nil
	}
	types, _, err := l.svcCtx.ChannelTypeModel.List(l.ctx, model.DevopsChannelTypeListFilter{
		GroupCode: groupCode,
		Status:    1,
		Page:      1,
		PageSize:  200,
	})
	if err != nil {
		l.Errorf("查询渠道类型映射字段失败: %v", err)
		return nil, err
	}
	for _, item := range types {
		fields = append(fields, mappingFieldsForType(item)...)
	}
	return &pb.ChannelMappingFieldOptionsResp{Data: dedupeMappingFields(fields)}, nil
}

func (l *ChannelMappingFieldOptionsLogic) findChannelType(in *pb.ChannelMappingFieldOptionsReq) (*model.DevopsChannelType, bool, error) {
	channelTypeID := strings.TrimSpace(in.ChannelTypeId)
	groupCode := strings.TrimSpace(in.ChannelGroupCode)
	if channelTypeID != "" {
		item, err := l.svcCtx.ChannelTypeModel.FindOne(l.ctx, channelTypeID)
		if err != nil {
			if errors.Is(err, model.ErrInvalidObjectID) || errors.Is(err, model.ErrNotFound) {
				return nil, false, nil
			}
			return nil, false, err
		}
		if groupCode != "" && strings.TrimSpace(item.GroupCode) != groupCode {
			return nil, false, nil
		}
		return item, true, nil
	}
	channelType := strings.TrimSpace(in.ChannelType)
	if channelType == "" {
		return nil, false, nil
	}
	item, err := l.svcCtx.ChannelTypeModel.FindOneByCode(l.ctx, channelType)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if groupCode != "" && strings.TrimSpace(item.GroupCode) != groupCode {
		list, _, err := l.svcCtx.ChannelTypeModel.List(l.ctx, model.DevopsChannelTypeListFilter{
			GroupCode: groupCode,
			Status:    1,
			Page:      1,
			PageSize:  200,
		})
		if err != nil {
			return nil, false, err
		}
		for _, candidate := range list {
			if strings.TrimSpace(candidate.Code) == channelType {
				return candidate, true, nil
			}
		}
		return nil, false, nil
	}
	return item, true, nil
}

func mappingFieldsForType(item *model.DevopsChannelType) []*pb.ChannelMappingFieldOption {
	if item == nil {
		return variableFieldsToPb(channelvars.DefaultVariableFields())
	}
	schema := channelvars.ParseVariableSchema(item.MappingFields)
	fields := append(channelvars.StandardVariableFields(item.GroupCode, item.Code), schema.Fields...)
	if strings.TrimSpace(item.GroupCode) == channelvars.GroupCodeRepo {
		fields = filterCodeRepoMappingFields(fields)
	}
	if strings.TrimSpace(item.GroupCode) == channelvars.GroupImageRepo {
		fields = filterImageRegistryMappingFields(fields)
	}
	if strings.TrimSpace(item.GroupCode) == channelvars.GroupCodeScan {
		fields = filterCodeScanMappingFields(fields)
	}
	if strings.TrimSpace(item.GroupCode) == channelvars.GroupArtifactRepo {
		fields = filterArtifactRepositoryMappingFields(fields)
	}
	if strings.TrimSpace(item.GroupCode) == channelvars.GroupDeployTarget {
		fields = filterDeployTargetMappingFields(item.Code, fields)
	}
	return variableFieldsToPb(channelvars.MergeVariableFields(fields))
}

func filterCodeRepoMappingFields(fields []channelvars.VariableField) []channelvars.VariableField {
	allowed := map[string]struct{}{
		channelvars.FieldChannelID:         {},
		channelvars.FieldChannelName:       {},
		channelvars.FieldChannelCode:       {},
		channelvars.FieldEndpoint:          {},
		channelvars.FieldDynamicProject:    {},
		channelvars.FieldDynamicBranch:     {},
		channelvars.FieldDynamicTag:        {},
		channelvars.FieldAddressProjectURL: {},
	}
	return filterMappingFields(fields, allowed)
}

func filterImageRegistryMappingFields(fields []channelvars.VariableField) []channelvars.VariableField {
	allowed := map[string]struct{}{
		channelvars.FieldChannelID:          {},
		channelvars.FieldChannelName:        {},
		channelvars.FieldChannelCode:        {},
		channelvars.FieldEndpoint:           {},
		channelvars.FieldDynamicProject:     {},
		channelvars.FieldDynamicImage:       {},
		channelvars.FieldDynamicTag:         {},
		channelvars.FieldDynamicImageTag:    {},
		channelvars.FieldAddressProjectURL:  {},
		channelvars.FieldAddressImageURL:    {},
		channelvars.FieldAddressImageTagURL: {},
	}
	return filterMappingFields(fields, allowed)
}

func filterCodeScanMappingFields(fields []channelvars.VariableField) []channelvars.VariableField {
	allowed := map[string]struct{}{
		channelvars.FieldChannelID:          {},
		channelvars.FieldChannelName:        {},
		channelvars.FieldChannelCode:        {},
		channelvars.FieldEndpoint:           {},
		channelvars.FieldDynamicProjectName: {},
		channelvars.FieldDynamicProjectKey:  {},
	}
	return filterMappingFields(fields, allowed)
}

func filterArtifactRepositoryMappingFields(fields []channelvars.VariableField) []channelvars.VariableField {
	allowed := map[string]struct{}{
		channelvars.FieldChannelID:                 {},
		channelvars.FieldChannelName:               {},
		channelvars.FieldChannelCode:               {},
		channelvars.FieldEndpoint:                  {},
		channelvars.FieldDynamicRepository:         {},
		channelvars.FieldDynamicArtifactName:       {},
		channelvars.FieldDynamicArtifactVersion:    {},
		channelvars.FieldAddressRepositoryURL:      {},
		channelvars.FieldAddressArtifactURL:        {},
		channelvars.FieldAddressArtifactVersionURL: {},
	}
	return filterMappingFields(fields, allowed)
}

func filterDeployTargetMappingFields(channelType string, fields []channelvars.VariableField) []channelvars.VariableField {
	allowed := map[string]struct{}{
		channelvars.FieldChannelID:   {},
		channelvars.FieldChannelName: {},
		channelvars.FieldChannelCode: {},
		channelvars.FieldEndpoint:    {},
	}
	switch strings.TrimSpace(channelType) {
	case "kubernetes":
		allowed[channelvars.FieldDynamicNamespace] = struct{}{}
		allowed[channelvars.FieldDynamicWorkloadType] = struct{}{}
		allowed[channelvars.FieldDynamicResourceName] = struct{}{}
		allowed[channelvars.FieldDynamicContainerName] = struct{}{}
		allowed[channelvars.FieldDynamicImageName] = struct{}{}
		allowed[channelvars.FieldAddressDeploymentConfig] = struct{}{}
	case "kube-nova":
		allowed[channelvars.FieldAddressDeployConfig] = struct{}{}
	case "host":
		allowed[channelvars.FieldAddressHostConfig] = struct{}{}
	case "host_group":
		allowed[channelvars.FieldAddressGroupConfig] = struct{}{}
	}
	return filterMappingFields(fields, allowed)
}

func filterMappingFields(fields []channelvars.VariableField, allowed map[string]struct{}) []channelvars.VariableField {
	result := make([]channelvars.VariableField, 0, len(fields))
	for _, field := range fields {
		if _, ok := allowed[strings.TrimSpace(field.Field)]; ok {
			result = append(result, field)
		}
	}
	return result
}

func variableFieldsToPb(fields []channelvars.VariableField) []*pb.ChannelMappingFieldOption {
	result := make([]*pb.ChannelMappingFieldOption, 0, len(fields))
	for _, item := range fields {
		if strings.TrimSpace(item.Field) == "" {
			continue
		}
		if channelvars.IsLegacyVariableField(item.Field) {
			continue
		}
		result = append(result, variableFieldToPb(item))
	}
	return result
}

func variableFieldToPb(item channelvars.VariableField) *pb.ChannelMappingFieldOption {
	deps := make([]*pb.ChannelMappingFieldDependency, 0, len(item.Dependencies))
	for _, dep := range item.Dependencies {
		if strings.TrimSpace(dep.Field) == "" {
			continue
		}
		deps = append(deps, &pb.ChannelMappingFieldDependency{
			Field:    dep.Field,
			Source:   dep.Source,
			Name:     dep.Name,
			Required: dep.Required,
		})
	}
	allOf := variableDependenciesToPb(channelvars.DependencyAllOf(item))
	anyOf := variableDependenciesToPb(channelvars.DependencyAnyOf(item))
	options := make([]*pb.StepParamOption, 0, len(item.Options))
	for _, option := range item.Options {
		if strings.TrimSpace(option.Value) == "" {
			continue
		}
		options = append(options, &pb.StepParamOption{
			Label: option.Label,
			Value: option.Value,
		})
	}
	return &pb.ChannelMappingFieldOption{
		Field:                 item.Field,
		Name:                  item.Name,
		Kind:                  item.Kind,
		UiControl:             item.UIControl,
		Provider:              item.Provider,
		Dependencies:          deps,
		AllowManualInput:      item.AllowManualInput,
		AddressBuilder:        item.AddressBuilder,
		Required:              item.Required,
		GroupCode:             item.GroupCode,
		ChannelTypes:          item.ChannelTypes,
		ValueType:             item.ValueType,
		OutputMode:            item.OutputMode,
		OutputTemplate:        item.OutputTemplate,
		CredentialPolicy:      item.CredentialPolicy,
		FallbackAddressFields: item.FallbackAddressFields,
		ResolveEndpoint:       item.ResolveEndpoint,
		AllOf:                 allOf,
		AnyOf:                 anyOf,
		Options:               options,
		SortOrder:             item.SortOrder,
	}
}

func variableDependenciesToPb(items []channelvars.VariableDependency) []*pb.ChannelMappingFieldDependency {
	result := make([]*pb.ChannelMappingFieldDependency, 0, len(items))
	for _, dep := range items {
		if strings.TrimSpace(dep.Field) == "" {
			continue
		}
		result = append(result, &pb.ChannelMappingFieldDependency{
			Field:    dep.Field,
			Source:   dep.Source,
			Name:     dep.Name,
			Required: dep.Required,
		})
	}
	return result
}

func dedupeMappingFields(items []*pb.ChannelMappingFieldOption) []*pb.ChannelMappingFieldOption {
	result := make([]*pb.ChannelMappingFieldOption, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item == nil || strings.TrimSpace(item.Field) == "" {
			continue
		}
		if _, ok := seen[item.Field]; ok {
			continue
		}
		seen[item.Field] = struct{}{}
		result = append(result, item)
	}
	return result
}
