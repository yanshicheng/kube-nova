package alertservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertGroupLevelChannelsDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertGroupLevelChannelsDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertGroupLevelChannelsDelLogic {
	return &AlertGroupLevelChannelsDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AlertGroupLevelChannelsDelLogic) AlertGroupLevelChannelsDel(in *pb.DelAlertGroupLevelChannelsReq) (*pb.DelAlertGroupLevelChannelsResp, error) {
	err := l.svcCtx.AlertGroupLevelChannelsModel.Delete(l.ctx, in.Id)
	if err != nil {
		return nil, errorx.Msg("删除告警组级别渠道失败")
	}

	return &pb.DelAlertGroupLevelChannelsResp{}, nil
}
