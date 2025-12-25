package alert

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type BatchDeleteAlertRuleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 批量删除告警规则
func NewBatchDeleteAlertRuleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchDeleteAlertRuleLogic {
	return &BatchDeleteAlertRuleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BatchDeleteAlertRuleLogic) BatchDeleteAlertRule(req *types.BatchDeleteAlertRuleRequest) (resp string, err error) {
	// 调用RPC服务批量删除告警规则
	result, err := l.svcCtx.ManagerRpc.AlertRuleBatchDel(l.ctx, &pb.BatchDelAlertRuleReq{
		Ids: req.Ids,
	})

	if err != nil {
		l.Errorf("批量删除告警规则失败: %v", err)
		return "", fmt.Errorf("批量删除告警规则失败: %v", err)
	}

	return fmt.Sprintf("成功批量删除 %d 个告警规则", result.DeletedCount), nil
}
