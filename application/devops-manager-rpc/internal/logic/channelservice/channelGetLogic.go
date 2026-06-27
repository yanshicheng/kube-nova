package channelservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ChannelGetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewChannelGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChannelGetLogic {
	return &ChannelGetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ChannelGetLogic) ChannelGet(in *pb.GetByIdReq) (*pb.GetChannelResp, error) {
	data, err := l.svcCtx.ChannelModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("渠道查询详情失败: %v", err)
		return nil, err
	}

	return &pb.GetChannelResp{Data: channelToPb(data)}, nil
}
