package alertservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type DelAlertNotificationsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDelAlertNotificationsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DelAlertNotificationsLogic {
	return &DelAlertNotificationsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// -----------------------告警通知记录表-----------------------
func (l *DelAlertNotificationsLogic) DelAlertNotifications(in *pb.DelAlertNotificationsReq) (*pb.DelAlertNotificationsResp, error) {
	err := l.svcCtx.AlertNotificationsModel.DeleteSoft(l.ctx, in.Id)
	if err != nil {
		return nil, errorx.Msg("删除告警通知记录失败")
	}

	return &pb.DelAlertNotificationsResp{}, nil
}
