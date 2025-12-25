package alert

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateAlertRuleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新告警规则信息
func NewUpdateAlertRuleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateAlertRuleLogic {
	return &UpdateAlertRuleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateAlertRuleLogic) UpdateAlertRule(req *types.UpdateAlertRuleRequest) (resp string, err error) {
	// 获取当前用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用RPC服务更新告警规则
	_, err = l.svcCtx.ManagerRpc.AlertRuleUpdate(l.ctx, &pb.UpdateAlertRuleReq{
		Id:          req.Id,
		AlertName:   req.AlertName,
		RuleNameCn:  req.RuleNameCn,
		Expr:        req.Expr,
		ForDuration: req.ForDuration,
		Severity:    req.Severity,
		Summary:     req.Summary,
		Description: req.Description,
		Labels:      req.Labels,
		Annotations: req.Annotations,
		IsEnabled:   req.IsEnabled,
		SortOrder:   req.SortOrder,
		UpdatedBy:   username,
	})

	if err != nil {
		l.Errorf("更新告警规则失败: %v", err)
		return "", fmt.Errorf("更新告警规则失败: %v", err)
	}

	return "告警规则更新成功", nil
}
