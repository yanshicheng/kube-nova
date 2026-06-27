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

type DevopsDynamicParamOptionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 动态参数选项
func NewDevopsDynamicParamOptionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsDynamicParamOptionsLogic {
	return &DevopsDynamicParamOptionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsDynamicParamOptionsLogic) DevopsDynamicParamOptions(req *types.DynamicParamOptionsRequest) (resp *types.DynamicParamOptionsResponse, err error) {
	result, err := l.svcCtx.PipelineRpc.DynamicParamOptions(l.ctx, &pipelineconfigservice.DynamicParamOptionsReq{
		ProjectId:        req.ProjectId,
		SystemId:         req.SystemId,
		EnvironmentId:    req.EnvironmentId,
		ParamType:        req.ParamType,
		ChannelBindingId: req.ChannelBindingId,
		ProjectValue:     req.ProjectValue,
		ComponentValue:   req.ComponentValue,
		ConfigTypeId:     req.ConfigTypeId,
		ConfigTypeCode:   req.ConfigTypeCode,
		MappingField:     req.MappingField,
		DependencyValues: req.DependencyValues,
		Keyword:          req.Keyword,
		Page:             req.Page,
		PageSize:         req.PageSize,
		CurrentUserId:    currentUserID(l.ctx),
		CurrentRoles:     currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.DynamicParamOption, 0, len(result.Data))
	for _, item := range result.Data {
		if item == nil {
			continue
		}
		items = append(items, types.DynamicParamOption{Label: item.Label, Value: item.Value, Metadata: item.Metadata})
	}

	return &types.DynamicParamOptionsResponse{Items: items, Total: result.Total}, nil
}
