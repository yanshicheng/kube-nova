package alert

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type AddAlertRuleGroupLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建新的告警规则分组
func NewAddAlertRuleGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddAlertRuleGroupLogic {
	return &AddAlertRuleGroupLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddAlertRuleGroupLogic) AddAlertRuleGroup(req *types.AddAlertRuleGroupRequest) (resp string, err error) {
	// 获取当前用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用RPC服务添加告警规则分组
	_, err = l.svcCtx.ManagerRpc.AlertRuleGroupAdd(l.ctx, &pb.AddAlertRuleGroupReq{
		FileId:      req.FileId,
		GroupCode:   req.GroupCode,
		GroupName:   req.GroupName,
		Description: req.Description,
		Interval:    req.Interval,
		IsEnabled:   req.IsEnabled,
		SortOrder:   req.SortOrder,
		CreatedBy:   username,
	})

	if err != nil {
		l.Errorf("添加告警规则分组失败: %v", err)
		return "", fmt.Errorf("添加告警规则分组失败: %v", err)
	}

	return "告警规则分组添加成功", nil
}
