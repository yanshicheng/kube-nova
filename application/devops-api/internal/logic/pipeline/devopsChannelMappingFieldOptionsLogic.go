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

type DevopsChannelMappingFieldOptionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 渠道映射字段选项
func NewDevopsChannelMappingFieldOptionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsChannelMappingFieldOptionsLogic {
	return &DevopsChannelMappingFieldOptionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsChannelMappingFieldOptionsLogic) DevopsChannelMappingFieldOptions(req *types.ChannelMappingFieldOptionsRequest) (resp *types.ChannelMappingFieldOptionsResponse, err error) {
	result, err := l.svcCtx.PipelineRpc.ChannelMappingFieldOptions(l.ctx, &pipelineconfigservice.ChannelMappingFieldOptionsReq{
		ChannelGroupCode: req.ChannelGroupCode,
		ChannelType:      req.ChannelType,
		ChannelTypeId:    req.ChannelTypeId,
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.ChannelMappingFieldOption, 0, len(result.Data))
	for _, item := range result.Data {
		if item == nil {
			continue
		}
		options := make([]types.StepParamOption, 0, len(item.Options))
		for _, option := range item.Options {
			if option == nil {
				continue
			}
			options = append(options, types.StepParamOption{
				Label: option.Label,
				Value: option.Value,
			})
		}
		items = append(items, types.ChannelMappingFieldOption{
			GroupCode:             item.GroupCode,
			Field:                 item.Field,
			Name:                  item.Name,
			Kind:                  item.Kind,
			UiControl:             item.UiControl,
			Provider:              item.Provider,
			Dependencies:          mappingDependenciesToType(item.Dependencies),
			AllOf:                 mappingDependenciesToType(item.AllOf),
			AnyOf:                 mappingDependenciesToType(item.AnyOf),
			AllowManualInput:      item.AllowManualInput,
			AddressBuilder:        item.AddressBuilder,
			Required:              item.Required,
			ChannelTypes:          item.ChannelTypes,
			ValueType:             item.ValueType,
			OutputMode:            item.OutputMode,
			OutputTemplate:        item.OutputTemplate,
			CredentialPolicy:      item.CredentialPolicy,
			FallbackAddressFields: item.FallbackAddressFields,
			ResolveEndpoint:       item.ResolveEndpoint,
			Options:               options,
			SortOrder:             item.SortOrder,
		})
	}

	return &types.ChannelMappingFieldOptionsResponse{Items: items}, nil
}

func mappingDependenciesToType(items []*pipelineconfigservice.ChannelMappingFieldDependency) []types.ChannelMappingFieldDependency {
	result := make([]types.ChannelMappingFieldDependency, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, types.ChannelMappingFieldDependency{
			Field:    item.Field,
			Source:   item.Source,
			Name:     item.Name,
			Required: item.Required,
		})
	}
	return result
}
