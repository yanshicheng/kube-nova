package alertservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertChannelsGetByIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertChannelsGetByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertChannelsGetByIdLogic {
	return &AlertChannelsGetByIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AlertChannelsGetByIdLogic) AlertChannelsGetById(in *pb.GetAlertChannelsByIdReq) (*pb.GetAlertChannelsByIdResp, error) {
	data, err := l.svcCtx.AlertChannelsModel.FindOne(l.ctx, in.Id)
	if err != nil {
		logx.Error(err)
		return nil, errorx.Msg("告警渠道不存在")
	}

	return &pb.GetAlertChannelsByIdResp{
		Data: &pb.AlertChannels{
			Id:          data.Id,
			Uuid:        data.Uuid,
			ChannelName: data.ChannelName,
			ChannelType: data.ChannelType,
			Config:      data.Config,
			Description: data.Description,
			RetryTimes:  int64(data.RetryTimes),
			Timeout:     int64(data.Timeout),
			RateLimit:   int64(data.RateLimit),
			CreatedBy:   data.CreatedBy,
			UpdatedBy:   data.UpdatedBy,
			CreatedAt:   data.CreatedAt.Unix(),
			UpdatedAt:   data.UpdatedAt.Unix(),
		},
	}, nil
}
