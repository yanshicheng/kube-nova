// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logging

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	logservice "github.com/yanshicheng/kube-nova/application/manager-rpc/client/logservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetLogClusterSyncStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询集群日志规则同步状态
func NewGetLogClusterSyncStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetLogClusterSyncStatusLogic {
	return &GetLogClusterSyncStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetLogClusterSyncStatusLogic) GetLogClusterSyncStatus(req *types.LogClusterSyncStatusRequest) (resp *types.LogClusterSyncStatusResponse, err error) {
	rpcResp, err := l.svcCtx.LogRpc.GetLogClusterSyncStatus(l.ctx, &logservice.GetLogClusterSyncStatusReq{
		ClusterUuid: req.ClusterUuid,
	})
	if err != nil {
		return nil, err
	}

	return &types.LogClusterSyncStatusResponse{
		ClusterUuid:   rpcResp.ClusterUuid,
		TotalRules:    rpcResp.TotalRules,
		EnabledRules:  rpcResp.EnabledRules,
		SuccessRules:  rpcResp.SuccessRules,
		FailedRules:   rpcResp.FailedRules,
		PendingRules:  rpcResp.PendingRules,
		DisabledRules: rpcResp.DisabledRules,
		HealthStatus:  rpcResp.HealthStatus,
		LastSyncError: rpcResp.LastSyncError,
		LastSyncAt:    rpcResp.LastSyncAt,
	}, nil
}
