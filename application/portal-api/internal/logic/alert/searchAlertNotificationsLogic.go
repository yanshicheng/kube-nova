package alert

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SearchAlertNotificationsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSearchAlertNotificationsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchAlertNotificationsLogic {
	return &SearchAlertNotificationsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchAlertNotificationsLogic) SearchAlertNotifications(req *types.SearchAlertNotificationsRequest) (resp *types.SearchAlertNotificationsResponse, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	l.Infof("搜索告警通知记录请求: operator=%s, page=%d, pageSize=%d", username, req.Page, req.PageSize)

	// 调用 RPC 服务搜索告警通知记录
	result, err := l.svcCtx.AlertPortalRpc.SearchAlertNotifications(l.ctx, &pb.SearchAlertNotificationsReq{
		Page:        req.Page,
		PageSize:    req.PageSize,
		OrderField:  req.OrderField,
		IsAsc:       req.IsAsc,
		Uuid:        req.Uuid,
		Severity:    req.Severity,
		ChannelId:   req.ChannelId,
		ChannelType: req.ChannelType,
		Subject:     req.Subject,
		Status:      req.Status,
	})
	if err != nil {
		l.Errorf("搜索告警通知记录失败: operator=%s, error=%v", username, err)
		return nil, err
	}

	// 转换数据
	items := make([]types.AlertNotifications, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, types.AlertNotifications{
			Id:          item.Id,
			Uuid:        item.Uuid,
			InstanceId:  item.InstanceId,
			GroupId:     item.GroupId,
			Severity:    item.Severity,
			ChannelId:   item.ChannelId,
			ChannelType: item.ChannelType,
			SendFormat:  item.SendFormat,
			Recipients:  item.Recipients,
			Subject:     item.Subject,
			Content:     item.Content,
			Status:      item.Status,
			ErrorMsg:    item.ErrorMsg,
			SentAt:      item.SentAt,
			Response:    item.Response,
			CostMs:      item.CostMs,
			CreatedBy:   item.CreatedBy,
			UpdatedBy:   item.UpdatedBy,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		})
	}

	resp = &types.SearchAlertNotificationsResponse{
		Items: items,
		Total: result.Total,
	}

	l.Infof("搜索告警通知记录成功: operator=%s, total=%d", username, result.Total)
	return resp, nil
}
