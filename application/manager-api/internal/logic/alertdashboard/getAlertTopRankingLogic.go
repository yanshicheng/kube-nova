package alertdashboard

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetAlertTopRankingLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// NewGetAlertTopRankingLogic 获取告警排行榜
func NewGetAlertTopRankingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAlertTopRankingLogic {
	return &GetAlertTopRankingLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// GetAlertTopRanking 获取告警排行榜
// 按集群、项目、工作空间、规则或实例维度获取告警数量排行
func (l *GetAlertTopRankingLogic) GetAlertTopRanking(req *types.GetAlertTopRankingRequest) (resp *types.GetAlertTopRankingResponse, err error) {
	// 构建过滤条件
	filter := &pb.AlertDashboardFilter{
		ClusterUuid: req.ClusterUuid,
		ProjectId:   req.ProjectId,
		WorkspaceId: req.WorkspaceId,
	}

	// 设置默认排行数量
	topN := req.TopN
	if topN <= 0 {
		topN = 10
	}

	// 调用 RPC 服务获取告警排行榜
	rpcResp, err := l.svcCtx.ManagerRpc.AlertDashboardTopRanking(l.ctx, &pb.GetAlertTopRankingReq{
		Filter:      filter,
		RankingType: req.RankingType,
		TopN:        topN,
		Severity:    req.Severity,
		Status:      req.Status,
	})
	if err != nil {
		l.Errorf("获取告警排行榜失败: %v", err)
		return nil, fmt.Errorf("获取告警排行榜失败")
	}

	// 转换响应
	var items []types.RankingItem
	for _, item := range rpcResp.Items {
		items = append(items, types.RankingItem{
			Rank:          item.Rank,
			ItemId:        item.ItemId,
			ItemName:      item.ItemName,
			AlertCount:    item.AlertCount,
			FiringCount:   item.FiringCount,
			CriticalCount: item.CriticalCount,
			WarningCount:  item.WarningCount,
			Percentage:    item.Percentage,
			ExtraInfo:     item.ExtraInfo,
		})
	}

	resp = &types.GetAlertTopRankingResponse{
		Items:       items,
		TotalAlerts: rpcResp.TotalAlerts,
	}

	return resp, nil
}
