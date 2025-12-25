package alertservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetAlertNotificationsByIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetAlertNotificationsByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAlertNotificationsByIdLogic {
	return &GetAlertNotificationsByIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetAlertNotificationsByIdLogic) GetAlertNotificationsById(in *pb.GetAlertNotificationsByIdReq) (*pb.GetAlertNotificationsByIdResp, error) {
	data, err := l.svcCtx.AlertNotificationsModel.FindOne(l.ctx, in.Id)
	if err != nil {
		return nil, errorx.Msg("告警通知记录不存在")
	}

	return &pb.GetAlertNotificationsByIdResp{
		Data: &pb.AlertNotifications{
			Id:          data.Id,
			Uuid:        data.Uuid,
			InstanceId:  data.InstanceId,
			GroupId:     data.GroupId,
			Severity:    data.Severity,
			ChannelId:   data.ChannelId,
			ChannelType: data.ChannelType,
			SendFormat:  data.SendFormat,
			Recipients:  data.Recipients,
			Subject:     data.Subject,
			Content:     data.Content,
			Status:      data.Status,
			ErrorMsg:    data.ErrorMsg,
			SentAt:      data.SentAt.Unix(),
			Response:    data.Response,
			CostMs:      data.CostMs,
			CreatedBy:   data.CreatedBy,
			UpdatedBy:   data.UpdatedBy,
			CreatedAt:   data.CreatedAt.Unix(),
			UpdatedAt:   data.UpdatedAt.Unix(),
		},
	}, nil
}
