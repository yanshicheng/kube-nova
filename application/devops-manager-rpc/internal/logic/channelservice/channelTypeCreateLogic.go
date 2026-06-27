package channelservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ChannelTypeCreateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewChannelTypeCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChannelTypeCreateLogic {
	return &ChannelTypeCreateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ChannelTypeCreateLogic) ChannelTypeCreate(in *pb.CreateChannelTypeReq) (*pb.IdResp, error) {
	if _, err := l.svcCtx.ChannelTypeModel.FindOneByCode(l.ctx, in.Code); err == nil {
		l.Errorf("渠道类型编码已存在")
		return nil, errorx.Msg("渠道类型编码已存在")
	} else if !errors.Is(err, model.ErrNotFound) {
		l.Errorf("渠道类型创建失败: %v", err)
		return nil, err
	}
	mappingFields, err := normalizeChannelTypeMappingFields(in.MappingFields)
	if err != nil {
		l.Errorf("渠道类型映射字段校验失败: %v", err)
		return nil, err
	}
	data := &model.DevopsChannelType{
		Code:             in.Code,
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
		IsSystem:         in.IsSystem,
		Status:           in.Status,
		CreatedBy:        in.CreatedBy,
		UpdatedBy:        in.CreatedBy,
	}
	if data.Status == 0 {
		data.Status = 1
	}
	if err := l.svcCtx.ChannelTypeModel.Insert(l.ctx, data); err != nil {
		if model.IsDuplicateKey(err) {
			l.Errorf("渠道类型编码已存在")
			return nil, errorx.Msg("渠道类型编码已存在")
		}
		l.Errorf("渠道类型创建失败: %v", err)
		return nil, err
	}
	if err := syncGroupAllowedChannelType(l.ctx, l.svcCtx, data.GroupCode, data.Code, data.CreatedBy, true); err != nil {
		l.Errorf("同步渠道分组允许类型失败: %v", err)
		return nil, err
	}

	return &pb.IdResp{Id: data.ID.Hex()}, nil
}
