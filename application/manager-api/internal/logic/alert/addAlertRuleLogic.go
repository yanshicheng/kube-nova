package alert

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type AddAlertRuleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建新的告警规则
func NewAddAlertRuleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddAlertRuleLogic {
	return &AddAlertRuleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddAlertRuleLogic) AddAlertRule(req *types.AddAlertRuleRequest) (resp string, err error) {
	// 获取当前用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用RPC服务添加告警规则
	_, err = l.svcCtx.ManagerRpc.AlertRuleAdd(l.ctx, &pb.AddAlertRuleReq{
		GroupId:     req.GroupId,
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
		CreatedBy:   username,
	})

	if err != nil {
		l.Errorf("添加告警规则失败: %v", err)
		return "", fmt.Errorf("添加告警规则失败: %v", err)
	}

	return "告警规则添加成功", nil
}
