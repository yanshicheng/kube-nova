package sitemessagesservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type SiteMessagesByIdGetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSiteMessagesByIdGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SiteMessagesByIdGetLogic {
	return &SiteMessagesByIdGetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SiteMessagesByIdGetLogic) SiteMessagesByIdGet(in *pb.GetSiteMessagesByIdReq) (*pb.GetSiteMessagesByIdResp, error) {
	// 参数校验
	if in.Id == 0 {
		return nil, errorx.Msg("消息ID不能为空")
	}

	// 查询站内信
	message, err := l.svcCtx.SiteMessagesModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("查询站内信失败: %v", err)
		return nil, errorx.Msg("查询站内信失败")
	}

	// 转换为 protobuf 格式
	return &pb.GetSiteMessagesByIdResp{
		Data: &pb.SiteMessages{
			Id:             message.Id,
			Uuid:           message.Uuid,
			NotificationId: message.NotificationId,
			InstanceId:     message.InstanceId,
			UserId:         message.UserId,
			Title:          message.Title,
			Content:        message.Content,
			MessageType:    message.MessageType,
			Severity:       message.Severity,
			Category:       message.Category,
			ExtraData:      message.ExtraData,
			ActionUrl:      message.ActionUrl,
			ActionText:     message.ActionText,
			IsRead:         message.IsRead,
			ReadAt:         message.ReadAt.Unix(),
			IsStarred:      message.IsStarred,
			ExpireAt:       message.ExpireAt.Unix(),
			CreatedAt:      message.CreatedAt.Unix(),
			UpdatedAt:      message.UpdatedAt.Unix(),
		},
	}, nil
}
