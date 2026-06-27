package projectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectChannelBindLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectChannelBindLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectChannelBindLogic {
	return &ProjectChannelBindLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ProjectChannelBindLogic) ProjectChannelBind(in *pb.BindProjectChannelsReq) (*pb.EmptyResp, error) {
	project, err := l.svcCtx.ProjectModel.FindOne(l.ctx, in.ProjectId)
	if err != nil {
		l.Errorf("项目渠道绑定失败: %v", err)
		return nil, err
	}
	groupCode := in.ChannelGroupCode
	if groupCode == "" && in.UsageScope == model.BuildUsageScope {
		groupCode = model.BuildChannelGroupCode
	}
	defaultChannelID, groupCode, channels, err := prepareProjectGroupChannels(l.ctx, l.svcCtx, in.ChannelIds, in.DefaultChannelId, groupCode)
	if err != nil {
		l.Errorf("项目渠道绑定失败: %v", err)
		return nil, err
	}
	if isBuildGroupCode(l.ctx, l.svcCtx.ChannelGroupModel, groupCode) {
		var defaultChannel *model.DevopsChannel
		for _, channel := range channels {
			if channel.ID.Hex() == defaultChannelID {
				defaultChannel = channel
				break
			}
		}
		if defaultChannel == nil {
			l.Errorf("默认构建渠道不存在")
			return nil, errorx.Msg("默认构建渠道不存在")
		}
		project.PipelineEngineType = defaultBuildChannelType([]*model.DevopsChannel{defaultChannel}, defaultChannelID)
		project.DefaultEngineChannelID = defaultChannelID
		project.UpdatedBy = in.UpdatedBy
		if err := l.svcCtx.ProjectModel.Update(l.ctx, project); err != nil {
			if model.IsDuplicateKey(err) {
				l.Errorf("项目编码已存在")
				return nil, errorx.Msg("项目编码已存在")
			}
			l.Errorf("项目渠道绑定失败: %v", err)
			return nil, err
		}
	}
	if err := replaceProjectGroupChannels(l.ctx, l.svcCtx, in.ProjectId, groupCode, channels, defaultChannelID, in.AllowUseGlobalCredential, in.ProjectCredentialId, in.BindingConfig, in.UpdatedBy); err != nil {
		l.Errorf("项目渠道绑定失败: %v", err)
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}
