package projectservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/tektonsync"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectChannelUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectChannelUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectChannelUpdateLogic {
	return &ProjectChannelUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ProjectChannelUpdateLogic) ProjectChannelUpdate(in *pb.UpdateProjectChannelBindingReq) (*pb.EmptyResp, error) {
	binding, err := l.svcCtx.ProjectChannelModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("项目渠道更新失败: %v", err)
		return nil, err
	}
	groupCode := resolveBindingGroupCode(l.ctx, l.svcCtx, binding)
	channel, err := l.svcCtx.ChannelModel.FindOne(l.ctx, binding.ChannelID)
	if err != nil {
		l.Errorf("项目渠道更新失败: %v", err)
		return nil, err
	}
	project, err := l.svcCtx.ProjectModel.FindOne(l.ctx, binding.ProjectID)
	if err != nil {
		l.Errorf("项目渠道更新失败: %v", err)
		return nil, err
	}
	isDefault := false
	if resolveGroup(l.ctx, l.svcCtx.ChannelGroupModel, groupCode).IsBuildGroup() {
		isDefault = in.IsDefault
		if binding.IsDefault && !isDefault {
			l.Errorf("默认构建渠道不能直接取消，请设置其他构建渠道为默认")
			return nil, errorx.Msg("默认构建渠道不能直接取消，请设置其他构建渠道为默认")
		}
	}
	projectCredentialID := strings.TrimSpace(in.ProjectCredentialId)
	if err := validateProjectBindingCredential(l.ctx, l.svcCtx, binding.ProjectID, groupCode, channel.ChannelType, in.AllowUseGlobalCredential, projectCredentialID); err != nil {
		l.Errorf("项目渠道更新失败: %v", err)
		return nil, err
	}
	bindingConfig := in.BindingConfig
	if channel.ChannelType == tektonsync.EngineTypeTekton {
		bindingConfig, err = tektonsync.NormalizeBindingConfigForProject(project.Code, in.BindingConfig)
		if err != nil {
			l.Errorf("项目渠道更新失败: %v", err)
			return nil, err
		}
	}
	if err := tektonsync.ValidateBindingConfig(l.ctx, l.svcCtx, channel, bindingConfig); err != nil {
		l.Errorf("项目渠道更新失败: %v", err)
		return nil, err
	}
	credentialChanged := binding.AllowUseGlobalCredential != in.AllowUseGlobalCredential ||
		binding.ProjectCredentialID != projectCredentialID
	binding.IsDefault = isDefault
	binding.AllowUseGlobalCredential = in.AllowUseGlobalCredential
	binding.ProjectCredentialID = projectCredentialID
	binding.BindingConfig = bindingConfig
	binding.Status = in.Status
	binding.UpdatedBy = in.UpdatedBy
	if err := l.svcCtx.ProjectChannelModel.UpdateBinding(l.ctx, binding); err != nil {
		l.Errorf("项目渠道更新失败: %v", err)
		return nil, err
	}
	if credentialChanged {
		if err := l.svcCtx.ProjectChannelModel.UpdateHealth(l.ctx, binding.ID.Hex(), "unknown", "凭据已变更，请重新测试", "", in.UpdatedBy, 0); err != nil {
			l.Errorf("项目渠道更新失败: %v", err)
			return nil, err
		}
	}
	if resolveGroup(l.ctx, l.svcCtx.ChannelGroupModel, groupCode).IsBuildGroup() && isDefault {
		if err := l.svcCtx.ProjectChannelModel.ClearDefaultByProjectGroup(l.ctx, binding.ProjectID, groupCode, binding.ID.Hex(), in.UpdatedBy); err != nil {
			l.Errorf("项目渠道更新失败: %v", err)
			return nil, err
		}
		if err := updateProjectDefaultBuildChannel(l.ctx, l.svcCtx, binding.ProjectID, channel, in.UpdatedBy); err != nil {
			l.Errorf("项目渠道更新失败: %v", err)
			return nil, err
		}
	}

	return &pb.EmptyResp{}, nil
}
