// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package inspection

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetInspectionDashboardLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询巡检数据大盘
func NewGetInspectionDashboardLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetInspectionDashboardLogic {
	return &GetInspectionDashboardLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetInspectionDashboardLogic) GetInspectionDashboard(req *types.InspectionDashboardRequest) (resp *types.InspectionDashboardResponse, err error) {
	rpcResp, err := l.svcCtx.ManagerRpc.InspectionDashboardGet(l.ctx, &pb.InspectionDashboardReq{
		ClusterUuid: req.ClusterUuid,
		Days:        req.Days,
	})
	if err != nil {
		return nil, err
	}
	resp = &types.InspectionDashboardResponse{
		TotalRecords:      rpcResp.TotalRecords,
		SuccessRecords:    rpcResp.SuccessRecords,
		FailedRecords:     rpcResp.FailedRecords,
		CriticalRecords:   rpcResp.CriticalRecords,
		AvgScore:          rpcResp.AvgScore,
		LatestHealthLevel: rpcResp.LatestHealthLevel,
		LatestRecordId:    rpcResp.LatestRecordId,
		CategoryStats:     make([]types.InspectionCategoryStat, 0, len(rpcResp.CategoryStats)),
	}
	for _, item := range rpcResp.CategoryStats {
		resp.CategoryStats = append(resp.CategoryStats, types.InspectionCategoryStat{
			Category:      item.Category,
			TotalCount:    item.TotalCount,
			SuccessCount:  item.SuccessCount,
			WarningCount:  item.WarningCount,
			CriticalCount: item.CriticalCount,
			FailedCount:   item.FailedCount,
		})
	}

	return resp, nil
}
