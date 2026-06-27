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

type DevopsStepChannelParamMappingCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建步骤参数到渠道分组映射
func NewDevopsStepChannelParamMappingCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsStepChannelParamMappingCreateLogic {
	return &DevopsStepChannelParamMappingCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsStepChannelParamMappingCreateLogic) DevopsStepChannelParamMappingCreate(req *types.CreateStepChannelParamMappingRequest) (resp *types.IdResponse, err error) {
	result, err := l.svcCtx.PipelineRpc.CreateStepChannelParamMapping(l.ctx, &pipelineconfigservice.CreateStepChannelParamMappingReq{
		ParamType:         req.ParamType,
		ParamName:         req.ParamName,
		GroupCode:         req.GroupCode,
		ChannelTypeFilter: req.ChannelTypeFilter,
		Description:       req.Description,
		SortOrder:         req.SortOrder,
		Status:            req.Status,
		CreatedBy:         currentUsername(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	return &types.IdResponse{Id: result.Id}, nil
}
