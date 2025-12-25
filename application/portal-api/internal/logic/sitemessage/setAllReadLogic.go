package sitemessage

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SetAllReadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSetAllReadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SetAllReadLogic {
	return &SetAllReadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SetAllReadLogic) SetAllRead(req *types.SetAllReadRequest) (resp string, err error) {
	// 获取当前登录用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	targetUserId, ok := l.ctx.Value("userId").(uint64)
	if !ok {
		l.Errorf("获取当前登录用户信息失败")
		return "", fmt.Errorf("获取当前登录用户信息失败")
	}

	// 调用 RPC 服务设置全部已读
	_, err = l.svcCtx.SiteMessagesRpc.SiteMessagesSetAllRead(l.ctx, &pb.SetAllReadReq{
		UserId: targetUserId,
	})
	if err != nil {
		l.Errorf("设置全部已读失败: operator=%s, targetUserId=%d, error=%v", username, targetUserId, err)
		return "", err
	}

	l.Infof("设置全部已读成功: operator=%s, targetUserId=%d", username, targetUserId)
	return "设置全部已读成功", nil
}
