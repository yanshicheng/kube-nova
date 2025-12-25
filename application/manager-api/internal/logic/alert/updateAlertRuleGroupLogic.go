package alert

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateAlertRuleGroupLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新告警规则分组信息
func NewUpdateAlertRuleGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateAlertRuleGroupLogic {
	return &UpdateAlertRuleGroupLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateAlertRuleGroupLogic) UpdateAlertRuleGroup(req *types.UpdateAlertRuleGroupRequest) (resp string, err error) {
	// 获取当前用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用RPC服务更新告警规则分组
	_, err = l.svcCtx.ManagerRpc.AlertRuleGroupUpdate(l.ctx, &pb.UpdateAlertRuleGroupReq{
		Id:          req.Id,
		GroupCode:   req.GroupCode,
		GroupName:   req.GroupName,
		Description: req.Description,
		Interval:    req.Interval,
		IsEnabled:   req.IsEnabled,
		SortOrder:   req.SortOrder,
		UpdatedBy:   username,
	})

	if err != nil {
		l.Errorf("更新告警规则分组失败: %v", err)
		return "", fmt.Errorf("更新告警规则分组失败: %v", err)
	}

	return "告警规则分组更新成功", nil
}
