package alertservicelogic

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/notification"
	notification2 "github.com/yanshicheng/kube-nova/application/portal-rpc/internal/notification"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertChannelsAddLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertChannelsAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertChannelsAddLogic {
	return &AlertChannelsAddLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// -----------------------告警渠道配置表-----------------------
func (l *AlertChannelsAddLogic) AlertChannelsAdd(in *pb.AddAlertChannelsReq) (*pb.AddAlertChannelsResp, error) {
	// 验证 Config 字段
	var config notification.Config
	if err := json.Unmarshal([]byte(in.Config), &config); err != nil {
		l.Errorf("解析告警渠道配置失败: %v", err)
		return nil, errorx.Msg("告警渠道配置格式错误")
	}

	// 设置类型和名称
	config.Type = notification2.AlertType(in.ChannelType)
	config.Name = in.ChannelName

	// 验证配置
	if err := notification2.ValidateConfig(config); err != nil {
		l.Errorf("告警渠道配置验证失败: %v", err)
		return nil, errorx.Msg("告警渠道配置验证失败: " + err.Error())
	}

	// 生成UUID
	channelUUID := uuid.New().String()
	config.UUID = channelUUID

	// 将验证后的配置序列化回 JSON
	configBytes, err := json.Marshal(config)
	if err != nil {
		l.Errorf("序列化告警渠道配置失败: %v", err)
		return nil, errorx.Msg("序列化告警渠道配置失败")
	}

	// 构建数据模型
	data := &model.AlertChannels{
		Uuid:        channelUUID,
		ChannelName: in.ChannelName,
		ChannelType: in.ChannelType,
		Config:      string(configBytes),
		Description: in.Description,
		RetryTimes:  uint64(in.RetryTimes),
		Timeout:     uint64(in.Timeout),
		RateLimit:   uint64(in.RateLimit),
		CreatedBy:   in.CreatedBy,
		UpdatedBy:   in.UpdatedBy,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		IsDeleted:   0,
	}

	// 插入数据
	_, err = l.svcCtx.AlertChannelsModel.Insert(l.ctx, data)
	if err != nil {
		l.Errorf("添加告警渠道失败: %v", err)
		return nil, errorx.Msg("添加告警渠道失败")
	}

	return &pb.AddAlertChannelsResp{}, nil
}
