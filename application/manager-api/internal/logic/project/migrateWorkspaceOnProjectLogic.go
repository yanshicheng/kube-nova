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

type MigrateWorkspaceOnProjectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// NewMigrateWorkspaceOnProjectLogic 同集群迁移工作空间
func NewMigrateWorkspaceOnProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MigrateWorkspaceOnProjectLogic {
	return &MigrateWorkspaceOnProjectLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MigrateWorkspaceOnProjectLogic) MigrateWorkspaceOnProject(req *types.MigrateWorkspaceOnProjectRequest) (resp string, err error) {
	// 获取当前用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 先查询工作空间信息用于审计日志
	workspaceInfo, queryErr := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &pb.GetOnecProjectWorkspaceByIdReq{
		Id: req.WorkloadId,
	})

	var clusterUuid string
	var workspaceName string
	if queryErr == nil && workspaceInfo.Data != nil {
		clusterUuid = workspaceInfo.Data.ClusterUuid
		workspaceName = workspaceInfo.Data.Name
	}

	_, err = l.svcCtx.ManagerRpc.MigrateWorkspace(l.ctx, &pb.MigrateWorkspaceReq{
		NewProjectId: req.NewProjectId,
		WorkspaceId:  req.WorkloadId,
	})

	// 记录审计日志
	auditStatus := int64(1)
	if err != nil {
		auditStatus = 0
	}
	actionDetail := fmt.Sprintf("用户 %s 迁移工作空间, 工作空间ID: %d, 名称: %s, 迁移到新项目ID: %d",
		username, req.WorkloadId, workspaceName, req.NewProjectId)
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  clusterUuid,
		WorkspaceId:  uint64(req.WorkloadId),
		Title:        "迁移工作空间",
		ActionDetail: actionDetail,
		Status:       auditStatus,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	if err != nil {
		l.Errorf("MigrateWorkspaceOnProject error: %v", err)
		return "", err
	}
	return "success", nil
}
