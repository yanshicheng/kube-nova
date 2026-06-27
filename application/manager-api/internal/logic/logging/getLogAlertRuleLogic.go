package logging

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	logservice "github.com/yanshicheng/kube-nova/application/manager-rpc/client/logservice"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetLogAlertRuleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetLogAlertRuleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetLogAlertRuleLogic {
	return &GetLogAlertRuleLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *GetLogAlertRuleLogic) GetLogAlertRule(req *types.GetLogAlertRuleRequest) (resp *types.LogAlertRule, err error) {
	rpcResp, err := l.svcCtx.LogRpc.GetLogAlertRuleById(l.ctx, &logservice.GetLogAlertRuleByIdReq{Id: req.Id})
	if err != nil {
		return nil, err
	}
	return convertLogAlertRule(rpcResp.Data), nil
}
