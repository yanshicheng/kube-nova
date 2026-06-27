package logging

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	logservice "github.com/yanshicheng/kube-nova/application/manager-rpc/client/logservice"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
)

func getUsername(ctx context.Context) string {
	username, ok := ctx.Value("username").(string)
	if !ok || username == "" {
		return "system"
	}
	return username
}

func getRoles(ctx context.Context) []string {
	roles, ok := ctx.Value("roles").([]string)
	if !ok {
		return nil
	}
	return roles
}

func isSuperAdmin(ctx context.Context) bool {
	for _, role := range getRoles(ctx) {
		if strings.EqualFold(strings.TrimSpace(role), "super_admin") {
			return true
		}
	}
	return false
}

func validateLogAlertScope(
	ctx context.Context,
	clusterUUID string,
	workspaceID uint64,
	projectUUID, namespace, application, resourceName string,
) error {
	clusterUUID = strings.TrimSpace(clusterUUID)
	projectUUID = strings.TrimSpace(projectUUID)
	namespace = strings.TrimSpace(namespace)
	application = strings.TrimSpace(application)
	resourceName = strings.TrimSpace(resourceName)

	if clusterUUID == "" {
		return errorx.Msg("请选择集群")
	}
	if workspaceID > 0 {
		if resourceName != "" && application == "" {
			return errorx.Msg("选择版本前请先选择服务")
		}
		return nil
	}
	if projectUUID == "" && namespace == "" {
		if application != "" || resourceName != "" {
			return errorx.Msg("全局规则不能直接选择服务或版本，请先切换到命名空间或服务范围")
		}
		if !isSuperAdmin(ctx) {
			return errorx.Msg("只有 SUPER_ADMIN 可以创建集群全局规则")
		}
		return nil
	}
	if projectUUID == "" || namespace == "" {
		return errorx.Msg("请选择完整的项目和命名空间范围")
	}
	if resourceName != "" && application == "" {
		return errorx.Msg("选择版本前请先选择服务")
	}
	return nil
}

func normalizeLogAlertQuery(conditionType, queryText string) string {
	conditionType = strings.ToLower(strings.TrimSpace(conditionType))
	queryText = strings.TrimSpace(queryText)
	if conditionType == "flatline" {
		return ""
	}
	return queryText
}

func validateLogAlertDefinition(conditionType, queryText string) error {
	conditionType = strings.ToLower(strings.TrimSpace(conditionType))
	queryText = strings.TrimSpace(queryText)

	switch conditionType {
	case "no_data", "flatline":
		return nil
	default:
		if queryText == "" {
			return errorx.Msg("当前条件类型必须填写查询内容")
		}
		return nil
	}
}

func validateLogAlertStrategy(window, evalInterval, forDuration, silenceWindow string) error {
	if err := validatePositiveDuration("时间窗口", window, false); err != nil {
		return err
	}
	if err := validatePositiveDuration("评估间隔", evalInterval, true); err != nil {
		return err
	}
	if err := validateNonNegativeDuration("持续命中", forDuration, true); err != nil {
		return err
	}
	if err := validateNonNegativeDuration("静默窗口", silenceWindow, true); err != nil {
		return err
	}
	return nil
}

func validatePositiveDuration(label, value string, allowEmpty bool) error {
	value = strings.TrimSpace(value)
	if value == "" {
		if allowEmpty {
			return nil
		}
		return errorx.Msg(label + "不能为空")
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return errorx.Msg(label + "格式错误，请使用类似 30s、5m、1h 的值")
	}
	return nil
}

func validateNonNegativeDuration(label, value string, allowEmpty bool) error {
	value = strings.TrimSpace(value)
	if value == "" {
		if allowEmpty {
			return nil
		}
		return errorx.Msg(label + "不能为空")
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed < 0 {
		return errorx.Msg(label + "格式错误，请使用类似 0s、30s、5m、1h 的值")
	}
	return nil
}

func allowEmptyQueryForLogAlert(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	clusterUUID string,
	conditionType string,
) bool {
	conditionType = strings.ToLower(strings.TrimSpace(conditionType))
	if conditionType != "no_data" && conditionType != "flatline" {
		return false
	}
	if svcCtx == nil || strings.TrimSpace(clusterUUID) == "" {
		return false
	}
	resp, err := svcCtx.LogRpc.GetLogClusterConfig(ctx, &logservice.GetLogClusterConfigReq{
		ClusterUuid: clusterUUID,
	})
	if err != nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(resp.DefaultBackend)) {
	case "elasticsearch", "es", "loki":
		return true
	default:
		return false
	}
}

func parseLogTime(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, &timeParseError{}
	}
	if millis, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		return time.UnixMilli(millis), nil
	}
	formats := []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05.999Z07:00", "2006-01-02T15:04:05Z07:00", "2006-01-02T15:04:05Z"}
	for _, format := range formats {
		if t, err := time.Parse(format, trimmed); err == nil {
			return t, nil
		}
	}
	return time.Time{}, &timeParseError{}
}

func parseRFC3339Time(value string) (time.Time, error) {
	return parseLogTime(value)
}

type timeParseError struct{}

func (e *timeParseError) Error() string {
	return "时间格式错误，请使用毫秒时间戳字符串"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func convertLogAlertRule(pbRule *logservice.LogAlertRule) *types.LogAlertRule {
	if pbRule == nil {
		return nil
	}
	return &types.LogAlertRule{
		Id:             pbRule.Id,
		ClusterUuid:    pbRule.ClusterUuid,
		BackendType:    pbRule.BackendType,
		ProjectUuid:    pbRule.ProjectUuid,
		WorkspaceId:    pbRule.WorkspaceId,
		Namespace:      pbRule.Namespace,
		Application:    pbRule.Application,
		ResourceName:   pbRule.ResourceName,
		Name:           pbRule.Name,
		Description:    pbRule.Description,
		QueryText:      pbRule.QueryText,
		ConditionType:  pbRule.ConditionType,
		Threshold:      pbRule.Threshold,
		Window:         pbRule.Window,
		Severity:       pbRule.Severity,
		NotifyChannels: pbRule.NotifyChannels,
		Enabled:        pbRule.Enabled,
		BackendRuleId:  pbRule.BackendRuleId,
		LastSyncStatus: pbRule.LastSyncStatus,
		LastSyncError:  pbRule.LastSyncError,
		CreatedBy:      pbRule.CreatedBy,
		UpdatedBy:      pbRule.UpdatedBy,
		CreatedAt:      pbRule.CreatedAt,
		UpdatedAt:      pbRule.UpdatedAt,
		EvalInterval:   pbRule.EvalInterval,
		ForDuration:    pbRule.ForDuration,
		SilenceWindow:  pbRule.SilenceWindow,
	}
}

func convertLogAlertEvent(pbEvent *logservice.LogAlertEvent) types.LogAlertEvent {
	if pbEvent == nil {
		return types.LogAlertEvent{}
	}
	return types.LogAlertEvent{
		Id:               pbEvent.Id,
		RuleId:           pbEvent.RuleId,
		RuleName:         pbEvent.RuleName,
		BackendType:      pbEvent.BackendType,
		ClusterUuid:      pbEvent.ClusterUuid,
		ClusterName:      pbEvent.ClusterName,
		ProjectUuid:      pbEvent.ProjectUuid,
		ProjectName:      pbEvent.ProjectName,
		WorkspaceId:      pbEvent.WorkspaceId,
		WorkspaceName:    pbEvent.WorkspaceName,
		Namespace:        pbEvent.Namespace,
		Application:      pbEvent.Application,
		ResourceName:     pbEvent.ResourceName,
		PodName:          pbEvent.PodName,
		ContainerName:    pbEvent.ContainerName,
		Host:             pbEvent.Host,
		EventFingerprint: pbEvent.EventFingerprint,
		HitCount:         pbEvent.HitCount,
		Severity:         pbEvent.Severity,
		EventStatus:      pbEvent.EventStatus,
		NotifyStatus:     pbEvent.NotifyStatus,
		NotifyError:      pbEvent.NotifyError,
		RetryCount:       pbEvent.RetryCount,
		NextRetryAt:      pbEvent.NextRetryAt,
		SampleTimestamp:  pbEvent.SampleTimestamp,
		SampleMessage:    pbEvent.SampleMessage,
		SampleRaw:        pbEvent.SampleRaw,
		SampleLabels:     pbEvent.SampleLabels,
		CloseReason:      pbEvent.CloseReason,
		ClosedBy:         pbEvent.ClosedBy,
		ClosedAt:         pbEvent.ClosedAt,
		ManualReason:     pbEvent.ManualReason,
		ManualReasonBy:   pbEvent.ManualReasonBy,
		ManualReasonAt:   pbEvent.ManualReasonAt,
		CreatedAt:        pbEvent.CreatedAt,
		UpdatedAt:        pbEvent.UpdatedAt,
	}
}
