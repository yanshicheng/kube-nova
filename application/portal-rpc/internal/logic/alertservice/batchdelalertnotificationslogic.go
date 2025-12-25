package alertservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type BatchDelAlertNotificationsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBatchDelAlertNotificationsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchDelAlertNotificationsLogic {
	return &BatchDelAlertNotificationsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 批量删除
func (l *BatchDelAlertNotificationsLogic) BatchDelAlertNotifications(in *pb.BatchDelAlertNotificationsReq) (*pb.BatchDelAlertNotificationsResp, error) {
	for _, id := range in.Ids {
		err := l.svcCtx.AlertNotificationsModel.Delete(l.ctx, id)
		if err != nil {
			return nil, errorx.Msg("批量删除告警通知记录失败")
		}
	}

	return &pb.BatchDelAlertNotificationsResp{}, nil
}
