package sitemessagesservicelogic

import (
	"context"
	"errors"
	"strings"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type SiteMessagesSearchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSiteMessagesSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SiteMessagesSearchLogic {
	return &SiteMessagesSearchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SiteMessagesSearchLogic) SiteMessagesSearch(in *pb.SearchSiteMessagesReq) (*pb.SearchSiteMessagesResp, error) {
	// 构建查询条件
	var conditions []string
	var args []interface{}

	// UUID 查询
	if in.Uuid != "" {
		conditions = append(conditions, "`uuid` = ?")
		args = append(args, in.Uuid)
	}

	// 标题模糊查询
	if in.Title != "" {
		conditions = append(conditions, "`title` LIKE ?")
		args = append(args, "%"+in.Title+"%")
	}

	// 严重级别查询
	if in.Severity != "" {
		conditions = append(conditions, "`severity` = ?")
		args = append(args, in.Severity)
	}

	// 消息分类查询
	if in.Category != "" {
		conditions = append(conditions, "`category` = ?")
		args = append(args, in.Category)
	}

	// 是否已读查询
	if in.IsRead == 0 || in.IsRead == 1 {
		conditions = append(conditions, "`is_read` = ?")
		args = append(args, in.IsRead)
	}

	// 拼接查询条件
	queryStr := ""
	if len(conditions) > 0 {
		queryStr = strings.Join(conditions, " AND ")
	}

	// 设置默认分页参数
	page := in.Page
	if page < 1 {
		page = 1
	}
	pageSize := in.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	// 查询数据
	messages, total, err := l.svcCtx.SiteMessagesModel.Search(
		l.ctx,
		in.OrderStr,
		in.IsAsc,
		page,
		pageSize,
		queryStr,
		args...,
	)

	if err != nil {
		// 如果是 ErrNotFound，返回空结果而不是错误
		if errors.Is(err, model.ErrNotFound) {
			return &pb.SearchSiteMessagesResp{
				Data:  []*pb.SiteMessages{},
				Total: 0,
			}, nil
		}
		// 其他错误才记录日志并返回
		l.Errorf("搜索站内信失败: %v", err)
		return nil, errorx.Msg("搜索站内信失败")
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

	return &pb.SearchSiteMessagesResp{
		Data:  pbMessages,
		Total: total,
	}, nil
}
