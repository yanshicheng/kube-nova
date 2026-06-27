package pipelineconfigservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateStepChannelParamMappingLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateStepChannelParamMappingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateStepChannelParamMappingLogic {
	return &UpdateStepChannelParamMappingLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateStepChannelParamMappingLogic) UpdateStepChannelParamMapping(in *pb.UpdateStepChannelParamMappingReq) (*pb.EmptyResp, error) {
	data, err := l.svcCtx.StepChannelParamModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("更新步骤渠道参数映射失败: %v", err)
		return nil, err
	}
	if err := validateStepChannelParamTarget(l.ctx, l.svcCtx, in.GroupCode, in.ChannelTypeFilter); err != nil {
		l.Errorf("更新步骤渠道参数映射失败: %v", err)
		return nil, err
	}
	data.ParamName = strings.TrimSpace(in.ParamName)
	data.GroupCode = strings.TrimSpace(in.GroupCode)
	data.ChannelTypeFilter = strings.TrimSpace(in.ChannelTypeFilter)
	data.Description = strings.TrimSpace(in.Description)
	data.SortOrder = in.SortOrder
	data.Status = in.Status
	data.UpdatedBy = in.UpdatedBy
	if err := l.svcCtx.StepChannelParamModel.Update(l.ctx, data); err != nil {
		l.Errorf("更新步骤渠道参数映射失败: %v", err)
		return nil, err
	}
	if err := refreshStepChannelLookup(l.ctx, l.svcCtx); err != nil {
		l.Errorf("刷新步骤渠道参数映射缓存失败: %v", err)
		return nil, err
	}
	return &pb.EmptyResp{}, nil
}
