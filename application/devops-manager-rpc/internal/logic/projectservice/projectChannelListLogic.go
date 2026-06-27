package projectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectChannelListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectChannelListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectChannelListLogic {
	return &ProjectChannelListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ProjectChannelListLogic) ProjectChannelList(in *pb.ListProjectChannelBindingReq) (*pb.ListProjectChannelBindingResp, error) {
	projectIDs, restricted, err := memberProjectScope(l.ctx, l.svcCtx, in.CurrentUserId, in.CurrentRoles)
	if err != nil {
		l.Errorf("项目渠道查询列表失败: %v", err)
		return nil, err
	}
	data, total, err := l.svcCtx.ProjectChannelModel.List(l.ctx, model.DevopsProjectChannelBindingListFilter{
		ProjectID:        in.ProjectId,
		ProjectIDs:       projectIDs,
		Restricted:       restricted,
		ChannelID:        in.ChannelId,
		ChannelGroupCode: in.ChannelGroupCode,
		ChannelType:      in.ChannelType,
		UsageScope:       in.UsageScope,
		Status:           in.Status,
		Page:             in.Page,
		PageSize:         in.PageSize,
	})
	if err != nil {
		l.Errorf("项目渠道查询列表失败: %v", err)
		return nil, err
	}
	channels, groups, err := l.preloadChannelBindings(data)
	if err != nil {
		l.Errorf("项目渠道查询列表失败: %v", err)
		return nil, err
	}
	items := make([]*pb.DevopsProjectChannelBinding, 0, len(data))
	for _, item := range data {
		channel := l.hydrateChannelBinding(item, channels, groups)
		out := projectChannelBindingToPb(item)
		if channel != nil {
			out.ChannelConfig = channel.Config
		}
		items = append(items, out)
	}

	return &pb.ListProjectChannelBindingResp{Data: items, Total: total}, nil
}

func (l *ProjectChannelListLogic) preloadChannelBindings(items []*model.DevopsProjectChannelBinding) (map[string]*model.DevopsChannel, map[string]*model.DevopsChannelGroup, error) {
	channelIDs := make([]string, 0, len(items))
	for _, item := range items {
		if item != nil && item.ChannelID != "" {
			channelIDs = append(channelIDs, item.ChannelID)
		}
	}
	channels, err := l.svcCtx.ChannelModel.FindByIDs(l.ctx, channelIDs)
	if err != nil {
		l.Errorf("预加载渠道绑定失败: %v", err)
		return nil, nil, err
	}
	groupIDs := make([]string, 0, len(channels))
	for _, channel := range channels {
		if channel != nil && channel.GroupID != "" {
			groupIDs = append(groupIDs, channel.GroupID)
		}
	}
	groups, err := l.svcCtx.ChannelGroupModel.FindByIDs(l.ctx, groupIDs)
	if err != nil {
		l.Errorf("预加载渠道绑定失败: %v", err)
		return nil, nil, err
	}
	return channels, groups, nil
}

func (l *ProjectChannelListLogic) hydrateChannelBinding(item *model.DevopsProjectChannelBinding, channels map[string]*model.DevopsChannel, groups map[string]*model.DevopsChannelGroup) *model.DevopsChannel {
	if item == nil || item.ChannelID == "" {
		return nil
	}
	channel := channels[item.ChannelID]
	if channel == nil {
		if item.ChannelGroupCode == "" {
			item.ChannelGroupCode = item.UsageScope
			if item.UsageScope == model.BuildUsageScope {
				item.ChannelGroupCode = model.BuildChannelGroupCode
			}
		}
		return nil
	}
	if item.ChannelName == "" {
		item.ChannelName = channel.Name
	}
	if item.ChannelCode == "" {
		item.ChannelCode = channel.Code
	}
	if item.ChannelType == "" {
		item.ChannelType = channel.ChannelType
	}
	if item.ChannelGroupCode == "" {
		if group := groups[channel.GroupID]; group != nil {
			item.ChannelGroupCode = group.Code
		}
	}
	return channel
}
