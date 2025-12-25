package sitemessagesservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUserUnreadMessagesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUserUnreadMessagesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserUnreadMessagesLogic {
	return &GetUserUnreadMessagesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetUserUnreadMessages 获取用户未读消息列表（用于 WebSocket 首次连接）
func (l *GetUserUnreadMessagesLogic) GetUserUnreadMessages(in *pb.GetUserUnreadMessagesReq) (*pb.GetUserUnreadMessagesResp, error) {
	// 参数校验
	if in.UserId == 0 {
		return nil, errorx.Msg("用户ID不能为空")
	}

	// 设置默认限制
	limit := in.Limit
	if limit == 0 {
		limit = 50 // 默认返回最近50条未读消息
	}

	// 构建查询条件：用户ID + 未读
	queryStr := "`user_id` = ? AND `is_read` = 0"

	// 查询未读消息（按创建时间倒序）
	messages, err := l.svcCtx.SiteMessagesModel.SearchNoPage(
		l.ctx,
		"created_at", // 按创建时间排序
		false,        // 降序
		queryStr,
		in.UserId,
	)

	if err != nil {
		if err.Error() == "record not found" {
			// 没有未读消息，返回空列表
			return &pb.GetUserUnreadMessagesResp{
				Data:  []*pb.SiteMessages{},
				Total: 0,
			}, nil
		}
		l.Errorf("查询用户未读消息失败: userId=%d, error=%v", in.UserId, err)
		return nil, errorx.Msg("查询未读消息失败")
	}

	// 如果没有消息，返回空列表
	if len(messages) == 0 {
		return &pb.GetUserUnreadMessagesResp{
			Data:  []*pb.SiteMessages{},
			Total: 0,
		}, nil
	}

	// 限制返回数量
	total := uint64(len(messages))
	if uint64(len(messages)) > limit {
		messages = messages[:limit]
	}

	// 转换为 protobuf 格式
	var pbMessages []*pb.SiteMessages
	for _, msg := range messages {
		pbMessages = append(pbMessages, &pb.SiteMessages{
			Id:             msg.Id,
			Uuid:           msg.Uuid,
			NotificationId: msg.NotificationId,
			InstanceId:     msg.InstanceId,
			UserId:         msg.UserId,
			Title:          msg.Title,
			Content:        msg.Content,
			MessageType:    msg.MessageType,
			Severity:       msg.Severity,
			Category:       msg.Category,
			ExtraData:      msg.ExtraData,
			ActionUrl:      msg.ActionUrl,
			ActionText:     msg.ActionText,
			IsRead:         msg.IsRead,
			ReadAt:         msg.ReadAt.Unix(),
			IsStarred:      msg.IsStarred,
			ExpireAt:       msg.ExpireAt.Unix(),
			CreatedAt:      msg.CreatedAt.Unix(),
			UpdatedAt:      msg.UpdatedAt.Unix(),
		})
	}

	l.Infof("获取用户未读消息成功: userId=%d, total=%d, returned=%d",
		in.UserId, total, len(pbMessages))

	return &pb.GetUserUnreadMessagesResp{
		Data:  pbMessages,
		Total: total,
	}, nil
}
