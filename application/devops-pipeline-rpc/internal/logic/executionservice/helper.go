package executionservicelogic

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/client/pipelineconfigservice"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/client/projectservice"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/pb"
	"github.com/yanshicheng/kube-nova/application/devops-quality-rpc/client/qualityservice"
	"github.com/yanshicheng/kube-nova/common/devops/authutil"
	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
	"github.com/yanshicheng/kube-nova/common/devops/jenkins"
	"github.com/yanshicheng/kube-nova/common/devops/qualityreport"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

const (
	engineJenkins        = "jenkins"
	engineTekton         = "tekton"
	defaultPipelineAgent = "any"
	workflowStartNodeID  = "workflow-start"
	workflowPostNodeID   = "workflow-post"
)

var pipelineCodeRegexp = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
var tektonParamNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`)
var runStageNamePattern = regexp.MustCompile(`stage\s*\(\s*(['"])([^'"]*)(['"])\s*\)`)
var mavenCommandPattern = regexp.MustCompile(`(^|[^A-Za-z0-9_./-])(\./mvnw|mvn)(\s+)`)
var mavenSettingsArgPattern = regexp.MustCompile(`(^|\s)(-s(\s|=)|--settings(\s|=|$))`)
var kubernetesSecretNamePattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$`)
var tektonDurationPattern = regexp.MustCompile(`^([0-9]+(ms|h|m|s))+$`)
var tektonTaskKindPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9.]*$`)
var tektonAPIVersionPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+(/[A-Za-z0-9_.-]+)?$`)

const objectListParamType = "objectList"

const pipelineRuntimeCacheTTL = 30 * time.Second
const runJenkinsStatusSyncInterval = 5 * time.Second

type pipelineRuntimeCacheItem struct {
	value     *pipelineconfigservice.ResolvePipelineRuntimeResp
	expiresAt time.Time
}

var pipelineRuntimeCache sync.Map
var runJenkinsStatusSyncCache sync.Map

type pipelineRenderResult struct {
	Script     string
	StageNames map[string]string
	Params     []jenkins.JenkinsRenderParam
}

type pipelineRenderContext struct {
	UserID uint64
	Roles  []string
	Params map[string]string
}

var shanghaiLocation = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	return loc
}()

func validateCode(code, label string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		logx.Errorf("%s", label+"不能为空")
		return errorx.Msg(label + "不能为空")
	}
	if !pipelineCodeRegexp.MatchString(code) {
		logx.Errorf("%s", label+"只能包含字母、数字、下划线和中划线")
		return errorx.Msg(label + "只能包含字母、数字、下划线和中划线")
	}
	return nil
}

func isSuperAdmin(roles []string) bool {
	for _, role := range roles {
		if strings.EqualFold(strings.TrimSpace(role), "SUPER_ADMIN") {
			return true
		}
	}
	return false
}

func accessibleProjectIDs(ctx context.Context, svcCtx *svc.ServiceContext, userID uint64, roles []string) ([]string, bool, error) {
	if isSuperAdmin(roles) {
		return nil, false, nil
	}
	resp, err := svcCtx.ProjectRpc.ProjectList(ctx, &projectservice.ListProjectReq{
		Page:          1,
		PageSize:      200,
		CurrentUserId: userID,
		CurrentRoles:  roles,
	})
	if err != nil {
		logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
		return nil, false, err
	}
	ids := make([]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		ids = append(ids, item.Id)
	}
	return ids, true, nil
}

func ensureProjectAccess(ctx context.Context, svcCtx *svc.ServiceContext, projectID string, userID uint64, roles []string) error {
	if strings.TrimSpace(projectID) == "" {
		logx.Errorf("请选择项目")
		return errorx.Msg("请选择项目")
	}
	_, err := svcCtx.ProjectRpc.ProjectGet(ctx, &projectservice.GetByIdReq{
		Id:            projectID,
		CurrentUserId: userID,
		CurrentRoles:  roles,
	})
	if err != nil {
		logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
	}
	return err
}

func pipelineToPb(in *model.DevopsPipeline) *pb.DevopsPipeline {
	if in == nil {
		return nil
	}
	return &pb.DevopsPipeline{
		Id:                         in.ID.Hex(),
		ProjectId:                  in.ProjectID,
		ProjectName:                in.ProjectName,
		ProjectCode:                in.ProjectCode,
		SystemId:                   in.SystemID,
		SystemName:                 in.SystemName,
		SystemCode:                 in.SystemCode,
		EnvironmentId:              in.EnvironmentID,
		EnvironmentName:            in.EnvironmentName,
		EnvironmentCode:            in.EnvironmentCode,
		Name:                       in.Name,
		Code:                       in.Code,
		EngineType:                 in.EngineType,
		TemplateId:                 in.TemplateID,
		BuildChannelBindingId:      in.BuildChannelBindingID,
		BuildChannelName:           in.BuildChannelName,
		Steps:                      stepsToPb(in.Steps),
		Params:                     paramsToPb(in.Params),
		Agent:                      agentToPb(in.Agent),
		Tools:                      toolsToPb(in.Tools),
		Options:                    optionsToPb(in.Options),
		Triggers:                   triggersToPb(in.Triggers),
		JobName:                    in.JobName,
		JobFullName:                in.JobFullName,
		TriggerMode:                normalizeTriggerMode(in.TriggerMode, in.Triggers),
		ScanEnabled:                in.ScanEnabled,
		ScanMode:                   in.ScanMode,
		RejectUnexpectedReports:    in.RejectUnexpectedReports,
		Enforce:                    in.Enforce,
		ScanItems:                  scanItemsToPb(in.ScanItems),
		TektonDagConfig:            in.TektonDagConfig,
		TektonRunPolicy:            in.TektonRunPolicy,
		TektonTriggerConfig:        in.TektonTriggerConfig,
		TektonPrunerPolicyRef:      in.TektonPrunerPolicyRef,
		TektonPipelineYamlSnapshot: in.TektonPipelineYamlSnapshot,
		TektonTemplateId:           in.TektonTemplateID,
		TektonTemplateSnapshot:     in.TektonTemplateSnapshot,
		TektonParamBindings:        in.TektonParamBindings,
		TektonWorkspaceBindings:    in.TektonWorkspaceBindings,
		TektonResourceBindings:     in.TektonResourceBindings,
		SyncStatus:                 in.SyncStatus,
		SyncMessage:                in.SyncMessage,
		LastRunStatus:              in.LastRunStatus,
		Status:                     in.Status,
		CreatedBy:                  in.CreatedBy,
		UpdatedBy:                  in.UpdatedBy,
		CreatedAt:                  in.CreateAt.Unix(),
		UpdatedAt:                  in.UpdateAt.Unix(),
	}
}

func scanItemsFromPb(items []*pb.ScanPlanItem) []model.ScanPlanItem {
	result := make([]model.ScanPlanItem, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, model.ScanPlanItem{
			StepID:          strings.TrimSpace(item.StepId),
			StageID:         strings.TrimSpace(item.StageId),
			Tool:            strings.TrimSpace(item.Tool),
			ToolMode:        strings.TrimSpace(item.ToolMode),
			ToolBindingID:   strings.TrimSpace(item.ToolBindingId),
			TargetType:      strings.TrimSpace(item.TargetType),
			TargetName:      strings.TrimSpace(item.TargetName),
			TargetParamCode: strings.TrimSpace(item.TargetParamCode),
			ReportSource:    strings.TrimSpace(item.ReportSource),
			ReportPath:      strings.TrimSpace(item.ReportPath),
			ReportFormat:    strings.TrimSpace(item.ReportFormat),
			Required:        item.Required,
			Enforce:         item.Enforce,
			Parser:          strings.TrimSpace(item.Parser),
		})
	}
	return result
}

func validatePipelineScanItems(scanEnabled bool, items []model.ScanPlanItem) error {
	if !scanEnabled {
		return nil
	}
	if len(items) == 0 {
		return errorx.Msg("扫描计划不能为空")
	}
	for _, item := range items {
		tool := strings.TrimSpace(item.Tool)
		if tool == "" {
			return errorx.Msg("扫描工具不能为空")
		}
		if strings.TrimSpace(item.ReportFormat) == "" {
			return errorx.Msg("扫描报告格式不能为空")
		}
		mode := strings.ToLower(strings.TrimSpace(item.ToolMode))
		switch mode {
		case "service", "service_api":
			if strings.TrimSpace(item.ToolBindingID) == "" {
				return errorx.Msg("服务型扫描工具必须绑定项目渠道")
			}
			if mode == "service_api" && strings.TrimSpace(item.TargetName) == "" && strings.TrimSpace(item.TargetParamCode) == "" {
				return errorx.Msg("服务 API 型扫描工具必须配置目标或目标参数")
			}
		case "local", "local_report":
			if strings.TrimSpace(item.ToolBindingID) != "" {
				return errorx.Msg("本地扫描工具不能绑定项目渠道")
			}
		case "hybrid":
			// 混合模式只做基础字段校验，由具体接入通道决定是否需要绑定。
		default:
			return errorx.Msg("扫描工具模式不支持")
		}
	}
	return nil
}

func scanItemsToPb(items []model.ScanPlanItem) []*pb.ScanPlanItem {
	result := make([]*pb.ScanPlanItem, 0, len(items))
	for _, item := range items {
		result = append(result, &pb.ScanPlanItem{
			StepId:          item.StepID,
			StageId:         item.StageID,
			Tool:            item.Tool,
			ToolMode:        item.ToolMode,
			ToolBindingId:   item.ToolBindingID,
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

func fillPipelineLastRun(out *pb.DevopsPipeline, run *model.DevopsPipelineRun) {
	if out == nil || run == nil {
		return
	}
	out.LastRunId = run.ID.Hex()
	out.LastRunBuildNumber = run.JenkinsBuildNumber
	out.LastRunBuildUrl = run.JenkinsBuildURL
	out.LastRunStartedAt = formatTime(run.StartedAt)
	if strings.TrimSpace(run.Status) != "" {
		out.LastRunStatus = run.Status
	}
}

func stepsFromPb(items []*pb.PipelineStep) []model.PipelineStep {
	result := make([]model.PipelineStep, 0, len(items))
	for index, item := range items {
		if item == nil {
			continue
		}
		sortOrder := item.SortOrder
		if sortOrder <= 0 {
			sortOrder = int64(index + 1)
		}
		result = append(result, model.PipelineStep{
			ID:             strings.TrimSpace(item.Id),
			StepID:         strings.TrimSpace(item.StepId),
			StepCode:       strings.TrimSpace(item.StepCode),
			StepName:       strings.TrimSpace(item.StepName),
			NodeName:       strings.TrimSpace(item.NodeName),
			ContainerName:  strings.TrimSpace(item.ContainerName),
			StepType:       strings.TrimSpace(item.StepType),
			Icon:           strings.TrimSpace(item.Icon),
			IconColor:      strings.TrimSpace(item.IconColor),
			ParentNodeID:   normalizeParentNodeID(item.ParentNodeId),
			BranchType:     normalizeBranchType(item.BranchType),
			SortOrder:      sortOrder,
			Enabled:        item.Enabled,
			X:              item.X,
			Y:              item.Y,
			ParamValues:    item.ParamValues,
			StageContent:   item.StageContent,
			ArtifactConfig: artifactConfigFromPb(item.ArtifactConfig),
		})
	}
	return result
}

func stepsToPb(items []model.PipelineStep) []*pb.PipelineStep {
	result := make([]*pb.PipelineStep, 0, len(items))
	for _, item := range items {
		result = append(result, &pb.PipelineStep{
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
			StageContent:   item.StageContent,
			ArtifactConfig: artifactConfigToPb(item.ArtifactConfig),
		})
	}
	return result
}

func paramsFromPb(items []*pb.PipelineParam) []model.PipelineParam {
	result := make([]model.PipelineParam, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		readonly := item.Readonly
		runtimeConfig := item.RuntimeConfig
		if strings.TrimSpace(item.ParamType) == channelvars.ParamChannelCredential {
			readonly = true
			runtimeConfig = false
		}
		result = append(result, model.PipelineParam{
			Name:          strings.TrimSpace(item.Name),
			Code:          strings.TrimSpace(item.Code),
			SourceCode:    strings.TrimSpace(item.SourceCode),
			StepNodeID:    strings.TrimSpace(item.StepNodeId),
			DefaultValue:  item.DefaultValue,
			CurrentValue:  item.CurrentValue,
			ParamType:     strings.TrimSpace(item.ParamType),
			Mode:          normalizeParamMode(item.Mode),
			Required:      item.Required,
			Readonly:      readonly,
			Description:   item.Description,
			SelectList:    stepParamOptionsFromPb(item.SelectList),
			AllowCustom:   item.AllowCustom,
			SortOrder:     item.SortOrder,
			RuntimeMode:   normalizeRuntimeMode(item.RuntimeMode, item.Mode),
			RuntimeConfig: runtimeConfig,
			Config:        stepParamConfigFromPb(item.Config),
		})
	}
	normalizePipelineParams(result)
	return result
}

func compoundFieldsFromPb(items []*pb.CompoundParamField) []model.CompoundParamField {
	result := make([]model.CompoundParamField, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		config := stepParamConfigFromPb(item.Config)
		result = append(result, model.CompoundParamField{
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

func normalizePipelineParams(items []model.PipelineParam) {
	sort.SliceStable(items, func(i, j int) bool {
		left := items[i].SortOrder
		right := items[j].SortOrder
		if left <= 0 {
			left = int64(i + 1)
		}
		if right <= 0 {
			right = int64(j + 1)
		}
		if left == right {
			return items[i].Code < items[j].Code
		}
		return left < right
	})
	for idx := range items {
		items[idx].SortOrder = int64(idx + 1)
		items[idx].Config = normalizePipelineParamConfig(items[idx])
		normalizeCompoundFields(items[idx].Config.CompoundFields)
	}
}

func normalizeCompoundFields(items []model.CompoundParamField) {
	for idx := range items {
		items[idx].Config = normalizeCompoundFieldConfig(items[idx])
	}
}

func compoundFieldsToPb(items []model.CompoundParamField) []*pb.CompoundParamField {
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

func paramsToPb(items []model.PipelineParam) []*pb.PipelineParam {
	result := make([]*pb.PipelineParam, 0, len(items))
	for _, item := range items {
		result = append(result, &pb.PipelineParam{
			Name:          item.Name,
			Code:          item.Code,
			SourceCode:    item.SourceCode,
			StepNodeId:    item.StepNodeID,
			DefaultValue:  item.DefaultValue,
			CurrentValue:  item.CurrentValue,
			ParamType:     item.ParamType,
			Mode:          item.Mode,
			Required:      item.Required,
			Readonly:      item.Readonly,
			Description:   item.Description,
			SelectList:    stepParamOptionsToPb(item.SelectList),
			AllowCustom:   item.AllowCustom,
			SortOrder:     item.SortOrder,
			RuntimeMode:   normalizeRuntimeMode(item.RuntimeMode, item.Mode),
			RuntimeConfig: item.RuntimeConfig,
			Config:        stepParamConfigToPb(normalizePipelineParamConfig(item)),
		})
	}
	return result
}

func stepParamOptionsFromPb(items []*pb.StepParamOption) []model.StepParamOption {
	result := make([]model.StepParamOption, 0, len(items))
	for _, item := range items {
		if item == nil || strings.TrimSpace(item.Value) == "" {
			continue
		}
		result = append(result, model.StepParamOption{Label: strings.TrimSpace(item.Label), Value: strings.TrimSpace(item.Value)})
	}
	return result
}

func stepParamOptionsToPb(items []model.StepParamOption) []*pb.StepParamOption {
	result := make([]*pb.StepParamOption, 0, len(items))
	for _, item := range items {
		result = append(result, &pb.StepParamOption{Label: item.Label, Value: item.Value})
	}
	return result
}

func stepParamConfigFromPb(in *pb.StepParamConfig) model.StepParamConfig {
	if in == nil {
		return model.StepParamConfig{}
	}
	return model.StepParamConfig{
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
		DependencyParamCodes:      stepParamDependenciesFromPb(in.DependencyParamCodes),
		CompoundFields:            compoundFieldsFromPb(in.CompoundFields),
		FullRow:                   in.FullRow,
		RenderMode:                strings.TrimSpace(in.RenderMode),
		ConfigTypeID:              strings.TrimSpace(in.ConfigTypeId),
		ConfigTypeCode:            strings.TrimSpace(in.ConfigTypeCode),
		ValueMode:                 strings.TrimSpace(in.ValueMode),
	}
}

func stepParamConfigToPb(in model.StepParamConfig) *pb.StepParamConfig {
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
		DependencyParamCodes:      stepParamDependenciesToPb(in.DependencyParamCodes),
		CompoundFields:            compoundFieldsToPb(in.CompoundFields),
		FullRow:                   in.FullRow,
		RenderMode:                in.RenderMode,
		ConfigTypeId:              in.ConfigTypeID,
		ConfigTypeCode:            in.ConfigTypeCode,
		ValueMode:                 strings.TrimSpace(in.ValueMode),
	}
}

func stepParamDependenciesFromPb(items []*pb.StepParamDependency) []model.StepParamDependency {
	result := make([]model.StepParamDependency, 0, len(items))
	for _, item := range items {
		if item == nil || strings.TrimSpace(item.Field) == "" || strings.TrimSpace(item.ParamCode) == "" {
			continue
		}
		result = append(result, model.StepParamDependency{
			Field:     strings.TrimSpace(item.Field),
			ParamCode: strings.TrimSpace(item.ParamCode),
		})
	}
	return result
}

func stepParamDependenciesToPb(items []model.StepParamDependency) []*pb.StepParamDependency {
	result := make([]*pb.StepParamDependency, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Field) == "" || strings.TrimSpace(item.ParamCode) == "" {
			continue
		}
		result = append(result, &pb.StepParamDependency{
			Field:     strings.TrimSpace(item.Field),
			ParamCode: strings.TrimSpace(item.ParamCode),
		})
	}
	return result
}

func artifactConfigFromPb(in *pb.StepArtifactConfig) model.StepArtifactConfig {
	if in == nil {
		return model.StepArtifactConfig{}
	}
	return model.StepArtifactConfig{
		Enabled:     in.Enabled,
		Type:        strings.TrimSpace(in.Type),
		Name:        strings.TrimSpace(in.Name),
		Path:        strings.TrimSpace(in.Path),
		Required:    in.Required,
		ContentType: strings.TrimSpace(in.ContentType),
	}
}

func artifactConfigToPb(in model.StepArtifactConfig) *pb.StepArtifactConfig {
	return &pb.StepArtifactConfig{
		Enabled:     in.Enabled,
		Type:        in.Type,
		Name:        in.Name,
		Path:        in.Path,
		Required:    in.Required,
		ContentType: in.ContentType,
	}
}

func validatePipelineArtifactConfig(config model.StepArtifactConfig) error {
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

func managerParamConfigToModel(in *pipelineconfigservice.StepParamConfig) model.StepParamConfig {
	if in == nil {
		return model.StepParamConfig{}
	}
	options := make([]model.StepParamOption, 0, len(in.Options))
	for _, item := range in.Options {
		if item == nil || strings.TrimSpace(item.Value) == "" {
			continue
		}
		options = append(options, model.StepParamOption{Label: strings.TrimSpace(item.Label), Value: strings.TrimSpace(item.Value)})
	}
	return model.StepParamConfig{
		Options:                   options,
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
		DependencyParamCodes:      managerStepParamDependenciesToModel(in.DependencyParamCodes),
		CompoundFields:            managerCompoundFieldsToModel(in.CompoundFields),
		FullRow:                   in.FullRow,
		RenderMode:                strings.TrimSpace(in.RenderMode),
		ConfigTypeID:              strings.TrimSpace(in.ConfigTypeId),
		ConfigTypeCode:            strings.TrimSpace(in.ConfigTypeCode),
		ValueMode:                 strings.TrimSpace(in.ValueMode),
	}
}

func managerStepParamDependenciesToModel(items []*pipelineconfigservice.StepParamDependency) []model.StepParamDependency {
	result := make([]model.StepParamDependency, 0, len(items))
	for _, item := range items {
		if item == nil || strings.TrimSpace(item.Field) == "" || strings.TrimSpace(item.ParamCode) == "" {
			continue
		}
		result = append(result, model.StepParamDependency{
			Field:     strings.TrimSpace(item.Field),
			ParamCode: strings.TrimSpace(item.ParamCode),
		})
	}
	return result
}

func managerCompoundFieldsToModel(in []*pipelineconfigservice.CompoundParamField) []model.CompoundParamField {
	result := make([]model.CompoundParamField, 0, len(in))
	for _, item := range in {
		if item == nil {
			continue
		}
		config := managerParamConfigToModel(item.Config)
		result = append(result, model.CompoundParamField{
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
	return result
}

func managerArtifactConfigToModel(in *pipelineconfigservice.StepArtifactConfig) model.StepArtifactConfig {
	if in == nil {
		return model.StepArtifactConfig{}
	}
	return model.StepArtifactConfig{
		Enabled:     in.Enabled,
		Type:        strings.TrimSpace(in.Type),
		Name:        strings.TrimSpace(in.Name),
		Path:        strings.TrimSpace(in.Path),
		Required:    in.Required,
		ContentType: strings.TrimSpace(in.ContentType),
	}
}

func agentFromPb(in *pb.JenkinsAgent) model.JenkinsAgent {
	if in == nil {
		return model.JenkinsAgent{Type: defaultPipelineAgent}
	}
	agent := model.JenkinsAgent{
		Type:        normalizePipelineAgentType(in.Type),
		Label:       strings.TrimSpace(in.Label),
		DockerImage: strings.TrimSpace(in.DockerImage),
		DockerArgs:  in.DockerArgs,
		Raw:         in.Raw,
		ID:          strings.TrimSpace(in.Id),
		Name:        strings.TrimSpace(in.Name),
		AgentType:   normalizePipelineAgentType(in.AgentType),
		MatchMode:   strings.TrimSpace(in.MatchMode),
		MatchValue:  strings.TrimSpace(in.MatchValue),
		Cloud:       strings.TrimSpace(in.Cloud),
		PodYaml:     strings.TrimSpace(in.PodYaml),
		Containers:  agentContainersFromPb(in.Containers),
	}
	if agent.Type == "" {
		agent.Type = defaultPipelineAgent
	}
	switch agent.Type {
	case "static", "dynamic":
		agent.Label = ""
		agent.DockerImage = ""
		agent.DockerArgs = ""
		agent.Raw = ""
	case "label":
		agent.DockerImage = ""
		agent.DockerArgs = ""
		agent.Raw = ""
	case "docker":
		agent.Label = ""
		agent.Raw = ""
	case "raw":
		agent.Label = ""
		agent.DockerImage = ""
		agent.DockerArgs = ""
	default:
		agent.Label = ""
		agent.DockerImage = ""
		agent.DockerArgs = ""
		agent.Raw = ""
	}
	return agent
}

func normalizePipelineAgentType(agentType string) string {
	agentType = strings.TrimSpace(agentType)
	if agentType == "pod" {
		return "dynamic"
	}
	return agentType
}

func isDynamicPipelineAgentType(agentType string) bool {
	return normalizePipelineAgentType(agentType) == "dynamic"
}

func agentToPb(in model.JenkinsAgent) *pb.JenkinsAgent {
	return &pb.JenkinsAgent{
		Type:        normalizePipelineAgentType(in.Type),
		Label:       in.Label,
		DockerImage: in.DockerImage,
		DockerArgs:  in.DockerArgs,
		Raw:         in.Raw,
		Id:          in.ID,
		Name:        in.Name,
		AgentType:   normalizePipelineAgentType(in.AgentType),
		MatchMode:   in.MatchMode,
		MatchValue:  in.MatchValue,
		Cloud:       in.Cloud,
		PodYaml:     in.PodYaml,
		Containers:  agentContainersToPb(in.Containers),
	}
}

func agentContainersFromPb(items []*pb.JenkinsAgentContainer) []model.JenkinsAgentContainer {
	result := make([]model.JenkinsAgentContainer, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		if item == nil {
			continue
		}
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, model.JenkinsAgentContainer{Name: name, Image: strings.TrimSpace(item.Image)})
	}
	return result
}

func agentContainersToPb(items []model.JenkinsAgentContainer) []*pb.JenkinsAgentContainer {
	result := make([]*pb.JenkinsAgentContainer, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Name) == "" {
			continue
		}
		result = append(result, &pb.JenkinsAgentContainer{Name: item.Name, Image: item.Image})
	}
	return result
}

func toolsFromPb(items []*pb.JenkinsTool) []model.JenkinsTool {
	result := make([]model.JenkinsTool, 0, len(items))
	for _, item := range items {
		if item != nil {
			result = append(result, model.JenkinsTool{Name: item.Name, Type: item.Type, Version: item.Version})
		}
	}
	return result
}

func toolsToPb(items []model.JenkinsTool) []*pb.JenkinsTool {
	result := make([]*pb.JenkinsTool, 0, len(items))
	for _, item := range items {
		result = append(result, &pb.JenkinsTool{Name: item.Name, Type: item.Type, Version: item.Version})
	}
	return result
}

func optionsFromPb(items []*pb.JenkinsOption) []model.JenkinsOption {
	result := make([]model.JenkinsOption, 0, len(items))
	for _, item := range items {
		if item != nil {
			result = append(result, model.JenkinsOption{Name: item.Name, Value: item.Value})
		}
	}
	return result
}

func optionsToPb(items []model.JenkinsOption) []*pb.JenkinsOption {
	result := make([]*pb.JenkinsOption, 0, len(items))
	for _, item := range items {
		result = append(result, &pb.JenkinsOption{Name: item.Name, Value: item.Value})
	}
	return result
}

func triggersFromPb(items []*pb.JenkinsTrigger) []model.JenkinsTrigger {
	result := make([]model.JenkinsTrigger, 0, len(items))
	for _, item := range items {
		if item != nil {
			result = append(result, model.JenkinsTrigger{Type: item.Type, Spec: item.Spec, Enabled: item.Enabled})
		}
	}
	return result
}

func triggersToPb(items []model.JenkinsTrigger) []*pb.JenkinsTrigger {
	result := make([]*pb.JenkinsTrigger, 0, len(items))
	for _, item := range items {
		result = append(result, &pb.JenkinsTrigger{Type: item.Type, Spec: item.Spec, Enabled: item.Enabled})
	}
	return result
}

func normalizeTriggerMode(mode string, triggers []model.JenkinsTrigger) string {
	mode = strings.TrimSpace(mode)
	if mode == "manual" || mode == "scheduled" {
		return mode
	}
	for _, item := range triggers {
		if item.Enabled && strings.TrimSpace(item.Spec) != "" {
			return "scheduled"
		}
	}
	return "manual"
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

func normalizePipelineParamConfig(item model.PipelineParam) model.StepParamConfig {
	config := item.Config
	if len(config.Options) == 0 {
		config.Options = item.SelectList
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
				config.MappingField = "endpoint"
			}
		}
	}
	config.MappingField = normalizePipelineChannelVariableMappingField(config.ChannelGroupCode, config.ChannelTypeFilter, config.MappingField)
	return config
}

func normalizeCompoundFieldConfig(item model.CompoundParamField) model.StepParamConfig {
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
				config.MappingField = "endpoint"
			}
		}
	}
	config.MappingField = normalizePipelineChannelVariableMappingField(config.ChannelGroupCode, config.ChannelTypeFilter, config.MappingField)
	return config
}

func normalizePipelineChannelVariableMappingField(groupCode, channelType, mappingField string) string {
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

func normalizeParentNodeID(parentID string) string {
	parentID = strings.TrimSpace(parentID)
	if parentID == "" {
		return workflowStartNodeID
	}
	return parentID
}

func normalizeBranchType(branchType string) string {
	branchType = strings.TrimSpace(branchType)
	if branchType == "" {
		return "next"
	}
	return branchType
}

func normalizePipelineStepBranches(items []model.PipelineStep) {
	activeIDs, parallelParentByID := activePipelineParallelBranches(items)
	for idx := range items {
		if items[idx].BranchType == "parallel" {
			continue
		}
		parentID := strings.TrimSpace(items[idx].ParentNodeID)
		if parallelParentID, ok := parallelParentByID[parentID]; ok {
			items[idx].ParentNodeID = parallelParentID
			items[idx].BranchType = "next"
		}
	}

	activeIDs, _ = activePipelineParallelBranches(items)
	for idx := range items {
		if _, ok := activeIDs[items[idx].ID]; ok {
			items[idx].BranchType = "parallel"
			continue
		}
		items[idx].BranchType = "next"
	}
}

func activePipelineParallelBranches(items []model.PipelineStep) (map[string]struct{}, map[string]string) {
	grouped := make(map[string][]model.PipelineStep)
	for _, item := range items {
		if strings.TrimSpace(item.BranchType) != "parallel" {
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

func pipelineRunToPb(in *model.DevopsPipelineRun) *pb.DevopsPipelineRun {
	if in == nil {
		return nil
	}
	return &pb.DevopsPipelineRun{
		Id:                    in.ID.Hex(),
		ProjectId:             in.ProjectID,
		ProjectName:           in.ProjectName,
		ProjectCode:           in.ProjectCode,
		SystemId:              in.SystemID,
		SystemName:            in.SystemName,
		SystemCode:            in.SystemCode,
		EnvironmentId:         in.EnvironmentID,
		EnvironmentName:       in.EnvironmentName,
		EnvironmentCode:       in.EnvironmentCode,
		PipelineId:            in.PipelineID,
		PipelineName:          in.PipelineName,
		PipelineCode:          in.PipelineCode,
		EngineType:            in.EngineType,
		JenkinsJobName:        in.JenkinsJobName,
		JenkinsJobFullName:    in.JenkinsJobFullName,
		JenkinsBuildNumber:    in.JenkinsBuildNumber,
		JenkinsQueueId:        in.JenkinsQueueID,
		JenkinsBuildUrl:       in.JenkinsBuildURL,
		BuildId:               in.BuildID,
		TektonNamespace:       in.TektonNamespace,
		TektonPipelineName:    in.TektonPipelineName,
		TektonPipelineRunName: in.TektonPipelineRunName,
		TriggerType:           in.TriggerType,
		TriggerUserId:         in.TriggerUserID,
		TriggerUsername:       in.TriggerUsername,
		Status:                in.Status,
		StartedAt:             formatTime(in.StartedAt),
		FinishedAt:            formatTime(in.FinishedAt),
		DurationSeconds:       in.DurationSeconds,
		ErrorMessage:          in.ErrorMessage,
		LogStorageType:        in.LogStorageType,
		LogObjectKey:          in.LogObjectKey,
		LogSize:               in.LogSize,
		LogArchivedAt:         formatTime(in.LogArchivedAt),
	}
}

func runStageToPb(in *model.DevopsPipelineRunStage) *pb.PipelineRunStage {
	if in == nil {
		return nil
	}
	return &pb.PipelineRunStage{
		Id:                in.ID.Hex(),
		RunId:             in.RunID,
		PipelineId:        in.PipelineID,
		NodeId:            in.NodeID,
		StageName:         in.StageName,
		StageType:         in.StageType,
		Status:            in.Status,
		StartedAt:         formatTime(in.StartedAt),
		FinishedAt:        formatTime(in.FinishedAt),
		DurationSeconds:   in.DurationSeconds,
		JenkinsNodeId:     in.JenkinsNodeID,
		LogObjectKey:      in.LogObjectKey,
		ErrorMessage:      in.ErrorMessage,
		TektonTaskRunName: in.TektonTaskRunName,
		TektonPodName:     in.TektonPodName,
		ContainerName:     in.ContainerName,
		ContainerNames:    in.ContainerNames,
		TektonTaskRuns:    tektonTaskRunSnapshotsToPb(in.TektonTaskRuns),
	}
}

func tektonTaskRunSnapshotsToPb(items []model.TektonTaskRunSnapshot) []*pb.TektonTaskRunSnapshot {
	result := make([]*pb.TektonTaskRunSnapshot, 0, len(items))
	for _, item := range items {
		result = append(result, &pb.TektonTaskRunSnapshot{
			Name:             item.Name,
			PipelineTaskName: item.PipelineTaskName,
			PodName:          item.PodName,
			ContainerName:    item.ContainerName,
			ContainerNames:   item.ContainerNames,
			Status:           item.Status,
			StartedAt:        formatTime(item.StartedAt),
			FinishedAt:       formatTime(item.FinishedAt),
			DurationSeconds:  item.DurationSeconds,
			ErrorMessage:     item.ErrorMessage,
		})
	}
	return result
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.In(shanghaiLocation).Format("2006-01-02 15:04:05")
}

func preparePipelineSnapshot(ctx context.Context, svcCtx *svc.ServiceContext, engineType string, inSteps []*pb.PipelineStep, inParams []*pb.PipelineParam, templateID, projectID string, userID uint64, roles []string) ([]model.PipelineStep, []model.PipelineParam, error) {
	steps := stepsFromPb(inSteps)
	params := paramsFromPb(inParams)
	var template *pipelineconfigservice.DevopsPipelineTemplate
	if templateID != "" {
		resp, err := svcCtx.PipelineConfigRpc.PipelineTemplateGet(ctx, &pipelineconfigservice.GetByIdReq{Id: templateID, CurrentUserId: userID, CurrentRoles: roles})
		if err != nil {
			logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
			return nil, nil, err
		}
		if resp == nil || resp.Data == nil {
			logx.Errorf("流水线模板不存在")
			return nil, nil, errorx.Msg("流水线模板不存在")
		}
		template = resp.Data
		if strings.EqualFold(strings.TrimSpace(template.Scope), "project") && strings.TrimSpace(template.ProjectId) != strings.TrimSpace(projectID) {
			logx.Errorf("项目流水线不能引用其他项目模板")
			return nil, nil, errorx.Msg("项目流水线不能引用其他项目模板")
		}
	}
	if len(steps) == 0 && template != nil {
		steps = make([]model.PipelineStep, 0, len(template.Steps))
		for _, item := range template.Steps {
			steps = append(steps, model.PipelineStep{
				ID:             item.Id,
				StepID:         item.StepId,
				StepCode:       item.StepCode,
				StepName:       item.StepName,
				NodeName:       item.NodeName,
				StepType:       item.StepType,
				Icon:           item.Icon,
				IconColor:      item.IconColor,
				ParentNodeID:   item.ParentNodeId,
				BranchType:     item.BranchType,
				SortOrder:      item.SortOrder,
				Enabled:        item.Enabled,
				X:              item.X,
				Y:              item.Y,
				ParamValues:    item.ParamValues,
				ArtifactConfig: managerArtifactConfigToModel(item.ArtifactConfig),
			})
		}
		vars, err := svcCtx.PipelineConfigRpc.PipelineTemplateVariables(ctx, &pipelineconfigservice.GetByIdReq{Id: templateID, CurrentUserId: userID, CurrentRoles: roles})
		if err == nil {
			params = make([]model.PipelineParam, 0, len(vars.Data))
			for _, item := range vars.Data {
				currentValue := item.CurrentValue
				runtimeConfig := item.RuntimeConfig
				readonly := item.Readonly
				if item.ParamType == channelvars.ParamChannelCredential {
					currentValue = ""
					runtimeConfig = false
					readonly = true
				}
				params = append(params, model.PipelineParam{
					Name:          item.Name,
					Code:          item.Code,
					SourceCode:    strings.TrimSpace(item.SourceCode),
					StepNodeID:    strings.TrimSpace(item.StepNodeId),
					DefaultValue:  item.DefaultValue,
					CurrentValue:  currentValue,
					ParamType:     item.ParamType,
					Mode:          item.Mode,
					Required:      item.Required,
					Readonly:      readonly,
					Description:   item.Description,
					SortOrder:     item.SortOrder,
					RuntimeMode:   normalizeRuntimeMode(item.RuntimeMode, item.Mode),
					RuntimeConfig: runtimeConfig,
					Config:        managerParamConfigToModel(item.Config),
				})
			}
		}
	}
	if len(steps) == 0 {
		logx.Errorf("请配置流水线步骤")
		return nil, nil, errorx.Msg("请配置流水线步骤")
	}
	stepTemplatesByID, err := loadPipelineStepTemplates(ctx, svcCtx, engineType, steps)
	if err != nil {
		logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
		return nil, nil, err
	}
	stepParamsByNode := make(map[string][]*pipelineconfigservice.DevopsStepParam)
	for index := range steps {
		if steps[index].ID == "" {
			steps[index].ID = fmt.Sprintf("node-%d", index+1)
		}
		if steps[index].ParentNodeID == "" {
			steps[index].ParentNodeID = workflowStartNodeID
		}
		if steps[index].BranchType == "" {
			steps[index].BranchType = "next"
		}
		if steps[index].StepID != "" {
			step := stepTemplatesByID[strings.TrimSpace(steps[index].StepID)]
			if step == nil || step.Data == nil {
				logx.Errorf("流水线步骤不存在")
				return nil, nil, errorx.Msg("流水线步骤不存在")
			}
			stepParamsByNode[steps[index].ID] = step.Data.Params
			steps[index].StageContent = step.Data.StageContent
			steps[index].StepName = step.Data.Name
			steps[index].StepCode = step.Data.Code
			steps[index].StepType = step.Data.Type
			if !steps[index].ArtifactConfig.Enabled && step.Data.ArtifactConfig != nil && step.Data.ArtifactConfig.Enabled {
				steps[index].ArtifactConfig = managerArtifactConfigToModel(step.Data.ArtifactConfig)
			}
			if steps[index].NodeName == "" {
				steps[index].NodeName = step.Data.Name
			}
			if err := validateStepRequiredParams(steps[index], step.Data.Params); err != nil {
				logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
				return nil, nil, err
			}
		}
		if err := validatePipelineArtifactConfig(steps[index].ArtifactConfig); err != nil {
			logx.Errorf("校验流水线阶段产物配置失败: step=%s err=%v", displayStepName(steps[index]), err)
			return nil, nil, errorx.Msg("阶段产物配置错误：" + displayStepName(steps[index]))
		}
	}
	normalizePipelineStepBranches(steps)
	if err := validatePipelinePostConditionUniqueness(steps); err != nil {
		logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
		return nil, nil, err
	}
	ensureUniquePipelineStepNodeNames(steps)
	params = rebuildPipelineParamsFromSteps(steps, stepParamsByNode, params)
	if err := validatePipelineParams(engineType, params); err != nil {
		logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
		return nil, nil, err
	}
	if err := validateRequiredParams(params); err != nil {
		logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
		return nil, nil, err
	}
	return steps, params, nil
}

func preparePipelineSnapshotForEngine(ctx context.Context, svcCtx *svc.ServiceContext, engineType string, inSteps []*pb.PipelineStep, inParams []*pb.PipelineParam, templateID, tektonDagConfig, projectID string, userID uint64, roles []string) ([]model.PipelineStep, []model.PipelineParam, error) {
	if engineType == engineTekton && hasTektonDagConfig(tektonDagConfig) {
		steps := stepsFromPb(inSteps)
		params := paramsFromPb(inParams)
		if err := validatePipelineParams(engineType, params); err != nil {
			logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
			return nil, nil, err
		}
		if err := validateTektonDagPipelineParams(params); err != nil {
			logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
			return nil, nil, err
		}
		if err := validateRequiredParams(params); err != nil {
			logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
			return nil, nil, err
		}
		if err := validateTektonDagConfig(tektonDagConfig); err != nil {
			logx.Errorf("校验 Tekton DAG 配置失败: %v", err)
			return nil, nil, err
		}
		if err := validateTektonDagPipelineParamBindings(tektonDagConfig, params); err != nil {
			logx.Errorf("校验 Tekton Pipeline 参数绑定失败: %v", err)
			return nil, nil, err
		}
		return steps, params, nil
	}
	return preparePipelineSnapshot(ctx, svcCtx, engineType, inSteps, inParams, templateID, projectID, userID, roles)
}

func loadPipelineStepTemplates(ctx context.Context, svcCtx *svc.ServiceContext, engineType string, steps []model.PipelineStep) (map[string]*pipelineconfigservice.GetStepTemplateResp, error) {
	stepIDs := make([]string, 0, len(steps))
	seen := make(map[string]struct{}, len(steps))
	for _, step := range steps {
		stepID := strings.TrimSpace(step.StepID)
		if stepID == "" {
			continue
		}
		if _, ok := seen[stepID]; ok {
			continue
		}
		seen[stepID] = struct{}{}
		stepIDs = append(stepIDs, stepID)
	}
	result := make(map[string]*pipelineconfigservice.GetStepTemplateResp, len(stepIDs))
	if len(stepIDs) == 0 {
		return result, nil
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	sem := make(chan struct{}, 6)
	for _, stepID := range stepIDs {
		stepID := stepID
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			resp, err := getPipelineStepTemplate(ctx, svcCtx, engineType, stepID)
			<-sem
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				return
			}
			result[stepID] = resp
		}()
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return result, nil
}

func resolveTektonProjectPipelineTemplate(ctx context.Context, svcCtx *svc.ServiceContext, templateID, projectID string, userID uint64, roles []string) (*pipelineconfigservice.DevopsPipelineTemplate, string, error) {
	templateID = strings.TrimSpace(templateID)
	if templateID == "" {
		logx.Errorf("Tekton 流水线必须选择 Pipeline 模板")
		return nil, "", errorx.Msg("Tekton 流水线必须选择 Pipeline 模板")
	}
	resp, err := svcCtx.PipelineConfigRpc.PipelineTemplateGet(ctx, &pipelineconfigservice.GetByIdReq{
		Id:            templateID,
		CurrentUserId: userID,
		CurrentRoles:  roles,
	})
	if err != nil {
		logx.Errorf("查询 Tekton Pipeline 模板失败: %v", err)
		return nil, "", err
	}
	if resp == nil || resp.Data == nil {
		logx.Errorf("Tekton Pipeline 模板不存在")
		return nil, "", errorx.Msg("Tekton Pipeline 模板不存在")
	}
	template := resp.Data
	if strings.TrimSpace(template.EngineType) != engineTekton {
		logx.Errorf("只能引用 Tekton Pipeline 模板")
		return nil, "", errorx.Msg("只能引用 Tekton Pipeline 模板")
	}
	if strings.EqualFold(strings.TrimSpace(template.Scope), "project") && strings.TrimSpace(template.ProjectId) != strings.TrimSpace(projectID) {
		logx.Errorf("项目流水线不能引用其他项目的 Tekton 模板")
		return nil, "", errorx.Msg("项目流水线不能引用其他项目的 Tekton 模板")
	}
	if !hasTektonDagConfig(template.TektonDagConfig) {
		logx.Errorf("Tekton Pipeline 模板缺少 DAG 配置")
		return nil, "", errorx.Msg("Tekton Pipeline 模板缺少 DAG 配置")
	}
	if err := validateTektonDagConfig(template.TektonDagConfig); err != nil {
		logx.Errorf("Tekton Pipeline 模板 DAG 配置无效: %v", err)
		return nil, "", err
	}
	snapshot, err := json.Marshal(template)
	if err != nil {
		logx.Errorf("生成 Tekton Pipeline 模板快照失败: %v", err)
		return nil, "", err
	}
	return template, string(snapshot), nil
}

func getPipelineStepTemplate(ctx context.Context, svcCtx *svc.ServiceContext, engineType, stepID string) (*pipelineconfigservice.GetStepTemplateResp, error) {
	if strings.TrimSpace(engineType) == engineTekton {
		return svcCtx.PipelineConfigRpc.TektonStepGet(ctx, &pipelineconfigservice.GetByIdReq{Id: stepID})
	}
	return svcCtx.PipelineConfigRpc.JenkinsStepGet(ctx, &pipelineconfigservice.GetByIdReq{Id: stepID})
}

func mergeStepParamValuesToPipelineParams(step model.PipelineStep, stepParams []*pipelineconfigservice.DevopsStepParam, params []model.PipelineParam) {
	if strings.TrimSpace(step.ParamValues) == "" || len(stepParams) == 0 || len(params) == 0 {
		return
	}
	values := make(map[string]any)
	if err := json.Unmarshal([]byte(step.ParamValues), &values); err != nil {
		return
	}
	stepParamByCode := make(map[string]*pipelineconfigservice.DevopsStepParam, len(stepParams))
	for _, item := range stepParams {
		if item == nil {
			continue
		}
		stepParamByCode[item.Code] = item
	}
	for index := range params {
		stepParam, ok := stepParamByCode[params[index].Code]
		if !ok {
			continue
		}
		value, ok := values[params[index].Code]
		if !ok || !hasParamValue(value) {
			continue
		}
		if stepParam.ParamType == objectListParamType {
			data, err := json.Marshal(value)
			if err != nil {
				continue
			}
			params[index].CurrentValue = string(data)
			continue
		}
		if stepParam.ParamType == "voucherModel" {
			if !isLikelyObjectID(strings.TrimSpace(fmt.Sprint(value))) {
				continue
			}
		}
		if stepParam.ParamType == channelvars.ParamChannelCredential {
			continue
		}
		params[index].CurrentValue = strings.TrimSpace(fmt.Sprint(value))
	}
}

func ensureUniquePipelineStepNodeNames(steps []model.PipelineStep) {
	used := map[string]struct{}{}
	for index := range steps {
		name := strings.TrimSpace(steps[index].NodeName)
		if name == "" {
			name = strings.TrimSpace(steps[index].StepName)
		}
		if name == "" {
			name = "未命名步骤"
		}
		name = uniquePipelineStepNodeName(name, used)
		used[name] = struct{}{}
		steps[index].NodeName = name
	}
}

func uniquePipelineStepNodeName(base string, used map[string]struct{}) string {
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

type pipelineStepParamRef struct {
	Step  model.PipelineStep
	Param *pipelineconfigservice.DevopsStepParam
}

func rebuildPipelineParamsFromSteps(steps []model.PipelineStep, stepParamsByNode map[string][]*pipelineconfigservice.DevopsStepParam, existing []model.PipelineParam) []model.PipelineParam {
	refs := make([]pipelineStepParamRef, 0)
	reserved := make(map[string]struct{})
	for _, step := range steps {
		for _, param := range stepParamsByNode[step.ID] {
			if param == nil {
				continue
			}
			sourceCode := strings.TrimSpace(param.Code)
			if sourceCode == "" {
				continue
			}
			reserved[sourceCode] = struct{}{}
			refs = append(refs, pipelineStepParamRef{Step: step, Param: param})
		}
	}
	existingByKey := make(map[string]model.PipelineParam, len(existing))
	legacyByCode := make(map[string][]model.PipelineParam)
	for _, item := range existing {
		sourceCode := pipelineParamSourceCode(item)
		if strings.TrimSpace(item.StepNodeID) != "" && sourceCode != "" {
			existingByKey[pipelineParamInstanceKey(item.StepNodeID, sourceCode)] = item
			continue
		}
		if sourceCode != "" {
			legacyByCode[sourceCode] = append(legacyByCode[sourceCode], item)
		}
	}

	used := make(map[string]struct{})
	occurrence := make(map[string]int)
	codeMaps := make(map[string]map[string]string)
	for _, ref := range refs {
		stepNodeID := strings.TrimSpace(ref.Step.ID)
		sourceCode := strings.TrimSpace(ref.Param.Code)
		if stepNodeID == "" || sourceCode == "" {
			continue
		}
		if _, ok := codeMaps[stepNodeID]; !ok {
			codeMaps[stepNodeID] = make(map[string]string)
		}
		existingParam, hasExisting := existingByKey[pipelineParamInstanceKey(stepNodeID, sourceCode)]
		runtimeCode := ""
		if hasExisting && canUseRuntimeParamCode(existingParam.Code, sourceCode, used, reserved) {
			runtimeCode = strings.TrimSpace(existingParam.Code)
			used[runtimeCode] = struct{}{}
			occurrence[sourceCode]++
		}
		if runtimeCode == "" {
			runtimeCode = nextPipelineRuntimeParamCode(sourceCode, occurrence, used, reserved)
		}
		codeMaps[stepNodeID][sourceCode] = runtimeCode
	}

	sortOrder := int64(1)
	legacyIndex := make(map[string]int)
	result := make([]model.PipelineParam, 0, len(refs))
	for _, ref := range refs {
		stepNodeID := strings.TrimSpace(ref.Step.ID)
		sourceCode := strings.TrimSpace(ref.Param.Code)
		if stepNodeID == "" || sourceCode == "" {
			continue
		}
		existingParam, hasExisting := existingByKey[pipelineParamInstanceKey(stepNodeID, sourceCode)]
		if !hasExisting {
			items := legacyByCode[sourceCode]
			if index := legacyIndex[sourceCode]; index < len(items) {
				existingParam = items[index]
				hasExisting = true
				legacyIndex[sourceCode] = index + 1
			}
		}
		config := remapPipelineParamConfig(managerParamConfigToModel(ref.Param.Config), codeMaps[stepNodeID])
		currentValue := pipelineStepParamCurrentValue(ref.Step, ref.Param, existingParam, hasExisting)
		readonly := ref.Param.Readonly
		runtimeConfig := ref.Param.RuntimeConfig
		if ref.Param.ParamType == channelvars.ParamChannelCredential {
			currentValue = ""
			readonly = true
			runtimeConfig = false
		}
		result = append(result, model.PipelineParam{
			Name:          strings.TrimSpace(ref.Param.Name),
			Code:          runtimeParamCode(sourceCode, codeMaps[stepNodeID]),
			SourceCode:    sourceCode,
			StepNodeID:    stepNodeID,
			DefaultValue:  ref.Param.DefaultValue,
			CurrentValue:  currentValue,
			ParamType:     strings.TrimSpace(ref.Param.ParamType),
			Mode:          normalizeParamMode(ref.Param.Mode),
			Required:      ref.Param.Required,
			Readonly:      readonly,
			Description:   ref.Param.Description,
			SelectList:    config.Options,
			AllowCustom:   ref.Param.AllowCustom,
			SortOrder:     sortOrder,
			RuntimeMode:   normalizeRuntimeMode(ref.Param.RuntimeMode, ref.Param.Mode),
			RuntimeConfig: runtimeConfig,
			Config:        config,
		})
		sortOrder++
	}
	return result
}

func pipelineStepParamCurrentValue(step model.PipelineStep, param *pipelineconfigservice.DevopsStepParam, existing model.PipelineParam, hasExisting bool) string {
	if param == nil {
		return ""
	}
	values := make(map[string]any)
	if strings.TrimSpace(step.ParamValues) != "" {
		_ = json.Unmarshal([]byte(step.ParamValues), &values)
	}
	if value, ok := values[param.Code]; ok && hasParamValue(value) {
		if param.ParamType == objectListParamType {
			data, err := json.Marshal(value)
			if err == nil {
				return string(data)
			}
			return ""
		}
		if param.ParamType == "voucherModel" && !isLikelyObjectID(strings.TrimSpace(fmt.Sprint(value))) {
			return ""
		}
		return strings.TrimSpace(fmt.Sprint(value))
	}
	if hasExisting {
		return existing.CurrentValue
	}
	return ""
}

func pipelineParamSourceCode(item model.PipelineParam) string {
	if strings.TrimSpace(item.SourceCode) != "" {
		return strings.TrimSpace(item.SourceCode)
	}
	return strings.TrimSpace(item.Code)
}

func pipelineParamInstanceKey(stepNodeID, sourceCode string) string {
	return strings.TrimSpace(stepNodeID) + "\x00" + strings.TrimSpace(sourceCode)
}

func canUseRuntimeParamCode(code, sourceCode string, used, reserved map[string]struct{}) bool {
	code = strings.TrimSpace(code)
	if code == "" {
		return false
	}
	if _, ok := used[code]; ok {
		return false
	}
	if _, ok := reserved[code]; ok && code != sourceCode {
		return false
	}
	return true
}

func nextPipelineRuntimeParamCode(sourceCode string, occurrence map[string]int, used, reserved map[string]struct{}) string {
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

func remapPipelineParamConfig(config model.StepParamConfig, codeMap map[string]string) model.StepParamConfig {
	config.ChannelParamCode = runtimeParamCode(config.ChannelParamCode, codeMap)
	config.ProjectParamCode = runtimeParamCode(config.ProjectParamCode, codeMap)
	config.ComponentParamCode = runtimeParamCode(config.ComponentParamCode, codeMap)
	config.CredentialSourceParamCode = runtimeParamCode(config.CredentialSourceParamCode, codeMap)
	for i := range config.DependencyParamCodes {
		config.DependencyParamCodes[i].ParamCode = runtimeParamCode(config.DependencyParamCodes[i].ParamCode, codeMap)
	}
	return config
}

func pipelineDynamicParamType(item model.PipelineParam) string {
	if dynamicType := channelDynamicParamType(item); dynamicType != "" {
		return dynamicType
	}
	return strings.TrimSpace(item.ParamType)
}

func channelDynamicParamType(item model.PipelineParam) string {
	if strings.TrimSpace(item.ParamType) != channelvars.ParamChannelVariable {
		return ""
	}
	config := normalizePipelineParamConfig(item)
	channelType := strings.TrimSpace(config.ChannelTypeFilter)
	groupCode := strings.TrimSpace(config.ChannelGroupCode)
	switch strings.TrimSpace(config.MappingField) {
	case channelvars.FieldDynamicProject:
		if isRepositoryChannelType(channelType) {
			return channelvars.ParamGitProject
		}
		switch channelType {
		case "harbor":
			return channelvars.ParamHarborProject
		case "registry", "aliyun_registry":
			return channelvars.ParamRegistryRepository
		}
		if channelType == "nexus" {
			return channelvars.ParamNexusRepository
		}
		if channelType == "jfrog" {
			return channelvars.ParamJfrogRepository
		}
		if channelType == "kube-nova" {
			return channelvars.ParamKubeNovaProject
		}
	case channelvars.FieldDynamicBranch:
		if isRepositoryChannelType(channelType) {
			return channelvars.ParamGitBranch
		}
	case channelvars.FieldDynamicTag:
		if isRepositoryChannelType(channelType) {
			return channelvars.ParamGitTag
		}
		if groupCode == channelvars.GroupImageRepo {
			switch channelType {
			case "harbor":
				return channelvars.ParamHarborImageTag
			case "registry", "aliyun_registry":
				return channelvars.ParamRegistryImageTag
			}
		}
	case channelvars.FieldAddressProjectURL:
		if isRepositoryChannelType(channelType) {
			return channelvars.ParamGitRepositoryURL
		}
		switch channelType {
		case "harbor":
			return channelvars.ParamHarborProjectURL
		case "registry", "aliyun_registry":
			return channelvars.ParamRegistryRepositoryURL
		}
		if channelType == "sonarqube" {
			return channelvars.ParamSonarProjectURL
		}
	case channelvars.FieldDynamicRegistry:
		switch channelType {
		case "harbor":
			return channelvars.ParamHarborProject
		case "registry", "aliyun_registry":
			return channelvars.ParamRegistryRepository
		}
	case channelvars.FieldDynamicRepository:
		switch channelType {
		case "harbor":
			return channelvars.ParamHarborProject
		case "registry", "aliyun_registry":
			return channelvars.ParamRegistryRepository
		}
		switch channelType {
		case "nexus":
			return channelvars.ParamNexusRepository
		case "jfrog":
			return channelvars.ParamJfrogRepository
		}
	case channelvars.FieldDynamicImage:
		switch channelType {
		case "harbor":
			return channelvars.ParamHarborImage
		case "registry", "aliyun_registry":
			return channelvars.ParamRegistryImage
		}
	case channelvars.FieldDynamicImageTag:
		switch channelType {
		case "harbor":
			return channelvars.ParamHarborImageTag
		case "registry", "aliyun_registry":
			return channelvars.ParamRegistryImageTag
		}
	case channelvars.FieldAddressRegistryURL:
		switch channelType {
		case "harbor":
			return channelvars.ParamHarborProjectURL
		case "registry", "aliyun_registry":
			return channelvars.ParamRegistryRepositoryURL
		}
	case channelvars.FieldAddressImageURL:
		switch channelType {
		case "harbor":
			return channelvars.ParamHarborImageURL
		case "registry", "aliyun_registry":
			return channelvars.ParamRegistryImageURL
		}
	case channelvars.FieldAddressImageTagURL:
		switch channelType {
		case "harbor":
			return channelvars.ParamHarborImageTagURL
		case "registry", "aliyun_registry":
			return channelvars.ParamRegistryImageTagURL
		}
	case channelvars.FieldDynamicArtifactName, channelvars.FieldDynamicArtifact:
		if channelType == "nexus" {
			return channelvars.ParamNexusArtifactName
		}
		if channelType == "jfrog" {
			return channelvars.ParamJfrogArtifactName
		}
	case channelvars.FieldDynamicArtifactVersion, channelvars.FieldDynamicArtifactTag:
		if channelType == "nexus" {
			return channelvars.ParamNexusArtifactVersion
		}
		if channelType == "jfrog" {
			return channelvars.ParamJfrogArtifactVersion
		}
	case channelvars.FieldAddressRepositoryURL:
		if channelType == "nexus" {
			return channelvars.ParamNexusRepositoryURL
		}
		if channelType == "jfrog" {
			return channelvars.ParamJfrogRepositoryURL
		}
	case channelvars.FieldAddressArtifactURL:
		if channelType == "nexus" {
			return channelvars.ParamNexusArtifactURL
		}
		if channelType == "jfrog" {
			return channelvars.ParamJfrogArtifactURL
		}
	case channelvars.FieldAddressArtifactVersionURL, channelvars.FieldAddressArtifactTagURL:
		if channelType == "nexus" {
			return channelvars.ParamNexusArtifactVersionURL
		}
		if channelType == "jfrog" {
			return channelvars.ParamJfrogArtifactVersionURL
		}
	case channelvars.FieldDynamicProjectName:
		if channelType == "sonarqube" {
			return channelvars.ParamSonarProjectName
		}
	case channelvars.FieldDynamicProjectKey:
		if channelType == "sonarqube" {
			return channelvars.ParamSonarProjectKey
		}
	case channelvars.FieldAddressProjectKeyURL:
		if channelType == "sonarqube" {
			return channelvars.ParamSonarProjectKeyURL
		}
	case channelvars.FieldDynamicCluster:
		if channelType == "kube-nova" {
			return channelvars.ParamKubeNovaCluster
		}
	case channelvars.FieldDynamicWorkspace:
		if channelType == "kube-nova" {
			return channelvars.ParamKubeNovaWorkspace
		}
	case channelvars.FieldDynamicApplication:
		if channelType == "kube-nova" {
			return channelvars.ParamKubeNovaApplication
		}
	case channelvars.FieldDynamicVersion:
		if channelType == "kube-nova" {
			return channelvars.ParamKubeNovaVersion
		}
	case channelvars.FieldDynamicNamespace:
		if channelType == "kubernetes" {
			return channelvars.ParamKubernetesNamespace
		}
	case channelvars.FieldDynamicWorkloadType:
		if channelType == "kubernetes" {
			return channelvars.ParamKubernetesWorkloadType
		}
	case channelvars.FieldDynamicResourceName, channelvars.FieldDynamicDeployment, channelvars.FieldDynamicStatefulSet, channelvars.FieldDynamicDaemonSet, channelvars.FieldDynamicCronJob:
		if channelType == "kubernetes" {
			return channelvars.ParamKubernetesResource
		}
	case channelvars.FieldDynamicContainerName, channelvars.FieldDynamicContainer:
		if channelType == "kube-nova" {
			return channelvars.ParamKubeNovaContainer
		}
		if channelType == "kubernetes" {
			return channelvars.ParamKubernetesContainer
		}
	case channelvars.FieldDynamicImageName:
		if channelType == "kubernetes" {
			return channelvars.ParamKubernetesImage
		}
	case channelvars.FieldDynamicDeployConfig:
		if channelType == "kube-nova" {
			return channelvars.ParamKubeNovaDeployConfig
		}
		if channelType == "kubernetes" {
			return channelvars.ParamKubernetesDeployConfig
		}
	case channelvars.FieldAddressDeployConfig:
		if channelType == "kube-nova" {
			return channelvars.ParamKubeNovaDeployConfig
		}
	case channelvars.FieldAddressDeploymentConfig:
		if channelType == "kubernetes" {
			return channelvars.ParamKubernetesDeployConfig
		}
	case channelvars.FieldAddressHostConfig:
		if channelType == "host" {
			return channelvars.ParamHostConfig
		}
	case channelvars.FieldAddressGroupConfig:
		if channelType == "host_group" {
			return channelvars.ParamHostGroupConfig
		}
	default:
		return ""
	}
	return ""
}

func validateStepRequiredParams(step model.PipelineStep, params []*pipelineconfigservice.DevopsStepParam) error {
	if len(params) == 0 {
		return nil
	}
	values := make(map[string]any)
	if strings.TrimSpace(step.ParamValues) != "" {
		if err := json.Unmarshal([]byte(step.ParamValues), &values); err != nil {
			logx.Errorf("%s", "阶段「"+displayStepName(step)+"」参数配置必须是 JSON 对象")
			return errorx.Msg("阶段「" + displayStepName(step) + "」参数配置必须是 JSON 对象")
		}
	}
	for _, item := range params {
		if item == nil {
			continue
		}
		if item.ParamType == objectListParamType {
			if err := validateTemplateObjectListValue(displayStepName(step), item, values[item.Code]); err != nil {
				logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
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
		if hasParamValue(values[item.Code]) || strings.TrimSpace(item.DefaultValue) != "" {
			continue
		}
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = item.Code
		}
		logx.Errorf("%s", "请配置阶段「"+displayStepName(step)+"」参数："+name)
		return errorx.Msg("请配置阶段「" + displayStepName(step) + "」参数：" + name)
	}
	return nil
}

func validateTemplateObjectListValue(stepName string, item *pipelineconfigservice.DevopsStepParam, raw any) error {
	if item == nil {
		return nil
	}
	if !item.Required && raw == nil {
		return nil
	}
	rows, ok := raw.([]any)
	if !ok {
		if item.Required {
			logx.Errorf("%s", "请配置阶段「"+stepName+"」参数："+item.Name)
			return errorx.Msg("请配置阶段「" + stepName + "」参数：" + item.Name)
		}
		logx.Errorf("%s", "阶段「"+stepName+"」参数「"+item.Name+"」必须是对象数组")
		return errorx.Msg("阶段「" + stepName + "」参数「" + item.Name + "」必须是对象数组")
	}
	if item.Required && len(rows) == 0 {
		logx.Errorf("%s", "请配置阶段「"+stepName+"」参数："+item.Name)
		return errorx.Msg("请配置阶段「" + stepName + "」参数：" + item.Name)
	}
	fields := item.GetConfig().GetCompoundFields()
	for _, row := range rows {
		rowMap, ok := row.(map[string]any)
		if !ok {
			logx.Errorf("%s", "阶段「"+stepName+"」参数「"+item.Name+"」必须是对象数组")
			return errorx.Msg("阶段「" + stepName + "」参数「" + item.Name + "」必须是对象数组")
		}
		for _, field := range fields {
			if field == nil {
				continue
			}
			if field.ParamType == channelvars.ParamChannelCredential {
				continue
			}
			if !field.Required {
				continue
			}
			if hasParamValue(rowMap[field.Code]) || strings.TrimSpace(field.DefaultValue) != "" {
				continue
			}
			logx.Errorf("%s", "阶段「"+stepName+"」参数「"+item.Name+"」缺少必填字段："+field.Name)
			return errorx.Msg("阶段「" + stepName + "」参数「" + item.Name + "」缺少必填字段：" + field.Name)
		}
	}
	return nil
}

func hasParamValue(value any) bool {
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

func displayStepName(step model.PipelineStep) string {
	if strings.TrimSpace(step.NodeName) != "" {
		return strings.TrimSpace(step.NodeName)
	}
	if strings.TrimSpace(step.StepName) != "" {
		return strings.TrimSpace(step.StepName)
	}
	return "未命名步骤"
}

func validatePipelinePostConditionUniqueness(steps []model.PipelineStep) error {
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
		postSteps = append(postSteps, jenkins.JenkinsRenderStep{
			ID:           item.ID,
			NodeName:     displayStepName(item),
			Enabled:      item.Enabled,
			StageContent: item.StageContent,
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

func pipelinePostNodeIDs(items []model.PipelineStep) map[string]struct{} {
	byID := make(map[string]model.PipelineStep, len(items))
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

func validateRequiredParams(params []model.PipelineParam) error {
	for _, item := range params {
		if effectiveParamRuntimeMode(item) != "env" {
			continue
		}
		if item.ParamType == objectListParamType {
			if err := validateRequiredObjectListParam(item); err != nil {
				logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
				return err
			}
			continue
		}
		if item.ParamType == channelvars.ParamChannelCredential {
			continue
		}
		if item.Required && strings.TrimSpace(item.CurrentValue) == "" && strings.TrimSpace(item.DefaultValue) == "" {
			logx.Errorf("%s", "请配置必填参数："+item.Name)
			return errorx.Msg("请配置必填参数：" + item.Name)
		}
	}
	return nil
}

func validatePipelineParams(engineType string, params []model.PipelineParam) error {
	seen := make(map[string]struct{}, len(params))
	paramByCode := make(map[string]model.PipelineParam, len(params))
	channelParamByType := make(map[string]string)
	for index := range params {
		params[index].Code = strings.TrimSpace(params[index].Code)
		params[index].Mode = normalizeParamMode(params[index].Mode)
		params[index].RuntimeMode = normalizeRuntimeMode(params[index].RuntimeMode, params[index].Mode)
		if params[index].Name == "" {
			logx.Errorf("参数名称不能为空")
			return errorx.Msg("参数名称不能为空")
		}
		if err := validateCode(params[index].Code, "参数编码"); err != nil {
			logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
			return err
		}
		if _, ok := seen[params[index].Code]; ok {
			logx.Errorf("流水线参数编码不能重复")
			return errorx.Msg("流水线参数编码不能重复")
		}
		seen[params[index].Code] = struct{}{}
		paramByCode[params[index].Code] = params[index]
	}
	for _, item := range params {
		if item.ParamType == objectListParamType {
			var err error
			if engineType == engineTekton {
				err = validateTektonPipelineObjectListParam(item)
			} else {
				err = validatePipelineObjectListParam(item)
			}
			if err != nil {
				logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
				return err
			}
			continue
		}
		if engineType == engineTekton {
			if err := validateTektonPipelineParam(item, paramByCode); err != nil {
				logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
				return err
			}
			continue
		}
		if err := validatePipelineParamModeType(item, paramByCode); err != nil {
			logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
			return err
		}
		if item.Mode == "paas" && isPipelinePlainChannelParam(item) {
			config := normalizePipelineParamConfig(item)
			key := strings.TrimSpace(config.ChannelGroupCode) + "\x00" + strings.TrimSpace(config.ChannelTypeFilter) + "\x00" + strings.TrimSpace(item.StepNodeID)
			if previous := channelParamByType[key]; previous != "" {
				logx.Errorf("%s", "同一流水线普通参数不能重复使用同类型渠道："+config.ChannelGroupCode+"/"+config.ChannelTypeFilter)
				return errorx.Msg("同一流水线普通参数不能重复使用同类型渠道：" + config.ChannelGroupCode + "/" + config.ChannelTypeFilter)
			}
			channelParamByType[key] = item.Code
		}
	}
	return nil
}

func isPipelinePlainChannelParam(item model.PipelineParam) bool {
	if !channelvars.IsChannelParamType(item.ParamType) {
		return false
	}
	if strings.TrimSpace(item.ParamType) != channelvars.ParamChannelVariable {
		return true
	}
	return strings.TrimSpace(normalizePipelineParamConfig(item).MappingField) == channelvars.FieldEndpoint
}

func validateTektonPipelineParam(item model.PipelineParam, paramByCode map[string]model.PipelineParam) error {
	config := normalizePipelineParamConfig(item)
	if !isSupportedTektonPipelineParamType(item.ParamType) {
		logx.Errorf("参数类型不支持")
		return errorx.Msg("参数类型不支持")
	}
	if item.Mode != "params" || item.RuntimeMode != "params" {
		logx.Errorf("Tekton 平台参数只支持 params 映射")
		return errorx.Msg("Tekton 平台参数只支持 params 映射")
	}
	if item.ParamType == channelvars.ParamChannelCredential {
		if config.CredentialMode == credentialModeJenkinsID {
			logx.Errorf("Tekton 渠道凭证不支持 Jenkins credentialId 模式")
			return errorx.Msg("Tekton 渠道凭证不支持 Jenkins credentialId 模式")
		}
		return validateChannelCredentialPipelineParam(item, config, paramByCode)
	}
	switch item.ParamType {
	case "choice":
		if len(config.Options) == 0 {
			logx.Errorf("选项参数必须配置选项")
			return errorx.Msg("选项参数必须配置选项")
		}
	case "list":
		if len(config.Options) == 0 {
			if err := validateTektonPipelineStructuredValue(item); err != nil {
				logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
				return err
			}
		}
	case "array", "object", objectListParamType:
		if err := validateTektonPipelineStructuredValue(item); err != nil {
			logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
			return err
		}
	case "booleanParam", "number":
		if err := validatePipelineParamScalarValue(item.ParamType, item.DefaultValue, item.CurrentValue, item.Name); err != nil {
			logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
			return err
		}
	case channelvars.ParamKubernetesSecretName:
		if err := validateTektonPipelineKubernetesSecretParam(item, config); err != nil {
			logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
			return err
		}
	case channelvars.ParamKubernetesPVCName:
		if err := validatePipelineKubernetesPVCName(item.DefaultValue, item.CurrentValue, item.Name); err != nil {
			logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
			return err
		}
	case channelvars.ParamChannelVariable:
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
		if channelVariablePipelineRequiresBinding(config) && !channelVariablePipelineUsesAddressDependency(config, paramByCode) {
			if err := validateChannelVariablePipelineBinding(config, paramByCode); err != nil {
				logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
				return err
			}
		}
		if err := validateChannelVariablePipelineDependencies(item, config, paramByCode); err != nil {
			logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
			return err
		}
	case "voucherModel":
		if config.VoucherModel == "" {
			logx.Errorf("凭证变量必须配置凭证类型")
			return errorx.Msg("凭证变量必须配置凭证类型")
		}
	case channelvars.ParamConfigCenter:
		if strings.TrimSpace(config.ConfigTypeID) == "" && strings.TrimSpace(config.ConfigTypeCode) == "" {
			logx.Errorf("配置中心变量必须选择配置类型")
			return errorx.Msg("配置中心变量必须选择配置类型")
		}
	case channelvars.ParamHostGroupTargets:
		if config.RenderMode == "" {
			config.RenderMode = "normal"
		}
		if !isSupportedHostGroupRenderMode(config.RenderMode) {
			logx.Errorf("主机组输出模式不支持")
			return errorx.Msg("主机组输出模式不支持")
		}
	}
	return nil
}

func validateTektonDagPipelineParams(params []model.PipelineParam) error {
	for _, item := range params {
		if isSupportedTektonDagPipelineParamType(item.ParamType) {
			continue
		}
		logx.Errorf("Tekton DAG 参数类型不支持: param=%s type=%s", item.Code, item.ParamType)
		return errorx.Msg("Tekton DAG 参数类型不支持：" + displayPipelineParamName(item))
	}
	return nil
}

func isSupportedTektonDagPipelineParamType(paramType string) bool {
	if channelvars.IsCompositeAddressParamType(paramType) {
		return true
	}
	switch strings.TrimSpace(paramType) {
	case "string", "text", "booleanParam", "number", "password", "choice", "file", "array", "object", "list", objectListParamType,
		channelvars.ParamChannelVariable,
		"voucherModel",
		channelvars.ParamChannelCredential,
		channelvars.ParamConfigCenter,
		channelvars.ParamHostGroupTargets:
		return true
	default:
		return channelvars.IsDynamicParamType(paramType)
	}
}

func validateTektonPipelineStructuredValue(item model.PipelineParam) error {
	paramType := tektonParamType(firstNotBlank(item.ParamType, "string"))
	for _, value := range []string{item.DefaultValue, item.CurrentValue} {
		if strings.TrimSpace(value) == "" {
			continue
		}
		if _, err := tektonParamDefaultValue(paramType, value, displayPipelineParamName(item)); err != nil {
			return err
		}
	}
	return nil
}

func validateTektonPipelineKubernetesSecretParam(item model.PipelineParam, config model.StepParamConfig) error {
	if strings.TrimSpace(config.VoucherModel) == "" {
		return validatePipelineKubernetesSecretName(item.DefaultValue, item.CurrentValue, item.Name)
	}
	if !isSupportedTektonCredentialType(config.VoucherModel) {
		logx.Errorf("Kubernetes Secret 凭证类型不支持: %s", config.VoucherModel)
		return errorx.Msg("Kubernetes Secret 凭证类型不支持")
	}
	if !isSupportedTektonSecretProvider(config.Provider, config.VoucherModel) {
		logx.Errorf("Kubernetes Secret 投影类型不匹配: provider=%s credentialType=%s", config.Provider, config.VoucherModel)
		return errorx.Msg("Kubernetes Secret 投影类型与凭证类型不匹配")
	}
	for _, value := range []string{item.DefaultValue, item.CurrentValue} {
		value = strings.TrimSpace(value)
		if value != "" && !isLikelyObjectID(value) {
			logx.Errorf("Kubernetes Secret 平台凭证参数值不是凭证 ID: param=%s", item.Code)
			return errorx.Msg("Kubernetes Secret 参数必须选择平台凭证：" + displayPipelineParamName(item))
		}
	}
	return nil
}

func isSupportedTektonCredentialType(credentialType string) bool {
	switch strings.TrimSpace(credentialType) {
	case "username_password", "token", "ssh_key", "kubeconfig", "secret_text", "certificate":
		return true
	default:
		return false
	}
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

func isSupportedTektonPipelineParamType(paramType string) bool {
	if channelvars.IsCompositeAddressParamType(paramType) {
		return true
	}
	switch paramType {
	case "string", "text", "booleanParam", "number", "password", "choice", "file", "array", "object", channelvars.ParamKubernetesSecretName, channelvars.ParamKubernetesPVCName, "list", "channelGroupCode", "voucherModel",
		channelvars.ParamChannelCredential,
		channelvars.ParamRepositoryChannel, channelvars.ParamGitProject, channelvars.ParamGitRepositoryURL, channelvars.ParamGitBranch, channelvars.ParamGitTag,
		channelvars.ParamNexusChannel, channelvars.ParamNexusRepository, channelvars.ParamNexusArtifactName, channelvars.ParamNexusArtifactVersion, channelvars.ParamNexusRepositoryURL, channelvars.ParamNexusArtifactURL, channelvars.ParamNexusArtifactVersionURL,
		channelvars.ParamHarborChannel, channelvars.ParamHarborProject, channelvars.ParamHarborImage, channelvars.ParamHarborProjectURL, channelvars.ParamHarborImageURL, channelvars.ParamHarborImageTagURL,
		channelvars.ParamJfrogChannel, channelvars.ParamJfrogRepository, channelvars.ParamJfrogArtifactName, channelvars.ParamJfrogArtifactVersion, channelvars.ParamJfrogRepositoryURL, channelvars.ParamJfrogArtifactURL, channelvars.ParamJfrogArtifactVersionURL,
		channelvars.ParamMavenConfig, channelvars.ParamConfigCenter, channelvars.ParamSonarAddress, channelvars.ParamSonarProjectName, channelvars.ParamSonarProjectKey, channelvars.ParamHostGroupTargets, channelvars.ParamKubeNovaDeployConfig, channelvars.ParamKubernetesDeployConfig,
		channelvars.ParamKubernetesNamespace, channelvars.ParamKubernetesWorkloadType, channelvars.ParamKubernetesResource, channelvars.ParamKubernetesContainer, channelvars.ParamKubernetesImage:
		return true
	default:
		return false
	}
}

func normalizeHostGroupRenderMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "normal":
		return "normal"
	case "json":
		return "json"
	case "inventory", "ansible":
		return "inventory"
	case "jsonbase64":
		return "jsonBase64"
	case "ansiblebase64":
		return "ansibleBase64"
	default:
		return strings.TrimSpace(value)
	}
}

func isSupportedHostGroupRenderMode(value string) bool {
	switch normalizeHostGroupRenderMode(value) {
	case "normal", "json", "jsonBase64", "inventory", "ansibleBase64":
		return true
	default:
		return false
	}
}

func validatePipelineParamModeType(item model.PipelineParam, paramByCode map[string]model.PipelineParam) error {
	config := normalizePipelineParamConfig(item)
	switch item.Mode {
	case "env":
		switch item.ParamType {
		case "string", "text", "booleanParam", "number":
			if err := validatePipelineParamScalarValue(item.ParamType, item.DefaultValue, item.CurrentValue, item.Name); err != nil {
				logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
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
			if err := validatePipelineParamScalarValue(item.ParamType, item.DefaultValue, item.CurrentValue, item.Name); err != nil {
				logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
				return err
			}
		case channelvars.ParamKubernetesSecretName:
			if err := validatePipelineKubernetesSecretName(item.DefaultValue, item.CurrentValue, item.Name); err != nil {
				logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
				return err
			}
		case channelvars.ParamKubernetesPVCName:
			if err := validatePipelineKubernetesPVCName(item.DefaultValue, item.CurrentValue, item.Name); err != nil {
				logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
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
			if channelVariablePipelineRequiresBinding(config) && !channelVariablePipelineUsesAddressDependency(config, paramByCode) {
				if err := validateChannelVariablePipelineBinding(config, paramByCode); err != nil {
					logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
					return err
				}
			}
			if err := validateChannelVariablePipelineDependencies(item, config, paramByCode); err != nil {
				logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
				return err
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
			}
		case channelvars.ParamChannelCredential:
			if err := validateChannelCredentialPipelineParam(item, config, paramByCode); err != nil {
				logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
				return err
			}
		case channelvars.ParamRepositoryChannel, channelvars.ParamNexusChannel, channelvars.ParamHarborChannel, channelvars.ParamJfrogChannel, channelvars.ParamKubeNovaDeployConfig, channelvars.ParamKubernetesDeployConfig:
			if item.ParamType == channelvars.ParamKubeNovaDeployConfig || item.ParamType == channelvars.ParamKubernetesDeployConfig {
				config.ValueMode = normalizeDeployConfigValueMode(config.ValueMode)
			}
			if config.MappingField == "" {
				switch item.ParamType {
				case channelvars.ParamKubeNovaDeployConfig:
					config.MappingField = channelvars.FieldAddressDeployConfig
				case channelvars.ParamKubernetesDeployConfig:
					config.MappingField = channelvars.FieldAddressDeploymentConfig
				default:
					config.MappingField = "endpoint"
				}
			}
			if config.ChannelGroupCode == "" {
				config.ChannelGroupCode = channelvars.ChannelGroupCode(item.ParamType)
			}
			if config.ChannelGroupCode != channelvars.ChannelGroupCode(item.ParamType) {
				logx.Errorf("平台渠道变量必须使用对应渠道分组")
				return errorx.Msg("平台渠道变量必须使用对应渠道分组")
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
			if err := validateDynamicPipelineParamDependency(item, paramByCode); err != nil {
				logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
				return err
			}
		default:
			logx.Errorf("平台变量类型不支持")
			return errorx.Msg("平台变量类型不支持")
		}
	default:
		logx.Errorf("参数模式不支持")
		return errorx.Msg("参数模式不支持")
	}
	return nil
}

func validateChannelCredentialPipelineParam(item model.PipelineParam, config model.StepParamConfig, paramByCode map[string]model.PipelineParam) error {
	if item.Mode != "paas" && item.Mode != "params" {
		logx.Errorf("渠道凭证只支持平台变量")
		return errorx.Msg("渠道凭证只支持平台变量")
	}
	sourceCode := strings.TrimSpace(config.CredentialSourceParamCode)
	if sourceCode == "" {
		logx.Errorf("渠道凭证参数缺少渠道来源")
		return errorx.Msg("渠道凭证参数缺少渠道来源")
	}
	sourceParam, ok := paramByCode[sourceCode]
	if !ok || !isCredentialSourcePipelineParam(sourceParam) {
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

func isCredentialSourcePipelineParam(item model.PipelineParam) bool {
	if channelvars.IsChannelParamType(item.ParamType) && strings.TrimSpace(item.ParamType) != channelvars.ParamChannelVariable {
		return true
	}
	if channelvars.IsCompositeAddressParamType(item.ParamType) {
		return true
	}
	if strings.TrimSpace(item.ParamType) != channelvars.ParamChannelVariable {
		return false
	}
	config := normalizePipelineParamConfig(item)
	if strings.TrimSpace(config.MappingField) == channelvars.FieldEndpoint {
		return true
	}
	if strings.HasPrefix(strings.TrimSpace(config.MappingField), "address.") {
		return true
	}
	dynamicType := pipelineDynamicParamType(item)
	return channelvars.IsCompositeAddressParamType(dynamicType)
}

func validateDynamicPipelineParamDependency(item model.PipelineParam, paramByCode map[string]model.PipelineParam) error {
	config := normalizePipelineParamConfig(item)
	paramType := pipelineDynamicParamType(item)
	spec, ok := channelvars.DynamicSpecFor(paramType)
	if !ok {
		logx.Errorf("平台动态参数类型不支持")
		return errorx.Msg("平台动态参数类型不支持")
	}
	if strings.TrimSpace(config.ChannelParamCode) != "" {
		if err := validateDynamicPipelineChannelParam(config.ChannelParamCode, spec.ChannelParamType, spec.GroupCode, paramByCode); err != nil {
			logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
			return err
		}
	}
	if err := validateDynamicPipelineProjectParam(paramType, config, spec, paramByCode); err != nil {
		logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
		return err
	}
	return nil
}

func validateChannelVariablePipelineDynamicParamDependency(item model.PipelineParam, config model.StepParamConfig, paramByCode map[string]model.PipelineParam) error {
	paramType := pipelineDynamicParamType(item)
	spec, ok := channelvars.DynamicSpecFor(paramType)
	if !ok {
		logx.Errorf("平台动态参数类型不支持")
		return errorx.Msg("平台动态参数类型不支持")
	}
	if strings.TrimSpace(config.ChannelParamCode) != "" {
		if err := validateDynamicPipelineChannelParam(config.ChannelParamCode, spec.ChannelParamType, spec.GroupCode, paramByCode); err != nil {
			logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
			return err
		}
	}
	if err := validateDynamicPipelineProjectParam(paramType, config, spec, paramByCode); err != nil {
		logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
		return err
	}
	return nil
}

func validateChannelVariablePipelineDependencies(item model.PipelineParam, config model.StepParamConfig, paramByCode map[string]model.PipelineParam) error {
	if handled, err := validateCodeRepoPipelineChannelVariableDependencies(item, config, paramByCode); handled {
		return err
	}
	if handled, err := validateImageRegistryPipelineChannelVariableDependencies(item, config, paramByCode); handled {
		return err
	}
	requiredFields := requiredPipelineChannelVariableDependencies(config)
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
		paramCode := dependencyByField[field]
		if paramCode == "" {
			continue
		}
		target, ok := paramByCode[paramCode]
		if !ok || !samePipelineChannelVariableDependency(config, field, target) {
			logx.Errorf("渠道变量依赖参数不匹配: field=%s param=%s", field, paramCode)
			return errorx.Msg("渠道变量依赖参数必须同分组、同类型、同字段")
		}
	}
	_ = item
	return nil
}

func validateCodeRepoPipelineChannelVariableDependencies(item model.PipelineParam, config model.StepParamConfig, paramByCode map[string]model.PipelineParam) (bool, error) {
	if strings.TrimSpace(config.ChannelGroupCode) != channelvars.GroupCodeRepo {
		return false, nil
	}
	switch strings.TrimSpace(config.MappingField) {
	case channelvars.FieldDynamicProject:
		_ = item
		return true, nil
	case channelvars.FieldDynamicBranch, channelvars.FieldDynamicTag:
		_ = item
		return true, nil
	default:
		return false, nil
	}
}

func codeRepoPipelineUsesAddressDependency(config model.StepParamConfig, paramByCode map[string]model.PipelineParam) bool {
	if strings.TrimSpace(config.ChannelGroupCode) != channelvars.GroupCodeRepo {
		return false
	}
	switch strings.TrimSpace(config.MappingField) {
	case channelvars.FieldDynamicBranch, channelvars.FieldDynamicTag:
		return hasSamePipelineChannelVariableFieldDependency(config, channelvars.FieldAddressProjectURL, paramByCode)
	default:
		return false
	}
}

func channelVariablePipelineUsesAddressDependency(config model.StepParamConfig, paramByCode map[string]model.PipelineParam) bool {
	if codeRepoPipelineUsesAddressDependency(config, paramByCode) {
		return true
	}
	if strings.TrimSpace(config.ChannelGroupCode) != channelvars.GroupImageRepo {
		return false
	}
	switch strings.TrimSpace(config.MappingField) {
	case channelvars.FieldDynamicImage, channelvars.FieldDynamicTag, channelvars.FieldDynamicImageTag:
		return hasAnyPipelineImageRegistryAddressDependency(config, paramByCode)
	default:
		return false
	}
}

func validateImageRegistryPipelineChannelVariableDependencies(item model.PipelineParam, config model.StepParamConfig, paramByCode map[string]model.PipelineParam) (bool, error) {
	if strings.TrimSpace(config.ChannelGroupCode) != channelvars.GroupImageRepo {
		return false, nil
	}
	switch strings.TrimSpace(config.MappingField) {
	case channelvars.FieldDynamicProject:
		_ = item
		return true, nil
	case channelvars.FieldDynamicImage:
		_ = item
		return true, nil
	case channelvars.FieldDynamicTag:
		_ = item
		return true, nil
	case channelvars.FieldDynamicImageTag:
		_ = item
		return true, nil
	default:
		return false, nil
	}
}

func hasAnyPipelineImageRegistryAddressDependency(config model.StepParamConfig, paramByCode map[string]model.PipelineParam) bool {
	for _, field := range imageRegistryAddressFields() {
		if hasSamePipelineChannelVariableFieldDependency(config, field, paramByCode) {
			return true
		}
	}
	return false
}

func hasPipelineChannelVariableEndpointDependency(config model.StepParamConfig, paramByCode map[string]model.PipelineParam) bool {
	if strings.TrimSpace(config.ChannelBindingID) != "" {
		return true
	}
	channelParamCode := strings.TrimSpace(config.ChannelParamCode)
	if channelParamCode == "" {
		return false
	}
	target, ok := paramByCode[channelParamCode]
	return ok && samePipelineChannelVariableDependency(config, channelvars.FieldEndpoint, target)
}

func hasSamePipelineChannelVariableFieldDependency(config model.StepParamConfig, field string, paramByCode map[string]model.PipelineParam) bool {
	if strings.TrimSpace(config.ProjectParamCode) != "" {
		if target, ok := paramByCode[strings.TrimSpace(config.ProjectParamCode)]; ok && samePipelineChannelVariableDependency(config, field, target) {
			return true
		}
	}
	for _, dep := range config.DependencyParamCodes {
		paramCode := strings.TrimSpace(dep.ParamCode)
		if paramCode == "" {
			continue
		}
		target, ok := paramByCode[paramCode]
		if ok && samePipelineChannelVariableDependency(config, field, target) {
			return true
		}
	}
	return false
}

func validateChannelVariablePipelineBinding(config model.StepParamConfig, paramByCode map[string]model.PipelineParam) error {
	if strings.TrimSpace(config.ChannelParamCode) == "" && strings.TrimSpace(config.ChannelBindingID) == "" {
		return nil
	}
	if strings.TrimSpace(config.ChannelParamCode) == "" {
		return nil
	}
	target, ok := paramByCode[strings.TrimSpace(config.ChannelParamCode)]
	if !ok || !samePipelineChannelVariableDependency(config, channelvars.FieldEndpoint, target) {
		logx.Errorf("渠道变量渠道来源不匹配: param=%s", config.ChannelParamCode)
		return errorx.Msg("渠道变量渠道来源必须同分组、同类型、渠道实例来源")
	}
	return nil
}

func channelVariablePipelineRequiresBinding(config model.StepParamConfig) bool {
	return false
}

func requiredPipelineChannelVariableDependencies(config model.StepParamConfig) []string {
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

func samePipelineChannelVariableDependency(source model.StepParamConfig, depField string, target model.PipelineParam) bool {
	targetConfig := normalizePipelineParamConfig(target)
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

func validateDynamicPipelineProjectParam(paramType string, config model.StepParamConfig, spec channelvars.DynamicSpec, paramByCode map[string]model.PipelineParam) error {
	if !spec.RequireProject {
		return nil
	}
	if config.ProjectParamCode == "" {
		return nil
	}
	projectParam, ok := paramByCode[config.ProjectParamCode]
	projectType := pipelineDynamicParamType(projectParam)
	if !ok || !isAllowedDynamicPipelineProjectParamType(paramType, projectType, spec.ProjectParamType) {
		logx.Errorf("动态参数依赖的项目参数不匹配")
		return errorx.Msg("动态参数依赖的项目参数不匹配")
	}
	return nil
}

func allowGitRepositoryURLPipelineProjectDependency(item model.PipelineParam, paramByCode map[string]model.PipelineParam) bool {
	switch pipelineDynamicParamType(item) {
	case channelvars.ParamGitBranch, channelvars.ParamGitTag:
	default:
		return false
	}
	config := normalizePipelineParamConfig(item)
	projectParam, ok := paramByCode[strings.TrimSpace(config.ProjectParamCode)]
	return ok && pipelineDynamicParamType(projectParam) == channelvars.ParamGitRepositoryURL
}

func isAllowedDynamicPipelineProjectParamType(paramType, actualType, expectedType string) bool {
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

func validateDynamicPipelineChannelParam(code, paramType, groupCode string, paramByCode map[string]model.PipelineParam) error {
	if strings.TrimSpace(code) == "" {
		return nil
	}
	param, ok := paramByCode[code]
	if ok && isChannelVariablePipelineBindingParam(param, groupCode) {
		return nil
	}
	actualType := pipelineDynamicParamType(param)
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
	config := normalizePipelineParamConfig(param)
	if config.ChannelGroupCode == "" {
		config.ChannelGroupCode = channelvars.ChannelGroupCode(param.ParamType)
	}
	if config.ChannelGroupCode != groupCode {
		logx.Errorf("动态参数依赖的渠道分组不匹配")
		return errorx.Msg("动态参数依赖的渠道分组不匹配")
	}
	return nil
}

func isChannelVariablePipelineBindingParam(item model.PipelineParam, groupCode string) bool {
	if strings.TrimSpace(item.ParamType) != channelvars.ParamChannelVariable {
		return false
	}
	config := normalizePipelineParamConfig(item)
	if strings.TrimSpace(config.ChannelGroupCode) != groupCode {
		return false
	}
	return strings.TrimSpace(config.MappingField) == channelvars.FieldEndpoint
}

func validatePipelineParamScalarValue(paramType, defaultValue, currentValue, name string) error {
	for _, value := range []string{defaultValue, currentValue} {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		switch paramType {
		case "booleanParam":
			if !isBoolLiteral(value) {
				logx.Errorf("%s", "布尔参数值只能是 true 或 false："+name)
				return errorx.Msg("布尔参数值只能是 true 或 false：" + name)
			}
		case "number":
			if _, err := strconv.ParseFloat(value, 64); err != nil {
				logx.Errorf("%s", "数字参数值必须是合法数字："+name)
				return errorx.Msg("数字参数值必须是合法数字：" + name)
			}
		}
	}
	return nil
}

func validatePipelineKubernetesSecretName(defaultValue, currentValue, name string) error {
	for _, value := range []string{defaultValue, currentValue} {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if !kubernetesSecretNamePattern.MatchString(value) {
			logx.Errorf("%s", "Kubernetes Secret 名称不合法："+name)
			return errorx.Msg("Kubernetes Secret 名称不合法：" + name)
		}
	}
	return nil
}

func validatePipelineKubernetesPVCName(defaultValue, currentValue, name string) error {
	for _, value := range []string{defaultValue, currentValue} {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if !kubernetesSecretNamePattern.MatchString(value) {
			logx.Errorf("%s", "Kubernetes PVC 名称不合法："+name)
			return errorx.Msg("Kubernetes PVC 名称不合法：" + name)
		}
	}
	return nil
}

func isBoolLiteral(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "true" || value == "false"
}

func validateReadonlyPipelineParams(oldParams, newParams []model.PipelineParam) error {
	oldItems := append([]model.PipelineParam(nil), oldParams...)
	newItems := append([]model.PipelineParam(nil), newParams...)
	normalizePipelineParams(oldItems)
	normalizePipelineParams(newItems)
	nextByKey := readonlyPipelineParamMap(newItems)
	nextByCode := readonlyPipelineParamCodeMap(newItems)
	for _, oldParam := range oldItems {
		if !oldParam.Readonly {
			continue
		}
		nextParam, ok := nextByKey[readonlyPipelineParamKey(oldParam)]
		if !ok && strings.TrimSpace(oldParam.StepNodeID) == "" {
			nextParam, ok = nextByCode[strings.TrimSpace(oldParam.Code)]
		}
		if !ok {
			continue
		}
		if !sameReadonlyPipelineParamValue(oldParam, nextParam) {
			logx.Errorf("不允许修改的流水线参数只能查看，不能修改：stepNodeId=%s code=%s", oldParam.StepNodeID, oldParam.Code)
			return errorx.Msg("不允许修改的流水线参数只能查看，不能修改")
		}
	}
	return nil
}

func validatePipelineObjectListParam(item model.PipelineParam) error {
	mode := normalizeParamMode(item.Mode)
	if mode != "env" && mode != "paas" {
		logx.Errorf("对象列表只支持环境变量或平台变量模式")
		return errorx.Msg("对象列表只支持环境变量或平台变量模式")
	}
	runtimeMode := effectiveParamRuntimeMode(item)
	if runtimeMode != "env" && runtimeMode != "params" {
		logx.Errorf("对象列表输出方式不支持")
		return errorx.Msg("对象列表输出方式不支持")
	}
	fields := normalizePipelineParamConfig(item).CompoundFields
	if len(fields) == 0 {
		logx.Errorf("对象列表必须配置字段")
		return errorx.Msg("对象列表必须配置字段")
	}
	seen := make(map[string]struct{}, len(fields))
	paramByCode := make(map[string]model.PipelineParam, len(fields))
	for _, field := range fields {
		if strings.TrimSpace(field.Name) == "" {
			logx.Errorf("对象列表字段名称不能为空")
			return errorx.Msg("对象列表字段名称不能为空")
		}
		if err := validateCode(field.Code, "字段编码"); err != nil {
			logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
			return err
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
		paramByCode[field.Code] = model.PipelineParam{
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
		if err := validatePipelineParamModeType(paramByCode[field.Code], paramByCode); err != nil {
			logx.Errorf("校验对象列表字段失败: field=%s err=%v", field.Name, err)
			return errorx.Msg("对象列表字段配置错误：" + field.Name)
		}
	}
	return validateObjectListJSONString(item.Name, item.CurrentValue, item.DefaultValue, fields, item.Required)
}

func validateTektonPipelineObjectListParam(item model.PipelineParam) error {
	if item.Mode != "params" || effectiveParamRuntimeMode(item) != "params" {
		logx.Errorf("Tekton 对象列表只支持执行变量映射")
		return errorx.Msg("Tekton 对象列表只支持执行变量映射")
	}
	fields := normalizePipelineParamConfig(item).CompoundFields
	if len(fields) == 0 {
		logx.Errorf("对象列表必须配置字段")
		return errorx.Msg("对象列表必须配置字段")
	}
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		if strings.TrimSpace(field.Name) == "" {
			logx.Errorf("对象列表字段名称不能为空")
			return errorx.Msg("对象列表字段名称不能为空")
		}
		if err := validateCode(field.Code, "字段编码"); err != nil {
			logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
			return err
		}
		if field.ParamType == objectListParamType || field.ParamType == "file" {
			logx.Errorf("对象列表字段类型不支持")
			return errorx.Msg("对象列表字段类型不支持：" + field.Name)
		}
		if _, ok := seen[field.Code]; ok {
			logx.Errorf("对象列表字段编码不能重复")
			return errorx.Msg("对象列表字段编码不能重复")
		}
		seen[field.Code] = struct{}{}
	}
	return validateObjectListJSONString(item.Name, item.CurrentValue, item.DefaultValue, fields, item.Required)
}

func validateRequiredObjectListParam(item model.PipelineParam) error {
	return validateObjectListJSONString(item.Name, item.CurrentValue, item.DefaultValue, normalizePipelineParamConfig(item).CompoundFields, item.Required)
}

func validateObjectListJSONString(name, currentValue, defaultValue string, fields []model.CompoundParamField, required bool) error {
	raw := strings.TrimSpace(currentValue)
	if raw == "" {
		raw = strings.TrimSpace(defaultValue)
	}
	if raw == "" {
		if required {
			logx.Errorf("%s", "请配置必填参数："+name)
			return errorx.Msg("请配置必填参数：" + name)
		}
		return nil
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		logx.Errorf("对象列表参数必须是 JSON 数组")
		return errorx.Msg("对象列表参数必须是 JSON 数组")
	}
	if required && len(rows) == 0 {
		logx.Errorf("%s", "请配置必填参数："+name)
		return errorx.Msg("请配置必填参数：" + name)
	}
	for _, row := range rows {
		for _, field := range fields {
			value, ok := row[field.Code]
			if ok && hasParamValue(value) {
				if err := validateObjectListFieldScalarValue(field, value, name); err != nil {
					logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
					return err
				}
			}
			if !field.Required {
				continue
			}
			if ok && hasParamValue(value) {
				continue
			}
			if strings.TrimSpace(field.DefaultValue) != "" {
				continue
			}
			logx.Errorf("%s", "对象列表参数「"+name+"」缺少必填字段："+field.Name)
			return errorx.Msg("对象列表参数「" + name + "」缺少必填字段：" + field.Name)
		}
	}
	return nil
}

func validateObjectListFieldScalarValue(field model.CompoundParamField, value any, objectName string) error {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" {
		return nil
	}
	switch field.ParamType {
	case "booleanParam":
		if !isBoolLiteral(text) {
			logx.Errorf("%s", "对象列表参数「"+objectName+"」字段「"+field.Name+"」必须是 true 或 false")
			return errorx.Msg("对象列表参数「" + objectName + "」字段「" + field.Name + "」必须是 true 或 false")
		}
	case "number":
		if _, err := strconv.ParseFloat(text, 64); err != nil {
			logx.Errorf("%s", "对象列表参数「"+objectName+"」字段「"+field.Name+"」必须是合法数字")
			return errorx.Msg("对象列表参数「" + objectName + "」字段「" + field.Name + "」必须是合法数字")
		}
	}
	return nil
}

func sameReadonlyPipelineParamValue(left, right model.PipelineParam) bool {
	return readonlyPipelineParamValue(left) == readonlyPipelineParamValue(right)
}

func readonlyPipelineParamMap(items []model.PipelineParam) map[string]model.PipelineParam {
	result := make(map[string]model.PipelineParam, len(items))
	for _, item := range items {
		key := readonlyPipelineParamKey(item)
		if key == "" {
			continue
		}
		result[key] = item
	}
	return result
}

func readonlyPipelineParamCodeMap(items []model.PipelineParam) map[string]model.PipelineParam {
	result := make(map[string]model.PipelineParam, len(items))
	for _, item := range items {
		code := strings.TrimSpace(item.Code)
		if code == "" {
			continue
		}
		if _, exists := result[code]; exists {
			continue
		}
		result[code] = item
	}
	return result
}

func readonlyPipelineParamKey(item model.PipelineParam) string {
	code := strings.TrimSpace(item.Code)
	if code == "" {
		return ""
	}
	sourceCode := pipelineParamSourceCode(item)
	stepNodeID := strings.TrimSpace(item.StepNodeID)
	if stepNodeID == "" {
		return "\x00" + sourceCode + "\x00" + code
	}
	return stepNodeID + "\x00" + sourceCode
}

func readonlyPipelineParamValue(item model.PipelineParam) string {
	if item.CurrentValue != "" {
		return item.CurrentValue
	}
	return item.DefaultValue
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
	switch strings.TrimSpace(credentialType) {
	case "username_password", "ssh_key":
		return "username"
	case "token":
		return "token"
	case "kubeconfig":
		return "kubeconfig"
	case "secret_text":
		return "secretText"
	case "certificate":
		return "certificate"
	default:
		return ""
	}
}

func isJenkinsCredentialIDSupportedType(credentialType string) bool {
	switch strings.TrimSpace(credentialType) {
	case "username_password", "token", "secret_text", "ssh_key", "kubeconfig":
		return true
	default:
		return false
	}
}

func buildRuntime(ctx context.Context, svcCtx *svc.ServiceContext, projectID, systemID, envID, bindingID string, userID uint64, roles []string) (*pipelineconfigservice.ResolvePipelineRuntimeResp, error) {
	resp, err := svcCtx.PipelineConfigRpc.ResolvePipelineRuntime(ctx, &pipelineconfigservice.ResolvePipelineRuntimeReq{
		ProjectId:             projectID,
		SystemId:              systemID,
		EnvironmentId:         envID,
		BuildChannelBindingId: bindingID,
		CurrentUserId:         userID,
		CurrentRoles:          roles,
	})
	if err != nil {
		logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
		return nil, err
	}
	if !resp.CredentialReady {
		if resp.CredentialMessage != "" {
			logx.Errorf("%s", resp.CredentialMessage)
			return nil, errorx.Msg(resp.CredentialMessage)
		}
		logx.Errorf("构建渠道凭据不可用")
		return nil, errorx.Msg("构建渠道凭据不可用")
	}
	return resp, nil
}

func validateJenkinsRuntime(runtime *pipelineconfigservice.ResolvePipelineRuntimeResp) error {
	if runtime == nil || runtime.Binding == nil || runtime.Channel == nil {
		return errorx.Msg("Jenkins 运行时配置不完整")
	}
	if runtime.Binding.ChannelType != engineJenkins || runtime.Channel.ChannelType != engineJenkins {
		return errorx.Msg("Jenkins 流水线必须选择 Jenkins 构建渠道")
	}
	return nil
}

func buildRuntimeCached(ctx context.Context, svcCtx *svc.ServiceContext, projectID, systemID, envID, bindingID string, userID uint64, roles []string) (*pipelineconfigservice.ResolvePipelineRuntimeResp, error) {
	key := pipelineRuntimeCacheKey(projectID, systemID, envID, bindingID, userID, roles)
	if raw, ok := pipelineRuntimeCache.Load(key); ok {
		if item, ok := raw.(pipelineRuntimeCacheItem); ok && item.value != nil && time.Now().Before(item.expiresAt) {
			return item.value, nil
		}
		pipelineRuntimeCache.Delete(key)
	}
	resp, err := buildRuntime(ctx, svcCtx, projectID, systemID, envID, bindingID, userID, roles)
	if err != nil {
		return nil, err
	}
	pipelineRuntimeCache.Store(key, pipelineRuntimeCacheItem{
		value:     resp,
		expiresAt: time.Now().Add(pipelineRuntimeCacheTTL),
	})
	return resp, nil
}

func pipelineRuntimeCacheKey(projectID, systemID, envID, bindingID string, userID uint64, roles []string) string {
	roleCopy := append([]string(nil), roles...)
	sort.Strings(roleCopy)
	return strings.Join([]string{
		projectID,
		systemID,
		envID,
		bindingID,
		strconv.FormatUint(userID, 10),
		strings.Join(roleCopy, ","),
	}, "|")
}

func shouldSyncJenkinsRunStatus(runID string) bool {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return true
	}
	now := time.Now()
	if raw, ok := runJenkinsStatusSyncCache.Load(runID); ok {
		if last, ok := raw.(time.Time); ok && now.Sub(last) < runJenkinsStatusSyncInterval {
			return false
		}
	}
	runJenkinsStatusSyncCache.Store(runID, now)
	return true
}

func renderPipeline(data *model.DevopsPipeline) (string, error) {
	result, err := renderPipelineWithMetadata(context.Background(), nil, data, pipelineRenderContext{})
	if err != nil {
		logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
		return "", err
	}
	return result.Script, nil
}

func renderPipelineWithMetadata(ctx context.Context, svcCtx *svc.ServiceContext, data *model.DevopsPipeline, renderCtx pipelineRenderContext) (pipelineRenderResult, error) {
	env := map[string]string{
		"PROJECT_CODE":      data.ProjectCode,
		"SYSTEM_CODE":       data.SystemCode,
		"SYSTEM_NAME":       data.SystemName,
		"PIPELINE_CODE":     data.Code,
		"PIPELINE_NAME":     data.Name,
		"PIPELINE_ENV":      data.EnvironmentCode,
		"PIPELINE_ENV_NAME": data.EnvironmentName,
	}
	for _, item := range data.Params {
		if effectiveParamRuntimeMode(item) == "env" {
			if item.ParamType == objectListParamType {
				value, err := resolveObjectListValue(ctx, svcCtx, data, renderCtx, item)
				if err != nil {
					logx.Errorf("处理对象列表参数失败: %v", err)
					return pipelineRenderResult{}, err
				}
				if value != "" {
					env[item.Code] = value
				}
				continue
			}
			value := item.CurrentValue
			if value == "" {
				value = item.DefaultValue
			}
			value = normalizeRenderParamValue(item.ParamType, value)
			if value != "" {
				env[item.Code] = value
			}
		}
	}
	if svcCtx != nil {
		if err := resolvePlatformEnvValues(ctx, svcCtx, data, renderCtx, env); err != nil {
			logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
			return pipelineRenderResult{}, err
		}
	}
	mavenSettingsContent := ""
	if svcCtx != nil {
		content, err := resolveMavenSettingsContent(ctx, svcCtx, data, renderCtx)
		if err != nil {
			logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
			return pipelineRenderResult{}, err
		}
		mavenSettingsContent = content
		if strings.TrimSpace(mavenSettingsContent) != "" {
			env["MAVEN_SETTINGS_XML"] = mavenSettingsContent
		}
	}
	renderJenkinsParams, err := renderParamsWithResolvedValues(ctx, svcCtx, data, renderCtx)
	if err != nil {
		logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
		return pipelineRenderResult{}, err
	}
	stepParamCodeMaps := pipelineStepParamCodeMapsForRender(ctx, svcCtx, data.Steps, data.Params)
	renderedSteps := renderSteps(data.Steps, stepParamCodeMaps, isDynamicPipelineAgentType(data.Agent.Type))
	if strings.TrimSpace(mavenSettingsContent) != "" {
		renderedSteps = injectMavenSettingsStep(renderedSteps)
	}
	result, err := jenkins.RenderDeclarativePipelineWithMetadata(jenkins.RenderPipeline{
		Agent: jenkins.JenkinsRenderAgent{
			Type:        normalizePipelineAgentType(data.Agent.Type),
			Label:       data.Agent.Label,
			DockerImage: data.Agent.DockerImage,
			DockerArgs:  data.Agent.DockerArgs,
			Raw:         data.Agent.Raw,
			ID:          data.Agent.ID,
			Name:        data.Agent.Name,
			AgentType:   normalizePipelineAgentType(data.Agent.AgentType),
			MatchMode:   data.Agent.MatchMode,
			MatchValue:  data.Agent.MatchValue,
			Cloud:       data.Agent.Cloud,
			PodYaml:     data.Agent.PodYaml,
		},
		Tools:       renderTools(data.Tools),
		Options:     renderOptions(data.Options),
		Triggers:    renderTriggers(data.Triggers),
		Environment: env,
		Params:      renderJenkinsParams,
		Steps:       renderedSteps,
	})
	if err != nil {
		logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
		return pipelineRenderResult{}, err
	}
	return pipelineRenderResult{Script: result.Pipeline, StageNames: result.StageNames, Params: renderJenkinsParams}, nil
}

func renderTools(items []model.JenkinsTool) []jenkins.JenkinsRenderKV {
	result := make([]jenkins.JenkinsRenderKV, 0, len(items))
	for _, item := range items {
		result = append(result, jenkins.JenkinsRenderKV{Name: item.Name, Type: item.Type, Value: item.Version})
	}
	return result
}

func renderOptions(items []model.JenkinsOption) []jenkins.JenkinsRenderKV {
	result := make([]jenkins.JenkinsRenderKV, 0, len(items))
	for _, item := range items {
		result = append(result, jenkins.JenkinsRenderKV{Name: item.Name, Value: item.Value})
	}
	return result
}

func renderTriggers(items []model.JenkinsTrigger) []jenkins.JenkinsRenderTrigger {
	result := make([]jenkins.JenkinsRenderTrigger, 0, len(items))
	for _, item := range items {
		result = append(result, jenkins.JenkinsRenderTrigger{Type: item.Type, Spec: item.Spec, Enabled: item.Enabled})
	}
	return result
}

func renderParams(items []model.PipelineParam) []jenkins.JenkinsRenderParam {
	result := make([]jenkins.JenkinsRenderParam, 0, len(items))
	for _, item := range items {
		if item.ParamType == objectListParamType {
			continue
		}
		config := normalizePipelineParamConfig(item)
		options := make([]jenkins.JenkinsRenderParamOption, 0, len(config.Options))
		for _, option := range config.Options {
			options = append(options, jenkins.JenkinsRenderParamOption{Label: option.Label, Value: option.Value})
		}
		paramType := renderJenkinsParamType(item, config)
		result = append(result, jenkins.JenkinsRenderParam{
			Code:         item.Code,
			DefaultValue: item.DefaultValue,
			CurrentValue: item.CurrentValue,
			ParamType:    paramType,
			Mode:         item.Mode,
			RuntimeMode:  effectiveParamRuntimeMode(item),
			Required:     item.Required,
			Readonly:     item.Readonly,
			Description:  item.Description,
			SelectList:   options,
		})
	}
	return result
}

func renderJenkinsParamType(item model.PipelineParam, config model.StepParamConfig) string {
	paramType := strings.TrimSpace(item.ParamType)
	if item.Mode == "paas" && isDeployConfigPipelineParam(item, config) && effectiveParamRuntimeMode(item) == "params" {
		return "string"
	}
	if item.Mode == "paas" && paramType == channelvars.ParamConfigCenter && effectiveParamRuntimeMode(item) == "params" {
		if normalizeConfigCenterValueMode(config.ValueMode) == "text" {
			return "text"
		}
		return "string"
	}
	return paramType
}

func renderParamsWithResolvedValues(ctx context.Context, svcCtx *svc.ServiceContext, data *model.DevopsPipeline, renderCtx pipelineRenderContext) ([]jenkins.JenkinsRenderParam, error) {
	result := renderParams(data.Params)
	if svcCtx == nil {
		return result, nil
	}
	paramByCode := make(map[string]model.PipelineParam, len(data.Params))
	for _, item := range data.Params {
		paramByCode[item.Code] = item
	}
	for index := range result {
		item, ok := paramByCode[result[index].Code]
		if !ok || item.Mode != "paas" || effectiveParamRuntimeMode(item) != "params" {
			continue
		}
		value, err := resolvePlatformParamValue(ctx, svcCtx, data, item, renderCtx)
		if err != nil {
			logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
			return nil, err
		}
		if value == "" {
			continue
		}
		result[index].CurrentValue = value
		result[index].DefaultValue = value
	}
	return result, nil
}

func normalizeRenderParamValue(paramType, value string) string {
	value = strings.TrimSpace(value)
	switch paramType {
	case "booleanParam":
		if isBoolLiteral(value) && strings.EqualFold(value, "true") {
			return "true"
		}
		if value == "" {
			return ""
		}
		return "false"
	case "number":
		return value
	default:
		return value
	}
}

func resolvePlatformParamValue(ctx context.Context, svcCtx *svc.ServiceContext, data *model.DevopsPipeline, item model.PipelineParam, renderCtx pipelineRenderContext) (string, error) {
	if item.ParamType == channelvars.ParamChannelCredential {
		return resolveChannelCredentialParamValue(ctx, svcCtx, data, item, renderCtx)
	}
	value := renderParamValue(item, renderCtx.Params)
	if value == "" {
		return "", nil
	}
	if item.ParamType == channelvars.ParamKubeNovaDeployConfig {
		return resolveKubeNovaDeployConfigParamValue(ctx, svcCtx, data, item, renderCtx, value)
	}
	if item.ParamType == channelvars.ParamKubernetesDeployConfig {
		return resolveKubernetesDeployConfigParamValue(ctx, svcCtx, data, item, renderCtx, value)
	}
	if item.ParamType == channelvars.ParamChannelVariable {
		config := normalizePipelineParamConfig(item)
		if isKubeNovaDeployConfigPipelineParam(item, config) {
			return resolveKubeNovaDeployConfigParamValue(ctx, svcCtx, data, item, renderCtx, value)
		}
		if isKubernetesDeployConfigPipelineParam(item, config) {
			return resolveKubernetesDeployConfigParamValue(ctx, svcCtx, data, item, renderCtx, value)
		}
		if channelDynamicParamType(item) != "" || strings.HasPrefix(strings.TrimSpace(config.MappingField), "dynamic.") {
			return value, nil
		}
		if !isLikelyObjectID(value) {
			return value, nil
		}
		resolved, err := resolvePipelineChannelBinding(ctx, svcCtx, data.ProjectID, value, renderCtx)
		if err != nil {
			logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
			return "", err
		}
		return resolveChannelMappingValue(resolved.GetChannel(), config.MappingField), nil
	}
	if channelvars.IsChannelParamType(item.ParamType) {
		if !isLikelyObjectID(value) {
			return value, nil
		}
		endpoint, err := resolvePipelineChannelEndpoint(ctx, svcCtx, data.ProjectID, value, renderCtx)
		if err != nil {
			logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
			return "", err
		}
		return endpoint, nil
	}
	if item.ParamType == channelvars.ParamMavenConfig {
		resp, err := svcCtx.PipelineConfigRpc.ResolveMavenConfig(ctx, &pipelineconfigservice.ResolveMavenConfigReq{
			ProjectId:     data.ProjectID,
			MavenConfigId: value,
			CurrentUserId: renderCtx.UserID,
			CurrentRoles:  renderCtx.Roles,
		})
		if err != nil {
			logx.Errorf("解析 Maven 配置失败，流水线: %s, 参数: %s(%s), 错误: %v",
				data.Name, item.Name, item.Code, err)
			return "", err
		}
		return resp.GetContent(), nil
	}
	if item.ParamType == channelvars.ParamConfigCenter {
		resp, err := resolveProjectConfigParam(ctx, svcCtx, data, item, value, renderCtx)
		if err != nil {
			logx.Errorf("解析配置中心失败，流水线: %s, 参数: %s(%s), 错误: %v",
				data.Name, item.Name, item.Code, err)
			return "", err
		}
		content := resp.GetContent()
		switch normalizeConfigCenterValueMode(normalizePipelineParamConfig(item).ValueMode) {
		case "base64":
			return base64.StdEncoding.EncodeToString([]byte(content)), nil
		case "credentialsSecretFile":
			credentialID, err := ensureConfigCenterSecretFileCredential(ctx, svcCtx, data, resp, renderCtx)
			if err != nil {
				logx.Errorf("创建配置中心 Jenkins Secret file 失败，流水线: %s, 参数: %s(%s), 错误: %v",
					data.Name, item.Name, item.Code, err)
				return "", err
			}
			return credentialID, nil
		default:
			return content, nil
		}
	}
	if item.ParamType == channelvars.ParamSonarAddress {
		return value, nil
	}
	if item.ParamType == channelvars.ParamHostGroupTargets {
		resp, err := svcCtx.PipelineConfigRpc.ResolveHostGroupTargets(ctx, &pipelineconfigservice.ResolveHostGroupTargetsReq{
			ProjectId:          data.ProjectID,
			HostGroupBindingId: value,
			RenderMode:         normalizePipelineParamConfig(item).RenderMode,
			CurrentUserId:      renderCtx.UserID,
			CurrentRoles:       renderCtx.Roles,
		})
		if err != nil {
			logx.Errorf("解析主机组失败，流水线: %s, 参数: %s(%s), 错误: %v",
				data.Name, item.Name, item.Code, err)
			return "", err
		}
		return resp.GetValue(), nil
	}
	if item.ParamType == "voucherModel" {
		config := normalizePipelineParamConfig(item)
		if !isLikelyObjectID(value) {
			logx.Errorf("解析流水线凭证参数失败，流水线: %s, 参数: %s(%s), 凭证类型: %s, 原因: 参数值不是凭证 ID",
				data.Name, item.Name, item.Code, config.VoucherModel)
			return "", errorx.Msg("凭证参数必须选择平台凭证：" + displayPipelineParamName(item))
		}
		resp, err := svcCtx.PipelineConfigRpc.ResolveCredentialValue(ctx, &pipelineconfigservice.ResolveCredentialValueReq{
			ProjectId:             data.ProjectID,
			CredentialId:          value,
			CredentialType:        config.VoucherModel,
			MappingField:          config.MappingField,
			CredentialMode:        config.CredentialMode,
			CurrentUserId:         renderCtx.UserID,
			CurrentRoles:          renderCtx.Roles,
			BuildChannelBindingId: data.BuildChannelBindingID,
		})
		if err != nil {
			logx.Errorf("解析流水线凭证参数失败，流水线: %s, 参数: %s(%s), 凭证类型: %s, 错误: %v",
				data.Name, item.Name, item.Code, config.VoucherModel, err)
			return "", err
		}
		return strings.TrimSpace(resp.GetValue()), nil
	}
	return value, nil
}

func resolveKubeNovaDeployConfigParamValue(ctx context.Context, svcCtx *svc.ServiceContext, data *model.DevopsPipeline, item model.PipelineParam, renderCtx pipelineRenderContext, value string) (string, error) {
	if svcCtx == nil {
		return formatDeployConfigValue(normalizePipelineParamConfig(item).ValueMode, value), nil
	}
	var config map[string]any
	if err := json.Unmarshal([]byte(value), &config); err != nil {
		logx.Errorf("解析 kube-nova 部署配置失败，流水线: %s, 参数: %s(%s), 错误: %v", data.Name, item.Name, item.Code, err)
		return "", errorx.Msg("kube-nova 部署配置不是合法 JSON")
	}
	channelBindingID := stringJSONValue(config, "channelBindingId")
	if channelBindingID == "" {
		channelBindingID = stringJSONValue(config, "bindingId")
	}
	if channelBindingID == "" {
		logx.Errorf("kube-nova 部署配置缺少渠道实例，流水线: %s, 参数: %s(%s)", data.Name, item.Name, item.Code)
		return "", errorx.Msg("kube-nova 部署配置缺少渠道实例")
	}
	token, err := resolveKubeNovaToken(ctx, svcCtx, data, channelBindingID, renderCtx)
	if err != nil {
		logx.Errorf("解析 kube-nova 部署凭证失败，流水线: %s, 参数: %s(%s), 错误: %v", data.Name, item.Name, item.Code, err)
		return "", err
	}
	if strings.TrimSpace(token) == "" {
		logx.Errorf("解析 kube-nova 部署凭证失败，token 为空，流水线: %s, 参数: %s(%s)", data.Name, item.Name, item.Code)
		return "", errorx.Msg("kube-nova 部署凭证 token 为空")
	}
	token = authutil.NormalizeBearerToken("kube-nova", token)
	config["tokenBase64"] = base64.StdEncoding.EncodeToString([]byte(token))
	dataBytes, err := json.Marshal(config)
	if err != nil {
		logx.Errorf("生成 kube-nova 部署配置失败，流水线: %s, 参数: %s(%s), 错误: %v", data.Name, item.Name, item.Code, err)
		return "", err
	}
	return formatDeployConfigValue(normalizePipelineParamConfig(item).ValueMode, string(dataBytes)), nil
}

func isDeployConfigPipelineParam(item model.PipelineParam, config model.StepParamConfig) bool {
	return isKubeNovaDeployConfigPipelineParam(item, config) || isKubernetesDeployConfigPipelineParam(item, config)
}

func isKubeNovaDeployConfigPipelineParam(item model.PipelineParam, config model.StepParamConfig) bool {
	if strings.TrimSpace(item.ParamType) == channelvars.ParamKubeNovaDeployConfig {
		return true
	}
	return isChannelVariableDeployConfigParam(item, config, "kube-nova", channelvars.FieldAddressDeployConfig)
}

func isKubernetesDeployConfigPipelineParam(item model.PipelineParam, config model.StepParamConfig) bool {
	if strings.TrimSpace(item.ParamType) == channelvars.ParamKubernetesDeployConfig {
		return true
	}
	return isChannelVariableDeployConfigParam(item, config, "kubernetes", channelvars.FieldAddressDeploymentConfig)
}

func isChannelVariableDeployConfigParam(item model.PipelineParam, config model.StepParamConfig, channelType, mappingField string) bool {
	if strings.TrimSpace(item.ParamType) != channelvars.ParamChannelVariable {
		return false
	}
	if strings.TrimSpace(config.ChannelTypeFilter) != channelType {
		return false
	}
	field := strings.TrimSpace(config.MappingField)
	if field == mappingField {
		return true
	}
	return field == channelvars.FieldDynamicDeployConfig &&
		(channelType == "kube-nova" || channelType == "kubernetes")
}

func resolveKubeNovaToken(ctx context.Context, svcCtx *svc.ServiceContext, data *model.DevopsPipeline, channelBindingID string, renderCtx pipelineRenderContext) (string, error) {
	resp, err := svcCtx.PipelineConfigRpc.ResolveChannelCredentialValue(ctx, &pipelineconfigservice.ResolveChannelCredentialValueReq{
		ProjectId:              data.ProjectID,
		SourceParamType:        channelvars.ParamKubeNovaDeployConfig,
		SourceValue:            channelBindingID,
		SourceChannelBindingId: channelBindingID,
		MappingField:           credentialModeJSONData,
		CredentialMode:         credentialModeJSONData,
		BuildChannelBindingId:  data.BuildChannelBindingID,
		CurrentUserId:          renderCtx.UserID,
		CurrentRoles:           renderCtx.Roles,
	})
	if err != nil {
		return "", err
	}
	token := kubeNovaTokenFromCredentialJSON(resp.GetValue())
	if token == "" {
		logx.Errorf("kube-nova 部署凭证缺少 token 字段，流水线: %s, channelBindingId: %s", data.Name, channelBindingID)
		return "", errorx.Msg("kube-nova 部署凭证必须是 Token 或 Secret Text")
	}
	return token, nil
}

func kubeNovaTokenFromCredentialJSON(value string) string {
	var data map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(value)), &data); err != nil {
		return ""
	}
	for _, key := range []string{"token", "secretText", "secret_text", "accessToken", "access_token"} {
		if token := stringJSONValue(data, key); token != "" {
			return token
		}
	}
	return ""
}

func resolveKubernetesDeployConfigParamValue(ctx context.Context, svcCtx *svc.ServiceContext, data *model.DevopsPipeline, item model.PipelineParam, renderCtx pipelineRenderContext, value string) (string, error) {
	var config map[string]any
	if err := json.Unmarshal([]byte(value), &config); err != nil {
		logx.Errorf("解析 Kubernetes 部署配置失败，流水线: %s, 参数: %s(%s), 错误: %v", data.Name, item.Name, item.Code, err)
		return "", errorx.Msg("Kubernetes 部署配置不是合法 JSON")
	}
	channelBindingID := stringJSONValue(config, "channelBindingId")
	if channelBindingID == "" {
		channelBindingID = stringJSONValue(config, "bindingId")
	}
	if channelBindingID == "" {
		logx.Errorf("Kubernetes 部署配置缺少渠道实例，流水线: %s, 参数: %s(%s)", data.Name, item.Name, item.Code)
		return "", errorx.Msg("Kubernetes 部署配置缺少渠道实例")
	}
	resp, err := svcCtx.PipelineConfigRpc.ResolveChannelCredentialValue(ctx, &pipelineconfigservice.ResolveChannelCredentialValueReq{
		ProjectId:              data.ProjectID,
		SourceParamType:        channelvars.ParamKubernetesDeployConfig,
		SourceValue:            channelBindingID,
		SourceChannelBindingId: channelBindingID,
		MappingField:           "credentialId",
		CredentialMode:         credentialModeJenkinsID,
		BuildChannelBindingId:  data.BuildChannelBindingID,
		CurrentUserId:          renderCtx.UserID,
		CurrentRoles:           renderCtx.Roles,
	})
	if err != nil {
		logx.Errorf("解析 Kubernetes 部署凭证失败，流水线: %s, 参数: %s(%s), 错误: %v", data.Name, item.Name, item.Code, err)
		return "", err
	}
	jenkinsCredentialID := strings.TrimSpace(resp.GetJenkinsCredentialId())
	if jenkinsCredentialID == "" {
		jenkinsCredentialID = strings.TrimSpace(resp.GetValue())
	}
	if jenkinsCredentialID == "" {
		logx.Errorf("解析 Kubernetes 部署凭证失败，Jenkins credentialId 为空，流水线: %s, 参数: %s(%s)", data.Name, item.Name, item.Code)
		return "", errorx.Msg("Kubernetes 部署凭证未生成 Jenkins credentialId")
	}
	config["kubeCredentialsId"] = jenkinsCredentialID
	if strings.TrimSpace(stringJSONValue(config, "credentialContentType")) == "" {
		config["credentialContentType"] = "auto"
	}
	dataBytes, err := json.Marshal(config)
	if err != nil {
		logx.Errorf("生成 Kubernetes 部署配置失败，流水线: %s, 参数: %s(%s), 错误: %v", data.Name, item.Name, item.Code, err)
		return "", err
	}
	return formatDeployConfigValue(normalizePipelineParamConfig(item).ValueMode, string(dataBytes)), nil
}

func formatDeployConfigValue(valueMode, value string) string {
	switch strings.TrimSpace(valueMode) {
	case "jsonBase64":
		return base64.StdEncoding.EncodeToString([]byte(value))
	case "json", "string":
		return value
	default:
		return value
	}
}

func stringJSONValue(data map[string]any, key string) string {
	if data == nil {
		return ""
	}
	value, ok := data[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func resolveChannelCredentialParamValue(ctx context.Context, svcCtx *svc.ServiceContext, data *model.DevopsPipeline, item model.PipelineParam, renderCtx pipelineRenderContext) (string, error) {
	config := normalizePipelineParamConfig(item)
	sourceParam, ok := pipelineParamByCode(data.Params, config.CredentialSourceParamCode)
	if !ok {
		if !item.Required {
			return "", nil
		}
		logx.Errorf("渠道凭证参数缺少渠道来源，流水线: %s, 参数: %s(%s), 来源编码: %s",
			data.Name, item.Name, item.Code, config.CredentialSourceParamCode)
		return "", errorx.Msg("渠道凭证参数缺少渠道来源：" + displayPipelineParamName(item))
	}
	sourceValue := renderParamValue(sourceParam, renderCtx.Params)
	sourceChannelBindingID := resolveCredentialSourceBindingID(data.Params, sourceParam, renderCtx)
	if strings.TrimSpace(sourceValue) == "" && strings.TrimSpace(sourceChannelBindingID) == "" {
		if !item.Required {
			return "", nil
		}
		logx.Errorf("渠道凭证来源为空，流水线: %s, 参数: %s(%s), 来源参数: %s",
			data.Name, item.Name, item.Code, sourceParam.Code)
		return "", errorx.Msg("渠道凭证参数缺少渠道来源：" + displayPipelineParamName(item))
	}
	resp, err := svcCtx.PipelineConfigRpc.ResolveChannelCredentialValue(ctx, &pipelineconfigservice.ResolveChannelCredentialValueReq{
		ProjectId:              data.ProjectID,
		SourceParamType:        credentialSourceParamType(sourceParam),
		SourceValue:            sourceValue,
		SourceChannelBindingId: sourceChannelBindingID,
		MappingField:           credentialSourceMappingField(sourceParam, config.MappingField),
		CredentialMode:         config.CredentialMode,
		BuildChannelBindingId:  data.BuildChannelBindingID,
		CurrentUserId:          renderCtx.UserID,
		CurrentRoles:           renderCtx.Roles,
	})
	if err != nil {
		logx.Errorf("解析渠道凭证参数失败，流水线: %s, 参数: %s(%s), 来源参数: %s, 错误: %v",
			data.Name, item.Name, item.Code, sourceParam.Code, err)
		return "", err
	}
	return strings.TrimSpace(resp.GetValue()), nil
}

func credentialSourceMappingField(sourceParam model.PipelineParam, credentialMappingField string) string {
	sourceConfig := normalizePipelineParamConfig(sourceParam)
	if strings.HasPrefix(strings.TrimSpace(sourceConfig.MappingField), "address.") {
		return strings.TrimSpace(sourceConfig.MappingField)
	}
	return strings.TrimSpace(credentialMappingField)
}

func credentialSourceParamType(sourceParam model.PipelineParam) string {
	if strings.TrimSpace(sourceParam.ParamType) != channelvars.ParamChannelVariable {
		return strings.TrimSpace(sourceParam.ParamType)
	}
	if dynamicType := pipelineDynamicParamType(sourceParam); dynamicType != "" {
		return dynamicType
	}
	return channelvars.ParamChannelVariable
}

func resolveProjectConfigParam(ctx context.Context, svcCtx *svc.ServiceContext, data *model.DevopsPipeline, item model.PipelineParam, configID string, renderCtx pipelineRenderContext) (*pipelineconfigservice.ResolveProjectConfigResp, error) {
	config := normalizePipelineParamConfig(item)
	return svcCtx.PipelineConfigRpc.ResolveProjectConfig(ctx, &pipelineconfigservice.ResolveProjectConfigReq{
		ProjectId:     data.ProjectID,
		ConfigId:      configID,
		TypeId:        config.ConfigTypeID,
		TypeCode:      config.ConfigTypeCode,
		CurrentUserId: renderCtx.UserID,
		CurrentRoles:  renderCtx.Roles,
	})
}

func ensureConfigCenterSecretFileCredential(ctx context.Context, svcCtx *svc.ServiceContext, data *model.DevopsPipeline, config *pipelineconfigservice.ResolveProjectConfigResp, renderCtx pipelineRenderContext) (string, error) {
	if strings.TrimSpace(data.BuildChannelBindingID) == "" {
		logx.Errorf("配置中心 Secret file 模式必须选择 Jenkins 构建渠道")
		return "", errorx.Msg("配置中心 Secret file 模式必须选择 Jenkins 构建渠道")
	}
	runtime, err := buildRuntimeCached(ctx, svcCtx, data.ProjectID, data.SystemID, data.EnvironmentID, data.BuildChannelBindingID, renderCtx.UserID, renderCtx.Roles)
	if err != nil {
		logx.Errorf("创建配置中心 Secret file 凭证失败: %v", err)
		return "", err
	}
	manager := jenkinsManagerFromRuntime(runtime)
	if manager == nil {
		logx.Errorf("创建配置中心 Secret file 凭证失败: Jenkins 构建渠道不可用")
		return "", errorx.Msg("Jenkins 构建渠道不可用")
	}
	folder := jenkinsFolder(runtime.GetProject().GetName(), runtime.GetProject().GetCode())
	credentialID := stableConfigCenterCredentialID(data.ProjectCode, config.GetTypeCode(), config.GetCode())
	if err := manager.UpsertFolderCredential(ctx, folder, jenkins.CredentialConfig{
		ID:          credentialID,
		Description: config.GetName(),
		Type:        "secret_file",
		FileName:    configCenterSecretFileName(config),
		SecretText:  base64.StdEncoding.EncodeToString([]byte(config.GetContent())),
	}); err != nil {
		logx.Errorf("创建配置中心 Secret file 凭证失败: %v", err)
		return "", errorx.Msg("创建配置中心 Secret file 凭证失败")
	}
	return credentialID, nil
}

func stableConfigCenterCredentialID(projectCode, typeCode, configCode string) string {
	projectCode = sanitizeConfigCenterCredentialCode(projectCode)
	typeCode = sanitizeConfigCenterCredentialCode(typeCode)
	configCode = sanitizeConfigCenterCredentialCode(configCode)
	if projectCode == "" {
		projectCode = "project"
	}
	if typeCode == "" {
		typeCode = "config"
	}
	if configCode == "" {
		configCode = "item"
	}
	return "kn-" + projectCode + "-" + typeCode + "-" + configCode
}

func configCenterSecretFileName(config *pipelineconfigservice.ResolveProjectConfigResp) string {
	name := strings.TrimSpace(config.GetCode())
	if name == "" {
		name = strings.TrimSpace(config.GetName())
	}
	name = sanitizeConfigCenterCredentialCode(name)
	if name == "" {
		name = "config"
	}
	return name
}

func sanitizeConfigCenterCredentialCode(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = regexp.MustCompile(`[^A-Za-z0-9_-]+`).ReplaceAllString(value, "-")
	return strings.Trim(value, "-_")
}

func pipelineParamByCode(items []model.PipelineParam, code string) (model.PipelineParam, bool) {
	code = strings.TrimSpace(code)
	for _, item := range items {
		if item.Code == code {
			return item, true
		}
	}
	return model.PipelineParam{}, false
}

func resolveCredentialSourceBindingID(items []model.PipelineParam, sourceParam model.PipelineParam, renderCtx pipelineRenderContext) string {
	if channelvars.IsChannelParamType(sourceParam.ParamType) {
		config := normalizePipelineParamConfig(sourceParam)
		if strings.TrimSpace(sourceParam.ParamType) != channelvars.ParamChannelVariable {
			return bindingIDParamValue(sourceParam, renderCtx)
		}
		if channelDynamicParamType(sourceParam) == "" && !strings.HasPrefix(strings.TrimSpace(config.MappingField), "dynamic.") {
			return bindingIDParamValue(sourceParam, renderCtx)
		}
	}
	config := normalizePipelineParamConfig(sourceParam)
	if strings.TrimSpace(config.ChannelBindingID) != "" {
		return strings.TrimSpace(config.ChannelBindingID)
	}
	if strings.TrimSpace(config.ChannelParamCode) == "" {
		return ""
	}
	channelParam, ok := pipelineParamByCode(items, config.ChannelParamCode)
	if !ok {
		return ""
	}
	return bindingIDParamValue(channelParam, renderCtx)
}

func bindingIDParamValue(param model.PipelineParam, renderCtx pipelineRenderContext) string {
	value := renderParamValue(param, renderCtx.Params)
	if looksLikeManualChannelSourceValue(value) {
		return ""
	}
	return strings.TrimSpace(value)
}

func looksLikeManualChannelSourceValue(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	if strings.Contains(value, "://") || strings.Contains(value, "@") {
		return true
	}
	if strings.Contains(value, "/") {
		return true
	}
	if strings.Contains(value, ".") {
		return true
	}
	return false
}

func resolvePlatformEnvValues(ctx context.Context, svcCtx *svc.ServiceContext, data *model.DevopsPipeline, renderCtx pipelineRenderContext, env map[string]string) error {
	for _, item := range data.Params {
		if item.Mode != "paas" || effectiveParamRuntimeMode(item) != "env" {
			continue
		}
		if item.ParamType == objectListParamType {
			if err := resolveObjectListPlatformEnvValues(ctx, svcCtx, data, renderCtx, item, env); err != nil {
				logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
				return err
			}
			continue
		}
		value, err := resolvePlatformParamValue(ctx, svcCtx, data, item, renderCtx)
		if err != nil {
			logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
			return err
		}
		if value != "" {
			env[item.Code] = value
		}
	}
	return nil
}

func resolveMavenSettingsContent(ctx context.Context, svcCtx *svc.ServiceContext, data *model.DevopsPipeline, renderCtx pipelineRenderContext) (string, error) {
	for _, item := range data.Params {
		if item.Mode != "paas" {
			continue
		}
		config := normalizePipelineParamConfig(item)
		isMavenSettings := item.ParamType == channelvars.ParamMavenConfig ||
			(item.ParamType == channelvars.ParamConfigCenter && (config.ConfigTypeCode == "MavenSettings" || config.ConfigTypeID != ""))
		if !isMavenSettings {
			continue
		}
		value := renderParamValue(item, renderCtx.Params)
		if strings.TrimSpace(value) == "" {
			continue
		}
		var content string
		var err error
		if item.ParamType == channelvars.ParamConfigCenter {
			resp, resolveErr := resolveProjectConfigParam(ctx, svcCtx, data, item, value, renderCtx)
			if resolveErr == nil && resp.GetTypeCode() != "MavenSettings" {
				continue
			}
			if resolveErr == nil {
				content = resp.GetContent()
			}
			err = resolveErr
		} else {
			resp, resolveErr := svcCtx.PipelineConfigRpc.ResolveMavenConfig(ctx, &pipelineconfigservice.ResolveMavenConfigReq{
				ProjectId:     data.ProjectID,
				MavenConfigId: value,
				CurrentUserId: renderCtx.UserID,
				CurrentRoles:  renderCtx.Roles,
			})
			if resolveErr == nil {
				content = resp.GetContent()
			}
			err = resolveErr
		}
		if err != nil {
			logx.Errorf("解析 Maven 配置失败，流水线: %s, 参数: %s(%s), 错误: %v",
				data.Name, item.Name, item.Code, err)
			return "", err
		}
		return content, nil
	}
	return "", nil
}

func resolveObjectListPlatformEnvValues(ctx context.Context, svcCtx *svc.ServiceContext, data *model.DevopsPipeline, renderCtx pipelineRenderContext, item model.PipelineParam, env map[string]string) error {
	value, err := resolveObjectListValue(ctx, svcCtx, data, renderCtx, item)
	if err != nil {
		logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
		return err
	}
	if value != "" {
		env[item.Code] = value
	}
	return nil
}

func resolveObjectListValue(ctx context.Context, svcCtx *svc.ServiceContext, data *model.DevopsPipeline, renderCtx pipelineRenderContext, item model.PipelineParam) (string, error) {
	raw := renderParamValue(item, renderCtx.Params)
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		logx.Errorf("对象列表参数必须是 JSON 数组")
		return "", errorx.Msg("对象列表参数必须是 JSON 数组")
	}
	if svcCtx == nil {
		payload, err := json.Marshal(rows)
		if err != nil {
			logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
			return "", err
		}
		return formatObjectListValue(item, string(payload)), nil
	}
	fields := normalizePipelineParamConfig(item).CompoundFields
	for index, row := range rows {
		if row == nil {
			continue
		}
		for _, field := range fields {
			if field.Mode != "paas" {
				continue
			}
			value := objectListFieldStringValue(row, field)
			if value == "" {
				continue
			}
			switch {
			case channelvars.IsChannelParamType(field.ParamType):
				endpoint, err := resolvePipelineChannelEndpoint(ctx, svcCtx, data.ProjectID, value, renderCtx)
				if err != nil {
					logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
					return "", err
				}
				row[field.Code] = endpoint
			case field.ParamType == "voucherModel":
				config := normalizeCompoundFieldConfig(field)
				if !isLikelyObjectID(value) {
					logx.Errorf("解析对象列表凭证字段失败，流水线: %s, 参数: %s(%s), 字段: %s(%s), 凭证类型: %s, 原因: 字段值不是凭证 ID",
						data.Name, item.Name, item.Code, field.Name, field.Code, config.VoucherModel)
					return "", errorx.Msg("对象列表凭证字段必须选择平台凭证：" + field.Name)
				}
				resp, err := svcCtx.PipelineConfigRpc.ResolveCredentialValue(ctx, &pipelineconfigservice.ResolveCredentialValueReq{
					ProjectId:             data.ProjectID,
					CredentialId:          value,
					CredentialType:        config.VoucherModel,
					MappingField:          config.MappingField,
					CredentialMode:        config.CredentialMode,
					CurrentUserId:         renderCtx.UserID,
					CurrentRoles:          renderCtx.Roles,
					BuildChannelBindingId: data.BuildChannelBindingID,
				})
				if err != nil {
					logx.Errorf("解析对象列表凭证字段失败，流水线: %s, 参数: %s(%s), 字段: %s(%s), 凭证类型: %s, 错误: %v",
						data.Name, item.Name, item.Code, field.Name, field.Code, config.VoucherModel, err)
					return "", err
				}
				row[field.Code] = strings.TrimSpace(resp.GetValue())
			}
		}
		rows[index] = row
	}
	payload, err := json.Marshal(rows)
	if err != nil {
		logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
		return "", err
	}
	return formatObjectListValue(item, string(payload)), nil
}

func formatObjectListValue(item model.PipelineParam, payload string) string {
	if normalizeObjectListValueMode(normalizePipelineParamConfig(item).ValueMode) == "base64" {
		return base64.StdEncoding.EncodeToString([]byte(payload))
	}
	return payload
}

func normalizeObjectListValueMode(valueMode string) string {
	if strings.TrimSpace(valueMode) == "base64" {
		return "base64"
	}
	return "string"
}

func resolvePipelineChannelBinding(ctx context.Context, svcCtx *svc.ServiceContext, projectID, bindingID string, renderCtx pipelineRenderContext) (*pipelineconfigservice.ResolveChannelBindingResp, error) {
	resp, err := svcCtx.PipelineConfigRpc.ResolveChannelBinding(ctx, &pipelineconfigservice.ResolveChannelBindingReq{
		ProjectId:        projectID,
		ChannelBindingId: bindingID,
		CurrentUserId:    renderCtx.UserID,
		CurrentRoles:     renderCtx.Roles,
	})
	if err != nil {
		logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
		return nil, err
	}
	if resp.GetChannel() == nil {
		logx.Errorf("渠道信息不完整")
		return nil, errorx.Msg("渠道信息不完整")
	}
	return resp, nil
}

func resolvePipelineChannelEndpoint(ctx context.Context, svcCtx *svc.ServiceContext, projectID, bindingID string, renderCtx pipelineRenderContext) (string, error) {
	if strings.TrimSpace(bindingID) == "" {
		return "", nil
	}
	resolved, err := resolvePipelineChannelBinding(ctx, svcCtx, projectID, bindingID, renderCtx)
	if err != nil {
		logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
		return "", err
	}
	return strings.TrimSpace(resolved.Channel.GetEndpoint()), nil
}

func resolveChannelMappingValue(channel *pipelineconfigservice.DevopsChannel, mappingField string) string {
	if channel == nil {
		return ""
	}
	switch strings.TrimSpace(mappingField) {
	case channelvars.FieldChannelID:
		return strings.TrimSpace(channel.GetId())
	case channelvars.FieldChannelName:
		return strings.TrimSpace(channel.GetName())
	case channelvars.FieldChannelCode:
		return strings.TrimSpace(channel.GetCode())
	case channelvars.FieldEndpoint:
		return strings.TrimSpace(channel.GetEndpoint())
	case channelvars.FieldChannelType:
		return strings.TrimSpace(channel.GetChannelType())
	case channelvars.FieldDescription:
		return strings.TrimSpace(channel.GetDescription())
	case channelvars.FieldGlobalCredentialID:
		return strings.TrimSpace(channel.GetGlobalCredentialId())
	case channelvars.FieldCredentialID:
		return strings.TrimSpace(channel.GetCredentialId())
	case channelvars.FieldConfig:
		return strings.TrimSpace(channel.GetConfig())
	case channelvars.FieldLabels:
		return strings.TrimSpace(channel.GetLabels())
	case channelvars.FieldHealthStatus:
		return strings.TrimSpace(channel.GetHealthStatus())
	case channelvars.FieldLastCheckAt:
		return strconv.FormatInt(channel.GetLastCheckAt(), 10)
	case channelvars.FieldLastCheckMessage:
		return strings.TrimSpace(channel.GetLastCheckMessage())
	case channelvars.FieldMetadata:
		return strings.TrimSpace(channel.GetMetadata())
	case channelvars.FieldMetadataVersion:
		return jsonStringPath(channel.GetMetadata(), "version")
	case channelvars.FieldStatus:
		return strconv.FormatInt(channel.GetStatus(), 10)
	case channelvars.FieldAuthType:
		return strings.TrimSpace(channel.GetAuthType())
	case channelvars.FieldInsecureSkipTLS:
		return strconv.FormatBool(channel.GetInsecureSkipTls())
	case channelvars.FieldIcon:
		return strings.TrimSpace(channel.GetIcon())
	case channelvars.FieldIconColor:
		return strings.TrimSpace(channel.GetIconColor())
	case channelvars.FieldCreatedBy:
		return strings.TrimSpace(channel.GetCreatedBy())
	case channelvars.FieldUpdatedBy:
		return strings.TrimSpace(channel.GetUpdatedBy())
	case channelvars.FieldCreatedAt:
		return strconv.FormatInt(channel.GetCreatedAt(), 10)
	case channelvars.FieldUpdatedAt:
		return strconv.FormatInt(channel.GetUpdatedAt(), 10)
	}
	if strings.HasPrefix(mappingField, "config.") {
		return jsonStringPath(channel.GetConfig(), strings.TrimPrefix(mappingField, "config."))
	}
	if strings.HasPrefix(mappingField, "metadata.") {
		return jsonStringPath(channel.GetMetadata(), strings.TrimPrefix(mappingField, "metadata."))
	}
	return ""
}

func jsonStringPath(raw, path string) string {
	raw = strings.TrimSpace(raw)
	path = strings.TrimSpace(path)
	if raw == "" || path == "" {
		return ""
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return ""
	}
	var current any = data
	for _, part := range strings.Split(path, ".") {
		node, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current, ok = node[part]
		if !ok {
			return ""
		}
	}
	return strings.TrimSpace(fmt.Sprint(current))
}

func renderParamValue(item model.PipelineParam, runValues map[string]string) string {
	if runValues != nil {
		if value, ok := runValues[item.Code]; ok {
			return strings.TrimSpace(value)
		}
	}
	value := strings.TrimSpace(item.CurrentValue)
	if value == "" {
		value = strings.TrimSpace(item.DefaultValue)
	}
	return value
}

func renderSteps(items []model.PipelineStep, codeMaps map[string]map[string]string, enableContainer bool) []jenkins.JenkinsRenderStep {
	result := make([]jenkins.JenkinsRenderStep, 0, len(items))
	postNodeIDs := pipelinePostNodeIDs(items)
	for _, item := range items {
		containerName := ""
		if enableContainer {
			containerName = item.ContainerName
		}
		parentID := item.ParentNodeID
		if _, ok := postNodeIDs[item.ID]; ok {
			parentID = workflowPostNodeID
		}
		result = append(result, jenkins.JenkinsRenderStep{
			ID:            item.ID,
			NodeName:      item.NodeName,
			ContainerName: containerName,
			StepType:      item.StepType,
			ParentNodeID:  parentID,
			BranchType:    item.BranchType,
			SortOrder:     item.SortOrder,
			Enabled:       item.Enabled,
			StageContent:  item.StageContent,
			ParamCodeMap:  codeMaps[item.ID],
			Artifact: jenkins.JenkinsRenderArtifact{
				Enabled:     item.ArtifactConfig.Enabled,
				Type:        item.ArtifactConfig.Type,
				Name:        item.ArtifactConfig.Name,
				Path:        item.ArtifactConfig.Path,
				Required:    item.ArtifactConfig.Required,
				ContentType: item.ArtifactConfig.ContentType,
			},
		})
	}
	return result
}

func injectMavenSettingsStep(items []jenkins.JenkinsRenderStep) []jenkins.JenkinsRenderStep {
	const mavenSettingsStepID = "workflow-maven-settings"
	result := make([]jenkins.JenkinsRenderStep, 0, len(items)+1)
	result = append(result, jenkins.JenkinsRenderStep{
		ID:           mavenSettingsStepID,
		NodeName:     "准备 Maven 配置",
		ParentNodeID: workflowStartNodeID,
		BranchType:   "next",
		SortOrder:    -1,
		Enabled:      true,
		StageContent: "steps {\n  script {\n    sh 'mkdir -p ./maven'\n    writeFile file: './maven/settings.xml', text: env.MAVEN_SETTINGS_XML\n  }\n}",
	})
	for _, item := range items {
		if item.ID == mavenSettingsStepID {
			continue
		}
		if item.ParentNodeID == workflowStartNodeID {
			item.ParentNodeID = mavenSettingsStepID
		}
		item.StageContent = appendMavenSettingsArg(item.StageContent)
		result = append(result, item)
	}
	return result
}

func appendMavenSettingsArg(content string) string {
	if strings.TrimSpace(content) == "" {
		return content
	}
	matches := mavenCommandPattern.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return content
	}
	var builder strings.Builder
	last := 0
	for _, match := range matches {
		commandEnd := match[5]
		argsStart := match[7]
		builder.WriteString(content[last:commandEnd])
		if !mavenCommandHasSettingsArg(content[argsStart:mavenCommandSegmentEnd(content, argsStart)]) {
			builder.WriteString(" -s ./maven/settings.xml")
		}
		last = commandEnd
	}
	builder.WriteString(content[last:])
	return builder.String()
}

func mavenCommandSegmentEnd(content string, start int) int {
	for i := start; i < len(content); i++ {
		switch content[i] {
		case '\n', ';', '&', '|':
			return i
		}
	}
	return len(content)
}

func mavenCommandHasSettingsArg(args string) bool {
	return mavenSettingsArgPattern.MatchString(args)
}

func pipelineStepParamCodeMaps(params []model.PipelineParam) map[string]map[string]string {
	result := make(map[string]map[string]string)
	for _, item := range params {
		stepNodeID := strings.TrimSpace(item.StepNodeID)
		sourceCode := pipelineParamSourceCode(item)
		code := strings.TrimSpace(item.Code)
		if stepNodeID == "" || sourceCode == "" || code == "" {
			continue
		}
		if _, ok := result[stepNodeID]; !ok {
			result[stepNodeID] = make(map[string]string)
		}
		result[stepNodeID][sourceCode] = code
	}
	return result
}

func pipelineStepParamCodeMapsForRender(ctx context.Context, svcCtx *svc.ServiceContext, steps []model.PipelineStep, params []model.PipelineParam) map[string]map[string]string {
	result := pipelineStepParamCodeMaps(params)
	if svcCtx == nil || pipelineParamsHaveRenderSources(params) {
		return result
	}
	usedBySource := make(map[string]map[string]struct{})
	for _, codeMap := range result {
		for sourceCode, code := range codeMap {
			if strings.TrimSpace(sourceCode) == "" || strings.TrimSpace(code) == "" {
				continue
			}
			if _, ok := usedBySource[sourceCode]; !ok {
				usedBySource[sourceCode] = make(map[string]struct{})
			}
			usedBySource[sourceCode][code] = struct{}{}
		}
	}
	stepParamsByID := make(map[string][]*pipelineconfigservice.DevopsStepParam)
	for _, step := range steps {
		stepID := strings.TrimSpace(step.StepID)
		if stepID == "" {
			continue
		}
		if _, ok := stepParamsByID[stepID]; ok {
			continue
		}
		resp, err := svcCtx.PipelineConfigRpc.JenkinsStepGet(ctx, &pipelineconfigservice.GetByIdReq{Id: stepID})
		if err != nil || resp == nil || resp.Data == nil {
			continue
		}
		stepParamsByID[stepID] = resp.Data.Params
	}
	sourceCodes := make(map[string]struct{})
	for _, items := range stepParamsByID {
		for _, param := range items {
			if param == nil || strings.TrimSpace(param.Code) == "" {
				continue
			}
			sourceCodes[strings.TrimSpace(param.Code)] = struct{}{}
		}
	}
	paramsBySource := pipelineParamsBySourceForRender(params, sourceCodes)
	for _, step := range steps {
		stepNodeID := strings.TrimSpace(step.ID)
		if stepNodeID == "" {
			continue
		}
		if _, ok := result[stepNodeID]; !ok {
			result[stepNodeID] = make(map[string]string)
		}
		for _, param := range stepParamsByID[strings.TrimSpace(step.StepID)] {
			if param == nil {
				continue
			}
			sourceCode := strings.TrimSpace(param.Code)
			if sourceCode == "" || strings.TrimSpace(result[stepNodeID][sourceCode]) != "" {
				continue
			}
			code := nextRenderParamCode(sourceCode, paramsBySource[sourceCode], usedBySource)
			if code == "" {
				continue
			}
			result[stepNodeID][sourceCode] = code
		}
	}
	return result
}

func pipelineParamsHaveRenderSources(params []model.PipelineParam) bool {
	if len(params) == 0 {
		return false
	}
	for _, item := range params {
		if strings.TrimSpace(item.Code) == "" {
			return false
		}
		if strings.TrimSpace(item.StepNodeID) == "" || strings.TrimSpace(item.SourceCode) == "" {
			return false
		}
	}
	return true
}

func pipelineParamsBySourceForRender(params []model.PipelineParam, sourceCodes map[string]struct{}) map[string][]string {
	result := make(map[string][]string)
	for _, item := range params {
		code := strings.TrimSpace(item.Code)
		if code == "" {
			continue
		}
		if sourceCode := strings.TrimSpace(item.SourceCode); sourceCode != "" {
			result[sourceCode] = append(result[sourceCode], code)
			continue
		}
		for sourceCode := range sourceCodes {
			if code == sourceCode || strings.HasPrefix(code, sourceCode+"_") {
				result[sourceCode] = append(result[sourceCode], code)
			}
		}
	}
	return result
}

func nextRenderParamCode(sourceCode string, candidates []string, usedBySource map[string]map[string]struct{}) string {
	if _, ok := usedBySource[sourceCode]; !ok {
		usedBySource[sourceCode] = make(map[string]struct{})
	}
	for _, code := range candidates {
		if strings.TrimSpace(code) == "" {
			continue
		}
		if _, ok := usedBySource[sourceCode][code]; ok {
			continue
		}
		usedBySource[sourceCode][code] = struct{}{}
		return code
	}
	return ""
}

func jenkinsManagerFromRuntime(runtime *pipelineconfigservice.ResolvePipelineRuntimeResp) *jenkins.Manager {
	credential := runtime.Credential
	username, password, token := "", "", ""
	if credential != nil {
		username = credential.Username
		password = credential.Password
		token = credential.Token
	}
	if username == "" && runtime.Channel != nil {
		username = runtime.Channel.Username
	}
	if password == "" && token == "" && runtime.Channel != nil {
		password = runtime.Channel.Password
		token = runtime.Channel.Token
	}
	return jenkins.NewManager(jenkins.ClientConfig{
		Endpoint:        runtime.Channel.Endpoint,
		Username:        username,
		Password:        password,
		Token:           token,
		InsecureSkipTLS: runtime.Channel.InsecureSkipTls,
	})
}

func syncPipelineCredentials(ctx context.Context, svcCtx *svc.ServiceContext, data *model.DevopsPipeline, credentialIDs []string, userID uint64, roles []string) error {
	if len(credentialIDs) == 0 {
		credentialIDs = collectPipelineCredentialIDs(data)
	}
	if len(credentialIDs) == 0 {
		return nil
	}
	_, err := svcCtx.PipelineConfigRpc.SyncJenkinsCredentials(ctx, &pipelineconfigservice.SyncJenkinsCredentialsReq{
		ProjectId:             data.ProjectID,
		BuildChannelBindingId: data.BuildChannelBindingID,
		CredentialIds:         credentialIDs,
		CurrentUserId:         userID,
		CurrentRoles:          roles,
	})
	if err != nil {
		logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
		return err
	}
	return nil
}

func collectPipelineCredentialIDs(data *model.DevopsPipeline) []string {
	return collectPipelineCredentialIDsWithParams(data, nil)
}

func collectPipelineCredentialIDsWithParams(data *model.DevopsPipeline, runParams map[string]string) []string {
	if data == nil {
		return nil
	}
	seen := make(map[string]struct{})
	result := make([]string, 0)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if !isLikelyObjectID(value) {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	for _, item := range data.Params {
		config := normalizePipelineParamConfig(item)
		if item.Mode == "paas" && item.ParamType == "voucherModel" && config.CredentialMode == credentialModeJenkinsID {
			add(renderParamValue(item, runParams))
		}
		if config.CredentialID != "" {
			add(config.CredentialID)
		}
		if item.ParamType == objectListParamType {
			collectObjectListCredentialIDs(item, runParams, add)
		}
	}
	return result
}

func isLikelyObjectID(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 24 {
		return false
	}
	for _, item := range value {
		if (item >= '0' && item <= '9') || (item >= 'a' && item <= 'f') || (item >= 'A' && item <= 'F') {
			continue
		}
		return false
	}
	return true
}

func isRepositoryChannelType(channelType string) bool {
	switch strings.TrimSpace(channelType) {
	case "gitlab", "github", "gitee", "svn":
		return true
	default:
		return false
	}
}

func collectObjectListCredentialIDs(item model.PipelineParam, runParams map[string]string, add func(string)) {
	raw := renderParamValue(item, runParams)
	if raw == "" {
		return
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		return
	}
	for _, field := range normalizePipelineParamConfig(item).CompoundFields {
		if field.Mode != "paas" || field.ParamType != "voucherModel" {
			continue
		}
		config := normalizeCompoundFieldConfig(field)
		if config.CredentialMode != credentialModeJenkinsID {
			continue
		}
		if config.CredentialID != "" {
			add(config.CredentialID)
		}
		for _, row := range rows {
			if row == nil {
				continue
			}
			add(objectListFieldStringValue(row, field))
		}
	}
}

func objectListFieldStringValue(row map[string]any, field model.CompoundParamField) string {
	if row != nil {
		if value, ok := row[field.Code]; ok && hasParamValue(value) {
			return strings.TrimSpace(fmt.Sprint(value))
		}
	}
	return strings.TrimSpace(field.DefaultValue)
}

func pipelineParamsMap(params []model.PipelineParam, raw string) (map[string]string, error) {
	result := make(map[string]string)
	allowed := make(map[string]model.PipelineParam)
	readonly := make(map[string]struct{})
	for _, item := range params {
		if effectiveParamRuntimeMode(item) != "params" {
			continue
		}
		value := item.CurrentValue
		if value == "" {
			value = item.DefaultValue
		}
		result[item.Code] = value
		if !item.RuntimeConfig || item.Readonly {
			readonly[item.Code] = struct{}{}
			continue
		}
		allowed[item.Code] = item
	}
	if strings.TrimSpace(raw) == "" {
		return result, nil
	}
	var extra map[string]any
	if err := json.Unmarshal([]byte(raw), &extra); err != nil {
		logx.Errorf("运行参数必须是 JSON 对象")
		return nil, errorx.Msg("运行参数必须是 JSON 对象")
	}
	for key, value := range extra {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, ok := readonly[key]; ok {
			continue
		}
		item, ok := allowed[key]
		if !ok {
			logx.Errorf("%s", "运行参数不支持："+key)
			return nil, errorx.Msg("运行参数不支持：" + key)
		}
		if item.ParamType == "booleanParam" {
			result[key] = normalizeBoolString(value)
			continue
		}
		if item.ParamType == "number" {
			normalized := strings.TrimSpace(fmt.Sprint(value))
			if _, err := strconv.ParseFloat(normalized, 64); err != nil {
				logx.Errorf("%s", "数字运行参数必须是合法数字："+item.Name)
				return nil, errorx.Msg("数字运行参数必须是合法数字：" + item.Name)
			}
			result[key] = normalized
			continue
		}
		result[key] = runtimeParamStringValue(value)
	}
	return result, nil
}

func runtimeParamStringValue(value any) string {
	switch data := value.(type) {
	case nil:
		return ""
	case string:
		return data
	default:
		if out, err := json.Marshal(data); err == nil {
			return string(out)
		}
		return fmt.Sprint(data)
	}
}

func pipelineStringMap(raw, fieldName string) (map[string]string, error) {
	result := make(map[string]string)
	if strings.TrimSpace(raw) == "" {
		return result, nil
	}
	var extra map[string]any
	if err := json.Unmarshal([]byte(raw), &extra); err != nil {
		logx.Errorf("%s必须是 JSON 对象", fieldName)
		return nil, errorx.Msg(fieldName + "必须是 JSON 对象")
	}
	for key, value := range extra {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		result[key] = strings.TrimSpace(fmt.Sprint(value))
	}
	return result, nil
}

func mergeRuntimeWorkspaceValues(params, workspaces map[string]string) map[string]string {
	result := make(map[string]string, len(params)+len(workspaces))
	for key, value := range params {
		result[key] = value
	}
	for key, value := range workspaces {
		if strings.TrimSpace(key) == "" {
			continue
		}
		result[key] = value
	}
	return result
}

func pipelineBuildParamsMap(ctx context.Context, svcCtx *svc.ServiceContext, pipeline *model.DevopsPipeline, params map[string]string, userID uint64, roles []string) (map[string]string, error) {
	result := make(map[string]string, len(params))
	for key, value := range params {
		result[key] = value
	}
	if pipeline == nil || svcCtx == nil {
		return result, nil
	}
	renderCtx := pipelineRenderContext{UserID: userID, Roles: roles, Params: params}
	for _, item := range pipeline.Params {
		if !shouldResolvePipelineBuildParam(pipeline, item) {
			continue
		}
		if item.ParamType == objectListParamType {
			value, err := resolveObjectListValue(ctx, svcCtx, pipeline, renderCtx, item)
			if err != nil {
				logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
				return nil, err
			}
			if value != "" {
				result[item.Code] = value
			}
			continue
		}
		value, err := resolvePlatformParamValue(ctx, svcCtx, pipeline, item, renderCtx)
		if err != nil {
			logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
			return nil, err
		}
		if value != "" {
			result[item.Code] = value
		}
	}
	return result, nil
}

func shouldResolvePipelineBuildParam(pipeline *model.DevopsPipeline, item model.PipelineParam) bool {
	if effectiveParamRuntimeMode(item) != "params" {
		return false
	}
	if item.ParamType == objectListParamType {
		return true
	}
	if item.Mode == "paas" {
		return true
	}
	if pipeline == nil || pipeline.EngineType != engineTekton {
		return false
	}
	switch strings.TrimSpace(item.ParamType) {
	case channelvars.ParamChannelVariable, channelvars.ParamConfigCenter, channelvars.ParamHostGroupTargets:
		return true
	default:
		return false
	}
}

func effectiveParamRuntimeMode(item model.PipelineParam) string {
	if item.RuntimeConfig && (item.Mode == "params" || item.Mode == "paas") {
		return "params"
	}
	return normalizeRuntimeMode(item.RuntimeMode, item.Mode)
}

func normalizeBoolString(value any) string {
	switch v := value.(type) {
	case bool:
		if v {
			return "true"
		}
		return "false"
	case string:
		if strings.EqualFold(strings.TrimSpace(v), "true") {
			return "true"
		}
		return "false"
	default:
		if strings.EqualFold(strings.TrimSpace(fmt.Sprint(value)), "true") {
			return "true"
		}
		return "false"
	}
}

func uniqueJobName(systemCode, pipelineCode, envCode string) string {
	return strings.Trim(strings.Join([]string{systemCode, pipelineCode, envCode}, "-"), "-")
}

func jenkinsFolder(projectName, projectCode string) string {
	folder := sanitizeJenkinsFolder(projectName)
	if folder == "" {
		folder = sanitizeJenkinsFolder(projectCode)
	}
	if folder == "" {
		return "default-project"
	}
	return folder
}

func sanitizeJenkinsFolder(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "/", "-")
	value = strings.ReplaceAll(value, "\\", "-")
	return value
}

func createRunStages(ctx context.Context, svcCtx *svc.ServiceContext, runID, pipelineID string, steps []model.PipelineStep, stageNames map[string]string) error {
	postNodeIDs := pipelinePostNodeIDs(steps)
	stageCount := make(map[string]int)
	stages := make([]*model.DevopsPipelineRunStage, 0, len(steps))
	for _, step := range steps {
		if !step.Enabled {
			continue
		}
		stageName := strings.TrimSpace(stageNames[step.ID])
		if stageName == "" {
			stageName = uniqueRunStageName(runStageNameForStep(step), stageCount)
		} else {
			stageCount[stageName]++
		}
		stageType := step.StepType
		if _, ok := postNodeIDs[step.ID]; ok {
			stageType = "post"
		}
		stages = append(stages, &model.DevopsPipelineRunStage{
			RunID:      runID,
			PipelineID: pipelineID,
			NodeID:     step.ID,
			StageName:  stageName,
			StageType:  stageType,
			Status:     "queued",
		})
	}
	if err := svcCtx.RunStageModel.InsertMany(ctx, stages); err != nil {
		logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
		return err
	}
	return nil
}

func runStageNameForStep(step model.PipelineStep) string {
	if name := strings.TrimSpace(step.NodeName); name != "" {
		return name
	}
	if match := runStageNamePattern.FindStringSubmatch(step.StageContent); len(match) >= 3 && strings.TrimSpace(match[2]) != "" {
		return strings.TrimSpace(match[2])
	}
	if name := strings.TrimSpace(step.StepName); name != "" {
		return name
	}
	if name := strings.TrimSpace(step.StepType); name != "" {
		return name
	}
	return "未命名步骤"
}

func uniqueRunStageName(name string, stageCount map[string]int) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "未命名步骤"
	}
	stageCount[name]++
	if stageCount[name] == 1 {
		return name
	}
	return fmt.Sprintf("%s-%d", name, stageCount[name])
}

func ensureRunBuildNumber(ctx context.Context, svcCtx *svc.ServiceContext, run *model.DevopsPipelineRun, manager *jenkins.Manager, operator string) (bool, error) {
	if run.JenkinsBuildNumber > 0 {
		return true, nil
	}
	if strings.TrimSpace(run.JenkinsQueueID) == "" {
		return false, nil
	}
	buildNumber, buildURL, ready, err := manager.QueueBuild(ctx, run.JenkinsQueueID)
	if err != nil {
		if strings.Contains(err.Error(), "queue item cancelled") {
			return false, markRunFinal(ctx, svcCtx, run, "aborted", "Jenkins 队列已取消", operator)
		}
		// Jenkins 队列记录短时间后可能被清理，不能因此阻断日志页。
		if strings.Contains(err.Error(), "404") {
			return ensureRunLastBuild(ctx, svcCtx, run, manager, operator)
		}
		logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
		return false, err
	}
	if !ready {
		if time.Since(run.StartedAt) > 10*time.Minute {
			return false, markRunFinal(ctx, svcCtx, run, "failed", "Jenkins 长时间未分配构建号，请检查构建队列和 Agent 状态", operator)
		}
		return false, nil
	}
	run.JenkinsBuildNumber = buildNumber
	if strings.TrimSpace(buildURL) != "" {
		run.JenkinsBuildURL = buildURL
	}
	run.Status = "running"
	if strings.TrimSpace(operator) != "" {
		run.UpdatedBy = operator
	}
	if err := svcCtx.PipelineRunModel.Update(ctx, run); err != nil {
		logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
		return false, err
	}
	_ = svcCtx.PipelineModel.UpdateRunStatus(ctx, run.PipelineID, "running", run.UpdatedBy)
	return true, nil
}

func markRunFinal(ctx context.Context, svcCtx *svc.ServiceContext, run *model.DevopsPipelineRun, status, message, operator string) error {
	run.Status = status
	if strings.TrimSpace(message) != "" {
		run.ErrorMessage = message
	}
	if run.FinishedAt.IsZero() {
		run.FinishedAt = time.Now()
	}
	if !run.StartedAt.IsZero() {
		run.DurationSeconds = int64(run.FinishedAt.Sub(run.StartedAt).Seconds())
	}
	if strings.TrimSpace(operator) != "" {
		run.UpdatedBy = operator
	}
	if err := svcCtx.PipelineRunModel.Update(ctx, run); err != nil {
		logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
		return err
	}
	_ = svcCtx.PipelineModel.UpdateRunStatus(ctx, run.PipelineID, status, run.UpdatedBy)
	_ = recordPipelineRunMetric(ctx, svcCtx, run)
	return nil
}

func ensureRunLastBuild(ctx context.Context, svcCtx *svc.ServiceContext, run *model.DevopsPipelineRun, manager *jenkins.Manager, operator string) (bool, error) {
	info, ok, err := manager.LastBuildInfo(ctx, run.JenkinsJobFullName)
	if err != nil || !ok || info == nil || info.Number <= 0 {
		logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
		return false, err
	}
	if !run.StartedAt.IsZero() && info.Timestamp > 0 {
		buildStartedAt := time.UnixMilli(info.Timestamp)
		if buildStartedAt.Before(run.StartedAt.Add(-2*time.Minute)) || buildStartedAt.After(time.Now().Add(2*time.Minute)) {
			return false, nil
		}
	}
	run.JenkinsBuildNumber = info.Number
	if strings.TrimSpace(info.Url) != "" {
		run.JenkinsBuildURL = info.Url
	}
	run.Status = "running"
	if strings.TrimSpace(operator) != "" {
		run.UpdatedBy = operator
	}
	if err := svcCtx.PipelineRunModel.Update(ctx, run); err != nil {
		logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
		return false, err
	}
	_ = svcCtx.PipelineModel.UpdateRunStatus(ctx, run.PipelineID, "running", run.UpdatedBy)
	return true, nil
}

func syncRunStatusFromJenkins(ctx context.Context, svcCtx *svc.ServiceContext, run *model.DevopsPipelineRun, manager *jenkins.Manager, operator string) error {
	if run.JenkinsBuildNumber <= 0 {
		return nil
	}
	info, err := manager.BuildInfo(ctx, run.JenkinsJobFullName, run.JenkinsBuildNumber)
	if err != nil {
		logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
		return err
	}
	status := "running"
	if !info.Building {
		status = normalizeJenkinsResult(info.Result)
		if status == "" {
			status = "finished"
		}
		if info.Timestamp > 0 && info.Duration > 0 {
			run.FinishedAt = time.UnixMilli(info.Timestamp + info.Duration)
			run.DurationSeconds = info.Duration / 1000
		} else if run.FinishedAt.IsZero() {
			run.FinishedAt = time.Now()
			run.DurationSeconds = int64(run.FinishedAt.Sub(run.StartedAt).Seconds())
		}
	} else if stages, stageErr := manager.StageStatus(ctx, run.JenkinsJobFullName, run.JenkinsBuildNumber); stageErr == nil && hasPausedJenkinsStage(stages) {
		status = "paused"
	}
	if run.Status == status && (info.Building || !run.FinishedAt.IsZero()) {
		if isFinalRunStatus(status) {
			_ = syncRunArtifacts(ctx, svcCtx, run, manager, operator)
			_ = recordPipelineRunMetric(ctx, svcCtx, run)
		}
		return nil
	}
	run.Status = status
	if strings.TrimSpace(operator) != "" {
		run.UpdatedBy = operator
	}
	if err := svcCtx.PipelineRunModel.Update(ctx, run); err != nil {
		logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
		return err
	}
	_ = svcCtx.PipelineModel.UpdateRunStatus(ctx, run.PipelineID, status, run.UpdatedBy)
	if isFinalRunStatus(status) {
		_ = syncRunArtifacts(ctx, svcCtx, run, manager, operator)
		_ = recordPipelineRunMetric(ctx, svcCtx, run)
	}
	return nil
}

func syncRunArtifacts(ctx context.Context, svcCtx *svc.ServiceContext, run *model.DevopsPipelineRun, manager *jenkins.Manager, operator string) error {
	if run == nil || run.JenkinsBuildNumber <= 0 || svcCtx.ArtifactModel == nil {
		return nil
	}
	pipeline, err := svcCtx.PipelineModel.FindOne(ctx, run.PipelineID)
	if err != nil {
		return ignoreNotFound(err)
	}
	if err := syncServiceScanReports(ctx, svcCtx, run, pipeline, operator); err != nil {
		logx.Errorf("同步服务型扫描报告失败: %v", err)
	}
	configs := artifactConfigsFromPipeline(pipeline)
	if len(configs) == 0 {
		if err := evaluateQualityGateForRun(ctx, svcCtx, run, pipeline, operator); err != nil {
			logx.Errorf("自动评估质量门禁失败: %v", err)
		}
		return nil
	}
	existing, _ := svcCtx.ArtifactModel.ListByRun(ctx, run.ID.Hex())
	existingKeys := make(map[string]struct{}, len(existing))
	for _, item := range existing {
		existingKeys[item.ObjectKey] = struct{}{}
	}
	artifacts, err := manager.BuildArtifacts(ctx, run.JenkinsJobFullName, run.JenkinsBuildNumber)
	if err != nil {
		logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
		return err
	}
	for _, cfg := range configs {
		matched := matchedJenkinsArtifacts(cfg.Config.Path, artifacts)
		if len(matched) == 0 {
			if cfg.Config.Required && !svcCtx.ArtifactModel.Exists(ctx, run.ID.Hex(), cfg.StepID, "", "missing") {
				_ = svcCtx.ArtifactModel.Insert(ctx, &model.DevopsPipelineRunArtifact{
					RunID:        run.ID.Hex(),
					StepID:       cfg.StepID,
					Name:         artifactName(cfg.Config),
					Type:         artifactType(cfg.Config),
					Status:       "missing",
					ErrorMessage: "未找到匹配的 Jenkins 产物：" + cfg.Config.Path,
					CreatedBy:    operator,
					UpdatedBy:    operator,
				})
			}
			continue
		}
		for _, item := range matched {
			objectKey := pipelineArtifactObjectKey(run, item.RelativePath)
			if _, ok := existingKeys[objectKey]; ok {
				continue
			}
			data, contentType, err := manager.DownloadArtifact(ctx, run.JenkinsJobFullName, run.JenkinsBuildNumber, item.RelativePath)
			if err != nil {
				if !svcCtx.ArtifactModel.Exists(ctx, run.ID.Hex(), cfg.StepID, objectKey, "failed") {
					_ = svcCtx.ArtifactModel.Insert(ctx, &model.DevopsPipelineRunArtifact{
						RunID:        run.ID.Hex(),
						StepID:       cfg.StepID,
						Name:         artifactName(cfg.Config),
						Type:         artifactType(cfg.Config),
						ObjectKey:    objectKey,
						Status:       "failed",
						ErrorMessage: err.Error(),
						CreatedBy:    operator,
						UpdatedBy:    operator,
					})
				}
				continue
			}
			if contentType == "" {
				contentType = cfg.Config.ContentType
			}
			upload, err := svcCtx.PipelineConfigRpc.UploadPipelineArtifact(ctx, &pipelineconfigservice.UploadPipelineArtifactReq{
				ObjectKey:   objectKey,
				Data:        data,
				ContentType: contentType,
			})
			if err != nil {
				if !svcCtx.ArtifactModel.Exists(ctx, run.ID.Hex(), cfg.StepID, objectKey, "failed") {
					_ = svcCtx.ArtifactModel.Insert(ctx, &model.DevopsPipelineRunArtifact{
						RunID:        run.ID.Hex(),
						StepID:       cfg.StepID,
						Name:         artifactName(cfg.Config),
						Type:         artifactType(cfg.Config),
						ObjectKey:    objectKey,
						Status:       "failed",
						ErrorMessage: err.Error(),
						CreatedBy:    operator,
						UpdatedBy:    operator,
					})
				}
				continue
			}
			existingKeys[objectKey] = struct{}{}
			_ = svcCtx.ArtifactModel.Insert(ctx, &model.DevopsPipelineRunArtifact{
				RunID:       run.ID.Hex(),
				StepID:      cfg.StepID,
				Name:        artifactName(cfg.Config),
				Type:        artifactType(cfg.Config),
				Bucket:      upload.Bucket,
				ObjectKey:   upload.ObjectKey,
				Size:        upload.Size,
				ContentType: contentType,
				Status:      "success",
				CreatedBy:   operator,
				UpdatedBy:   operator,
			})
			if strings.EqualFold(artifactType(cfg.Config), "report") {
				if err := syncQualityReportArtifact(ctx, svcCtx, run, pipeline, cfg, item, upload, data, contentType, operator); err != nil {
					logx.Errorf("同步 Jenkins 扫描报告产物失败: %v", err)
				}
			}
		}
	}
	if err := evaluateQualityGateForRun(ctx, svcCtx, run, pipeline, operator); err != nil {
		logx.Errorf("自动评估质量门禁失败: %v", err)
	}
	return nil
}

func syncServiceScanReports(ctx context.Context, svcCtx *svc.ServiceContext, run *model.DevopsPipelineRun, pipeline *model.DevopsPipeline, operator string) error {
	if run == nil || pipeline == nil || !pipeline.ScanEnabled {
		return nil
	}
	hasServiceAPI := false
	for _, item := range pipeline.ScanItems {
		if strings.EqualFold(strings.TrimSpace(item.ToolMode), "service_api") {
			hasServiceAPI = true
			break
		}
	}
	if !hasServiceAPI {
		return nil
	}
	if strings.TrimSpace(operator) == "" {
		operator = strings.TrimSpace(run.UpdatedBy)
	}
	if strings.TrimSpace(operator) == "" {
		operator = "system"
	}
	_, err := svcCtx.QualityRpc.ScanServiceReportSync(ctx, &qualityservice.SyncServiceScanReportReq{
		RunId:        run.ID.Hex(),
		Operator:     operator,
		CurrentRoles: []string{"SUPER_ADMIN"},
	})
	return err
}

func evaluateQualityGateForRun(ctx context.Context, svcCtx *svc.ServiceContext, run *model.DevopsPipelineRun, pipeline *model.DevopsPipeline, operator string) error {
	if run == nil || pipeline == nil || !pipeline.ScanEnabled {
		return nil
	}
	if strings.TrimSpace(operator) == "" {
		operator = strings.TrimSpace(run.UpdatedBy)
	}
	if strings.TrimSpace(operator) == "" {
		operator = "system"
	}
	_, err := svcCtx.QualityRpc.ScanGateEvaluate(ctx, &qualityservice.ScanGateEvaluateReq{
		RunId:        run.ID.Hex(),
		Operator:     operator,
		CurrentRoles: []string{"SUPER_ADMIN"},
	})
	return err
}

type pipelineArtifactConfig struct {
	StepID string
	Config model.StepArtifactConfig
}

func artifactConfigsFromPipeline(pipeline *model.DevopsPipeline) []pipelineArtifactConfig {
	items := make([]pipelineArtifactConfig, 0)
	if pipeline == nil {
		return items
	}
	for _, step := range pipeline.Steps {
		if !step.Enabled || !step.ArtifactConfig.Enabled || strings.TrimSpace(step.ArtifactConfig.Path) == "" {
			continue
		}
		items = append(items, pipelineArtifactConfig{StepID: step.ID, Config: step.ArtifactConfig})
	}
	return items
}

func syncQualityReportArtifact(ctx context.Context, svcCtx *svc.ServiceContext, run *model.DevopsPipelineRun, pipeline *model.DevopsPipeline, cfg pipelineArtifactConfig, artifact jenkins.BuildArtifact, upload *pipelineconfigservice.UploadPipelineArtifactResp, data []byte, contentType, operator string) error {
	if run == nil || pipeline == nil || upload == nil || !pipeline.ScanEnabled {
		return nil
	}
	scanItem := matchScanItemForArtifact(pipeline.ScanItems, cfg.StepID, artifact.RelativePath)
	if scanItem == nil {
		return nil
	}
	sha := fmt.Sprintf("%x", sha256.Sum256(data))
	batch, err := svcCtx.QualityRpc.ReportUploadBatchCreate(ctx, &qualityservice.CreateReportUploadBatchReq{
		RunId:            run.ID.Hex(),
		StageId:          scanItem.StageID,
		StepId:           scanItem.StepID,
		Tool:             scanItem.Tool,
		ReportFormat:     scanItem.ReportFormat,
		Parser:           scanItem.Parser,
		SourceType:       "jenkins_artifact",
		OriginalFileName: artifact.FileName,
		Bucket:           upload.Bucket,
		ObjectKey:        upload.ObjectKey,
		Size:             upload.Size,
		ContentType:      contentType,
		Sha256:           sha,
		Operator:         operator,
	})
	if err != nil {
		return err
	}
	entries, err := qualityreport.ExpandReportUpload(artifact.FileName, scanItem.ReportPath, data)
	if err != nil {
		entries = []qualityreport.Entry{{
			EntryPath:        artifact.RelativePath,
			OriginalFileName: artifact.FileName,
			Status:           "rejected",
			Reason:           err.Error(),
		}}
	}
	var acceptedCount, rejectedCount, ignoredCount int64
	archive := isArchiveArtifact(artifact.FileName)
	for _, entry := range entries {
		status := entry.Status
		reason := entry.Reason
		reportID := ""
		objectKey := ""
		entryBucket := upload.Bucket
		entryContentType := entry.ContentType
		entrySize := entry.Size
		entrySHA := entry.Sha256
		if status == "" {
			entryObjectKey := upload.ObjectKey
			if archive {
				entryObjectKey = path.Join(upload.ObjectKey, "entries", entry.EntryPath)
				entryUpload, err := svcCtx.PipelineConfigRpc.UploadPipelineArtifact(ctx, &pipelineconfigservice.UploadPipelineArtifactReq{
					ObjectKey:   entryObjectKey,
					Data:        entry.Data,
					ContentType: entry.ContentType,
				})
				if err != nil {
					status = "rejected"
					reason = "上传解压后的扫描报告失败"
				} else {
					entryBucket = entryUpload.Bucket
					entryObjectKey = entryUpload.ObjectKey
					entrySize = entryUpload.Size
				}
			}
			objectKey = entryObjectKey
			if status == "" {
				entrySHA = fmt.Sprintf("%x", sha256.Sum256(entry.Data))
				summaryJSON, issuesJSON, err := qualityreport.ParseScanReport(scanItem.Tool, scanItem.Parser, scanItem.ReportFormat, entry.Data)
				if err != nil {
					status = "rejected"
					reason = err.Error()
				} else {
					completeReq := qualityReportCompleteReq(run.ID.Hex(), batch.Id, scanItem, entry, entryBucket, objectKey, entryContentType, entrySHA, entrySize, operator, summaryJSON, issuesJSON)
					if _, err := svcCtx.QualityRpc.ScanReportValidate(ctx, completeReq); err != nil {
						status = "rejected"
						reason = err.Error()
					} else if completed, err := svcCtx.QualityRpc.ScanReportComplete(ctx, completeReq); err != nil {
						status = "rejected"
						reason = err.Error()
					} else {
						status = "accepted"
						reportID = completed.Id
					}
				}
			}
		}
		if status == "" {
			status = "rejected"
			reason = "扫描报告处理失败"
		}
		switch status {
		case "accepted":
			acceptedCount++
		case "ignored":
			ignoredCount++
		default:
			rejectedCount++
		}
		if _, err := svcCtx.QualityRpc.ReportUploadEntryCreate(ctx, &qualityservice.CreateReportUploadEntryReq{
			BatchId:          batch.Id,
			RunId:            run.ID.Hex(),
			StageId:          scanItem.StageID,
			StepId:           scanItem.StepID,
			Tool:             scanItem.Tool,
			ReportFormat:     scanItem.ReportFormat,
			Parser:           scanItem.Parser,
			EntryPath:        entry.EntryPath,
			OriginalFileName: entry.OriginalFileName,
			Bucket:           entryBucket,
			ObjectKey:        objectKey,
			Size:             entrySize,
			ContentType:      entryContentType,
			Sha256:           entrySHA,
			Status:           status,
			Reason:           reason,
			ReportId:         reportID,
			Operator:         operator,
		}); err != nil {
			return err
		}
	}
	status := "success"
	message := "Jenkins 扫描报告产物处理完成"
	if acceptedCount == 0 && (rejectedCount > 0 || ignoredCount > 0) {
		status = "failed"
		message = "Jenkins 扫描报告产物处理失败"
	} else if rejectedCount > 0 || ignoredCount > 0 {
		status = "partial"
		message = "Jenkins 扫描报告产物部分处理成功"
	}
	_, err = svcCtx.QualityRpc.ReportUploadBatchUpdate(ctx, &qualityservice.UpdateReportUploadBatchReq{
		Id:            batch.Id,
		Status:        status,
		AcceptedCount: acceptedCount,
		RejectedCount: rejectedCount,
		IgnoredCount:  ignoredCount,
		Message:       message,
		Operator:      operator,
	})
	return err
}

func matchScanItemForArtifact(items []model.ScanPlanItem, stepID, relativePath string) *model.ScanPlanItem {
	for i := range items {
		item := &items[i]
		if strings.TrimSpace(item.Tool) == "" || strings.TrimSpace(item.ReportFormat) == "" {
			continue
		}
		if strings.TrimSpace(item.ReportPath) == "" {
			continue
		}
		if item.StepID != "" && stepID != "" && item.StepID != stepID {
			continue
		}
		if !artifactPathMatch(item.ReportPath, strings.Trim(relativePath, "/")) {
			continue
		}
		return item
	}
	return nil
}

func qualityReportCompleteReq(runID, batchID string, item *model.ScanPlanItem, entry qualityreport.Entry, bucket, objectKey, contentType, sha string, size int64, operator, summaryJSON, issuesJSON string) *qualityservice.CompleteScanReportReq {
	return &qualityservice.CompleteScanReportReq{
		RunId:            runID,
		StageId:          item.StageID,
		StepId:           item.StepID,
		Tool:             item.Tool,
		ReportFormat:     item.ReportFormat,
		TargetType:       item.TargetType,
		TargetName:       item.TargetName,
		ImageRef:         qualityReportImageRef(item),
		Bucket:           bucket,
		ObjectKey:        objectKey,
		Size:             size,
		ContentType:      contentType,
		Sha256:           sha,
		SummaryJson:      summaryJSON,
		IssuesJson:       issuesJSON,
		ReportPath:       entry.EntryPath,
		Parser:           item.Parser,
		BatchId:          batchID,
		SourceType:       "jenkins_artifact",
		ArchiveEntryPath: entry.EntryPath,
		OriginalFileName: entry.OriginalFileName,
		Operator:         operator,
	}
}

func isArchiveArtifact(fileName string) bool {
	lower := strings.ToLower(strings.TrimSpace(fileName))
	return strings.HasSuffix(lower, ".zip") || strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz")
}

func qualityReportImageRef(item *model.ScanPlanItem) string {
	if item != nil && strings.EqualFold(strings.TrimSpace(item.TargetType), "image") {
		return strings.TrimSpace(item.TargetName)
	}
	return ""
}

func matchedJenkinsArtifacts(pattern string, artifacts []jenkins.BuildArtifact) []jenkins.BuildArtifact {
	pattern = strings.Trim(strings.TrimSpace(pattern), "/")
	if pattern == "" {
		return nil
	}
	result := make([]jenkins.BuildArtifact, 0)
	for _, item := range artifacts {
		relative := strings.Trim(strings.TrimSpace(item.RelativePath), "/")
		if relative == "" {
			continue
		}
		if artifactPathMatch(pattern, relative) {
			result = append(result, item)
		}
	}
	return result
}

func artifactPathMatch(pattern, relative string) bool {
	if pattern == relative {
		return true
	}
	ok, err := path.Match(pattern, relative)
	if err == nil && ok {
		return true
	}
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		return strings.HasPrefix(relative, strings.Trim(prefix, "/")+"/")
	}
	return false
}

func pipelineArtifactObjectKey(run *model.DevopsPipelineRun, relativePath string) string {
	build := fmt.Sprintf("%d", run.JenkinsBuildNumber)
	parts := []string{run.ProjectCode, run.SystemCode, run.EnvironmentCode, build, strings.Trim(relativePath, "/")}
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(strings.TrimSpace(part), "/")
		if part != "" {
			cleaned = append(cleaned, part)
		}
	}
	return path.Join(cleaned...)
}

func artifactName(config model.StepArtifactConfig) string {
	if strings.TrimSpace(config.Name) != "" {
		return strings.TrimSpace(config.Name)
	}
	if strings.TrimSpace(config.Path) != "" {
		return path.Base(strings.Trim(config.Path, "/"))
	}
	return "构建产物"
}

func artifactType(config model.StepArtifactConfig) string {
	if strings.TrimSpace(config.Type) != "" {
		return strings.TrimSpace(config.Type)
	}
	return "report"
}

func isFinalRunStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "success", "failed", "aborted", "unstable", "skipped", "finished":
		return true
	default:
		return false
	}
}

func activeRunStatuses() []string {
	return []string{"preparing", "queued", "running", "paused"}
}

func refreshPipelineLastRunStatus(ctx context.Context, svcCtx *svc.ServiceContext, pipelineID, updatedBy string) {
	if strings.TrimSpace(pipelineID) == "" {
		return
	}
	latest, err := svcCtx.PipelineRunModel.FindLatestByPipeline(ctx, pipelineID)
	status := "never"
	if err == nil && latest != nil && strings.TrimSpace(latest.Status) != "" {
		status = latest.Status
	} else if err != nil && !errors.Is(err, model.ErrNotFound) {
		logx.Errorf("刷新流水线最近运行状态失败: %v", err)
	}
	_ = svcCtx.PipelineModel.UpdateRunStatus(ctx, pipelineID, status, updatedBy)
}

func removePipelineRunLogCache(ctx context.Context, svcCtx *svc.ServiceContext, runID string, stages []*model.DevopsPipelineRunStage) {
	if svcCtx == nil || svcCtx.Cache == nil || strings.TrimSpace(runID) == "" {
		return
	}
	_, _ = svcCtx.Cache.DelCtx(ctx, pipelineRunFullLogCacheKey(runID))
	for _, stage := range stages {
		if stage == nil {
			continue
		}
		_, _ = svcCtx.Cache.DelCtx(ctx, pipelineRunStageLogCacheKey(runID, stage.ID.Hex()))
	}
}

func recordPipelineRunMetric(ctx context.Context, svcCtx *svc.ServiceContext, run *model.DevopsPipelineRun) error {
	if run == nil || run.StatsRecorded {
		return nil
	}
	changed, err := svcCtx.PipelineRunModel.MarkStatsRecorded(ctx, run.ID.Hex())
	if err != nil || !changed {
		logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
		return err
	}
	run.StatsRecorded = true
	inc := model.PipelineMetricIncrement{
		BuildCount:           1,
		TotalDurationSeconds: run.DurationSeconds,
	}
	switch strings.TrimSpace(run.Status) {
	case "success":
		inc.SuccessCount = 1
	case "failed", "unstable":
		inc.FailureCount = 1
	case "aborted", "skipped":
		inc.AbortedCount = 1
	default:
		inc.FailureCount = 1
	}
	if err := svcCtx.MetricModel.UpsertFromRun(ctx, run, inc); err != nil {
		_ = svcCtx.PipelineRunModel.ResetStatsRecorded(ctx, run.ID.Hex())
		run.StatsRecorded = false
		logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
		return err
	}
	return nil
}

func normalizeJenkinsResult(result string) string {
	switch strings.ToUpper(strings.TrimSpace(result)) {
	case "SUCCESS":
		return "success"
	case "FAILURE":
		return "failed"
	case "ABORTED":
		return "aborted"
	case "UNSTABLE":
		return "unstable"
	case "NOT_BUILT":
		return "skipped"
	default:
		return strings.ToLower(strings.TrimSpace(result))
	}
}

func ensurePipelineAccess(ctx context.Context, svcCtx *svc.ServiceContext, pipeline *model.DevopsPipeline, userID uint64, roles []string) error {
	if pipeline == nil {
		logx.Errorf("流水线不存在")
		return errorx.Msg("流水线不存在")
	}
	return ensureProjectAccess(ctx, svcCtx, pipeline.ProjectID, userID, roles)
}

func ignoreNotFound(err error) error {
	if errors.Is(err, model.ErrNotFound) {
		return nil
	}
	logx.Errorf("处理流水线执行辅助逻辑失败: %v", err)
	return err
}
