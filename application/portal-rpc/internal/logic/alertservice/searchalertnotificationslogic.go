package alertservicelogic

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type SearchAlertNotificationsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSearchAlertNotificationsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchAlertNotificationsLogic {
	return &SearchAlertNotificationsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SearchAlertNotificationsLogic) SearchAlertNotifications(in *pb.SearchAlertNotificationsReq) (*pb.SearchAlertNotificationsResp, error) {
	var queryStr string
	var args []interface{}

	conditions := []string{}

	if in.Uuid != "" {
		conditions = append(conditions, "`uuid` = ?")
		args = append(args, in.Uuid)
	}

	if in.Severity != "" {
		conditions = append(conditions, "`severity` = ?")
		args = append(args, in.Severity)
	}

	if in.ChannelId != 0 {
		conditions = append(conditions, "`channel_id` = ?")
		args = append(args, in.ChannelId)
	}

	if in.ChannelType != "" {
		conditions = append(conditions, "`channel_type` = ?")
		args = append(args, in.ChannelType)
	}

	if in.Subject != "" {
		conditions = append(conditions, "`subject` LIKE ?")
		args = append(args, "%"+in.Subject+"%")
	}

	if in.Status != "" {
		conditions = append(conditions, "`status` = ?")
		args = append(args, in.Status)
	}

	if len(conditions) > 0 {
		queryStr = fmt.Sprintf("%s", joinConditions(conditions, " AND "))
	}

	dataList, total, err := l.svcCtx.AlertNotificationsModel.Search(
		l.ctx,
		in.OrderField,
		in.IsAsc,
		in.Page,
		in.PageSize,
		queryStr,
		args...,
	)
	if err != nil {
		return nil, errorx.Msg("搜索告警通知记录失败")
	}

	list := make([]*pb.AlertNotifications, 0, len(dataList))
	for _, data := range dataList {
		list = append(list, &pb.AlertNotifications{
			Id:          data.Id,
			Uuid:        data.Uuid,
			InstanceId:  data.InstanceId,
			GroupId:     data.GroupId,
			Severity:    data.Severity,
			ChannelId:   data.ChannelId,
			ChannelType: data.ChannelType,
			SendFormat:  data.SendFormat,
			Recipients:  data.Recipients,
			Subject:     data.Subject,
			Content:     data.Content,
			Status:      data.Status,
			ErrorMsg:    data.ErrorMsg,
			SentAt:      data.SentAt.Unix(),
			Response:    data.Response,
			CostMs:      data.CostMs,
			CreatedBy:   data.CreatedBy,
			UpdatedBy:   data.UpdatedBy,
			CreatedAt:   data.CreatedAt.Unix(),
			UpdatedAt:   data.UpdatedAt.Unix(),
		})
	}

	return &pb.SearchAlertNotificationsResp{
		Data:  list,
		Total: total,
	}, nil
}
