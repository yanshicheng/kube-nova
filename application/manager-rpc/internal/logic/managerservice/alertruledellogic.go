package managerservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertRuleDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertRuleDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertRuleDelLogic {
	return &AlertRuleDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AlertRuleDel 删除告警规则
func (l *AlertRuleDelLogic) AlertRuleDel(in *pb.DelAlertRuleReq) (*pb.DelAlertRuleResp, error) {
	// 检查规则是否存在
	_, err := l.svcCtx.AlertRulesModel.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, errorx.Msg("告警规则不存在")
		}
		return nil, errorx.Msg("查询告警规则失败")
	}

	// 软删除
	err = l.svcCtx.AlertRulesModel.DeleteSoft(l.ctx, in.Id)
	if err != nil {
		return nil, errorx.Msg("删除告警规则失败")
	}

	return &pb.DelAlertRuleResp{}, nil
}
