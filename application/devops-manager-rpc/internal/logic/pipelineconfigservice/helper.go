package pipelineconfigservicelogic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	portalprojectservice "github.com/yanshicheng/kube-nova/application/portal-rpc/client/portalprojectservice"
	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
	"github.com/yanshicheng/kube-nova/common/devops/jenkins"
	devopstekton "github.com/yanshicheng/kube-nova/common/devops/tekton"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
	yamlv3 "gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	jenkinsEngineType             = "jenkins"
	tektonEngineType              = "tekton"
	jenkinsEngineChannelType      = "jenkins"
	tektonEngineChannelType       = "tekton"
	jenkinsEngineChannelGroupCode = model.BuildChannelGroupCode
	tektonEngineChannelGroupCode  = model.BuildChannelGroupCode
	stepTypeTask                  = "task"
	stepTypeManualApproval        = "manual_approval"
	workflowStartNodeID           = "workflow-start"
	workflowPostNodeID            = "workflow-post"
	branchTypeNext                = "next"
	branchTypeParallel            = "parallel"
	stepCategoryCodePost          = "post"
)

var pipelineCodePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
var tektonParamNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`)
var tektonDurationPattern = regexp.MustCompile(`^([0-9]+(ms|h|m|s))+$`)
var stageDeclarePattern = regexp.MustCompile(`stage\s*\(\s*['"][^'"]*['"]\s*\)`)
var stageCallPattern = regexp.MustCompile(`\bstage\s*\(`)
var parallelBlockPattern = regexp.MustCompile(`\bparallel\s*\{`)
var postBlockPattern = regexp.MustCompile(`\bpost\s*\{`)
var stepsBlockPattern = regexp.MustCompile(`\bsteps\s*\{`)
var postConditionPattern = regexp.MustCompile(`^\s*(always|success|failure|changed|aborted|unstable)\s*\{`)

const objectListParamType = "objectList"

var postConditionLinePattern = regexp.MustCompile(`(?m)^\s*(always|success|failure|changed|aborted|unstable)\s*\{`)
var kubernetesSecretNamePattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$`)
var tektonResourceNameInvalidPattern = regexp.MustCompile(`[^a-z0-9-]+`)
var tektonTaskKindPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9.]*$`)
var tektonAPIVersionPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+(/[A-Za-z0-9_.-]+)?$`)

func stepCategoryToPb(in *model.DevopsStepCategory) *pb.DevopsStepCategory {
	if in == nil {
		return nil
	}
	return &pb.DevopsStepCategory{
		Id:          in.ID.Hex(),
		Name:        in.Name,
		Code:        in.Code,
		Description: in.Description,
		Icon:        in.Icon,
		IconColor:   in.IconColor,
		SortOrder:   in.SortOrder,
		Status:      in.Status,
		CreatedBy:   in.CreatedBy,
		UpdatedBy:   in.UpdatedBy,
		CreatedAt:   in.CreateAt.Unix(),
		UpdatedAt:   in.UpdateAt.Unix(),
	}
}

func stepTemplateToPb(ctx context.Context, svcCtx *svc.ServiceContext, in *model.DevopsStepTemplate) *pb.DevopsStepTemplate {
	if in == nil {
		return nil
	}
	categoryName := ""
	categoryCode := ""
	if in.CategoryID != "" {
		if category, err := svcCtx.StepCategoryModel.FindOne(ctx, in.CategoryID); err == nil {
			categoryName = category.Name
			categoryCode = category.Code
		}
	}
	return stepTemplateToPbWithCategory(in, categoryName, categoryCode)
}

func stepTemplateToPbWithCategory(in *model.DevopsStepTemplate, categoryName, categoryCode string) *pb.DevopsStepTemplate {
	if in == nil {
		return nil
	}
	taskParams, taskResults, taskWorkspaces := in.TaskParams, in.TaskResults, in.TaskWorkspaces
	if in.EngineType == tektonEngineType && len(taskParams) == 0 && len(taskResults) == 0 && len(taskWorkspaces) == 0 {
		if params, results, workspaces, err := tektonTaskContractsFromContent(in.StageContent); err == nil {
			taskParams = params
			taskResults = results
			taskWorkspaces = workspaces
		}
	}
	return &pb.DevopsStepTemplate{
		Id:                     in.ID.Hex(),
		Name:                   in.Name,
		Code:                   in.Code,
		Icon:                   in.Icon,
		IconColor:              in.IconColor,
		Description:            in.Description,
		Type:                   normalizeStepType(in.Type),
		CategoryId:             in.CategoryID,
		CategoryName:           categoryName,
		EngineType:             in.EngineType,
		EngineChannelGroupCode: in.EngineChannelGroupCode,
		EngineChannelType:      in.EngineChannelType,
		StageContent:           stepContentForResponse(in.EngineType, in.StageContent, categoryCode == stepCategoryCodePost),
		Params:                 stepParamsToPb(in.Params),
		TaskParams:             tektonTaskParamsToPb(taskParams),
		TaskResults:            tektonTaskResultsToPb(taskResults),
		TaskWorkspaces:         tektonWorkspacesToPb(taskWorkspaces),
		ArtifactConfig:         artifactConfigToPb(in.ArtifactConfig),
		Status:                 in.Status,
		CreatedBy:              in.CreatedBy,
		UpdatedBy:              in.UpdatedBy,
		CreatedAt:              in.CreateAt.Unix(),
		UpdatedAt:              in.UpdateAt.Unix(),
	}
}

func stepContentForResponse(engineType, content string, isPost bool) string {
	content = strings.TrimSpace(content)
	if engineType == tektonEngineType {
		return content
	}
	if content == "" || isPost {
		return content
	}
	if _, ok := extractWholeGroovyNamedBlock(content, "steps"); ok {
		return content
	}
	body, err := stepsBodyContent(content)
	if err != nil {
		return content
	}
	return fmt.Sprintf("steps {\n%s\n}", indentPlainText(body, 2))
}

func pipelineTemplateToPb(in *model.DevopsPipelineTemplate) *pb.DevopsPipelineTemplate {
	if in == nil {
		return nil
	}
	return &pb.DevopsPipelineTemplate{
		Id:                         in.ID.Hex(),
		Name:                       in.Name,
		Code:                       in.Code,
		Icon:                       in.Icon,
		IconColor:                  in.IconColor,
		Description:                in.Description,
		Scope:                      normalizeTemplatePersistedScope(in.Scope, in.ProjectID),
		ProjectId:                  in.ProjectID,
		EngineType:                 in.EngineType,
		Steps:                      pipelineStepsToPb(in.Steps),
		TektonDagConfig:            in.TektonDagConfig,
		TektonRunPolicy:            in.TektonRunPolicy,
		TektonTriggerConfig:        in.TektonTriggerConfig,
		TektonPrunerPolicyRef:      in.TektonPrunerPolicyRef,
		TektonPipelineYamlSnapshot: in.TektonPipelineYamlSnapshot,
		Status:                     in.Status,
		CreatedBy:                  in.CreatedBy,
		UpdatedBy:                  in.UpdatedBy,
		CreatedAt:                  in.CreateAt.Unix(),
		UpdatedAt:                  in.UpdateAt.Unix(),
	}
}

func systemToPb(in *model.DevopsSystem) *pb.DevopsSystem {
	if in == nil {
		return nil
	}
	return &pb.DevopsSystem{
		Id:           in.ID.Hex(),
		ProjectId:    in.ProjectID,
		Name:         in.Name,
		Code:         in.Code,
		Description:  in.Description,
		OwnerUserIds: in.OwnerUserIDs,
		Status:       in.Status,
		CreatedBy:    in.CreatedBy,
		UpdatedBy:    in.UpdatedBy,
		CreatedAt:    in.CreateAt.Unix(),
		UpdatedAt:    in.UpdateAt.Unix(),
	}
}

func pipelineEnvironmentToPb(in *model.DevopsPipelineEnvironment) *pb.DevopsPipelineEnvironment {
	if in == nil {
		return nil
	}
	return &pb.DevopsPipelineEnvironment{
		Id:          in.ID.Hex(),
		Name:        in.Name,
		Code:        in.Code,
		Description: in.Description,
		Icon:        in.Icon,
		IconColor:   in.IconColor,
		SortOrder:   in.SortOrder,
		Status:      in.Status,
		CreatedBy:   in.CreatedBy,
		UpdatedBy:   in.UpdatedBy,
		CreatedAt:   in.CreateAt.Unix(),
		UpdatedAt:   in.UpdateAt.Unix(),
	}
}

func projectToPb(in *model.DevopsProject) *pb.DevopsProject {
	if in == nil {
		return nil
	}
	return &pb.DevopsProject{
		Id:                     in.ID.Hex(),
		Name:                   in.Name,
		Code:                   in.Code,
		Description:            in.Description,
		PipelineEngineType:     in.PipelineEngineType,
		DefaultEngineChannelId: in.DefaultEngineChannelID,
		Status:                 in.Status,
		ExtraConfig:            in.ExtraConfig,
		CreatedBy:              in.CreatedBy,
		UpdatedBy:              in.UpdatedBy,
		CreatedAt:              in.CreateAt.Unix(),
		UpdatedAt:              in.UpdateAt.Unix(),
	}
}

func projectChannelBindingToPb(in *model.DevopsProjectChannelBinding) *pb.DevopsProjectChannelBinding {
	if in == nil {
		return nil
	}
	return &pb.DevopsProjectChannelBinding{
		Id:                       in.ID.Hex(),
		ProjectId:                in.ProjectID,
		ChannelId:                in.ChannelID,
		ChannelGroupCode:         in.ChannelGroupCode,
		ChannelName:              in.ChannelName,
		ChannelCode:              in.ChannelCode,
		ChannelType:              in.ChannelType,
		UsageScope:               in.UsageScope,
		IsDefault:                in.IsDefault,
		AllowUseGlobalCredential: in.AllowUseGlobalCredential,
		ProjectCredentialId:      in.ProjectCredentialID,
		BindingConfig:            in.BindingConfig,
		Status:                   in.Status,
		HealthStatus:             in.HealthStatus,
		LastCheckAt:              in.LastCheckAt,
		LastCheckMessage:         in.LastCheckMessage,
		Metadata:                 in.Metadata,
		CreatedBy:                in.CreatedBy,
		UpdatedBy:                in.UpdatedBy,
		CreatedAt:                in.CreateAt.Unix(),
		UpdatedAt:                in.UpdateAt.Unix(),
	}
}

func channelToPb(in *model.DevopsChannel) *pb.DevopsChannel {
	if in == nil {
		return nil
	}
	return &pb.DevopsChannel{
		Id:                 in.ID.Hex(),
		GroupId:            in.GroupID,
		Name:               in.Name,
		Code:               in.Code,
		ChannelType:        in.ChannelType,
		Endpoint:           in.Endpoint,
		Description:        in.Description,
		GlobalCredentialId: in.GlobalCredentialID,
		Config:             in.Config,
		Labels:             in.Labels,
		HealthStatus:       in.HealthStatus,
		LastCheckAt:        in.LastCheckAt,
		LastCheckMessage:   in.LastCheckMessage,
		Status:             in.Status,
		CreatedBy:          in.CreatedBy,
		UpdatedBy:          in.UpdatedBy,
		CreatedAt:          in.CreateAt.Unix(),
		UpdatedAt:          in.UpdateAt.Unix(),
		IsSystem:           in.IsSystem,
		AuthType:           in.AuthType,
		Username:           in.Username,
		Password:           in.Password,
		Token:              in.Token,
		InsecureSkipTls:    in.InsecureSkipTLS,
		CredentialId:       in.CredentialID,
		Metadata:           in.Metadata,
		Icon:               in.Icon,
		IconColor:          in.IconColor,
	}
}

func credentialToPb(in *model.DevopsCredential, revealSecret bool) (*pb.DevopsCredential, error) {
	if in == nil {
		return nil, nil
	}
	out := &pb.DevopsCredential{
		Id:             in.ID.Hex(),
		Name:           in.Name,
		Code:           in.Code,
		CredentialType: in.CredentialType,
		Description:    in.Description,
		Status:         in.Status,
		IsSystem:       in.IsSystem,
		Scope:          in.Scope,
		ProjectId:      in.ProjectID,
		CreatedBy:      in.CreatedBy,
		UpdatedBy:      in.UpdatedBy,
		CreatedAt:      in.CreateAt.Unix(),
		UpdatedAt:      in.UpdateAt.Unix(),
	}
	if !revealSecret {
		out.Username = in.Username
		return out, nil
	}
	secret, err := in.Secret()
	if err != nil {
		logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
		return nil, err
	}
	out.Username = secret.Username
	out.Password = secret.Password
	out.Token = secret.Token
	out.PrivateKey = secret.PrivateKey
	out.Passphrase = secret.Passphrase
	out.Kubeconfig = secret.Kubeconfig
	out.SecretText = secret.SecretText
	out.Certificate = secret.Certificate
	out.JsonData = secret.JsonData
	return out, nil
}

func stepParamsFromPb(items []*pb.DevopsStepParam) []model.DevopsStepParam {
	result := make([]model.DevopsStepParam, 0, len(items))
	for index, item := range items {
		if item == nil {
			continue
		}
		sortOrder := item.SortOrder
		if sortOrder <= 0 {
			sortOrder = int64(index + 1)
		}
		config := stepParamConfigFromPb(item.Config)
		mergeLegacyParamConfig(&config, item)
		runtimeMode := normalizeRuntimeMode(item.RuntimeMode, item.Mode)
		readonly := item.Readonly
		runtimeConfig := item.RuntimeConfig
		if strings.TrimSpace(item.ParamType) == channelvars.ParamChannelCredential {
			readonly = true
			runtimeConfig = false
		}
		result = append(result, model.DevopsStepParam{
			Name:             strings.TrimSpace(item.Name),
			Code:             strings.TrimSpace(item.Code),
			DefaultValue:     item.DefaultValue,
			ParamType:        strings.TrimSpace(item.ParamType),
			Mode:             normalizeParamMode(item.Mode),
			Required:         item.Required,
			Readonly:         readonly,
			Description:      item.Description,
			SelectList:       config.Options,
			AllowCustom:      item.AllowCustom,
			ChannelGroupCode: config.ChannelGroupCode,
			MappingField:     config.MappingField,
			VoucherModel:     config.VoucherModel,
			SortOrder:        sortOrder,
			RuntimeMode:      runtimeMode,
			RuntimeConfig:    runtimeConfig,
			Config:           config,
		})
	}
	normalizeStepParams(result)
	return result
}

func compoundFieldsFromPb(items []*pb.CompoundParamField) []model.DevopsCompoundParamField {
	result := make([]model.DevopsCompoundParamField, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		config := stepParamConfigFromPb(item.Config)
		result = append(result, model.DevopsCompoundParamField{
			Name:         strings.TrimSpace(item.Name),
			Code:         strings.TrimSpace(item.Code),
			DefaultValue: item.DefaultValue,
			ParamType:    strings.TrimSpace(item.ParamType),
			Mode:         normalizeParamMode(item.Mode),
			Required:     item.Required,
			Readonly:     item.Readonly,
			Description:  item.Description,
			SelectList:   config.Options,
			AllowCustom:  item.AllowCustom,
			RuntimeMode:  normalizeRuntimeMode(item.RuntimeMode, item.Mode),
			Config:       config,
		})
	}
	normalizeCompoundFields(result)
	return result
}

func normalizeStepParams(items []model.DevopsStepParam) {
	sort.SliceStable(items, func(i, j int) bool {
		left := items[i].SortOrder
		right := items[j].SortOrder
		if left <= 0 {
			left = int64(i + 1)
		}
		if right <= 0 {
			right = int64(j + 1)
		}
		return left < right
	})
	for idx := range items {
		if items[idx].SortOrder <= 0 {
			items[idx].SortOrder = int64(idx + 1)
		}
		items[idx].Config = normalizeStepParamConfig(items[idx])
		normalizeCompoundFields(items[idx].Config.CompoundFields)
	}
}

func normalizeCompoundFields(items []model.DevopsCompoundParamField) {
	for idx := range items {
		items[idx].Config = normalizeCompoundFieldConfig(items[idx])
	}
}

func compoundFieldsToPb(items []model.DevopsCompoundParamField) []*pb.CompoundParamField {
	result := make([]*pb.CompoundParamField, 0, len(items))
	for _, item := range items {
		config := normalizeCompoundFieldConfig(item)
		result = append(result, &pb.CompoundParamField{
			Name:         item.Name,
			Code:         item.Code,
			DefaultValue: item.DefaultValue,
			ParamType:    item.ParamType,
			Mode:         item.Mode,
			Required:     item.Required,
			Readonly:     item.Readonly,
			Description:  item.Description,
			SelectList:   stepParamOptionsToPb(config.Options),
			AllowCustom:  item.AllowCustom,
			RuntimeMode:  normalizeRuntimeMode(item.RuntimeMode, item.Mode),
			Config:       stepParamConfigToPb(config),
		})
	}
	return result
}

func stepParamsToPb(items []model.DevopsStepParam) []*pb.DevopsStepParam {
	result := make([]*pb.DevopsStepParam, 0, len(items))
	for _, item := range items {
		result = append(result, &pb.DevopsStepParam{
			Name:             item.Name,
			Code:             item.Code,
			DefaultValue:     item.DefaultValue,
			ParamType:        item.ParamType,
			Mode:             item.Mode,
			Required:         item.Required,
			Readonly:         item.Readonly,
			Description:      item.Description,
			SelectList:       stepParamOptionsToPb(normalizeStepParamConfig(item).Options),
			AllowCustom:      item.AllowCustom,
			ChannelGroupCode: item.ChannelGroupCode,
			MappingField:     item.MappingField,
			VoucherModel:     item.VoucherModel,
			SortOrder:        item.SortOrder,
			RuntimeMode:      normalizeRuntimeMode(item.RuntimeMode, item.Mode),
			RuntimeConfig:    item.RuntimeConfig,
			Config:           stepParamConfigToPb(normalizeStepParamConfig(item)),
		})
	}
	return result
}

func tektonTaskParamsToPb(items []model.DevopsTektonTaskParam) []*pb.DevopsTektonTaskParam {
	result := make([]*pb.DevopsTektonTaskParam, 0, len(items))
	for _, item := range items {
		result = append(result, &pb.DevopsTektonTaskParam{
			Name:         item.Name,
			Type:         normalizeTektonContractType(item.Type),
			DefaultValue: item.DefaultValue,
			Description:  item.Description,
			Enum:         item.Enum,
			Properties:   tektonPropertiesToPb(item.Properties),
			Required:     item.Required,
		})
	}
	return result
}

func tektonTaskResultsToPb(items []model.DevopsTektonTaskResult) []*pb.DevopsTektonTaskResult {
	result := make([]*pb.DevopsTektonTaskResult, 0, len(items))
	for _, item := range items {
		result = append(result, &pb.DevopsTektonTaskResult{
			Name:        item.Name,
			Type:        normalizeTektonContractType(item.Type),
			Description: item.Description,
			Value:       item.Value,
			Properties:  tektonPropertiesToPb(item.Properties),
		})
	}
	return result
}

func tektonWorkspacesToPb(items []model.DevopsTektonWorkspace) []*pb.DevopsTektonWorkspace {
	result := make([]*pb.DevopsTektonWorkspace, 0, len(items))
	for _, item := range items {
		result = append(result, &pb.DevopsTektonWorkspace{
			Name:        item.Name,
			Description: item.Description,
			Optional:    item.Optional,
			ReadOnly:    item.ReadOnly,
			MountPath:   item.MountPath,
		})
	}
	return result
}

func tektonPropertiesToPb(items map[string]model.DevopsTektonPropertySpec) map[string]*pb.TektonPropertySpec {
	if len(items) == 0 {
		return nil
	}
	result := make(map[string]*pb.TektonPropertySpec, len(items))
	for name, item := range items {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		result[name] = &pb.TektonPropertySpec{Type: normalizeTektonContractType(item.Type)}
	}
	return result
}

func tektonTaskContractsFromContent(content string) ([]model.DevopsTektonTaskParam, []model.DevopsTektonTaskResult, []model.DevopsTektonWorkspace, error) {
	resource, err := devopstekton.ParseStepResource(content)
	if err != nil {
		logx.Errorf("解析 Tekton Task 契约失败: %v", err)
		return nil, nil, nil, err
	}
	obj := resource.Object.Object
	params, err := tektonTaskParamsFromObject(obj)
	if err != nil {
		return nil, nil, nil, err
	}
	results, err := tektonTaskResultsFromObject(obj)
	if err != nil {
		return nil, nil, nil, err
	}
	workspaces, err := tektonWorkspacesFromObject(obj)
	if err != nil {
		return nil, nil, nil, err
	}
	return params, results, workspaces, nil
}

func mergeTektonTaskParamContracts(items []model.DevopsTektonTaskParam, overrides []*pb.DevopsTektonTaskParam) []model.DevopsTektonTaskParam {
	if len(items) == 0 || len(overrides) == 0 {
		return items
	}
	overrideByName := make(map[string]*pb.DevopsTektonTaskParam, len(overrides))
	for _, item := range overrides {
		if item == nil {
			continue
		}
		name := strings.TrimSpace(item.Name)
		if name != "" {
			overrideByName[name] = item
		}
	}
	for index := range items {
		override := overrideByName[items[index].Name]
		if override == nil {
			continue
		}
		items[index].Required = override.Required
	}
	return items
}

func mergeTektonTaskParamStepContracts(items []model.DevopsTektonTaskParam, overrides []*pb.DevopsStepParam) []model.DevopsTektonTaskParam {
	if len(items) == 0 || len(overrides) == 0 {
		return items
	}
	overrideByCode := make(map[string]*pb.DevopsStepParam, len(overrides))
	for _, item := range overrides {
		if item == nil {
			continue
		}
		code := strings.TrimSpace(item.Code)
		if code != "" {
			overrideByCode[code] = item
		}
	}
	for index := range items {
		override := overrideByCode[items[index].Name]
		if override == nil {
			continue
		}
		items[index].Required = override.Required
	}
	return items
}

func tektonTaskParamsFromObject(obj map[string]any) ([]model.DevopsTektonTaskParam, error) {
	rawParams, _, err := unstructured.NestedSlice(obj, "spec", "params")
	if err != nil {
		logx.Errorf("读取 Tekton Task 参数契约失败: %v", err)
		return nil, err
	}
	result := make([]model.DevopsTektonTaskParam, 0, len(rawParams))
	for _, item := range rawParams {
		data, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(fmt.Sprint(data["name"]))
		if name == "" {
			continue
		}
		_, hasDefault := data["default"]
		result = append(result, model.DevopsTektonTaskParam{
			Name:         name,
			Type:         normalizeTektonContractType(fmt.Sprint(data["type"])),
			DefaultValue: tektonContractValueText(data["default"]),
			Description:  strings.TrimSpace(fmt.Sprint(data["description"])),
			Enum:         tektonContractStringSlice(data["enum"]),
			Properties:   tektonContractProperties(data["properties"]),
			Required:     !hasDefault,
		})
	}
	return result, nil
}

func tektonTaskResultsFromObject(obj map[string]any) ([]model.DevopsTektonTaskResult, error) {
	rawResults, _, err := unstructured.NestedSlice(obj, "spec", "results")
	if err != nil {
		logx.Errorf("读取 Tekton Task Result 契约失败: %v", err)
		return nil, err
	}
	result := make([]model.DevopsTektonTaskResult, 0, len(rawResults))
	for _, item := range rawResults {
		data, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(fmt.Sprint(data["name"]))
		if name == "" {
			continue
		}
		result = append(result, model.DevopsTektonTaskResult{
			Name:        name,
			Type:        normalizeTektonContractType(fmt.Sprint(data["type"])),
			Description: strings.TrimSpace(fmt.Sprint(data["description"])),
			Value:       tektonContractValueText(data["value"]),
			Properties:  tektonContractProperties(data["properties"]),
		})
	}
	return result, nil
}

func tektonWorkspacesFromObject(obj map[string]any) ([]model.DevopsTektonWorkspace, error) {
	rawWorkspaces, _, err := unstructured.NestedSlice(obj, "spec", "workspaces")
	if err != nil {
		logx.Errorf("读取 Tekton Task Workspace 契约失败: %v", err)
		return nil, err
	}
	result := make([]model.DevopsTektonWorkspace, 0, len(rawWorkspaces))
	for _, item := range rawWorkspaces {
		data, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(fmt.Sprint(data["name"]))
		if name == "" {
			continue
		}
		result = append(result, model.DevopsTektonWorkspace{
			Name:        name,
			Description: strings.TrimSpace(fmt.Sprint(data["description"])),
			Optional:    data["optional"] == true,
			ReadOnly:    data["readOnly"] == true,
			MountPath:   strings.TrimSpace(fmt.Sprint(data["mountPath"])),
		})
	}
	return result, nil
}

func stepParamsFromTektonTaskParams(items []model.DevopsTektonTaskParam, legacy []model.DevopsStepParam) []model.DevopsStepParam {
	legacyByCode := make(map[string]model.DevopsStepParam, len(legacy))
	for _, item := range legacy {
		code := strings.TrimSpace(item.Code)
		if code != "" {
			legacyByCode[code] = item
		}
	}
	result := make([]model.DevopsStepParam, 0, len(items))
	for index, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		old := legacyByCode[name]
		options := tektonEnumOptions(item.Enum)
		config := normalizeStepParamConfig(model.DevopsStepParam{
			ParamType:  normalizeTektonContractType(item.Type),
			SelectList: options,
			Config: model.DevopsStepParamConfig{
				Options:        options,
				CompoundFields: tektonCompoundFieldsFromProperties(item.Properties),
			},
		})
		displayName := strings.TrimSpace(old.Name)
		if displayName == "" {
			displayName = strings.TrimSpace(item.Description)
		}
		if displayName == "" {
			displayName = name
		}
		result = append(result, model.DevopsStepParam{
			Name:          displayName,
			Code:          name,
			DefaultValue:  item.DefaultValue,
			ParamType:     normalizeTektonContractType(item.Type),
			Mode:          "params",
			Required:      item.Required,
			Readonly:      false,
			Description:   strings.TrimSpace(item.Description),
			SelectList:    options,
			AllowCustom:   true,
			SortOrder:     int64(index + 1),
			RuntimeMode:   "params",
			RuntimeConfig: false,
			Config:        config,
		})
	}
	return result
}

func tektonEnumOptions(items []string) []model.DevopsStepParamOption {
	result := make([]model.DevopsStepParamOption, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, model.DevopsStepParamOption{Label: value, Value: value})
	}
	return result
}

func tektonCompoundFieldsFromProperties(items map[string]model.DevopsTektonPropertySpec) []model.DevopsCompoundParamField {
	result := make([]model.DevopsCompoundParamField, 0, len(items))
	names := make([]string, 0, len(items))
	for name := range items {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		property := items[name]
		result = append(result, model.DevopsCompoundParamField{
			Name:        name,
			Code:        name,
			ParamType:   normalizeTektonContractType(property.Type),
			Mode:        "params",
			RuntimeMode: "params",
		})
	}
	return result
}

func tektonContractValueText(raw any) string {
	if raw == nil {
		return ""
	}
	if value, ok := raw.(string); ok {
		return value
	}
	data, err := yamlv3.Marshal(raw)
	if err != nil {
		return fmt.Sprint(raw)
	}
	return strings.TrimSpace(string(data))
}

func tektonContractStringSlice(raw any) []string {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		value := strings.TrimSpace(fmt.Sprint(item))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func tektonContractProperties(raw any) map[string]model.DevopsTektonPropertySpec {
	data, ok := raw.(map[string]any)
	if !ok || len(data) == 0 {
		return nil
	}
	result := make(map[string]model.DevopsTektonPropertySpec, len(data))
	for key, value := range data {
		name := strings.TrimSpace(key)
		if name == "" {
			continue
		}
		property, _ := value.(map[string]any)
		result[name] = model.DevopsTektonPropertySpec{
			Type: normalizeTektonContractType(fmt.Sprint(property["type"])),
		}
	}
	return result
}

func normalizeTektonContractType(value string) string {
	switch strings.TrimSpace(value) {
	case "array":
		return "array"
	case "object":
		return "object"
	default:
		return "string"
	}
}

func stepParamConfigFromPb(in *pb.StepParamConfig) model.DevopsStepParamConfig {
	if in == nil {
		return model.DevopsStepParamConfig{}
	}
	return model.DevopsStepParamConfig{
		Options:                   stepParamOptionsFromPb(in.Options),
		ChannelGroupCode:          strings.TrimSpace(in.ChannelGroupCode),
		ChannelTypeFilter:         strings.TrimSpace(in.ChannelTypeFilter),
		ChannelParamCode:          strings.TrimSpace(in.ChannelParamCode),
		ChannelBindingID:          strings.TrimSpace(in.ChannelBindingId),
		MappingField:              strings.TrimSpace(in.MappingField),
		VoucherModel:              strings.TrimSpace(in.VoucherModel),
		CredentialID:              strings.TrimSpace(in.CredentialId),
		CredentialMode:            strings.TrimSpace(in.CredentialMode),
		CredentialSourceParamCode: strings.TrimSpace(in.CredentialSourceParamCode),
		ProjectParamCode:          strings.TrimSpace(in.ProjectParamCode),
		ComponentParamCode:        strings.TrimSpace(in.ComponentParamCode),
		Provider:                  strings.TrimSpace(in.Provider),
		ValidateRemote:            in.ValidateRemote,
		CompoundFields:            compoundFieldsFromPb(in.CompoundFields),
		FullRow:                   in.FullRow,
		RenderMode:                strings.TrimSpace(in.RenderMode),
		ConfigTypeID:              strings.TrimSpace(in.ConfigTypeId),
		ConfigTypeCode:            strings.TrimSpace(in.ConfigTypeCode),
		ValueMode:                 strings.TrimSpace(in.ValueMode),
		DependencyParamCodes:      stepParamDependenciesFromPb(in.DependencyParamCodes),
	}
}

func stepParamConfigToPb(in model.DevopsStepParamConfig) *pb.StepParamConfig {
	return &pb.StepParamConfig{
		Options:                   stepParamOptionsToPb(in.Options),
		ChannelGroupCode:          in.ChannelGroupCode,
		ChannelTypeFilter:         in.ChannelTypeFilter,
		ChannelParamCode:          in.ChannelParamCode,
		ChannelBindingId:          in.ChannelBindingID,
		MappingField:              in.MappingField,
		VoucherModel:              in.VoucherModel,
		CredentialId:              in.CredentialID,
		CredentialMode:            strings.TrimSpace(in.CredentialMode),
		CredentialSourceParamCode: in.CredentialSourceParamCode,
		ProjectParamCode:          in.ProjectParamCode,
		ComponentParamCode:        in.ComponentParamCode,
		Provider:                  in.Provider,
		ValidateRemote:            in.ValidateRemote,
		CompoundFields:            compoundFieldsToPb(in.CompoundFields),
		FullRow:                   in.FullRow,
		RenderMode:                in.RenderMode,
		ConfigTypeId:              in.ConfigTypeID,
		ConfigTypeCode:            in.ConfigTypeCode,
		ValueMode:                 strings.TrimSpace(in.ValueMode),
		DependencyParamCodes:      stepParamDependenciesToPb(in.DependencyParamCodes),
	}
}

func stepParamDependenciesFromPb(items []*pb.StepParamDependency) []model.DevopsStepParamDependency {
	result := make([]model.DevopsStepParamDependency, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		field := strings.TrimSpace(item.Field)
		paramCode := strings.TrimSpace(item.ParamCode)
		if field == "" || paramCode == "" {
			continue
		}
		result = append(result, model.DevopsStepParamDependency{Field: field, ParamCode: paramCode})
	}
	return result
}

func stepParamDependenciesToPb(items []model.DevopsStepParamDependency) []*pb.StepParamDependency {
	result := make([]*pb.StepParamDependency, 0, len(items))
	for _, item := range items {
		field := strings.TrimSpace(item.Field)
		paramCode := strings.TrimSpace(item.ParamCode)
		if field == "" || paramCode == "" {
			continue
		}
		result = append(result, &pb.StepParamDependency{Field: field, ParamCode: paramCode})
	}
	return result
}

func mergeLegacyParamConfig(config *model.DevopsStepParamConfig, item *pb.DevopsStepParam) {
	if len(config.Options) == 0 {
		config.Options = stepParamOptionsFromPb(item.SelectList)
	}
	if config.ChannelGroupCode == "" {
		config.ChannelGroupCode = strings.TrimSpace(item.ChannelGroupCode)
	}
	if config.ChannelTypeFilter == "" && channelvars.IsChannelParamType(strings.TrimSpace(item.ParamType)) {
		config.ChannelTypeFilter = channelvars.ChannelTypeFilter(strings.TrimSpace(item.ParamType))
	}
	if config.MappingField == "" {
		config.MappingField = strings.TrimSpace(item.MappingField)
	}
	if config.VoucherModel == "" {
		config.VoucherModel = strings.TrimSpace(item.VoucherModel)
	}
	if strings.TrimSpace(item.ParamType) == channelvars.ParamChannelCredential {
		config.CredentialMode = normalizeChannelCredentialMode(config.CredentialMode, config.MappingField)
		if config.CredentialMode != credentialModeField {
			config.MappingField = ""
		}
	} else {
		config.CredentialMode = normalizeCredentialMode(config.CredentialMode, config.MappingField)
		if config.CredentialMode == credentialModeJenkinsID && config.MappingField == "credentialId" {
			config.MappingField = defaultCredentialMappingField(config.VoucherModel)
		}
	}
}

func normalizeStepParamConfig(item model.DevopsStepParam) model.DevopsStepParamConfig {
	config := item.Config
	if len(config.Options) == 0 {
		config.Options = item.SelectList
	}
	if config.ChannelGroupCode == "" {
		config.ChannelGroupCode = item.ChannelGroupCode
	}
	if config.ChannelTypeFilter == "" && channelvars.IsChannelParamType(item.ParamType) {
		config.ChannelTypeFilter = channelvars.ChannelTypeFilter(item.ParamType)
	}
	if config.MappingField == "" {
		config.MappingField = item.MappingField
	}
	if config.VoucherModel == "" {
		config.VoucherModel = item.VoucherModel
	}
	if item.ParamType == channelvars.ParamChannelCredential {
		config.CredentialMode = normalizeChannelCredentialMode(config.CredentialMode, config.MappingField)
		config.VoucherModel = ""
		config.CredentialID = ""
		if config.CredentialMode != credentialModeField {
			config.MappingField = ""
		}
	} else {
		config.CredentialMode = normalizeCredentialMode(config.CredentialMode, config.MappingField)
		if config.CredentialMode == credentialModeJenkinsID && config.MappingField == "credentialId" {
			config.MappingField = defaultCredentialMappingField(config.VoucherModel)
		}
	}
	if item.ParamType == channelvars.ParamHostGroupTargets && config.RenderMode == "" {
		config.RenderMode = "normal"
	}
	if item.ParamType == channelvars.ParamConfigCenter && config.ValueMode == "" {
		config.ValueMode = "string"
	}
	if item.ParamType == channelvars.ParamKubeNovaDeployConfig || item.ParamType == channelvars.ParamKubernetesDeployConfig {
		config.ValueMode = normalizeDeployConfigValueMode(config.ValueMode)
		config.FullRow = true
	}
	if channelvars.IsChannelParamType(item.ParamType) {
		if config.ChannelGroupCode == "" {
			config.ChannelGroupCode = channelvars.ChannelGroupCode(item.ParamType)
		}
		if config.ChannelTypeFilter == "" {
			config.ChannelTypeFilter = channelvars.ChannelTypeFilter(item.ParamType)
		}
		if config.MappingField == "" {
			switch item.ParamType {
			case channelvars.ParamKubeNovaDeployConfig:
				config.MappingField = channelvars.FieldAddressDeployConfig
			case channelvars.ParamKubernetesDeployConfig:
				config.MappingField = channelvars.FieldAddressDeploymentConfig
			default:
				config.MappingField = channelvars.FieldEndpoint
			}
		}
	}
	config.MappingField = normalizeChannelVariableMappingField(config.ChannelGroupCode, config.ChannelTypeFilter, config.MappingField)
	return config
}

func normalizeCompoundFieldConfig(item model.DevopsCompoundParamField) model.DevopsStepParamConfig {
	config := item.Config
	if len(config.Options) == 0 {
		config.Options = item.SelectList
	}
	config.CredentialMode = normalizeCredentialMode(config.CredentialMode, config.MappingField)
	if config.CredentialMode == credentialModeJenkinsID && config.MappingField == "credentialId" {
		config.MappingField = defaultCredentialMappingField(config.VoucherModel)
	}
	if item.ParamType == channelvars.ParamHostGroupTargets && config.RenderMode == "" {
		config.RenderMode = "normal"
	}
	if item.ParamType == channelvars.ParamConfigCenter && config.ValueMode == "" {
		config.ValueMode = "string"
	}
	if item.ParamType == channelvars.ParamKubeNovaDeployConfig || item.ParamType == channelvars.ParamKubernetesDeployConfig {
		config.ValueMode = normalizeDeployConfigValueMode(config.ValueMode)
		config.FullRow = true
	}
	if channelvars.IsChannelParamType(item.ParamType) {
		if config.ChannelGroupCode == "" {
			config.ChannelGroupCode = channelvars.ChannelGroupCode(item.ParamType)
		}
		if config.ChannelTypeFilter == "" {
			config.ChannelTypeFilter = channelvars.ChannelTypeFilter(item.ParamType)
		}
		if config.MappingField == "" {
			switch item.ParamType {
			case channelvars.ParamKubeNovaDeployConfig:
				config.MappingField = channelvars.FieldAddressDeployConfig
			case channelvars.ParamKubernetesDeployConfig:
				config.MappingField = channelvars.FieldAddressDeploymentConfig
			default:
				config.MappingField = channelvars.FieldEndpoint
			}
		}
	}
	config.MappingField = normalizeChannelVariableMappingField(config.ChannelGroupCode, config.ChannelTypeFilter, config.MappingField)
	return config
}

func normalizeChannelVariableMappingField(groupCode, channelType, mappingField string) string {
	field := strings.TrimSpace(mappingField)
	if strings.TrimSpace(groupCode) == channelvars.GroupImageRepo || isImageRegistryChannelType(channelType) {
		switch field {
		case channelvars.FieldDynamicRegistry, channelvars.FieldDynamicRepository:
			return channelvars.FieldDynamicProject
		case channelvars.FieldAddressRegistryURL:
			return channelvars.FieldAddressProjectURL
		}
	}
	if strings.TrimSpace(groupCode) == channelvars.GroupArtifactRepo || isArtifactRepositoryChannelType(channelType) {
		switch field {
		case channelvars.FieldDynamicArtifact:
			return channelvars.FieldDynamicArtifactName
		case channelvars.FieldDynamicArtifactTag, channelvars.FieldDynamicTag:
			return channelvars.FieldDynamicArtifactVersion
		case channelvars.FieldAddressArtifactTagURL:
			return channelvars.FieldAddressArtifactVersionURL
		}
	}
	if strings.TrimSpace(channelType) == "kubernetes" {
		switch field {
		case channelvars.FieldDynamicResource:
			return channelvars.FieldDynamicResourceName
		case channelvars.FieldDynamicContainer:
			return channelvars.FieldDynamicContainerName
		case channelvars.FieldDynamicImage:
			return channelvars.FieldDynamicImageName
		case channelvars.FieldDynamicDeployConfig:
			return channelvars.FieldAddressDeploymentConfig
		}
	}
	if strings.TrimSpace(channelType) == "kube-nova" && field == channelvars.FieldDynamicDeployConfig {
		return channelvars.FieldAddressDeployConfig
	}
	return field
}

func isImageRegistryChannelType(channelType string) bool {
	switch strings.TrimSpace(channelType) {
	case "harbor", "registry", "aliyun_registry":
		return true
	default:
		return false
	}
}

func isArtifactRepositoryChannelType(channelType string) bool {
	switch strings.TrimSpace(channelType) {
	case "nexus", "jfrog":
		return true
	default:
		return false
	}
}

func imageRegistryAddressFields() []string {
	return []string{
		channelvars.FieldAddressProjectURL,
		channelvars.FieldAddressImageURL,
		channelvars.FieldAddressImageTagURL,
	}
}

func artifactRepositoryAddressFields() []string {
	return []string{
		channelvars.FieldAddressRepositoryURL,
		channelvars.FieldAddressArtifactURL,
		channelvars.FieldAddressArtifactVersionURL,
		channelvars.FieldAddressArtifactTagURL,
	}
}

func artifactConfigFromPb(in *pb.StepArtifactConfig) model.DevopsStepArtifactConfig {
	if in == nil {
		return model.DevopsStepArtifactConfig{}
	}
	return model.DevopsStepArtifactConfig{
		Enabled:     in.Enabled,
		Type:        strings.TrimSpace(in.Type),
		Name:        strings.TrimSpace(in.Name),
		Path:        strings.TrimSpace(in.Path),
		Required:    in.Required,
		ContentType: strings.TrimSpace(in.ContentType),
	}
}

func artifactConfigToPb(in model.DevopsStepArtifactConfig) *pb.StepArtifactConfig {
	return &pb.StepArtifactConfig{
		Enabled:     in.Enabled,
		Type:        in.Type,
		Name:        in.Name,
		Path:        in.Path,
		Required:    in.Required,
		ContentType: in.ContentType,
	}
}

func validateStepArtifactConfig(config model.DevopsStepArtifactConfig) error {
	if !config.Enabled {
		return nil
	}
	if strings.TrimSpace(config.Path) == "" {
		logx.Errorf("产物配置已启用时必须填写文件路径")
		return errorx.Msg("产物配置已启用时必须填写文件路径")
	}
	artifactType := strings.TrimSpace(config.Type)
	if artifactType == "" {
		return nil
	}
	switch artifactType {
	case "report", "artifact":
		return nil
	default:
		logx.Errorf("产物类型不支持")
		return errorx.Msg("产物类型不支持")
	}
}

func stepParamOptionsFromPb(items []*pb.StepParamOption) []model.DevopsStepParamOption {
	result := make([]model.DevopsStepParamOption, 0, len(items))
	for _, item := range items {
		if item == nil || strings.TrimSpace(item.Value) == "" {
			continue
		}
		result = append(result, model.DevopsStepParamOption{
			Label: strings.TrimSpace(item.Label),
			Value: strings.TrimSpace(item.Value),
		})
	}
	return result
}

func stepParamOptionsToPb(items []model.DevopsStepParamOption) []*pb.StepParamOption {
	result := make([]*pb.StepParamOption, 0, len(items))
	for _, item := range items {
		result = append(result, &pb.StepParamOption{Label: item.Label, Value: item.Value})
	}
	return result
}

func pipelineStepsFromPb(items []*pb.PipelineTemplateStep) []model.PipelineTemplateStep {
	result := make([]model.PipelineTemplateStep, 0, len(items))
	for idx, item := range items {
		if item == nil || strings.TrimSpace(item.StepId) == "" {
			continue
		}
		sortOrder := item.SortOrder
		if sortOrder == 0 {
			sortOrder = int64(idx + 1)
		}
		result = append(result, model.PipelineTemplateStep{
			ID:             strings.TrimSpace(item.Id),
			StepID:         strings.TrimSpace(item.StepId),
			StepCode:       strings.TrimSpace(item.StepCode),
			StepName:       strings.TrimSpace(item.StepName),
			NodeName:       strings.TrimSpace(item.NodeName),
			ContainerName:  strings.TrimSpace(item.ContainerName),
			StepType:       strings.TrimSpace(item.StepType),
			Icon:           strings.TrimSpace(item.Icon),
			IconColor:      strings.TrimSpace(item.IconColor),
			ParentNodeID:   strings.TrimSpace(item.ParentNodeId),
			BranchType:     strings.TrimSpace(item.BranchType),
			SortOrder:      sortOrder,
			Enabled:        item.Enabled,
			X:              item.X,
			Y:              item.Y,
			ParamValues:    "",
			ArtifactConfig: artifactConfigFromPb(item.ArtifactConfig),
		})
	}
	normalizePipelineStepOrder(result)
	return result
}

func pipelineStepsToPb(items []model.PipelineTemplateStep) []*pb.PipelineTemplateStep {
	result := make([]*pb.PipelineTemplateStep, 0, len(items))
	for _, item := range items {
		result = append(result, &pb.PipelineTemplateStep{
			Id:             item.ID,
			StepId:         item.StepID,
			StepCode:       item.StepCode,
			StepName:       item.StepName,
			NodeName:       item.NodeName,
			ContainerName:  item.ContainerName,
			StepType:       item.StepType,
			Icon:           item.Icon,
			IconColor:      item.IconColor,
			ParentNodeId:   item.ParentNodeID,
			BranchType:     item.BranchType,
			SortOrder:      item.SortOrder,
			Enabled:        item.Enabled,
			X:              item.X,
			Y:              item.Y,
			ParamValues:    item.ParamValues,
			ArtifactConfig: artifactConfigToPb(item.ArtifactConfig),
		})
	}
	return result
}

func normalizePipelineStepOrder(items []model.PipelineTemplateStep) {
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].SortOrder < items[j].SortOrder
	})
	for idx := range items {
		if items[idx].SortOrder <= 0 {
			items[idx].SortOrder = int64(idx + 1)
		}
		if items[idx].ID == "" {
			items[idx].ID = fmt.Sprintf("node-%d", idx+1)
		}
	}
}

func normalizePipelineStepGraph(items []model.PipelineTemplateStep) []model.PipelineTemplateStep {
	normalizePipelineStepOrder(items)
	if len(items) == 0 {
		return items
	}

	hasTreeFields := false
	idSet := make(map[string]struct{}, len(items))
	for idx := range items {
		items[idx].ID = strings.TrimSpace(items[idx].ID)
		if items[idx].ID == "" {
			items[idx].ID = fmt.Sprintf("node-%d", idx+1)
		}
		idSet[items[idx].ID] = struct{}{}
		if strings.TrimSpace(items[idx].ParentNodeID) != "" || normalizeBranchType(items[idx].BranchType) == branchTypeParallel {
			hasTreeFields = true
		}
		if strings.TrimSpace(items[idx].NodeName) == "" {
			items[idx].NodeName = strings.TrimSpace(items[idx].StepName)
		}
		items[idx].BranchType = normalizeBranchType(items[idx].BranchType)
	}

	if !hasTreeFields {
		parentID := workflowStartNodeID
		for idx := range items {
			items[idx].ParentNodeID = parentID
			items[idx].BranchType = branchTypeNext
			parentID = items[idx].ID
		}
		return items
	}

	for idx := range items {
		parentID := strings.TrimSpace(items[idx].ParentNodeID)
		if parentID == "" || parentID == items[idx].ID {
			parentID = workflowStartNodeID
		}
		if parentID != workflowStartNodeID && parentID != workflowPostNodeID {
			if _, ok := idSet[parentID]; !ok {
				parentID = workflowStartNodeID
			}
		}
		items[idx].ParentNodeID = parentID
		items[idx].BranchType = normalizeBranchType(items[idx].BranchType)
	}
	normalizePipelineTemplateStepBranches(items)
	return items
}

func normalizePipelineTemplateStepBranches(items []model.PipelineTemplateStep) {
	activeIDs, parallelParentByID := activePipelineTemplateParallelBranches(items)
	for idx := range items {
		if items[idx].BranchType == branchTypeParallel {
			continue
		}
		parentID := strings.TrimSpace(items[idx].ParentNodeID)
		if parallelParentID, ok := parallelParentByID[parentID]; ok {
			items[idx].ParentNodeID = parallelParentID
			items[idx].BranchType = branchTypeNext
		}
	}

	activeIDs, _ = activePipelineTemplateParallelBranches(items)
	for idx := range items {
		if _, ok := activeIDs[items[idx].ID]; ok {
			items[idx].BranchType = branchTypeParallel
			continue
		}
		items[idx].BranchType = branchTypeNext
	}
}

func activePipelineTemplateParallelBranches(items []model.PipelineTemplateStep) (map[string]struct{}, map[string]string) {
	grouped := make(map[string][]model.PipelineTemplateStep)
	for _, item := range items {
		if item.BranchType != branchTypeParallel {
			continue
		}
		parentID := strings.TrimSpace(item.ParentNodeID)
		if parentID == "" {
			parentID = workflowStartNodeID
		}
		grouped[parentID] = append(grouped[parentID], item)
	}

	activeIDs := make(map[string]struct{})
	parentByID := make(map[string]string)
	for parentID, children := range grouped {
		if len(children) < 2 {
			continue
		}
		for _, child := range children {
			if strings.TrimSpace(child.ID) == "" {
				continue
			}
			activeIDs[child.ID] = struct{}{}
			parentByID[child.ID] = parentID
		}
	}
	return activeIDs, parentByID
}

func normalizeBranchType(branchType string) string {
	branchType = strings.ToLower(strings.TrimSpace(branchType))
	if branchType == "" {
		return branchTypeNext
	}
	return branchType
}

func validateBranchType(branchType string) error {
	switch normalizeBranchType(branchType) {
	case branchTypeNext, branchTypeParallel:
		return nil
	default:
		logx.Errorf("步骤分支类型不支持")
		return errorx.Msg("步骤分支类型不支持")
	}
}

func validatePipelineCode(code, field string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		logx.Errorf("%s", field+"不能为空")
		return errorx.Msg(field + "不能为空")
	}
	if !pipelineCodePattern.MatchString(code) {
		logx.Errorf("%s", field+"只能包含字母、数字、下划线和中划线")
		return errorx.Msg(field + "只能包含字母、数字、下划线和中划线")
	}
	return nil
}

func normalizeParamMode(mode string) string {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		return "params"
	}
	return mode
}

func normalizeRuntimeMode(runtimeMode, mode string) string {
	runtimeMode = strings.TrimSpace(runtimeMode)
	mode = normalizeParamMode(mode)
	if mode == "env" {
		return "env"
	}
	if mode == "params" {
		return "params"
	}
	if runtimeMode == "params" || runtimeMode == "env" {
		return runtimeMode
	}
	if mode == "paas" {
		return "env"
	}
	return "params"
}

func normalizeStepType(stepType string) string {
	stepType = strings.TrimSpace(stepType)
	if stepType == "" {
		return stepTypeTask
	}
	return stepType
}

func validateStepType(stepType string) error {
	switch normalizeStepType(stepType) {
	case stepTypeTask, stepTypeManualApproval:
		return nil
	default:
		logx.Errorf("步骤类型不支持")
		return errorx.Msg("步骤类型不支持")
	}
}

func validateStepParams(ctx context.Context, svcCtx *svc.ServiceContext, engineType, excludeStepID string, params []model.DevopsStepParam) error {
	seen := make(map[string]struct{}, len(params))
	paramByCode := make(map[string]model.DevopsStepParam, len(params))
	normalizedParams := make([]model.DevopsStepParam, 0, len(params))
	for _, item := range params {
		if item.Name == "" {
			logx.Errorf("参数名称不能为空")
			return errorx.Msg("参数名称不能为空")
		}
		if engineType == tektonEngineType {
			if !tektonParamNamePattern.MatchString(item.Code) {
				logx.Errorf("Tekton 参数名称不合法: %s", item.Code)
				return errorx.Msg("Tekton 参数名称不合法")
			}
		} else {
			if err := validatePipelineCode(item.Code, "参数编码"); err != nil {
				logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
				return err
			}
		}
		if _, ok := seen[item.Code]; ok {
			logx.Errorf("同一步骤内参数编码不能重复")
			return errorx.Msg("同一步骤内参数编码不能重复")
		}
		seen[item.Code] = struct{}{}
		paramByCode[item.Code] = item
		item.Mode = normalizeParamMode(item.Mode)
		item.RuntimeMode = normalizeRuntimeMode(item.RuntimeMode, item.Mode)
		item.Config = normalizeStepParamConfig(item)
		paramByCode[item.Code] = item
		normalizedParams = append(normalizedParams, item)
	}
	for _, item := range normalizedParams {
		if item.Mode != "params" && item.Mode != "env" && item.Mode != "paas" {
			logx.Errorf("参数模式不支持")
			return errorx.Msg("参数模式不支持")
		}
		if !isSupportedParamType(item.ParamType) {
			logx.Errorf("参数类型不支持")
			return errorx.Msg("参数类型不支持")
		}
		if item.ParamType == objectListParamType && engineType != tektonEngineType {
			if err := validateObjectListParam(item); err != nil {
				logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
				return err
			}
			continue
		}
		if engineType == tektonEngineType {
			if err := validateTektonStepParam(ctx, svcCtx, item, paramByCode); err != nil {
				logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
				return err
			}
			continue
		}
		if err := validateParamModeType(ctx, svcCtx, item, paramByCode); err != nil {
			logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
			return err
		}
	}
	return nil
}

func validateTektonStepParam(ctx context.Context, svcCtx *svc.ServiceContext, item model.DevopsStepParam, paramByCode map[string]model.DevopsStepParam) error {
	if item.Mode != "params" || item.RuntimeMode != "params" {
		logx.Errorf("Tekton 平台参数只支持 params 映射")
		return errorx.Msg("Tekton 平台参数只支持 params 映射")
	}
	switch item.ParamType {
	case "array", "object", "string":
		return nil
	default:
		logx.Errorf("Tekton Task 参数类型不支持: %s", item.ParamType)
		return errorx.Msg("Tekton Task 参数类型只支持 string、array、object")
	}
}

func validateTektonKubernetesSecretParam(item model.DevopsStepParam, config model.DevopsStepParamConfig) error {
	if strings.TrimSpace(config.VoucherModel) == "" {
		return validateKubernetesSecretName(item.DefaultValue, item.Name)
	}
	if !isSupportedCredentialType(config.VoucherModel) {
		logx.Errorf("Kubernetes Secret 凭证类型不支持: %s", config.VoucherModel)
		return errorx.Msg("Kubernetes Secret 凭证类型不支持")
	}
	if !isSupportedTektonSecretProvider(config.Provider, config.VoucherModel) {
		logx.Errorf("Kubernetes Secret 投影类型不匹配: provider=%s credentialType=%s", config.Provider, config.VoucherModel)
		return errorx.Msg("Kubernetes Secret 投影类型与凭证类型不匹配")
	}
	return nil
}

func isSupportedCredentialType(credentialType string) bool {
	for _, item := range credentialTypeOptions() {
		if item.CredentialType == credentialType {
			return true
		}
	}
	return false
}

func isSupportedTektonSecretProvider(provider, credentialType string) bool {
	switch strings.TrimSpace(provider) {
	case "", "opaque":
		return true
	case "basic-auth":
		return credentialType == "username_password"
	case "ssh":
		return credentialType == "ssh_key"
	case "dockerconfigjson":
		return credentialType == "secret_text"
	default:
		return false
	}
}

func validateParamModeType(ctx context.Context, svcCtx *svc.ServiceContext, item model.DevopsStepParam, paramByCode map[string]model.DevopsStepParam) error {
	config := normalizeStepParamConfig(item)
	switch item.Mode {
	case "env":
		switch item.ParamType {
		case "string", "text", "booleanParam", "number":
			if err := validateParamScalarValue(item.ParamType, item.DefaultValue, item.Name); err != nil {
				logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
				return err
			}
		default:
			logx.Errorf("环境变量只支持字符串、多行文本、布尔、数字类型")
			return errorx.Msg("环境变量只支持字符串、多行文本、布尔、数字类型")
		}
	case "params":
		switch item.ParamType {
		case "string", "text", "booleanParam", "number", "password", "choice", "file":
			if item.ParamType == "choice" && len(config.Options) == 0 {
				logx.Errorf("单选参数必须配置选项")
				return errorx.Msg("单选参数必须配置选项")
			}
			if err := validateParamScalarValue(item.ParamType, item.DefaultValue, item.Name); err != nil {
				logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
				return err
			}
		case channelvars.ParamKubernetesSecretName:
			if err := validateKubernetesSecretName(item.DefaultValue, item.Name); err != nil {
				logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
				return err
			}
		case channelvars.ParamKubernetesPVCName:
			if err := validateKubernetesPVCName(item.DefaultValue, item.Name); err != nil {
				logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
				return err
			}
		default:
			logx.Errorf("执行变量只支持 Jenkins 原生参数类型")
			return errorx.Msg("执行变量只支持 Jenkins 原生参数类型")
		}
	case "paas":
		if item.RuntimeMode != "params" && item.RuntimeMode != "env" {
			logx.Errorf("平台变量映射方式不支持")
			return errorx.Msg("平台变量映射方式不支持")
		}
		switch item.ParamType {
		case "list":
			if len(config.Options) == 0 {
				logx.Errorf("平台列表必须配置选项")
				return errorx.Msg("平台列表必须配置选项")
			}
		case "channelGroupCode":
			if config.ChannelGroupCode == "" {
				logx.Errorf("渠道变量必须配置渠道分组")
				return errorx.Msg("渠道变量必须配置渠道分组")
			}
			if config.ChannelTypeFilter == "" {
				logx.Errorf("渠道变量必须配置渠道类型")
				return errorx.Msg("渠道变量必须配置渠道类型")
			}
			if config.MappingField == "" {
				logx.Errorf("渠道变量必须配置映射字段")
				return errorx.Msg("渠道变量必须配置映射字段")
			}
			if ctx != nil && svcCtx != nil {
				if err := validateChannelVariableParamSchema(ctx, svcCtx, item, config, paramByCode); err != nil {
					logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
					return err
				}
			} else if len(requiredStepChannelVariableDependencies(config)) > 0 {
				if err := validateChannelVariableStepDependencies(item, config, paramByCode); err != nil {
					logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
					return err
				}
			} else if strings.TrimSpace(config.ChannelGroupCode) == channelvars.GroupCodeRepo {
				if handled, err := validateCodeRepoChannelVariableDependencies(item, config, channelvars.VariableField{Field: config.MappingField}, paramByCode); handled && err != nil {
					logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
					return err
				}
			}
		case "voucherModel":
			if config.VoucherModel == "" {
				logx.Errorf("凭证变量必须配置凭证类型")
				return errorx.Msg("凭证变量必须配置凭证类型")
			}
			switch config.CredentialMode {
			case credentialModeJenkinsID:
				if !isJenkinsCredentialIDSupportedType(config.VoucherModel) {
					logx.Errorf("该凭证类型不支持 Jenkins credentialId 模式")
					return errorx.Msg("该凭证类型不支持 Jenkins credentialId 模式")
				}
			case credentialModeJSONData, credentialModeJSONBase64:
			default:
				if config.MappingField == "" {
					logx.Errorf("凭证变量必须配置凭证类型和映射字段")
					return errorx.Msg("凭证变量必须配置凭证类型和映射字段")
				}
				if !isCredentialMappingField(config.VoucherModel, config.MappingField) {
					logx.Errorf("凭证映射字段不匹配")
					return errorx.Msg("凭证映射字段不匹配")
				}
			}
		case channelvars.ParamChannelCredential:
			if err := validateChannelCredentialParam(item, config, paramByCode); err != nil {
				logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
				return err
			}
		case channelvars.ParamRepositoryChannel, channelvars.ParamNexusChannel, channelvars.ParamHarborChannel, channelvars.ParamJfrogChannel, channelvars.ParamKubeNovaDeployConfig, channelvars.ParamKubernetesDeployConfig:
			if item.ParamType == channelvars.ParamKubeNovaDeployConfig || item.ParamType == channelvars.ParamKubernetesDeployConfig {
				config.ValueMode = normalizeDeployConfigValueMode(config.ValueMode)
			}
			if config.MappingField == "" {
				config.MappingField = "endpoint"
			}
			if config.ChannelGroupCode == "" {
				config.ChannelGroupCode = channelvars.ChannelGroupCode(item.ParamType)
			}
			if config.ChannelTypeFilter == "" {
				config.ChannelTypeFilter = channelvars.ChannelTypeFilter(item.ParamType)
			}
			if config.ChannelGroupCode != channelvars.ChannelGroupCode(item.ParamType) {
				logx.Errorf("平台渠道变量必须使用对应渠道分组")
				return errorx.Msg("平台渠道变量必须使用对应渠道分组")
			}
			if ctx != nil && svcCtx != nil && config.ChannelTypeFilter != "" {
				if err := validateStepChannelParamTarget(ctx, svcCtx, config.ChannelGroupCode, config.ChannelTypeFilter); err != nil {
					logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
					return err
				}
			}
		case channelvars.ParamGitRepositoryURL, channelvars.ParamNexusRepositoryURL, channelvars.ParamNexusArtifactURL, channelvars.ParamNexusArtifactVersionURL, channelvars.ParamHarborProjectURL, channelvars.ParamHarborImageURL, channelvars.ParamHarborImageTagURL, channelvars.ParamRegistryRepositoryURL, channelvars.ParamRegistryImageURL, channelvars.ParamRegistryImageTagURL, channelvars.ParamJfrogRepositoryURL, channelvars.ParamJfrogArtifactURL, channelvars.ParamJfrogArtifactVersionURL, channelvars.ParamSonarProjectURL, channelvars.ParamSonarProjectKeyURL, channelvars.ParamHostConfig, channelvars.ParamHostGroupConfig:
			// 组合地址类型保存最终字符串，页面负责资源选择和拼接。
		case channelvars.ParamMavenConfig, channelvars.ParamSonarAddress:
			// 项目级动态下拉，运行时按当前项目解析。
		case channelvars.ParamConfigCenter:
			if strings.TrimSpace(config.ConfigTypeID) == "" && strings.TrimSpace(config.ConfigTypeCode) == "" {
				logx.Errorf("配置中心变量必须选择配置类型")
				return errorx.Msg("配置中心变量必须选择配置类型")
			}
			if item.RuntimeMode == "params" {
				config.ValueMode = normalizeConfigCenterValueMode(config.ValueMode)
			}
		case channelvars.ParamHostGroupTargets:
			if config.RenderMode == "" {
				config.RenderMode = "normal"
			}
			if !isSupportedHostGroupRenderMode(config.RenderMode) {
				logx.Errorf("主机组输出模式不支持")
				return errorx.Msg("主机组输出模式不支持")
			}
		case channelvars.ParamGitProject, channelvars.ParamGitBranch, channelvars.ParamGitTag, channelvars.ParamNexusRepository, channelvars.ParamNexusArtifactName, channelvars.ParamNexusArtifactVersion, channelvars.ParamNexusArtifact, channelvars.ParamHarborProject, channelvars.ParamHarborImage, channelvars.ParamHarborImageTag, channelvars.ParamRegistryRepository, channelvars.ParamRegistryImage, channelvars.ParamRegistryImageTag, channelvars.ParamJfrogRepository, channelvars.ParamJfrogArtifactName, channelvars.ParamJfrogArtifactVersion, channelvars.ParamJfrogArtifact, channelvars.ParamSonarProjectName, channelvars.ParamSonarProjectKey, channelvars.ParamKubeNovaProject, channelvars.ParamKubeNovaCluster, channelvars.ParamKubeNovaWorkspace, channelvars.ParamKubeNovaApplication, channelvars.ParamKubeNovaVersion, channelvars.ParamKubeNovaContainer, channelvars.ParamKubernetesNamespace, channelvars.ParamKubernetesWorkloadType, channelvars.ParamKubernetesResource, channelvars.ParamKubernetesContainer, channelvars.ParamKubernetesImage:
			if err := validateDynamicParamDependency(item, paramByCode); err != nil {
				logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
				return err
			}
		default:
			logx.Errorf("平台变量类型不支持")
			return errorx.Msg("平台变量类型不支持")
		}
	}
	return nil
}

func validateDynamicParamDependency(item model.DevopsStepParam, paramByCode map[string]model.DevopsStepParam) error {
	config := normalizeStepParamConfig(item)
	paramType := stepDynamicParamType(item)
	spec, ok := channelvars.DynamicSpecFor(paramType)
	if !ok {
		logx.Errorf("平台动态参数类型不支持")
		return errorx.Msg("平台动态参数类型不支持")
	}
	if strings.TrimSpace(config.ChannelParamCode) != "" {
		if err := validateDynamicChannelParam(config.ChannelParamCode, spec.ChannelParamType, spec.GroupCode, paramByCode); err != nil {
			logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
			return err
		}
	}
	if err := validateDynamicProjectParam(paramType, config, spec, paramByCode); err != nil {
		logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
		return err
	}
	return nil
}

func validateChannelVariableDynamicParamDependency(item model.DevopsStepParam, config model.DevopsStepParamConfig, paramByCode map[string]model.DevopsStepParam) error {
	paramType := stepDynamicParamType(item)
	spec, ok := channelvars.DynamicSpecFor(paramType)
	if !ok {
		logx.Errorf("平台动态参数类型不支持")
		return errorx.Msg("平台动态参数类型不支持")
	}
	if strings.TrimSpace(config.ChannelParamCode) != "" {
		if err := validateDynamicChannelParam(config.ChannelParamCode, spec.ChannelParamType, spec.GroupCode, paramByCode); err != nil {
			logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
			return err
		}
	}
	if err := validateDynamicProjectParam(paramType, config, spec, paramByCode); err != nil {
		logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
		return err
	}
	return nil
}

func validateChannelVariableParamSchema(ctx context.Context, svcCtx *svc.ServiceContext, item model.DevopsStepParam, config model.DevopsStepParamConfig, paramByCode map[string]model.DevopsStepParam) error {
	if err := validateStepChannelParamTarget(ctx, svcCtx, config.ChannelGroupCode, config.ChannelTypeFilter); err != nil {
		logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
		return err
	}
	channelType, err := svcCtx.ChannelTypeModel.FindOneByCode(ctx, config.ChannelTypeFilter)
	if err != nil {
		logx.Errorf("查询渠道类型失败: %v", err)
		return err
	}
	if strings.TrimSpace(channelType.GroupCode) != strings.TrimSpace(config.ChannelGroupCode) {
		logx.Errorf("渠道变量分组和类型不匹配")
		return errorx.Msg("渠道变量分组和类型不匹配")
	}
	if channelvars.IsLegacyVariableField(config.MappingField) {
		logx.Errorf("渠道变量映射字段使用旧协议: %s", config.MappingField)
		return errorx.Msg("渠道变量映射字段不能使用旧协议")
	}
	schema := channelvars.ParseVariableSchema(channelType.MappingFields)
	fields := append(channelvars.StandardVariableFields(channelType.GroupCode, channelType.Code), schema.Fields...)
	if strings.TrimSpace(channelType.GroupCode) == channelvars.GroupCodeRepo {
		fields = filterCodeRepoMappingFields(fields)
	}
	if strings.TrimSpace(channelType.GroupCode) == channelvars.GroupImageRepo {
		fields = filterImageRegistryMappingFields(fields)
	}
	if strings.TrimSpace(channelType.GroupCode) == channelvars.GroupCodeScan {
		fields = filterCodeScanMappingFields(fields)
	}
	if strings.TrimSpace(channelType.GroupCode) == channelvars.GroupArtifactRepo {
		fields = filterArtifactRepositoryMappingFields(fields)
	}
	if strings.TrimSpace(channelType.GroupCode) == channelvars.GroupDeployTarget {
		fields = filterDeployTargetMappingFields(channelType.Code, fields)
	}
	field, ok := channelvars.FindVariableFieldInList(channelvars.MergeVariableFields(fields), config.MappingField)
	if !ok {
		logx.Errorf("渠道变量映射字段不存在")
		return errorx.Msg("渠道变量映射字段不存在")
	}
	return validateChannelVariableSchemaDependencies(item, field, paramByCode)
}

func validateChannelVariableSchemaDependencies(item model.DevopsStepParam, field channelvars.VariableField, paramByCode map[string]model.DevopsStepParam) error {
	config := normalizeStepParamConfig(item)
	if handled, err := validateCodeRepoChannelVariableDependencies(item, config, field, paramByCode); handled {
		return err
	}
	if handled, err := validateImageRegistryChannelVariableDependencies(item, config, field, paramByCode); handled {
		return err
	}
	if handled, err := validateArtifactRepositoryChannelVariableDependencies(item, config, field, paramByCode); handled {
		return err
	}
	if requiresChannelVariableBinding(field) {
		if strings.TrimSpace(config.ChannelParamCode) == "" && strings.TrimSpace(config.ChannelBindingID) == "" {
			if !field.AllowManualInput {
				logx.Errorf("渠道变量缺少渠道来源: %s", field.Field)
				return errorx.Msg("渠道变量必须绑定渠道来源")
			}
		} else if strings.TrimSpace(config.ChannelParamCode) != "" {
			target, ok := paramByCode[strings.TrimSpace(config.ChannelParamCode)]
			if !ok || !sameChannelVariableDependency(config, channelvars.FieldEndpoint, target) {
				logx.Errorf("渠道变量渠道来源不匹配: field=%s param=%s", field.Field, config.ChannelParamCode)
				return errorx.Msg("渠道变量渠道来源必须同分组、同类型、渠道实例来源")
			}
		}
	}
	requiredDeps := channelvars.RequiredParamDependencies(field)
	if len(requiredDeps) == 0 {
		return nil
	}
	dependencyByField := map[string]string{}
	for _, dep := range config.DependencyParamCodes {
		if strings.TrimSpace(dep.Field) == "" || strings.TrimSpace(dep.ParamCode) == "" {
			continue
		}
		dependencyByField[strings.TrimSpace(dep.Field)] = strings.TrimSpace(dep.ParamCode)
	}
	for _, dep := range requiredDeps {
		depField := strings.TrimSpace(dep.Field)
		paramCode := dependencyByField[depField]
		if paramCode == "" {
			if field.AllowManualInput {
				continue
			}
			logx.Errorf("渠道变量缺少依赖字段: %s", depField)
			return errorx.Msg("渠道变量缺少依赖字段：" + dep.Name)
		}
		target, ok := paramByCode[paramCode]
		if !ok {
			logx.Errorf("渠道变量依赖参数不存在: %s", paramCode)
			return errorx.Msg("渠道变量依赖参数不存在")
		}
		if !sameChannelVariableDependency(config, depField, target) {
			logx.Errorf("渠道变量依赖参数不匹配: field=%s param=%s", depField, paramCode)
			return errorx.Msg("渠道变量依赖参数必须同分组、同类型、同字段")
		}
	}
	return nil
}

func validateCodeRepoChannelVariableDependencies(item model.DevopsStepParam, config model.DevopsStepParamConfig, field channelvars.VariableField, paramByCode map[string]model.DevopsStepParam) (bool, error) {
	if strings.TrimSpace(config.ChannelGroupCode) != channelvars.GroupCodeRepo {
		return false, nil
	}
	return validateCodeRepoChannelVariableDependencyByField(item, config, strings.TrimSpace(field.Field), paramByCode)
}

func validateCodeRepoChannelVariableDependencyByField(item model.DevopsStepParam, config model.DevopsStepParamConfig, mappingField string, paramByCode map[string]model.DevopsStepParam) (bool, error) {
	if strings.TrimSpace(config.ChannelGroupCode) != channelvars.GroupCodeRepo {
		return false, nil
	}
	switch strings.TrimSpace(mappingField) {
	case channelvars.FieldDynamicProject:
		return true, nil
	case channelvars.FieldDynamicBranch, channelvars.FieldDynamicTag:
		if hasSameChannelVariableFieldDependency(config, channelvars.FieldAddressProjectURL, paramByCode) {
			return true, nil
		}
		return true, nil
	default:
		return false, nil
	}
}

func validateImageRegistryChannelVariableDependencies(item model.DevopsStepParam, config model.DevopsStepParamConfig, field channelvars.VariableField, paramByCode map[string]model.DevopsStepParam) (bool, error) {
	if strings.TrimSpace(config.ChannelGroupCode) != channelvars.GroupImageRepo {
		return false, nil
	}
	switch strings.TrimSpace(field.Field) {
	case channelvars.FieldDynamicProject:
		return true, nil
	case channelvars.FieldDynamicImage:
		if hasAnyImageRegistryAddressDependency(config, paramByCode) {
			return true, nil
		}
		return true, nil
	case channelvars.FieldDynamicTag:
		if hasSameChannelVariableFieldDependency(config, channelvars.FieldAddressImageURL, paramByCode) ||
			hasSameChannelVariableFieldDependency(config, channelvars.FieldAddressImageTagURL, paramByCode) {
			return true, nil
		}
		return true, nil
	case channelvars.FieldDynamicImageTag:
		if hasAnyImageRegistryAddressDependency(config, paramByCode) {
			return true, nil
		}
		return true, nil
	default:
		return false, nil
	}
}

func hasAnyImageRegistryAddressDependency(config model.DevopsStepParamConfig, paramByCode map[string]model.DevopsStepParam) bool {
	for _, field := range imageRegistryAddressFields() {
		if hasSameChannelVariableFieldDependency(config, field, paramByCode) {
			return true
		}
	}
	return false
}

func validateArtifactRepositoryChannelVariableDependencies(item model.DevopsStepParam, config model.DevopsStepParamConfig, field channelvars.VariableField, paramByCode map[string]model.DevopsStepParam) (bool, error) {
	if strings.TrimSpace(config.ChannelGroupCode) != channelvars.GroupArtifactRepo {
		return false, nil
	}
	switch strings.TrimSpace(field.Field) {
	case channelvars.FieldDynamicRepository:
		return true, nil
	case channelvars.FieldDynamicArtifactName, channelvars.FieldDynamicArtifact:
		if hasAnyArtifactRepositoryAddressDependency(config, paramByCode) {
			return true, nil
		}
		return true, nil
	case channelvars.FieldDynamicArtifactVersion, channelvars.FieldDynamicArtifactTag:
		if hasAnyArtifactRepositoryAddressDependency(config, paramByCode) {
			return true, nil
		}
		return true, nil
	default:
		return false, nil
	}
}

func hasAnyArtifactRepositoryAddressDependency(config model.DevopsStepParamConfig, paramByCode map[string]model.DevopsStepParam) bool {
	for _, field := range artifactRepositoryAddressFields() {
		if hasSameChannelVariableFieldDependency(config, field, paramByCode) {
			return true
		}
	}
	return false
}

func hasChannelVariableEndpointDependency(config model.DevopsStepParamConfig, paramByCode map[string]model.DevopsStepParam) bool {
	if strings.TrimSpace(config.ChannelBindingID) != "" {
		return true
	}
	channelParamCode := strings.TrimSpace(config.ChannelParamCode)
	if channelParamCode == "" {
		return false
	}
	target, ok := paramByCode[channelParamCode]
	return ok && sameChannelVariableDependency(config, channelvars.FieldEndpoint, target)
}

func hasSameChannelVariableFieldDependency(config model.DevopsStepParamConfig, field string, paramByCode map[string]model.DevopsStepParam) bool {
	if strings.TrimSpace(config.ProjectParamCode) != "" {
		if target, ok := paramByCode[strings.TrimSpace(config.ProjectParamCode)]; ok && sameChannelVariableDependency(config, field, target) {
			return true
		}
	}
	for _, dep := range config.DependencyParamCodes {
		paramCode := strings.TrimSpace(dep.ParamCode)
		if paramCode == "" {
			continue
		}
		target, ok := paramByCode[paramCode]
		if ok && sameChannelVariableDependency(config, field, target) {
			return true
		}
	}
	return false
}

func requiresChannelVariableBinding(field channelvars.VariableField) bool {
	if channelvars.IsAddressVariableField(field) || strings.TrimSpace(field.UIControl) == "deployConfig" {
		return false
	}
	for _, dep := range field.Dependencies {
		if strings.TrimSpace(dep.Source) == "channel" && strings.TrimSpace(dep.Field) == channelvars.FieldEndpoint && dep.Required {
			return true
		}
	}
	return false
}

func sameChannelVariableDependency(source model.DevopsStepParamConfig, depField string, target model.DevopsStepParam) bool {
	targetConfig := normalizeStepParamConfig(target)
	if strings.TrimSpace(target.ParamType) != channelvars.ParamChannelVariable {
		return false
	}
	if strings.TrimSpace(targetConfig.ChannelGroupCode) != strings.TrimSpace(source.ChannelGroupCode) {
		return false
	}
	if strings.TrimSpace(targetConfig.ChannelTypeFilter) != strings.TrimSpace(source.ChannelTypeFilter) {
		return false
	}
	return strings.TrimSpace(targetConfig.MappingField) == strings.TrimSpace(depField)
}

func validateChannelVariableStepDependencies(item model.DevopsStepParam, config model.DevopsStepParamConfig, paramByCode map[string]model.DevopsStepParam) error {
	requiredFields := requiredStepChannelVariableDependencies(config)
	if len(requiredFields) == 0 {
		return nil
	}
	dependencyByField := map[string]string{}
	for _, dep := range config.DependencyParamCodes {
		if strings.TrimSpace(dep.Field) == "" || strings.TrimSpace(dep.ParamCode) == "" {
			continue
		}
		dependencyByField[strings.TrimSpace(dep.Field)] = strings.TrimSpace(dep.ParamCode)
	}
	for _, field := range requiredFields {
		if strings.TrimSpace(config.ChannelGroupCode) == channelvars.GroupCodeRepo &&
			(field == channelvars.FieldDynamicProject || field == channelvars.FieldDynamicBranch || field == channelvars.FieldDynamicTag) {
			if handled, err := validateCodeRepoChannelVariableDependencyByField(item, config, config.MappingField, paramByCode); handled {
				return err
			}
		}
		if strings.TrimSpace(config.ChannelGroupCode) == channelvars.GroupImageRepo {
			if handled, err := validateImageRegistryChannelVariableDependencies(item, config, channelvars.VariableField{Field: config.MappingField}, paramByCode); handled {
				return err
			}
		}
		if strings.TrimSpace(config.ChannelGroupCode) == channelvars.GroupArtifactRepo {
			if handled, err := validateArtifactRepositoryChannelVariableDependencies(item, config, channelvars.VariableField{Field: config.MappingField}, paramByCode); handled {
				return err
			}
		}
		paramCode := dependencyByField[field]
		if paramCode == "" {
			continue
		}
		target, ok := paramByCode[paramCode]
		if !ok || !sameChannelVariableDependency(config, field, target) {
			logx.Errorf("渠道变量依赖参数不匹配: field=%s param=%s", field, paramCode)
			return errorx.Msg("渠道变量依赖参数必须同分组、同类型、同字段")
		}
	}
	_ = item
	return nil
}

func requiredStepChannelVariableDependencies(config model.DevopsStepParamConfig) []string {
	switch strings.TrimSpace(config.MappingField) {
	case channelvars.FieldDynamicBranch, channelvars.FieldDynamicTag:
		return []string{channelvars.FieldDynamicProject}
	case channelvars.FieldDynamicArtifactName, channelvars.FieldDynamicArtifact:
		return []string{channelvars.FieldDynamicRepository}
	case channelvars.FieldDynamicArtifactVersion, channelvars.FieldDynamicArtifactTag:
		return []string{channelvars.FieldDynamicRepository, channelvars.FieldDynamicArtifactName}
	case channelvars.FieldDynamicImage:
		return []string{channelvars.FieldDynamicProject}
	case channelvars.FieldDynamicImageTag:
		return []string{channelvars.FieldDynamicProject}
	case channelvars.FieldDynamicCluster:
		return []string{channelvars.FieldDynamicProject}
	case channelvars.FieldDynamicWorkspace:
		return []string{channelvars.FieldDynamicProject, channelvars.FieldDynamicCluster}
	case channelvars.FieldDynamicApplication:
		return []string{channelvars.FieldDynamicWorkspace}
	case channelvars.FieldDynamicVersion:
		return []string{channelvars.FieldDynamicApplication}
	case channelvars.FieldDynamicContainerName, channelvars.FieldDynamicContainer:
		if strings.TrimSpace(config.ChannelTypeFilter) == "kube-nova" {
			return []string{channelvars.FieldDynamicApplication, channelvars.FieldDynamicVersion}
		}
		if strings.TrimSpace(config.ChannelTypeFilter) == "kubernetes" {
			return []string{channelvars.FieldDynamicNamespace, channelvars.FieldDynamicWorkloadType, channelvars.FieldDynamicResourceName}
		}
	case channelvars.FieldDynamicResourceName:
		return []string{channelvars.FieldDynamicNamespace, channelvars.FieldDynamicWorkloadType}
	case channelvars.FieldDynamicImageName:
		return []string{channelvars.FieldDynamicNamespace, channelvars.FieldDynamicWorkloadType, channelvars.FieldDynamicResourceName, channelvars.FieldDynamicContainerName}
	case channelvars.FieldDynamicDeployment, channelvars.FieldDynamicStatefulSet, channelvars.FieldDynamicDaemonSet, channelvars.FieldDynamicCronJob:
		return []string{channelvars.FieldDynamicNamespace}
	}
	return nil
}

func validateDynamicProjectParam(paramType string, config model.DevopsStepParamConfig, spec channelvars.DynamicSpec, paramByCode map[string]model.DevopsStepParam) error {
	if spec.RequireProject {
		if config.ProjectParamCode == "" {
			return nil
		}
		projectParam, ok := paramByCode[config.ProjectParamCode]
		projectType := stepDynamicParamType(projectParam)
		if !ok || !isAllowedDynamicProjectParamType(paramType, projectType, spec.ProjectParamType) {
			logx.Errorf("动态参数依赖的项目参数不匹配")
			return errorx.Msg("动态参数依赖的项目参数不匹配")
		}
	}
	return nil
}

func stepDynamicParamType(item model.DevopsStepParam) string {
	if strings.TrimSpace(item.ParamType) == channelvars.ParamChannelVariable {
		config := normalizeStepParamConfig(item)
		if composite := channelVariableCompositeParamType(config.ChannelTypeFilter, config.MappingField); composite != "" {
			return composite
		}
		if paramType := channelVariableDynamicParamType(config.ChannelTypeFilter, config.MappingField); paramType != "" {
			return paramType
		}
	}
	return strings.TrimSpace(item.ParamType)
}

func channelVariableCompositeParamType(channelType, mappingField string) string {
	switch strings.TrimSpace(mappingField) {
	case channelvars.FieldAddressProjectURL:
		switch strings.TrimSpace(channelType) {
		case "gitlab", "gitee", "github", "svn":
			return channelvars.ParamGitRepositoryURL
		case "sonarqube":
			return channelvars.ParamSonarProjectURL
		}
	case channelvars.FieldAddressRegistryURL:
		switch strings.TrimSpace(channelType) {
		case "harbor":
			return channelvars.ParamHarborProjectURL
		case "registry", "aliyun_registry":
			return channelvars.ParamRegistryRepositoryURL
		}
	case channelvars.FieldAddressImageURL:
		switch strings.TrimSpace(channelType) {
		case "harbor":
			return channelvars.ParamHarborImageURL
		case "registry", "aliyun_registry":
			return channelvars.ParamRegistryImageURL
		}
	case channelvars.FieldAddressImageTagURL:
		switch strings.TrimSpace(channelType) {
		case "harbor":
			return channelvars.ParamHarborImageTagURL
		case "registry", "aliyun_registry":
			return channelvars.ParamRegistryImageTagURL
		}
	case channelvars.FieldAddressRepositoryURL:
		switch strings.TrimSpace(channelType) {
		case "nexus":
			return channelvars.ParamNexusRepositoryURL
		case "jfrog":
			return channelvars.ParamJfrogRepositoryURL
		}
	case channelvars.FieldAddressArtifactURL:
		switch strings.TrimSpace(channelType) {
		case "nexus":
			return channelvars.ParamNexusArtifactURL
		case "jfrog":
			return channelvars.ParamJfrogArtifactURL
		}
	case channelvars.FieldAddressArtifactVersionURL, channelvars.FieldAddressArtifactTagURL:
		switch strings.TrimSpace(channelType) {
		case "nexus":
			return channelvars.ParamNexusArtifactVersionURL
		case "jfrog":
			return channelvars.ParamJfrogArtifactVersionURL
		}
	case channelvars.FieldAddressProjectKeyURL:
		if strings.TrimSpace(channelType) == "sonarqube" {
			return channelvars.ParamSonarProjectKeyURL
		}
	case channelvars.FieldAddressHostConfig:
		if strings.TrimSpace(channelType) == "host" {
			return channelvars.ParamHostConfig
		}
	case channelvars.FieldAddressGroupConfig:
		if strings.TrimSpace(channelType) == "host_group" {
			return channelvars.ParamHostGroupConfig
		}
	}
	return ""
}

func isAllowedDynamicProjectParamType(paramType, actualType, expectedType string) bool {
	if actualType == expectedType {
		return true
	}
	switch strings.TrimSpace(paramType) {
	case channelvars.ParamGitBranch, channelvars.ParamGitTag:
		return actualType == channelvars.ParamGitRepositoryURL
	case channelvars.ParamHarborImage, channelvars.ParamRegistryImage,
		channelvars.ParamHarborImageTag, channelvars.ParamRegistryImageTag:
		switch actualType {
		case channelvars.ParamHarborProjectURL, channelvars.ParamHarborImageURL, channelvars.ParamHarborImageTagURL,
			channelvars.ParamRegistryRepositoryURL, channelvars.ParamRegistryImageURL, channelvars.ParamRegistryImageTagURL:
			return true
		default:
			return false
		}
	case channelvars.ParamNexusArtifactName, channelvars.ParamJfrogArtifactName:
		switch actualType {
		case channelvars.ParamNexusRepositoryURL, channelvars.ParamNexusArtifactURL, channelvars.ParamNexusArtifactVersionURL,
			channelvars.ParamJfrogRepositoryURL, channelvars.ParamJfrogArtifactURL, channelvars.ParamJfrogArtifactVersionURL:
			return true
		default:
			return false
		}
	case channelvars.ParamNexusArtifactVersion, channelvars.ParamJfrogArtifactVersion:
		switch actualType {
		case channelvars.ParamNexusRepositoryURL, channelvars.ParamNexusArtifactURL, channelvars.ParamNexusArtifactVersionURL,
			channelvars.ParamJfrogRepositoryURL, channelvars.ParamJfrogArtifactURL, channelvars.ParamJfrogArtifactVersionURL:
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func validateDynamicChannelParam(code, paramType, groupCode string, paramByCode map[string]model.DevopsStepParam) error {
	if strings.TrimSpace(code) == "" {
		return nil
	}
	param, ok := paramByCode[code]
	if ok && isChannelVariableBindingParam(param, groupCode) {
		return nil
	}
	actualType := stepDynamicParamType(param)
	if !ok || actualType != paramType {
		logx.Errorf("动态参数必须依赖渠道参数")
		return errorx.Msg("动态参数必须依赖渠道参数")
	}
	if actualType == channelvars.ParamSonarAddress {
		if groupCode != channelvars.GroupCodeScan {
			logx.Errorf("动态参数依赖的渠道分组不匹配")
			return errorx.Msg("动态参数依赖的渠道分组不匹配")
		}
		return nil
	}
	config := normalizeStepParamConfig(param)
	if config.ChannelGroupCode == "" {
		config.ChannelGroupCode = channelvars.ChannelGroupCode(param.ParamType)
	}
	if config.ChannelGroupCode != groupCode {
		logx.Errorf("动态参数依赖的渠道分组不匹配")
		return errorx.Msg("动态参数依赖的渠道分组不匹配")
	}
	return nil
}

func isChannelVariableBindingParam(item model.DevopsStepParam, groupCode string) bool {
	if strings.TrimSpace(item.ParamType) != channelvars.ParamChannelVariable {
		return false
	}
	config := normalizeStepParamConfig(item)
	if strings.TrimSpace(config.ChannelGroupCode) != groupCode {
		return false
	}
	return strings.TrimSpace(config.MappingField) == channelvars.FieldEndpoint
}

func validateParamScalarValue(paramType, value, name string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	switch paramType {
	case "booleanParam":
		if !isBoolLiteral(value) {
			logx.Errorf("%s", "布尔参数默认值只能是 true 或 false："+name)
			return errorx.Msg("布尔参数默认值只能是 true 或 false：" + name)
		}
	case "number":
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			logx.Errorf("%s", "数字参数默认值必须是合法数字："+name)
			return errorx.Msg("数字参数默认值必须是合法数字：" + name)
		}
	}
	return nil
}

func validateKubernetesSecretName(value, name string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if !kubernetesSecretNamePattern.MatchString(value) {
		logx.Errorf("%s", "Kubernetes Secret 名称不合法："+name)
		return errorx.Msg("Kubernetes Secret 名称不合法：" + name)
	}
	return nil
}

func validateKubernetesPVCName(value, name string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if !kubernetesSecretNamePattern.MatchString(value) {
		logx.Errorf("%s", "Kubernetes PVC 名称不合法："+name)
		return errorx.Msg("Kubernetes PVC 名称不合法：" + name)
	}
	return nil
}

func isBoolLiteral(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "true" || value == "false"
}

func isSupportedParamType(paramType string) bool {
	switch paramType {
	case "string", "array", "object", "text", "booleanParam", "number", "password", "choice", "file", channelvars.ParamKubernetesSecretName, channelvars.ParamKubernetesPVCName, "list", "channelGroupCode", "voucherModel",
		channelvars.ParamChannelCredential,
		channelvars.ParamGitProject, channelvars.ParamGitRepositoryURL, channelvars.ParamGitBranch, channelvars.ParamGitTag,
		channelvars.ParamNexusRepository, channelvars.ParamNexusRepositoryURL,
		channelvars.ParamHarborProject, channelvars.ParamHarborImage, channelvars.ParamHarborProjectURL, channelvars.ParamHarborImageURL, channelvars.ParamHarborImageTagURL,
		channelvars.ParamJfrogRepository, channelvars.ParamJfrogRepositoryURL,
		channelvars.ParamMavenConfig, channelvars.ParamConfigCenter, channelvars.ParamSonarProjectKey, channelvars.ParamHostGroupTargets, channelvars.ParamHostConfig, channelvars.ParamHostGroupConfig,
		objectListParamType:
		return true
	default:
		return channelvars.IsChannelParamType(paramType) || channelvars.IsDynamicParamType(paramType) || channelvars.IsCompositeAddressParamType(paramType)
	}
}

func validateObjectListParam(item model.DevopsStepParam) error {
	mode := normalizeParamMode(item.Mode)
	if mode != "env" && mode != "paas" {
		logx.Errorf("对象列表只支持环境变量或平台变量模式")
		return errorx.Msg("对象列表只支持环境变量或平台变量模式")
	}
	runtimeMode := normalizeRuntimeMode(item.RuntimeMode, mode)
	if runtimeMode != "env" && runtimeMode != "params" {
		logx.Errorf("对象列表输出方式不支持")
		return errorx.Msg("对象列表输出方式不支持")
	}
	fields := normalizeStepParamConfig(item).CompoundFields
	if len(fields) == 0 {
		logx.Errorf("对象列表必须配置字段")
		return errorx.Msg("对象列表必须配置字段")
	}
	seen := make(map[string]struct{}, len(fields))
	paramByCode := make(map[string]model.DevopsStepParam, len(fields))
	for _, field := range fields {
		if err := validatePipelineCode(field.Code, "字段编码"); err != nil {
			logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
			return err
		}
		if strings.TrimSpace(field.Name) == "" {
			logx.Errorf("对象列表字段名称不能为空")
			return errorx.Msg("对象列表字段名称不能为空")
		}
		fieldMode := normalizeParamMode(field.Mode)
		if fieldMode != "env" && fieldMode != "paas" {
			logx.Errorf("对象列表字段只支持环境变量或平台变量")
			return errorx.Msg("对象列表字段只支持环境变量或平台变量")
		}
		if normalizeRuntimeMode(field.RuntimeMode, fieldMode) != "env" {
			logx.Errorf("对象列表字段不支持执行变量映射")
			return errorx.Msg("对象列表字段不支持执行变量映射")
		}
		if field.ParamType == objectListParamType {
			logx.Errorf("对象列表字段不支持嵌套对象列表")
			return errorx.Msg("对象列表字段不支持嵌套对象列表")
		}
		if field.ParamType == "file" {
			logx.Errorf("对象列表字段不支持文件类型")
			return errorx.Msg("对象列表字段不支持文件类型")
		}
		if _, ok := seen[field.Code]; ok {
			logx.Errorf("对象列表字段编码不能重复")
			return errorx.Msg("对象列表字段编码不能重复")
		}
		seen[field.Code] = struct{}{}
		paramByCode[field.Code] = model.DevopsStepParam{
			Name:         field.Name,
			Code:         field.Code,
			DefaultValue: field.DefaultValue,
			ParamType:    field.ParamType,
			Mode:         fieldMode,
			Required:     field.Required,
			Readonly:     field.Readonly,
			Description:  field.Description,
			SelectList:   field.SelectList,
			AllowCustom:  field.AllowCustom,
			RuntimeMode:  "env",
			Config:       normalizeCompoundFieldConfig(field),
		}
	}
	for _, field := range fields {
		param := paramByCode[field.Code]
		if err := validateParamModeType(nil, nil, param, paramByCode); err != nil {
			logx.Errorf("校验对象列表字段失败: field=%s err=%v", field.Name, err)
			return errorx.Msg("对象列表字段配置错误：" + field.Name)
		}
	}
	return nil
}

func isCredentialMappingField(credentialType, field string) bool {
	for _, item := range credentialTypeOptions() {
		if item.CredentialType == credentialType {
			for _, mapping := range item.MappingFields {
				if mapping == field {
					return true
				}
			}
		}
	}
	return false
}

func validateChannelCredentialParam(item model.DevopsStepParam, config model.DevopsStepParamConfig, paramByCode map[string]model.DevopsStepParam) error {
	if item.Mode != "paas" {
		logx.Errorf("渠道凭证只支持平台变量")
		return errorx.Msg("渠道凭证只支持平台变量")
	}
	sourceCode := strings.TrimSpace(config.CredentialSourceParamCode)
	if sourceCode == "" {
		logx.Errorf("渠道凭证参数缺少渠道来源")
		return errorx.Msg("渠道凭证参数缺少渠道来源")
	}
	sourceParam, ok := paramByCode[sourceCode]
	if !ok || !isCredentialSourceStepParam(sourceParam) {
		logx.Errorf("渠道凭证来源参数不支持")
		return errorx.Msg("渠道凭证来源参数不支持")
	}
	if config.CredentialMode == credentialModeJenkinsID {
		return nil
	}
	if config.CredentialMode == credentialModeJSONData || config.CredentialMode == credentialModeJSONBase64 {
		return nil
	}
	if !isChannelCredentialFieldMappingField(config.MappingField) {
		logx.Errorf("渠道凭证映射字段不支持")
		return errorx.Msg("渠道凭证映射字段不支持")
	}
	return nil
}

func isCredentialSourceStepParam(item model.DevopsStepParam) bool {
	if channelvars.IsChannelParamType(item.ParamType) && strings.TrimSpace(item.ParamType) != channelvars.ParamChannelVariable {
		return true
	}
	if channelvars.IsCompositeAddressParamType(item.ParamType) {
		return true
	}
	if strings.TrimSpace(item.ParamType) != channelvars.ParamChannelVariable {
		return false
	}
	config := normalizeStepParamConfig(item)
	if strings.TrimSpace(config.MappingField) == channelvars.FieldEndpoint {
		return true
	}
	if strings.HasPrefix(strings.TrimSpace(config.MappingField), "address.") {
		return true
	}
	dynamicType := stepDynamicParamType(item)
	return channelvars.IsCompositeAddressParamType(dynamicType)
}

const (
	credentialModeField      = "field"
	credentialModeJSONData   = "jsonData"
	credentialModeJSONBase64 = "jsonBase64"
	credentialModeJenkinsID  = "jenkinsCredentialId"
)

func normalizeConfigCenterValueMode(valueMode string) string {
	switch strings.TrimSpace(valueMode) {
	case "text":
		return "text"
	case "credentialsSecretFile":
		return "credentialsSecretFile"
	case "base64":
		return "base64"
	default:
		return "string"
	}
}

func normalizeDeployConfigValueMode(valueMode string) string {
	switch strings.TrimSpace(valueMode) {
	case "json":
		return "json"
	case "jsonBase64":
		return "jsonBase64"
	default:
		return "string"
	}
}

func normalizeCredentialMode(mode, mappingField string) string {
	mode = strings.TrimSpace(mode)
	switch mode {
	case credentialModeField:
		return credentialModeField
	case credentialModeJSONData:
		return credentialModeJSONData
	case credentialModeJSONBase64:
		return credentialModeJSONBase64
	case credentialModeJenkinsID:
		return credentialModeJenkinsID
	}
	if mode != "" {
		return credentialModeField
	}
	mappingField = strings.TrimSpace(mappingField)
	switch mappingField {
	case "credentialId":
		return credentialModeJenkinsID
	case credentialModeJSONData:
		return credentialModeJSONData
	case credentialModeJSONBase64:
		return credentialModeJSONBase64
	}
	return credentialModeField
}

func normalizeChannelCredentialMode(mode, mappingField string) string {
	mode = strings.TrimSpace(mode)
	switch mode {
	case credentialModeJenkinsID, credentialModeJSONData, credentialModeJSONBase64:
		return mode
	}
	switch strings.TrimSpace(mappingField) {
	case "credentialId":
		return credentialModeJenkinsID
	case credentialModeJSONData:
		return credentialModeJSONData
	case credentialModeJSONBase64:
		return credentialModeJSONBase64
	default:
		return credentialModeField
	}
}

func isChannelCredentialFieldMappingField(field string) bool {
	switch strings.TrimSpace(field) {
	case "username", "password", "token", "secretText", "privateKey", "passphrase", "kubeconfig", "certificate":
		return true
	default:
		return false
	}
}

func defaultCredentialMappingField(credentialType string) string {
	for _, item := range credentialTypeOptions() {
		if item.CredentialType == credentialType {
			return item.DefaultMappingField
		}
	}
	return ""
}

func credentialTypeOptions() []*pb.CredentialTypeOption {
	return []*pb.CredentialTypeOption{
		{CredentialType: "username_password", Name: "用户名密码", MappingFields: []string{"username", "password"}, DefaultMappingField: "username"},
		{CredentialType: "token", Name: "Token", MappingFields: []string{"token"}, DefaultMappingField: "token"},
		{CredentialType: "ssh_key", Name: "SSH Key", MappingFields: []string{"username", "privateKey", "passphrase"}, DefaultMappingField: "username"},
		{CredentialType: "kubeconfig", Name: "Kubeconfig", MappingFields: []string{"kubeconfig"}, DefaultMappingField: "kubeconfig"},
		{CredentialType: "secret_text", Name: "Secret Text", MappingFields: []string{"secretText"}, DefaultMappingField: "secretText"},
		{CredentialType: "certificate", Name: "证书", MappingFields: []string{"certificate"}, DefaultMappingField: "certificate"},
		{CredentialType: "json", Name: "JSON", MappingFields: []string{"jsonData", "jsonBase64"}, DefaultMappingField: "jsonBase64"},
	}
}

func channelMappingFields(groupCode string) []*pb.ChannelMappingFieldOption {
	return variableFieldsToPb(channelvars.DefaultVariableFields())
}

func isSuperAdminRole(roles []string) bool {
	for _, role := range roles {
		if strings.EqualFold(strings.TrimSpace(role), "SUPER_ADMIN") {
			return true
		}
	}
	return false
}

func userProjectIDs(ctx context.Context, svcCtx *svc.ServiceContext, userID uint64, roles []string) ([]string, bool, error) {
	if isSuperAdminRole(roles) {
		return nil, false, nil
	}
	ids, err := svcCtx.ProjectMemberModel.ListProjectIDsByUser(ctx, userID)
	if err != nil {
		logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
		return nil, false, err
	}
	platformID, _ := ctx.Value("platformId").(uint64)
	if platformID == 0 || len(ids) == 0 {
		return []string{}, true, nil
	}
	portalResp, err := svcCtx.PortalRpc.ListProjects(ctx, &portalprojectservice.PortalListProjectsReq{
		Page:       1,
		PageSize:   uint64(len(ids)),
		UserId:     userID,
		PlatformId: platformID,
	})
	if err != nil {
		logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
		return nil, false, err
	}
	allowed := make(map[string]struct{}, len(portalResp.Data))
	for _, item := range portalResp.Data {
		if item == nil || item.Uuid == "" {
			continue
		}
		allowed[item.Uuid] = struct{}{}
	}
	filtered := make([]string, 0, len(ids))
	for _, id := range ids {
		project, err := svcCtx.ProjectModel.FindOne(ctx, id)
		if err != nil || project == nil || project.PortalProjectUuid == "" {
			continue
		}
		if _, ok := allowed[project.PortalProjectUuid]; ok {
			filtered = append(filtered, id)
		}
	}
	return filtered, true, nil
}

func ensureTemplateTargetAccess(ctx context.Context, svcCtx *svc.ServiceContext, scope, projectID string, userID uint64, roles []string) error {
	scope = normalizeTemplateScope(scope)
	if scope == "system" {
		if !isSuperAdminRole(roles) {
			logx.Errorf("只有 SUPER_ADMIN 可以维护系统级模板")
			return errorx.Msg("只有 SUPER_ADMIN 可以维护系统级模板")
		}
		return nil
	}
	if strings.TrimSpace(projectID) == "" {
		logx.Errorf("项目级模板必须选择项目")
		return errorx.Msg("项目级模板必须选择项目")
	}
	if isSuperAdminRole(roles) {
		return nil
	}
	projectIDs, _, err := userProjectIDs(ctx, svcCtx, userID, roles)
	if err != nil {
		logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
		return err
	}
	for _, id := range projectIDs {
		if id == projectID {
			return nil
		}
	}
	logx.Errorf("只能维护自己绑定项目的模板")
	return errorx.Msg("只能维护自己绑定项目的模板")
}

func ensureTemplateReadAccess(ctx context.Context, svcCtx *svc.ServiceContext, template *model.DevopsPipelineTemplate, userID uint64, roles []string) error {
	if template == nil {
		logx.Errorf("流水线模板不存在")
		return errorx.Msg("流水线模板不存在")
	}
	if normalizeTemplatePersistedScope(template.Scope, template.ProjectID) == "system" || isSuperAdminRole(roles) {
		return nil
	}
	projectIDs, _, err := userProjectIDs(ctx, svcCtx, userID, roles)
	if err != nil {
		logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
		return err
	}
	for _, id := range projectIDs {
		if id == template.ProjectID {
			return nil
		}
	}
	logx.Errorf("无权访问该项目模板")
	return errorx.Msg("无权访问该项目模板")
}

func ensureTemplateWriteAccess(ctx context.Context, svcCtx *svc.ServiceContext, template *model.DevopsPipelineTemplate, userID uint64, roles []string) error {
	if template == nil {
		logx.Errorf("流水线模板不存在")
		return errorx.Msg("流水线模板不存在")
	}
	scope := normalizeTemplatePersistedScope(template.Scope, template.ProjectID)
	if scope == "system" {
		if !isSuperAdminRole(roles) {
			logx.Errorf("只有 SUPER_ADMIN 可以维护系统级模板")
			return errorx.Msg("只有 SUPER_ADMIN 可以维护系统级模板")
		}
		return nil
	}
	return ensureTemplateTargetAccess(ctx, svcCtx, scope, template.ProjectID, userID, roles)
}

func ensureTemplateReferenceAccess(ctx context.Context, svcCtx *svc.ServiceContext, template *model.DevopsPipelineTemplate, projectID string, userID uint64, roles []string) error {
	if err := ensureProjectAccess(ctx, svcCtx, projectID, userID, roles); err != nil {
		return err
	}
	if template == nil {
		logx.Errorf("流水线模板不存在")
		return errorx.Msg("流水线模板不存在")
	}
	if normalizeTemplatePersistedScope(template.Scope, template.ProjectID) == "system" {
		return nil
	}
	if strings.TrimSpace(template.ProjectID) == strings.TrimSpace(projectID) {
		return nil
	}
	logx.Errorf("项目流水线不能引用其他项目模板")
	return errorx.Msg("项目流水线不能引用其他项目模板")
}

func ensureProjectAccess(ctx context.Context, svcCtx *svc.ServiceContext, projectID string, userID uint64, roles []string) error {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		logx.Errorf("请选择项目")
		return errorx.Msg("请选择项目")
	}
	if isSuperAdminRole(roles) {
		return nil
	}
	projectIDs, _, err := userProjectIDs(ctx, svcCtx, userID, roles)
	if err != nil {
		logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
		return err
	}
	for _, id := range projectIDs {
		if id == projectID {
			return nil
		}
	}
	logx.Errorf("无权访问该项目")
	return errorx.Msg("无权访问该项目")
}

func resolveProjectConfigType(ctx context.Context, svcCtx *svc.ServiceContext, typeID, typeCode string) (*model.DevopsConfigType, error) {
	typeID = strings.TrimSpace(typeID)
	typeCode = strings.TrimSpace(typeCode)
	if typeID != "" {
		return svcCtx.ConfigTypeModel.FindOne(ctx, typeID)
	}
	if typeCode != "" {
		return svcCtx.ConfigTypeModel.FindOneByCode(ctx, typeCode)
	}
	logx.Errorf("配置类型不能为空")
	return nil, errorx.Msg("配置类型不能为空")
}

func ensureSystemAccess(ctx context.Context, svcCtx *svc.ServiceContext, system *model.DevopsSystem, userID uint64, roles []string) error {
	if system == nil {
		logx.Errorf("系统不存在")
		return errorx.Msg("系统不存在")
	}
	return ensureProjectAccess(ctx, svcCtx, system.ProjectID, userID, roles)
}

func ensureEnvironmentAccess(ctx context.Context, svcCtx *svc.ServiceContext, environment *model.DevopsPipelineEnvironment, userID uint64, roles []string) error {
	if environment == nil {
		logx.Errorf("流水线环境不存在")
		return errorx.Msg("流水线环境不存在")
	}
	return nil
}

func normalizeTemplateScope(scope string) string {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return "system"
	}
	return scope
}

func normalizeTemplatePersistedScope(scope, projectID string) string {
	scope = strings.TrimSpace(scope)
	if scope == "" && strings.TrimSpace(projectID) != "" {
		return "project"
	}
	return normalizeTemplateScope(scope)
}

func normalizeTemplateEngineType(engineType string) string {
	engineType = strings.TrimSpace(engineType)
	if engineType == "" {
		return jenkinsEngineType
	}
	return engineType
}

func validateTemplateEngineType(engineType string) error {
	switch normalizeTemplateEngineType(engineType) {
	case jenkinsEngineType, tektonEngineType:
		return nil
	default:
		logx.Errorf("模板引擎不支持")
		return errorx.Msg("模板引擎不支持")
	}
}

func validateJenkinsStepContent(content string, isPost bool) (string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		logx.Errorf("步骤代码不能为空")
		return "", errorx.Msg("步骤代码不能为空")
	}
	if isPost {
		return validateJenkinsPostStepContent(content)
	}
	if stageCallPattern.MatchString(content) || parallelBlockPattern.MatchString(content) || postBlockPattern.MatchString(content) {
		logx.Errorf("步骤代码只能保存 steps 块，不能包含 stage、parallel 或 post")
		return "", errorx.Msg("步骤代码只能保存 steps 块，不能包含 stage、parallel 或 post")
	}
	if _, ok := extractWholeGroovyNamedBlock(content, "steps"); !ok {
		logx.Errorf("步骤代码必须是完整 steps 块")
		return "", errorx.Msg("步骤代码必须是完整 steps 块")
	}
	if postConditionLinePattern.MatchString(content) {
		logx.Errorf("always、success、failure、changed、aborted、unstable 只能用于 post 步骤分类")
		return "", errorx.Msg("always、success、failure、changed、aborted、unstable 只能用于 post 步骤分类")
	}
	return content, nil
}

func validateTektonStepContent(content string, code string) (string, error) {
	content = strings.TrimSpace(content)
	resource, err := devopstekton.ParseStepResource(content)
	if err != nil {
		logx.Errorf("Tekton步骤内容不合法: %v", err)
		return "", errorx.Msg(err.Error())
	}
	resourceName := strings.TrimSpace(code)
	if resourceName == "" {
		resourceName = resource.Name
	}
	data, err := normalizeTektonStepYAML(content, normalizeTektonResourceName(resourceName))
	if err != nil {
		logx.Errorf("规范化 Tekton Task YAML 失败: %v", err)
		return "", err
	}
	return data, nil
}

func normalizeTektonStepYAML(content, resourceName string) (string, error) {
	var doc yamlv3.Node
	if err := yamlv3.Unmarshal([]byte(content), &doc); err != nil {
		return "", err
	}
	root := yamlDocumentRoot(&doc)
	if root == nil || root.Kind != yamlv3.MappingNode {
		return "", errorx.Msg("Tekton YAML 根节点必须是对象")
	}
	metadata := yamlMappingValue(root, "metadata")
	if metadata == nil {
		metadata = &yamlv3.Node{Kind: yamlv3.MappingNode}
		root.Content = append(root.Content,
			&yamlv3.Node{Kind: yamlv3.ScalarNode, Value: "metadata"},
			metadata,
		)
	}
	if metadata.Kind != yamlv3.MappingNode {
		return "", errorx.Msg("Tekton metadata 必须是对象")
	}
	yamlSetScalar(metadata, "name", resourceName)
	yamlRemoveKey(metadata, "namespace")
	data, err := yamlv3.Marshal(&doc)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func yamlDocumentRoot(doc *yamlv3.Node) *yamlv3.Node {
	if doc == nil {
		return nil
	}
	if doc.Kind == yamlv3.DocumentNode {
		if len(doc.Content) == 0 {
			return nil
		}
		return doc.Content[0]
	}
	return doc
}

func yamlMappingValue(node *yamlv3.Node, key string) *yamlv3.Node {
	if node == nil || node.Kind != yamlv3.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func yamlSetScalar(node *yamlv3.Node, key, value string) {
	if node == nil || node.Kind != yamlv3.MappingNode {
		return
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content[i+1].Kind = yamlv3.ScalarNode
			node.Content[i+1].Tag = "!!str"
			node.Content[i+1].Value = value
			return
		}
	}
	node.Content = append(node.Content,
		&yamlv3.Node{Kind: yamlv3.ScalarNode, Value: key},
		&yamlv3.Node{Kind: yamlv3.ScalarNode, Tag: "!!str", Value: value},
	)
}

func yamlRemoveKey(node *yamlv3.Node, key string) {
	if node == nil || node.Kind != yamlv3.MappingNode {
		return
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content = append(node.Content[:i], node.Content[i+2:]...)
			return
		}
	}
}

func normalizeTektonResourceName(value string) string {
	name := strings.ToLower(strings.TrimSpace(value))
	name = tektonResourceNameInvalidPattern.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if name == "" {
		name = "tekton-task"
	}
	if len(name) > 63 {
		name = strings.Trim(name[:63], "-")
	}
	if name == "" {
		name = "tekton-task"
	}
	return name
}

func validateTektonStepParamMappings(content string, params []model.DevopsStepParam) error {
	resource, err := devopstekton.ParseStepResource(content)
	if err != nil {
		logx.Errorf("Tekton步骤内容不合法: %v", err)
		return errorx.Msg(err.Error())
	}
	taskParams := map[string]bool{}
	rawParams, _, err := unstructured.NestedSlice(resource.Object.Object, "spec", "params")
	if err != nil {
		logx.Errorf("读取 Tekton Task 参数失败: %v", err)
		return err
	}
	for _, item := range rawParams {
		data, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(fmt.Sprint(data["name"]))
		if name == "" {
			continue
		}
		if _, ok := taskParams[name]; ok {
			logx.Errorf("Tekton Task 参数名称重复: %s", name)
			return errorx.Msg("Tekton Task 参数名称不能重复")
		}
		_, hasDefault := data["default"]
		taskParams[name] = hasDefault
	}
	taskWorkspaces := map[string]struct{}{}
	rawWorkspaces, _, err := unstructured.NestedSlice(resource.Object.Object, "spec", "workspaces")
	if err != nil {
		logx.Errorf("读取 Tekton Task 工作区失败: %v", err)
		return err
	}
	for _, item := range rawWorkspaces {
		data, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(fmt.Sprint(data["name"]))
		if name != "" {
			if _, ok := taskWorkspaces[name]; ok {
				logx.Errorf("Tekton Task workspace 名称重复: %s", name)
				return errorx.Msg("Tekton Task workspace 名称不能重复")
			}
			taskWorkspaces[name] = struct{}{}
		}
	}
	mappedParams := map[string]struct{}{}
	workspaceBindingKinds := map[string]string{}
	workspaceBindingCodes := map[string]string{}
	volumeClaimTemplateFields := map[string]map[string]string{}
	for _, item := range params {
		code := strings.TrimSpace(item.Code)
		if code == "" {
			continue
		}
		config := normalizeStepParamConfig(item)
		workspaceName := strings.TrimSpace(config.MappingField)
		if bindingKind := tektonStepWorkspaceBindingKind(item, config); bindingKind != "" {
			if _, ok := taskParams[code]; ok {
				logx.Errorf("workspace 绑定参数编码与 Task 参数重名: %s", code)
				return errorx.Msg("workspace 绑定参数编码不能与 Task 参数重名")
			}
			if _, ok := taskWorkspaces[workspaceName]; !ok {
				logx.Errorf("workspace 绑定参数映射的 workspace 不存在: param=%s workspace=%s", code, workspaceName)
				return errorx.Msg("workspace 绑定参数映射的 workspace 不存在")
			}
			if existKind := workspaceBindingKinds[workspaceName]; existKind != "" && existKind != bindingKind {
				logx.Errorf("workspace 重复配置不同挂载方式: workspace=%s kind=%s/%s", workspaceName, existKind, bindingKind)
				return errorx.Msg("同一个 workspace 只能选择一种挂载方式")
			}
			workspaceBindingKinds[workspaceName] = bindingKind
			switch bindingKind {
			case "secret", "persistentVolumeClaim":
				if existCode, ok := workspaceBindingCodes[workspaceName]; ok && existCode != code {
					logx.Errorf("workspace 重复映射: workspace=%s params=%s,%s", workspaceName, existCode, code)
					if bindingKind == "secret" {
						return errorx.Msg("同一个 Secret workspace 只能映射一个 Secret 参数")
					}
					return errorx.Msg("同一个 workspace 只能映射一个挂载参数")
				}
				workspaceBindingCodes[workspaceName] = code
			case "volumeClaimTemplate":
				field, err := tektonVolumeClaimTemplateProviderField(config.Provider)
				if err != nil {
					return err
				}
				if _, ok := volumeClaimTemplateFields[workspaceName]; !ok {
					volumeClaimTemplateFields[workspaceName] = map[string]string{}
				}
				if existCode, ok := volumeClaimTemplateFields[workspaceName][field]; ok && existCode != code {
					logx.Errorf("动态 PVC workspace 字段重复映射: workspace=%s field=%s params=%s,%s", workspaceName, field, existCode, code)
					return errorx.Msg("动态 PVC workspace 字段不能重复映射")
				}
				volumeClaimTemplateFields[workspaceName][field] = code
			}
			continue
		}
		if _, ok := taskParams[code]; !ok {
			logx.Errorf("Tekton 平台参数未匹配 Task 参数: %s", code)
			return errorx.Msg("Tekton 平台参数必须对应 Task spec.params")
		}
		mappedParams[code] = struct{}{}
	}
	_ = mappedParams
	for workspaceName, fields := range volumeClaimTemplateFields {
		if _, ok := fields["storage"]; !ok {
			logx.Errorf("动态 PVC workspace 缺少存储容量字段: workspace=%s", workspaceName)
			return errorx.Msg("动态 PVC workspace 必须配置存储容量")
		}
	}
	return nil
}

func tektonStepWorkspaceBindingKind(item model.DevopsStepParam, config model.DevopsStepParamConfig) string {
	if strings.TrimSpace(config.MappingField) == "" {
		return ""
	}
	switch item.ParamType {
	case channelvars.ParamKubernetesSecretName:
		return "secret"
	case channelvars.ParamKubernetesPVCName:
		return "persistentVolumeClaim"
	}
	if strings.HasPrefix(strings.TrimSpace(config.Provider), "volumeClaimTemplate.") {
		return "volumeClaimTemplate"
	}
	return ""
}

func tektonVolumeClaimTemplateProviderField(provider string) (string, error) {
	field := strings.TrimPrefix(strings.TrimSpace(provider), "volumeClaimTemplate.")
	switch field {
	case "storage", "storageClassName", "accessMode", "volumeMode":
		return field, nil
	default:
		logx.Errorf("动态 PVC workspace 字段不支持: %s", provider)
		return "", errorx.Msg("动态 PVC workspace 字段不支持")
	}
}

type tektonPipelineTemplateDagConfig struct {
	Version        string                            `json:"version"`
	Pipeline       tektonPipelineTemplatePipeline    `json:"pipeline"`
	Params         []tektonPipelineTemplateParam     `json:"params"`
	Results        []tektonPipelineTemplateResult    `json:"results"`
	Workspaces     []tektonPipelineTemplateWorkspace `json:"workspaces"`
	Nodes          []tektonPipelineTemplateNode      `json:"nodes"`
	Edges          []tektonPipelineTemplateEdge      `json:"edges"`
	FinallyNodes   []tektonPipelineTemplateNode      `json:"finallyNodes"`
	RunPolicy      tektonPipelineTemplateRunPolicy   `json:"runPolicy"`
	TriggerConfig  tektonPipelineTemplateTrigger     `json:"triggerConfig"`
	ScheduleConfig tektonPipelineTemplateSchedule    `json:"scheduleConfig"`
	PrunerPolicy   tektonPipelineTemplatePruner      `json:"prunerPolicy"`
	Labels         map[string]string                 `json:"labels"`
	Annotations    map[string]string                 `json:"annotations"`
}

type tektonPipelineTemplatePipeline struct {
	Name               string                            `json:"name"`
	Namespace          string                            `json:"namespace"`
	DisplayName        string                            `json:"displayName"`
	Description        string                            `json:"description"`
	ServiceAccountName string                            `json:"serviceAccountName"`
	Timeout            string                            `json:"timeout"`
	Timeouts           map[string]string                 `json:"timeouts"`
	Labels             []tektonPipelineTemplateNameVal   `json:"labels"`
	Annotations        []tektonPipelineTemplateNameVal   `json:"annotations"`
	Params             []tektonPipelineTemplateParam     `json:"params"`
	Results            []tektonPipelineTemplateResult    `json:"results"`
	Workspaces         []tektonPipelineTemplateWorkspace `json:"workspaces"`
}

type tektonPipelineTemplateParam struct {
	Name          string                         `json:"name"`
	DisplayName   string                         `json:"displayName"`
	Type          string                         `json:"type"`
	Enum          []any                          `json:"enum"`
	Properties    map[string]tektonParamProperty `json:"properties"`
	Default       any                            `json:"default"`
	DefaultValue  any                            `json:"defaultValue"`
	Description   string                         `json:"description"`
	Required      bool                           `json:"required"`
	RuntimeConfig bool                           `json:"runtimeConfig"`
	Readonly      bool                           `json:"readonly"`
	ParamType     string                         `json:"paramType"`
	SelectList    []model.DevopsStepParamOption  `json:"selectList"`
	Options       []model.DevopsStepParamOption  `json:"options"`
	AllowCustom   bool                           `json:"allowCustom"`
	Config        model.DevopsStepParamConfig    `json:"config"`
	SourceCode    string                         `json:"sourceCode"`
}

type tektonPipelineTemplateResult struct {
	Name        string                         `json:"name"`
	Type        string                         `json:"type"`
	Properties  map[string]tektonParamProperty `json:"properties"`
	Description string                         `json:"description"`
	Value       any                            `json:"value"`
}

type tektonParamProperty struct {
	Type string `json:"type"`
}

type tektonPipelineTemplateWorkspace struct {
	Name                 string   `json:"name"`
	Description          string   `json:"description"`
	Optional             bool     `json:"optional"`
	BindingType          string   `json:"bindingType"`
	AllowedBindingTypes  []string `json:"allowedBindingTypes"`
	AllowedSourceTypes   []string `json:"allowedSourceTypes"`
	AllowedPlatformTypes []string `json:"allowedPlatformTypes"`
	RuntimeConfig        bool     `json:"runtimeConfig"`
	ClaimName            string   `json:"claimName"`
	SecretName           string   `json:"secretName"`
	ConfigMapName        string   `json:"configMapName"`
	ResourceID           string   `json:"resourceId"`
}

type tektonPipelineTemplateNode struct {
	ID                    string                               `json:"id"`
	Type                  string                               `json:"type"`
	Name                  string                               `json:"name"`
	TaskName              string                               `json:"taskName"`
	DisplayName           string                               `json:"displayName"`
	Description           string                               `json:"description"`
	TaskID                string                               `json:"taskId"`
	TaskCode              string                               `json:"taskCode"`
	StepCode              string                               `json:"stepCode"`
	TaskRef               tektonPipelineTemplateTaskRef        `json:"taskRef"`
	TaskSpec              map[string]any                       `json:"taskSpec"`
	PipelineRef           tektonPipelineTemplatePipelineRef    `json:"pipelineRef"`
	PipelineSpec          map[string]any                       `json:"pipelineSpec"`
	Params                []tektonPipelineTemplateTaskParam    `json:"params"`
	ParamRequirements     []tektonPipelineTemplateParamRequire `json:"paramRequirements"`
	Workspaces            []tektonPipelineTemplateWorkspaceMap `json:"workspaces"`
	WorkspaceRequirements []tektonPipelineTemplateWorkspaceReq `json:"workspaceRequirements"`
	When                  []tektonPipelineTemplateWhen         `json:"when"`
	Retries               *int64                               `json:"retries"`
	Timeout               string                               `json:"timeout"`
	OnError               string                               `json:"onError"`
	Matrix                map[string]any                       `json:"matrix"`
	Results               []string                             `json:"results"`
	Enabled               *bool                                `json:"enabled"`
}

type tektonPipelineTemplateWhen struct {
	Input    string `json:"input"`
	Operator string `json:"operator"`
	Values   []any  `json:"values"`
	CEL      string `json:"cel"`
}

type tektonPipelineTemplateTaskRef struct {
	Kind       string                          `json:"kind"`
	Name       string                          `json:"name"`
	APIVersion string                          `json:"apiVersion"`
	Bundle     string                          `json:"bundle"`
	Resolver   string                          `json:"resolver"`
	Namespace  string                          `json:"namespace"`
	Params     []tektonPipelineTemplateNameVal `json:"params"`
}

type tektonPipelineTemplatePipelineRef struct {
	Name       string                          `json:"name"`
	APIVersion string                          `json:"apiVersion"`
	Bundle     string                          `json:"bundle"`
	Resolver   string                          `json:"resolver"`
	Params     []tektonPipelineTemplateNameVal `json:"params"`
}

type tektonPipelineTemplateNameVal struct {
	Name  string `json:"name"`
	Value any    `json:"value"`
}

type tektonPipelineTemplateRunPolicy struct {
	ServiceAccountName string                            `json:"serviceAccountName"`
	ManagedBy          string                            `json:"managedBy"`
	Timeout            string                            `json:"timeout"`
	PipelineTimeout    string                            `json:"pipelineTimeout"`
	TasksTimeout       string                            `json:"tasksTimeout"`
	FinallyTimeout     string                            `json:"finallyTimeout"`
	Timeouts           map[string]string                 `json:"timeouts"`
	Workspaces         []tektonPipelineTemplateWorkspace `json:"workspaces"`
	Labels             []tektonPipelineTemplateNameVal   `json:"labels"`
	Annotations        []tektonPipelineTemplateNameVal   `json:"annotations"`
	PodTemplateEnv     []tektonPipelineTemplateNameVal   `json:"podTemplateEnv"`
	PodTemplateRaw     string                            `json:"podTemplateRaw"`
	TaskRunTemplateRaw string                            `json:"taskRunTemplateRaw"`
	TaskRunSpecs       []map[string]any                  `json:"taskRunSpecs"`
	TaskRunSpecsRaw    string                            `json:"taskRunSpecsRaw"`
	CleanBeforeRun     bool                              `json:"cleanBeforeRun"`
	CleanAfterRun      bool                              `json:"cleanAfterRun"`
	CleanupImage       string                            `json:"cleanupImage"`
	ConcurrencyPolicy  string                            `json:"concurrencyPolicy"`
	WorkspaceStrategy  string                            `json:"workspaceStrategy"`
}

type tektonPipelineTemplateTrigger struct {
	Enabled             bool                                 `json:"enabled"`
	APIVersion          string                               `json:"apiVersion"`
	Provider            string                               `json:"provider"`
	Namespace           string                               `json:"namespace"`
	EventListenerName   string                               `json:"eventListenerName"`
	TriggerName         string                               `json:"triggerName"`
	TriggerBindingName  string                               `json:"triggerBindingName"`
	TriggerTemplateName string                               `json:"triggerTemplateName"`
	ServiceAccountName  string                               `json:"serviceAccountName"`
	ServiceType         string                               `json:"serviceType"`
	IngressHost         string                               `json:"ingressHost"`
	IngressPath         string                               `json:"ingressPath"`
	IngressClassName    string                               `json:"ingressClassName"`
	TLSSecretName       string                               `json:"tlsSecretName"`
	WebhookPath         string                               `json:"webhookPath"`
	WebhookURL          string                               `json:"webhookUrl"`
	InterceptorType     string                               `json:"interceptorType"`
	CELFilter           string                               `json:"celFilter"`
	EventTypes          []string                             `json:"eventTypes"`
	SecretName          string                               `json:"secretName"`
	SecretKey           string                               `json:"secretKey"`
	BindingParams       []tektonPipelineTemplateTriggerParam `json:"bindingParams"`
	TemplateParams      []tektonPipelineTemplateTriggerParam `json:"templateParams"`
	Labels              []tektonPipelineTemplateNameVal      `json:"labels"`
	Annotations         []tektonPipelineTemplateNameVal      `json:"annotations"`
}

type tektonPipelineTemplateTriggerParam struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Description string `json:"description"`
}

type tektonPipelineTemplateSchedule struct {
	Enabled           bool              `json:"enabled"`
	Cron              string            `json:"cron"`
	Timezone          string            `json:"timezone"`
	ConcurrencyPolicy string            `json:"concurrencyPolicy"`
	ParameterSnapshot bool              `json:"parameterSnapshot"`
	ParamsSnapshot    map[string]string `json:"paramsSnapshot"`
}

type tektonPipelineTemplatePruner struct {
	Enabled                     bool                            `json:"enabled"`
	ClusterResourceCleanup      *bool                           `json:"clusterResourceCleanup"`
	PlatformRecordCleanup       *bool                           `json:"platformRecordCleanup"`
	NativePrunerMode            string                          `json:"nativePrunerMode"`
	NativePrunerTargetNamespace string                          `json:"nativePrunerTargetNamespace"`
	NativePrunerConfigLevel     string                          `json:"nativePrunerConfigLevel"`
	ResourceTypes               []string                        `json:"resourceTypes"`
	Scope                       string                          `json:"scope"`
	HistoryLimit                *int                            `json:"historyLimit"`
	SuccessfulHistoryLimit      *int                            `json:"successfulHistoryLimit"`
	FailedHistoryLimit          *int                            `json:"failedHistoryLimit"`
	TTLSecondsAfterFinished     *int                            `json:"ttlSecondsAfterFinished"`
	PlatformRecordRetentionDays *int                            `json:"platformRecordRetentionDays"`
	PlatformRecordLimit         *int                            `json:"platformRecordLimit"`
	SelectorLabels              []tektonPipelineTemplateNameVal `json:"selectorLabels"`
	SelectorAnnotations         []tektonPipelineTemplateNameVal `json:"selectorAnnotations"`
}

type tektonPipelineTemplateTaskParam struct {
	Name          string `json:"name"`
	Source        string `json:"source"`
	Value         any    `json:"value"`
	PipelineParam string `json:"pipelineParam"`
	ResultNodeID  string `json:"resultNodeId"`
	ResultName    string `json:"resultName"`
	Required      bool   `json:"required"`
	DefaultValue  any    `json:"defaultValue"`
	ParamType     string `json:"paramType"`
}

type tektonPipelineTemplateParamRequire struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	Required     bool   `json:"required"`
	DefaultValue any    `json:"defaultValue"`
	ParamType    string `json:"paramType"`
}

type tektonPipelineTemplateWorkspaceMap struct {
	TaskWorkspace       string `json:"taskWorkspace"`
	PipelineWorkspace   string `json:"pipelineWorkspace"`
	SubPath             string `json:"subPath"`
	Optional            bool   `json:"optional"`
	Description         string `json:"description"`
	RequiredBindingType string `json:"requiredBindingType"`
	BindingParamCode    string `json:"bindingParamCode"`
}

type tektonPipelineTemplateWorkspaceReq struct {
	Name                string `json:"name"`
	Optional            bool   `json:"optional"`
	Description         string `json:"description"`
	RequiredBindingType string `json:"requiredBindingType"`
	BindingParamCode    string `json:"bindingParamCode"`
}

type tektonTemplateTaskSpec struct {
	Params     map[string]bool
	Workspaces map[string]bool
	Results    map[string]bool
}

type tektonPipelineTemplateEdge struct {
	Source       string `json:"source"`
	Target       string `json:"target"`
	Type         string `json:"type"`
	ResultName   string `json:"resultName"`
	TargetParam  string `json:"targetParam"`
	WhenInput    string `json:"whenInput"`
	WhenOperator string `json:"whenOperator"`
	WhenValues   []any  `json:"whenValues"`
}

func validateTektonPipelineTemplateConfig(ctx context.Context, svcCtx *svc.ServiceContext, content string, roles []string) (string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		logx.Errorf("Tekton Pipeline 模板配置不能为空")
		return "", errorx.Msg("Tekton Pipeline 模板配置不能为空")
	}
	var cfg tektonPipelineTemplateDagConfig
	if err := json.Unmarshal([]byte(content), &cfg); err != nil {
		logx.Errorf("Tekton Pipeline 模板配置必须是 JSON 对象: %v", err)
		return "", errorx.Msg("Tekton Pipeline 模板配置必须是 JSON 对象")
	}
	if strings.TrimSpace(cfg.Version) == "" {
		cfg.Version = "tekton-dag/v1"
	}
	if cfg.Version != "tekton-dag/v1" {
		logx.Errorf("Tekton Pipeline 模板版本不支持: %s", cfg.Version)
		return "", errorx.Msg("Tekton Pipeline 模板版本不支持")
	}
	if err := validateTektonTemplatePipeline(cfg); err != nil {
		return "", err
	}
	if err := validateTektonTemplateParams(cfg); err != nil {
		return "", err
	}
	if err := validateTektonTemplateResults(cfg); err != nil {
		return "", err
	}
	if err := validateTektonTemplateWorkspaces(cfg); err != nil {
		return "", err
	}
	if err := validateTektonTemplateRunPolicy(cfg.RunPolicy); err != nil {
		return "", err
	}
	if err := validateTektonTemplateTriggerConfig(cfg.TriggerConfig); err != nil {
		return "", err
	}
	if err := validateTektonTemplateScheduleConfig(cfg.ScheduleConfig); err != nil {
		return "", err
	}
	if err := validateTektonTemplatePrunerPolicy(cfg.PrunerPolicy); err != nil {
		return "", err
	}
	if err := validateTektonTemplateStringMap(cfg.Labels, "Pipeline 标签"); err != nil {
		return "", err
	}
	if err := validateTektonTemplateStringMap(cfg.Annotations, "Pipeline 注解"); err != nil {
		return "", err
	}
	if err := validateTektonTemplateNodes(ctx, svcCtx, cfg, roles); err != nil {
		return "", err
	}
	if err := validateTektonTemplateRunPolicyTaskRunSpecRefs(cfg); err != nil {
		return "", err
	}
	if err := validateTektonTemplateEdges(cfg); err != nil {
		return "", err
	}
	normalized, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(normalized), nil
}

type tektonPipelineTemplateTriggerPolicy struct {
	tektonPipelineTemplateTrigger
	ScheduleConfig tektonPipelineTemplateSchedule `json:"scheduleConfig"`
}

func validateTektonPipelineTemplateRunPolicyContent(content string) (string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", nil
	}
	var policy tektonPipelineTemplateRunPolicy
	if err := json.Unmarshal([]byte(content), &policy); err != nil {
		logx.Errorf("Tekton 运行策略必须是 JSON 对象: %v", err)
		return "", errorx.Msg("Tekton 运行策略必须是 JSON 对象")
	}
	if err := validateTektonTemplateRunPolicy(policy); err != nil {
		return "", err
	}
	normalized, err := json.Marshal(policy)
	if err != nil {
		return "", err
	}
	return string(normalized), nil
}

func validateTektonPipelineTemplateTriggerConfigContent(content string) (string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", nil
	}
	var policy tektonPipelineTemplateTriggerPolicy
	if err := json.Unmarshal([]byte(content), &policy); err != nil {
		logx.Errorf("Tekton Trigger 配置必须是 JSON 对象: %v", err)
		return "", errorx.Msg("Tekton Trigger 配置必须是 JSON 对象")
	}
	if err := validateTektonTemplateTriggerConfig(policy.tektonPipelineTemplateTrigger); err != nil {
		return "", err
	}
	if err := validateTektonTemplateScheduleConfig(policy.ScheduleConfig); err != nil {
		return "", err
	}
	normalized, err := json.Marshal(policy)
	if err != nil {
		return "", err
	}
	return string(normalized), nil
}

func validateTektonPipelineTemplatePrunerPolicyContent(content string) (string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", nil
	}
	var policy tektonPipelineTemplatePruner
	if err := json.Unmarshal([]byte(content), &policy); err != nil {
		logx.Errorf("Tekton Pruner 配置必须是 JSON 对象: %v", err)
		return "", errorx.Msg("Tekton Pruner 配置必须是 JSON 对象")
	}
	if err := validateTektonTemplatePrunerPolicy(policy); err != nil {
		return "", err
	}
	normalized, err := json.Marshal(policy)
	if err != nil {
		return "", err
	}
	return string(normalized), nil
}

func validateTektonTemplatePipeline(cfg tektonPipelineTemplateDagConfig) error {
	name := strings.TrimSpace(cfg.Pipeline.Name)
	if name == "" {
		logx.Errorf("Tekton Pipeline 名称不能为空")
		return errorx.Msg("Tekton Pipeline 名称不能为空")
	}
	if !kubernetesSecretNamePattern.MatchString(name) {
		logx.Errorf("Tekton Pipeline 名称不合法: %s", name)
		return errorx.Msg("Tekton Pipeline 名称不符合 Kubernetes 命名规范")
	}
	if namespace := strings.TrimSpace(cfg.Pipeline.Namespace); namespace != "" && !kubernetesSecretNamePattern.MatchString(namespace) {
		logx.Errorf("Tekton Pipeline 模板不能配置非法 Namespace: %s", namespace)
		return errorx.Msg("Tekton Pipeline Namespace 不符合 Kubernetes 命名规范")
	}
	if serviceAccount := strings.TrimSpace(cfg.Pipeline.ServiceAccountName); serviceAccount != "" && !kubernetesSecretNamePattern.MatchString(serviceAccount) {
		logx.Errorf("Tekton Pipeline ServiceAccount 名称不合法: %s", serviceAccount)
		return errorx.Msg("Tekton Pipeline ServiceAccount 名称不符合 Kubernetes 命名规范")
	}
	if timeout := strings.TrimSpace(cfg.Pipeline.Timeout); timeout != "" && !tektonDurationPattern.MatchString(timeout) {
		logx.Errorf("Tekton Pipeline Timeout 格式不合法: %s", timeout)
		return errorx.Msg("Tekton Pipeline Timeout 格式不合法")
	}
	return nil
}

func validateTektonTemplateParams(cfg tektonPipelineTemplateDagConfig) error {
	seen := map[string]struct{}{}
	paramByCode := map[string]model.DevopsStepParam{}
	normalizedParams := make([]model.DevopsStepParam, 0, len(tektonTemplatePipelineParams(cfg)))
	for _, item := range tektonTemplatePipelineParams(cfg) {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			logx.Errorf("Tekton Pipeline 参数名不能为空")
			return errorx.Msg("Tekton Pipeline 参数名不能为空")
		}
		if !tektonParamNamePattern.MatchString(name) {
			logx.Errorf("Tekton Pipeline 参数名不合法: %s", name)
			return errorx.Msg("Tekton Pipeline 参数名不合法")
		}
		if _, ok := seen[name]; ok {
			logx.Errorf("Tekton Pipeline 参数重复: %s", name)
			return errorx.Msg("Tekton Pipeline 参数重复：" + name)
		}
		seen[name] = struct{}{}
		officialType, ok := tektonOfficialParamType(item.Type)
		if !ok {
			logx.Errorf("Tekton Pipeline 参数类型不支持: %s", item.Type)
			return errorx.Msg("Tekton Pipeline 参数类型不支持")
		}
		paramType := strings.TrimSpace(item.ParamType)
		if paramType == "" {
			paramType = tektonPlatformParamTypeFromTektonType(officialType)
		}
		if want := tektonTypeFromPlatformParamType(paramType); want != officialType {
			logx.Errorf("Tekton Pipeline 参数平台类型与 Tekton 类型不匹配: param=%s type=%s paramType=%s", name, item.Type, paramType)
			return errorx.Msg("Tekton Pipeline 参数平台类型与 Tekton 类型不匹配：" + name)
		}
		for _, enumValue := range item.Enum {
			if !hasTemplateParamValue(enumValue) {
				logx.Errorf("Tekton Pipeline 参数枚举值不能为空: %s", name)
				return errorx.Msg("Tekton Pipeline 参数枚举值不能为空：" + name)
			}
		}
		if len(item.Properties) > 0 && officialType != "object" {
			logx.Errorf("Tekton Pipeline 参数 properties 类型不匹配: %s", name)
			return errorx.Msg("Tekton Pipeline 参数 properties 只能用于 object 类型：" + name)
		}
		if officialType == "object" && strings.Contains(name, ".") {
			logx.Errorf("Tekton Pipeline 参数 object 名称包含点号: %s", name)
			return errorx.Msg("Tekton Pipeline 参数类型为 object 时名称不能包含点号：" + name)
		}
		if officialType == "object" && len(item.Properties) == 0 {
			logx.Errorf("Tekton Pipeline 参数 object 缺少 properties: %s", name)
			return errorx.Msg("Tekton Pipeline 参数类型为 object 时必须配置 properties：" + name)
		}
		for propertyName, property := range item.Properties {
			propertyName = strings.TrimSpace(propertyName)
			if propertyName == "" || !tektonParamNamePattern.MatchString(propertyName) {
				logx.Errorf("Tekton Pipeline 参数 properties 名称不合法: param=%s property=%s", name, propertyName)
				return errorx.Msg("Tekton Pipeline 参数 properties 名称不合法：" + name)
			}
			if _, ok := tektonOfficialParamType(property.Type); !ok {
				logx.Errorf("Tekton Pipeline 参数 properties 类型不支持: param=%s property=%s type=%s", name, propertyName, property.Type)
				return errorx.Msg("Tekton Pipeline 参数 properties 类型不支持：" + name)
			}
		}
		defaultValue := ""
		if item.Default != nil {
			defaultValue = templateParamCurrentValue(item.Default)
		} else if item.DefaultValue != nil {
			defaultValue = templateParamCurrentValue(item.DefaultValue)
		}
		options := item.SelectList
		if len(options) == 0 {
			options = item.Options
		}
		config := item.Config
		if len(config.Options) == 0 {
			config.Options = options
		}
		param := model.DevopsStepParam{
			Name:         firstNotBlank(item.DisplayName, name),
			Code:         name,
			DefaultValue: defaultValue,
			ParamType:    paramType,
			Mode:         "params",
			Required:     item.Required,
			Readonly:     item.Readonly,
			Description:  item.Description,
			SelectList:   options,
			AllowCustom:  item.AllowCustom,
			RuntimeMode:  "params",
			Config:       normalizeStepParamConfig(model.DevopsStepParam{ParamType: paramType, Config: config}),
		}
		paramByCode[name] = param
		normalizedParams = append(normalizedParams, param)
	}
	for _, item := range normalizedParams {
		if err := validateTektonTemplatePipelineParam(item, paramByCode); err != nil {
			return err
		}
	}
	return nil
}

func tektonPlatformParamTypeFromTektonType(value string) string {
	switch tektonParamType(value) {
	case "array":
		return "list"
	case "object":
		return objectListParamType
	default:
		return "string"
	}
}

func tektonTypeFromPlatformParamType(value string) string {
	switch strings.TrimSpace(value) {
	case "array", "list":
		return "array"
	case "object", objectListParamType:
		return "object"
	default:
		return "string"
	}
}

func validateTektonTemplatePipelineParam(item model.DevopsStepParam, paramByCode map[string]model.DevopsStepParam) error {
	config := normalizeStepParamConfig(item)
	if !isSupportedTektonTemplatePipelineParamType(item.ParamType) {
		logx.Errorf("Tekton Pipeline 参数平台类型不支持: param=%s type=%s", item.Code, item.ParamType)
		return errorx.Msg("Tekton Pipeline 参数平台类型不支持：" + displayStepParamName(item))
	}
	switch strings.TrimSpace(item.ParamType) {
	case "choice":
		if len(config.Options) == 0 {
			logx.Errorf("Tekton Pipeline 选项参数必须配置选项: %s", item.Code)
			return errorx.Msg("Tekton Pipeline 选项参数必须配置选项：" + displayStepParamName(item))
		}
	case "list":
		if len(config.Options) == 0 {
			if err := validateTektonTemplateStructuredDefaultValue("array", item.DefaultValue, displayStepParamName(item)); err != nil {
				return err
			}
		}
	case "array":
		if err := validateTektonTemplateStructuredDefaultValue("array", item.DefaultValue, displayStepParamName(item)); err != nil {
			return err
		}
	case "object", objectListParamType:
		if err := validateTektonTemplateStructuredDefaultValue("object", item.DefaultValue, displayStepParamName(item)); err != nil {
			return err
		}
	case "booleanParam", "number":
		if err := validateParamScalarValue(item.ParamType, item.DefaultValue, item.Name); err != nil {
			return err
		}
	case channelvars.ParamKubernetesSecretName:
		if err := validateTektonKubernetesSecretParam(item, config); err != nil {
			return err
		}
	case channelvars.ParamKubernetesPVCName:
		if err := validateKubernetesPVCName(item.DefaultValue, item.Name); err != nil {
			return err
		}
	case channelvars.ParamChannelVariable:
		if err := validateTektonTemplateChannelVariableParam(item, config, paramByCode); err != nil {
			return err
		}
	case channelvars.ParamChannelCredential:
		if err := validateTektonTemplateChannelCredentialParam(item, config, paramByCode); err != nil {
			return err
		}
	case "voucherModel":
		if strings.TrimSpace(config.VoucherModel) == "" {
			logx.Errorf("Tekton Pipeline 凭证变量必须配置凭证类型: %s", item.Code)
			return errorx.Msg("Tekton Pipeline 凭证变量必须配置凭证类型：" + displayStepParamName(item))
		}
	case channelvars.ParamConfigCenter:
		if strings.TrimSpace(config.ConfigTypeID) == "" && strings.TrimSpace(config.ConfigTypeCode) == "" {
			logx.Errorf("Tekton Pipeline 配置中心变量必须选择配置类型: %s", item.Code)
			return errorx.Msg("Tekton Pipeline 配置中心变量必须选择配置类型：" + displayStepParamName(item))
		}
	case channelvars.ParamHostGroupTargets:
		if config.RenderMode == "" {
			config.RenderMode = "normal"
		}
		if !isSupportedHostGroupRenderMode(config.RenderMode) {
			logx.Errorf("Tekton Pipeline 主机组输出模式不支持: %s", item.Code)
			return errorx.Msg("Tekton Pipeline 主机组输出模式不支持：" + displayStepParamName(item))
		}
	}
	return nil
}

func isSupportedTektonTemplatePipelineParamType(paramType string) bool {
	switch strings.TrimSpace(paramType) {
	case "string", "text", "booleanParam", "number", "password", "choice", "file", "array", "object", "list", objectListParamType,
		channelvars.ParamKubernetesSecretName, channelvars.ParamKubernetesPVCName,
		channelvars.ParamChannelVariable, "voucherModel", channelvars.ParamChannelCredential,
		channelvars.ParamConfigCenter, channelvars.ParamHostGroupTargets:
		return true
	default:
		return false
	}
}

func displayStepParamName(item model.DevopsStepParam) string {
	return firstNotBlank(item.Name, item.Code, "未命名参数")
}

func validateTektonTemplateStructuredDefaultValue(paramType, value, name string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	var parsed any
	if err := yamlv3.Unmarshal([]byte(value), &parsed); err != nil {
		return errorx.Msg("Tekton Pipeline 参数默认值结构不合法：" + name)
	}
	switch strings.TrimSpace(paramType) {
	case "array":
		if items, ok := parsed.([]any); ok {
			if !tektonTemplateAllStrings(items) {
				return errorx.Msg("Tekton Pipeline 参数默认值必须是字符串数组：" + name)
			}
			return nil
		}
		parts := strings.Split(value, ",")
		validItems := 0
		for _, part := range parts {
			if strings.TrimSpace(part) != "" {
				validItems++
			}
		}
		if validItems == 0 {
			return errorx.Msg("Tekton Pipeline 参数默认值必须是数组：" + name)
		}
	case "object":
		if _, ok := parsed.(map[string]any); !ok {
			return errorx.Msg("Tekton Pipeline 参数默认值必须是对象：" + name)
		}
	}
	return nil
}

func validateTektonTemplateChannelVariableParam(item model.DevopsStepParam, config model.DevopsStepParamConfig, paramByCode map[string]model.DevopsStepParam) error {
	if strings.TrimSpace(config.ChannelGroupCode) == "" {
		return errorx.Msg("Tekton Pipeline 渠道变量必须配置渠道分组：" + displayStepParamName(item))
	}
	if strings.TrimSpace(config.ChannelTypeFilter) == "" {
		return errorx.Msg("Tekton Pipeline 渠道变量必须配置渠道类型：" + displayStepParamName(item))
	}
	if strings.TrimSpace(config.MappingField) == "" {
		return errorx.Msg("Tekton Pipeline 渠道变量必须配置映射字段：" + displayStepParamName(item))
	}
	if handled, err := validateCodeRepoChannelVariableDependencies(item, config, channelvars.VariableField{Field: config.MappingField}, paramByCode); handled {
		if err != nil {
			return errorx.Msg("Tekton Pipeline " + err.Error() + "：" + displayStepParamName(item))
		}
		return nil
	}
	if strings.TrimSpace(config.ChannelParamCode) != "" {
		target, ok := paramByCode[strings.TrimSpace(config.ChannelParamCode)]
		if !ok || !sameChannelVariableDependency(config, channelvars.FieldEndpoint, target) {
			return errorx.Msg("Tekton Pipeline 渠道来源不匹配：" + displayStepParamName(item))
		}
	}
	if len(requiredStepChannelVariableDependencies(config)) > 0 {
		if err := validateChannelVariableStepDependencies(item, config, paramByCode); err != nil {
			return err
		}
	}
	return nil
}

func validateTektonTemplateChannelCredentialParam(item model.DevopsStepParam, config model.DevopsStepParamConfig, paramByCode map[string]model.DevopsStepParam) error {
	sourceCode := strings.TrimSpace(config.CredentialSourceParamCode)
	if sourceCode == "" {
		return errorx.Msg("Tekton Pipeline 渠道凭证必须选择渠道来源参数：" + displayStepParamName(item))
	}
	sourceParam, ok := paramByCode[sourceCode]
	if !ok || !isCredentialSourceStepParam(sourceParam) {
		return errorx.Msg("Tekton Pipeline 渠道凭证来源参数不匹配：" + displayStepParamName(item))
	}
	if config.CredentialMode == credentialModeJenkinsID {
		return errorx.Msg("Tekton Pipeline 渠道凭证不支持 Jenkins credentialId 模式：" + displayStepParamName(item))
	}
	if config.CredentialMode == credentialModeJSONData || config.CredentialMode == credentialModeJSONBase64 {
		return nil
	}
	if !isChannelCredentialFieldMappingField(config.MappingField) {
		return errorx.Msg("Tekton Pipeline 渠道凭证映射字段不支持：" + displayStepParamName(item))
	}
	return nil
}

func validateTektonTemplateResults(cfg tektonPipelineTemplateDagConfig) error {
	seen := map[string]struct{}{}
	for _, item := range tektonTemplatePipelineResults(cfg) {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			logx.Errorf("Tekton Pipeline Result 名称不能为空")
			return errorx.Msg("Tekton Pipeline Result 名称不能为空")
		}
		if !tektonParamNamePattern.MatchString(name) {
			logx.Errorf("Tekton Pipeline Result 名称不合法: %s", name)
			return errorx.Msg("Tekton Pipeline Result 名称不合法")
		}
		if _, ok := seen[name]; ok {
			logx.Errorf("Tekton Pipeline Result 重复: %s", name)
			return errorx.Msg("Tekton Pipeline Result 重复：" + name)
		}
		seen[name] = struct{}{}
		officialType, ok := tektonOfficialParamType(item.Type)
		if !ok {
			logx.Errorf("Tekton Pipeline Result 类型不支持: %s", item.Type)
			return errorx.Msg("Tekton Pipeline Result 类型不支持")
		}
		if len(item.Properties) > 0 && officialType != "object" {
			logx.Errorf("Tekton Pipeline Result properties 类型不匹配: %s", name)
			return errorx.Msg("Tekton Pipeline Result properties 只能用于 object 类型：" + name)
		}
		if officialType == "object" && len(item.Properties) == 0 {
			logx.Errorf("Tekton Pipeline Result object 缺少 properties: %s", name)
			return errorx.Msg("Tekton Pipeline Result 类型为 object 时必须配置 properties：" + name)
		}
		for propertyName, property := range item.Properties {
			propertyName = strings.TrimSpace(propertyName)
			if propertyName == "" || !tektonParamNamePattern.MatchString(propertyName) {
				logx.Errorf("Tekton Pipeline Result properties 名称不合法: result=%s property=%s", name, propertyName)
				return errorx.Msg("Tekton Pipeline Result properties 名称不合法：" + name)
			}
			if _, ok := tektonOfficialParamType(property.Type); !ok {
				logx.Errorf("Tekton Pipeline Result properties 类型不支持: result=%s property=%s type=%s", name, propertyName, property.Type)
				return errorx.Msg("Tekton Pipeline Result properties 类型不支持：" + name)
			}
		}
		value := strings.TrimSpace(fmt.Sprint(item.Value))
		if value == "" || value == "<nil>" {
			logx.Errorf("Tekton Pipeline Result 缺少 value: %s", name)
			return errorx.Msg("Tekton Pipeline Result 必须配置 value：" + name)
		}
	}
	return nil
}

func validateTektonTemplateWorkspaces(cfg tektonPipelineTemplateDagConfig) error {
	seen := map[string]struct{}{}
	for _, item := range tektonTemplatePipelineWorkspaces(cfg) {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			logx.Errorf("Tekton Workspace 名称不能为空")
			return errorx.Msg("Tekton Workspace 名称不能为空")
		}
		if !kubernetesSecretNamePattern.MatchString(name) {
			logx.Errorf("Tekton Workspace 名称不合法: %s", name)
			return errorx.Msg("Tekton Workspace 名称不符合 Kubernetes 命名规范")
		}
		if _, ok := seen[name]; ok {
			logx.Errorf("Tekton Workspace 重复: %s", name)
			return errorx.Msg("Tekton Workspace 重复：" + name)
		}
		seen[name] = struct{}{}
		bindingType := firstNotBlank(item.BindingType, "emptyDir")
		if !isSupportedTektonWorkspaceBindingType(bindingType) {
			logx.Errorf("Tekton Workspace 绑定类型不支持: workspace=%s type=%s", name, bindingType)
			return errorx.Msg("Tekton Workspace 绑定类型不支持：" + bindingType)
		}
		allowedBindings := normalizeTektonWorkspaceBindingTypes(item.AllowedBindingTypes)
		for _, allowedBinding := range allowedBindings {
			if !isSupportedTektonWorkspaceBindingType(allowedBinding) {
				logx.Errorf("Tekton Workspace 允许绑定类型不支持: workspace=%s type=%s", name, allowedBinding)
				return errorx.Msg("Tekton Workspace 允许绑定类型不支持：" + allowedBinding)
			}
		}
		if len(allowedBindings) == 0 {
			allowedBindings = []string{bindingType}
			item.AllowedBindingTypes = allowedBindings
		}
		if !stringInList(allowedBindings, bindingType) {
			logx.Errorf("Tekton Workspace 默认绑定类型不在允许范围内: workspace=%s type=%s allowed=%v", name, bindingType, allowedBindings)
			return errorx.Msg("Tekton Workspace 默认绑定类型不在允许范围内：" + name)
		}
		for _, sourceType := range item.AllowedSourceTypes {
			if !isSupportedTektonWorkspaceSourceType(sourceType) {
				logx.Errorf("Tekton Workspace 平台来源类型不支持: workspace=%s sourceType=%s", name, sourceType)
				return errorx.Msg("Tekton Workspace 平台来源类型不支持：" + sourceType)
			}
			if stringInList(allowedBindings, "generatedSecret") && strings.TrimSpace(sourceType) == "kubernetesResource" {
				logx.Errorf("Tekton Workspace 生成 Secret 来源类型不支持 Kubernetes 资源: workspace=%s", name)
				return errorx.Msg("Tekton Workspace 生成 Secret 只能来自平台凭证或配置中心")
			}
		}
		if strings.TrimSpace(item.ClaimName) != "" ||
			strings.TrimSpace(item.SecretName) != "" ||
			strings.TrimSpace(item.ConfigMapName) != "" ||
			strings.TrimSpace(item.ResourceID) != "" {
			logx.Errorf("Tekton Pipeline 模板不能配置具体 Workspace 资源: workspace=%s", name)
			return errorx.Msg("Tekton Pipeline 模板不能配置具体 Workspace 资源")
		}
	}
	return nil
}

func validateTektonTemplateRunPolicy(policy tektonPipelineTemplateRunPolicy) error {
	if serviceAccount := strings.TrimSpace(policy.ServiceAccountName); serviceAccount != "" && !kubernetesSecretNamePattern.MatchString(serviceAccount) {
		logx.Errorf("Tekton 运行策略 ServiceAccount 名称不合法: %s", serviceAccount)
		return errorx.Msg("Tekton 运行策略 ServiceAccount 名称不符合 Kubernetes 命名规范")
	}
	for _, item := range []struct {
		name  string
		value string
	}{
		{"timeout", policy.Timeout},
		{"pipelineTimeout", policy.PipelineTimeout},
		{"tasksTimeout", policy.TasksTimeout},
		{"finallyTimeout", policy.FinallyTimeout},
	} {
		if strings.TrimSpace(item.value) != "" && !tektonDurationPattern.MatchString(strings.TrimSpace(item.value)) {
			logx.Errorf("Tekton 运行策略超时时间格式不合法: field=%s value=%s", item.name, item.value)
			return errorx.Msg("Tekton 运行策略超时时间格式不合法：" + item.name)
		}
	}
	for key, value := range policy.Timeouts {
		switch strings.TrimSpace(key) {
		case "pipeline", "tasks", "finally":
		default:
			logx.Errorf("Tekton 运行策略 timeouts 字段不支持: %s", key)
			return errorx.Msg("Tekton 运行策略 timeouts 只支持 pipeline、tasks、finally")
		}
		if strings.TrimSpace(value) != "" && !tektonDurationPattern.MatchString(strings.TrimSpace(value)) {
			logx.Errorf("Tekton 运行策略 timeouts 格式不合法: key=%s value=%s", key, value)
			return errorx.Msg("Tekton 运行策略 timeouts 格式不合法：" + key)
		}
	}
	switch strings.TrimSpace(policy.ConcurrencyPolicy) {
	case "", "allow", "skip", "replace":
	default:
		logx.Errorf("Tekton 运行策略并发策略不支持: %s", policy.ConcurrencyPolicy)
		return errorx.Msg("Tekton 运行策略并发策略不支持")
	}
	switch strings.TrimSpace(policy.WorkspaceStrategy) {
	case "", "emptyDir", "persistentVolumeClaim", "volumeClaimTemplate":
	default:
		logx.Errorf("Tekton 运行策略 Workspace 策略不支持: %s", policy.WorkspaceStrategy)
		return errorx.Msg("Tekton 运行策略 Workspace 策略不支持")
	}
	if err := validateTektonTemplateNameValues(policy.Labels, "运行策略标签"); err != nil {
		return err
	}
	if err := validateTektonTemplateNameValues(policy.Annotations, "运行策略注解"); err != nil {
		return err
	}
	if err := validateTektonTemplateNameValues(policy.PodTemplateEnv, "Pod 环境变量"); err != nil {
		return err
	}
	if err := validateTektonTemplateYamlObject(policy.PodTemplateRaw, "podTemplate"); err != nil {
		return err
	}
	if err := validateTektonTemplateYamlObject(policy.TaskRunTemplateRaw, "taskRunTemplate"); err != nil {
		return err
	}
	if _, err := tektonTemplateRunPolicyTaskRunSpecs(policy); err != nil {
		return err
	}
	for _, workspace := range policy.Workspaces {
		if strings.TrimSpace(workspace.Name) == "" {
			logx.Errorf("Tekton 运行策略 Workspace 名称不能为空")
			return errorx.Msg("Tekton 运行策略 Workspace 名称不能为空")
		}
		if !isSupportedTektonWorkspaceBindingType(firstNotBlank(workspace.BindingType, "emptyDir")) {
			logx.Errorf("Tekton 运行策略 Workspace 绑定类型不支持: workspace=%s type=%s", workspace.Name, workspace.BindingType)
			return errorx.Msg("Tekton 运行策略 Workspace 绑定类型不支持：" + workspace.BindingType)
		}
	}
	return nil
}

func tektonTemplateRunPolicyTaskRunSpecs(policy tektonPipelineTemplateRunPolicy) ([]map[string]any, error) {
	if len(policy.TaskRunSpecs) > 0 {
		items := normalizeTektonTemplateTaskRunSpecs(policy.TaskRunSpecs)
		if err := validateTektonTemplateTaskRunSpecs(items); err != nil {
			return nil, err
		}
		return items, nil
	}
	raw := strings.TrimSpace(policy.TaskRunSpecsRaw)
	if raw == "" {
		return nil, nil
	}
	var list []map[string]any
	if err := yamlv3.Unmarshal([]byte(raw), &list); err == nil {
		list = normalizeTektonTemplateTaskRunSpecs(list)
		if err := validateTektonTemplateTaskRunSpecs(list); err != nil {
			return nil, err
		}
		return list, nil
	}
	var item map[string]any
	if err := yamlv3.Unmarshal([]byte(raw), &item); err != nil || len(item) == 0 {
		logx.Errorf("Tekton 运行策略 taskRunSpecsRaw 不是合法 YAML: %v", err)
		return nil, errorx.Msg("Tekton 运行策略 taskRunSpecsRaw 不是合法 YAML")
	}
	list = []map[string]any{item}
	list = normalizeTektonTemplateTaskRunSpecs(list)
	if err := validateTektonTemplateTaskRunSpecs(list); err != nil {
		return nil, err
	}
	return list, nil
}

func normalizeTektonTemplateTaskRunSpecs(items []map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		next := make(map[string]any, len(item))
		for key, value := range item {
			next[key] = value
		}
		aliases := map[string]string{
			"taskServiceAccountName": "serviceAccountName",
			"taskPodTemplate":        "podTemplate",
			"stepOverrides":          "stepSpecs",
			"sidecarOverrides":       "sidecarSpecs",
		}
		for legacyKey, v1Key := range aliases {
			if hasTektonTemplateTaskRunSpecValue(next[legacyKey]) && !hasTektonTemplateTaskRunSpecValue(next[v1Key]) {
				next[v1Key] = next[legacyKey]
			}
			delete(next, legacyKey)
		}
		result = append(result, next)
	}
	return result
}

func hasTektonTemplateTaskRunSpecValue(value any) bool {
	switch data := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(data) != ""
	case []any:
		return len(data) > 0
	case map[string]any:
		return len(data) > 0
	default:
		return true
	}
}

func validateTektonTemplateTaskRunSpecs(items []map[string]any) error {
	for _, item := range items {
		if strings.TrimSpace(fmt.Sprint(item["pipelineTaskName"])) == "" || fmt.Sprint(item["pipelineTaskName"]) == "<nil>" {
			logx.Errorf("Tekton 运行策略 taskRunSpecs 缺少 pipelineTaskName")
			return errorx.Msg("Tekton 运行策略 taskRunSpecs 每一项必须配置 pipelineTaskName")
		}
	}
	return nil
}

func validateTektonTemplateRunPolicyTaskRunSpecRefs(cfg tektonPipelineTemplateDagConfig) error {
	taskRunSpecs, err := tektonTemplateRunPolicyTaskRunSpecs(cfg.RunPolicy)
	if err != nil {
		return err
	}
	if len(taskRunSpecs) == 0 {
		return nil
	}
	taskNames := map[string]struct{}{}
	for _, node := range enabledTektonTemplateNodes(cfg) {
		if name := strings.TrimSpace(tektonTemplateTaskName(node)); name != "" {
			taskNames[name] = struct{}{}
		}
	}
	for _, item := range taskRunSpecs {
		name := strings.TrimSpace(fmt.Sprint(item["pipelineTaskName"]))
		if _, ok := taskNames[name]; !ok {
			logx.Errorf("Tekton 运行策略 taskRunSpecs 引用的 PipelineTask 不存在: %s", name)
			return errorx.Msg("Tekton 运行策略 taskRunSpecs 引用的 PipelineTask 不存在：" + name)
		}
	}
	return nil
}

func validateTektonTemplateTriggerConfig(cfg tektonPipelineTemplateTrigger) error {
	if !cfg.Enabled {
		return nil
	}
	for _, item := range []struct {
		name  string
		value string
	}{
		{"EventListener", cfg.EventListenerName},
		{"Trigger", cfg.TriggerName},
		{"TriggerBinding", cfg.TriggerBindingName},
		{"TriggerTemplate", cfg.TriggerTemplateName},
	} {
		if strings.TrimSpace(item.value) == "" {
			logx.Errorf("Tekton Trigger 缺少必填字段: %s", item.name)
			return errorx.Msg("Tekton Trigger 必须配置 " + item.name)
		}
		if !kubernetesSecretNamePattern.MatchString(strings.TrimSpace(item.value)) {
			logx.Errorf("Tekton Trigger 资源名称不合法: field=%s value=%s", item.name, item.value)
			return errorx.Msg("Tekton Trigger 资源名称不合法：" + item.name)
		}
	}
	if namespace := strings.TrimSpace(cfg.Namespace); namespace != "" && !kubernetesSecretNamePattern.MatchString(namespace) {
		logx.Errorf("Tekton Trigger Namespace 不合法: %s", namespace)
		return errorx.Msg("Tekton Trigger Namespace 不合法")
	}
	if serviceAccount := strings.TrimSpace(cfg.ServiceAccountName); serviceAccount != "" && !kubernetesSecretNamePattern.MatchString(serviceAccount) {
		logx.Errorf("Tekton Trigger ServiceAccount 不合法: %s", serviceAccount)
		return errorx.Msg("Tekton Trigger ServiceAccount 不合法")
	}
	switch strings.TrimSpace(cfg.APIVersion) {
	case "", "triggers.tekton.dev/v1beta1", "triggers.tekton.dev/v1alpha1":
	default:
		logx.Errorf("Tekton Trigger apiVersion 不支持: %s", cfg.APIVersion)
		return errorx.Msg("Tekton Trigger apiVersion 不支持")
	}
	switch firstNotBlank(cfg.ServiceType, "ClusterIP") {
	case "ClusterIP", "NodePort", "LoadBalancer":
	default:
		logx.Errorf("Tekton EventListener Service 类型不支持: %s", cfg.ServiceType)
		return errorx.Msg("Tekton EventListener Service 类型不支持")
	}
	switch firstNotBlank(cfg.InterceptorType, "none") {
	case "none", "github", "gitlab":
	case "cel":
		if strings.TrimSpace(cfg.CELFilter) == "" {
			logx.Errorf("Tekton CEL interceptor 缺少过滤表达式")
			return errorx.Msg("Tekton CEL interceptor 必须配置过滤表达式")
		}
	default:
		logx.Errorf("Tekton Trigger interceptor 不支持: %s", cfg.InterceptorType)
		return errorx.Msg("Tekton Trigger interceptor 不支持")
	}
	for _, item := range cfg.BindingParams {
		if strings.TrimSpace(item.Name) == "" || strings.TrimSpace(item.Value) == "" {
			logx.Errorf("Tekton TriggerBinding 参数不完整")
			return errorx.Msg("Tekton TriggerBinding 参数不完整")
		}
	}
	for _, item := range cfg.TemplateParams {
		if strings.TrimSpace(item.Name) == "" {
			logx.Errorf("Tekton TriggerTemplate 参数名称不能为空")
			return errorx.Msg("Tekton TriggerTemplate 参数名称不能为空")
		}
	}
	if err := validateTektonTemplateNameValues(cfg.Labels, "Trigger 标签"); err != nil {
		return err
	}
	return validateTektonTemplateNameValues(cfg.Annotations, "Trigger 注解")
}

func validateTektonTemplateScheduleConfig(cfg tektonPipelineTemplateSchedule) error {
	if !cfg.Enabled {
		return nil
	}
	if strings.TrimSpace(cfg.Cron) == "" {
		logx.Errorf("Tekton 定时 Cron 不能为空")
		return errorx.Msg("Tekton 定时 Cron 不能为空")
	}
	if strings.TrimSpace(cfg.Timezone) == "" {
		logx.Errorf("Tekton 定时时区不能为空")
		return errorx.Msg("Tekton 定时时区不能为空")
	}
	if _, err := time.LoadLocation(strings.TrimSpace(cfg.Timezone)); err != nil {
		logx.Errorf("Tekton 定时时区不合法: %s", cfg.Timezone)
		return errorx.Msg("Tekton 定时时区不合法")
	}
	cron := strings.TrimSpace(cfg.Cron)
	if !strings.HasPrefix(cron, "@") && len(strings.Fields(cron)) != 5 {
		logx.Errorf("Tekton 定时 Cron 格式不合法: %s", cron)
		return errorx.Msg("Tekton 定时 Cron 需要使用 5 段标准表达式")
	}
	switch strings.TrimSpace(cfg.ConcurrencyPolicy) {
	case "", "allow", "skip", "replace":
	default:
		logx.Errorf("Tekton 定时并发策略不支持: %s", cfg.ConcurrencyPolicy)
		return errorx.Msg("Tekton 定时并发策略不支持")
	}
	return nil
}

func validateTektonTemplateYamlObject(raw, name string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var item map[string]any
	if err := yamlv3.Unmarshal([]byte(raw), &item); err != nil || len(item) == 0 {
		logx.Errorf("Tekton 运行策略 %s 不是合法 YAML 对象: %v", name, err)
		return errorx.Msg("Tekton 运行策略 " + name + " 不是合法 YAML 对象")
	}
	return nil
}

func validateTektonTemplatePrunerPolicy(policy tektonPipelineTemplatePruner) error {
	if !policy.Enabled {
		return nil
	}
	switch firstNotBlank(policy.Scope, "pipeline") {
	case "pipeline", "namespace":
	default:
		logx.Errorf("Tekton Pruner scope 不支持: %s", policy.Scope)
		return errorx.Msg("Tekton Pruner scope 不支持")
	}
	switch firstNotBlank(policy.NativePrunerMode, "disabled") {
	case "disabled", "namespaceConfigMap", "globalConfigMap":
	default:
		logx.Errorf("Tekton 原生 Pruner 模式不支持: %s", policy.NativePrunerMode)
		return errorx.Msg("Tekton 原生 Pruner 模式不支持")
	}
	if namespace := strings.TrimSpace(policy.NativePrunerTargetNamespace); namespace != "" && !kubernetesSecretNamePattern.MatchString(namespace) {
		logx.Errorf("Tekton 原生 Pruner Namespace 不合法: %s", namespace)
		return errorx.Msg("Tekton 原生 Pruner Namespace 不合法")
	}
	switch firstNotBlank(policy.NativePrunerConfigLevel, "namespace") {
	case "namespace", "global":
	default:
		logx.Errorf("Tekton 原生 Pruner 配置级别不支持: %s", policy.NativePrunerConfigLevel)
		return errorx.Msg("Tekton 原生 Pruner 配置级别不支持")
	}
	if len(policy.ResourceTypes) == 0 {
		logx.Errorf("Tekton Pruner 必须选择清理资源类型")
		return errorx.Msg("Tekton Pruner 必须选择清理资源类型")
	}
	for _, item := range policy.ResourceTypes {
		switch item {
		case "PipelineRun", "TaskRun":
		default:
			logx.Errorf("Tekton Pruner 资源类型不支持: %s", item)
			return errorx.Msg("Tekton Pruner 资源类型不支持：" + item)
		}
	}
	for _, item := range []struct {
		name  string
		value *int
	}{
		{"historyLimit", policy.HistoryLimit},
		{"successfulHistoryLimit", policy.SuccessfulHistoryLimit},
		{"failedHistoryLimit", policy.FailedHistoryLimit},
		{"ttlSecondsAfterFinished", policy.TTLSecondsAfterFinished},
		{"platformRecordRetentionDays", policy.PlatformRecordRetentionDays},
		{"platformRecordLimit", policy.PlatformRecordLimit},
	} {
		if item.value != nil && *item.value < 0 {
			logx.Errorf("Tekton Pruner 数值不能小于 0: field=%s value=%d", item.name, *item.value)
			return errorx.Msg("Tekton Pruner 数值不能小于 0：" + item.name)
		}
	}
	if err := validateTektonTemplateNameValues(policy.SelectorLabels, "Pruner 标签选择器"); err != nil {
		return err
	}
	return validateTektonTemplateNameValues(policy.SelectorAnnotations, "Pruner 注解选择器")
}

func validateTektonTemplateNameValues(items []tektonPipelineTemplateNameVal, label string) error {
	for _, item := range items {
		if strings.TrimSpace(item.Name) == "" || strings.TrimSpace(fmt.Sprint(item.Value)) == "" {
			logx.Errorf("Tekton %s 配置不完整", label)
			return errorx.Msg("Tekton " + label + " 配置不完整")
		}
	}
	return nil
}

func validateTektonTemplateStringMap(items map[string]string, label string) error {
	for key, value := range items {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			logx.Errorf("Tekton %s 配置不完整", label)
			return errorx.Msg("Tekton " + label + " 配置不完整")
		}
	}
	return nil
}

func normalizeTektonWorkspaceBindingTypes(items []string) []string {
	result := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func isSupportedTektonWorkspaceBindingType(value string) bool {
	switch strings.TrimSpace(value) {
	case "emptyDir", "persistentVolumeClaim", "volumeClaimTemplate", "dynamicPVC", "secret", "generatedSecret", "configMap", "projected":
		return true
	default:
		return false
	}
}

func isSupportedTektonWorkspaceSourceType(value string) bool {
	switch strings.TrimSpace(value) {
	case "kubernetesResource", "platformCredential", "configCenter":
		return true
	default:
		return false
	}
}

func stringInList(items []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, item := range items {
		if strings.TrimSpace(item) == target {
			return true
		}
	}
	return false
}

func extractTektonTemplateParamRefs(values ...any) []string {
	result := make([]string, 0)
	seen := map[string]struct{}{}
	const prefix = "$(params."
	for _, raw := range values {
		value := fmt.Sprint(raw)
		for {
			index := strings.Index(value, prefix)
			if index < 0 {
				break
			}
			rest := value[index+len(prefix):]
			end := strings.Index(rest, ")")
			if end < 0 {
				break
			}
			name := strings.TrimSpace(rest[:end])
			if name != "" {
				if _, ok := seen[name]; !ok {
					seen[name] = struct{}{}
					result = append(result, name)
				}
			}
			value = rest[end+1:]
		}
	}
	return result
}

type tektonTemplateResultRef struct {
	TaskName   string
	ResultName string
}

func extractTektonTemplateResultRefs(value string) []tektonTemplateResultRef {
	result := make([]tektonTemplateResultRef, 0)
	for _, prefix := range []string{"$(tasks.", "$(finally."} {
		rest := value
		for {
			index := strings.Index(rest, prefix)
			if index < 0 {
				break
			}
			part := rest[index+len(prefix):]
			taskEnd := strings.Index(part, ".results.")
			if taskEnd < 0 {
				rest = part
				continue
			}
			taskName := strings.TrimSpace(part[:taskEnd])
			resultPart := part[taskEnd+len(".results."):]
			resultEnd := strings.Index(resultPart, ")")
			if resultEnd < 0 {
				break
			}
			resultName := strings.TrimSpace(resultPart[:resultEnd])
			if taskName != "" && resultName != "" {
				result = append(result, tektonTemplateResultRef{TaskName: taskName, ResultName: resultName})
			}
			rest = resultPart[resultEnd+1:]
		}
	}
	return result
}

func validateTektonTemplateNodes(ctx context.Context, svcCtx *svc.ServiceContext, cfg tektonPipelineTemplateDagConfig, roles []string) error {
	nodes := enabledTektonTemplateNodes(cfg)
	if len(enabledTektonTemplateMainNodes(cfg)) == 0 {
		logx.Errorf("Tekton Pipeline 模板至少需要一个启用 Task")
		return errorx.Msg("Tekton Pipeline 模板至少需要一个启用 Task")
	}
	ids := map[string]struct{}{}
	names := map[string]struct{}{}
	specByID := map[string]tektonTemplateTaskSpec{}
	specByTaskName := map[string]tektonTemplateTaskSpec{}
	paramNames := tektonTemplateParamNameSet(cfg)
	workspaces := tektonTemplateWorkspaceByName(cfg)
	for _, node := range nodes {
		id := strings.TrimSpace(node.ID)
		if id == "" {
			logx.Errorf("Tekton PipelineTask 节点 ID 不能为空")
			return errorx.Msg("Tekton PipelineTask 节点 ID 不能为空")
		}
		if _, ok := ids[id]; ok {
			logx.Errorf("Tekton PipelineTask 节点 ID 重复: %s", id)
			return errorx.Msg("Tekton PipelineTask 节点 ID 重复：" + id)
		}
		ids[id] = struct{}{}
		name := tektonTemplateTaskName(node)
		if name == "" {
			logx.Errorf("Tekton PipelineTask 名称不能为空")
			return errorx.Msg("Tekton PipelineTask 名称不能为空")
		}
		if !kubernetesSecretNamePattern.MatchString(name) {
			logx.Errorf("Tekton PipelineTask 名称不合法: %s", name)
			return errorx.Msg("Tekton PipelineTask 名称不符合 Kubernetes 命名规范")
		}
		if _, ok := names[name]; ok {
			logx.Errorf("Tekton PipelineTask 名称重复: %s", name)
			return errorx.Msg("Tekton PipelineTask 名称重复：" + name)
		}
		names[name] = struct{}{}
		spec, err := validateTektonTemplateTaskRef(ctx, svcCtx, node, roles, paramNames)
		if err != nil {
			return err
		}
		specByID[id] = spec
		specByTaskName[name] = spec
		if err := validateTektonTemplateNodeParams(node, spec, paramNames); err != nil {
			return err
		}
		if err := validateTektonTemplateNodeWorkspaces(node, spec, workspaces, paramNames); err != nil {
			return err
		}
		if err := validateTektonTemplateNodeExecution(node, paramNames); err != nil {
			return err
		}
	}
	return validateTektonTemplateResultRefs(cfg, specByID, specByTaskName)
}

func validateTektonTemplateNodeExecution(node tektonPipelineTemplateNode, pipelineParams map[string]struct{}) error {
	name := tektonTemplateTaskName(node)
	if node.Retries != nil && *node.Retries < 0 {
		logx.Errorf("Tekton PipelineTask retries 不能小于 0: task=%s retries=%d", name, *node.Retries)
		return errorx.Msg("Tekton PipelineTask retries 不能小于 0：" + name)
	}
	if timeout := strings.TrimSpace(node.Timeout); timeout != "" && !tektonDurationPattern.MatchString(timeout) {
		logx.Errorf("Tekton PipelineTask timeout 格式不合法: task=%s timeout=%s", name, timeout)
		return errorx.Msg("Tekton PipelineTask timeout 格式不合法：" + name)
	}
	if onError := strings.TrimSpace(node.OnError); onError != "" && onError != "continue" && onError != "stopAndFail" {
		logx.Errorf("Tekton PipelineTask onError 不支持: task=%s onError=%s", name, onError)
		return errorx.Msg("Tekton PipelineTask onError 只支持 continue 或 stopAndFail：" + name)
	}
	for _, item := range node.When {
		if strings.TrimSpace(item.CEL) != "" {
			for _, paramName := range extractTektonTemplateParamRefs(item.CEL) {
				if _, ok := pipelineParams[paramName]; !ok {
					logx.Errorf("Tekton PipelineTask When CEL 引用的 Pipeline 参数未声明: task=%s ref=%s", name, paramName)
					return errorx.Msg("Tekton PipelineTask When CEL 引用的 Pipeline 参数未声明：" + paramName)
				}
			}
			continue
		}
		operator := firstNotBlank(item.Operator, "in")
		if strings.TrimSpace(item.Input) == "" || (operator != "in" && operator != "notin") || len(item.Values) == 0 {
			logx.Errorf("Tekton PipelineTask When 表达式不完整: task=%s", name)
			return errorx.Msg("Tekton PipelineTask When 表达式不完整：" + name)
		}
		if !tektonTemplateAllNonEmptyStrings(item.Values) {
			logx.Errorf("Tekton PipelineTask When values 必须是非空字符串: task=%s", name)
			return errorx.Msg("Tekton PipelineTask When values 必须是非空字符串：" + name)
		}
		for _, paramName := range extractTektonTemplateParamRefs(item.Input) {
			if _, ok := pipelineParams[paramName]; !ok {
				logx.Errorf("Tekton PipelineTask When 引用的 Pipeline 参数未声明: task=%s ref=%s", name, paramName)
				return errorx.Msg("Tekton PipelineTask When 引用的 Pipeline 参数未声明：" + paramName)
			}
		}
	}
	if len(node.Matrix) > 0 {
		if err := validateTektonTemplateMatrix(node.Matrix, name); err != nil {
			return err
		}
	}
	return nil
}

func validateTektonTemplateMatrix(raw map[string]any, taskName string) error {
	params, ok := templateAnySlice(raw["params"])
	if !ok {
		params, ok = templateAnySlice(raw["Params"])
	}
	if !ok || len(params) == 0 {
		logx.Errorf("Tekton PipelineTask Matrix 必须配置 params: task=%s", taskName)
		return errorx.Msg("Tekton PipelineTask Matrix 必须配置 params：" + taskName)
	}
	for _, rawParam := range params {
		param, ok := rawParam.(map[string]any)
		if !ok {
			logx.Errorf("Tekton PipelineTask Matrix params 格式不正确: task=%s", taskName)
			return errorx.Msg("Tekton PipelineTask Matrix params 格式不正确：" + taskName)
		}
		value := firstTemplateValue(param, "value", "values", "Value", "Values")
		if strings.TrimSpace(fmt.Sprint(param["name"])) == "" || !hasTemplateParamValue(value) {
			logx.Errorf("Tekton PipelineTask Matrix 参数必须配置 name 和 value: task=%s", taskName)
			return errorx.Msg("Tekton PipelineTask Matrix 参数必须配置 name 和 value：" + taskName)
		}
		values, ok := templateAnySlice(value)
		if !ok || !tektonTemplateAllNonEmptyStrings(values) {
			logx.Errorf("Tekton PipelineTask Matrix value 必须是非空字符串数组: task=%s", taskName)
			return errorx.Msg("Tekton PipelineTask Matrix value 必须是非空字符串数组：" + taskName)
		}
	}
	includes, ok := templateAnySlice(raw["include"])
	if !ok {
		includes, ok = templateAnySlice(raw["Include"])
	}
	if ok {
		for _, rawInclude := range includes {
			include, ok := rawInclude.(map[string]any)
			if !ok {
				logx.Errorf("Tekton PipelineTask Matrix include 格式不正确: task=%s", taskName)
				return errorx.Msg("Tekton PipelineTask Matrix include 格式不正确：" + taskName)
			}
			includeParams, ok := templateAnySlice(include["params"])
			if !ok {
				includeParams, ok = templateAnySlice(include["Params"])
			}
			if !ok || len(includeParams) == 0 {
				logx.Errorf("Tekton PipelineTask Matrix include 必须配置 params: task=%s", taskName)
				return errorx.Msg("Tekton PipelineTask Matrix include 必须配置 params：" + taskName)
			}
			for _, rawParam := range includeParams {
				param, ok := rawParam.(map[string]any)
				if !ok {
					logx.Errorf("Tekton PipelineTask Matrix include params 格式不正确: task=%s", taskName)
					return errorx.Msg("Tekton PipelineTask Matrix include params 格式不正确：" + taskName)
				}
				value := firstTemplateValue(param, "value", "values", "Value", "Values")
				if strings.TrimSpace(fmt.Sprint(param["name"])) == "" || !tektonTemplateNonEmptyString(value) {
					logx.Errorf("Tekton PipelineTask Matrix include 参数必须配置 name 和 value: task=%s", taskName)
					return errorx.Msg("Tekton PipelineTask Matrix include 参数必须配置 name 和 value：" + taskName)
				}
			}
		}
	}
	return nil
}

func tektonTemplateMatrixParamNameSet(raw map[string]any) map[string]struct{} {
	result := map[string]struct{}{}
	params, ok := templateAnySlice(raw["params"])
	if !ok {
		params, ok = templateAnySlice(raw["Params"])
	}
	if !ok {
		return result
	}
	for _, rawParam := range params {
		param, ok := rawParam.(map[string]any)
		if !ok {
			continue
		}
		if name := strings.TrimSpace(fmt.Sprint(param["name"])); name != "" {
			result[name] = struct{}{}
		}
	}
	return result
}

func tektonTemplateMatrixAllParamNameSet(raw map[string]any) map[string]struct{} {
	result := tektonTemplateMatrixParamNameSet(raw)
	includes, ok := templateAnySlice(raw["include"])
	if !ok {
		includes, ok = templateAnySlice(raw["Include"])
	}
	if !ok {
		return result
	}
	for _, rawInclude := range includes {
		include, ok := rawInclude.(map[string]any)
		if !ok {
			continue
		}
		params, ok := templateAnySlice(include["params"])
		if !ok {
			params, ok = templateAnySlice(include["Params"])
		}
		if !ok {
			continue
		}
		for _, rawParam := range params {
			param, ok := rawParam.(map[string]any)
			if !ok {
				continue
			}
			if name := strings.TrimSpace(fmt.Sprint(param["name"])); name != "" {
				result[name] = struct{}{}
			}
		}
	}
	return result
}

func validateTektonTemplateTaskRef(ctx context.Context, svcCtx *svc.ServiceContext, node tektonPipelineTemplateNode, roles []string, pipelineParams map[string]struct{}) (tektonTemplateTaskSpec, error) {
	refCount := 0
	if hasTektonTemplateTaskRef(node) {
		refCount++
	}
	if hasTektonTemplatePipelineRef(node.PipelineRef) {
		refCount++
	}
	if len(node.TaskSpec) > 0 {
		refCount++
	}
	if len(node.PipelineSpec) > 0 {
		refCount++
	}
	if refCount == 0 {
		logx.Errorf("Tekton PipelineTask 引用不能为空")
		return tektonTemplateTaskSpec{}, errorx.Msg("Tekton PipelineTask 引用不能为空")
	}
	if refCount > 1 {
		logx.Errorf("Tekton PipelineTask taskRef、pipelineRef、taskSpec、pipelineSpec 只能配置一个")
		return tektonTemplateTaskSpec{}, errorx.Msg("Tekton PipelineTask taskRef、pipelineRef、taskSpec、pipelineSpec 只能配置一个")
	}
	if len(node.TaskSpec) > 0 {
		logx.Errorf("平台默认禁止 Tekton inline taskSpec")
		return tektonTemplateTaskSpec{}, errorx.Msg("平台默认禁止 Tekton inline taskSpec")
	}
	if len(node.PipelineSpec) > 0 {
		logx.Errorf("平台默认禁止 Tekton inline pipelineSpec")
		return tektonTemplateTaskSpec{}, errorx.Msg("平台默认禁止 Tekton inline pipelineSpec")
	}
	if hasTektonTemplatePipelineRef(node.PipelineRef) {
		if err := validateTektonTemplatePipelineRef(node.PipelineRef, pipelineParams); err != nil {
			return tektonTemplateTaskSpec{}, err
		}
		if !isSuperAdminRole(roles) && (strings.TrimSpace(node.PipelineRef.Resolver) != "" || len(node.PipelineRef.Params) > 0) {
			logx.Errorf("普通用户不能使用 Pipeline Resolver")
			return tektonTemplateTaskSpec{}, errorx.Msg("普通用户不能使用 Pipeline Resolver")
		}
		return tektonTemplateTaskSpecFromRequirements(node), nil
	}
	ref := node.TaskRef
	ref.Name = firstNotBlank(ref.Name, node.TaskName, node.StepCode, node.TaskCode)
	ref.Kind = firstNotBlank(ref.Kind, "Task")
	if strings.TrimSpace(ref.Bundle) != "" {
		logx.Errorf("Tekton taskRef.bundle 已废弃")
		return tektonTemplateTaskSpec{}, errorx.Msg("Tekton taskRef.bundle 已废弃，请使用 resolver")
	}
	kind := strings.TrimSpace(ref.Kind)
	kindLower := strings.ToLower(kind)
	apiVersion := strings.TrimSpace(ref.APIVersion)
	resolver := strings.TrimSpace(ref.Resolver)
	isResolver := resolver != "" || strings.TrimSpace(ref.Namespace) != "" || len(ref.Params) > 0
	isCustomTask := apiVersion != "" || (kindLower != "task" && kindLower != "clustertask")
	if !tektonTaskKindPattern.MatchString(kind) {
		logx.Errorf("Tekton taskRef.kind 不合法: %s", ref.Kind)
		return tektonTemplateTaskSpec{}, errorx.Msg("Tekton taskRef.kind 不合法")
	}
	if apiVersion != "" && !tektonAPIVersionPattern.MatchString(apiVersion) {
		logx.Errorf("Tekton taskRef.apiVersion 不合法: %s", apiVersion)
		return tektonTemplateTaskSpec{}, errorx.Msg("Tekton taskRef.apiVersion 不合法")
	}
	if kindLower != "task" && kindLower != "clustertask" && apiVersion == "" {
		logx.Errorf("Tekton Custom Task 必须配置 apiVersion: kind=%s", ref.Kind)
		return tektonTemplateTaskSpec{}, errorx.Msg("Tekton Custom Task 必须配置 apiVersion")
	}
	for _, item := range ref.Params {
		for _, paramName := range extractTektonTemplateParamRefs(item.Value) {
			if _, ok := pipelineParams[paramName]; !ok {
				logx.Errorf("Tekton taskRef 参数引用的 Pipeline 参数未声明: %s", paramName)
				return tektonTemplateTaskSpec{}, errorx.Msg("Tekton taskRef 参数引用的 Pipeline 参数未声明：" + paramName)
			}
		}
	}
	if isResolver {
		if err := validateTektonTemplateResolverRef(ref); err != nil {
			return tektonTemplateTaskSpec{}, err
		}
	}
	if kindLower == "clustertask" || isResolver || isCustomTask {
		if !isSuperAdminRole(roles) {
			logx.Errorf("普通用户不能使用 ClusterTask、Resolver Task 或 Custom Task")
			return tektonTemplateTaskSpec{}, errorx.Msg("普通用户不能使用 ClusterTask、Resolver Task 或 Custom Task")
		}
		return tektonTemplateTaskSpecFromRequirements(node), nil
	}
	if kindLower != "" && kindLower != "task" {
		logx.Errorf("Tekton taskRef.kind 不支持: %s", ref.Kind)
		return tektonTemplateTaskSpec{}, errorx.Msg("Tekton taskRef.kind 只支持 Task 或 ClusterTask")
	}
	step, err := tektonTemplateTaskFromRepo(ctx, svcCtx, node)
	if err != nil {
		return tektonTemplateTaskSpec{}, err
	}
	resource, err := devopstekton.ParseStepResource(step.StageContent)
	if err != nil {
		logx.Errorf("Tekton Task 内容不合法: %v", err)
		return tektonTemplateTaskSpec{}, errorx.Msg(err.Error())
	}
	taskName := strings.TrimSpace(resource.Name)
	if ref.Name != "" && taskName != "" && ref.Name != taskName {
		logx.Errorf("Tekton taskRef.name 与 Task 仓库资源名不一致: ref=%s task=%s", ref.Name, taskName)
		return tektonTemplateTaskSpec{}, errorx.Msg("Tekton taskRef.name 必须和 Task 仓库资源名一致")
	}
	return tektonTemplateTaskSpecFromResource(resource.Object.Object)
}

func hasTektonTemplateTaskRef(node tektonPipelineTemplateNode) bool {
	ref := node.TaskRef
	kind := strings.TrimSpace(ref.Kind)
	return strings.TrimSpace(ref.Name) != "" ||
		strings.TrimSpace(ref.APIVersion) != "" ||
		strings.TrimSpace(ref.Bundle) != "" ||
		strings.TrimSpace(ref.Resolver) != "" ||
		strings.TrimSpace(ref.Namespace) != "" ||
		len(ref.Params) > 0 ||
		(kind != "" && kind != "Task")
}

func hasTektonTemplatePipelineRef(ref tektonPipelineTemplatePipelineRef) bool {
	return strings.TrimSpace(ref.Name) != "" ||
		strings.TrimSpace(ref.APIVersion) != "" ||
		strings.TrimSpace(ref.Bundle) != "" ||
		strings.TrimSpace(ref.Resolver) != "" ||
		len(ref.Params) > 0
}

func tektonTemplateTaskSpecFromRequirements(node tektonPipelineTemplateNode) tektonTemplateTaskSpec {
	return tektonTemplateTaskSpec{
		Params:     tektonTemplateParamRequirementSet(node.ParamRequirements),
		Workspaces: tektonTemplateWorkspaceRequirementSet(node.WorkspaceRequirements),
		Results:    tektonTemplateStringSet(node.Results),
	}
}

func validateTektonTemplatePipelineRef(ref tektonPipelineTemplatePipelineRef, pipelineParams map[string]struct{}) error {
	if strings.TrimSpace(ref.Bundle) != "" {
		logx.Errorf("Tekton pipelineRef.bundle 已废弃")
		return errorx.Msg("Tekton pipelineRef.bundle 已废弃，请使用 resolver")
	}
	if apiVersion := strings.TrimSpace(ref.APIVersion); apiVersion != "" && !tektonAPIVersionPattern.MatchString(apiVersion) {
		logx.Errorf("Tekton pipelineRef.apiVersion 不合法: %s", apiVersion)
		return errorx.Msg("Tekton pipelineRef.apiVersion 不合法")
	}
	resolver := strings.TrimSpace(ref.Resolver)
	isResolver := resolver != "" || len(ref.Params) > 0
	if !isResolver {
		name := strings.TrimSpace(ref.Name)
		if name == "" {
			logx.Errorf("Tekton pipelineRef.name 不能为空")
			return errorx.Msg("Tekton pipelineRef.name 不能为空")
		}
		if !kubernetesSecretNamePattern.MatchString(name) {
			logx.Errorf("Tekton pipelineRef.name 不合法: %s", name)
			return errorx.Msg("Tekton pipelineRef.name 不合法：" + name)
		}
		return nil
	}
	if resolver == "" {
		resolver = "cluster"
	}
	if !kubernetesSecretNamePattern.MatchString(resolver) {
		logx.Errorf("Tekton pipelineRef resolver 名称不合法: %s", resolver)
		return errorx.Msg("Tekton pipelineRef resolver 名称不合法：" + resolver)
	}
	if len(ref.Params) == 0 {
		logx.Errorf("Tekton pipelineRef resolver params 不能为空")
		return errorx.Msg("Tekton pipelineRef resolver params 不能为空")
	}
	for _, item := range ref.Params {
		name := strings.TrimSpace(item.Name)
		value := strings.TrimSpace(fmt.Sprint(item.Value))
		if name == "" || value == "" {
			logx.Errorf("Tekton pipelineRef resolver params 必须包含 name 和 value")
			return errorx.Msg("Tekton pipelineRef resolver params 必须包含 name 和 value")
		}
		for _, paramName := range extractTektonTemplateParamRefs(item.Value) {
			if _, ok := pipelineParams[paramName]; !ok {
				logx.Errorf("Tekton pipelineRef 参数引用的 Pipeline 参数未声明: %s", paramName)
				return errorx.Msg("Tekton pipelineRef 参数引用的 Pipeline 参数未声明：" + paramName)
			}
		}
	}
	return nil
}

func validateTektonTemplateResolverRef(ref tektonPipelineTemplateTaskRef) error {
	resolver := strings.TrimSpace(ref.Resolver)
	if resolver == "" {
		resolver = "cluster"
	}
	if !kubernetesSecretNamePattern.MatchString(resolver) {
		logx.Errorf("Tekton resolver 名称不合法: %s", resolver)
		return errorx.Msg("Tekton resolver 名称不合法：" + resolver)
	}
	paramMap := map[string]string{}
	for _, item := range ref.Params {
		name := strings.TrimSpace(item.Name)
		value := strings.TrimSpace(fmt.Sprint(item.Value))
		if name == "" || value == "" {
			logx.Errorf("Tekton resolver params 必须包含 name 和 value")
			return errorx.Msg("Tekton resolver params 必须包含 name 和 value")
		}
		paramMap[name] = value
	}
	if resolver != "cluster" || len(ref.Params) == 0 {
		return nil
	}
	resolvedKind := strings.ToLower(strings.TrimSpace(paramMap["kind"]))
	resolvedName := strings.TrimSpace(paramMap["name"])
	if resolvedKind == "" || resolvedName == "" {
		logx.Errorf("Tekton cluster resolver 必须包含 kind 和 name")
		return errorx.Msg("Tekton cluster resolver 必须包含 kind 和 name")
	}
	if resolvedKind != "task" && resolvedKind != "clustertask" {
		logx.Errorf("Tekton cluster resolver kind 只支持 task 或 clustertask: %s", resolvedKind)
		return errorx.Msg("Tekton cluster resolver kind 只支持 task 或 clustertask")
	}
	if !kubernetesSecretNamePattern.MatchString(resolvedName) {
		logx.Errorf("Tekton cluster resolver name 不合法: %s", resolvedName)
		return errorx.Msg("Tekton cluster resolver name 不合法：" + resolvedName)
	}
	if namespace := strings.TrimSpace(paramMap["namespace"]); namespace != "" && !kubernetesSecretNamePattern.MatchString(namespace) {
		logx.Errorf("Tekton cluster resolver namespace 不合法: %s", namespace)
		return errorx.Msg("Tekton cluster resolver namespace 不合法：" + namespace)
	}
	return nil
}

func tektonTemplateTaskFromRepo(ctx context.Context, svcCtx *svc.ServiceContext, node tektonPipelineTemplateNode) (*model.DevopsStepTemplate, error) {
	if id := strings.TrimSpace(node.TaskID); id != "" {
		step, err := svcCtx.StepTemplateModel.FindOne(ctx, id)
		if err != nil {
			logx.Errorf("Tekton Task 仓库记录不存在: %v", err)
			return nil, errorx.Msg("Tekton Task 仓库记录不存在")
		}
		if step.EngineType != tektonEngineType {
			logx.Errorf("Task 仓库记录不是 Tekton Task: %s", id)
			return nil, errorx.Msg("Task 仓库记录不是 Tekton Task")
		}
		return step, nil
	}
	code := firstNotBlank(node.TaskCode, node.StepCode, node.TaskRef.Name)
	if strings.TrimSpace(code) == "" {
		logx.Errorf("Tekton taskRef.name 不能为空")
		return nil, errorx.Msg("Tekton taskRef.name 不能为空")
	}
	step, err := svcCtx.StepTemplateModel.FindOneByCode(ctx, tektonEngineType, code)
	if err != nil {
		logx.Errorf("Tekton taskRef.name 必须存在于 Task 仓库: %v", err)
		return nil, errorx.Msg("Tekton taskRef.name 必须存在于 Task 仓库")
	}
	return step, nil
}

func tektonTemplateTaskSpecFromResource(obj map[string]any) (tektonTemplateTaskSpec, error) {
	spec := tektonTemplateTaskSpec{
		Params:     map[string]bool{},
		Workspaces: map[string]bool{},
		Results:    map[string]bool{},
	}
	rawParams, _, err := unstructured.NestedSlice(obj, "spec", "params")
	if err != nil {
		logx.Errorf("读取 Tekton Task 参数失败: %v", err)
		return spec, err
	}
	for _, item := range rawParams {
		param, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(fmt.Sprint(param["name"]))
		if name == "" {
			continue
		}
		_, hasDefault := param["default"]
		spec.Params[name] = hasDefault
	}
	rawWorkspaces, _, err := unstructured.NestedSlice(obj, "spec", "workspaces")
	if err != nil {
		logx.Errorf("读取 Tekton Task workspace 失败: %v", err)
		return spec, err
	}
	for _, item := range rawWorkspaces {
		workspace, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(fmt.Sprint(workspace["name"]))
		if name != "" {
			spec.Workspaces[name] = workspace["optional"] == true
		}
	}
	rawResults, _, err := unstructured.NestedSlice(obj, "spec", "results")
	if err != nil {
		logx.Errorf("读取 Tekton Task result 失败: %v", err)
		return spec, err
	}
	for _, item := range rawResults {
		result, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(fmt.Sprint(result["name"]))
		if name != "" {
			spec.Results[name] = true
		}
	}
	return spec, nil
}

func validateTektonTemplateNodeParams(node tektonPipelineTemplateNode, spec tektonTemplateTaskSpec, pipelineParams map[string]struct{}) error {
	bindings := map[string]tektonPipelineTemplateTaskParam{}
	matrixParamNames := tektonTemplateMatrixParamNameSet(node.Matrix)
	requirementByName := map[string]tektonPipelineTemplateParamRequire{}
	for _, item := range node.ParamRequirements {
		name := strings.TrimSpace(item.Name)
		if name != "" {
			requirementByName[name] = item
		}
	}
	if len(spec.Params) > 0 {
		for name := range tektonTemplateMatrixAllParamNameSet(node.Matrix) {
			if _, ok := spec.Params[name]; !ok {
				logx.Errorf("PipelineTask Matrix 参数不属于 Task spec.params: task=%s param=%s", tektonTemplateTaskName(node), name)
				return errorx.Msg("PipelineTask Matrix 参数不属于 Task spec.params：" + name)
			}
		}
	}
	for _, item := range node.Params {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		bindings[name] = item
		if _, ok := spec.Params[name]; !ok && len(spec.Params) > 0 {
			logx.Errorf("PipelineTask 参数不属于 Task spec.params: task=%s param=%s", tektonTemplateTaskName(node), name)
			return errorx.Msg("PipelineTask 参数不属于 Task spec.params：" + name)
		}
		source := strings.TrimSpace(item.Source)
		switch source {
		case "pipelineParam", "runtime":
			paramName := strings.TrimSpace(item.PipelineParam)
			if paramName == "" {
				paramName = strings.TrimSpace(fmt.Sprint(item.Value))
			}
			if _, ok := pipelineParams[paramName]; !ok {
				logx.Errorf("PipelineTask 引用的 Pipeline 参数未声明: %s", paramName)
				return errorx.Msg("PipelineTask 引用的 Pipeline 参数未声明：" + paramName)
			}
		case "result":
			if strings.TrimSpace(item.ResultNodeID) == "" || strings.TrimSpace(item.ResultName) == "" {
				logx.Errorf("PipelineTask result 参数引用不完整")
				return errorx.Msg("PipelineTask result 参数引用不完整")
			}
		case "", "fixed", "dynamic":
			if isTektonSensitiveParamType(item.ParamType) && source != "dynamic" {
				logx.Errorf("凭证类资源不能写入普通 Pipeline 参数: %s", item.Name)
				return errorx.Msg("凭证类资源不能写入普通 Pipeline 参数")
			}
			if source != "dynamic" {
				requirement := requirementByName[name]
				paramType := firstNotBlank(requirement.Type, requirement.ParamType, item.ParamType)
				if value, ok := firstTektonTemplateProvidedValue(item.Value, item.DefaultValue, requirement.DefaultValue); ok {
					if err := validateTektonTemplateTypedParamValue(paramType, value, tektonTemplateTaskName(node)+"."+name); err != nil {
						logx.Errorf("PipelineTask 参数值类型不匹配: task=%s param=%s err=%v", tektonTemplateTaskName(node), name, err)
						return err
					}
				}
			}
		default:
			logx.Errorf("PipelineTask 参数来源不支持: %s", source)
			return errorx.Msg("PipelineTask 参数来源不支持")
		}
		for _, paramName := range extractTektonTemplateParamRefs(item.Value, item.DefaultValue) {
			if _, ok := pipelineParams[paramName]; !ok {
				logx.Errorf("PipelineTask 参数值引用的 Pipeline 参数未声明: task=%s param=%s ref=%s", tektonTemplateTaskName(node), name, paramName)
				return errorx.Msg("PipelineTask 参数值引用的 Pipeline 参数未声明：" + paramName)
			}
		}
	}
	for name, hasDefault := range spec.Params {
		if hasDefault {
			continue
		}
		if _, ok := bindings[name]; !ok {
			if _, matrixBound := matrixParamNames[name]; matrixBound {
				continue
			}
			logx.Errorf("PipelineTask 缺少 Task 必填参数绑定: task=%s param=%s", tektonTemplateTaskName(node), name)
			return errorx.Msg("PipelineTask 缺少 Task 必填参数绑定：" + name)
		}
	}
	for _, item := range node.ParamRequirements {
		name := strings.TrimSpace(item.Name)
		if name == "" || !item.Required || hasTektonTemplateProvidedValue(item.DefaultValue) {
			continue
		}
		if _, ok := bindings[name]; !ok {
			if _, matrixBound := matrixParamNames[name]; matrixBound {
				continue
			}
			logx.Errorf("PipelineTask 缺少必填参数绑定: task=%s param=%s", tektonTemplateTaskName(node), name)
			return errorx.Msg("PipelineTask 缺少必填参数绑定：" + name)
		}
	}
	return nil
}

func validateTektonTemplateNodeWorkspaces(node tektonPipelineTemplateNode, spec tektonTemplateTaskSpec, pipelineWorkspaces map[string]tektonPipelineTemplateWorkspace, pipelineParams map[string]struct{}) error {
	bindings := map[string]struct{}{}
	boundPipelineWorkspaces := map[string]tektonPipelineTemplateWorkspace{}
	for _, item := range node.Workspaces {
		taskWorkspace := strings.TrimSpace(item.TaskWorkspace)
		pipelineWorkspace := strings.TrimSpace(item.PipelineWorkspace)
		if taskWorkspace == "" || pipelineWorkspace == "" {
			logx.Errorf("PipelineTask workspace 映射不完整")
			return errorx.Msg("PipelineTask workspace 映射不完整")
		}
		if _, ok := bindings[taskWorkspace]; ok {
			logx.Errorf("PipelineTask workspace 重复映射: task=%s workspace=%s", tektonTemplateTaskName(node), taskWorkspace)
			return errorx.Msg("PipelineTask workspace 重复映射：" + taskWorkspace)
		}
		if _, ok := spec.Workspaces[taskWorkspace]; !ok && len(spec.Workspaces) > 0 {
			logx.Errorf("PipelineTask workspace 不属于 Task spec.workspaces: %s", taskWorkspace)
			return errorx.Msg("PipelineTask workspace 不属于 Task spec.workspaces：" + taskWorkspace)
		}
		pipelineWorkspaceData, ok := pipelineWorkspaces[pipelineWorkspace]
		if !ok {
			logx.Errorf("PipelineTask workspace 引用的 Pipeline workspace 未声明: %s", pipelineWorkspace)
			return errorx.Msg("PipelineTask workspace 引用的 Pipeline workspace 未声明：" + pipelineWorkspace)
		}
		if requiredBindingType := strings.TrimSpace(item.RequiredBindingType); requiredBindingType != "" {
			if !isSupportedTektonWorkspaceBindingType(requiredBindingType) {
				logx.Errorf("PipelineTask workspace 要求的绑定类型不支持: %s", requiredBindingType)
				return errorx.Msg("PipelineTask workspace 要求的绑定类型不支持：" + requiredBindingType)
			}
			bindingType := firstNotBlank(pipelineWorkspaceData.BindingType, "emptyDir")
			if bindingType != requiredBindingType {
				logx.Errorf("PipelineTask workspace 绑定类型不匹配: task=%s workspace=%s required=%s actual=%s", tektonTemplateTaskName(node), taskWorkspace, requiredBindingType, bindingType)
				return errorx.Msg("PipelineTask workspace 绑定类型不匹配：" + taskWorkspace)
			}
		}
		for _, paramName := range extractTektonTemplateParamRefs(item.SubPath) {
			if _, ok := pipelineParams[paramName]; !ok {
				logx.Errorf("PipelineTask workspace subPath 引用的 Pipeline 参数未声明: task=%s workspace=%s ref=%s", tektonTemplateTaskName(node), taskWorkspace, paramName)
				return errorx.Msg("PipelineTask workspace subPath 引用的 Pipeline 参数未声明：" + paramName)
			}
		}
		bindings[taskWorkspace] = struct{}{}
		boundPipelineWorkspaces[taskWorkspace] = pipelineWorkspaceData
	}
	for name, optional := range spec.Workspaces {
		if optional {
			continue
		}
		if _, ok := bindings[name]; !ok {
			logx.Errorf("PipelineTask 缺少必填 workspace 映射: task=%s workspace=%s", tektonTemplateTaskName(node), name)
			return errorx.Msg("PipelineTask 缺少必填 workspace 映射：" + name)
		}
	}
	for _, item := range node.WorkspaceRequirements {
		name := strings.TrimSpace(item.Name)
		requiredBindingType := strings.TrimSpace(item.RequiredBindingType)
		if requiredBindingType != "" && !isSupportedTektonWorkspaceBindingType(requiredBindingType) {
			logx.Errorf("PipelineTask workspace 要求的绑定类型不支持: %s", requiredBindingType)
			return errorx.Msg("PipelineTask workspace 要求的绑定类型不支持：" + requiredBindingType)
		}
		if name == "" || (item.Optional && requiredBindingType == "") {
			continue
		}
		if _, ok := bindings[name]; !ok {
			logx.Errorf("PipelineTask 缺少必填 workspace 映射: task=%s workspace=%s", tektonTemplateTaskName(node), name)
			return errorx.Msg("PipelineTask 缺少必填 workspace 映射：" + name)
		}
		if requiredBindingType != "" {
			bindingType := firstNotBlank(boundPipelineWorkspaces[name].BindingType, "emptyDir")
			if bindingType != requiredBindingType {
				logx.Errorf("PipelineTask workspace 绑定类型不匹配: task=%s workspace=%s required=%s actual=%s", tektonTemplateTaskName(node), name, requiredBindingType, bindingType)
				return errorx.Msg("PipelineTask workspace 绑定类型不匹配：" + name)
			}
		}
	}
	return nil
}

func validateTektonTemplateResultRefs(cfg tektonPipelineTemplateDagConfig, specByID, specByTaskName map[string]tektonTemplateTaskSpec) error {
	for _, edge := range cfg.Edges {
		if strings.TrimSpace(edge.Type) != "result" {
			continue
		}
		source := strings.TrimSpace(edge.Source)
		resultName := strings.TrimSpace(edge.ResultName)
		if source == "" || resultName == "" {
			logx.Errorf("Tekton Result 引用不完整")
			return errorx.Msg("Tekton Result 引用不完整")
		}
		if _, ok := specByID[source].Results[resultName]; !ok {
			logx.Errorf("Tekton Result 引用不存在: task=%s result=%s", source, resultName)
			return errorx.Msg("Tekton Result 引用不存在：" + resultName)
		}
	}
	for _, node := range enabledTektonTemplateNodes(cfg) {
		for _, param := range node.Params {
			if strings.TrimSpace(param.Source) != "result" {
				continue
			}
			source := strings.TrimSpace(param.ResultNodeID)
			resultName := strings.TrimSpace(param.ResultName)
			if source == "" || resultName == "" {
				logx.Errorf("Tekton Result 参数引用不完整")
				return errorx.Msg("Tekton Result 参数引用不完整")
			}
			if _, ok := specByID[source].Results[resultName]; !ok {
				logx.Errorf("Tekton Result 参数引用不存在: task=%s result=%s", source, resultName)
				return errorx.Msg("Tekton Result 参数引用不存在：" + resultName)
			}
		}
	}
	for _, result := range tektonTemplatePipelineResults(cfg) {
		resultName := strings.TrimSpace(result.Name)
		for _, ref := range extractTektonTemplateResultRefs(fmt.Sprint(result.Value)) {
			spec, ok := specByTaskName[ref.TaskName]
			if !ok {
				logx.Errorf("Tekton Pipeline Result 引用的 Task 不存在: result=%s task=%s", resultName, ref.TaskName)
				return errorx.Msg("Tekton Pipeline Result 引用的 Task 不存在：" + ref.TaskName)
			}
			if _, ok := spec.Results[ref.ResultName]; !ok {
				logx.Errorf("Tekton Pipeline Result 引用不存在: result=%s task=%s taskResult=%s", resultName, ref.TaskName, ref.ResultName)
				return errorx.Msg("Tekton Pipeline Result 引用不存在：" + ref.TaskName + "." + ref.ResultName)
			}
		}
	}
	return nil
}

func validateTektonTemplateEdges(cfg tektonPipelineTemplateDagConfig) error {
	nodes := enabledTektonTemplateNodes(cfg)
	ids := map[string]struct{}{}
	paramNames := tektonTemplateParamNameSet(cfg)
	for _, node := range nodes {
		ids[strings.TrimSpace(node.ID)] = struct{}{}
	}
	mainIDs := map[string]struct{}{}
	for _, node := range enabledTektonTemplateMainNodes(cfg) {
		mainIDs[strings.TrimSpace(node.ID)] = struct{}{}
	}
	graph := map[string][]string{}
	for _, edge := range cfg.Edges {
		source := strings.TrimSpace(edge.Source)
		target := strings.TrimSpace(edge.Target)
		edgeType := strings.TrimSpace(edge.Type)
		if edgeType == "" {
			edgeType = "runAfter"
		}
		switch edgeType {
		case "runAfter", "result", "when", "finally":
		default:
			logx.Errorf("Tekton DAG 边类型不支持: %s", edgeType)
			return errorx.Msg("Tekton DAG 边类型不支持：" + edgeType)
		}
		if source == "" || target == "" {
			logx.Errorf("Tekton DAG 边必须包含 source 和 target")
			return errorx.Msg("Tekton DAG 边必须包含 source 和 target")
		}
		if _, ok := ids[source]; !ok {
			logx.Errorf("Tekton DAG 边 source 不存在: %s", source)
			return errorx.Msg("Tekton DAG 边 source 不存在")
		}
		if _, ok := ids[target]; !ok {
			logx.Errorf("Tekton DAG 边 target 不存在: %s", target)
			return errorx.Msg("Tekton DAG 边 target 不存在")
		}
		if source == target {
			logx.Errorf("Tekton DAG 不能自依赖: %s", source)
			return errorx.Msg("Tekton DAG 不能自依赖")
		}
		if edgeType == "finally" {
			continue
		}
		if _, ok := mainIDs[source]; !ok {
			logx.Errorf("普通 Task 不能依赖 Finally Task: source=%s", source)
			return errorx.Msg("普通 Task 不能依赖 Finally Task")
		}
		if edgeType == "result" && (strings.TrimSpace(edge.ResultName) == "" || strings.TrimSpace(edge.TargetParam) == "") {
			logx.Errorf("Tekton DAG Result 边必须配置 Result 和目标参数")
			return errorx.Msg("Tekton DAG Result 边必须配置 Result 和目标参数")
		}
		if edgeType == "when" {
			operator := firstNotBlank(edge.WhenOperator, "in")
			if operator != "in" && operator != "notin" {
				logx.Errorf("Tekton DAG When 边 operator 不支持: %s", operator)
				return errorx.Msg("Tekton DAG When 边 operator 只支持 in/notin")
			}
			if strings.TrimSpace(edge.WhenInput) == "" || len(edge.WhenValues) == 0 {
				logx.Errorf("Tekton DAG When 边必须配置 input 和 values")
				return errorx.Msg("Tekton DAG When 边必须配置 input 和 values")
			}
			if !tektonTemplateAllNonEmptyStrings(edge.WhenValues) {
				logx.Errorf("Tekton DAG When 边 values 必须是非空字符串")
				return errorx.Msg("Tekton DAG When 边 values 必须是非空字符串")
			}
			for _, paramName := range extractTektonTemplateParamRefs(edge.WhenInput) {
				if _, ok := paramNames[paramName]; !ok {
					logx.Errorf("Tekton DAG When 边引用的 Pipeline 参数未声明: %s", paramName)
					return errorx.Msg("Tekton DAG When 边引用的 Pipeline 参数未声明：" + paramName)
				}
			}
		}
		graph[source] = append(graph[source], target)
	}
	if tektonTemplateHasCycle(graph) {
		logx.Errorf("Tekton DAG 不能存在循环依赖")
		return errorx.Msg("Tekton DAG 不能存在循环依赖")
	}
	return nil
}

func tektonTemplateHasCycle(graph map[string][]string) bool {
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var walk func(string) bool
	walk = func(node string) bool {
		if visiting[node] {
			return true
		}
		if visited[node] {
			return false
		}
		visiting[node] = true
		for _, next := range graph[node] {
			if walk(next) {
				return true
			}
		}
		visiting[node] = false
		visited[node] = true
		return false
	}
	for node := range graph {
		if walk(node) {
			return true
		}
	}
	return false
}

func tektonTemplatePipelineParams(cfg tektonPipelineTemplateDagConfig) []tektonPipelineTemplateParam {
	items := make([]tektonPipelineTemplateParam, 0, len(cfg.Pipeline.Params)+len(cfg.Params))
	items = append(items, cfg.Pipeline.Params...)
	items = append(items, cfg.Params...)
	return items
}

func tektonTemplatePipelineResults(cfg tektonPipelineTemplateDagConfig) []tektonPipelineTemplateResult {
	items := make([]tektonPipelineTemplateResult, 0, len(cfg.Pipeline.Results)+len(cfg.Results))
	items = append(items, cfg.Pipeline.Results...)
	items = append(items, cfg.Results...)
	return items
}

func tektonTemplatePipelineWorkspaces(cfg tektonPipelineTemplateDagConfig) []tektonPipelineTemplateWorkspace {
	items := make([]tektonPipelineTemplateWorkspace, 0, len(cfg.Pipeline.Workspaces)+len(cfg.Workspaces))
	items = append(items, cfg.Pipeline.Workspaces...)
	items = append(items, cfg.Workspaces...)
	return items
}

func tektonTemplateParamNameSet(cfg tektonPipelineTemplateDagConfig) map[string]struct{} {
	result := map[string]struct{}{}
	for _, item := range tektonTemplatePipelineParams(cfg) {
		if name := strings.TrimSpace(item.Name); name != "" {
			result[name] = struct{}{}
		}
	}
	return result
}

func tektonTemplateWorkspaceByName(cfg tektonPipelineTemplateDagConfig) map[string]tektonPipelineTemplateWorkspace {
	result := map[string]tektonPipelineTemplateWorkspace{}
	for _, item := range tektonTemplatePipelineWorkspaces(cfg) {
		if name := strings.TrimSpace(item.Name); name != "" {
			result[name] = item
		}
	}
	return result
}

func enabledTektonTemplateMainNodes(cfg tektonPipelineTemplateDagConfig) []tektonPipelineTemplateNode {
	result := make([]tektonPipelineTemplateNode, 0, len(cfg.Nodes))
	for _, node := range cfg.Nodes {
		if !tektonTemplateNodeEnabled(node) {
			continue
		}
		if tektonTemplateNodeIsNote(node) || tektonTemplateNodeIsFinally(node) {
			continue
		}
		result = append(result, node)
	}
	return result
}

func enabledTektonTemplateNodes(cfg tektonPipelineTemplateDagConfig) []tektonPipelineTemplateNode {
	result := enabledTektonTemplateMainNodes(cfg)
	for _, node := range cfg.FinallyNodes {
		if !tektonTemplateNodeEnabled(node) || tektonTemplateNodeIsNote(node) {
			continue
		}
		result = append(result, node)
	}
	for _, node := range cfg.Nodes {
		if !tektonTemplateNodeEnabled(node) || !tektonTemplateNodeIsFinally(node) || tektonTemplateNodeIsNote(node) {
			continue
		}
		result = append(result, node)
	}
	return result
}

func tektonTemplateNodeEnabled(node tektonPipelineTemplateNode) bool {
	return node.Enabled == nil || *node.Enabled
}

func tektonTemplateNodeIsNote(node tektonPipelineTemplateNode) bool {
	return strings.TrimSpace(node.Type) == "note"
}

func tektonTemplateNodeIsFinally(node tektonPipelineTemplateNode) bool {
	nodeType := strings.ToLower(strings.TrimSpace(node.Type))
	return nodeType == "finally" || nodeType == "finallytask"
}

func tektonTemplateTaskName(node tektonPipelineTemplateNode) string {
	return firstNotBlank(node.Name, node.TaskName, node.DisplayName, node.TaskRef.Name, node.TaskCode, node.StepCode)
}

func tektonTemplateParamRequirementSet(items []tektonPipelineTemplateParamRequire) map[string]bool {
	result := map[string]bool{}
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name != "" {
			result[name] = !item.Required || hasTektonTemplateProvidedValue(item.DefaultValue)
		}
	}
	return result
}

func tektonTemplateWorkspaceRequirementSet(items []tektonPipelineTemplateWorkspaceReq) map[string]bool {
	result := map[string]bool{}
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name != "" {
			result[name] = item.Optional
		}
	}
	return result
}

func tektonTemplateStringSet(items []string) map[string]bool {
	result := map[string]bool{}
	for _, item := range items {
		if name := strings.TrimSpace(item); name != "" {
			result[name] = true
		}
	}
	return result
}

func tektonParamType(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "string"
	}
	return value
}

func tektonOfficialParamType(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "string", true
	}
	switch value {
	case "string", "array", "object":
		return value, true
	default:
		return "", false
	}
}

func isTektonSensitiveParamType(paramType string) bool {
	switch strings.TrimSpace(paramType) {
	case "password", "voucherModel", channelvars.ParamChannelCredential, channelvars.ParamKubernetesSecretName:
		return true
	default:
		return false
	}
}

func templateAnySlice(value any) ([]any, bool) {
	items, ok := value.([]any)
	return items, ok
}

func firstTemplateValue(values map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			return value
		}
	}
	return nil
}

func firstNotBlank(items ...string) string {
	for _, item := range items {
		if value := strings.TrimSpace(item); value != "" {
			return value
		}
	}
	return ""
}

func validateJenkinsPostStepContent(content string) (string, error) {
	if stageCallPattern.MatchString(content) || parallelBlockPattern.MatchString(content) || stepsBlockPattern.MatchString(content) {
		logx.Errorf("post 步骤只能保存 post 条件块，不能包含 stage、parallel 或 steps")
		return "", errorx.Msg("post 步骤只能保存 post 条件块，不能包含 stage、parallel 或 steps")
	}
	if body, ok := extractWholeGroovyNamedBlock(content, "post"); ok {
		if err := validatePostConditions(body); err != nil {
			logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
			return "", err
		}
		return content, nil
	}
	if err := validatePostConditions(content); err != nil {
		logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
		return "", err
	}
	return content, nil
}

func validatePostConditions(content string) error {
	rest := strings.TrimSpace(content)
	if rest == "" {
		logx.Errorf("post 步骤条件不能为空")
		return errorx.Msg("post 步骤条件不能为空")
	}
	seen := make(map[string]struct{})
	for rest != "" {
		match := postConditionPattern.FindStringSubmatchIndex(rest)
		if match == nil || match[0] != 0 {
			logx.Errorf("post 步骤只允许 always、success、failure、changed、aborted、unstable 条件块")
			return errorx.Msg("post 步骤只允许 always、success、failure、changed、aborted、unstable 条件块")
		}
		condition := rest[match[2]:match[3]]
		if _, ok := seen[condition]; ok {
			logx.Errorf("%s", "post 条件 "+condition+" 只能配置一次")
			return errorx.Msg("post 条件 " + condition + " 只能配置一次")
		}
		seen[condition] = struct{}{}
		openIndex := strings.Index(rest[match[0]:match[1]], "{")
		if openIndex < 0 {
			logx.Errorf("post 条件代码块不完整")
			return errorx.Msg("post 条件代码块不完整")
		}
		openIndex += match[0]
		closeIndex := findMatchingBrace(rest, openIndex)
		if closeIndex < 0 {
			logx.Errorf("post 条件代码块不完整")
			return errorx.Msg("post 条件代码块不完整")
		}
		rest = strings.TrimSpace(rest[closeIndex+1:])
	}
	return nil
}

func validateTemplateSteps(ctx context.Context, svcCtx *svc.ServiceContext, engineType string, steps []model.PipelineTemplateStep) ([]model.PipelineTemplateStep, error) {
	engineType = normalizeTemplateEngineType(engineType)
	for idx := range steps {
		step, err := svcCtx.StepTemplateModel.FindOne(ctx, steps[idx].StepID)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				logx.Errorf("步骤不存在")
				return nil, errorx.Msg("步骤不存在")
			}
			logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
			return nil, err
		}
		if step.EngineType != engineType {
			logx.Errorf("步骤引擎必须和模板引擎一致")
			return nil, errorx.Msg("步骤引擎必须和模板引擎一致")
		}
		nodeName := strings.TrimSpace(steps[idx].NodeName)
		if nodeName == "" {
			nodeName = strings.TrimSpace(steps[idx].StepName)
		}
		if nodeName == "" {
			nodeName = step.Name
		}
		branchType := normalizeBranchType(steps[idx].BranchType)
		if err := validateBranchType(branchType); err != nil {
			logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
			return nil, err
		}
		steps[idx].StepCode = step.Code
		steps[idx].StepName = step.Name
		steps[idx].NodeName = nodeName
		steps[idx].StepType = normalizeStepType(step.Type)
		steps[idx].Icon = step.Icon
		steps[idx].IconColor = step.IconColor
		steps[idx].ParentNodeID = strings.TrimSpace(steps[idx].ParentNodeID)
		steps[idx].BranchType = branchType
		if steps[idx].ID == "" {
			steps[idx].ID = fmt.Sprintf("node-%d", idx+1)
		}
		steps[idx].ParamValues = ""
		if err := validateStepArtifactConfig(steps[idx].ArtifactConfig); err != nil {
			logx.Errorf("校验模板步骤产物配置失败: step=%s err=%v", nodeName, err)
			return nil, errorx.Msg("步骤产物配置错误：" + nodeName)
		}
	}
	steps = normalizePipelineStepGraph(steps)
	ensureUniqueTemplateStepNodeNames(steps)
	enabledTaskCount := 0
	for _, item := range steps {
		if item.Enabled && normalizeStepType(item.StepType) == stepTypeTask {
			enabledTaskCount++
		}
	}
	if enabledTaskCount == 0 {
		logx.Errorf("流水线模板至少需要一个启用 Task")
		return nil, errorx.Msg("流水线模板至少需要一个启用 Task")
	}
	if err := validatePipelineStepCategoryPlacement(ctx, svcCtx, steps); err != nil {
		logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
		return nil, err
	}
	if err := validateTemplatePostConditionUniqueness(ctx, svcCtx, steps); err != nil {
		logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
		return nil, err
	}
	return steps, nil
}

func validateTemplateStepRequiredParams(step model.PipelineTemplateStep, params []model.DevopsStepParam) error {
	if len(params) == 0 {
		return nil
	}
	values := make(map[string]any)
	if strings.TrimSpace(step.ParamValues) != "" {
		if err := json.Unmarshal([]byte(step.ParamValues), &values); err != nil {
			logx.Errorf("步骤参数配置必须是合法 JSON")
			return errorx.Msg("步骤参数配置必须是合法 JSON")
		}
	}
	for _, item := range params {
		if item.ParamType == objectListParamType {
			if err := validateTemplateObjectListValue(item, values[item.Code], pipelineNodeName(step)); err != nil {
				logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
				return err
			}
			continue
		}
		if item.ParamType == channelvars.ParamChannelCredential {
			continue
		}
		if item.Mode == "params" && item.RuntimeConfig {
			continue
		}
		if !item.Required {
			continue
		}
		if hasTemplateParamValue(values[item.Code]) || strings.TrimSpace(item.DefaultValue) != "" {
			continue
		}
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = item.Code
		}
		logx.Errorf("%s", "步骤「"+pipelineNodeName(step)+"」缺少必填参数："+name)
		return errorx.Msg("步骤「" + pipelineNodeName(step) + "」缺少必填参数：" + name)
	}
	return nil
}

func validateTemplateObjectListValue(item model.DevopsStepParam, raw any, nodeName string) error {
	fields := normalizeStepParamConfig(item).CompoundFields
	if !item.Required && raw == nil {
		return nil
	}
	rows, ok := raw.([]any)
	if !ok {
		if item.Required {
			logx.Errorf("%s", "步骤「"+nodeName+"」缺少必填参数："+item.Name)
			return errorx.Msg("步骤「" + nodeName + "」缺少必填参数：" + item.Name)
		}
		logx.Errorf("%s", "步骤「"+nodeName+"」参数「"+item.Name+"」必须是对象数组")
		return errorx.Msg("步骤「" + nodeName + "」参数「" + item.Name + "」必须是对象数组")
	}
	if item.Required && len(rows) == 0 {
		logx.Errorf("%s", "步骤「"+nodeName+"」缺少必填参数："+item.Name)
		return errorx.Msg("步骤「" + nodeName + "」缺少必填参数：" + item.Name)
	}
	for _, row := range rows {
		rowMap, ok := row.(map[string]any)
		if !ok {
			logx.Errorf("%s", "步骤「"+nodeName+"」参数「"+item.Name+"」必须是对象数组")
			return errorx.Msg("步骤「" + nodeName + "」参数「" + item.Name + "」必须是对象数组")
		}
		for _, field := range fields {
			if field.ParamType == channelvars.ParamChannelCredential {
				continue
			}
			if !field.Required {
				continue
			}
			if hasTemplateParamValue(rowMap[field.Code]) || strings.TrimSpace(field.DefaultValue) != "" {
				continue
			}
			logx.Errorf("%s", "步骤「"+nodeName+"」参数「"+item.Name+"」缺少必填字段："+field.Name)
			return errorx.Msg("步骤「" + nodeName + "」参数「" + item.Name + "」缺少必填字段：" + field.Name)
		}
	}
	return nil
}

func hasTemplateParamValue(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(v) != ""
	case []any:
		return len(v) > 0
	default:
		return true
	}
}

func tektonTemplateNonEmptyString(value any) bool {
	text, ok := value.(string)
	return ok && strings.TrimSpace(text) != ""
}

func tektonTemplateAllNonEmptyStrings(values []any) bool {
	if len(values) == 0 {
		return false
	}
	for _, value := range values {
		if !tektonTemplateNonEmptyString(value) {
			return false
		}
	}
	return true
}

func tektonTemplateTypedParamType(paramType string) string {
	switch strings.TrimSpace(paramType) {
	case "array", "list":
		return "array"
	case "object", objectListParamType:
		return "object"
	default:
		return "string"
	}
}

func validateTektonTemplateTypedParamValue(paramType string, raw any, name string) error {
	switch tektonTemplateTypedParamType(paramType) {
	case "array":
		switch data := raw.(type) {
		case []any:
			if !tektonTemplateAllStrings(data) {
				return errorx.Msg("Tekton PipelineTask 参数值不是合法 string array：" + name)
			}
			return nil
		case []string:
			return nil
		}
		value := strings.TrimSpace(fmt.Sprint(raw))
		if value == "" || strings.Contains(value, "$(") {
			return nil
		}
		var parsed any
		_ = yamlv3.Unmarshal([]byte(value), &parsed)
		if items, ok := parsed.([]any); ok {
			if !tektonTemplateAllStrings(items) {
				return errorx.Msg("Tekton PipelineTask 参数值不是合法 string array：" + name)
			}
			return nil
		}
		for _, part := range strings.Split(value, ",") {
			if strings.TrimSpace(part) != "" {
				return nil
			}
		}
		return errorx.Msg("Tekton PipelineTask 参数值不是合法 array：" + name)
	case "object":
		switch raw.(type) {
		case map[string]any, map[string]string:
			return nil
		}
		value := strings.TrimSpace(fmt.Sprint(raw))
		if value == "" || strings.Contains(value, "$(") {
			return nil
		}
		var parsed any
		if err := yamlv3.Unmarshal([]byte(value), &parsed); err != nil {
			return errorx.Msg("Tekton PipelineTask 参数值不是合法 object：" + name)
		}
		if _, ok := parsed.(map[string]any); ok {
			return nil
		}
		return errorx.Msg("Tekton PipelineTask 参数值不是合法 object：" + name)
	default:
		return nil
	}
}

func tektonTemplateAllStrings(items []any) bool {
	for _, item := range items {
		if _, ok := item.(string); !ok {
			return false
		}
	}
	return true
}

func firstTektonTemplateProvidedValue(values ...any) (any, bool) {
	for _, value := range values {
		if hasTektonTemplateProvidedValue(value) {
			return value, true
		}
	}
	return nil, false
}

func hasTektonTemplateProvidedValue(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(v) != ""
	default:
		return true
	}
}

func ensureUniqueTemplateStepNodeNames(steps []model.PipelineTemplateStep) {
	used := map[string]struct{}{}
	for index := range steps {
		name := pipelineNodeName(steps[index])
		name = uniqueTemplateStepNodeName(name, used)
		used[name] = struct{}{}
		steps[index].NodeName = name
	}
}

func uniqueTemplateStepNodeName(base string, used map[string]struct{}) string {
	if _, ok := used[base]; !ok {
		return base
	}
	for index := 1; ; index++ {
		name := fmt.Sprintf("%s-%d", base, index)
		if _, ok := used[name]; !ok {
			return name
		}
	}
}

func validatePipelineStepCategoryPlacement(ctx context.Context, svcCtx *svc.ServiceContext, steps []model.PipelineTemplateStep) error {
	postNodeIDs := pipelinePostNodeIDs(steps)
	for _, item := range steps {
		step, err := svcCtx.StepTemplateModel.FindOne(ctx, item.StepID)
		if err != nil {
			logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
			return err
		}
		category, err := svcCtx.StepCategoryModel.FindOne(ctx, step.CategoryID)
		if err != nil {
			logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
			return err
		}
		_, isPostNode := postNodeIDs[item.ID]
		if isPostNode && category.Code != stepCategoryCodePost {
			logx.Errorf("Post 阶段只能选择 post 步骤分组中的步骤")
			return errorx.Msg("Post 阶段只能选择 post 步骤分组中的步骤")
		}
		if !isPostNode && category.Code == stepCategoryCodePost {
			logx.Errorf("普通流水线阶段不能选择 post 步骤分组中的步骤")
			return errorx.Msg("普通流水线阶段不能选择 post 步骤分组中的步骤")
		}
	}
	return nil
}

func validateTemplatePostConditionUniqueness(ctx context.Context, svcCtx *svc.ServiceContext, steps []model.PipelineTemplateStep) error {
	postNodeIDs := pipelinePostNodeIDs(steps)
	if len(postNodeIDs) == 0 {
		return nil
	}
	postSteps := make([]jenkins.JenkinsRenderStep, 0, len(postNodeIDs))
	for _, item := range steps {
		if !item.Enabled {
			continue
		}
		if _, ok := postNodeIDs[item.ID]; !ok {
			continue
		}
		step, err := svcCtx.StepTemplateModel.FindOne(ctx, item.StepID)
		if err != nil {
			logx.Errorf("查询 post 步骤失败: %v", err)
			return err
		}
		postSteps = append(postSteps, jenkins.JenkinsRenderStep{
			ID:           item.ID,
			NodeName:     pipelineNodeName(item),
			Enabled:      item.Enabled,
			StageContent: step.StageContent,
		})
	}
	if len(postSteps) == 0 {
		return nil
	}
	if err := jenkins.ValidatePostConditionUniqueness(postSteps); err != nil {
		logx.Errorf("Post 阶段条件重复: %v", err)
		return errorx.Msg(err.Error())
	}
	return nil
}

func renderJenkinsPipeline(ctx context.Context, svcCtx *svc.ServiceContext, template *model.DevopsPipelineTemplate) (string, error) {
	steps := append([]model.PipelineTemplateStep(nil), template.Steps...)
	steps = normalizePipelineStepGraph(steps)
	postNodeIDs := pipelinePostNodeIDs(steps)
	stepByNode := make(map[string]*model.DevopsStepTemplate, len(steps))
	stepRefs := make([]templateStepParamRef, 0, len(steps))
	for _, item := range steps {
		step, err := svcCtx.StepTemplateModel.FindOne(ctx, item.StepID)
		if err != nil {
			logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
			return "", err
		}
		stepByNode[item.ID] = step
		stepRefs = append(stepRefs, templateStepParamRef{Node: item, Template: step})
	}
	codeMaps := templateStepParamCodeMaps(stepRefs)
	renderSteps := make([]jenkins.JenkinsRenderStep, 0, len(steps))
	for _, item := range steps {
		if !item.Enabled {
			continue
		}
		step := stepByNode[item.ID]
		parentID := strings.TrimSpace(item.ParentNodeID)
		if parentID == "" {
			parentID = workflowStartNodeID
		}
		if _, ok := postNodeIDs[item.ID]; ok {
			parentID = workflowPostNodeID
		}
		renderSteps = append(renderSteps, jenkins.JenkinsRenderStep{
			ID:           item.ID,
			NodeName:     pipelineNodeName(item),
			StepType:     normalizeStepType(step.Type),
			ParentNodeID: parentID,
			BranchType:   strings.TrimSpace(item.BranchType),
			SortOrder:    item.SortOrder,
			Enabled:      item.Enabled,
			StageContent: step.StageContent,
			ParamCodeMap: codeMaps[item.ID],
		})
	}
	return jenkins.RenderDeclarativePipeline(jenkins.RenderPipeline{
		Agent: jenkins.JenkinsRenderAgent{Type: "any"},
		Steps: renderSteps,
	})
}

func pipelinePostNodeIDs(items []model.PipelineTemplateStep) map[string]struct{} {
	byID := make(map[string]model.PipelineTemplateStep, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}
	result := make(map[string]struct{})
	for _, item := range items {
		parentID := strings.TrimSpace(item.ParentNodeID)
		visited := map[string]struct{}{}
		for parentID != "" && parentID != workflowStartNodeID {
			if parentID == workflowPostNodeID {
				result[item.ID] = struct{}{}
				break
			}
			if _, ok := visited[parentID]; ok {
				break
			}
			visited[parentID] = struct{}{}
			parent, ok := byID[parentID]
			if !ok {
				break
			}
			parentID = strings.TrimSpace(parent.ParentNodeID)
		}
	}
	return result
}

func splitPipelineStepsByPost(items []model.PipelineTemplateStep, postNodeIDs map[string]struct{}) ([]model.PipelineTemplateStep, []model.PipelineTemplateStep) {
	stageSteps := make([]model.PipelineTemplateStep, 0, len(items))
	postSteps := make([]model.PipelineTemplateStep, 0)
	for _, item := range items {
		if _, ok := postNodeIDs[item.ID]; ok {
			postSteps = append(postSteps, item)
			continue
		}
		stageSteps = append(stageSteps, item)
	}
	return stageSteps, postSteps
}

func enabledPipelineSteps(items []model.PipelineTemplateStep) []model.PipelineTemplateStep {
	result := make([]model.PipelineTemplateStep, 0, len(items))
	for _, item := range items {
		if item.Enabled {
			result = append(result, item)
		}
	}
	return result
}

func pipelineChildrenByParent(items []model.PipelineTemplateStep) map[string][]model.PipelineTemplateStep {
	allByID := make(map[string]model.PipelineTemplateStep, len(items))
	enabledByID := make(map[string]struct{}, len(items))
	for _, item := range items {
		allByID[item.ID] = item
		if item.Enabled {
			enabledByID[item.ID] = struct{}{}
		}
	}

	children := make(map[string][]model.PipelineTemplateStep, len(items))
	for _, item := range items {
		if !item.Enabled {
			continue
		}
		parentID := enabledPipelineParentID(item.ParentNodeID, allByID, enabledByID)
		children[parentID] = append(children[parentID], item)
	}
	for parentID := range children {
		sort.SliceStable(children[parentID], func(i, j int) bool {
			return children[parentID][i].SortOrder < children[parentID][j].SortOrder
		})
	}
	return children
}

func enabledPipelineParentID(parentID string, allByID map[string]model.PipelineTemplateStep, enabledByID map[string]struct{}) string {
	parentID = strings.TrimSpace(parentID)
	if parentID == "" {
		return workflowStartNodeID
	}
	if parentID == workflowPostNodeID {
		return workflowPostNodeID
	}
	for parentID != workflowStartNodeID && parentID != workflowPostNodeID {
		if _, ok := enabledByID[parentID]; ok {
			return parentID
		}
		parent, ok := allByID[parentID]
		if !ok {
			return workflowStartNodeID
		}
		parentID = strings.TrimSpace(parent.ParentNodeID)
		if parentID == "" {
			return workflowStartNodeID
		}
	}
	if parentID == workflowPostNodeID {
		return workflowPostNodeID
	}
	return workflowStartNodeID
}

func pipelineNodeName(item model.PipelineTemplateStep) string {
	name := strings.TrimSpace(item.NodeName)
	if name == "" {
		name = strings.TrimSpace(item.StepName)
	}
	if name == "" {
		name = "未命名步骤"
	}
	return name
}

func renderPostPipelineSteps(ctx context.Context, svcCtx *svc.ServiceContext, builder *strings.Builder, steps []model.PipelineTemplateStep, indent int) error {
	orderedSteps := flattenPostPipelineSteps(steps)
	grouped := map[string][]string{}
	rawBlocks := make([]string, 0)
	for _, item := range orderedSteps {
		step, err := svcCtx.StepTemplateModel.FindOne(ctx, item.StepID)
		if err != nil {
			logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
			return err
		}
		condition, body, rawPost := postStepContent(step.StageContent, pipelineNodeName(item))
		if rawPost {
			rawBlocks = append(rawBlocks, body)
			continue
		}
		grouped[condition] = append(grouped[condition], body)
	}

	conditionOrder := []string{"always", "success", "failure", "changed", "aborted", "unstable"}
	written := map[string]struct{}{}
	for _, condition := range conditionOrder {
		bodies := grouped[condition]
		if len(bodies) == 0 {
			continue
		}
		writeIndent(builder, indent)
		builder.WriteString(condition + " {\n")
		for _, body := range bodies {
			writeIndentedBlock(builder, body, indent+2)
		}
		writeIndent(builder, indent)
		builder.WriteString("}\n")
		written[condition] = struct{}{}
	}
	for condition, bodies := range grouped {
		if _, ok := written[condition]; ok {
			continue
		}
		writeIndent(builder, indent)
		builder.WriteString(condition + " {\n")
		for _, body := range bodies {
			writeIndentedBlock(builder, body, indent+2)
		}
		writeIndent(builder, indent)
		builder.WriteString("}\n")
	}
	for _, body := range rawBlocks {
		writeIndentedBlock(builder, body, indent)
	}
	return nil
}

func flattenPostPipelineSteps(items []model.PipelineTemplateStep) []model.PipelineTemplateStep {
	childrenByParent := pipelineChildrenByParent(items)
	result := make([]model.PipelineTemplateStep, 0, len(items))
	visited := map[string]struct{}{}
	var walk func(parentID string)
	walk = func(parentID string) {
		for _, child := range childrenByParent[parentID] {
			if _, ok := visited[child.ID]; ok {
				continue
			}
			visited[child.ID] = struct{}{}
			result = append(result, child)
			walk(child.ID)
		}
	}
	walk(workflowPostNodeID)
	for _, item := range items {
		if !item.Enabled {
			continue
		}
		if _, ok := visited[item.ID]; ok {
			continue
		}
		result = append(result, item)
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].SortOrder < result[j].SortOrder
	})
	return result
}

func postStepContent(content, nodeName string) (string, string, bool) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "always", fmt.Sprintf("echo '未配置 post 步骤: %s'", quoteJenkinsString(nodeName)), false
	}
	if body, ok := extractGroovyNamedBlock(content, "post"); ok {
		return "", body, true
	}
	if match := postConditionPattern.FindStringSubmatch(content); len(match) > 1 {
		if body, ok := extractGroovyNamedBlock(content, match[1]); ok {
			return match[1], body, false
		}
	}
	if body, ok := extractGroovyNamedBlock(content, "steps"); ok {
		return "always", body, false
	}
	return "always", fmt.Sprintf("script {\n%s\n}", indentPlainText(content, 2)), false
}

func stageContentWithName(content, stageName string) (string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return fmt.Sprintf("stage('%s') {\n  steps {\n    echo '未配置步骤内容'\n  }\n}", quoteJenkinsString(stageName)), nil
	}
	body, err := stepsBodyContent(content)
	if err != nil {
		logx.Errorf("解析阶段步骤内容失败: stage=%s err=%v", stageName, err)
		return "", errorx.Msg("阶段步骤内容不合法：" + stageName)
	}
	return fmt.Sprintf("stage('%s') {\n  steps {\n%s\n  }\n}", quoteJenkinsString(stageName), indentPlainText(body, 4)), nil
}

func stepsBodyContent(content string) (string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", nil
	}
	loc := stageDeclarePattern.FindStringIndex(content)
	if loc != nil {
		openIndex := strings.Index(content[loc[1]:], "{")
		if openIndex < 0 {
			logx.Errorf("stage 代码块不完整")
			return "", errors.New("stage 代码块不完整")
		}
		openIndex += loc[1]
		closeIndex := findMatchingBrace(content, openIndex)
		if closeIndex < 0 {
			logx.Errorf("stage 代码块不完整")
			return "", errors.New("stage 代码块不完整")
		}
		stageBody := strings.TrimSpace(content[openIndex+1 : closeIndex])
		if body, ok := extractGroovyNamedBlock(stageBody, "steps"); ok {
			return body, nil
		}
		logx.Errorf("步骤模板只能保存 steps 级别内容，完整 stage 必须包含 steps 块")
		return "", errors.New("步骤模板只能保存 steps 级别内容，完整 stage 必须包含 steps 块")
	}
	if body, ok := extractWholeGroovyNamedBlock(content, "steps"); ok {
		return body, nil
	}
	if strings.Contains(content, "stage(") || strings.Contains(content, "parallel {") {
		logx.Errorf("步骤模板只能保存 steps 级别内容，不能包含 stage 或 parallel")
		return "", errors.New("步骤模板只能保存 steps 级别内容，不能包含 stage 或 parallel")
	}
	return content, nil
}

func extractGroovyNamedBlock(content, name string) (string, bool) {
	pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\s*\{`)
	loc := pattern.FindStringIndex(content)
	if loc == nil {
		return "", false
	}
	openIndex := strings.Index(content[loc[0]:loc[1]], "{")
	if openIndex < 0 {
		return "", false
	}
	openIndex += loc[0]
	closeIndex := findMatchingBrace(content, openIndex)
	if closeIndex < 0 {
		return "", false
	}
	return strings.TrimSpace(content[openIndex+1 : closeIndex]), true
}

func extractWholeGroovyNamedBlock(content, name string) (string, bool) {
	pattern := regexp.MustCompile(`^\s*` + regexp.QuoteMeta(name) + `\s*\{`)
	loc := pattern.FindStringIndex(content)
	if loc == nil {
		return "", false
	}
	openIndex := strings.Index(content[loc[0]:loc[1]], "{")
	if openIndex < 0 {
		return "", false
	}
	openIndex += loc[0]
	closeIndex := findMatchingBrace(content, openIndex)
	if closeIndex < 0 || strings.TrimSpace(content[closeIndex+1:]) != "" {
		return "", false
	}
	return strings.TrimSpace(content[openIndex+1 : closeIndex]), true
}

func findMatchingBrace(content string, openIndex int) int {
	depth := 0
	var quote rune
	escaped := false
	for index, char := range content {
		if index < openIndex {
			continue
		}
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if char == '\\' {
				escaped = true
				continue
			}
			if char == quote {
				quote = 0
			}
			continue
		}
		if char == '\'' || char == '"' {
			quote = char
			continue
		}
		if char == '{' {
			depth++
			continue
		}
		if char == '}' {
			depth--
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}

func indentPlainText(content string, indent int) string {
	prefix := strings.Repeat(" ", indent)
	lines := strings.Split(strings.TrimSpace(content), "\n")
	for idx := range lines {
		lines[idx] = prefix + lines[idx]
	}
	return strings.Join(lines, "\n")
}

func writeIndentedBlock(builder *strings.Builder, content string, indent int) {
	for _, line := range strings.Split(strings.TrimSpace(content), "\n") {
		writeIndent(builder, indent)
		builder.WriteString(line)
		builder.WriteString("\n")
	}
}

func writeIndent(builder *strings.Builder, indent int) {
	builder.WriteString(strings.Repeat(" ", indent))
}

func quoteJenkinsString(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `'`, `\'`)
	return value
}

func pipelineVariables(ctx context.Context, svcCtx *svc.ServiceContext, template *model.DevopsPipelineTemplate) ([]*pb.PipelineTemplateVariable, error) {
	if normalizeTemplateEngineType(template.EngineType) == tektonEngineType {
		return tektonPipelineTemplateVariables(template)
	}
	steps := append([]model.PipelineTemplateStep(nil), template.Steps...)
	normalizePipelineStepOrder(steps)
	stepRefs := make([]templateStepParamRef, 0, len(steps))
	for _, item := range steps {
		step, err := svcCtx.StepTemplateModel.FindOne(ctx, item.StepID)
		if err != nil {
			logx.Errorf("处理流水线配置辅助逻辑失败: %v", err)
			return nil, err
		}
		stepRefs = append(stepRefs, templateStepParamRef{Node: item, Template: step})
	}
	codeMaps := templateStepParamCodeMaps(stepRefs)
	result := make([]*pb.PipelineTemplateVariable, 0)
	for _, ref := range stepRefs {
		item := ref.Node
		step := ref.Template
		codeMap := codeMaps[item.ID]
		for _, param := range step.Params {
			currentValue := ""
			runtimeConfig := param.RuntimeConfig
			readonly := param.Readonly
			if param.ParamType == channelvars.ParamChannelCredential {
				currentValue = ""
				runtimeConfig = false
				readonly = true
			}
			result = append(result, &pb.PipelineTemplateVariable{
				StepId:        step.ID.Hex(),
				StepNodeId:    item.ID,
				StepCode:      step.Code,
				StepName:      pipelineNodeName(item),
				Name:          param.Name,
				Code:          runtimeParamCode(param.Code, codeMap),
				SourceCode:    param.Code,
				ParamType:     param.ParamType,
				Mode:          param.Mode,
				DefaultValue:  param.DefaultValue,
				CurrentValue:  currentValue,
				Required:      param.Required,
				Readonly:      readonly,
				Description:   param.Description,
				SortOrder:     param.SortOrder,
				RuntimeMode:   normalizeRuntimeMode(param.RuntimeMode, param.Mode),
				RuntimeConfig: runtimeConfig,
				Config:        stepParamConfigToPb(remapStepParamConfig(normalizeStepParamConfig(param), codeMap)),
			})
		}
	}
	return result, nil
}

func tektonPipelineTemplateVariables(template *model.DevopsPipelineTemplate) ([]*pb.PipelineTemplateVariable, error) {
	var cfg tektonPipelineTemplateDagConfig
	if err := json.Unmarshal([]byte(strings.TrimSpace(template.TektonDagConfig)), &cfg); err != nil {
		logx.Errorf("Tekton Pipeline 模板配置必须是 JSON 对象: %v", err)
		return nil, errorx.Msg("Tekton Pipeline 模板配置必须是 JSON 对象")
	}
	items := tektonTemplatePipelineParams(cfg)
	result := make([]*pb.PipelineTemplateVariable, 0, len(items))
	for index, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		defaultValue := ""
		if item.Default != nil {
			defaultValue = templateParamCurrentValue(item.Default)
		} else if item.DefaultValue != nil {
			defaultValue = templateParamCurrentValue(item.DefaultValue)
		}
		paramType := strings.TrimSpace(item.ParamType)
		if paramType == "" {
			paramType = tektonParamType(item.Type)
		}
		options := item.SelectList
		if len(options) == 0 {
			options = item.Options
		}
		config := item.Config
		if len(config.Options) == 0 {
			config.Options = options
		}
		sortOrder := int64(index + 1)
		result = append(result, &pb.PipelineTemplateVariable{
			StepId:        template.ID.Hex(),
			StepNodeId:    "",
			StepCode:      template.Code,
			StepName:      template.Name,
			Name:          firstNotBlank(item.DisplayName, name),
			Code:          name,
			SourceCode:    firstNotBlank(item.SourceCode, name),
			ParamType:     paramType,
			Mode:          "params",
			DefaultValue:  defaultValue,
			CurrentValue:  "",
			Required:      item.Required,
			Readonly:      item.Readonly,
			Description:   item.Description,
			SortOrder:     sortOrder,
			RuntimeMode:   "params",
			RuntimeConfig: item.RuntimeConfig,
			Config:        stepParamConfigToPb(config),
		})
	}
	return result, nil
}

type templateStepParamRef struct {
	Node     model.PipelineTemplateStep
	Template *model.DevopsStepTemplate
}

func templateStepParamCodeMaps(refs []templateStepParamRef) map[string]map[string]string {
	reserved := make(map[string]struct{})
	for _, ref := range refs {
		for _, param := range ref.Template.Params {
			if strings.TrimSpace(param.Code) != "" {
				reserved[param.Code] = struct{}{}
			}
		}
	}
	used := make(map[string]struct{})
	occurrence := make(map[string]int)
	result := make(map[string]map[string]string, len(refs))
	for _, ref := range refs {
		nodeID := strings.TrimSpace(ref.Node.ID)
		if nodeID == "" {
			continue
		}
		result[nodeID] = make(map[string]string)
		for _, param := range ref.Template.Params {
			sourceCode := strings.TrimSpace(param.Code)
			if sourceCode == "" {
				continue
			}
			result[nodeID][sourceCode] = nextRuntimeParamCode(sourceCode, occurrence, used, reserved)
		}
	}
	return result
}

func nextRuntimeParamCode(sourceCode string, occurrence map[string]int, used, reserved map[string]struct{}) string {
	occurrence[sourceCode]++
	if occurrence[sourceCode] == 1 {
		if _, ok := used[sourceCode]; !ok {
			used[sourceCode] = struct{}{}
			return sourceCode
		}
	}
	for index := occurrence[sourceCode]; ; index++ {
		if index < 2 {
			index = 2
		}
		code := fmt.Sprintf("%s_%d", sourceCode, index)
		if _, ok := used[code]; ok {
			continue
		}
		if _, ok := reserved[code]; ok && code != sourceCode {
			continue
		}
		used[code] = struct{}{}
		return code
	}
}

func runtimeParamCode(sourceCode string, codeMap map[string]string) string {
	if code := strings.TrimSpace(codeMap[sourceCode]); code != "" {
		return code
	}
	return sourceCode
}

func remapStepParamConfig(config model.DevopsStepParamConfig, codeMap map[string]string) model.DevopsStepParamConfig {
	config.ChannelParamCode = runtimeParamCode(config.ChannelParamCode, codeMap)
	config.ProjectParamCode = runtimeParamCode(config.ProjectParamCode, codeMap)
	config.ComponentParamCode = runtimeParamCode(config.ComponentParamCode, codeMap)
	config.CredentialSourceParamCode = runtimeParamCode(config.CredentialSourceParamCode, codeMap)
	for i := range config.DependencyParamCodes {
		config.DependencyParamCodes[i].ParamCode = runtimeParamCode(config.DependencyParamCodes[i].ParamCode, codeMap)
	}
	return config
}

func templateParamCurrentValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprint(v)
		}
		return string(data)
	}
}
