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

type DeleteLogAlertEventLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除日志告警事件
func NewDeleteLogAlertEventLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteLogAlertEventLogic {
	return &DeleteLogAlertEventLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteLogAlertEventLogic) DeleteLogAlertEvent(req *types.DeleteLogAlertEventRequest) (resp string, err error) {
	operator := getUsername(l.ctx)
	_, err = l.svcCtx.LogRpc.DeleteLogAlertEvent(l.ctx, &logservice.DeleteLogAlertEventReq{
		Id:       req.Id,
		Operator: operator,
	})
	if err != nil {
		return "", err
	}
	return "删除成功", nil
}
