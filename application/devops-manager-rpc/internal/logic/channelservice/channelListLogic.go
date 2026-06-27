package channelservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ChannelListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewChannelListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChannelListLogic {
	return &ChannelListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ChannelListLogic) ChannelList(in *pb.ListChannelReq) (*pb.ListChannelResp, error) {
	data, total, err := l.svcCtx.ChannelModel.List(l.ctx, model.DevopsChannelListFilter{
		Name:        in.Name,
		Code:        in.Code,
		GroupID:     in.GroupId,
		ChannelType: in.ChannelType,
		Status:      in.Status,
		Page:        in.Page,
		PageSize:    in.PageSize,
	})
	if err != nil {
		l.Errorf("渠道查询列表失败: %v", err)
		return nil, err
	}
	items := make([]*pb.DevopsChannel, 0, len(data))
	for _, item := range data {
		items = append(items, channelToPb(item))
	}

	return &pb.ListChannelResp{Data: items, Total: total}, nil
}
