package alert

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteAlertRuleGroupLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除告警规则分组
func NewDeleteAlertRuleGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteAlertRuleGroupLogic {
	return &DeleteAlertRuleGroupLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteAlertRuleGroupLogic) DeleteAlertRuleGroup(req *types.DeleteAlertRuleGroupRequest) (resp string, err error) {
	// 调用RPC服务删除告警规则分组
	_, err = l.svcCtx.ManagerRpc.AlertRuleGroupDel(l.ctx, &pb.DelAlertRuleGroupReq{
		Id:      req.Id,
		Cascade: req.Cascade,
	})

	if err != nil {
		l.Errorf("删除告警规则分组失败: %v", err)
		return "", fmt.Errorf("删除告警规则分组失败: %v", err)
	}

	return "告警规则分组删除成功", nil
}
