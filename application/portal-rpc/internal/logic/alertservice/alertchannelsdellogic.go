package alertservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertChannelsDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertChannelsDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertChannelsDelLogic {
	return &AlertChannelsDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AlertChannelsDelLogic) AlertChannelsDel(in *pb.DelAlertChannelsReq) (*pb.DelAlertChannelsResp, error) {
	err := l.svcCtx.AlertChannelsModel.Delete(l.ctx, in.Id)
	if err != nil {
		l.Errorf("删除告警渠道失败: %v", err)
		return nil, errorx.Msg("删除告警渠道失败")
	}

	return &pb.DelAlertChannelsResp{}, nil
}
