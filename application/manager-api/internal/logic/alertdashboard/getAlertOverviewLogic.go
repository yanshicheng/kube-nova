package alertdashboard

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetAlertOverviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// NewGetAlertOverviewLogic 获取告警总览统计
func NewGetAlertOverviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAlertOverviewLogic {
	return &GetAlertOverviewLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// GetAlertOverview 获取告警总览统计
// 返回告警总数、触发中数量、已恢复数量、今日新增、今日恢复、平均持续时长等
func (l *GetAlertOverviewLogic) GetAlertOverview(req *types.GetAlertOverviewRequest) (resp *types.GetAlertOverviewResponse, err error) {
	// 构建过滤条件
	filter := &pb.AlertDashboardFilter{
		ClusterUuid: req.ClusterUuid,
		ProjectId:   req.ProjectId,
		WorkspaceId: req.WorkspaceId,
	}

	// 调用 RPC 服务获取告警总览统计
	rpcResp, err := l.svcCtx.ManagerRpc.AlertDashboardOverview(l.ctx, &pb.GetAlertOverviewReq{
		Filter: filter,
	})
	if err != nil {
		l.Errorf("获取告警总览统计失败: %v", err)
		return nil, fmt.Errorf("获取告警总览统计失败")
	}

	// 转换响应
	resp = &types.GetAlertOverviewResponse{
		TotalCount:         rpcResp.TotalCount,
		FiringCount:        rpcResp.FiringCount,
		ResolvedCount:      rpcResp.ResolvedCount,
		TodayNewCount:      rpcResp.TodayNewCount,
		TodayResolvedCount: rpcResp.TodayResolvedCount,
		AvgDuration:        rpcResp.AvgDuration,
		ResolvedRate:       rpcResp.ResolvedRate,
		CompareYesterday:   rpcResp.CompareYesterday,
	}

	return resp, nil
}
