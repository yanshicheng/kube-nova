package project

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type AddProjectAdminLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// NewAddProjectAdminLogic 批量添加项目管理员，使用事务先删除所有管理员再批量添加
func NewAddProjectAdminLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddProjectAdminLogic {
	return &AddProjectAdminLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddProjectAdminLogic) AddProjectAdmin(req *types.AddProjectAdminRequest) (resp string, err error) {
	// 获取当前用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用RPC服务添加项目管理员
	_, err = l.svcCtx.ManagerRpc.ProjectAdminAdd(l.ctx, &pb.AddOnecProjectAdminReq{
		ProjectId: req.ProjectId,
		UserIds:   req.UserIds,
	})

	// 记录审计日志
	auditStatus := int64(1)
	if err != nil {
		auditStatus = 0
	}
	userIdsStr := make([]string, len(req.UserIds))
	for i, id := range req.UserIds {
		userIdsStr[i] = fmt.Sprintf("%d", id)
	}
	actionDetail := fmt.Sprintf("用户 %s 设置项目管理员, 项目ID: %d, 管理员用户ID列表: [%s]",
		username, req.ProjectId, strings.Join(userIdsStr, ", "))
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ProjectId:    uint64(req.ProjectId),
		Title:        "设置项目管理员",
		ActionDetail: actionDetail,
		Status:       auditStatus,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	if err != nil {
		l.Errorf("添加项目管理员失败: %v", err)
		return "", fmt.Errorf("添加项目管理员失败: %v", err)
	}

	return "项目管理员添加成功", nil
}
