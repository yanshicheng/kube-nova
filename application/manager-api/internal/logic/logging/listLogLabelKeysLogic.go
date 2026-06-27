package logging

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	logservice "github.com/yanshicheng/kube-nova/application/manager-rpc/client/logservice"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListLogLabelKeysLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListLogLabelKeysLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListLogLabelKeysLogic {
	return &ListLogLabelKeysLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ListLogLabelKeysLogic) ListLogLabelKeys(req *types.LogLabelKeysRequest) (*types.LogLabelKeysResponse, error) {
	rpcResp, err := l.svcCtx.LogRpc.ListLogLabelKeys(l.ctx, &logservice.ListLogLabelKeysReq{
		SourceType:   req.SourceType,
		ClusterUuid:  req.ClusterUuid,
		QueryMode:    req.QueryMode,
		WorkspaceId:  req.WorkspaceId,
		Namespace:    req.Namespace,
		Application:  req.Application,
		ResourceName: req.ResourceName,
		Hosts:        req.Hosts,
	})
	if err != nil {
		return nil, err
	}
	return &types.LogLabelKeysResponse{Keys: rpcResp.Keys}, nil
}
