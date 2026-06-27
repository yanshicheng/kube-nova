package logservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

type AddLogAlertRuleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAddLogAlertRuleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddLogAlertRuleLogic {
	return &AddLogAlertRuleLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *AddLogAlertRuleLogic) AddLogAlertRule(in *pb.AddLogAlertRuleReq) (*pb.AddLogAlertRuleResp, error) {
	scope, err := NewResolveLogScopeLogic(l.ctx, l.svcCtx).ResolveLogScope(&pb.ResolveLogScopeReq{
		ClusterUuid:  in.ClusterUuid,
		WorkspaceId:  in.WorkspaceId,
		ProjectUuid:  in.ProjectUuid,
		Namespace:    in.Namespace,
		Application:  in.Application,
		ResourceName: in.ResourceName,
	})
	if err != nil {
		return nil, err
	}
	apps, err := l.svcCtx.OnecClusterAppModel.SearchNoPage(l.ctx, "created_at", false, "`cluster_uuid` = ? AND `app_type` = ? AND `is_default` = 1", scope.ClusterUuid, 2)
	if err != nil || len(apps) == 0 {
		return nil, errorx.Msg("未找到默认日志组件配置")
	}
	data := &model.OnecLogAlertRule{
		ClusterUuid:    scope.ClusterUuid,
		BackendType:    apps[0].AppCode,
		ProjectUuid:    scope.ProjectUuid,
		WorkspaceId:    scope.WorkspaceId,
		Namespace:      scope.Namespace,
		Application:    scope.ApplicationName,
		ResourceName:   scope.ResourceName,
		Name:           in.Name,
		Description:    in.Description,
		QueryText:      in.QueryText,
		ConditionType:  in.ConditionType,
		Threshold:      in.Threshold,
		Window:         in.Window,
		Severity:       in.Severity,
		NotifyChannels: toNullString(in.NotifyChannels),
		Enabled:        boolToInt(in.Enabled),
		BackendRuleId:  "",
		BackendPayload: toNullString(""),
		LastSyncStatus: "pending",
		LastSyncError:  toNullString(""),
		LogType:        "container",
		SearchMode:     "form",
		Expr:           toNullString(""),
		EvalInterval:   firstNonEmptyDuration(in.EvalInterval, "1m"),
		ForDuration:    firstNonEmptyDuration(in.ForDuration, "5m"),
		SilenceWindow:  firstNonEmptyDuration(in.SilenceWindow, "15m"),
		RuleVersion:    1,
		CreatedBy:      in.CreatedBy,
		UpdatedBy:      in.CreatedBy,
		IsDeleted:      0,
	}
	if !in.Enabled {
		data.LastSyncStatus = "disabled"
	}
	result, err := l.svcCtx.OnecLogAlertRuleModel.Insert(l.ctx, data)
	if err != nil {
		return nil, errorx.Msg("创建日志告警失败")
	}
	id, _ := result.LastInsertId()
	data.Id = uint64(id)
	if isPlatformManagedLogBackend(l.svcCtx, data.BackendType) {
		if in.Enabled {
			data.LastSyncStatus = "pending"
			data.LastSyncError = toNullString("")
			_ = l.svcCtx.OnecLogAlertRuleModel.Update(l.ctx, data)
		}
		_ = upsertPlatformEvalTask(l.ctx, l.svcCtx, data)
		if fresh, findErr := l.svcCtx.OnecLogAlertRuleModel.FindOne(l.ctx, data.Id); findErr == nil {
			data = fresh
		}
		return &pb.AddLogAlertRuleResp{Data: convertLogAlertRuleToPB(data)}, nil
	}
	if !in.Enabled {
		if fresh, findErr := l.svcCtx.OnecLogAlertRuleModel.FindOne(l.ctx, data.Id); findErr == nil {
			data = fresh
		}
		return &pb.AddLogAlertRuleResp{Data: convertLogAlertRuleToPB(data)}, nil
	}
	client, err := buildLogClient(l.ctx, apps[0], l.svcCtx.Config.LogSearch)
	if err == nil {
		if webhookURL, webhookToken, cbErr := getWebhookCallback(l.ctx, l.svcCtx); cbErr == nil {
			backendID, payload, syncErr := client.CreateAlert(buildAlertSyncRequest(data, webhookURL, webhookToken))
			if syncErr == nil {
				data.BackendRuleId = backendID
				data.BackendPayload = toNullString(payload)
				data.LastSyncStatus = "success"
				data.LastSyncError = toNullString("")
			} else {
				data.LastSyncStatus = "failed"
				data.LastSyncError = toNullString(syncErr.Error())
			}
			_ = l.svcCtx.OnecLogAlertRuleModel.Update(l.ctx, data)
		}
	}
	if fresh, findErr := l.svcCtx.OnecLogAlertRuleModel.FindOne(l.ctx, data.Id); findErr == nil {
		data = fresh
	}
	return &pb.AddLogAlertRuleResp{Data: convertLogAlertRuleToPB(data)}, nil
}

func boolToInt(v bool) int64 {
	if v {
		return 1
	}
	return 0
}

func firstNonEmptyDuration(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
