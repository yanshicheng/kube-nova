package alert

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetAlertNotificationsByIdLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetAlertNotificationsByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAlertNotificationsByIdLogic {
	return &GetAlertNotificationsByIdLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetAlertNotificationsByIdLogic) GetAlertNotificationsById(req *types.DefaultIdRequest) (resp *types.AlertNotifications, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	l.Infof("获取告警通知记录请求: operator=%s, id=%d", username, req.Id)

	// 调用 RPC 服务获取告警通知记录
	result, err := l.svcCtx.AlertPortalRpc.GetAlertNotificationsById(l.ctx, &pb.GetAlertNotificationsByIdReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("获取告警通知记录失败: operator=%s, id=%d, error=%v", username, req.Id, err)
		return nil, err
	}

	// 转换数据
	resp = &types.AlertNotifications{
		Id:          result.Data.Id,
		Uuid:        result.Data.Uuid,
		InstanceId:  result.Data.InstanceId,
		GroupId:     result.Data.GroupId,
		Severity:    result.Data.Severity,
		ChannelId:   result.Data.ChannelId,
		ChannelType: result.Data.ChannelType,
		SendFormat:  result.Data.SendFormat,
		Recipients:  result.Data.Recipients,
		Subject:     result.Data.Subject,
		Content:     result.Data.Content,
		Status:      result.Data.Status,
		ErrorMsg:    result.Data.ErrorMsg,
		SentAt:      result.Data.SentAt,
		Response:    result.Data.Response,
		CostMs:      result.Data.CostMs,
		CreatedBy:   result.Data.CreatedBy,
		UpdatedBy:   result.Data.UpdatedBy,
		CreatedAt:   result.Data.CreatedAt,
		UpdatedAt:   result.Data.UpdatedAt,
	}

	l.Infof("获取告警通知记录成功: operator=%s, id=%d", username, req.Id)
	return resp, nil
}
