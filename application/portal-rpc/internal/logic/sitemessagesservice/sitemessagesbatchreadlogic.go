package sitemessagesservicelogic

import (
	"context"
	"time"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type SiteMessagesBatchReadLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSiteMessagesBatchReadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SiteMessagesBatchReadLogic {
	return &SiteMessagesBatchReadLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SiteMessagesBatchRead 批量设置站内信为已读
func (l *SiteMessagesBatchReadLogic) SiteMessagesBatchRead(in *pb.BatchReadSiteMessagesReq) (*pb.BatchReadSiteMessagesResp, error) {
	// 参数校验
	if len(in.Ids) == 0 {
		return nil, errorx.Msg("消息ID列表不能为空")
	}

	var readCount int64 = 0
	var userId uint64 = 0

	// 使用事务批量设置已读
	err := l.svcCtx.SiteMessagesModel.TransCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		for _, id := range in.Ids {
			if id == 0 {
				continue
			}

			// 先查询消息是否存在
			message, err := l.svcCtx.SiteMessagesModel.FindOne(ctx, id)
			if err != nil {
				l.Errorf("查询站内信失败, id=%d, error=%v", id, err)
				continue // 跳过不存在的消息
			}

			// 记录用户ID（用于更新 Redis 计数）
			if userId == 0 {
				userId = message.UserId
			}

			// 如果已经是已读状态，跳过
			if message.IsRead == 1 {
				continue
			}

			// 更新为已读状态
			message.IsRead = 1
			message.ReadAt = time.Now()

			if err := l.svcCtx.SiteMessagesModel.Update(ctx, message); err != nil {
				l.Errorf("批量设置已读失败, id=%d, error=%v", id, err)
				return err
			}

			readCount++
		}
		return nil
	})

	if err != nil {
		return nil, errorx.Msg("批量设置已读失败")
	}

	if readCount > 0 && userId > 0 {
		go func() {
			if err := l.svcCtx.MessagePusher.DecrUnreadCount(context.Background(), userId, readCount); err != nil {
				l.Errorf("更新 Redis 未读计数失败: userId=%d, count=%d, error=%v", userId, readCount, err)
			}
		}()
	}

	l.Infof("批量设置已读成功，共处理 %d 条消息，实际更新 %d 条", len(in.Ids), readCount)
	return &pb.BatchReadSiteMessagesResp{}, nil
}
