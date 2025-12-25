package alert

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type DelAlertNotificationsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDelAlertNotificationsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DelAlertNotificationsLogic {
	return &DelAlertNotificationsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DelAlertNotificationsLogic) DelAlertNotifications(req *types.DefaultIdRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	l.Infof("删除告警通知记录请求: operator=%s, id=%d", username, req.Id)

	// 调用 RPC 服务删除告警通知记录
	_, err = l.svcCtx.AlertPortalRpc.DelAlertNotifications(l.ctx, &pb.DelAlertNotificationsReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("删除告警通知记录失败: operator=%s, id=%d, error=%v", username, req.Id, err)
		return "", err
	}

	l.Infof("删除告警通知记录成功: operator=%s, id=%d", username, req.Id)
	return "删除告警通知记录成功", nil
}
