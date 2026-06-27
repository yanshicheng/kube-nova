package logging

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	logservice "github.com/yanshicheng/kube-nova/application/manager-rpc/client/logservice"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateLogAlertRuleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateLogAlertRuleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateLogAlertRuleLogic {
	return &UpdateLogAlertRuleLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *UpdateLogAlertRuleLogic) UpdateLogAlertRule(req *types.UpdateLogAlertRuleRequest) (resp *types.LogAlertRule, err error) {
	username := getUsername(l.ctx)
	if err := validateLogAlertScope(
		l.ctx,
		req.ClusterUuid,
		req.WorkspaceId,
		req.ProjectUuid,
		req.Namespace,
		req.Application,
		req.ResourceName,
	); err != nil {
		return nil, err
	}
	queryText := normalizeLogAlertQuery(req.ConditionType, req.QueryText)
	if queryText == "" && !allowEmptyQueryForLogAlert(l.ctx, l.svcCtx, req.ClusterUuid, req.ConditionType) {
		return nil, errorx.Msg("当前日志后端需要填写查询内容，建议在 ES 集群中使用无匹配日志或日志停滞")
	}
	if err := validateLogAlertDefinition(req.ConditionType, queryText); err != nil {
		return nil, err
	}
	if err := validateLogAlertStrategy(req.Window, req.EvalInterval, req.ForDuration, req.SilenceWindow); err != nil {
		return nil, err
	}
	rpcResp, err := l.svcCtx.LogRpc.UpdateLogAlertRule(l.ctx, &logservice.UpdateLogAlertRuleReq{
		Id:             req.Id,
		ClusterUuid:    req.ClusterUuid,
		WorkspaceId:    req.WorkspaceId,
		ProjectUuid:    req.ProjectUuid,
		Namespace:      req.Namespace,
		Application:    req.Application,
		ResourceName:   req.ResourceName,
		Name:           req.Name,
		Description:    req.Description,
		QueryText:      queryText,
		ConditionType:  req.ConditionType,
		Threshold:      req.Threshold,
		Window:         req.Window,
		EvalInterval:   req.EvalInterval,
		ForDuration:    req.ForDuration,
		SilenceWindow:  req.SilenceWindow,
		Severity:       req.Severity,
		NotifyChannels: req.NotifyChannels,
		Enabled:        req.Enabled,
		UpdatedBy:      username,
	})
	if err != nil {
		return nil, err
	}
	return convertLogAlertRule(rpcResp.Data), nil
}
