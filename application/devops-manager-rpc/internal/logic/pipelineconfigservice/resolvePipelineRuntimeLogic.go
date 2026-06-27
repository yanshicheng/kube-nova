package pipelineconfigservicelogic

import (
	"context"
	"errors"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ResolvePipelineRuntimeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResolvePipelineRuntimeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResolvePipelineRuntimeLogic {
	return &ResolvePipelineRuntimeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ResolvePipelineRuntimeLogic) ResolvePipelineRuntime(in *pb.ResolvePipelineRuntimeReq) (*pb.ResolvePipelineRuntimeResp, error) {
	if err := ensureProjectAccess(l.ctx, l.svcCtx, in.ProjectId, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("解析流水线运行时失败: %v", err)
		return nil, err
	}
	project, err := l.svcCtx.ProjectModel.FindOne(l.ctx, in.ProjectId)
	if err != nil {
		l.Errorf("解析流水线运行时失败: %v", err)
		return nil, err
	}
	system, err := l.svcCtx.SystemModel.FindOne(l.ctx, in.SystemId)
	if err != nil {
		l.Errorf("解析流水线运行时失败: %v", err)
		return nil, err
	}
	if system.ProjectID != project.ID.Hex() {
		l.Errorf("系统不属于当前项目")
		return nil, errorx.Msg("系统不属于当前项目")
	}
	environment, err := l.svcCtx.PipelineEnvModel.FindOne(l.ctx, in.EnvironmentId)
	if err != nil {
		l.Errorf("解析流水线运行时失败: %v", err)
		return nil, err
	}
	binding, err := l.svcCtx.ProjectChannelModel.FindOne(l.ctx, in.BuildChannelBindingId)
	if err != nil {
		l.Errorf("解析流水线运行时失败: %v", err)
		return nil, err
	}
	if binding.ProjectID != project.ID.Hex() {
		l.Errorf("构建渠道绑定不属于当前项目")
		return nil, errorx.Msg("构建渠道绑定不属于当前项目")
	}
	if binding.Status != 1 {
		l.Errorf("构建渠道绑定已停用")
		return nil, errorx.Msg("构建渠道绑定已停用")
	}
	if binding.ChannelGroupCode != jenkinsEngineChannelGroupCode || binding.UsageScope != "build" {
		l.Errorf("请选择构建渠道绑定")
		return nil, errorx.Msg("请选择构建渠道绑定")
	}
	if binding.ChannelType != "jenkins" && binding.ChannelType != "tekton" {
		l.Errorf("构建渠道只支持 Jenkins 或 Tekton")
		return nil, errorx.Msg("构建渠道只支持 Jenkins 或 Tekton")
	}
	channel, err := l.svcCtx.ChannelModel.FindOne(l.ctx, binding.ChannelID)
	if err != nil {
		l.Errorf("解析流水线运行时失败: %v", err)
		return nil, err
	}
	if channel.Status != 1 {
		l.Errorf("构建渠道已停用")
		return nil, errorx.Msg("构建渠道已停用")
	}

	credential, credentialReady, credentialMessage, err := resolveBindingCredential(l.ctx, l.svcCtx, project.ID.Hex(), binding, channel)
	if err != nil {
		l.Errorf("解析流水线运行时失败: %v", err)
		return nil, err
	}

	return &pb.ResolvePipelineRuntimeResp{
		Project:           projectToPb(project),
		System:            systemToPb(system),
		Environment:       pipelineEnvironmentToPb(environment),
		Binding:           projectChannelBindingToPb(binding),
		Channel:           channelToPb(channel),
		Credential:        credential,
		CredentialReady:   credentialReady,
		CredentialMessage: credentialMessage,
	}, nil
}

func resolveBindingCredential(ctx context.Context, svcCtx *svc.ServiceContext, projectID string, binding *model.DevopsProjectChannelBinding, channel *model.DevopsChannel) (*pb.DevopsCredential, bool, string, error) {
	credentialID := strings.TrimSpace(binding.ProjectCredentialID)
	credentialReady := true
	credentialMessage := ""
	if credentialID == "" {
		if binding.AllowUseGlobalCredential {
			credentialID = strings.TrimSpace(channel.CredentialID)
			if credentialID == "" {
				credentialID = strings.TrimSpace(channel.GlobalCredentialID)
			}
		}
		if credentialID == "" {
			if !binding.AllowUseGlobalCredential {
				credentialReady = false
				credentialMessage = "未配置项目级凭据，且当前绑定不允许使用渠道全局凭据"
				return nil, credentialReady, credentialMessage, nil
			}
			if strings.TrimSpace(channel.Username) != "" || strings.TrimSpace(channel.Password) != "" || strings.TrimSpace(channel.Token) != "" {
				return nil, true, "", nil
			}
			credentialReady = false
			credentialMessage = "渠道未配置全局凭据"
		}
	}
	if credentialID == "" {
		return nil, credentialReady, credentialMessage, nil
	}
	credentialData, err := svcCtx.CredentialModel.FindOne(ctx, credentialID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, false, "渠道凭据不存在", nil
		}
		logx.Errorf("解析流水线运行时配置失败: %v", err)
		return nil, false, "", err
	}
	if credentialData.Status != 1 {
		credentialReady = false
		credentialMessage = "凭据已停用"
	}
	if binding.ProjectCredentialID != "" {
		if credentialData.ProjectID != projectID || credentialData.Scope != "project" || credentialData.IsSystem {
			logx.Errorf("项目渠道凭据必须属于当前项目")
			return nil, false, "", errorx.Msg("项目渠道凭据必须属于当前项目")
		}
	}
	credential, err := credentialToPb(credentialData, true)
	if err != nil {
		logx.Errorf("解析流水线运行时配置失败: %v", err)
		return nil, false, "", err
	}
	return credential, credentialReady, credentialMessage, nil
}
