package alert

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type UnbindAlertGroupAppsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUnbindAlertGroupAppsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnbindAlertGroupAppsLogic {
	return &UnbindAlertGroupAppsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UnbindAlertGroupAppsLogic) UnbindAlertGroupApps(req *types.UnbindAlertGroupAppsRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	l.Infof("解绑告警组应用请求: operator=%s, id=%d", username, req.Id)

	// 调用 RPC 服务解绑告警组应用
	_, err = l.svcCtx.AlertPortalRpc.UnbindAlertGroupApps(l.ctx, &pb.UnbindAlertGroupAppsReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("解绑告警组应用失败: operator=%s, id=%d, error=%v", username, req.Id, err)
		return "", err
	}

	l.Infof("解绑告警组应用成功: operator=%s, id=%d", username, req.Id)
	return "解绑告警组应用成功", nil
}
