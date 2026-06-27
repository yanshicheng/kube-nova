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

type CloseLogAlertEventLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 手动关闭日志告警事件
func NewCloseLogAlertEventLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CloseLogAlertEventLogic {
	return &CloseLogAlertEventLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CloseLogAlertEventLogic) CloseLogAlertEvent(req *types.CloseLogAlertEventRequest) (resp *types.LogAlertEvent, err error) {
	operator := getUsername(l.ctx)
	rpcResp, err := l.svcCtx.LogRpc.CloseLogAlertEvent(l.ctx, &logservice.CloseLogAlertEventReq{
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
