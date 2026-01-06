package alertdashboard

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetAlertRealtimeStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// NewGetAlertRealtimeStatusLogic 获取告警实时状态
func NewGetAlertRealtimeStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAlertRealtimeStatusLogic {
	return &GetAlertRealtimeStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// GetAlertRealtimeStatus 获取告警实时状态
// 返回当前触发中的告警数量和最近5分钟的变化情况，用于前端实时刷新
func (l *GetAlertRealtimeStatusLogic) GetAlertRealtimeStatus(req *types.GetAlertRealtimeStatusRequest) (resp *types.GetAlertRealtimeStatusResponse, err error) {
	// 构建过滤条件
	filter := &pb.AlertDashboardFilter{
		ClusterUuid: req.ClusterUuid,
		ProjectId:   req.ProjectId,
		WorkspaceId: req.WorkspaceId,
	}

	// 调用 RPC 服务获取告警实时状态
	rpcResp, err := l.svcCtx.ManagerRpc.AlertDashboardRealtimeStatus(l.ctx, &pb.GetAlertRealtimeStatusReq{
		Filter: filter,
	})
	if err != nil {
		l.Errorf("获取告警实时状态失败: %v", err)
		return nil, fmt.Errorf("获取告警实时状态失败")
	}

	// 转换响应
	resp = &types.GetAlertRealtimeStatusResponse{
		FiringCount:         rpcResp.FiringCount,
		CriticalFiringCount: rpcResp.CriticalFiringCount,
		WarningFiringCount:  rpcResp.WarningFiringCount,
		InfoFiringCount:     rpcResp.InfoFiringCount,
		Last5MinNewCount:    rpcResp.Last5MinNewCount,
		Last5MinResolved:    rpcResp.Last5MinResolvedCount,
		UpdateTimestamp:     rpcResp.UpdateTimestamp,
	}

	return resp, nil
}
