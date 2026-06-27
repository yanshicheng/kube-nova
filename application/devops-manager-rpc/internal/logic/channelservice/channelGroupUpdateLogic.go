package channelservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ChannelGroupUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewChannelGroupUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChannelGroupUpdateLogic {
	return &ChannelGroupUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ChannelGroupUpdateLogic) ChannelGroupUpdate(in *pb.UpdateChannelGroupReq) (*pb.EmptyResp, error) {
	oid, err := model.ObjectIDForUpdate(in.Id)
	if err != nil {
		l.Errorf("渠道分组更新失败: %v", err)
		return nil, err
	}
	data := &model.DevopsChannelGroup{
		ID:                  oid,
		Name:                in.Name,
		Description:         in.Description,
		SortOrder:           in.SortOrder,
		Status:              in.Status,
		GroupType:           in.GroupType,
		AllowedChannelTypes: normalizeStringList(in.AllowedChannelTypes),
		Icon:                in.Icon,
		IconColor:           in.IconColor,
		UpdatedBy:           in.UpdatedBy,
	}

	if err := l.svcCtx.ChannelGroupModel.Update(l.ctx, data); err != nil {
		if model.IsDuplicateKey(err) {
			l.Errorf("渠道分组编码已存在")
			return nil, errorx.Msg("渠道分组编码已存在")
		}
		l.Errorf("渠道分组更新失败: %v", err)
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}
