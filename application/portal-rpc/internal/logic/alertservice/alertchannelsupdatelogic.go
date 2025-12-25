package alertservicelogic

import (
	"context"
	"encoding/json"
	"time"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/notification"
	notification2 "github.com/yanshicheng/kube-nova/application/portal-rpc/internal/notification"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertChannelsUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertChannelsUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertChannelsUpdateLogic {
	return &AlertChannelsUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AlertChannelsUpdateLogic) AlertChannelsUpdate(in *pb.UpdateAlertChannelsReq) (*pb.UpdateAlertChannelsResp, error) {
	// 查找原有数据
	oldData, err := l.svcCtx.AlertChannelsModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("告警渠道不存在: %v", err)
		return nil, errorx.Msg("告警渠道不存在")
	}

	// 验证 Config 字段
	var config notification.Config
	if err := json.Unmarshal([]byte(in.Config), &config); err != nil {
		l.Errorf("解析告警渠道配置失败: %v", err)
		return nil, errorx.Msg("告警渠道配置格式错误")
	}

	// 设置类型和名称
	config.Type = notification2.AlertType(in.ChannelType)
	config.Name = in.ChannelName
	config.UUID = oldData.Uuid // 保持原有 UUID

	// 验证配置
	if err := notification2.ValidateConfig(config); err != nil {
		l.Errorf("告警渠道配置验证失败: %v", err)
		return nil, errorx.Msg("告警渠道配置验证失败: " + err.Error())
	}

	// 将验证后的配置序列化回 JSON
	configBytes, err := json.Marshal(config)
	if err != nil {
		l.Errorf("序列化告警渠道配置失败: %v", err)
		return nil, errorx.Msg("序列化告警渠道配置失败")
	}

	// 更新数据
	data := &model.AlertChannels{
		Id:          in.Id,
		Uuid:        oldData.Uuid,
		ChannelName: in.ChannelName,
		ChannelType: in.ChannelType,
		Config:      string(configBytes),
		Description: in.Description,
		RetryTimes:  uint64(in.RetryTimes),
		Timeout:     uint64(in.Timeout),
		RateLimit:   uint64(in.RateLimit),
		CreatedBy:   oldData.CreatedBy,
		UpdatedBy:   in.UpdatedBy,
		UpdatedAt:   time.Now(),
		IsDeleted:   oldData.IsDeleted,
	}

	err = l.svcCtx.AlertChannelsModel.Update(l.ctx, data)
	if err != nil {
		l.Errorf("更新告警渠道失败: %v", err)
		return nil, errorx.Msg("更新告警渠道失败")
	}

	return &pb.UpdateAlertChannelsResp{}, nil
}
