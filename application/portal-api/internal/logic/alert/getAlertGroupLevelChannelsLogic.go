package alert

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetAlertGroupLevelChannelsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetAlertGroupLevelChannelsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAlertGroupLevelChannelsLogic {
	return &GetAlertGroupLevelChannelsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetAlertGroupLevelChannelsLogic) GetAlertGroupLevelChannels(req *types.DefaultIdRequest) (resp *types.AlertGroupLevelChannels, err error) {
	// todo: add your logic here and delete this line

	return
}
