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

type ManualRetryLogAlertEventLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 手动补偿重试日志告警事件通知
func NewManualRetryLogAlertEventLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ManualRetryLogAlertEventLogic {
	return &ManualRetryLogAlertEventLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ManualRetryLogAlertEventLogic) ManualRetryLogAlertEvent(req *types.ManualRetryLogAlertEventRequest) (resp *types.ManualRetryLogAlertEventResponse, err error) {
	operator := getUsername(l.ctx)
	rpcResp, err := l.svcCtx.LogRpc.ManualRetryLogAlertEvent(l.ctx, &logservice.ManualRetryLogAlertEventReq{
		Id:       req.Id,
		Operator: operator,
	})
	if err != nil {
		return nil, err
	}
	return &types.ManualRetryLogAlertEventResponse{
		Id:           rpcResp.Id,
		NotifyStatus: rpcResp.NotifyStatus,
		RetryCount:   rpcResp.RetryCount,
		NotifyError:  rpcResp.NotifyError,
		NextRetryAt:  rpcResp.NextRetryAt,
	}, nil
}
