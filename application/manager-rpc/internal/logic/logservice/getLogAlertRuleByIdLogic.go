package logservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetLogAlertRuleByIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetLogAlertRuleByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetLogAlertRuleByIdLogic {
	return &GetLogAlertRuleByIdLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetLogAlertRuleByIdLogic) GetLogAlertRuleById(in *pb.GetLogAlertRuleByIdReq) (*pb.GetLogAlertRuleByIdResp, error) {
	item, err := l.svcCtx.OnecLogAlertRuleModel.FindOne(l.ctx, in.Id)
	if err != nil {
		return nil, errorx.Msg("日志告警不存在")
	}
	return &pb.GetLogAlertRuleByIdResp{Data: convertLogAlertRuleToPB(item)}, nil
}
