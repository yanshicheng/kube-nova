package sitemessage

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUnreadCountLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetUnreadCountLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUnreadCountLogic {
	return &GetUnreadCountLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetUnreadCountLogic) GetUnreadCount(req *types.GetUnreadCountRequest) (resp *types.GetUnreadCountResponse, err error) {
	// 获取当前登录用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	userId, ok := l.ctx.Value("userId").(uint64)
	if !ok || userId == 0 {
		l.Errorf("获取当前登录用户信息失败")
		return nil, err
	}

	l.Infof("获取未读消息数量请求: operator=%s, userId=%d", username, userId)

	// 调用 RPC 服务获取消息统计
	rpcResp, err := l.svcCtx.SiteMessagesRpc.SiteMessagesCount(l.ctx, &pb.GetSiteMessagesCountReq{
		UserId: userId,
	})
	if err != nil {
		l.Errorf("获取未读消息数量失败: operator=%s, userId=%d, error=%v", username, userId, err)
		return nil, err
	}

	l.Infof("获取未读消息数量成功: operator=%s, userId=%d, unread=%d", username, userId, rpcResp.Unread)

	return &types.GetUnreadCountResponse{
		Count: rpcResp.Unread,
	}, nil
}
