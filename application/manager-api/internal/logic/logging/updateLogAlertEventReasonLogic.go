// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logging

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	logservice "github.com/yanshicheng/kube-nova/application/manager-rpc/client/logservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateLogAlertEventReasonLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 补充日志告警事件原因
func NewUpdateLogAlertEventReasonLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateLogAlertEventReasonLogic {
	return &UpdateLogAlertEventReasonLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateLogAlertEventReasonLogic) UpdateLogAlertEventReason(req *types.UpdateLogAlertEventReasonRequest) (resp *types.LogAlertEvent, err error) {
	operator := getUsername(l.ctx)
	rpcResp, err := l.svcCtx.LogRpc.UpdateLogAlertEventReason(l.ctx, &logservice.UpdateLogAlertEventReasonReq{
		Id:       req.Id,
		Operator: operator,
		Reason:   req.Reason,
	})
	if err != nil {
		return nil, err
	}
	data := convertLogAlertEvent(rpcResp.Data)
	return &data, nil
}
