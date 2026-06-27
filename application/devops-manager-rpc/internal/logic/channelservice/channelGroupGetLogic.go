package channelservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ChannelGroupGetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewChannelGroupGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChannelGroupGetLogic {
	return &ChannelGroupGetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ChannelGroupGetLogic) ChannelGroupGet(in *pb.GetByIdReq) (*pb.GetChannelGroupResp, error) {
	data, err := l.svcCtx.ChannelGroupModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("渠道分组查询详情失败: %v", err)
		return nil, err
	}

	return &pb.GetChannelGroupResp{Data: channelGroupToPb(data)}, nil
}
