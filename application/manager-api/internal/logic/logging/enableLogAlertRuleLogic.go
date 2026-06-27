package logging

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	logservice "github.com/yanshicheng/kube-nova/application/manager-rpc/client/logservice"
	"github.com/zeromicro/go-zero/core/logx"
)

type EnableLogAlertRuleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewEnableLogAlertRuleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EnableLogAlertRuleLogic {
	return &EnableLogAlertRuleLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *EnableLogAlertRuleLogic) EnableLogAlertRule(req *types.ToggleLogAlertRuleRequest) (resp string, err error) {
	username := getUsername(l.ctx)
	_, err = l.svcCtx.LogRpc.ToggleLogAlertRule(l.ctx, &logservice.ToggleLogAlertRuleReq{Id: req.Id, Enabled: true, UpdatedBy: username})
	if err != nil {
		return "", err
	}
	return "启用成功", nil
}
