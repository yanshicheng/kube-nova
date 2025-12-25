package alert

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type BindAlertGroupAppsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBindAlertGroupAppsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BindAlertGroupAppsLogic {
	return &BindAlertGroupAppsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BindAlertGroupAppsLogic) BindAlertGroupApps(req *types.BindAlertGroupAppsRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	l.Infof("绑定告警组应用请求: operator=%s, groupId=%d, appId=%d, appType=%s",
		username, req.GroupId, req.AppId, req.AppType)

	// 调用 RPC 服务绑定告警组应用
	_, err = l.svcCtx.AlertPortalRpc.BindAlertGroupApps(l.ctx, &pb.BindAlertGroupAppsReq{
		GroupId:   req.GroupId,
		AppId:     req.AppId,
		AppType:   req.AppType,
		CreatedBy: username,
		UpdatedBy: username,
	})
	if err != nil {
		l.Errorf("绑定告警组应用失败: operator=%s, groupId=%d, appId=%d, error=%v",
			username, req.GroupId, req.AppId, err)
		return "", err
	}

	l.Infof("绑定告警组应用成功: operator=%s, groupId=%d, appId=%d", username, req.GroupId, req.AppId)
	return "绑定告警组应用成功", nil
}
