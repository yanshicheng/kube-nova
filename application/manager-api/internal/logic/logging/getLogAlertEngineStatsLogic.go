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

type GetLogAlertEngineStatsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询日志告警引擎统计面板
func NewGetLogAlertEngineStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetLogAlertEngineStatsLogic {
	return &GetLogAlertEngineStatsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetLogAlertEngineStatsLogic) GetLogAlertEngineStats(req *types.LogAlertEngineStatsRequest) (resp *types.LogAlertEngineStatsResponse, err error) {
	rpcResp, err := l.svcCtx.LogRpc.GetLogAlertEngineStats(l.ctx, &logservice.GetLogAlertEngineStatsReq{
		ClusterUuid: req.ClusterUuid,
	})
	if err != nil {
		return nil, err
	}
	return &types.LogAlertEngineStatsResponse{
		ClusterUuid:        rpcResp.ClusterUuid,
		TotalRules:         rpcResp.TotalRules,
		EnabledRules:       rpcResp.EnabledRules,
		DisabledRules:      rpcResp.DisabledRules,
		SuccessRules:       rpcResp.SuccessRules,
		FailedRules:        rpcResp.FailedRules,
		PendingRules:       rpcResp.PendingRules,
		ActiveFiringEvents: rpcResp.ActiveFiringEvents,
		FailedNotifyEvents: rpcResp.FailedNotifyEvents,
		DeadNotifyEvents:   rpcResp.DeadNotifyEvents,
		CriticalRules:      rpcResp.CriticalRules,
		WarningRules:       rpcResp.WarningRules,
		InfoRules:          rpcResp.InfoRules,
		HotRules:           rpcResp.HotRules,
		WarmRules:          rpcResp.WarmRules,
		ColdRules:          rpcResp.ColdRules,
	}, nil
}
