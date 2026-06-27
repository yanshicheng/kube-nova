package logservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteLogAlertEventLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteLogAlertEventLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteLogAlertEventLogic {
	return &DeleteLogAlertEventLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *DeleteLogAlertEventLogic) DeleteLogAlertEvent(in *pb.DeleteLogAlertEventReq) (*pb.DeleteLogAlertEventResp, error) {
	_, err := l.svcCtx.OnecLogAlertFireEventModel.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, errorx.Msg("日志告警事件不存在")
		}
		return nil, errorx.Msg("删除日志告警事件失败")
	}
	if err := l.svcCtx.OnecLogAlertFireEventModel.DeleteSoft(l.ctx, in.Id); err != nil {
		return nil, errorx.Msg("删除日志告警事件失败")
	}
	return &pb.DeleteLogAlertEventResp{}, nil
}
