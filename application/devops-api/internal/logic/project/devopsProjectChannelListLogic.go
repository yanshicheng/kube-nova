// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package project

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/client/projectservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type DevopsProjectChannelListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 分页查询项目渠道绑定
func NewDevopsProjectChannelListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsProjectChannelListLogic {
	return &DevopsProjectChannelListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsProjectChannelListLogic) DevopsProjectChannelList(req *types.ListDevopsProjectChannelBindingRequest) (resp *types.ListDevopsProjectChannelBindingResponse, err error) {
	result, err := l.svcCtx.ProjectRpc.ProjectChannelList(l.ctx, &projectservice.ListProjectChannelBindingReq{
		Page:             req.Page,
		PageSize:         req.PageSize,
		ProjectId:        req.ProjectId,
		ChannelId:        req.ChannelId,
		ChannelGroupCode: req.ChannelGroupCode,
		ChannelType:      req.ChannelType,
		UsageScope:       req.UsageScope,
		Status:           req.Status,
		CurrentUserId:    currentUserID(l.ctx),
		CurrentRoles:     currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.DevopsProjectChannelBinding, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, projectChannelBindingToType(item))
	}

	return &types.ListDevopsProjectChannelBindingResponse{Items: items, Total: result.Total}, nil
}
