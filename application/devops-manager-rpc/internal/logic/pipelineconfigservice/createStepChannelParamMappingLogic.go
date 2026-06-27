package pipelineconfigservicelogic

import (
	"context"
	"errors"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateStepChannelParamMappingLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateStepChannelParamMappingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateStepChannelParamMappingLogic {
	return &CreateStepChannelParamMappingLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateStepChannelParamMappingLogic) CreateStepChannelParamMapping(in *pb.CreateStepChannelParamMappingReq) (*pb.IdResp, error) {
	paramType := strings.TrimSpace(in.ParamType)
	groupCode := strings.TrimSpace(in.GroupCode)
	channelTypeFilter := strings.TrimSpace(in.ChannelTypeFilter)
	if total, err := l.svcCtx.StepChannelParamModel.CountByTarget(l.ctx, paramType, groupCode, channelTypeFilter); err != nil {
		l.Errorf("创建步骤渠道参数映射失败: %v", err)
		return nil, err
	} else if total > 0 {
		l.Errorf("步骤参数类型在当前渠道类型下已存在")
		return nil, errorx.Msg("步骤参数类型在当前渠道类型下已存在")
	}
	if err := validateStepChannelParamTarget(l.ctx, l.svcCtx, groupCode, channelTypeFilter); err != nil {
		l.Errorf("创建步骤渠道参数映射失败: %v", err)
		return nil, err
	}
	data := &model.DevopsStepChannelParam{
		ParamType:         paramType,
		ParamName:         strings.TrimSpace(in.ParamName),
		GroupCode:         groupCode,
		ChannelTypeFilter: channelTypeFilter,
		Description:       strings.TrimSpace(in.Description),
		SortOrder:         in.SortOrder,
		Status:            in.Status,
		CreatedBy:         in.CreatedBy,
		UpdatedBy:         in.CreatedBy,
	}
	if data.Status == 0 {
		data.Status = 1
	}
	if err := l.svcCtx.StepChannelParamModel.Insert(l.ctx, data); err != nil {
		l.Errorf("创建步骤渠道参数映射失败: %v", err)
		return nil, err
	}
	if err := refreshStepChannelLookup(l.ctx, l.svcCtx); err != nil {
		l.Errorf("刷新步骤渠道参数映射缓存失败: %v", err)
		return nil, err
	}
	return &pb.IdResp{Id: data.ID.Hex()}, nil
}

func validateStepChannelParamTarget(ctx context.Context, svcCtx *svc.ServiceContext, groupCode, channelType string) error {
	groupCode = strings.TrimSpace(groupCode)
	group, err := svcCtx.ChannelGroupModel.FindOneByCode(ctx, groupCode)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return errorx.Msg("渠道分组不存在")
		}
		return err
	}
	if group.Status != 1 {
		return errorx.Msg("渠道分组已停用")
	}
	channelType = strings.TrimSpace(channelType)
	if channelType == "" {
		return nil
	}
	typeData, err := svcCtx.ChannelTypeModel.FindOneByCode(ctx, channelType)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return errorx.Msg("渠道类型不存在")
		}
		return err
	}
	if typeData.Status != 1 {
		return errorx.Msg("渠道类型已停用")
	}
	if typeData.GroupCode != groupCode {
		return errorx.Msg("渠道类型与渠道分组不匹配")
	}
	return nil
}

func refreshStepChannelLookup(ctx context.Context, svcCtx *svc.ServiceContext) error {
	if svcCtx == nil || svcCtx.ChannelParamLookup == nil {
		return nil
	}
	return svcCtx.ChannelParamLookup.Refresh(ctx)
}
