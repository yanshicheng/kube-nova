package alertservicelogic

import (
	"context"
	"time"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertGroupLevelChannelsUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertGroupLevelChannelsUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertGroupLevelChannelsUpdateLogic {
	return &AlertGroupLevelChannelsUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AlertGroupLevelChannelsUpdateLogic) AlertGroupLevelChannelsUpdate(in *pb.UpdateAlertGroupLevelChannelsReq) (*pb.UpdateAlertGroupLevelChannelsResp, error) {
	oldData, err := l.svcCtx.AlertGroupLevelChannelsModel.FindOne(l.ctx, in.Id)
	if err != nil {
		return nil, errorx.Msg("告警组级别渠道不存在")
	}

	data := &model.AlertGroupLevelChannels{
		Id:        in.Id,
		GroupId:   in.GroupId,
		Severity:  in.Severity,
		ChannelId: in.ChannelId,
		CreatedBy: oldData.CreatedBy,
		UpdatedBy: in.UpdatedBy,
		UpdatedAt: time.Now(),
		IsDeleted: oldData.IsDeleted,
	}

	err = l.svcCtx.AlertGroupLevelChannelsModel.Update(l.ctx, data)
	if err != nil {
		return nil, errorx.Msg("更新告警组级别渠道失败")
	}

	return &pb.UpdateAlertGroupLevelChannelsResp{}, nil
}
