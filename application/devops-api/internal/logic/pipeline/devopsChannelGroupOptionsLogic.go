// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package pipeline

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/client/pipelineconfigservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type DevopsChannelGroupOptionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 渠道分组选项
func NewDevopsChannelGroupOptionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsChannelGroupOptionsLogic {
	return &DevopsChannelGroupOptionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsChannelGroupOptionsLogic) DevopsChannelGroupOptions() (resp *types.ChannelGroupOptionsResponse, err error) {
	result, err := l.svcCtx.PipelineRpc.ChannelGroupOptions(l.ctx, &pipelineconfigservice.EmptyResp{})
	if err != nil {
		return nil, err
	}
	items := make([]types.ChannelGroupOption, 0, len(result.Data))
	for _, item := range result.Data {
		if item == nil {
			continue
		}
		items = append(items, types.ChannelGroupOption{Code: item.Code, Name: item.Name})
	}

	return &types.ChannelGroupOptionsResponse{Items: items}, nil
}
