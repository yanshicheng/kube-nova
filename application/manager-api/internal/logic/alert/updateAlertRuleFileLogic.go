package alert

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateAlertRuleFileLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新告警规则文件信息
func NewUpdateAlertRuleFileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateAlertRuleFileLogic {
	return &UpdateAlertRuleFileLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateAlertRuleFileLogic) UpdateAlertRuleFile(req *types.UpdateAlertRuleFileRequest) (resp string, err error) {
	// 获取当前用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用RPC服务更新告警规则文件
	_, err = l.svcCtx.ManagerRpc.AlertRuleFileUpdate(l.ctx, &pb.UpdateAlertRuleFileReq{
		Id:          req.Id,
		FileCode:    req.FileCode,
		FileName:    req.FileName,
		Description: req.Description,
		Namespace:   req.Namespace,
		Labels:      req.Labels,
		IsEnabled:   req.IsEnabled,
		SortOrder:   req.SortOrder,
		UpdatedBy:   username,
	})

	if err != nil {
		l.Errorf("更新告警规则文件失败: %v", err)
		return "", fmt.Errorf("更新告警规则文件失败: %v", err)
	}

	return "告警规则文件更新成功", nil
}
