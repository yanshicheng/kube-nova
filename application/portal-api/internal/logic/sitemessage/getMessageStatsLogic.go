package sitemessage

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetMessageStatsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetMessageStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMessageStatsLogic {
	return &GetMessageStatsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetMessageStatsLogic) GetMessageStats(req *types.GetMessageStatsRequest) (resp *types.MessageStatsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
