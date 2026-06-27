package logservicelogic

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateLogAlertEventReasonLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateLogAlertEventReasonLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateLogAlertEventReasonLogic {
	return &UpdateLogAlertEventReasonLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *UpdateLogAlertEventReasonLogic) UpdateLogAlertEventReason(in *pb.UpdateLogAlertEventReasonReq) (*pb.UpdateLogAlertEventReasonResp, error) {
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return nil, errorx.Msg("原因不能为空")
	}
	operator := strings.TrimSpace(in.Operator)
	if operator == "" {
		operator = "manual-note"
	}

	event, err := l.svcCtx.OnecLogAlertFireEventModel.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, errorx.Msg("日志告警事件不存在")
		}
		return nil, errorx.Msg("补充日志告警原因失败")
	}

	payload, err := parseLogAlertEventPayload(event.PayloadJson)
	if err != nil {
		return nil, errorx.Msg("日志告警事件载荷损坏")
	}
	payload.ManualReason = reason
	payload.ManualReasonBy = operator
	payload.ManualReasonAt = time.Now().Unix()

	event.UpdatedBy = operator
	event.PayloadJson = toEventPayloadJSON(payload)
	if err := l.svcCtx.OnecLogAlertFireEventModel.Update(l.ctx, event); err != nil {
		return nil, errorx.Msg("补充日志告警原因失败")
	}

	builder := NewSearchLogAlertEventsLogic(l.ctx, l.svcCtx)
	data, buildErr := builder.buildEventResponse(event)
	if buildErr != nil {
		return nil, errorx.Msg("补充日志告警原因失败")
	}
	return &pb.UpdateLogAlertEventReasonResp{Data: data}, nil
}
