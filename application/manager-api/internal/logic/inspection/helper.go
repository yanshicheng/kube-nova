package inspection

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
)

func currentUsername(ctx context.Context) string {
	username, ok := ctx.Value("username").(string)
	if !ok || username == "" {
		return "system"
	}
	return username
}

func enabledFilter(v *bool) int64 {
	if v == nil {
		return -1
	}
	if *v {
		return 1
	}
	return 0
}

func convertTemplate(item *pb.InspectionTemplate) *types.InspectionTemplate {
	if item == nil {
		return nil
	}
	return &types.InspectionTemplate{
		Id:          item.Id,
		Name:        item.Name,
		Code:        item.Code,
		Description: item.Description,
		ScopeType:   item.ScopeType,
		Enabled:     item.Enabled,
		IsBuiltin:   item.IsBuiltin,
		Version:     item.Version,
		ConfigJson:  item.ConfigJson,
		CreatedBy:   item.CreatedBy,
		UpdatedBy:   item.UpdatedBy,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}

func convertGroup(item *pb.InspectionGroup) *types.InspectionGroup {
	if item == nil {
		return nil
	}
	return &types.InspectionGroup{
		Id:          item.Id,
		TemplateId:  item.TemplateId,
		GroupCode:   item.GroupCode,
		GroupName:   item.GroupName,
		Description: item.Description,
		Enabled:     item.Enabled,
		OrderNum:    item.OrderNum,
		ConfigJson:  item.ConfigJson,
		CreatedBy:   item.CreatedBy,
		UpdatedBy:   item.UpdatedBy,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}

func convertItem(item *pb.InspectionItem) *types.InspectionItem {
	if item == nil {
		return nil
	}
	return &types.InspectionItem{
		Id:         item.Id,
		TemplateId: item.TemplateId,
		GroupId:    item.GroupId,
		ItemCode:   item.ItemCode,
		ItemName:   item.ItemName,
		Category:   item.Category,
		CheckType:  item.CheckType,
		TargetType: item.TargetType,
		Severity:   item.Severity,
		Weight:     item.Weight,
		Enabled:    item.Enabled,
		TimeoutSec: item.TimeoutSec,
		OrderNum:   item.OrderNum,
		Promql:     item.Promql,
		Operator:   item.Operator,
		Threshold:  item.Threshold,
		Unit:       item.Unit,
		ConfigJson: item.ConfigJson,
		Advice:     item.Advice,
		CreatedBy:  item.CreatedBy,
		UpdatedBy:  item.UpdatedBy,
		CreatedAt:  item.CreatedAt,
		UpdatedAt:  item.UpdatedAt,
	}
}

func convertTask(item *pb.InspectionTask) *types.InspectionTask {
	if item == nil {
		return nil
	}
	return &types.InspectionTask{
		Id:                           item.Id,
		Name:                         item.Name,
		Description:                  item.Description,
		TemplateId:                   item.TemplateId,
		ScopeType:                    item.ScopeType,
		ClusterUuid:                  item.ClusterUuid,
		ScheduleType:                 item.ScheduleType,
		CronExpr:                     item.CronExpr,
		Enabled:                      item.Enabled,
		MaxConcurrency:               item.MaxConcurrency,
		TimeoutSec:                   item.TimeoutSec,
		LastRunAt:                    item.LastRunAt,
		NextRunAt:                    item.NextRunAt,
		LastRecordId:                 item.LastRecordId,
		LastStatus:                   item.LastStatus,
		LastError:                    item.LastError,
		ConfigJson:                   item.ConfigJson,
		PrometheusEnabled:            item.PrometheusEnabled,
		PrometheusEndpoint:           item.PrometheusEndpoint,
		PrometheusAuthEnabled:        item.PrometheusAuthEnabled,
		PrometheusAuthType:           item.PrometheusAuthType,
		PrometheusUsername:           item.PrometheusUsername,
		PrometheusPassword:           item.PrometheusPassword,
		PrometheusToken:              item.PrometheusToken,
		PrometheusTlsEnabled:         item.PrometheusTlsEnabled,
		PrometheusInsecureSkipVerify: item.PrometheusInsecureSkipVerify,
		PrometheusCaCert:             item.PrometheusCaCert,
		PrometheusClientCert:         item.PrometheusClientCert,
		PrometheusClientKey:          item.PrometheusClientKey,
		CreatedBy:                    item.CreatedBy,
		UpdatedBy:                    item.UpdatedBy,
		CreatedAt:                    item.CreatedAt,
		UpdatedAt:                    item.UpdatedAt,
	}
}

func convertRecord(item *pb.InspectionRecord) *types.InspectionRecord {
	if item == nil {
		return nil
	}
	return &types.InspectionRecord{
		Id:            item.Id,
		RecordNo:      item.RecordNo,
		TaskId:        item.TaskId,
		TemplateId:    item.TemplateId,
		TriggerType:   item.TriggerType,
		ScopeType:     item.ScopeType,
		ClusterUuid:   item.ClusterUuid,
		ClusterName:   item.ClusterName,
		Status:        item.Status,
		Score:         item.Score,
		HealthLevel:   item.HealthLevel,
		TotalCount:    item.TotalCount,
		SuccessCount:  item.SuccessCount,
		WarningCount:  item.WarningCount,
		CriticalCount: item.CriticalCount,
		FailedCount:   item.FailedCount,
		SummaryJson:   item.SummaryJson,
		ReportJson:    item.ReportJson,
		StartedAt:     item.StartedAt,
		FinishedAt:    item.FinishedAt,
		DurationMs:    item.DurationMs,
		ErrorMessage:  item.ErrorMessage,
		CreatedBy:     item.CreatedBy,
		UpdatedBy:     item.UpdatedBy,
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
	}
}

func convertResult(item *pb.InspectionResult) *types.InspectionResult {
	if item == nil {
		return nil
	}
	return &types.InspectionResult{
		Id:         item.Id,
		RecordId:   item.RecordId,
		ItemId:     item.ItemId,
		ItemCode:   item.ItemCode,
		ItemName:   item.ItemName,
		Category:   item.Category,
		TargetType: item.TargetType,
		TargetName: item.TargetName,
		Severity:   item.Severity,
		Status:     item.Status,
		Score:      item.Score,
		Expected:   item.Expected,
		Actual:     item.Actual,
		Value:      item.Value,
		Unit:       item.Unit,
		Message:    item.Message,
		Suggestion: item.Suggestion,
		DetailJson: item.DetailJson,
		CreatedBy:  item.CreatedBy,
		UpdatedBy:  item.UpdatedBy,
		CreatedAt:  item.CreatedAt,
		UpdatedAt:  item.UpdatedAt,
	}
}
