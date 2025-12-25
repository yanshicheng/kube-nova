package sitemessage

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type BatchReadSiteMessagesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBatchReadSiteMessagesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchReadSiteMessagesLogic {
	return &BatchReadSiteMessagesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BatchReadSiteMessagesLogic) BatchReadSiteMessages(req *types.BatchReadSiteMessagesRequest) (resp string, err error) {
	// 获取当前登录用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	userId, ok := l.ctx.Value("userId").(uint64)
	if !ok {
		userId = 0
	}

	l.Infof("批量已读站内消息请求: operator=%s, userId=%d, count=%d", username, userId, len(req.Ids))

	// 调用 RPC 服务批量已读站内消息
	_, err = l.svcCtx.SiteMessagesRpc.SiteMessagesBatchRead(l.ctx, &pb.BatchReadSiteMessagesReq{
		Ids: req.Ids,
	})
	if err != nil {
		l.Errorf("批量已读站内消息失败: operator=%s, error=%v", username, err)
		return "", err
	}

	l.Infof("批量已读站内消息成功: operator=%s, count=%d", username, len(req.Ids))
	return "批量已读站内消息成功", nil
}
