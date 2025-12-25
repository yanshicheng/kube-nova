package sitemessage

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSiteMessagesByIdLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetSiteMessagesByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSiteMessagesByIdLogic {
	return &GetSiteMessagesByIdLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetSiteMessagesByIdLogic) GetSiteMessagesById(req *types.GetSiteMessagesByIdRequest) (resp *types.GetSiteMessagesByIdResponse, err error) {
	// 获取当前登录用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	userId, ok := l.ctx.Value("userId").(uint64)
	if !ok {
		userId = 0
	}

	l.Infof("查询站内消息详情请求: operator=%s, userId=%d, messageId=%d", username, userId, req.Id)

	// 调用 RPC 服务查询站内消息
	rpcResp, err := l.svcCtx.SiteMessagesRpc.SiteMessagesByIdGet(l.ctx, &pb.GetSiteMessagesByIdReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("查询站内消息详情失败: operator=%s, messageId=%d, error=%v", username, req.Id, err)
		return nil, err
	}

	// 转换为 API 响应格式
	return &types.GetSiteMessagesByIdResponse{
		Data: types.SiteMessages{
			Id:             rpcResp.Data.Id,
			Uuid:           rpcResp.Data.Uuid,
			NotificationId: rpcResp.Data.NotificationId,
			InstanceId:     rpcResp.Data.InstanceId,
			UserId:         rpcResp.Data.UserId,
			Title:          rpcResp.Data.Title,
			Content:        rpcResp.Data.Content,
			MessageType:    rpcResp.Data.MessageType,
			Severity:       rpcResp.Data.Severity,
			Category:       rpcResp.Data.Category,
			ExtraData:      rpcResp.Data.ExtraData,
			ActionUrl:      rpcResp.Data.ActionUrl,
			ActionText:     rpcResp.Data.ActionText,
			IsRead:         rpcResp.Data.IsRead,
			ReadAt:         rpcResp.Data.ReadAt,
			IsStarred:      rpcResp.Data.IsStarred,
			ExpireAt:       rpcResp.Data.ExpireAt,
			CreatedAt:      rpcResp.Data.CreatedAt,
			UpdatedAt:      rpcResp.Data.UpdatedAt,
		},
	}, nil
}
