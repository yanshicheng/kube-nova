package logservicelogic

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

type CloseLogAlertEventLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCloseLogAlertEventLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CloseLogAlertEventLogic {
	return &CloseLogAlertEventLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *CloseLogAlertEventLogic) CloseLogAlertEvent(in *pb.CloseLogAlertEventReq) (*pb.CloseLogAlertEventResp, error) {
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return nil, errorx.Msg("关闭原因不能为空")
	}
	operator := strings.TrimSpace(in.Operator)
	if operator == "" {
		operator = "manual-close"
	}

	event, err := l.svcCtx.OnecLogAlertFireEventModel.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, errorx.Msg("日志告警事件不存在")
		}
		return nil, errorx.Msg("关闭日志告警事件失败")
	}

	payload, err := parseLogAlertEventPayload(event.PayloadJson)
	if err != nil {
		return nil, errorx.Msg("日志告警事件载荷损坏")
	}
	now := time.Now()
	payload.CloseReason = reason
	payload.ClosedBy = operator
	payload.ClosedAt = now.Unix()
	if payload.Status == "" {
		payload.Status = event.EventStatus
	}
	event.EventStatus = "closed"
	event.UpdatedBy = operator
	event.PayloadJson = toEventPayloadJSON(payload)
	if err := l.svcCtx.OnecLogAlertFireEventModel.Update(l.ctx, event); err != nil {
		return nil, errorx.Msg("关闭日志告警事件失败")
	}

	if rule, ruleErr := l.svcCtx.OnecLogAlertRuleModel.FindOne(l.ctx, event.RuleId); ruleErr == nil {
		_ = l.closeEventRuntime(rule, operator, now)
	}

	builder := NewSearchLogAlertEventsLogic(l.ctx, l.svcCtx)
	data, buildErr := builder.buildEventResponse(event)
	if buildErr != nil {
		return nil, errorx.Msg("关闭日志告警事件失败")
	}
	return &pb.CloseLogAlertEventResp{Data: data}, nil
}

func (l *CloseLogAlertEventLogic) closeEventRuntime(rule *model.OnecLogAlertRule, operator string, now time.Time) error {
	state, err := l.svcCtx.OnecLogAlertEvalStateModel.FindOneByRuleIdClusterUuid(l.ctx, rule.Id, rule.ClusterUuid)
	if err != nil || state == nil {
		return nil
	}
	runtimeState := decodeCloseRuntimeState(state.StateJson)
	runtimeState.Active = false
	runtimeState.LastEventFingerprint = ""
	runtimeState.SilenceUntilMs = now.Add(parseCloseSilenceWindow(rule.SilenceWindow)).UnixMilli()
	data, _ := json.Marshal(runtimeState)
	state.StateJson = sql.NullString{String: string(data), Valid: true}
	state.UpdatedBy = operator
	return l.svcCtx.OnecLogAlertEvalStateModel.Update(l.ctx, state)
}

type closeRuntimeState struct {
	Active               bool   `json:"active"`
	LastEventFingerprint string `json:"lastEventFingerprint,omitempty"`
	SilenceUntilMs       int64  `json:"silenceUntilMs,omitempty"`
}

func decodeCloseRuntimeState(value sql.NullString) closeRuntimeState {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return closeRuntimeState{}
	}
	var result closeRuntimeState
	if err := json.Unmarshal([]byte(value.String), &result); err != nil {
		return closeRuntimeState{}
	}
	return result
}

func parseCloseSilenceWindow(value string) time.Duration {
	parsed, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return 15 * time.Minute
	}
	return parsed
}
