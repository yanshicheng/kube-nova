package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListStepChannelParamMappingLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListStepChannelParamMappingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListStepChannelParamMappingLogic {
	return &ListStepChannelParamMappingLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListStepChannelParamMapping 查询步骤参数到渠道分组的映射列表，供前端动态获取。
func (l *ListStepChannelParamMappingLogic) ListStepChannelParamMapping(in *pb.ListStepChannelParamMappingReq) (*pb.ListStepChannelParamMappingResp, error) {
	page := uint64(in.Page)
	if page == 0 {
		page = 1
	}
	pageSize := uint64(in.PageSize)
	if pageSize == 0 {
		pageSize = 200
	}
	status := in.Status
	if status == 0 {
		status = 1
	}

	items, total, err := l.svcCtx.StepChannelParamModel.List(l.ctx, model.DevopsStepChannelParamListFilter{
		ParamType: in.ParamType,
		GroupCode: in.GroupCode,
		Status:    status,
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		l.Errorf("查询步骤参数映射失败: %v", err)
		return nil, err
	}

	result := make([]*pb.StepChannelParamMappingItem, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, &pb.StepChannelParamMappingItem{
			Id:                item.ID.Hex(),
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

	return &pb.ListStepChannelParamMappingResp{
		Items: result,
		Total: int64(total),
	}, nil
}
