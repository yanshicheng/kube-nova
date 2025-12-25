package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertRuleBatchDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertRuleBatchDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertRuleBatchDelLogic {
	return &AlertRuleBatchDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AlertRuleBatchDel 批量删除告警规则
func (l *AlertRuleBatchDelLogic) AlertRuleBatchDel(in *pb.BatchDelAlertRuleReq) (*pb.BatchDelAlertRuleResp, error) {
	if len(in.Ids) == 0 {
		return nil, errorx.Msg("请选择要删除的告警规则")
	}

	var deletedCount int64
	for _, id := range in.Ids {
		err := l.svcCtx.AlertRulesModel.DeleteSoft(l.ctx, id)
		if err != nil {
			l.Logger.Errorf("删除告警规则 %d 失败: %v", id, err)
			continue
		}
		deletedCount++
	}

	return &pb.BatchDelAlertRuleResp{
		DeletedCount: deletedCount,
	}, nil
}
