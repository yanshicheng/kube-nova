package executionservicelogic

import (
	"context"
	"time"

	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type PipelineDashboardLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPipelineDashboardLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PipelineDashboardLogic {
	return &PipelineDashboardLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PipelineDashboardLogic) PipelineDashboard(in *pb.PipelineDashboardReq) (*pb.PipelineDashboardResp, error) {
	projectIDs, restricted, err := accessibleProjectIDs(l.ctx, l.svcCtx, in.CurrentUserId, in.CurrentRoles)
	if err != nil {
		l.Errorf("流水线看板失败: %v", err)
		return nil, err
	}
	filter := model.MetricFilter{
		ProjectID:  in.ProjectId,
		ProjectIDs: projectIDs,
		Restricted: restricted,
	}
	_ = backfillUnrecordedPipelineMetrics(l.ctx, l.svcCtx, filter)
	summary, err := l.svcCtx.MetricModel.Summary(l.ctx, filter)
	if err != nil {
		l.Errorf("流水线看板失败: %v", err)
		return nil, err
	}
	metrics, err := l.svcCtx.MetricModel.List(l.ctx, model.MetricFilter{
		ProjectID:  filter.ProjectID,
		ProjectIDs: filter.ProjectIDs,
		Restricted: filter.Restricted,
		Limit:      500,
	})
	if err != nil {
		l.Errorf("流水线看板失败: %v", err)
		return nil, err
	}
	projectRanks, err := l.metricRanks(filter, map[string]string{"projectId": "projectId", "projectName": "projectName", "projectCode": "projectCode"})
	if err != nil {
		l.Errorf("流水线看板失败: %v", err)
		return nil, err
	}
	systemRanks, err := l.metricRanks(filter, map[string]string{"systemId": "systemId", "systemName": "systemName", "systemCode": "systemCode"})
	if err != nil {
		l.Errorf("流水线看板失败: %v", err)
		return nil, err
	}
	pipelineRanks, err := l.metricRanks(filter, map[string]string{"pipelineId": "pipelineId", "pipelineName": "pipelineName", "pipelineCode": "pipelineCode"})
	if err != nil {
		l.Errorf("流水线看板失败: %v", err)
		return nil, err
	}
	environmentRanks, err := l.metricRanks(filter, map[string]string{"environmentId": "environmentId", "environmentName": "environmentName", "environmentCode": "environmentCode"})
	if err != nil {
		l.Errorf("流水线看板失败: %v", err)
		return nil, err
	}
	channelRanks, err := l.metricRanks(filter, map[string]string{"buildChannelBindingId": "buildChannelBindingId", "channelName": "channelName", "channelId": "channelId"})
	if err != nil {
		l.Errorf("流水线看板失败: %v", err)
		return nil, err
	}
	trends, err := l.svcCtx.PipelineRunModel.Trend(l.ctx, filter, 14)
	if err != nil {
		l.Errorf("流水线看板失败: %v", err)
		return nil, err
	}
	return &pb.PipelineDashboardResp{
		Overview:         dashboardOverviewToPb(summary, metrics),
		Trends:           dashboardTrendsToPb(trends),
		ProjectRanks:     projectRanks,
		SystemRanks:      systemRanks,
		PipelineRanks:    pipelineRanks,
		EnvironmentRanks: environmentRanks,
		ChannelRanks:     channelRanks,
	}, nil
}

func (l *PipelineDashboardLogic) metricRanks(filter model.MetricFilter, group map[string]string) ([]*pb.PipelineDashboardMetric, error) {
	items, err := l.svcCtx.MetricModel.GroupRank(l.ctx, filter, group, 8)
	if err != nil {
		l.Errorf("查询指标排行失败: %v", err)
		return nil, err
	}
	result := make([]*pb.PipelineDashboardMetric, 0, len(items))
	for _, item := range items {
		result = append(result, dashboardMetricToPb(item))
	}
	return result, nil
}

func backfillUnrecordedPipelineMetrics(ctx context.Context, svcCtx *svc.ServiceContext, filter model.MetricFilter) error {
	items, _, err := svcCtx.PipelineRunModel.List(ctx, model.DevopsPipelineRunListFilter{
		ProjectID:  filter.ProjectID,
		ProjectIDs: filter.ProjectIDs,
		Restricted: filter.Restricted,
		FinalOnly:  true,
		Unrecorded: true,
		Limit:      200,
	})
	if err != nil {
		logx.Errorf("查询流水线看板数据失败: %v", err)
		return err
	}
	for _, item := range items {
		if err := recordPipelineRunMetric(ctx, svcCtx, item); err != nil {
			logx.WithContext(ctx).Errorf("回填流水线统计失败: runId=%s err=%v", item.ID.Hex(), err)
		}
	}
	return nil
}

func dashboardOverviewToPb(summary *model.DevopsPipelineMetric, metrics []*model.DevopsPipelineMetric) *pb.PipelineDashboardOverview {
	if summary == nil {
		summary = &model.DevopsPipelineMetric{}
	}
	projects := map[string]struct{}{}
	systems := map[string]struct{}{}
	environments := map[string]struct{}{}
	pipelines := map[string]struct{}{}
	channels := map[string]struct{}{}
	for _, item := range metrics {
		if item.ProjectID != "" {
			projects[item.ProjectID] = struct{}{}
		}
		if item.SystemID != "" {
			systems[item.SystemID] = struct{}{}
		}
		if item.EnvironmentID != "" {
			environments[item.EnvironmentID] = struct{}{}
		}
		if item.PipelineID != "" {
			pipelines[item.PipelineID] = struct{}{}
		}
		if item.BuildChannelBindingID != "" {
			channels[item.BuildChannelBindingID] = struct{}{}
		}
	}
	return &pb.PipelineDashboardOverview{
		BuildCount:           summary.BuildCount,
		SuccessCount:         summary.SuccessCount,
		FailureCount:         summary.FailureCount,
		AbortedCount:         summary.AbortedCount,
		TotalDurationSeconds: summary.TotalDurationSeconds,
		SuccessRate:          percent(summary.SuccessCount, summary.BuildCount),
		FailureRate:          percent(summary.FailureCount, summary.BuildCount),
		AvgDurationSeconds:   averageSeconds(summary.TotalDurationSeconds, summary.BuildCount),
		ProjectCount:         int64(len(projects)),
		SystemCount:          int64(len(systems)),
		EnvironmentCount:     int64(len(environments)),
		PipelineCount:        int64(len(pipelines)),
		ChannelCount:         int64(len(channels)),
	}
}

func dashboardMetricToPb(item *model.DevopsPipelineMetric) *pb.PipelineDashboardMetric {
	if item == nil {
		return &pb.PipelineDashboardMetric{}
	}
	return &pb.PipelineDashboardMetric{
		Id:                   firstNonEmpty(item.ProjectID, item.SystemID, item.EnvironmentID, item.PipelineID, item.BuildChannelBindingID),
		Name:                 firstNonEmpty(item.ProjectName, item.SystemName, item.EnvironmentName, item.PipelineName, item.ChannelName),
		Code:                 firstNonEmpty(item.ProjectCode, item.SystemCode, item.EnvironmentCode, item.PipelineCode, item.ChannelID),
		BuildCount:           item.BuildCount,
		SuccessCount:         item.SuccessCount,
		FailureCount:         item.FailureCount,
		AbortedCount:         item.AbortedCount,
		TotalDurationSeconds: item.TotalDurationSeconds,
		SuccessRate:          percent(item.SuccessCount, item.BuildCount),
		FailureRate:          percent(item.FailureCount, item.BuildCount),
		AvgDurationSeconds:   averageSeconds(item.TotalDurationSeconds, item.BuildCount),
		LastRunAt:            formatTime(item.LastRunAt),
	}
}

func dashboardTrendsToPb(items []model.PipelineRunTrend) []*pb.PipelineDashboardTrend {
	result := make([]*pb.PipelineDashboardTrend, 0, len(items))
	byDate := make(map[string]model.PipelineRunTrend, len(items))
	for _, item := range items {
		byDate[item.Date] = item
	}
	for i := 13; i >= 0; i-- {
		date := time.Now().In(shanghaiLocation).AddDate(0, 0, -i).Format("2006-01-02")
		item := byDate[date]
		result = append(result, &pb.PipelineDashboardTrend{
			Date:            date,
			BuildCount:      item.BuildCount,
			SuccessCount:    item.SuccessCount,
			FailureCount:    item.FailureCount,
			DurationSeconds: item.DurationSeconds,
		})
	}
	return result
}

func percent(part, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(part) * 100 / float64(total)
}

func averageSeconds(total, count int64) int64 {
	if count <= 0 {
		return 0
	}
	return total / count
}

func firstNonEmpty(items ...string) string {
	for _, item := range items {
		if item != "" {
			return item
		}
	}
	return ""
}
