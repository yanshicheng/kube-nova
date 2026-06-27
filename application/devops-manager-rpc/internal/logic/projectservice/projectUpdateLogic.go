package projectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectUpdateLogic {
	return &ProjectUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ProjectUpdateLogic) ProjectUpdate(in *pb.UpdateProjectReq) (*pb.EmptyResp, error) {
	oid, err := model.ObjectIDForUpdate(in.Id)
	if err != nil {
		l.Errorf("项目更新失败: %v", err)
		return nil, err
	}
	exist, err := l.svcCtx.ProjectModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("项目更新失败: %v", err)
		return nil, err
	}
	defaultChannelID := exist.DefaultEngineChannelID
	pipelineEngineType := exist.PipelineEngineType
	var buildChannels []*model.DevopsChannel
	if len(in.BuildChannelIds) > 0 {
		defaultChannelID, buildChannels, err = prepareProjectBuildChannels(l.ctx, l.svcCtx, in.BuildChannelIds, in.DefaultEngineChannelId)
		if err != nil {
			l.Errorf("项目更新失败: %v", err)
			return nil, err
		}
		pipelineEngineType = defaultBuildChannelType(buildChannels, defaultChannelID)
	}
	data := &model.DevopsProject{
		ID:                     oid,
		Name:                   in.Name,
		Description:            in.Description,
		PipelineEngineType:     pipelineEngineType,
		DefaultEngineChannelID: defaultChannelID,
		Status:                 in.Status,
		ExtraConfig:            in.ExtraConfig,
		UpdatedBy:              in.UpdatedBy,
	}

	if err := l.svcCtx.ProjectModel.Update(l.ctx, data); err != nil {
		if model.IsDuplicateKey(err) {
			l.Errorf("项目编码已存在")
			return nil, errorx.Msg("项目编码已存在")
		}
		l.Errorf("项目更新失败: %v", err)
		return nil, err
	}
	if len(buildChannels) > 0 {
		if err := replaceProjectBuildChannels(l.ctx, l.svcCtx, in.Id, buildChannels, defaultChannelID, true, "", "", in.UpdatedBy); err != nil {
			l.Errorf("项目更新失败: %v", err)
			return nil, err
		}
	}

	return &pb.EmptyResp{}, nil
}
