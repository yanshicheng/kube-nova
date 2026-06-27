package channelservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ChannelGroupDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewChannelGroupDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChannelGroupDeleteLogic {
	return &ChannelGroupDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ChannelGroupDeleteLogic) ChannelGroupDelete(in *pb.DeleteByIdReq) (*pb.EmptyResp, error) {
	group, err := l.svcCtx.ChannelGroupModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("渠道分组删除失败: %v", err)
		return nil, err
	}
	if group.IsSystem {
		l.Errorf("系统渠道分组不能删除")
		return nil, errorx.Msg("系统渠道分组不能删除")
	}
	total, err := l.svcCtx.ChannelModel.CountByGroup(l.ctx, in.Id)
	if err != nil {
		l.Errorf("渠道分组删除失败: %v", err)
		return nil, err
	}
	if total > 0 {
		l.Errorf("渠道分组下存在渠道，不能删除")
		return nil, errorx.Msg("渠道分组下存在渠道，不能删除")
	}
	if err := l.svcCtx.ChannelGroupModel.DeleteSoft(l.ctx, in.Id, in.UpdatedBy); err != nil {
		l.Errorf("渠道分组删除失败: %v", err)
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}
