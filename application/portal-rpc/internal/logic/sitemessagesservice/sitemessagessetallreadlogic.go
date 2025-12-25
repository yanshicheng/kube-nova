package sitemessagesservicelogic

import (
	"context"
	"fmt"
	"time"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type SiteMessagesSetAllReadLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSiteMessagesSetAllReadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SiteMessagesSetAllReadLogic {
	return &SiteMessagesSetAllReadLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SiteMessagesSetAllRead 设置用户的全部未读消息为已读
func (l *SiteMessagesSetAllReadLogic) SiteMessagesSetAllRead(in *pb.SetAllReadReq) (*pb.SetAllReadResp, error) {
	// 参数校验
	if in.UserId == 0 {
		return nil, errorx.Msg("用户ID不能为空")
	}

	queryStr := "`user_id` = ? AND `is_read` = 0"
	unreadMessages, err := l.svcCtx.SiteMessagesModel.SearchNoPage(
		l.ctx,
		"",
		false,
		queryStr,
		in.UserId,
	)

	if err != nil {
		if err.Error() == "record not found" {
			return &pb.SetAllReadResp{}, nil
		}
		l.Errorf("查询未读消息失败: %v", err)
		return nil, errorx.Msg("查询未读消息失败")
	}

	// 如果没有未读消息，直接返回
	if len(unreadMessages) == 0 {
		return &pb.SetAllReadResp{}, nil
	}

	sqlStr := "UPDATE {table} SET `is_read` = 1, `read_at` = ? WHERE `user_id` = ? AND `is_read` = 0 AND `is_deleted` = 0"

	_, err = l.svcCtx.SiteMessagesModel.ExecSql(
		l.ctx,
		0,
		sqlStr,
		time.Now(),
		in.UserId,
	)

	if err != nil {
		l.Errorf("设置全部已读失败: %v", err)
		return nil, errorx.Msg("设置全部已读失败")
	}

	// 批量清理受影响记录的缓存
	go func() {
		for _, msg := range unreadMessages {
			if _, err := l.svcCtx.Cache.DelCtx(l.ctx,
				fmt.Sprintf("cache:ikubeops:siteMessages:id:%d", msg.Id),
				fmt.Sprintf("cache:ikubeops:siteMessages:uuid:%s", msg.Uuid),
			); err != nil {
				l.Errorf("清理缓存失败: id=%d, uuid=%s, error=%v", msg.Id, msg.Uuid, err)
			}
		}
	}()

	l.Infof("用户 %d 设置全部已读成功，共影响 %d 条消息", in.UserId, len(unreadMessages))

	return &pb.SetAllReadResp{}, nil
}
