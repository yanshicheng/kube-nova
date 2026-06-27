package logservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateLogAlertRuleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateLogAlertRuleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateLogAlertRuleLogic {
	return &UpdateLogAlertRuleLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *UpdateLogAlertRuleLogic) UpdateLogAlertRule(in *pb.UpdateLogAlertRuleReq) (*pb.UpdateLogAlertRuleResp, error) {
	item, err := l.svcCtx.OnecLogAlertRuleModel.FindOne(l.ctx, in.Id)
	if err != nil {
		return nil, errorx.Msg("日志告警不存在")
	}
	ensureLogAlertRuleDefaults(item)
	scope, err := NewResolveLogScopeLogic(l.ctx, l.svcCtx).ResolveLogScope(&pb.ResolveLogScopeReq{ClusterUuid: in.ClusterUuid, WorkspaceId: in.WorkspaceId, ProjectUuid: in.ProjectUuid, Namespace: in.Namespace, Application: in.Application, ResourceName: in.ResourceName})
	if err != nil {
		return nil, err
	}
	apps, err := l.svcCtx.OnecClusterAppModel.SearchNoPage(l.ctx, "created_at", false, "`cluster_uuid` = ? AND `app_type` = ? AND `is_default` = 1", scope.ClusterUuid, 2)
	if err != nil || len(apps) == 0 {
		return nil, errorx.Msg("未找到默认日志组件配置")
	}
	item.ClusterUuid = scope.ClusterUuid
	item.BackendType = apps[0].AppCode
	item.ProjectUuid = scope.ProjectUuid
	item.WorkspaceId = scope.WorkspaceId
	item.Namespace = scope.Namespace
	item.Application = scope.ApplicationName
	item.ResourceName = scope.ResourceName
	item.Name = in.Name
	item.Description = in.Description
	item.QueryText = in.QueryText
	item.ConditionType = in.ConditionType
	item.Threshold = in.Threshold
	item.Window = in.Window
	item.Severity = in.Severity
	item.NotifyChannels = toNullString(in.NotifyChannels)
	item.Enabled = boolToInt(in.Enabled)
	item.UpdatedBy = in.UpdatedBy
	item.EvalInterval = firstNonEmptyDuration(in.EvalInterval, item.EvalInterval)
	item.ForDuration = firstNonEmptyDuration(in.ForDuration, item.ForDuration)
	item.SilenceWindow = firstNonEmptyDuration(in.SilenceWindow, item.SilenceWindow)
	if !in.Enabled {
		item.LastSyncStatus = "disabled"
		item.LastSyncError = toNullString("")
	} else if isPlatformManagedLogBackend(l.svcCtx, item.BackendType) {
		item.LastSyncStatus = "pending"
		item.LastSyncError = toNullString("")
	}
	if err := l.svcCtx.OnecLogAlertRuleModel.Update(l.ctx, item); err != nil {
		return nil, errorx.Msg("更新日志告警失败")
	}
	if isPlatformManagedLogBackend(l.svcCtx, item.BackendType) {
		_ = upsertPlatformEvalTask(l.ctx, l.svcCtx, item)
		return &pb.UpdateLogAlertRuleResp{Data: convertLogAlertRuleToPB(item)}, nil
	}
	client, buildErr := buildLogClient(l.ctx, apps[0], l.svcCtx.Config.LogSearch)
	if buildErr == nil {
		if in.Enabled {
			if webhookURL, webhookToken, cbErr := getWebhookCallback(l.ctx, l.svcCtx); cbErr == nil {
				var payload string
				if item.BackendRuleId == "" {
					backendID, syncPayload, syncErr := client.CreateAlert(buildAlertSyncRequest(item, webhookURL, webhookToken))
					payload = syncPayload
					if syncErr == nil {
						item.BackendRuleId = backendID
						item.BackendPayload = toNullString(payload)
						item.LastSyncStatus = "success"
						item.LastSyncError = toNullString("")
					} else {
						item.LastSyncStatus = "failed"
						item.LastSyncError = toNullString(syncErr.Error())
					}
				} else {
					syncPayload, syncErr := client.UpdateAlert(item.BackendRuleId, buildAlertSyncRequest(item, webhookURL, webhookToken))
					payload = syncPayload
					if syncErr == nil {
						item.BackendPayload = toNullString(payload)
						if item.BackendRuleId == "" {
							item.BackendRuleId = defaultBackendRuleID(item)
						}
						item.LastSyncStatus = "success"
						item.LastSyncError = toNullString("")
					} else {
						item.LastSyncStatus = "failed"
						item.LastSyncError = toNullString(syncErr.Error())
					}
				}
				_ = l.svcCtx.OnecLogAlertRuleModel.Update(l.ctx, item)
			}
		} else if item.BackendRuleId != "" {
			syncErr := client.DisableAlert(item.BackendRuleId)
			if syncErr == nil {
				item.LastSyncStatus = "disabled"
				item.LastSyncError = toNullString("")
			} else {
				item.LastSyncStatus = "failed"
				item.LastSyncError = toNullString(syncErr.Error())
			}
			_ = l.svcCtx.OnecLogAlertRuleModel.Update(l.ctx, item)
		}
	}
	return &pb.UpdateLogAlertRuleResp{Data: convertLogAlertRuleToPB(item)}, nil
}
