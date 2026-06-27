package logging

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	logservice "github.com/yanshicheng/kube-nova/application/manager-rpc/client/logservice"
	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteLogAlertRuleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteLogAlertRuleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteLogAlertRuleLogic {
	return &DeleteLogAlertRuleLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *DeleteLogAlertRuleLogic) DeleteLogAlertRule(req *types.DeleteLogAlertRuleRequest) (resp string, err error) {
	username := getUsername(l.ctx)
	_, err = l.svcCtx.LogRpc.DelLogAlertRule(l.ctx, &logservice.DelLogAlertRuleReq{Id: req.Id, UpdatedBy: username})
	if err != nil {
		return "", err
	}
	return "删除成功", nil
}
