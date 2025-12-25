package application

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ApplicationUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新应用
func NewApplicationUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApplicationUpdateLogic {
	return &ApplicationUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ApplicationUpdateLogic) ApplicationUpdate(req *types.UpdateOnecProjectApplicationReq) (resp string, err error) {
	username, _ := l.ctx.Value("username").(string)
	if username == "" {
		username = "system"
	}

	_, err = l.svcCtx.ManagerRpc.ApplicationUpdate(l.ctx, &managerservice.UpdateOnecProjectApplicationReq{
		Id:          req.Id,
		NameCn:      req.NameCn,
		Description: req.Description,
		UpdatedBy:   username,
	})
	if err != nil {
		l.Errorf("更新应用失败: %v", err)

		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ApplicationId: req.Id,
			Title:         "更新应用",
			ActionDetail:  fmt.Sprintf("更新应用信息失败: %s, 错误: %v", req.NameCn, err),
			Status:        0,
		})

		return "", err
	}

	// 记录成功的审计日志
	_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ApplicationId: req.Id,
		Title:         "更新应用",
		ActionDetail:  fmt.Sprintf("更新应用信息: %s", req.NameCn),
		Status:        1,
	})

	return "更新成功", nil
}
