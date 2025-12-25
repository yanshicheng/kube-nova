package alert

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type AddAlertGroupLevelChannelsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAddAlertGroupLevelChannelsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddAlertGroupLevelChannelsLogic {
	return &AddAlertGroupLevelChannelsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddAlertGroupLevelChannelsLogic) AddAlertGroupLevelChannels(req *types.AddAlertGroupLevelChannelsRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	l.Infof("添加告警组级别渠道请求: operator=%s, groupId=%d, severity=%s", username, req.GroupId, req.Severity)

	// 调用 RPC 服务添加告警组级别渠道
	_, err = l.svcCtx.AlertPortalRpc.AlertGroupLevelChannelsAdd(l.ctx, &pb.AddAlertGroupLevelChannelsReq{
		GroupId:   req.GroupId,
		Severity:  req.Severity,
		ChannelId: req.ChannelId,
		CreatedBy: username,
		UpdatedBy: username,
	})
	if err != nil {
		l.Errorf("添加告警组级别渠道失败: operator=%s, groupId=%d, severity=%s, error=%v", username, req.GroupId, req.Severity, err)
		return "", err
	}

	l.Infof("添加告警组级别渠道成功: operator=%s, groupId=%d, severity=%s", username, req.GroupId, req.Severity)
	return "添加告警组级别渠道成功", nil
}
