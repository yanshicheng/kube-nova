package alertservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertChannelsSearchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertChannelsSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertChannelsSearchLogic {
	return &AlertChannelsSearchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AlertChannelsSearch 搜索告警渠道（不分页）
func (l *AlertChannelsSearchLogic) AlertChannelsSearch(in *pb.SearchAlertChannelsReq) (*pb.SearchAlertChannelsResp, error) {
	// 构建查询条件
	var queryStr string
	var args []interface{}
	conditions := []string{}

	if in.Uuid != "" {
		conditions = append(conditions, "`uuid` = ?")
		args = append(args, in.Uuid)
	}

	if in.ChannelName != "" {
		conditions = append(conditions, "`channel_name` LIKE ?")
		args = append(args, "%"+in.ChannelName+"%")
	}

	if in.ChannelType != "" {
		conditions = append(conditions, "`channel_type` = ?")
		args = append(args, in.ChannelType)
	}

	if len(conditions) > 0 {
		queryStr = joinConditions(conditions, " AND ")
	}

	// 执行不分页搜索
	dataList, err := l.svcCtx.AlertChannelsModel.SearchNoPage(
		l.ctx,
		"id",  // 按id排序
		false, // 降序
		queryStr,
		args...,
	)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Infof("未找到匹配的告警渠道")
			return &pb.SearchAlertChannelsResp{
				Data: make([]*pb.AlertChannels, 0),
			}, nil
		}
		l.Errorf("搜索告警渠道失败: %v", err)
		return nil, errorx.Msg("搜索告警渠道失败")
	}

	// 转换数据
	list := make([]*pb.AlertChannels, 0, len(dataList))
	for _, data := range dataList {
		list = append(list, &pb.AlertChannels{
			Id:          data.Id,
			Uuid:        data.Uuid,
			ChannelName: data.ChannelName,
			ChannelType: data.ChannelType,
			Config:      data.Config,
			Description: data.Description,
			RetryTimes:  int64(data.RetryTimes),
			Timeout:     int64(data.Timeout),
			RateLimit:   int64(data.RateLimit),
			CreatedBy:   data.CreatedBy,
			UpdatedBy:   data.UpdatedBy,
			CreatedAt:   data.CreatedAt.Unix(),
			UpdatedAt:   data.UpdatedAt.Unix(),
		})
	}

	l.Infof("查询到 %d 条告警渠道记录", len(list))
	return &pb.SearchAlertChannelsResp{
		Data: list,
	}, nil
}
