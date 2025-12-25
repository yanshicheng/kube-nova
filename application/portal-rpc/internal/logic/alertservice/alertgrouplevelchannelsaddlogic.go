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

type AlertGroupLevelChannelsAddLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertGroupLevelChannelsAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertGroupLevelChannelsAddLogic {
	return &AlertGroupLevelChannelsAddLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// -----------------------告警组级别渠道关联表-----------------------
func (l *AlertGroupLevelChannelsAddLogic) AlertGroupLevelChannelsAdd(in *pb.AddAlertGroupLevelChannelsReq) (*pb.AddAlertGroupLevelChannelsResp, error) {
	data := &model.AlertGroupLevelChannels{
		GroupId:   in.GroupId,
		Severity:  in.Severity,
		ChannelId: in.ChannelId,
		CreatedBy: in.CreatedBy,
		UpdatedBy: in.UpdatedBy,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		IsDeleted: 0,
	}

	_, err := l.svcCtx.AlertGroupLevelChannelsModel.Insert(l.ctx, data)
	if err != nil {
		return nil, errorx.Msg("添加告警组级别渠道失败")
	}

	return &pb.AddAlertGroupLevelChannelsResp{}, nil
}
