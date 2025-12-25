package alert

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type AddAlertRuleFileLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建新的告警规则文件
func NewAddAlertRuleFileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddAlertRuleFileLogic {
	return &AddAlertRuleFileLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddAlertRuleFileLogic) AddAlertRuleFile(req *types.AddAlertRuleFileRequest) (resp string, err error) {
	// 获取当前用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用RPC服务添加告警规则文件
	_, err = l.svcCtx.ManagerRpc.AlertRuleFileAdd(l.ctx, &pb.AddAlertRuleFileReq{
		FileCode:    req.FileCode,
		FileName:    req.FileName,
		Description: req.Description,
		Namespace:   req.Namespace,
		Labels:      req.Labels,
		IsEnabled:   req.IsEnabled,
		SortOrder:   req.SortOrder,
		CreatedBy:   username,
	})

	if err != nil {
		l.Errorf("添加告警规则文件失败: %v", err)
		return "", fmt.Errorf("添加告警规则文件失败: %v", err)
	}

	return "告警规则文件添加成功", nil
}
