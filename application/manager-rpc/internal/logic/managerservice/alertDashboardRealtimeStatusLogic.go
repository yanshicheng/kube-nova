package managerservicelogic

import (
	"context"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertDashboardRealtimeStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertDashboardRealtimeStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertDashboardRealtimeStatusLogic {
	return &AlertDashboardRealtimeStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AlertDashboardRealtimeStatus 获取告警实时状态
func (l *AlertDashboardRealtimeStatusLogic) AlertDashboardRealtimeStatus(in *pb.GetAlertRealtimeStatusReq) (*pb.GetAlertRealtimeStatusResp, error) {
	// 解析过滤条件
	var clusterUuid string
	var projectId, workspaceId uint64

	if in.Filter != nil {
		clusterUuid = in.Filter.ClusterUuid
		projectId = in.Filter.ProjectId
		workspaceId = in.Filter.WorkspaceId
	}

	// 调用 Model 层获取实时状态
	stats, err := l.svcCtx.AlertInstancesModel.GetRealtimeStats(l.ctx, clusterUuid, projectId, workspaceId)
	if err != nil {
		l.Errorf("获取告警实时状态失败: %v", err)
		return nil, errorx.Msg("获取告警实时状态失败")
	}

	return &pb.GetAlertRealtimeStatusResp{
		FiringCount:           stats.FiringCount,
		CriticalFiringCount:   stats.CriticalFiringCount,
		WarningFiringCount:    stats.WarningFiringCount,
		InfoFiringCount:       stats.InfoFiringCount,
		Last5MinNewCount:      stats.Last5MinNewCount,
		Last5MinResolvedCount: stats.Last5MinResolved,
		UpdateTimestamp:       time.Now().Unix(),
	}, nil
}
