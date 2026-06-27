package logging

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	logservice "github.com/yanshicheng/kube-nova/application/manager-rpc/client/logservice"
	"github.com/zeromicro/go-zero/core/logx"
)

type DisableLogAlertRuleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDisableLogAlertRuleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DisableLogAlertRuleLogic {
	return &DisableLogAlertRuleLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *DisableLogAlertRuleLogic) DisableLogAlertRule(req *types.ToggleLogAlertRuleRequest) (resp string, err error) {
	username := getUsername(l.ctx)
	_, err = l.svcCtx.LogRpc.ToggleLogAlertRule(l.ctx, &logservice.ToggleLogAlertRuleReq{Id: req.Id, Enabled: false, UpdatedBy: username})
	if err != nil {
		return "", err
	}
	return "禁用成功", nil
}
