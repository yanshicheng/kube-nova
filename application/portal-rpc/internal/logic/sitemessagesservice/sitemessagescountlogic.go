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

// SiteMessagesCount 获取某用户所有站内消息数量和已读数量和未读数量
func (l *SiteMessagesCountLogic) SiteMessagesCount(in *pb.GetSiteMessagesCountReq) (*pb.GetSiteMessagesCountResp, error) {
	if in.UserId == 0 {
		return nil, errorx.Msg("用户ID不能为空")
	}

	total, read, unread, err := l.svcCtx.SiteMessagesModel.CountByUserId(l.ctx, in.UserId)
	if err != nil {
		l.Errorf("查询用户消息统计失败: userId=%d, error=%v", in.UserId, err)
		return nil, errorx.Msg("查询消息统计失败")
	}

	l.Infof("用户消息统计: userId=%d, total=%d, read=%d, unread=%d", in.UserId, total, read, unread)

	return &pb.GetSiteMessagesCountResp{
		Total:  uint64(total),
		Read:   uint64(read),
		Unread: uint64(unread),
	}, nil
}
