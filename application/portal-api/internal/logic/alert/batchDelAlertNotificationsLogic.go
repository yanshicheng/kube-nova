package alert

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type BatchDelAlertNotificationsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBatchDelAlertNotificationsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchDelAlertNotificationsLogic {
	return &BatchDelAlertNotificationsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BatchDelAlertNotificationsLogic) BatchDelAlertNotifications(req *types.BatchDelAlertNotificationsRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	l.Infof("批量删除告警通知记录请求: operator=%s, count=%d", username, len(req.Ids))

	// 调用 RPC 服务批量删除告警通知记录
	_, err = l.svcCtx.AlertPortalRpc.BatchDelAlertNotifications(l.ctx, &pb.BatchDelAlertNotificationsReq{
		Ids: req.Ids,
	})
	if err != nil {
		l.Errorf("批量删除告警通知记录失败: operator=%s, error=%v", username, err)
		return "", err
	}

	l.Infof("批量删除告警通知记录成功: operator=%s, count=%d", username, len(req.Ids))
	return "批量删除告警通知记录成功", nil
}
