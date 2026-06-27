package logging

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	logservice "github.com/yanshicheng/kube-nova/application/manager-rpc/client/logservice"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListLogAlertRulesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListLogAlertRulesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListLogAlertRulesLogic {
	return &ListLogAlertRulesLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ListLogAlertRulesLogic) ListLogAlertRules(req *types.ListLogAlertRulesRequest) (resp *types.ListLogAlertRulesResponse, err error) {
	enabled := int64(-1)
	if req.Enabled != nil {
		if *req.Enabled {
			enabled = 1
		} else {
			enabled = 0
		}
	}
	rpcResp, err := l.svcCtx.LogRpc.SearchLogAlertRule(l.ctx, &logservice.SearchLogAlertRuleReq{
		Page:         req.Page,
		PageSize:     req.PageSize,
		OrderField:   "id",
		IsAsc:        false,
		ClusterUuid:  req.ClusterUuid,
		WorkspaceId:  req.WorkspaceId,
		ProjectUuid:  req.ProjectUuid,
		Namespace:    req.Namespace,
		Application:  req.Application,
		ResourceName: req.ResourceName,
		Enabled:      enabled,
	})
	if err != nil {
		return nil, err
	}
	data := make([]types.LogAlertRule, 0, len(rpcResp.Data))
	for _, item := range rpcResp.Data {
		if rule := convertLogAlertRule(item); rule != nil {
			data = append(data, *rule)
		}
	}
	return &types.ListLogAlertRulesResponse{Data: data, Total: rpcResp.Total}, nil
}
