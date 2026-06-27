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

type DevopsStepChannelParamMappingListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询步骤参数到渠道分组映射
func NewDevopsStepChannelParamMappingListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsStepChannelParamMappingListLogic {
	return &DevopsStepChannelParamMappingListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsStepChannelParamMappingListLogic) DevopsStepChannelParamMappingList(req *types.StepChannelParamMappingListRequest) (resp *types.StepChannelParamMappingListResponse, err error) {
	result, err := l.svcCtx.PipelineRpc.ListStepChannelParamMapping(l.ctx, &pipelineconfigservice.ListStepChannelParamMappingReq{
		ParamType: req.ParamType,
		GroupCode: req.GroupCode,
		Status:    req.Status,
		Page:      req.Page,
		PageSize:  req.PageSize,
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.StepChannelParamMappingItem, 0, len(result.Items))
	for _, item := range result.Items {
		if item == nil {
			continue
		}
		items = append(items, types.StepChannelParamMappingItem{
			Id:                item.Id,
			ParamType:         item.ParamType,
			ParamName:         item.ParamName,
			GroupCode:         item.GroupCode,
			ChannelTypeFilter: item.ChannelTypeFilter,
			Description:       item.Description,
			SortOrder:         item.SortOrder,
			Status:            item.Status,
			IsSystem:          item.IsSystem,
		})
	}

	return &types.StepChannelParamMappingListResponse{Items: items, Total: result.Total}, nil
}
