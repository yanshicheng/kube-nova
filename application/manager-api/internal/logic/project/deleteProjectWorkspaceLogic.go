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

type DeleteProjectWorkspaceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// NewDeleteProjectWorkspaceLogic 软删除工作空间，并同步更新项目集群资源使用量
func NewDeleteProjectWorkspaceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteProjectWorkspaceLogic {
	return &DeleteProjectWorkspaceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteProjectWorkspaceLogic) DeleteProjectWorkspace(req *types.DefaultIdRequest) (resp string, err error) {
	// 获取当前用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 先查询工作空间信息用于审计日志
	workspaceInfo, queryErr := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &pb.GetOnecProjectWorkspaceByIdReq{
		Id: req.Id,
	})

	var clusterUuid string
	var workspaceName string
	var namespace string
	if queryErr == nil && workspaceInfo.Data != nil {
		clusterUuid = workspaceInfo.Data.ClusterUuid
		workspaceName = workspaceInfo.Data.Name
		namespace = workspaceInfo.Data.Namespace
	}

	// 调用RPC服务删除项目工作空间
	_, err = l.svcCtx.ManagerRpc.ProjectWorkspaceDel(l.ctx, &pb.DelOnecProjectWorkspaceReq{
		Id: req.Id,
	})

	// 记录审计日志
	auditStatus := int64(1)
	if err != nil {
		auditStatus = 0
	}
	actionDetail := fmt.Sprintf("用户 %s 删除项目工作空间, 工作空间ID: %d, 名称: %s, 命名空间: %s, 集群UUID: %s",
		username, req.Id, workspaceName, namespace, clusterUuid)
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  clusterUuid,
		WorkspaceId:  uint64(0),
		Title:        "删除项目工作空间",
		ActionDetail: actionDetail,
		Status:       auditStatus,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	if err != nil {
		l.Errorf("删除项目工作空间失败: %v", err)
		return "", fmt.Errorf("删除项目工作空间失败: %v", err)
	}

	return "项目工作空间删除成功", nil
}
