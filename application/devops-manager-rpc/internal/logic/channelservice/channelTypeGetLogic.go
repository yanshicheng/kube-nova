package channelservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ChannelTypeGetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewChannelTypeGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChannelTypeGetLogic {
	return &ChannelTypeGetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ChannelTypeGetLogic) ChannelTypeGet(in *pb.GetByIdReq) (*pb.GetChannelTypeResp, error) {
	data, err := l.svcCtx.ChannelTypeModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("渠道类型查询详情失败: %v", err)
		return nil, err
	}

	return &pb.GetChannelTypeResp{Data: channelTypeToPb(data)}, nil
}
