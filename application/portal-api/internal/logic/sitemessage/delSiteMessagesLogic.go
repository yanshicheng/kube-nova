package sitemessage

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type DelSiteMessagesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDelSiteMessagesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DelSiteMessagesLogic {
	return &DelSiteMessagesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DelSiteMessagesLogic) DelSiteMessages(req *types.DelSiteMessagesRequest) (resp string, err error) {
	// 获取当前登录用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	userId, ok := l.ctx.Value("userId").(uint64)
	if !ok {
		userId = 0
	}

	l.Infof("删除站内消息请求: operator=%s, userId=%d, messageId=%d", username, userId, req.Id)

	// 调用 RPC 服务删除站内消息
	_, err = l.svcCtx.SiteMessagesRpc.SiteMessagesDel(l.ctx, &pb.DelSiteMessagesReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("删除站内消息失败: operator=%s, messageId=%d, error=%v", username, req.Id, err)
		return "", err
	}

	l.Infof("删除站内消息成功: operator=%s, messageId=%d", username, req.Id)
	return "删除站内消息成功", nil
}
