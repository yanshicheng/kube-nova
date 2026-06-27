package logservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

type ToggleLogAlertRuleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewToggleLogAlertRuleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ToggleLogAlertRuleLogic {
	return &ToggleLogAlertRuleLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ToggleLogAlertRuleLogic) ToggleLogAlertRule(in *pb.ToggleLogAlertRuleReq) (*pb.ToggleLogAlertRuleResp, error) {
	item, err := l.svcCtx.OnecLogAlertRuleModel.FindOne(l.ctx, in.Id)
	if err != nil {
		return nil, errorx.Msg("日志告警不存在")
	}
	ensureLogAlertRuleDefaults(item)
	item.Enabled = boolToInt(in.Enabled)
	item.UpdatedBy = in.UpdatedBy
	if apps, appErr := l.svcCtx.OnecClusterAppModel.SearchNoPage(l.ctx, "created_at", false, "`cluster_uuid` = ? AND `app_type` = ? AND `is_default` = 1", item.ClusterUuid, 2); appErr == nil && len(apps) > 0 {
		item.BackendType = apps[0].AppCode
		if isPlatformManagedLogBackend(l.svcCtx, item.BackendType) {
			if in.Enabled {
				item.LastSyncStatus = "pending"
				item.LastSyncError = toNullString("")
			} else {
				item.LastSyncStatus = "disabled"
				item.LastSyncError = toNullString("")
			}
			if err := l.svcCtx.OnecLogAlertRuleModel.Update(l.ctx, item); err != nil {
				return nil, errorx.Msg("切换日志告警状态失败")
			}
			_ = upsertPlatformEvalTask(l.ctx, l.svcCtx, item)
			return &pb.ToggleLogAlertRuleResp{Data: convertLogAlertRuleToPB(item)}, nil
		}
		if client, buildErr := buildLogClient(l.ctx, apps[0], l.svcCtx.Config.LogSearch); buildErr == nil {
			if in.Enabled {
				if webhookURL, webhookToken, cbErr := getWebhookCallback(l.ctx, l.svcCtx); cbErr == nil {
					backendID, payload, syncErr := client.EnableAlert(buildAlertSyncRequest(item, webhookURL, webhookToken), item.BackendRuleId)
					if syncErr == nil {
						if backendID == "" {
							backendID = item.BackendRuleId
						}
						if backendID == "" {
							backendID = defaultBackendRuleID(item)
						}
						item.BackendRuleId = backendID
						if payload != "" {
							item.BackendPayload = toNullString(payload)
						}
						item.LastSyncStatus = "success"
						item.LastSyncError = toNullString("")
					} else {
						item.LastSyncStatus = "failed"
						item.LastSyncError = toNullString(syncErr.Error())
					}
				}
			} else if item.BackendRuleId == "" {
				item.LastSyncStatus = "disabled"
				item.LastSyncError = toNullString("")
			} else {
				syncErr := client.DisableAlert(item.BackendRuleId)
				if syncErr == nil {
					item.LastSyncStatus = "disabled"
					item.LastSyncError = toNullString("")
				} else {
					item.LastSyncStatus = "failed"
					item.LastSyncError = toNullString(syncErr.Error())
				}
			}
		}
	}
	if err := l.svcCtx.OnecLogAlertRuleModel.Update(l.ctx, item); err != nil {
		return nil, errorx.Msg("切换日志告警状态失败")
	}
	return &pb.ToggleLogAlertRuleResp{Data: convertLogAlertRuleToPB(item)}, nil
}
