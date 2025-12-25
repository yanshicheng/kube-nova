package alertservicelogic

import (
	"context"
	"errors"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertGroupLevelChannelsSearchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertGroupLevelChannelsSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertGroupLevelChannelsSearchLogic {
	return &AlertGroupLevelChannelsSearchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AlertGroupLevelChannelsSearchLogic) AlertGroupLevelChannelsSearch(in *pb.SearchAlertGroupLevelChannelsReq) (*pb.SearchAlertGroupLevelChannelsResp, error) {
	var queryStr string
	var args []interface{}

	conditions := []string{}

	if in.GroupId != 0 {
		conditions = append(conditions, "`group_id` = ?")
		args = append(args, in.GroupId)
	}

	if in.Severity != "" {
		conditions = append(conditions, "`severity` = ?")
		args = append(args, in.Severity)
	}

	if len(conditions) > 0 {
		queryStr = fmt.Sprintf("%s", joinConditions(conditions, " AND "))
	}

	// 使用 SearchNoPage 不分页查询
	dataList, err := l.svcCtx.AlertGroupLevelChannelsModel.SearchNoPage(
		l.ctx,
		in.OrderStr,
		in.IsAsc,
		queryStr,
		args...,
	)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Infof("未查询到告警组级别渠道: %s", queryStr)
			return &pb.SearchAlertGroupLevelChannelsResp{Data: []*pb.AlertGroupLevelChannels{}}, nil
		}
		return nil, errorx.Msg("搜索告警组级别渠道失败")
	}

	list := make([]*pb.AlertGroupLevelChannels, 0, len(dataList))
	for _, data := range dataList {
		list = append(list, &pb.AlertGroupLevelChannels{
			Id:        data.Id,
			GroupId:   data.GroupId,
			Severity:  data.Severity,
			ChannelId: data.ChannelId,
			CreatedBy: data.CreatedBy,
			UpdatedBy: data.UpdatedBy,
			CreatedAt: data.CreatedAt.Unix(),
			UpdatedAt: data.UpdatedAt.Unix(),
		})
	}

	return &pb.SearchAlertGroupLevelChannelsResp{
		Data: list,
	}, nil
}

func joinConditions(conditions []string, separator string) string {
	result := ""
	for i, cond := range conditions {
		if i > 0 {
			result += separator
		}
		result += cond
	}
	return result
}
