package sitemessage

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SearchSiteMessagesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSearchSiteMessagesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchSiteMessagesLogic {
	return &SearchSiteMessagesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchSiteMessagesLogic) SearchSiteMessages(req *types.SearchSiteMessagesRequest) (resp *types.SearchSiteMessagesResponse, err error) {
	// 获取当前登录用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	userId, ok := l.ctx.Value("userId").(uint64)
	if !ok {
		l.Errorf("获取当前登录用户信息失败")
		return nil, err
	}

	l.Infof("搜索站内消息请求: operator=%s, userId=%d, searchUserId=%d, page=%d, pageSize=%d",
		username, userId, userId, req.Page, req.PageSize)

	// 调用 RPC 服务搜索站内消息
	rpcResp, err := l.svcCtx.SiteMessagesRpc.SiteMessagesSearch(l.ctx, &pb.SearchSiteMessagesReq{
		Page:     req.Page,
		PageSize: req.PageSize,
		OrderStr: req.OrderStr,
		IsAsc:    req.IsAsc,
		Uuid:     req.Uuid,
		Title:    req.Title,
		UserId:   userId,
		Severity: req.Severity,
		Category: req.Category,
		IsRead:   req.IsRead,
	})
	if err != nil {
		l.Errorf("搜索站内消息失败: operator=%s, error=%v", username, err)
		return nil, err
	}

	// 转换为 API 响应格式
	var items []types.SiteMessages
	for _, msg := range rpcResp.Data {
		items = append(items, types.SiteMessages{
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

	return &types.SearchSiteMessagesResponse{
		Items: items,
		Total: rpcResp.Total,
	}, nil
}
