package logservicelogic

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/consumer"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	portalpb "github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ManualRetryLogAlertEventLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewManualRetryLogAlertEventLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ManualRetryLogAlertEventLogic {
	return &ManualRetryLogAlertEventLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ManualRetryLogAlertEventLogic) ManualRetryLogAlertEvent(in *pb.ManualRetryLogAlertEventReq) (*pb.ManualRetryLogAlertEventResp, error) {
	operator := in.Operator
	if operator == "" {
		operator = "manual-retry"
	}
	event, err := l.svcCtx.OnecLogAlertFireEventModel.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, errorx.Msg("日志告警事件不存在")
		}
		return nil, errorx.Msg("手动补偿重试失败")
	}
	rule, err := l.svcCtx.OnecLogAlertRuleModel.FindOne(l.ctx, event.RuleId)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, errorx.Msg("日志告警规则不存在")
		}
		return nil, errorx.Msg("手动补偿重试失败")
	}

	payload, err := parseManualRetryPayload(event.PayloadJson)
	if err != nil {
		return nil, errorx.Msg("日志告警事件载荷损坏")
	}

	clusterName, projectID, projectName, workspaceID, workspaceName := resolveManualRetryRuleContext(l.ctx, l.svcCtx, rule)
	alertData, err := json.Marshal([]*consumer.AlertInstance{{
		ID:            rule.Id,
		Instance:      rule.ClusterUuid,
		Fingerprint:   event.EventFingerprint,
		ClusterUUID:   rule.ClusterUuid,
		ClusterName:   clusterName,
		ProjectID:     projectID,
		ProjectName:   projectName,
		WorkspaceID:   workspaceID,
		WorkspaceName: workspaceName,
		AlertName:     rule.Name,
		Severity:      rule.Severity,
		Status:        payload.Status,
		Labels: map[string]string{
			"alert_source": "logging",
			"backend_type": rule.BackendType,
			"cluster_uuid": rule.ClusterUuid,
			"project_uuid": rule.ProjectUuid,
			"namespace":    rule.Namespace,
			"rule_id":      fmt.Sprintf("%d", rule.Id),
		},
		Annotations: map[string]string{
			"description": rule.Description,
			"queryText":   rule.QueryText,
			"hitCount":    fmt.Sprintf("%d", payload.HitCount),
		},
		GeneratorURL: "platform-log-alert-engine",
		StartsAt:     payload.WindowStartTime(),
		EndsAt:       nil,
		ResolvedAt:   nil,
		Duration:     uint(payload.WindowEndTime().Sub(payload.WindowStartTime()).Seconds()),
		RepeatCount:  0,
	}})
	if err != nil {
		return nil, errorx.Msg("手动补偿重试失败")
	}

	event.RetryCount++
	event.UpdatedBy = operator

	ctx, cancel := context.WithTimeout(l.ctx, 30*time.Second)
	defer cancel()
	_, err = l.svcCtx.AlertRpc.AlertNotify(ctx, &portalpb.AlertNotifyReq{
		AlertType: "prometheus",
		AlertData: string(alertData),
		Title:     fmt.Sprintf("日志告警[%s] %s", strings.ToUpper(payload.Status), rule.Name),
	})
	if err != nil {
		event.NotifyStatus = "failed"
		event.NotifyError = trimManualRetryErr(err.Error())
		event.NextRetryAt = sql.NullTime{Time: time.Now().Add(30 * time.Second), Valid: true}
	} else {
		event.NotifyStatus = "success"
		event.NotifyError = ""
		event.NextRetryAt = sql.NullTime{}
	}
	if updateErr := l.svcCtx.OnecLogAlertFireEventModel.Update(l.ctx, event); updateErr != nil {
		return nil, errorx.Msg("手动补偿重试失败")
	}
	nextRetryAt := int64(0)
	if event.NextRetryAt.Valid {
		nextRetryAt = event.NextRetryAt.Time.Unix()
	}
	return &pb.ManualRetryLogAlertEventResp{
		Id:           event.Id,
		NotifyStatus: event.NotifyStatus,
		RetryCount:   event.RetryCount,
		NotifyError:  event.NotifyError,
		NextRetryAt:  nextRetryAt,
	}, nil
}

type manualRetryPayload struct {
	Status      string `json:"status"`
	HitCount    int64  `json:"hitCount"`
	WindowStart int64  `json:"windowStart"`
	WindowEnd   int64  `json:"windowEnd"`
}

func parseManualRetryPayload(payload sql.NullString) (*manualRetryPayload, error) {
	if !payload.Valid || strings.TrimSpace(payload.String) == "" {
		return nil, fmt.Errorf("payload empty")
	}
	var result manualRetryPayload
	if err := json.Unmarshal([]byte(payload.String), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (p *manualRetryPayload) WindowStartTime() time.Time {
	return time.UnixMilli(p.WindowStart)
}

func (p *manualRetryPayload) WindowEndTime() time.Time {
	return time.UnixMilli(p.WindowEnd)
}

func resolveManualRetryRuleContext(ctx context.Context, svcCtx *svc.ServiceContext, rule *model.OnecLogAlertRule) (clusterName string, projectID uint64, projectName string, workspaceID uint64, workspaceName string) {
	if cluster, err := svcCtx.OnecClusterModel.FindOneByUuid(ctx, rule.ClusterUuid); err == nil {
		clusterName = cluster.Name
	}
	if project, err := svcCtx.OnecProjectModel.FindOneByUuid(ctx, rule.ProjectUuid); err == nil {
		projectID = project.Id
		projectName = project.Name
	}
	if rule.WorkspaceId > 0 {
		if workspace, err := svcCtx.OnecProjectWorkspaceModel.FindOne(ctx, rule.WorkspaceId); err == nil {
			workspaceID = workspace.Id
			workspaceName = workspace.Name
		}
	}
	return
}

func trimManualRetryErr(value string) string {
	if len(value) <= 1000 {
		return value
	}
	return value[:1000]
}
