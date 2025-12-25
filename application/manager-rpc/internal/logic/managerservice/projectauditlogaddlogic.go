package managerservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectAuditLogAddLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectAuditLogAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectAuditLogAddLogic {
	return &ProjectAuditLogAddLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ProjectAuditLogAdd 添加项目审计日志
// 支持多种层级的审计日志：
// 1. 版本级别：传递 versionId
// 2. 应用级别：传递 applicationId
// 3. 工作空间级别：传递 workspaceId
// 4. 项目集群级别：传递 projectId + clusterUuid
// 5. 项目级别：传递 projectId
// 6. 集群级别：传递 clusterUuid
func (l *ProjectAuditLogAddLogic) ProjectAuditLogAdd(in *pb.AddOnecProjectAuditLogReq) (*pb.AddOnecProjectAuditLogResp, error) {
	// 从 context 中获取操作人信息
	operatorId, operatorName := l.getOperatorFromContext()

	var (
		clusterName     string
		clusterUuid     string
		projectId       uint64
		projectName     string
		workspaceId     uint64
		workspaceName   string
		applicationId   uint64
		applicationName string
	)

	// 场景1: 版本级别 - 通过 versionId 查询所有相关信息
	if in.VersionId != 0 {
		versionInfo, err := l.svcCtx.OnecProjectVersion.GetVersionDetailInfo(l.ctx, in.VersionId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Errorf("版本不存在 [versionId=%d]", in.VersionId)
				return nil, errorx.Msg("版本不存在")
			}
			l.Errorf("查询版本详细信息失败: %v", err)
			return nil, errorx.Msg("查询版本详细信息失败")
		}

		clusterUuid = versionInfo.ClusterUuid
		applicationId = versionInfo.ApplicationId
		workspaceId = versionInfo.WorkspaceId

		// 查询集群信息
		cluster, err := l.svcCtx.OnecClusterModel.FindOneByUuid(l.ctx, clusterUuid)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Errorf("集群不存在 [clusterUuid=%s]", clusterUuid)
				return nil, errorx.Msg("集群不存在")
			}
			l.Errorf("查询集群信息失败: %v", err)
			return nil, errorx.Msg("查询集群信息失败")
		}
		clusterName = cluster.Name

		// 查询应用信息
		application, err := l.svcCtx.OnecProjectApplication.FindOne(l.ctx, applicationId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Errorf("应用不存在 [applicationId=%d]", applicationId)
				return nil, errorx.Msg("应用不存在")
			}
			l.Errorf("查询应用信息失败: %v", err)
			return nil, errorx.Msg("查询应用信息失败")
		}
		applicationName = application.NameCn

		// 查询工作空间信息
		workspace, err := l.svcCtx.OnecProjectWorkspaceModel.FindOne(l.ctx, workspaceId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Errorf("工作空间不存在 [workspaceId=%d]", workspaceId)
				return nil, errorx.Msg("工作空间不存在")
			}
			l.Errorf("查询工作空间信息失败: %v", err)
			return nil, errorx.Msg("查询工作空间信息失败")
		}
		workspaceName = workspace.Name

		// 查询项目集群信息
		projectCluster, err := l.svcCtx.OnecProjectClusterModel.FindOne(l.ctx, workspace.ProjectClusterId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Errorf("项目集群关系不存在 [projectClusterId=%d]", workspace.ProjectClusterId)
				return nil, errorx.Msg("项目集群关系不存在")
			}
			l.Errorf("查询项目集群关系失败: %v", err)
			return nil, errorx.Msg("查询项目集群关系失败")
		}
		projectId = projectCluster.ProjectId

		// 查询项目信息
		project, err := l.svcCtx.OnecProjectModel.FindOne(l.ctx, projectId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Errorf("项目不存在 [projectId=%d]", projectId)
				return nil, errorx.Msg("项目不存在")
			}
			l.Errorf("查询项目信息失败: %v", err)
			return nil, errorx.Msg("查询项目信息失败")
		}
		projectName = project.Name

		l.Infof("版本级别审计日志：versionId=%d, cluster=%s, project=%s, workspace=%s, application=%s",
			in.VersionId, clusterName, projectName, workspaceName, applicationName)

	} else if in.ApplicationId != 0 {
		// 场景2: 应用级别 - 通过 applicationId 查询
		applicationId = in.ApplicationId

		// 查询应用信息
		application, err := l.svcCtx.OnecProjectApplication.FindOne(l.ctx, applicationId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Errorf("应用不存在 [applicationId=%d]", applicationId)
				return nil, errorx.Msg("应用不存在")
			}
			l.Errorf("查询应用信息失败: %v", err)
			return nil, errorx.Msg("查询应用信息失败")
		}
		applicationName = application.NameCn
		workspaceId = application.WorkspaceId

		// 查询工作空间信息
		workspace, err := l.svcCtx.OnecProjectWorkspaceModel.FindOne(l.ctx, workspaceId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Errorf("工作空间不存在 [workspaceId=%d]", workspaceId)
				return nil, errorx.Msg("工作空间不存在")
			}
			l.Errorf("查询工作空间信息失败: %v", err)
			return nil, errorx.Msg("查询工作空间信息失败")
		}
		workspaceName = workspace.Name
		clusterUuid = workspace.ClusterUuid

		// 查询集群信息
		cluster, err := l.svcCtx.OnecClusterModel.FindOneByUuid(l.ctx, clusterUuid)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Errorf("集群不存在 [clusterUuid=%s]", clusterUuid)
				return nil, errorx.Msg("集群不存在")
			}
			l.Errorf("查询集群信息失败: %v", err)
			return nil, errorx.Msg("查询集群信息失败")
		}
		clusterName = cluster.Name

		// 查询项目集群信息
		projectCluster, err := l.svcCtx.OnecProjectClusterModel.FindOne(l.ctx, workspace.ProjectClusterId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Errorf("项目集群关系不存在 [projectClusterId=%d]", workspace.ProjectClusterId)
				return nil, errorx.Msg("项目集群关系不存在")
			}
			l.Errorf("查询项目集群关系失败: %v", err)
			return nil, errorx.Msg("查询项目集群关系失败")
		}
		projectId = projectCluster.ProjectId

		// 查询项目信息
		project, err := l.svcCtx.OnecProjectModel.FindOne(l.ctx, projectId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Errorf("项目不存在 [projectId=%d]", projectId)
				return nil, errorx.Msg("项目不存在")
			}
			l.Errorf("查询项目信息失败: %v", err)
			return nil, errorx.Msg("查询项目信息失败")
		}
		projectName = project.Name

		l.Infof("应用级别审计日志：applicationId=%d, cluster=%s, project=%s, workspace=%s, application=%s",
			applicationId, clusterName, projectName, workspaceName, applicationName)

	} else if in.WorkspaceId != 0 {
		// 场景3: 工作空间级别 - 通过 workspaceId 查询
		workspaceId = in.WorkspaceId

		// 查询工作空间信息
		workspace, err := l.svcCtx.OnecProjectWorkspaceModel.FindOne(l.ctx, workspaceId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Errorf("工作空间不存在 [workspaceId=%d]", workspaceId)
				return nil, errorx.Msg("工作空间不存在")
			}
			l.Errorf("查询工作空间信息失败: %v", err)
			return nil, errorx.Msg("查询工作空间信息失败")
		}
		workspaceName = workspace.Name
		clusterUuid = workspace.ClusterUuid

		// 查询集群信息
		cluster, err := l.svcCtx.OnecClusterModel.FindOneByUuid(l.ctx, clusterUuid)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Errorf("集群不存在 [clusterUuid=%s]", clusterUuid)
				return nil, errorx.Msg("集群不存在")
			}
			l.Errorf("查询集群信息失败: %v", err)
			return nil, errorx.Msg("查询集群信息失败")
		}
		clusterName = cluster.Name

		// 查询项目集群信息
		projectCluster, err := l.svcCtx.OnecProjectClusterModel.FindOne(l.ctx, workspace.ProjectClusterId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Errorf("项目集群关系不存在 [projectClusterId=%d]", workspace.ProjectClusterId)
				return nil, errorx.Msg("项目集群关系不存在")
			}
			l.Errorf("查询项目集群关系失败: %v", err)
			return nil, errorx.Msg("查询项目集群关系失败")
		}
		projectId = projectCluster.ProjectId

		// 查询项目信息
		project, err := l.svcCtx.OnecProjectModel.FindOne(l.ctx, projectId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Errorf("项目不存在 [projectId=%d]", projectId)
				return nil, errorx.Msg("项目不存在")
			}
			l.Errorf("查询项目信息失败: %v", err)
			return nil, errorx.Msg("查询项目信息失败")
		}
		projectName = project.Name

		l.Infof("工作空间级别审计日志：workspaceId=%d, cluster=%s, project=%s, workspace=%s",
			workspaceId, clusterName, projectName, workspaceName)

	} else if in.ProjectId != 0 && in.ClusterUuid != "" {
		// 场景4: 项目集群级别 - 同时传递了 projectId 和 clusterUuid
		projectId = in.ProjectId
		clusterUuid = in.ClusterUuid

		// 查询项目信息
		project, err := l.svcCtx.OnecProjectModel.FindOne(l.ctx, projectId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Errorf("项目不存在 [projectId=%d]", projectId)
				return nil, errorx.Msg("项目不存在")
			}
			l.Errorf("查询项目信息失败: %v", err)
			return nil, errorx.Msg("查询项目信息失败")
		}
		projectName = project.Name

		// 查询集群信息
		cluster, err := l.svcCtx.OnecClusterModel.FindOneByUuid(l.ctx, clusterUuid)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Errorf("集群不存在 [clusterUuid=%s]", clusterUuid)
				return nil, errorx.Msg("集群不存在")
			}
			l.Errorf("查询集群信息失败: %v", err)
			return nil, errorx.Msg("查询集群信息失败")
		}
		clusterName = cluster.Name

		l.Infof("项目集群级别审计日志：projectId=%d, clusterUuid=%s, cluster=%s, project=%s",
			projectId, clusterUuid, clusterName, projectName)

	} else if in.ProjectId != 0 {
		// 场景5: 项目级别 - 只传递了 projectId
		projectId = in.ProjectId

		// 查询项目信息
		project, err := l.svcCtx.OnecProjectModel.FindOne(l.ctx, projectId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Errorf("项目不存在 [projectId=%d]", projectId)
				return nil, errorx.Msg("项目不存在")
			}
			l.Errorf("查询项目信息失败: %v", err)
			return nil, errorx.Msg("查询项目信息失败")
		}
		projectName = project.Name

		l.Infof("项目级别审计日志：projectId=%d, project=%s", projectId, projectName)

	} else if in.ClusterUuid != "" {
		// 场景6: 集群级别 - 只传递了 clusterUuid
		clusterUuid = in.ClusterUuid

		// 查询集群信息
		cluster, err := l.svcCtx.OnecClusterModel.FindOneByUuid(l.ctx, clusterUuid)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Errorf("集群不存在 [clusterUuid=%s]", clusterUuid)
				return nil, errorx.Msg("集群不存在")
			}
			l.Errorf("查询集群信息失败: %v", err)
			return nil, errorx.Msg("查询集群信息失败")
		}
		clusterName = cluster.Name

		l.Infof("集群级别审计日志：clusterUuid=%s, cluster=%s", clusterUuid, clusterName)

	} else {
		// 没有提供任何必要的标识符
		l.Errorf("参数校验失败: 必须提供 versionId、applicationId、workspaceId、projectId 或 clusterUuid 中的至少一个")
		return nil, errorx.Msg("参数不完整，无法创建审计日志")
	}

	// 构建审计日志数据
	auditLog := &model.OnecProjectAuditLog{
		ClusterName:     clusterName,
		ClusterUuid:     clusterUuid,
		ProjectId:       projectId,
		ProjectName:     projectName,
		WorkspaceId:     workspaceId,
		WorkspaceName:   workspaceName,
		ApplicationId:   applicationId,
		ApplicationName: applicationName,
		Title:           in.Title,
		ActionDetail:    in.ActionDetail,
		Status:          in.Status,
		OperatorId:      operatorId,
		OperatorName:    operatorName,
	}

	result, err := l.svcCtx.OnecProjectAuditLog.Insert(l.ctx, auditLog)
	if err != nil {
		l.Errorf("添加审计日志失败: %v, data: %+v", err, auditLog)
		return nil, errorx.Msg("添加审计日志失败")
	}

	id, err := result.LastInsertId()
	if err != nil {
		l.Errorf("获取审计日志ID失败: %v", err)
	} else {
		l.Infof("审计日志添加成功: id=%d, title=%s, operator=%s(id=%d)",
			id, in.Title, operatorName, operatorId)
	}

	return &pb.AddOnecProjectAuditLogResp{}, nil
}

// getOperatorFromContext 从 context 中获取操作人信息
// 返回: operatorId, operatorName
func (l *ProjectAuditLogAddLogic) getOperatorFromContext() (uint64, string) {
	var operatorId uint64
	var operatorName string

	// 尝试从 context 中获取 userId
	if userId, ok := l.ctx.Value("userId").(uint64); ok {
		operatorId = userId
	} else {
		operatorId = 0
		l.Error("无法从 context 中获取 userId，使用默认值 0")
	}

	// 尝试从 context 中获取 username
	if username, ok := l.ctx.Value("username").(string); ok {
		operatorName = username
	} else {
		operatorName = "system"
		l.Error("无法从 context 中获取 username，使用默认值 'system'")
	}

	return operatorId, operatorName
}
