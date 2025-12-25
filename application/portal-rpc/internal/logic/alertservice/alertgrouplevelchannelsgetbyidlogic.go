package alertservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertGroupLevelChannelsGetByIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertGroupLevelChannelsGetByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertGroupLevelChannelsGetByIdLogic {
	return &AlertGroupLevelChannelsGetByIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AlertGroupLevelChannelsGetByIdLogic) AlertGroupLevelChannelsGetById(in *pb.GetAlertGroupLevelChannelsByIdReq) (*pb.GetAlertGroupLevelChannelsByIdResp, error) {
	data, err := l.svcCtx.AlertGroupLevelChannelsModel.FindOne(l.ctx, in.Id)
	if err != nil {
		return nil, errorx.Msg("告警组级别渠道不存在")
	}

	return &pb.GetAlertGroupLevelChannelsByIdResp{
		Data: &pb.AlertGroupLevelChannels{
			Id:        data.Id,
			GroupId:   data.GroupId,
			Severity:  data.Severity,
			ChannelId: data.ChannelId,
			CreatedBy: data.CreatedBy,
			UpdatedBy: data.UpdatedBy,
			CreatedAt: data.CreatedAt.Unix(),
			UpdatedAt: data.UpdatedAt.Unix(),
		},
	}, nil
}
