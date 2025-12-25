package alert

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteAlertRuleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除告警规则
func NewDeleteAlertRuleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteAlertRuleLogic {
	return &DeleteAlertRuleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteAlertRuleLogic) DeleteAlertRule(req *types.DefaultIdRequest) (resp string, err error) {
	// 调用RPC服务删除告警规则
	_, err = l.svcCtx.ManagerRpc.AlertRuleDel(l.ctx, &pb.DelAlertRuleReq{
		Id: req.Id,
	})

	if err != nil {
		l.Errorf("删除告警规则失败: %v", err)
		return "", fmt.Errorf("删除告警规则失败: %v", err)
	}

	return "告警规则删除成功", nil
}
