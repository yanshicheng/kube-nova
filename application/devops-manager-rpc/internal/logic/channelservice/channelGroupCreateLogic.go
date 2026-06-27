package channelservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ChannelGroupCreateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewChannelGroupCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChannelGroupCreateLogic {
	return &ChannelGroupCreateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ChannelGroupCreateLogic) ChannelGroupCreate(in *pb.CreateChannelGroupReq) (*pb.IdResp, error) {
	if err := validateDictionaryCode(in.Code, "分组编码"); err != nil {
		l.Errorf("渠道分组创建失败: %v", err)
		return nil, err
	}
	data := &model.DevopsChannelGroup{
		Name:                in.Name,
		Code:                in.Code,
		Description:         in.Description,
		SortOrder:           in.SortOrder,
		IsSystem:            in.IsSystem,
		Status:              in.Status,
		GroupType:           in.GroupType,
		AllowedChannelTypes: normalizeStringList(in.AllowedChannelTypes),
		Icon:                in.Icon,
		IconColor:           in.IconColor,
		CreatedBy:           in.CreatedBy,
		UpdatedBy:           in.CreatedBy,
	}
	if data.Status == 0 {
		data.Status = 1
	}

	if err := l.svcCtx.ChannelGroupModel.Insert(l.ctx, data); err != nil {
		if model.IsDuplicateKey(err) {
			l.Errorf("渠道分组编码已存在")
			return nil, errorx.Msg("渠道分组编码已存在")
		}
		l.Errorf("渠道分组创建失败: %v", err)
		return nil, err
	}

	return &pb.IdResp{Id: data.ID.Hex()}, nil
}
