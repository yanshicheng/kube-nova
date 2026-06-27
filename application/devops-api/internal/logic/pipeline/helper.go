package pipeline

import (
	"context"
	"strconv"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/client/pipelineconfigservice"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/client/executionservice"
)

func currentUsername(ctx context.Context) string {
	username, ok := ctx.Value("username").(string)
	if !ok || username == "" {
		return "system"
	}
	return username
}

func currentUserID(ctx context.Context) uint64 {
	value := ctx.Value("userId")
	switch v := value.(type) {
	case uint64:
		return v
	case uint:
		return uint64(v)
	case int:
		if v > 0 {
			return uint64(v)
		}
	case int64:
		if v > 0 {
			return uint64(v)
		}
	case float64:
		if v > 0 {
			return uint64(v)
		}
	case string:
		id, _ := strconv.ParseUint(v, 10, 64)
		return id
	}
	return 0
}

func currentRoles(ctx context.Context) []string {
	value := ctx.Value("roles")
	switch v := value.(type) {
	case []string:
		return v
	case []any:
		roles := make([]string, 0, len(v))
		for _, item := range v {
			if role, ok := item.(string); ok && role != "" {
				roles = append(roles, role)
			}
		}
		return roles
	case string:
		if v != "" {
			return []string{v}
		}
	}
	return nil
}

func stepCategoryToType(in *pipelineconfigservice.DevopsStepCategory) types.DevopsStepCategory {
	if in == nil {
		return types.DevopsStepCategory{}
	}
	return types.DevopsStepCategory{
		Id:          in.Id,
		Name:        in.Name,
		Code:        in.Code,
		Description: in.Description,
		Icon:        in.Icon,
		IconColor:   in.IconColor,
		SortOrder:   in.SortOrder,
		Status:      in.Status,
		CreatedBy:   in.CreatedBy,
		UpdatedBy:   in.UpdatedBy,
		CreatedAt:   in.CreatedAt,
		UpdatedAt:   in.UpdatedAt,
	}
}

func stepTemplateToType(in *pipelineconfigservice.DevopsStepTemplate) types.DevopsStepTemplate {
	if in == nil {
		return types.DevopsStepTemplate{}
	}
	return types.DevopsStepTemplate{
		Id:                     in.Id,
		Name:                   in.Name,
		Code:                   in.Code,
		Icon:                   in.Icon,
		IconColor:              in.IconColor,
		Description:            in.Description,
		Type:                   in.Type,
		CategoryId:             in.CategoryId,
		CategoryName:           in.CategoryName,
		EngineType:             in.EngineType,
		EngineChannelGroupCode: in.EngineChannelGroupCode,
		EngineChannelType:      in.EngineChannelType,
		StageContent:           in.StageContent,
		Params:                 stepParamsToType(in.Params),
		TaskParams:             tektonTaskParamsToType(in.TaskParams),
		TaskResults:            tektonTaskResultsToType(in.TaskResults),
		TaskWorkspaces:         tektonWorkspacesToType(in.TaskWorkspaces),
		ArtifactConfig:         artifactConfigToType(in.ArtifactConfig),
		Status:                 in.Status,
		CreatedBy:              in.CreatedBy,
		UpdatedBy:              in.UpdatedBy,
		CreatedAt:              in.CreatedAt,
		UpdatedAt:              in.UpdatedAt,
	}
}

func tektonStepImageToType(in *pipelineconfigservice.TektonStepImageItem) types.TektonStepImageItem {
	if in == nil {
		return types.TektonStepImageItem{}
	}
	return types.TektonStepImageItem{
		Id:        in.Id,
		Name:      in.Name,
		Code:      in.Code,
		Image:     in.Image,
		ImageType: in.ImageType,
		ParamName: in.ParamName,
	}
}

func tektonStepImageUpdatesToRpc(items []types.UpdateTektonStepImageItem) []*pipelineconfigservice.UpdateTektonStepImageItem {
	result := make([]*pipelineconfigservice.UpdateTektonStepImageItem, 0, len(items))
	for _, item := range items {
		result = append(result, &pipelineconfigservice.UpdateTektonStepImageItem{
			Id:        item.Id,
			Image:     item.Image,
			NewImage:  item.NewImage,
			ImageType: item.ImageType,
			ParamName: item.ParamName,
		})
	}
	return result
}

func stepParamsToRpc(items []types.DevopsStepParam) []*pipelineconfigservice.DevopsStepParam {
	result := make([]*pipelineconfigservice.DevopsStepParam, 0, len(items))
	for _, item := range items {
		result = append(result, &pipelineconfigservice.DevopsStepParam{
			Name:             item.Name,
			Code:             item.Code,
			DefaultValue:     item.DefaultValue,
			ParamType:        item.ParamType,
			Mode:             item.Mode,
			Required:         item.Required,
			Readonly:         item.Readonly,
			Description:      item.Description,
			SelectList:       stepParamOptionsToRpc(item.SelectList),
			AllowCustom:      item.AllowCustom,
			ChannelGroupCode: item.ChannelGroupCode,
			MappingField:     item.MappingField,
			VoucherModel:     item.VoucherModel,
			SortOrder:        item.SortOrder,
			RuntimeMode:      item.RuntimeMode,
			RuntimeConfig:    item.RuntimeConfig,
			Config:           stepParamConfigToRpc(item.Config),
		})
	}
	return result
}

func stepParamsToType(items []*pipelineconfigservice.DevopsStepParam) []types.DevopsStepParam {
	result := make([]types.DevopsStepParam, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, types.DevopsStepParam{
			Name:             item.Name,
			Code:             item.Code,
			DefaultValue:     item.DefaultValue,
			ParamType:        item.ParamType,
			Mode:             item.Mode,
			Required:         item.Required,
			Readonly:         item.Readonly,
			Description:      item.Description,
			SelectList:       stepParamOptionsToType(item.SelectList),
			AllowCustom:      item.AllowCustom,
			ChannelGroupCode: item.ChannelGroupCode,
			MappingField:     item.MappingField,
			VoucherModel:     item.VoucherModel,
			SortOrder:        item.SortOrder,
			RuntimeMode:      item.RuntimeMode,
			RuntimeConfig:    item.RuntimeConfig,
			Config:           stepParamConfigToType(item.Config),
		})
	}
	return result
}

func stepParamOptionsToRpc(items []types.StepParamOption) []*pipelineconfigservice.StepParamOption {
	result := make([]*pipelineconfigservice.StepParamOption, 0, len(items))
	for _, item := range items {
		result = append(result, &pipelineconfigservice.StepParamOption{Label: item.Label, Value: item.Value})
	}
	return result
}

func stepParamOptionsToType(items []*pipelineconfigservice.StepParamOption) []types.StepParamOption {
	result := make([]types.StepParamOption, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, types.StepParamOption{Label: item.Label, Value: item.Value})
	}
	return result
}

func tektonTaskParamsToRpc(items []types.DevopsTektonTaskParam) []*pipelineconfigservice.DevopsTektonTaskParam {
	result := make([]*pipelineconfigservice.DevopsTektonTaskParam, 0, len(items))
	for _, item := range items {
		result = append(result, &pipelineconfigservice.DevopsTektonTaskParam{
			Name:         item.Name,
			Type:         item.Type,
			DefaultValue: item.DefaultValue,
			Description:  item.Description,
			Enum:         item.Enum,
			Properties:   tektonPropertiesToRpc(item.Properties),
			Required:     item.Required,
		})
	}
	return result
}

func tektonTaskParamsToType(items []*pipelineconfigservice.DevopsTektonTaskParam) []types.DevopsTektonTaskParam {
	result := make([]types.DevopsTektonTaskParam, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, types.DevopsTektonTaskParam{
			Name:         item.Name,
			Type:         item.Type,
			DefaultValue: item.DefaultValue,
			Description:  item.Description,
			Enum:         item.Enum,
			Properties:   tektonPropertiesToType(item.Properties),
			Required:     item.Required,
		})
	}
	return result
}

func tektonTaskResultsToRpc(items []types.DevopsTektonTaskResult) []*pipelineconfigservice.DevopsTektonTaskResult {
	result := make([]*pipelineconfigservice.DevopsTektonTaskResult, 0, len(items))
	for _, item := range items {
		result = append(result, &pipelineconfigservice.DevopsTektonTaskResult{
			Name:        item.Name,
			Type:        item.Type,
			Description: item.Description,
			Value:       item.Value,
			Properties:  tektonPropertiesToRpc(item.Properties),
		})
	}
	return result
}

func tektonTaskResultsToType(items []*pipelineconfigservice.DevopsTektonTaskResult) []types.DevopsTektonTaskResult {
	result := make([]types.DevopsTektonTaskResult, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, types.DevopsTektonTaskResult{
			Name:        item.Name,
			Type:        item.Type,
			Description: item.Description,
			Value:       item.Value,
			Properties:  tektonPropertiesToType(item.Properties),
		})
	}
	return result
}

func tektonWorkspacesToRpc(items []types.DevopsTektonWorkspace) []*pipelineconfigservice.DevopsTektonWorkspace {
	result := make([]*pipelineconfigservice.DevopsTektonWorkspace, 0, len(items))
	for _, item := range items {
		result = append(result, &pipelineconfigservice.DevopsTektonWorkspace{
			Name:        item.Name,
			Description: item.Description,
			Optional:    item.Optional,
			ReadOnly:    item.ReadOnly,
			MountPath:   item.MountPath,
		})
	}
	return result
}

func tektonWorkspacesToType(items []*pipelineconfigservice.DevopsTektonWorkspace) []types.DevopsTektonWorkspace {
	result := make([]types.DevopsTektonWorkspace, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, types.DevopsTektonWorkspace{
			Name:        item.Name,
			Description: item.Description,
			Optional:    item.Optional,
			ReadOnly:    item.ReadOnly,
			MountPath:   item.MountPath,
		})
	}
	return result
}

func tektonPropertiesToRpc(items map[string]types.TektonPropertySpec) map[string]*pipelineconfigservice.TektonPropertySpec {
	if len(items) == 0 {
		return nil
	}
	result := make(map[string]*pipelineconfigservice.TektonPropertySpec, len(items))
	for name, item := range items {
		result[name] = &pipelineconfigservice.TektonPropertySpec{Type: item.Type}
	}
	return result
}

func tektonPropertiesToType(items map[string]*pipelineconfigservice.TektonPropertySpec) map[string]types.TektonPropertySpec {
	if len(items) == 0 {
		return nil
	}
	result := make(map[string]types.TektonPropertySpec, len(items))
	for name, item := range items {
		if item == nil {
			continue
		}
		result[name] = types.TektonPropertySpec{Type: item.Type}
	}
	return result
}

func compoundFieldsToRpc(items []types.CompoundParamField) []*pipelineconfigservice.CompoundParamField {
	result := make([]*pipelineconfigservice.CompoundParamField, 0, len(items))
	for _, item := range items {
		result = append(result, &pipelineconfigservice.CompoundParamField{
			Name:         item.Name,
			Code:         item.Code,
			DefaultValue: item.DefaultValue,
			ParamType:    item.ParamType,
			Mode:         item.Mode,
			Required:     item.Required,
			Readonly:     item.Readonly,
			Description:  item.Description,
			SelectList:   stepParamOptionsToRpc(item.SelectList),
			AllowCustom:  item.AllowCustom,
			RuntimeMode:  item.RuntimeMode,
			Config:       stepParamConfigToRpc(item.Config),
		})
	}
	return result
}

func compoundFieldsToType(items []*pipelineconfigservice.CompoundParamField) []types.CompoundParamField {
	result := make([]types.CompoundParamField, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, types.CompoundParamField{
			Name:         item.Name,
			Code:         item.Code,
			DefaultValue: item.DefaultValue,
			ParamType:    item.ParamType,
			Mode:         item.Mode,
			Required:     item.Required,
			Readonly:     item.Readonly,
			Description:  item.Description,
			SelectList:   stepParamOptionsToType(item.SelectList),
			AllowCustom:  item.AllowCustom,
			RuntimeMode:  item.RuntimeMode,
			Config:       stepParamConfigToType(item.Config),
		})
	}
	return result
}

func stepParamConfigToRpc(in types.StepParamConfig) *pipelineconfigservice.StepParamConfig {
	return &pipelineconfigservice.StepParamConfig{
		Options:                   stepParamOptionsToRpc(in.Options),
		ChannelGroupCode:          in.ChannelGroupCode,
		ChannelTypeFilter:         in.ChannelTypeFilter,
		ChannelParamCode:          in.ChannelParamCode,
		ChannelBindingId:          in.ChannelBindingId,
		MappingField:              in.MappingField,
		VoucherModel:              in.VoucherModel,
		CredentialId:              in.CredentialId,
		CredentialMode:            in.CredentialMode,
		CredentialSourceParamCode: in.CredentialSourceParamCode,
		ProjectParamCode:          in.ProjectParamCode,
		ComponentParamCode:        in.ComponentParamCode,
		Provider:                  in.Provider,
		ValidateRemote:            in.ValidateRemote,
		CompoundFields:            compoundFieldsToRpc(in.CompoundFields),
		FullRow:                   in.FullRow,
		RenderMode:                in.RenderMode,
		ConfigTypeId:              in.ConfigTypeId,
		ConfigTypeCode:            in.ConfigTypeCode,
		ValueMode:                 in.ValueMode,
		DependencyParamCodes:      stepParamDependenciesToRpc(in.DependencyParamCodes),
	}
}

func stepParamConfigToType(in *pipelineconfigservice.StepParamConfig) types.StepParamConfig {
	if in == nil {
		return types.StepParamConfig{}
	}
	return types.StepParamConfig{
		Options:                   stepParamOptionsToType(in.Options),
		ChannelGroupCode:          in.ChannelGroupCode,
		ChannelTypeFilter:         in.ChannelTypeFilter,
		ChannelParamCode:          in.ChannelParamCode,
		ChannelBindingId:          in.ChannelBindingId,
		MappingField:              in.MappingField,
		VoucherModel:              in.VoucherModel,
		CredentialId:              in.CredentialId,
		CredentialMode:            in.CredentialMode,
		CredentialSourceParamCode: in.CredentialSourceParamCode,
		ProjectParamCode:          in.ProjectParamCode,
		ComponentParamCode:        in.ComponentParamCode,
		Provider:                  in.Provider,
		ValidateRemote:            in.ValidateRemote,
		CompoundFields:            compoundFieldsToType(in.CompoundFields),
		FullRow:                   in.FullRow,
		RenderMode:                in.RenderMode,
		ConfigTypeId:              in.ConfigTypeId,
		ConfigTypeCode:            in.ConfigTypeCode,
		ValueMode:                 in.ValueMode,
		DependencyParamCodes:      stepParamDependenciesToType(in.DependencyParamCodes),
	}
}

func stepParamDependenciesToRpc(items []types.StepParamDependency) []*pipelineconfigservice.StepParamDependency {
	result := make([]*pipelineconfigservice.StepParamDependency, 0, len(items))
	for _, item := range items {
		result = append(result, &pipelineconfigservice.StepParamDependency{
			Field:     item.Field,
			ParamCode: item.ParamCode,
		})
	}
	return result
}

func stepParamDependenciesToType(items []*pipelineconfigservice.StepParamDependency) []types.StepParamDependency {
	result := make([]types.StepParamDependency, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, types.StepParamDependency{
			Field:     item.Field,
			ParamCode: item.ParamCode,
		})
	}
	return result
}

func artifactConfigToRpc(in types.StepArtifactConfig) *pipelineconfigservice.StepArtifactConfig {
	return &pipelineconfigservice.StepArtifactConfig{
		Enabled:     in.Enabled,
		Type:        in.Type,
		Name:        in.Name,
		Path:        in.Path,
		Required:    in.Required,
		ContentType: in.ContentType,
	}
}

func artifactConfigToType(in *pipelineconfigservice.StepArtifactConfig) types.StepArtifactConfig {
	if in == nil {
		return types.StepArtifactConfig{}
	}
	return types.StepArtifactConfig{
		Enabled:     in.Enabled,
		Type:        in.Type,
		Name:        in.Name,
		Path:        in.Path,
		Required:    in.Required,
		ContentType: in.ContentType,
	}
}

func pipelineTemplateToType(in *pipelineconfigservice.DevopsPipelineTemplate) types.DevopsPipelineTemplate {
	if in == nil {
		return types.DevopsPipelineTemplate{}
	}
	return types.DevopsPipelineTemplate{
		Id:                         in.Id,
		Name:                       in.Name,
		Code:                       in.Code,
		Icon:                       in.Icon,
		IconColor:                  in.IconColor,
		Description:                in.Description,
		Scope:                      in.Scope,
		ProjectId:                  in.ProjectId,
		EngineType:                 in.EngineType,
		Steps:                      pipelineStepsToType(in.Steps),
		TektonDagConfig:            in.TektonDagConfig,
		TektonRunPolicy:            in.TektonRunPolicy,
		TektonTriggerConfig:        in.TektonTriggerConfig,
		TektonPrunerPolicyRef:      in.TektonPrunerPolicyRef,
		TektonPipelineYamlSnapshot: in.TektonPipelineYamlSnapshot,
		Status:                     in.Status,
		CreatedBy:                  in.CreatedBy,
		UpdatedBy:                  in.UpdatedBy,
		CreatedAt:                  in.CreatedAt,
		UpdatedAt:                  in.UpdatedAt,
	}
}

func pipelineStepsToRpc(items []types.PipelineTemplateStep) []*pipelineconfigservice.PipelineTemplateStep {
	result := make([]*pipelineconfigservice.PipelineTemplateStep, 0, len(items))
	for _, item := range items {
		result = append(result, &pipelineconfigservice.PipelineTemplateStep{
			Id:             item.Id,
			StepId:         item.StepId,
			StepCode:       item.StepCode,
			StepName:       item.StepName,
			NodeName:       item.NodeName,
			ContainerName:  item.ContainerName,
			StepType:       item.StepType,
			Icon:           item.Icon,
			IconColor:      item.IconColor,
			ParentNodeId:   item.ParentNodeId,
			BranchType:     item.BranchType,
			SortOrder:      item.SortOrder,
			Enabled:        item.Enabled,
			X:              item.X,
			Y:              item.Y,
			ParamValues:    item.ParamValues,
			ArtifactConfig: artifactConfigToRpc(item.ArtifactConfig),
		})
	}
	return result
}

func pipelineStepsToType(items []*pipelineconfigservice.PipelineTemplateStep) []types.PipelineTemplateStep {
	result := make([]types.PipelineTemplateStep, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, types.PipelineTemplateStep{
			Id:             item.Id,
			StepId:         item.StepId,
			StepCode:       item.StepCode,
			StepName:       item.StepName,
			NodeName:       item.NodeName,
			ContainerName:  item.ContainerName,
			StepType:       item.StepType,
			Icon:           item.Icon,
			IconColor:      item.IconColor,
			ParentNodeId:   item.ParentNodeId,
			BranchType:     item.BranchType,
			SortOrder:      item.SortOrder,
			Enabled:        item.Enabled,
			X:              item.X,
			Y:              item.Y,
			ParamValues:    item.ParamValues,
			ArtifactConfig: artifactConfigToType(item.ArtifactConfig),
		})
	}
	return result
}

func pipelineVariableToType(in *pipelineconfigservice.PipelineTemplateVariable) types.PipelineTemplateVariable {
	if in == nil {
		return types.PipelineTemplateVariable{}
	}
	return types.PipelineTemplateVariable{
		StepId:        in.StepId,
		StepNodeId:    in.StepNodeId,
		StepCode:      in.StepCode,
		StepName:      in.StepName,
		Name:          in.Name,
		Code:          in.Code,
		SourceCode:    in.SourceCode,
		ParamType:     in.ParamType,
		Mode:          in.Mode,
		DefaultValue:  in.DefaultValue,
		CurrentValue:  in.CurrentValue,
		Required:      in.Required,
		Readonly:      in.Readonly,
		Description:   in.Description,
		SortOrder:     in.SortOrder,
		RuntimeMode:   in.RuntimeMode,
		RuntimeConfig: in.RuntimeConfig,
		Config:        stepParamConfigToType(in.Config),
	}
}

func jenkinsCredentialSyncRecordToType(in *pipelineconfigservice.JenkinsCredentialSyncRecord) types.JenkinsCredentialSyncRecord {
	if in == nil {
		return types.JenkinsCredentialSyncRecord{}
	}
	return types.JenkinsCredentialSyncRecord{
		CredentialId:        in.CredentialId,
		CredentialName:      in.CredentialName,
		CredentialCode:      in.CredentialCode,
		CredentialType:      in.CredentialType,
		JenkinsCredentialId: in.JenkinsCredentialId,
		SyncStatus:          in.SyncStatus,
		SyncMessage:         in.SyncMessage,
		LastSyncAt:          in.LastSyncAt,
		ReferenceCount:      in.ReferenceCount,
	}
}

func jenkinsCredentialReferenceToType(in *pipelineconfigservice.JenkinsCredentialReference) types.JenkinsCredentialReference {
	if in == nil {
		return types.JenkinsCredentialReference{}
	}
	return types.JenkinsCredentialReference{
		PipelineId:    in.PipelineId,
		PipelineName:  in.PipelineName,
		PipelineCode:  in.PipelineCode,
		ParamCode:     in.ParamCode,
		ParamName:     in.ParamName,
		ReferenceType: in.ReferenceType,
	}
}

func systemToType(in *pipelineconfigservice.DevopsSystem) types.DevopsSystem {
	if in == nil {
		return types.DevopsSystem{}
	}
	return types.DevopsSystem{
		Id:           in.Id,
		ProjectId:    in.ProjectId,
		Name:         in.Name,
		Code:         in.Code,
		Description:  in.Description,
		OwnerUserIds: in.OwnerUserIds,
		Status:       in.Status,
		CreatedBy:    in.CreatedBy,
		UpdatedBy:    in.UpdatedBy,
		CreatedAt:    in.CreatedAt,
		UpdatedAt:    in.UpdatedAt,
	}
}

func environmentToType(in *pipelineconfigservice.DevopsPipelineEnvironment) types.DevopsPipelineEnvironment {
	if in == nil {
		return types.DevopsPipelineEnvironment{}
	}
	return types.DevopsPipelineEnvironment{
		Id:          in.Id,
		Name:        in.Name,
		Code:        in.Code,
		Description: in.Description,
		Icon:        in.Icon,
		IconColor:   in.IconColor,
		SortOrder:   in.SortOrder,
		Status:      in.Status,
		CreatedBy:   in.CreatedBy,
		UpdatedBy:   in.UpdatedBy,
		CreatedAt:   in.CreatedAt,
		UpdatedAt:   in.UpdatedAt,
	}
}

func execStepsToRpc(items []types.PipelineTemplateStep) []*executionservice.PipelineStep {
	result := make([]*executionservice.PipelineStep, 0, len(items))
	for _, item := range items {
		result = append(result, &executionservice.PipelineStep{
			Id:             item.Id,
			StepId:         item.StepId,
			StepCode:       item.StepCode,
			StepName:       item.StepName,
			NodeName:       item.NodeName,
			ContainerName:  item.ContainerName,
			StepType:       item.StepType,
			Icon:           item.Icon,
			IconColor:      item.IconColor,
			ParentNodeId:   item.ParentNodeId,
			BranchType:     item.BranchType,
			SortOrder:      item.SortOrder,
			Enabled:        item.Enabled,
			X:              item.X,
			Y:              item.Y,
			ParamValues:    item.ParamValues,
			ArtifactConfig: execArtifactConfigToRpc(item.ArtifactConfig),
		})
	}
	return result
}

func execStepsToType(items []*executionservice.PipelineStep) []types.PipelineTemplateStep {
	result := make([]types.PipelineTemplateStep, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, types.PipelineTemplateStep{
			Id:             item.Id,
			StepId:         item.StepId,
			StepCode:       item.StepCode,
			StepName:       item.StepName,
			NodeName:       item.NodeName,
			ContainerName:  item.ContainerName,
			StepType:       item.StepType,
			Icon:           item.Icon,
			IconColor:      item.IconColor,
			ParentNodeId:   item.ParentNodeId,
			BranchType:     item.BranchType,
			SortOrder:      item.SortOrder,
			Enabled:        item.Enabled,
			X:              item.X,
			Y:              item.Y,
			ParamValues:    item.ParamValues,
			ArtifactConfig: execArtifactConfigToType(item.ArtifactConfig),
		})
	}
	return result
}

func pipelineParamsToRpc(items []types.PipelineParam) []*executionservice.PipelineParam {
	result := make([]*executionservice.PipelineParam, 0, len(items))
	for _, item := range items {
		result = append(result, &executionservice.PipelineParam{
			Name:          item.Name,
			Code:          item.Code,
			SourceCode:    item.SourceCode,
			StepNodeId:    item.StepNodeId,
			DefaultValue:  item.DefaultValue,
			CurrentValue:  item.CurrentValue,
			ParamType:     item.ParamType,
			Mode:          item.Mode,
			Required:      item.Required,
			Readonly:      item.Readonly,
			Description:   item.Description,
			SelectList:    execStepParamOptionsToRpc(item.SelectList),
			AllowCustom:   item.AllowCustom,
			SortOrder:     item.SortOrder,
			RuntimeMode:   item.RuntimeMode,
			RuntimeConfig: item.RuntimeConfig,
			Config:        execStepParamConfigToRpc(item.Config),
		})
	}
	return result
}

func pipelineParamsToType(items []*executionservice.PipelineParam) []types.PipelineParam {
	result := make([]types.PipelineParam, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, types.PipelineParam{
			Name:          item.Name,
			Code:          item.Code,
			SourceCode:    item.SourceCode,
			StepNodeId:    item.StepNodeId,
			DefaultValue:  item.DefaultValue,
			CurrentValue:  item.CurrentValue,
			ParamType:     item.ParamType,
			Mode:          item.Mode,
			Required:      item.Required,
			Readonly:      item.Readonly,
			Description:   item.Description,
			SelectList:    execStepParamOptionsToType(item.SelectList),
			AllowCustom:   item.AllowCustom,
			SortOrder:     item.SortOrder,
			RuntimeMode:   item.RuntimeMode,
			RuntimeConfig: item.RuntimeConfig,
			Config:        execStepParamConfigToType(item.Config),
		})
	}
	return result
}

func execStepParamOptionsToRpc(items []types.StepParamOption) []*executionservice.StepParamOption {
	result := make([]*executionservice.StepParamOption, 0, len(items))
	for _, item := range items {
		result = append(result, &executionservice.StepParamOption{Label: item.Label, Value: item.Value})
	}
	return result
}

func execStepParamOptionsToType(items []*executionservice.StepParamOption) []types.StepParamOption {
	result := make([]types.StepParamOption, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, types.StepParamOption{Label: item.Label, Value: item.Value})
	}
	return result
}

func execStepParamConfigToRpc(in types.StepParamConfig) *executionservice.StepParamConfig {
	return &executionservice.StepParamConfig{
		Options:                   execStepParamOptionsToRpc(in.Options),
		ChannelGroupCode:          in.ChannelGroupCode,
		ChannelTypeFilter:         in.ChannelTypeFilter,
		ChannelParamCode:          in.ChannelParamCode,
		ChannelBindingId:          in.ChannelBindingId,
		MappingField:              in.MappingField,
		VoucherModel:              in.VoucherModel,
		CredentialId:              in.CredentialId,
		CredentialMode:            in.CredentialMode,
		CredentialSourceParamCode: in.CredentialSourceParamCode,
		ProjectParamCode:          in.ProjectParamCode,
		ComponentParamCode:        in.ComponentParamCode,
		Provider:                  in.Provider,
		ValidateRemote:            in.ValidateRemote,
		CompoundFields:            execCompoundFieldsToRpc(in.CompoundFields),
		FullRow:                   in.FullRow,
		RenderMode:                in.RenderMode,
		ConfigTypeId:              in.ConfigTypeId,
		ConfigTypeCode:            in.ConfigTypeCode,
		ValueMode:                 in.ValueMode,
		DependencyParamCodes:      execStepParamDependenciesToRpc(in.DependencyParamCodes),
	}
}

func execStepParamConfigToType(in *executionservice.StepParamConfig) types.StepParamConfig {
	if in == nil {
		return types.StepParamConfig{}
	}
	return types.StepParamConfig{
		Options:                   execStepParamOptionsToType(in.Options),
		ChannelGroupCode:          in.ChannelGroupCode,
		ChannelTypeFilter:         in.ChannelTypeFilter,
		ChannelParamCode:          in.ChannelParamCode,
		ChannelBindingId:          in.ChannelBindingId,
		MappingField:              in.MappingField,
		VoucherModel:              in.VoucherModel,
		CredentialId:              in.CredentialId,
		CredentialMode:            in.CredentialMode,
		CredentialSourceParamCode: in.CredentialSourceParamCode,
		ProjectParamCode:          in.ProjectParamCode,
		ComponentParamCode:        in.ComponentParamCode,
		Provider:                  in.Provider,
		ValidateRemote:            in.ValidateRemote,
		CompoundFields:            execCompoundFieldsToType(in.CompoundFields),
		FullRow:                   in.FullRow,
		RenderMode:                in.RenderMode,
		ConfigTypeId:              in.ConfigTypeId,
		ConfigTypeCode:            in.ConfigTypeCode,
		ValueMode:                 in.ValueMode,
		DependencyParamCodes:      execStepParamDependenciesToType(in.DependencyParamCodes),
	}
}

func execStepParamDependenciesToRpc(items []types.StepParamDependency) []*executionservice.StepParamDependency {
	result := make([]*executionservice.StepParamDependency, 0, len(items))
	for _, item := range items {
		result = append(result, &executionservice.StepParamDependency{
			Field:     item.Field,
			ParamCode: item.ParamCode,
		})
	}
	return result
}

func execStepParamDependenciesToType(items []*executionservice.StepParamDependency) []types.StepParamDependency {
	result := make([]types.StepParamDependency, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, types.StepParamDependency{
			Field:     item.Field,
			ParamCode: item.ParamCode,
		})
	}
	return result
}

func execCompoundFieldsToRpc(items []types.CompoundParamField) []*executionservice.CompoundParamField {
	result := make([]*executionservice.CompoundParamField, 0, len(items))
	for _, item := range items {
		result = append(result, &executionservice.CompoundParamField{
			Name:         item.Name,
			Code:         item.Code,
			DefaultValue: item.DefaultValue,
			ParamType:    item.ParamType,
			Mode:         item.Mode,
			Required:     item.Required,
			Readonly:     item.Readonly,
			Description:  item.Description,
			SelectList:   execStepParamOptionsToRpc(item.SelectList),
			AllowCustom:  item.AllowCustom,
			RuntimeMode:  item.RuntimeMode,
			Config:       execStepParamConfigToRpc(item.Config),
		})
	}
	return result
}

func execCompoundFieldsToType(items []*executionservice.CompoundParamField) []types.CompoundParamField {
	result := make([]types.CompoundParamField, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, types.CompoundParamField{
			Name:         item.Name,
			Code:         item.Code,
			DefaultValue: item.DefaultValue,
			ParamType:    item.ParamType,
			Mode:         item.Mode,
			Required:     item.Required,
			Readonly:     item.Readonly,
			Description:  item.Description,
			SelectList:   execStepParamOptionsToType(item.SelectList),
			AllowCustom:  item.AllowCustom,
			RuntimeMode:  item.RuntimeMode,
			Config:       execStepParamConfigToType(item.Config),
		})
	}
	return result
}

func execArtifactConfigToRpc(in types.StepArtifactConfig) *executionservice.StepArtifactConfig {
	return &executionservice.StepArtifactConfig{
		Enabled:     in.Enabled,
		Type:        in.Type,
		Name:        in.Name,
		Path:        in.Path,
		Required:    in.Required,
		ContentType: in.ContentType,
	}
}

func execArtifactConfigToType(in *executionservice.StepArtifactConfig) types.StepArtifactConfig {
	if in == nil {
		return types.StepArtifactConfig{}
	}
	return types.StepArtifactConfig{
		Enabled:     in.Enabled,
		Type:        in.Type,
		Name:        in.Name,
		Path:        in.Path,
		Required:    in.Required,
		ContentType: in.ContentType,
	}
}

func agentToRpc(in types.JenkinsAgent) *executionservice.JenkinsAgent {
	agentType := strings.TrimSpace(in.Type)
	if agentType == "pod" {
		agentType = "dynamic"
	}
	if agentType == "" {
		agentType = "any"
	}
	configuredAgentType := strings.TrimSpace(in.AgentType)
	if configuredAgentType == "pod" {
		configuredAgentType = "dynamic"
	}
	resp := &executionservice.JenkinsAgent{
		Type:       agentType,
		Id:         in.Id,
		Name:       in.Name,
		AgentType:  configuredAgentType,
		MatchMode:  in.MatchMode,
		MatchValue: in.MatchValue,
		Cloud:      in.Cloud,
		PodYaml:    in.PodYaml,
		Containers: execAgentContainersToRpc(in.Containers),
	}
	switch agentType {
	case "static", "dynamic":
		return resp
	case "label":
		resp.Label = in.Label
	case "docker":
		resp.DockerImage = in.DockerImage
		resp.DockerArgs = in.DockerArgs
	case "raw":
		resp.Raw = in.Raw
	}
	return resp
}

func agentToType(in *executionservice.JenkinsAgent) types.JenkinsAgent {
	if in == nil {
		return types.JenkinsAgent{}
	}
	return types.JenkinsAgent{
		Type:        in.Type,
		Label:       in.Label,
		DockerImage: in.DockerImage,
		DockerArgs:  in.DockerArgs,
		Raw:         in.Raw,
		Id:          in.Id,
		Name:        in.Name,
		AgentType:   in.AgentType,
		MatchMode:   in.MatchMode,
		MatchValue:  in.MatchValue,
		Cloud:       in.Cloud,
		PodYaml:     in.PodYaml,
		Containers:  execAgentContainersToType(in.Containers),
	}
}

func execAgentContainersToRpc(items []types.JenkinsAgentContainer) []*executionservice.JenkinsAgentContainer {
	result := make([]*executionservice.JenkinsAgentContainer, 0, len(items))
	for _, item := range items {
		result = append(result, &executionservice.JenkinsAgentContainer{Name: item.Name, Image: item.Image})
	}
	return result
}

func execAgentContainersToType(items []*executionservice.JenkinsAgentContainer) []types.JenkinsAgentContainer {
	result := make([]types.JenkinsAgentContainer, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, types.JenkinsAgentContainer{Name: item.Name, Image: item.Image})
	}
	return result
}

func toolsToRpc(items []types.JenkinsTool) []*executionservice.JenkinsTool {
	result := make([]*executionservice.JenkinsTool, 0, len(items))
	for _, item := range items {
		result = append(result, &executionservice.JenkinsTool{Name: item.Name, Type: item.Type, Version: item.Version})
	}
	return result
}

func toolsToType(items []*executionservice.JenkinsTool) []types.JenkinsTool {
	result := make([]types.JenkinsTool, 0, len(items))
	for _, item := range items {
		if item != nil {
			result = append(result, types.JenkinsTool{Name: item.Name, Type: item.Type, Version: item.Version})
		}
	}
	return result
}

func execOptionsToRpc(items []types.JenkinsOption) []*executionservice.JenkinsOption {
	result := make([]*executionservice.JenkinsOption, 0, len(items))
	for _, item := range items {
		result = append(result, &executionservice.JenkinsOption{Name: item.Name, Value: item.Value})
	}
	return result
}

func execOptionsToType(items []*executionservice.JenkinsOption) []types.JenkinsOption {
	result := make([]types.JenkinsOption, 0, len(items))
	for _, item := range items {
		if item != nil {
			result = append(result, types.JenkinsOption{Name: item.Name, Value: item.Value})
		}
	}
	return result
}

func triggersToRpc(items []types.JenkinsTrigger) []*executionservice.JenkinsTrigger {
	result := make([]*executionservice.JenkinsTrigger, 0, len(items))
	for _, item := range items {
		result = append(result, &executionservice.JenkinsTrigger{Type: item.Type, Spec: item.Spec, Enabled: item.Enabled})
	}
	return result
}

func triggersToType(items []*executionservice.JenkinsTrigger) []types.JenkinsTrigger {
	result := make([]types.JenkinsTrigger, 0, len(items))
	for _, item := range items {
		if item != nil {
			result = append(result, types.JenkinsTrigger{Type: item.Type, Spec: item.Spec, Enabled: item.Enabled})
		}
	}
	return result
}

func pipelineToType(in *executionservice.DevopsPipeline) types.DevopsPipeline {
	if in == nil {
		return types.DevopsPipeline{}
	}
	return types.DevopsPipeline{
		Id:                         in.Id,
		ProjectId:                  in.ProjectId,
		ProjectName:                in.ProjectName,
		ProjectCode:                in.ProjectCode,
		SystemId:                   in.SystemId,
		SystemName:                 in.SystemName,
		SystemCode:                 in.SystemCode,
		EnvironmentId:              in.EnvironmentId,
		EnvironmentName:            in.EnvironmentName,
		EnvironmentCode:            in.EnvironmentCode,
		Name:                       in.Name,
		Code:                       in.Code,
		EngineType:                 in.EngineType,
		TemplateId:                 in.TemplateId,
		BuildChannelBindingId:      in.BuildChannelBindingId,
		BuildChannelName:           in.BuildChannelName,
		Steps:                      execStepsToType(in.Steps),
		Params:                     pipelineParamsToType(in.Params),
		Agent:                      agentToType(in.Agent),
		Tools:                      toolsToType(in.Tools),
		Options:                    execOptionsToType(in.Options),
		Triggers:                   triggersToType(in.Triggers),
		JobName:                    in.JobName,
		JobFullName:                in.JobFullName,
		TriggerMode:                in.TriggerMode,
		SyncStatus:                 in.SyncStatus,
		SyncMessage:                in.SyncMessage,
		LastRunStatus:              in.LastRunStatus,
		Status:                     in.Status,
		CreatedBy:                  in.CreatedBy,
		UpdatedBy:                  in.UpdatedBy,
		CreatedAt:                  in.CreatedAt,
		UpdatedAt:                  in.UpdatedAt,
		LastRunId:                  in.LastRunId,
		LastRunBuildNumber:         in.LastRunBuildNumber,
		LastRunBuildUrl:            in.LastRunBuildUrl,
		LastRunStartedAt:           in.LastRunStartedAt,
		ScanEnabled:                in.ScanEnabled,
		ScanMode:                   in.ScanMode,
		RejectUnexpectedReports:    in.RejectUnexpectedReports,
		Enforce:                    in.Enforce,
		ScanItems:                  pipelineScanItemsToType(in.ScanItems),
		TektonDagConfig:            in.TektonDagConfig,
		TektonRunPolicy:            in.TektonRunPolicy,
		TektonTriggerConfig:        in.TektonTriggerConfig,
		TektonPrunerPolicyRef:      in.TektonPrunerPolicyRef,
		TektonPipelineYamlSnapshot: in.TektonPipelineYamlSnapshot,
		TektonTemplateId:           in.TektonTemplateId,
		TektonTemplateSnapshot:     in.TektonTemplateSnapshot,
		TektonParamBindings:        in.TektonParamBindings,
		TektonWorkspaceBindings:    in.TektonWorkspaceBindings,
		TektonResourceBindings:     in.TektonResourceBindings,
	}
}

func pipelineScanItemsToRpc(items []types.PipelineScanPlanItem) []*executionservice.ScanPlanItem {
	result := make([]*executionservice.ScanPlanItem, 0, len(items))
	for _, item := range items {
		result = append(result, &executionservice.ScanPlanItem{
			StepId:          item.StepId,
			StageId:         item.StageId,
			Tool:            item.Tool,
			ToolMode:        item.ToolMode,
			ToolBindingId:   item.ToolBindingId,
			TargetType:      item.TargetType,
			TargetName:      item.TargetName,
			TargetParamCode: item.TargetParamCode,
			ReportSource:    item.ReportSource,
			ReportPath:      item.ReportPath,
			ReportFormat:    item.ReportFormat,
			Required:        item.Required,
			Enforce:         item.Enforce,
			Parser:          item.Parser,
		})
	}
	return result
}

func pipelineScanItemsToType(items []*executionservice.ScanPlanItem) []types.PipelineScanPlanItem {
	result := make([]types.PipelineScanPlanItem, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, types.PipelineScanPlanItem{
			StepId:          item.StepId,
			StageId:         item.StageId,
			Tool:            item.Tool,
			ToolMode:        item.ToolMode,
			ToolBindingId:   item.ToolBindingId,
			TargetType:      item.TargetType,
			TargetName:      item.TargetName,
			TargetParamCode: item.TargetParamCode,
			ReportSource:    item.ReportSource,
			ReportPath:      item.ReportPath,
			ReportFormat:    item.ReportFormat,
			Required:        item.Required,
			Enforce:         item.Enforce,
			Parser:          item.Parser,
		})
	}
	return result
}

func pipelineRunToType(in *executionservice.DevopsPipelineRun) types.DevopsPipelineRun {
	if in == nil {
		return types.DevopsPipelineRun{}
	}
	return types.DevopsPipelineRun{
		Id:                    in.Id,
		ProjectId:             in.ProjectId,
		ProjectName:           in.ProjectName,
		ProjectCode:           in.ProjectCode,
		SystemId:              in.SystemId,
		SystemName:            in.SystemName,
		SystemCode:            in.SystemCode,
		EnvironmentId:         in.EnvironmentId,
		EnvironmentName:       in.EnvironmentName,
		EnvironmentCode:       in.EnvironmentCode,
		PipelineId:            in.PipelineId,
		PipelineName:          in.PipelineName,
		PipelineCode:          in.PipelineCode,
		EngineType:            in.EngineType,
		JenkinsJobName:        in.JenkinsJobName,
		JenkinsJobFullName:    in.JenkinsJobFullName,
		JenkinsBuildNumber:    in.JenkinsBuildNumber,
		JenkinsQueueId:        in.JenkinsQueueId,
		JenkinsBuildUrl:       in.JenkinsBuildUrl,
		BuildId:               in.BuildId,
		TektonNamespace:       in.TektonNamespace,
		TektonPipelineName:    in.TektonPipelineName,
		TektonPipelineRunName: in.TektonPipelineRunName,
		TriggerType:           in.TriggerType,
		TriggerUserId:         in.TriggerUserId,
		TriggerUsername:       in.TriggerUsername,
		Status:                in.Status,
		StartedAt:             in.StartedAt,
		FinishedAt:            in.FinishedAt,
		DurationSeconds:       in.DurationSeconds,
		ErrorMessage:          in.ErrorMessage,
		LogStorageType:        in.LogStorageType,
		LogObjectKey:          in.LogObjectKey,
		LogSize:               in.LogSize,
		LogArchivedAt:         in.LogArchivedAt,
	}
}

func pipelineRunStageToType(in *executionservice.PipelineRunStage) types.DevopsPipelineRunStage {
	if in == nil {
		return types.DevopsPipelineRunStage{}
	}
	return types.DevopsPipelineRunStage{
		Id:                in.Id,
		RunId:             in.RunId,
		PipelineId:        in.PipelineId,
		NodeId:            in.NodeId,
		StageName:         in.StageName,
		StageType:         in.StageType,
		Status:            in.Status,
		StartedAt:         in.StartedAt,
		FinishedAt:        in.FinishedAt,
		DurationSeconds:   in.DurationSeconds,
		JenkinsNodeId:     in.JenkinsNodeId,
		LogObjectKey:      in.LogObjectKey,
		ErrorMessage:      in.ErrorMessage,
		TektonTaskRunName: in.TektonTaskRunName,
		TektonPodName:     in.TektonPodName,
		ContainerName:     in.ContainerName,
		ContainerNames:    in.ContainerNames,
		TektonTaskRuns:    tektonTaskRunSnapshotsToType(in.TektonTaskRuns),
	}
}

func tektonTaskRunSnapshotsToType(items []*executionservice.TektonTaskRunSnapshot) []types.TektonTaskRunSnapshot {
	result := make([]types.TektonTaskRunSnapshot, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, types.TektonTaskRunSnapshot{
			Name:             item.Name,
			PipelineTaskName: item.PipelineTaskName,
			PodName:          item.PodName,
			ContainerName:    item.ContainerName,
			ContainerNames:   item.ContainerNames,
			Status:           item.Status,
			StartedAt:        item.StartedAt,
			FinishedAt:       item.FinishedAt,
			DurationSeconds:  item.DurationSeconds,
			ErrorMessage:     item.ErrorMessage,
		})
	}
	return result
}

func tektonRuntimeResourceToType(in *executionservice.TektonRuntimeResource) types.TektonRuntimeResource {
	if in == nil {
		return types.TektonRuntimeResource{}
	}
	return types.TektonRuntimeResource{
		Name:                 in.Name,
		Namespace:            in.Namespace,
		Kind:                 in.Kind,
		ApiVersion:           in.ApiVersion,
		Description:          in.Description,
		Params:               in.Params,
		Workspaces:           in.Workspaces,
		Results:              in.Results,
		Provisioner:          in.Provisioner,
		ReclaimPolicy:        in.ReclaimPolicy,
		VolumeBindingMode:    in.VolumeBindingMode,
		AllowVolumeExpansion: in.AllowVolumeExpansion,
		Secrets:              in.Secrets,
		ImagePullSecrets:     in.ImagePullSecrets,
		CreatedAt:            in.CreatedAt,
		Yaml:                 in.Yaml,
		Keys:                 in.Keys,
	}
}

func pipelineRunInputToType(in *executionservice.PipelineRunInput) types.DevopsPipelineRunInput {
	if in == nil {
		return types.DevopsPipelineRunInput{}
	}
	params := make([]types.DevopsPipelineRunInputParam, 0, len(in.Params))
	for _, item := range in.Params {
		if item == nil {
			continue
		}
		options := make([]types.StepParamOption, 0, len(item.Options))
		for _, opt := range item.Options {
			if opt == nil {
				continue
			}
			options = append(options, types.StepParamOption{
				Label: opt.Label,
				Value: opt.Value,
			})
		}
		params = append(params, types.DevopsPipelineRunInputParam{
			Name:         item.Name,
			Type:         item.Type,
			Description:  item.Description,
			DefaultValue: item.DefaultValue,
			Options:      options,
		})
	}
	return types.DevopsPipelineRunInput{
		Id:      in.Id,
		Name:    in.Name,
		Message: in.Message,
		Params:  params,
	}
}

func pipelineDashboardOverviewToType(in *executionservice.PipelineDashboardOverview) types.DevopsPipelineDashboardOverview {
	if in == nil {
		return types.DevopsPipelineDashboardOverview{}
	}
	return types.DevopsPipelineDashboardOverview{
		BuildCount:           in.BuildCount,
		SuccessCount:         in.SuccessCount,
		FailureCount:         in.FailureCount,
		AbortedCount:         in.AbortedCount,
		TotalDurationSeconds: in.TotalDurationSeconds,
		SuccessRate:          in.SuccessRate,
		FailureRate:          in.FailureRate,
		AvgDurationSeconds:   in.AvgDurationSeconds,
		ProjectCount:         in.ProjectCount,
		SystemCount:          in.SystemCount,
		EnvironmentCount:     in.EnvironmentCount,
		PipelineCount:        in.PipelineCount,
		ChannelCount:         in.ChannelCount,
	}
}

func pipelineDashboardMetricsToType(items []*executionservice.PipelineDashboardMetric) []types.DevopsPipelineDashboardMetric {
	result := make([]types.DevopsPipelineDashboardMetric, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, types.DevopsPipelineDashboardMetric{
			Id:                   item.Id,
			Name:                 item.Name,
			Code:                 item.Code,
			BuildCount:           item.BuildCount,
			SuccessCount:         item.SuccessCount,
			FailureCount:         item.FailureCount,
			AbortedCount:         item.AbortedCount,
			TotalDurationSeconds: item.TotalDurationSeconds,
			SuccessRate:          item.SuccessRate,
			FailureRate:          item.FailureRate,
			AvgDurationSeconds:   item.AvgDurationSeconds,
			LastRunAt:            item.LastRunAt,
		})
	}
	return result
}

func pipelineDashboardTrendsToType(items []*executionservice.PipelineDashboardTrend) []types.DevopsPipelineDashboardTrend {
	result := make([]types.DevopsPipelineDashboardTrend, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, types.DevopsPipelineDashboardTrend{
			Date:            item.Date,
			BuildCount:      item.BuildCount,
			SuccessCount:    item.SuccessCount,
			FailureCount:    item.FailureCount,
			DurationSeconds: item.DurationSeconds,
		})
	}
	return result
}
