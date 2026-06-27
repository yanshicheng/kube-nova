package pipelineconfigservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
)

func ensureJenkinsBuildBinding(ctx context.Context, svcCtx *svc.ServiceContext, projectID, buildChannelBindingID string) (*model.DevopsProjectChannelBinding, error) {
	binding, err := svcCtx.ProjectChannelModel.FindOne(ctx, buildChannelBindingID)
	if err != nil {
		return nil, errorx.Msg("查询 Jenkins 构建渠道失败")
	}
	if binding.ProjectID != projectID {
		return nil, errorx.Msg("Jenkins 构建渠道不属于当前项目")
	}
	if binding.Status != 1 {
		return nil, errorx.Msg("Jenkins 构建渠道已停用")
	}
	if binding.ChannelGroupCode != jenkinsEngineChannelGroupCode || binding.UsageScope != "build" || binding.ChannelType != "jenkins" {
		return nil, errorx.Msg("请选择 Jenkins 构建渠道")
	}
	return binding, nil
}

func jenkinsCredentialSyncRecordToPb(ctx context.Context, svcCtx *svc.ServiceContext, item *model.DevopsCredentialSync) (*pb.JenkinsCredentialSyncRecord, error) {
	if item == nil {
		return &pb.JenkinsCredentialSyncRecord{}, nil
	}
	out := &pb.JenkinsCredentialSyncRecord{
		CredentialId:        item.CredentialID,
		JenkinsCredentialId: item.JenkinsCredentialID,
		SyncStatus:          item.SyncStatus,
		SyncMessage:         item.SyncMessage,
		LastSyncAt:          item.LastSyncAt.Unix(),
	}
	if credential, err := svcCtx.CredentialModel.FindOne(ctx, item.CredentialID); err == nil && credential != nil {
		out.CredentialName = credential.Name
		out.CredentialCode = credential.Code
		out.CredentialType = credential.CredentialType
	}
	_, total, err := collectJenkinsCredentialReferences(ctx, svcCtx, item.ProjectID, item.BuildChannelBindingID, item.CredentialID)
	if err != nil {
		return nil, err
	}
	out.ReferenceCount = int64(total)
	return out, nil
}

func collectJenkinsCredentialReferences(ctx context.Context, svcCtx *svc.ServiceContext, projectID, buildChannelBindingID, credentialID string) ([]*pb.JenkinsCredentialReference, uint64, error) {
	pipelines, err := svcCtx.PipelineUsageModel.ListByProjectBuildChannel(ctx, projectID, buildChannelBindingID, 0)
	if err != nil {
		return nil, 0, err
	}
	result := make([]*pb.JenkinsCredentialReference, 0)
	seen := make(map[string]struct{})
	for _, pipeline := range pipelines {
		if pipeline == nil {
			continue
		}
		paramByCode := make(map[string]model.UsagePipelineParam, len(pipeline.Params))
		for _, param := range pipeline.Params {
			paramByCode[param.Code] = param
		}
		for _, param := range pipeline.Params {
			referenceType, ok := pipelineCredentialReferenceType(ctx, svcCtx, param, paramByCode, credentialID)
			if !ok {
				continue
			}
			key := pipeline.ID.Hex() + ":" + param.Code + ":" + referenceType
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, &pb.JenkinsCredentialReference{
				PipelineId:    pipeline.ID.Hex(),
				PipelineName:  pipeline.Name,
				PipelineCode:  pipeline.Code,
				ParamCode:     param.Code,
				ParamName:     param.Name,
				ReferenceType: referenceType,
			})
		}
	}
	return result, uint64(len(result)), nil
}

func pipelineCredentialReferenceType(ctx context.Context, svcCtx *svc.ServiceContext, param model.UsagePipelineParam, paramByCode map[string]model.UsagePipelineParam, credentialID string) (string, bool) {
	config := param.Config
	if param.Mode == "paas" && param.ParamType == "voucherModel" && normalizeCredentialMode(config.CredentialMode, config.MappingField) == credentialModeJenkinsID {
		if paramValueMatchesCredential(param.CurrentValue, credentialID) || paramValueMatchesCredential(param.DefaultValue, credentialID) || strings.TrimSpace(config.CredentialID) == credentialID {
			return "voucherModel", true
		}
	}
	if strings.TrimSpace(config.CredentialID) == credentialID {
		return "configCredential", true
	}
	if param.Mode == "paas" && param.ParamType == channelvars.ParamChannelCredential && normalizeCredentialMode(config.CredentialMode, config.MappingField) == credentialModeJenkinsID {
		if channelCredentialMayReferenceCredential(ctx, svcCtx, param, paramByCode, credentialID) {
			return "channelCredential", true
		}
	}
	return "", false
}

func channelCredentialMayReferenceCredential(ctx context.Context, svcCtx *svc.ServiceContext, param model.UsagePipelineParam, paramByCode map[string]model.UsagePipelineParam, credentialID string) bool {
	sourceCode := strings.TrimSpace(param.Config.CredentialSourceParamCode)
	if sourceCode == "" {
		return true
	}
	sourceParam, ok := paramByCode[sourceCode]
	if !ok {
		return true
	}
	sourceBindingID := strings.TrimSpace(sourceParam.Config.ChannelBindingID)
	if sourceBindingID == "" && strings.TrimSpace(sourceParam.Config.ChannelParamCode) != "" {
		channelParam, ok := paramByCode[strings.TrimSpace(sourceParam.Config.ChannelParamCode)]
		if !ok {
			return true
		}
		sourceBindingID = firstNonEmptyString(channelParam.CurrentValue, channelParam.DefaultValue)
	}
	if sourceBindingID == "" && channelvars.IsChannelParamType(sourceParam.ParamType) {
		sourceBindingID = firstNonEmptyString(sourceParam.CurrentValue, sourceParam.DefaultValue)
	}
	if sourceBindingID == "" {
		return true
	}
	binding, err := svcCtx.ProjectChannelModel.FindOne(ctx, sourceBindingID)
	if err != nil || binding == nil {
		return true
	}
	if strings.TrimSpace(binding.ProjectCredentialID) == credentialID {
		return true
	}
	if !binding.AllowUseGlobalCredential {
		return false
	}
	channel, err := svcCtx.ChannelModel.FindOne(ctx, binding.ChannelID)
	if err != nil || channel == nil {
		return true
	}
	return strings.TrimSpace(channel.CredentialID) == credentialID || strings.TrimSpace(channel.GlobalCredentialID) == credentialID
}

func paramValueMatchesCredential(value, credentialID string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	return value == credentialID
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
