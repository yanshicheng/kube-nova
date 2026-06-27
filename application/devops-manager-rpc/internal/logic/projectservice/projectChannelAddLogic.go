package projectservicelogic

import (
	"context"
	"errors"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/tektonsync"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectChannelAddLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectChannelAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectChannelAddLogic {
	return &ProjectChannelAddLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ProjectChannelAddLogic) ProjectChannelAdd(in *pb.AddProjectChannelReq) (*pb.EmptyResp, error) {
	project, err := l.svcCtx.ProjectModel.FindOne(l.ctx, in.ProjectId)
	if err != nil {
		l.Errorf("项目渠道新增失败: %v", err)
		return nil, err
	}
	channel, err := l.svcCtx.ChannelModel.FindOne(l.ctx, in.ChannelId)
	if err != nil {
		l.Errorf("项目渠道新增失败: %v", err)
		return nil, err
	}
	if channel.Status != 1 {
		l.Errorf("渠道实例必须是启用状态")
		return nil, errorx.Msg("渠道实例必须是启用状态")
	}
	if channel.IsSystem {
		l.Errorf("只能绑定用户配置的渠道实例")
		return nil, errorx.Msg("只能绑定用户配置的渠道实例")
	}
	group, err := resolveChannelGroup(l.ctx, l.svcCtx, channel)
	if err != nil {
		l.Errorf("项目渠道新增失败: %v", err)
		return nil, err
	}
	exists, err := l.svcCtx.ProjectChannelModel.FindActiveByProjectChannel(l.ctx, in.ProjectId, in.ChannelId)
	if err == nil {
		if err := l.finishExistingBinding(in, channel, group, exists); err != nil {
			l.Errorf("项目渠道新增失败: %v", err)
			return nil, err
		}
		return &pb.EmptyResp{}, nil
	}
	if !errors.Is(err, model.ErrNotFound) {
		l.Errorf("项目渠道新增失败: %v", err)
		return nil, err
	}
	bindingConfig := in.BindingConfig
	if channel.ChannelType == tektonsync.EngineTypeTekton {
		bindingConfig, err = tektonsync.NormalizeBindingConfigForProject(project.Code, in.BindingConfig)
		if err != nil {
			l.Errorf("项目渠道新增失败: %v", err)
			return nil, err
		}
	}
	if err := validateProjectBindingCredential(l.ctx, l.svcCtx, in.ProjectId, group.Code, channel.ChannelType, in.AllowUseGlobalCredential, in.ProjectCredentialId); err != nil {
		l.Errorf("项目渠道新增失败: %v", err)
		return nil, err
	}
	if err := tektonsync.ValidateBindingConfig(l.ctx, l.svcCtx, channel, bindingConfig); err != nil {
		l.Errorf("项目渠道新增失败: %v", err)
		return nil, err
	}

	isDefault := false
	usageScope := group.Code
	if group.IsBuildGroup() {
		usageScope = model.BuildUsageScope
		isDefault = in.IsDefault
		hasDefault, err := l.svcCtx.ProjectChannelModel.HasDefaultActiveByProjectGroup(l.ctx, in.ProjectId, group.Code)
		if err != nil {
			l.Errorf("项目渠道新增失败: %v", err)
			return nil, err
		}
		if !hasDefault {
			isDefault = true
		}
	}

	data := &model.DevopsProjectChannelBinding{
		ProjectID:                in.ProjectId,
		ChannelID:                channel.ID.Hex(),
		ChannelGroupCode:         group.Code,
		ChannelName:              channel.Name,
		ChannelCode:              channel.Code,
		ChannelType:              channel.ChannelType,
		UsageScope:               usageScope,
		IsDefault:                isDefault,
		AllowUseGlobalCredential: in.AllowUseGlobalCredential,
		ProjectCredentialID:      in.ProjectCredentialId,
		BindingConfig:            bindingConfig,
		Status:                   1,
		CreatedBy:                in.CreatedBy,
		UpdatedBy:                in.CreatedBy,
	}
	if err := l.svcCtx.ProjectChannelModel.Insert(l.ctx, data); err != nil {
		if model.IsDuplicateKey(err) {
			exists, findErr := l.svcCtx.ProjectChannelModel.FindActiveByProjectChannel(l.ctx, in.ProjectId, in.ChannelId)
			if findErr == nil {
				if finishErr := l.finishExistingBinding(in, channel, group, exists); finishErr != nil {
					l.Errorf("%s", finishErr)
					return nil, finishErr
				}
			}
			return &pb.EmptyResp{}, nil
		}
		if model.IsDuplicateKey(err) {
			l.Errorf("项目渠道绑定已存在")
			return nil, errorx.Msg("项目渠道绑定已存在")
		}
		l.Errorf("项目渠道新增失败: %v", err)
		return nil, err
	}
	if group.IsBuildGroup() && isDefault {
		if err := l.svcCtx.ProjectChannelModel.ClearDefaultByProjectGroup(l.ctx, in.ProjectId, group.Code, data.ID.Hex(), in.CreatedBy); err != nil {
			l.Errorf("项目渠道新增失败: %v", err)
			return nil, err
		}
		if err := updateProjectDefaultBuildChannel(l.ctx, l.svcCtx, in.ProjectId, channel, in.CreatedBy); err != nil {
			l.Errorf("项目渠道新增失败: %v", err)
			return nil, err
		}
	}

	return &pb.EmptyResp{}, nil
}

func (l *ProjectChannelAddLogic) finishExistingBinding(in *pb.AddProjectChannelReq, channel *model.DevopsChannel, group *model.DevopsChannelGroup, binding *model.DevopsProjectChannelBinding) error {
	projectCredentialID := strings.TrimSpace(in.ProjectCredentialId)
	if projectCredentialID == "" {
		projectCredentialID = binding.ProjectCredentialID
	}
	if err := validateProjectBindingCredential(l.ctx, l.svcCtx, in.ProjectId, group.Code, channel.ChannelType, in.AllowUseGlobalCredential, projectCredentialID); err != nil {
		l.Errorf("处理已有渠道绑定失败: %v", err)
		return err
	}
	project, err := l.svcCtx.ProjectModel.FindOne(l.ctx, in.ProjectId)
	if err != nil {
		l.Errorf("处理已有渠道绑定失败: %v", err)
		return err
	}
	bindingConfig := in.BindingConfig
	if channel.ChannelType == tektonsync.EngineTypeTekton {
		bindingConfig, err = tektonsync.NormalizeBindingConfigForProject(project.Code, in.BindingConfig)
		if err != nil {
			l.Errorf("处理已有渠道绑定失败: %v", err)
			return err
		}
	}
	if err := tektonsync.ValidateBindingConfig(l.ctx, l.svcCtx, channel, bindingConfig); err != nil {
		l.Errorf("处理已有渠道绑定失败: %v", err)
		return err
	}
	if !group.IsBuildGroup() {
		return nil
	}
	needDefault := in.IsDefault
	if !needDefault {
		hasDefault, err := l.svcCtx.ProjectChannelModel.HasDefaultActiveByProjectGroup(l.ctx, in.ProjectId, group.Code)
		if err != nil {
			l.Errorf("处理已有渠道绑定失败: %v", err)
			return err
		}
		needDefault = !hasDefault
	}
	if !needDefault {
		return nil
	}
	binding.IsDefault = true
	binding.AllowUseGlobalCredential = in.AllowUseGlobalCredential
	binding.ProjectCredentialID = projectCredentialID
	binding.BindingConfig = bindingConfig
	binding.Status = 1
	binding.UpdatedBy = in.CreatedBy
	if err := l.svcCtx.ProjectChannelModel.UpdateBinding(l.ctx, binding); err != nil {
		l.Errorf("处理已有渠道绑定失败: %v", err)
		return err
	}
	if err := l.svcCtx.ProjectChannelModel.ClearDefaultByProjectGroup(l.ctx, in.ProjectId, group.Code, binding.ID.Hex(), in.CreatedBy); err != nil {
		l.Errorf("处理已有渠道绑定失败: %v", err)
		return err
	}
	return updateProjectDefaultBuildChannel(l.ctx, l.svcCtx, in.ProjectId, channel, in.CreatedBy)
}
