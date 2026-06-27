package managerservicelogic

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CheckProjectDependenciesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCheckProjectDependenciesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CheckProjectDependenciesLogic {
	return &CheckProjectDependenciesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CheckProjectDependenciesLogic) CheckProjectDependencies(in *pb.ManagerCheckProjectDependenciesReq) (*pb.ManagerCheckProjectDependenciesResp, error) {
	var deps []*pb.ManagerProjectDependency

	// 检查 project_cluster
	clusters, err := l.svcCtx.OnecProjectClusterModel.SearchNoPage(l.ctx, "", true, "project_id = ?", in.ProjectId)
	if err == nil && len(clusters) > 0 {
		deps = append(deps, &pb.ManagerProjectDependency{
			ResourceType: "project_cluster",
			Count:        int64(len(clusters)),
			Message:      fmt.Sprintf("项目集群配额 (%d个)", len(clusters)),
		})
	}

	// 检查 project_admin
	admins, err := l.svcCtx.OnecProjectAdminModel.SearchNoPage(l.ctx, "", true, "project_id = ?", in.ProjectId)
	if err == nil && len(admins) > 0 {
		deps = append(deps, &pb.ManagerProjectDependency{
			ResourceType: "project_admin",
			Count:        int64(len(admins)),
			Message:      fmt.Sprintf("项目管理员 (%d个)", len(admins)),
		})
	}

	// 检查 project_audit_log
	logs, err := l.svcCtx.OnecProjectAuditLog.SearchNoPage(l.ctx, "", true, "project_id = ?", in.ProjectId)
	if err == nil && len(logs) > 0 {
		deps = append(deps, &pb.ManagerProjectDependency{
			ResourceType: "project_audit_log",
			Count:        int64(len(logs)),
			Message:      fmt.Sprintf("项目审计日志 (%d条)", len(logs)),
		})
	}

	l.Infof("项目依赖检查完成，projectId: %d, 依赖数: %d", in.ProjectId, len(deps))
	return &pb.ManagerCheckProjectDependenciesResp{Dependencies: deps}, nil
}
