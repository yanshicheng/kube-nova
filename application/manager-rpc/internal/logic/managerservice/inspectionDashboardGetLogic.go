package managerservicelogic

import (
	"context"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type InspectionDashboardGetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewInspectionDashboardGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InspectionDashboardGetLogic {
	return &InspectionDashboardGetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *InspectionDashboardGetLogic) InspectionDashboardGet(in *pb.InspectionDashboardReq) (*pb.InspectionDashboardResp, error) {
	days := in.Days
	if days <= 0 {
		days = 7
	}
	conditions := make([]string, 0)
	args := make([]any, 0)
	if strings.TrimSpace(in.ClusterUuid) != "" {
		conditions = append(conditions, "`cluster_uuid` = ?")
		args = append(args, strings.TrimSpace(in.ClusterUuid))
	}
	conditions = append(conditions, "`created_at` >= ?")
	args = append(args, time.Now().AddDate(0, 0, -int(days)))

	records, err := l.svcCtx.OnecInspectionRecordModel.SearchNoPage(l.ctx, "id", false, strings.Join(conditions, " AND "), args...)
	if err != nil && err != model.ErrNotFound {
		return nil, errorx.Msg("查询巡检看板失败")
	}
	resp := &pb.InspectionDashboardResp{LatestHealthLevel: "unknown"}
	var scoreTotal float64
	for idx, record := range records {
		resp.TotalRecords++
		scoreTotal += record.Score
		if idx == 0 {
			resp.LatestRecordId = record.Id
			resp.LatestHealthLevel = record.HealthLevel
		}
		switch record.Status {
		case "failed", "partial":
			resp.FailedRecords++
		default:
			resp.SuccessRecords++
		}
		if record.HealthLevel == "critical" {
			resp.CriticalRecords++
		}
	}
	if resp.TotalRecords > 0 {
		resp.AvgScore = scoreTotal / float64(resp.TotalRecords)
	}
	if resp.LatestRecordId > 0 {
		results, err := l.svcCtx.OnecInspectionResultModel.SearchNoPage(l.ctx, "id", true, "`record_id` = ?", resp.LatestRecordId)
		if err != nil && err != model.ErrNotFound {
			return nil, errorx.Msg("查询巡检分类统计失败")
		}
		stats := make(map[string]*pb.InspectionCategoryStat)
		for _, result := range results {
			stat := stats[result.Category]
			if stat == nil {
				stat = &pb.InspectionCategoryStat{Category: result.Category}
				stats[result.Category] = stat
			}
			stat.TotalCount++
			switch result.Status {
			case "critical":
				stat.CriticalCount++
			case "warning":
				stat.WarningCount++
			case "failed":
				stat.FailedCount++
			default:
				stat.SuccessCount++
			}
		}
		for _, stat := range stats {
			resp.CategoryStats = append(resp.CategoryStats, stat)
		}
	}

	return resp, nil
}
