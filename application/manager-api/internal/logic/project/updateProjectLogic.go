package project

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateProjectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// NewUpdateProjectLogic 更新项目基本信息
func NewUpdateProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateProjectLogic {
	return &UpdateProjectLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateProjectLogic) UpdateProject(req *types.UpdateProjectRequest) (resp string, err error) {
	// 获取当前用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 先查询项目原信息用于审计日志对比
	oldProjectInfo, queryErr := l.svcCtx.ManagerRpc.ProjectGetById(l.ctx, &pb.GetOnecProjectByIdReq{
		Id: req.Id,
	})

	var oldName string
	var oldDescription string
	if queryErr == nil && oldProjectInfo.Data != nil {
		oldName = oldProjectInfo.Data.Name
		oldDescription = oldProjectInfo.Data.Description
	}

	// 调用RPC服务更新项目
	_, err = l.svcCtx.ManagerRpc.ProjectUpdate(l.ctx, &pb.UpdateOnecProjectReq{
		Id:          req.Id,
		Name:        req.Name,
		Description: req.Description,
		UpdatedBy:   username,
	})

	// 记录审计日志
	auditStatus := int64(1)
	if err != nil {
		auditStatus = 0
	}
	actionDetail := fmt.Sprintf("用户 %s 更新项目, 项目ID: %d, 名称: %s -> %s, 描述: %s -> %s",
		username, req.Id, oldName, req.Name, oldDescription, req.Description)
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ProjectId:    uint64(req.Id),
		Title:        "更新项目",
		ActionDetail: actionDetail,
		Status:       auditStatus,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	if err != nil {
		l.Errorf("更新项目失败: %v", err)
		return "", fmt.Errorf("更新项目失败: %v", err)
	}

	return "项目更新成功", nil
}
