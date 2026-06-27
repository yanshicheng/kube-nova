package logservicelogic

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
)

func convertLogAlertRuleToPB(item *model.OnecLogAlertRule) *pb.LogAlertRule {
	if item == nil {
		return nil
	}
	return &pb.LogAlertRule{
		Id:             item.Id,
		ClusterUuid:    item.ClusterUuid,
		BackendType:    item.BackendType,
		ProjectUuid:    item.ProjectUuid,
		WorkspaceId:    item.WorkspaceId,
		Namespace:      item.Namespace,
		Application:    item.Application,
		ResourceName:   item.ResourceName,
		Name:           item.Name,
		Description:    item.Description,
		QueryText:      item.QueryText,
		ConditionType:  item.ConditionType,
		Threshold:      item.Threshold,
		Window:         item.Window,
		Severity:       item.Severity,
		NotifyChannels: nullStringValue(item.NotifyChannels),
		Enabled:        item.Enabled == 1,
		BackendRuleId:  item.BackendRuleId,
		BackendPayload: nullStringValue(item.BackendPayload),
		LastSyncStatus: item.LastSyncStatus,
		LastSyncError:  nullStringValue(item.LastSyncError),
		CreatedBy:      item.CreatedBy,
		UpdatedBy:      item.UpdatedBy,
		CreatedAt:      item.CreatedAt.Unix(),
		UpdatedAt:      item.UpdatedAt.Unix(),
		EvalInterval:   item.EvalInterval,
		ForDuration:    item.ForDuration,
		SilenceWindow:  item.SilenceWindow,
	}
}

func buildLogAlertWebhookURL(baseURL string) string {
	return fmt.Sprintf("%s/manager/v1/webhook/log-alerts", baseURL)
}

func defaultBackendRuleID(item *model.OnecLogAlertRule) string {
	switch item.BackendType {
	case "loki":
		prefix := strings.TrimSpace(item.ProjectUuid)
		if prefix == "" {
			prefix = strings.TrimSpace(item.ClusterUuid)
		}
		if prefix == "" {
			prefix = "global"
		}
		return fmt.Sprintf("%s/log-rule-%d", prefix, item.Id)
	case "elasticsearch", "es":
		return fmt.Sprintf("log-rule-%d", item.Id)
	default:
		return ""
	}
}

func toNullString(value string) sql.NullString {
	if strings.TrimSpace(value) == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

func nullStringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func ensureLogAlertRuleDefaults(item *model.OnecLogAlertRule) {
	if item == nil {
		return
	}
	if strings.TrimSpace(item.LogType) == "" {
		item.LogType = "container"
	}
	if strings.TrimSpace(item.SearchMode) == "" {
		item.SearchMode = "form"
	}
	if strings.TrimSpace(item.EvalInterval) == "" {
		item.EvalInterval = "1m"
	}
	if strings.TrimSpace(item.ForDuration) == "" {
		item.ForDuration = "5m"
	}
	if strings.TrimSpace(item.SilenceWindow) == "" {
		item.SilenceWindow = "15m"
	}
	if item.RuleVersion == 0 {
		item.RuleVersion = 1
	}
}

func isPlatformManagedLogBackend(svcCtx *svc.ServiceContext, backendType string) bool {
	if svcCtx == nil {
		return false
	}
	if !svcCtx.Config.LogAlertEngine.Enabled {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(svcCtx.Config.LogAlertEngine.Mode), "platform") {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(backendType)) {
	case "elasticsearch", "es", "loki":
		return true
	default:
		return false
	}
}

func findApplicationByName(ctx context.Context, svcCtx *svc.ServiceContext, workspaceID uint64, application string) (*model.OnecProjectApplication, error) {
	appList, err := svcCtx.OnecProjectApplication.FindAllByWorkspaceId(ctx, workspaceID)
	if err != nil {
		return nil, errorx.Msg("应用不存在")
	}
	for _, item := range appList {
		if item.NameEn == application {
			return item, nil
		}
	}
	return nil, errorx.Msg("应用不存在")
}
