package alertdashboard

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetAlertTrendLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// NewGetAlertTrendLogic 获取告警趋势分析
func NewGetAlertTrendLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAlertTrendLogic {
	return &GetAlertTrendLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// GetAlertTrend 获取告警趋势分析
// 返回指定天数内每日的新增告警数、恢复告警数、触发中告警数
func (l *GetAlertTrendLogic) GetAlertTrend(req *types.GetAlertTrendRequest) (resp *types.GetAlertTrendResponse, err error) {
	// 构建过滤条件
	filter := &pb.AlertDashboardFilter{
		ClusterUuid: req.ClusterUuid,
		ProjectId:   req.ProjectId,
		WorkspaceId: req.WorkspaceId,
	}

	// 设置默认天数
	days := req.Days
	if days <= 0 {
		days = 7
	}

	// 调用 RPC 服务获取告警趋势
	rpcResp, err := l.svcCtx.ManagerRpc.AlertDashboardTrend(l.ctx, &pb.GetAlertTrendReq{
		Filter: filter,
		Days:   days,
	})
	if err != nil {
		l.Errorf("获取告警趋势分析失败: %v", err)
		return nil, fmt.Errorf("获取告警趋势分析失败")
	}

	// 转换响应
	var dataPoints []types.TrendDataPoint
	for _, point := range rpcResp.DataPoints {
		dataPoints = append(dataPoints, types.TrendDataPoint{
			Date:          point.Date,
			NewCount:      point.NewCount,
			ResolvedCount: point.ResolvedCount,
			FiringCount:   point.FiringCount,
		})
	}

	resp = &types.GetAlertTrendResponse{
		DataPoints:    dataPoints,
		TotalNew:      rpcResp.TotalNew,
		TotalResolved: rpcResp.TotalResolved,
	}

	return resp, nil
}
