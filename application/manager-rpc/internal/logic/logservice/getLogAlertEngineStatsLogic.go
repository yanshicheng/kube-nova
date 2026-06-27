package logservicelogic

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetLogAlertEngineStatsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetLogAlertEngineStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetLogAlertEngineStatsLogic {
	return &GetLogAlertEngineStatsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetLogAlertEngineStatsLogic) GetLogAlertEngineStats(in *pb.GetLogAlertEngineStatsReq) (*pb.GetLogAlertEngineStatsResp, error) {
	clusterUUID := strings.TrimSpace(in.ClusterUuid)

	ruleQuery := ""
	taskQuery := ""
	eventQuery := ""
	var ruleArgs []any
	var taskArgs []any
	var eventArgs []any
	if clusterUUID != "" {
		ruleQuery = "`cluster_uuid` = ?"
		taskQuery = "`cluster_uuid` = ?"
		eventQuery = "`cluster_uuid` = ?"
		ruleArgs = append(ruleArgs, clusterUUID)
		taskArgs = append(taskArgs, clusterUUID)
		eventArgs = append(eventArgs, clusterUUID)
	}

	rules, err := l.svcCtx.OnecLogAlertRuleModel.SearchNoPage(l.ctx, "id", true, ruleQuery, ruleArgs...)
	if err != nil && err != model.ErrNotFound {
		return nil, errorx.Msg("查询日志告警引擎统计失败")
	}
	tasks, err := l.svcCtx.OnecLogAlertEvalTaskModel.SearchNoPage(l.ctx, "id", true, taskQuery, taskArgs...)
	if err != nil && err != model.ErrNotFound {
		return nil, errorx.Msg("查询日志告警引擎统计失败")
	}
	events, err := l.svcCtx.OnecLogAlertFireEventModel.SearchNoPage(l.ctx, "id", true, eventQuery, eventArgs...)
	if err != nil && err != model.ErrNotFound {
		return nil, errorx.Msg("查询日志告警引擎统计失败")
	}
	states, err := l.svcCtx.OnecLogAlertEvalStateModel.SearchNoPage(l.ctx, "id", true, taskQuery, taskArgs...)
	if err != nil && err != model.ErrNotFound {
		return nil, errorx.Msg("查询日志告警引擎统计失败")
	}

	resp := &pb.GetLogAlertEngineStatsResp{ClusterUuid: clusterUUID}
	for _, rule := range rules {
		resp.TotalRules++
		if rule.Enabled == 1 {
			resp.EnabledRules++
		} else {
			resp.DisabledRules++
		}
		switch strings.ToLower(strings.TrimSpace(rule.Severity)) {
		case "critical":
			resp.CriticalRules++
		case "warning":
			resp.WarningRules++
		case "info":
			resp.InfoRules++
		}
	}
	for _, task := range tasks {
		switch strings.ToLower(strings.TrimSpace(task.LastEvalStatus)) {
		case "success":
			resp.SuccessRules++
		case "failed":
			resp.FailedRules++
		case "pending", "syncing":
			resp.PendingRules++
		}
		switch classifyStatsBucket(task.Priority) {
		case "hot":
			resp.HotRules++
		case "warm":
			resp.WarmRules++
		default:
			resp.ColdRules++
		}
	}
	for _, event := range events {
		switch strings.ToLower(strings.TrimSpace(event.NotifyStatus)) {
		case "failed":
			resp.FailedNotifyEvents++
		case "dead":
			resp.DeadNotifyEvents++
		}
	}
	for _, state := range states {
		if decodeEngineRuntimeState(state.StateJson).Active {
			resp.ActiveFiringEvents++
		}
	}
	return resp, nil
}

func classifyStatsBucket(priority int64) string {
	switch {
	case priority <= 20:
		return "hot"
	case priority <= 60:
		return "warm"
	default:
		return "cold"
	}
}

type engineRuntimeState struct {
	Active bool `json:"active"`
}

func decodeEngineRuntimeState(state sql.NullString) engineRuntimeState {
	if !state.Valid || strings.TrimSpace(state.String) == "" {
		return engineRuntimeState{}
	}
	var result engineRuntimeState
	if err := json.Unmarshal([]byte(state.String), &result); err != nil {
		return engineRuntimeState{}
	}
	return result
}
