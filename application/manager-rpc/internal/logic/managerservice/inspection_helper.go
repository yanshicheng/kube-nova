package managerservicelogic

import (
	"database/sql"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
)

func inspectionString(s, fallback string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return fallback
	}
	return s
}

func inspectionBool(v bool) int64 {
	if v {
		return 1
	}
	return 0
}

func inspectionNullString(s string) sql.NullString {
	s = strings.TrimSpace(s)
	return sql.NullString{String: s, Valid: s != ""}
}

func inspectionTimeToUnix(t sql.NullTime) int64 {
	if !t.Valid {
		return 0
	}
	return t.Time.Unix()
}

func inspectionNextRun(expr string, base time.Time) sql.NullTime {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return sql.NullTime{}
	}
	parser := cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	schedule, err := parser.Parse(expr)
	if err != nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: schedule.Next(base), Valid: true}
}

func inspectionTemplateToPB(item *model.OnecInspectionTemplate) *pb.InspectionTemplate {
	if item == nil {
		return nil
	}
	return &pb.InspectionTemplate{
		Id:          item.Id,
		Name:        item.Name,
		Code:        item.Code,
		Description: item.Description,
		ScopeType:   item.ScopeType,
		Enabled:     item.Enabled == 1,
		IsBuiltin:   item.IsBuiltin == 1,
		Version:     item.Version,
		ConfigJson:  item.ConfigJson.String,
		CreatedBy:   item.CreatedBy,
		UpdatedBy:   item.UpdatedBy,
		CreatedAt:   item.CreatedAt.Unix(),
		UpdatedAt:   item.UpdatedAt.Unix(),
	}
}

func inspectionGroupToPB(item *model.OnecInspectionGroup) *pb.InspectionGroup {
	if item == nil {
		return nil
	}
	return &pb.InspectionGroup{
		Id:          item.Id,
		TemplateId:  item.TemplateId,
		GroupCode:   item.GroupCode,
		GroupName:   item.GroupName,
		Description: item.Description,
		Enabled:     item.Enabled == 1,
		OrderNum:    item.OrderNum,
		ConfigJson:  item.ConfigJson.String,
		CreatedBy:   item.CreatedBy,
		UpdatedBy:   item.UpdatedBy,
		CreatedAt:   item.CreatedAt.Unix(),
		UpdatedAt:   item.UpdatedAt.Unix(),
	}
}

func inspectionItemToPB(item *model.OnecInspectionItem) *pb.InspectionItem {
	if item == nil {
		return nil
	}
	return &pb.InspectionItem{
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
		Enabled:    item.Enabled == 1,
		TimeoutSec: item.TimeoutSec,
		OrderNum:   item.OrderNum,
		Promql:     item.Promql.String,
		Operator:   item.Operator,
		Threshold:  item.Threshold,
		Unit:       item.Unit,
		ConfigJson: item.ConfigJson.String,
		Advice:     item.Advice,
		CreatedBy:  item.CreatedBy,
		UpdatedBy:  item.UpdatedBy,
		CreatedAt:  item.CreatedAt.Unix(),
		UpdatedAt:  item.UpdatedAt.Unix(),
	}
}

func inspectionTaskToPB(item *model.OnecInspectionTask) *pb.InspectionTask {
	if item == nil {
		return nil
	}
	return &pb.InspectionTask{
		Id:                           item.Id,
		Name:                         item.Name,
		Description:                  item.Description,
		TemplateId:                   item.TemplateId,
		ScopeType:                    item.ScopeType,
		ClusterUuid:                  item.ClusterUuid,
		ScheduleType:                 item.ScheduleType,
		CronExpr:                     item.CronExpr,
		Enabled:                      item.Enabled == 1,
		MaxConcurrency:               item.MaxConcurrency,
		TimeoutSec:                   item.TimeoutSec,
		LastRunAt:                    inspectionTimeToUnix(item.LastRunAt),
		NextRunAt:                    inspectionTimeToUnix(item.NextRunAt),
		LastRecordId:                 item.LastRecordId,
		LastStatus:                   item.LastStatus,
		LastError:                    item.LastError,
		ConfigJson:                   item.ConfigJson.String,
		PrometheusEnabled:            item.PrometheusEnabled == 1,
		PrometheusEndpoint:           item.PrometheusEndpoint,
		PrometheusAuthEnabled:        item.PrometheusAuthEnabled == 1,
		PrometheusAuthType:           item.PrometheusAuthType,
		PrometheusUsername:           item.PrometheusUsername,
		PrometheusPassword:           item.PrometheusPassword,
		PrometheusToken:              item.PrometheusToken.String,
		PrometheusTlsEnabled:         item.PrometheusTlsEnabled == 1,
		PrometheusInsecureSkipVerify: item.PrometheusInsecureSkipVerify == 1,
		PrometheusCaCert:             item.PrometheusCaCert.String,
		PrometheusClientCert:         item.PrometheusClientCert.String,
		PrometheusClientKey:          item.PrometheusClientKey.String,
		CreatedBy:                    item.CreatedBy,
		UpdatedBy:                    item.UpdatedBy,
		CreatedAt:                    item.CreatedAt.Unix(),
		UpdatedAt:                    item.UpdatedAt.Unix(),
	}
}
