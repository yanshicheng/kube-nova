package sitemessagesservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type SiteMessagesCountLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSiteMessagesCountLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SiteMessagesCountLogic {
	return &SiteMessagesCountLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SiteMessagesCount 获取某用户所有站内消息数量和 已读数量 和 未读数量
func (l *SiteMessagesCountLogic) SiteMessagesCount(in *pb.GetSiteMessagesCountReq) (*pb.GetSiteMessagesCountResp, error) {
	// 参数校验
	if in.UserId == 0 {
		return nil, errorx.Msg("用户ID不能为空")
	}

	// 查询总数（未删除的消息）
	queryTotal := "`user_id` = ?"
	totalMessages, err := l.svcCtx.SiteMessagesModel.SearchNoPage(
		l.ctx,
		"",
		false,
		queryTotal,
		in.UserId,
	)
	if err != nil && err.Error() != "record not found" {
		l.Errorf("查询用户消息总数失败: userId=%d, error=%v", in.UserId, err)
		return nil, errorx.Msg("查询消息统计失败")
	}

	total := uint64(len(totalMessages))

	// 查询已读数量
	queryRead := "`user_id` = ? AND `is_read` = 1"
	readMessages, err := l.svcCtx.SiteMessagesModel.SearchNoPage(
		l.ctx,
		"",
		false,
		queryRead,
		in.UserId,
	)
	if err != nil && err.Error() != "record not found" {
		l.Errorf("查询用户已读消息数失败: userId=%d, error=%v", in.UserId, err)
		return nil, errorx.Msg("查询消息统计失败")
	}

	read := uint64(len(readMessages))

	// 查询未读数量
	queryUnread := "`user_id` = ? AND `is_read` = 0"
	unreadMessages, err := l.svcCtx.SiteMessagesModel.SearchNoPage(
		l.ctx,
		"",
		false,
		queryUnread,
		in.UserId,
	)
	if err != nil && err.Error() != "record not found" {
		l.Errorf("查询用户未读消息数失败: userId=%d, error=%v", in.UserId, err)
		return nil, errorx.Msg("查询消息统计失败")
	}

	unread := uint64(len(unreadMessages))

	l.Infof("用户消息统计: userId=%d, total=%d, read=%d, unread=%d", in.UserId, total, read, unread)

	return &pb.GetSiteMessagesCountResp{
		Total:  total,
		Read:   read,
		Unread: unread,
	}, nil
}
