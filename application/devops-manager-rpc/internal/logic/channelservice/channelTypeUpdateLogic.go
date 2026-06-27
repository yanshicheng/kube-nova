package channelservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ChannelTypeUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewChannelTypeUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChannelTypeUpdateLogic {
	return &ChannelTypeUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ChannelTypeUpdateLogic) ChannelTypeUpdate(in *pb.UpdateChannelTypeReq) (*pb.EmptyResp, error) {
	exist, err := l.svcCtx.ChannelTypeModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("渠道类型更新失败: %v", err)
		return nil, err
	}
	mappingFields, err := normalizeChannelTypeMappingFields(in.MappingFields)
	if err != nil {
		l.Errorf("渠道类型映射字段校验失败: %v", err)
		return nil, err
	}
	data := &model.DevopsChannelType{
		ID:               exist.ID,
		Name:             in.Name,
		GroupCode:        in.GroupCode,
		CredentialTypes:  normalizeCredentialTypeList(in.CredentialTypes),
		ConfigSchema:     in.ConfigSchema,
		MappingFields:    mappingFields,
		TestStrategy:     in.TestStrategy,
		Icon:             in.Icon,
		IconColor:        in.IconColor,
		ConnectionMode:   in.ConnectionMode,
		MetadataStrategy: in.MetadataStrategy,
		Status:           in.Status,
		UpdatedBy:        in.UpdatedBy,
	}
	if err := l.svcCtx.ChannelTypeModel.Update(l.ctx, data); err != nil {
		l.Errorf("渠道类型更新失败: %v", err)
		return nil, err
	}
	if exist.GroupCode != data.GroupCode {
		if err := syncGroupAllowedChannelType(l.ctx, l.svcCtx, exist.GroupCode, exist.Code, data.UpdatedBy, false); err != nil {
			l.Errorf("同步渠道分组允许类型失败: %v", err)
			return nil, err
		}
	}
	if err := syncGroupAllowedChannelType(l.ctx, l.svcCtx, data.GroupCode, exist.Code, data.UpdatedBy, true); err != nil {
		l.Errorf("同步渠道分组允许类型失败: %v", err)
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}
