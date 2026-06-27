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

type DevopsStepChannelParamMappingUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新步骤参数到渠道分组映射
func NewDevopsStepChannelParamMappingUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsStepChannelParamMappingUpdateLogic {
	return &DevopsStepChannelParamMappingUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsStepChannelParamMappingUpdateLogic) DevopsStepChannelParamMappingUpdate(req *types.UpdateStepChannelParamMappingRequest) error {
	_, err := l.svcCtx.PipelineRpc.UpdateStepChannelParamMapping(l.ctx, &pipelineconfigservice.UpdateStepChannelParamMappingReq{
		Id:                req.Id,
		ParamName:         req.ParamName,
		GroupCode:         req.GroupCode,
		ChannelTypeFilter: req.ChannelTypeFilter,
		Description:       req.Description,
		SortOrder:         req.SortOrder,
		Status:            req.Status,
		UpdatedBy:         currentUsername(l.ctx),
	})
	return err
}
