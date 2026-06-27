package logging

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	logservice "github.com/yanshicheng/kube-nova/application/manager-rpc/client/logservice"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetLogQueryModeOptionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetLogQueryModeOptionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetLogQueryModeOptionsLogic {
	return &GetLogQueryModeOptionsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *GetLogQueryModeOptionsLogic) GetLogQueryModeOptions(req *types.LogQueryModeOptionsRequest) (*types.LogQueryModeOptionsResponse, error) {
	rpcResp, err := l.svcCtx.LogRpc.GetLogQueryModeOptions(l.ctx, &logservice.LogQueryModeOptionsReq{ClusterUuid: req.ClusterUuid})
	if err != nil {
		return nil, err
	}

	return &types.LogQueryModeOptionsResponse{
		Loki:          rpcResp.Loki,
		Elasticsearch: rpcResp.Elasticsearch,
	}, nil
}
