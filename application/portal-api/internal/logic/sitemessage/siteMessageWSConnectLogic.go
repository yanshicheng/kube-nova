package sitemessage

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/logic/common/wsutil"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SiteMessageWSConnectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
	ws     *wsutil.WSConnection
}

func NewSiteMessageWSConnectLogic(ctx context.Context, svcCtx *svc.ServiceContext, ws *wsutil.WSConnection) *SiteMessageWSConnectLogic {
	logx.Info("====== [DEBUG] Logic 构造函数被调用 ======")
	return &SiteMessageWSConnectLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
		ws:     ws,
	}
}

func (l *SiteMessageWSConnectLogic) SiteMessageWSConnect(req *types.SiteMessageWSConnectRequest) error {
	logx.Info("====== [DEBUG] Logic.SiteMessageWSConnect 开始执行 ======")

	userId, err := l.getUserIdFromContext()
	if err != nil {
		l.Errorf("获取用户ID失败: %v", err)
		return err
	}

	l.Infof("WebSocket 连接建立: userId=%d", userId)

	l.svcCtx.SiteMessageHub.Register(userId, l.ws)
	defer l.svcCtx.SiteMessageHub.Unregister(userId, l.ws)

	if err := l.sendInitialData(userId); err != nil {
		l.Errorf("发送初始数据失败: userId=%d, error=%v", userId, err)
		return err
	}

	l.handleClientMessages(userId)

	l.Infof("WebSocket 连接已关闭: userId=%d", userId)

	return nil
}

func (l *SiteMessageWSConnectLogic) getUserIdFromContext() (uint64, error) {
	userIdVal := l.ctx.Value("userId")
	if userIdVal == nil {
		return 0, fmt.Errorf("未找到用户ID")
	}

	switch v := userIdVal.(type) {
	case uint64:
		return v, nil
	case int64:
		return uint64(v), nil
	case float64:
		return uint64(v), nil
	case string:
		var userId uint64
		if _, err := fmt.Sscanf(v, "%d", &userId); err != nil {
			return 0, fmt.Errorf("用户ID格式错误: %s", v)
		}
		return userId, nil
	default:
		return 0, fmt.Errorf("未知的用户ID类型: %T", v)
	}
}

func (l *SiteMessageWSConnectLogic) sendInitialData(userId uint64) error {
	resp, err := l.svcCtx.SiteMessagesRpc.GetUserUnreadMessages(l.ctx, &pb.GetUserUnreadMessagesReq{
		UserId: userId,
		Limit:  50,
	})
	if err != nil {
		l.Errorf("获取未读消息失败: userId=%d, error=%v", userId, err)
		resp = &pb.GetUserUnreadMessagesResp{
			Data:  []*pb.SiteMessages{},
			Total: 0,
		}
	}

	messages := make([]types.SiteMessages, 0, len(resp.Data))
	for _, msg := range resp.Data {
		messages = append(messages, types.SiteMessages{
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
			ReadAt:         msg.ReadAt,
			IsStarred:      msg.IsStarred,
			ExpireAt:       msg.ExpireAt,
			CreatedAt:      msg.CreatedAt,
			UpdatedAt:      msg.UpdatedAt,
		})
	}

	initialData := map[string]interface{}{
		"messages": messages,
		"total":    resp.Total,
		"count":    len(messages),
	}

	if err := l.ws.SendMessage("initial", initialData); err != nil {
		return fmt.Errorf("发送初始消息失败: %v", err)
	}

	l.Infof("已发送初始数据: userId=%d, messageCount=%d, total=%d",
		userId, len(messages), resp.Total)

	return nil
}

func (l *SiteMessageWSConnectLogic) handleClientMessages(userId uint64) {
	for {
		if l.ws.IsClosed() {
			l.Infof("WebSocket 连接已关闭: userId=%d", userId)
			return
		}

		var msg wsutil.WSMessage
		if err := l.ws.ReadJSON(&msg); err != nil {
			if !l.ws.IsClosed() && !l.ws.IsClientClosed() {
				l.Errorf("读取消息失败: userId=%d, error=%v", userId, err)
			}
			return
		}

		switch msg.Type {
		case "pong":
			l.Debugf("收到客户端 pong: userId=%d", userId)

		case "ping":
			if err := l.ws.SendMessage("pong", nil); err != nil {
				l.Errorf("发送 pong 失败: userId=%d, error=%v", userId, err)
				return
			}

		default:
			l.Infof("收到客户端消息: userId=%d, type=%s", userId, msg.Type)
		}
	}
}
