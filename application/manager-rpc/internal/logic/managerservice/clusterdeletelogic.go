package managerservicelogic

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// 默认项目ID
const DefaultProjectId = 3

type ClusterDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewClusterDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterDeleteLogic {
	return &ClusterDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ProjectUsageInfo 项目使用信息
type ProjectUsageInfo struct {
	ProjectId   uint64
	ProjectName string
	Workspaces  []string // 工作空间名称列表
}

// ClusterDelete 删除集群
func (l *ClusterDeleteLogic) ClusterDelete(in *pb.DeleteClusterReq) (*pb.DeleteClusterResp, error) {
	// 1. 查询集群是否存在
	cluster, err := l.svcCtx.OnecClusterModel.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, fmt.Errorf("集群不存在")
		}
		return nil, fmt.Errorf("查询集群失败: %v", err)
	}

	// 2. 检查是否有非默认项目在使用此集群
	usageInfos, err := l.checkClusterUsage(cluster.Uuid)
	if err != nil {
		return nil, fmt.Errorf("检查集群使用情况失败: %v", err)
	}

	// 3. 如果有非默认项目在使用，返回详细错误信息
	if len(usageInfos) > 0 {
		errMsg := l.buildUsageErrorMessage(usageInfos)
		return nil, fmt.Errorf(errMsg)
	}

	// 4. 执行删除操作（事务）
	err = l.svcCtx.OnecClusterModel.TransCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		return l.deleteClusterCompletely(ctx, session, cluster)
	})
	if err != nil {
		return nil, fmt.Errorf("删除集群失败: %v", err)
	}

	// 5. 从 K8s Manager 中移除集群连接
	if l.svcCtx.K8sManager != nil {
		_ = l.svcCtx.K8sManager.RemoveCluster(cluster.Uuid)
	}

	l.Logger.Infof("集群 [%s] 删除成功", cluster.Name)

	return &pb.DeleteClusterResp{}, nil
}

// checkClusterUsage 检查集群使用情况，返回非默认项目的使用信息
func (l *ClusterDeleteLogic) checkClusterUsage(clusterUuid string) ([]ProjectUsageInfo, error) {
	// 查询使用此集群的项目集群关联（排除默认项目）
	projectClusters, err := l.svcCtx.OnecProjectClusterModel.SearchNoPage(
		l.ctx, "", true,
		"`cluster_uuid` = ? AND `project_id` != ?", clusterUuid, DefaultProjectId,
	)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}

	var usageInfos []ProjectUsageInfo

	for _, pc := range projectClusters {
		// 查询项目信息
		project, err := l.svcCtx.OnecProjectModel.FindOne(l.ctx, pc.ProjectId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				continue
			}
			return nil, fmt.Errorf("查询项目失败: %v", err)
		}

		// 查询该项目在此集群下的工作空间
		workspaces, err := l.svcCtx.OnecProjectWorkspaceModel.SearchNoPage(
			l.ctx, "", true,
			"`project_cluster_id` = ?", pc.Id,
		)
		if err != nil && !errors.Is(err, model.ErrNotFound) {
			return nil, fmt.Errorf("查询工作空间失败: %v", err)
		}

		var workspaceNames []string
		for _, ws := range workspaces {
			workspaceNames = append(workspaceNames, ws.Name)
		}

		usageInfos = append(usageInfos, ProjectUsageInfo{
			ProjectId:   project.Id,
			ProjectName: project.Name,
			Workspaces:  workspaceNames,
		})
	}

	return usageInfos, nil
}

// buildUsageErrorMessage 构建使用情况错误信息
func (l *ClusterDeleteLogic) buildUsageErrorMessage(usageInfos []ProjectUsageInfo) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("无法删除集群，当前有 %d 个项目正在使用此集群：\n", len(usageInfos)))

	for i, info := range usageInfos {
		sb.WriteString(fmt.Sprintf("%d. 项目「%s」", i+1, info.ProjectName))
		if len(info.Workspaces) > 0 {
			sb.WriteString(fmt.Sprintf("，包含 %d 个工作空间：%s",
				len(info.Workspaces),
				strings.Join(info.Workspaces, "、")))
		} else {
			sb.WriteString("（无工作空间）")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\n请先解绑以上项目后再删除集群。")
	return sb.String()
}

// deleteClusterCompletely 完全删除集群及其关联数据（硬删除）
func (l *ClusterDeleteLogic) deleteClusterCompletely(ctx context.Context, session sqlx.Session, cluster *model.OnecCluster) error {
	clusterUuid := cluster.Uuid

	// ========== 1. 删除默认项目下的关联数据 ==========
	// 查询默认项目在此集群下的 project_cluster
	defaultProjectCluster, err := l.svcCtx.OnecProjectClusterModel.FindOneByClusterUuidProjectId(ctx, clusterUuid, DefaultProjectId)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return fmt.Errorf("查询默认项目集群关联失败: %v", err)
	}

	if defaultProjectCluster != nil {
		// 1.1 删除版本 (version)
		if err := l.deleteVersionsByProjectCluster(ctx, session, defaultProjectCluster.Id); err != nil {
			return fmt.Errorf("删除版本失败: %v", err)
		}

		// 1.2 删除应用 (application)
		if err := l.deleteApplicationsByProjectCluster(ctx, session, defaultProjectCluster.Id); err != nil {
			return fmt.Errorf("删除应用失败: %v", err)
		}

		// 1.3 删除工作空间 (workspace)
		if err := l.deleteWorkspacesByProjectCluster(ctx, session, defaultProjectCluster.Id); err != nil {
			return fmt.Errorf("删除工作空间失败: %v", err)
		}

		// 1.4 删除项目集群关联 (project_cluster)
		_, err = l.svcCtx.OnecProjectClusterModel.TransOnSql(ctx, session, defaultProjectCluster.Id,
			"DELETE FROM {table} WHERE `id` = ?", defaultProjectCluster.Id)
		if err != nil {
			return fmt.Errorf("删除项目集群关联失败: %v", err)
		}
		l.Logger.Infof("删除默认项目集群关联 [id=%d]", defaultProjectCluster.Id)
	}

	// ========== 2. 删除集群相关数据 ==========
	// 2.1 删除集群应用配置 (cluster_app)
	if err := l.deleteClusterApps(ctx, session, clusterUuid); err != nil {
		return fmt.Errorf("删除集群应用配置失败: %v", err)
	}

	// 2.2 删除集群节点 (cluster_node)
	if err := l.deleteClusterNodes(ctx, session, clusterUuid); err != nil {
		return fmt.Errorf("删除集群节点失败: %v", err)
	}

	// 2.3 删除集群网络配置 (cluster_network)
	if err := l.deleteClusterNetwork(ctx, session, clusterUuid); err != nil {
		return fmt.Errorf("删除集群网络配置失败: %v", err)
	}

	// 2.4 删除集群资源配置 (cluster_resource)
	if err := l.deleteClusterResource(ctx, session, clusterUuid); err != nil {
		return fmt.Errorf("删除集群资源配置失败: %v", err)
	}

	// 2.5 删除集群认证信息 (cluster_auth)
	if err := l.deleteClusterAuth(ctx, session, clusterUuid); err != nil {
		return fmt.Errorf("删除集群认证信息失败: %v", err)
	}

	// 2.6 删除集群主表 (cluster)
	_, err = l.svcCtx.OnecClusterModel.TransOnSql(ctx, session, cluster.Id,
		"DELETE FROM {table} WHERE `id` = ?", cluster.Id)
	if err != nil {
		return fmt.Errorf("删除集群主表失败: %v", err)
	}
	l.Logger.Infof("删除集群 [%s] 主表记录", cluster.Name)

	return nil
}

// deleteVersionsByProjectCluster 删除项目集群下的所有版本
func (l *ClusterDeleteLogic) deleteVersionsByProjectCluster(ctx context.Context, session sqlx.Session, projectClusterId uint64) error {
	// 查询工作空间
	workspaces, err := l.svcCtx.OnecProjectWorkspaceModel.SearchNoPage(ctx, "", true,
		"`project_cluster_id` = ?", projectClusterId)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil
		}
		return err
	}

	for _, ws := range workspaces {
		// 查询应用
		apps, err := l.svcCtx.OnecProjectApplication.SearchNoPage(ctx, "", true,
			"`workspace_id` = ?", ws.Id)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				continue
			}
			return err
		}

		for _, app := range apps {
			// 查询版本
			versions, err := l.svcCtx.OnecProjectVersion.SearchNoPage(ctx, "", true,
				"`application_id` = ?", app.Id)
			if err != nil {
				if errors.Is(err, model.ErrNotFound) {
					continue
				}
				return err
			}

			for _, ver := range versions {
				_, err = l.svcCtx.OnecProjectVersion.TransOnSql(ctx, session, ver.Id,
					"DELETE FROM {table} WHERE `id` = ?", ver.Id)
				if err != nil {
					return err
				}
				l.Logger.Infof("删除版本 [id=%d, version=%s]", ver.Id, ver.Version)
			}
		}
	}
	return nil
}

// deleteApplicationsByProjectCluster 删除项目集群下的所有应用
func (l *ClusterDeleteLogic) deleteApplicationsByProjectCluster(ctx context.Context, session sqlx.Session, projectClusterId uint64) error {
	workspaces, err := l.svcCtx.OnecProjectWorkspaceModel.SearchNoPage(ctx, "", true,
		"`project_cluster_id` = ?", projectClusterId)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil
		}
		return err
	}

	for _, ws := range workspaces {
		apps, err := l.svcCtx.OnecProjectApplication.SearchNoPage(ctx, "", true,
			"`workspace_id` = ?", ws.Id)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				continue
			}
			return err
		}

		for _, app := range apps {
			_, err = l.svcCtx.OnecProjectApplication.TransOnSql(ctx, session, app.Id,
				"DELETE FROM {table} WHERE `id` = ?", app.Id)
			if err != nil {
				return err
			}
			l.Logger.Infof("删除应用 [id=%d, name=%s]", app.Id, app.NameEn)
		}
	}
	return nil
}

// deleteWorkspacesByProjectCluster 删除项目集群下的所有工作空间
func (l *ClusterDeleteLogic) deleteWorkspacesByProjectCluster(ctx context.Context, session sqlx.Session, projectClusterId uint64) error {
	workspaces, err := l.svcCtx.OnecProjectWorkspaceModel.SearchNoPage(ctx, "", true,
		"`project_cluster_id` = ?", projectClusterId)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil
		}
		return err
	}

	for _, ws := range workspaces {
		_, err = l.svcCtx.OnecProjectWorkspaceModel.TransOnSql(ctx, session, ws.Id,
			"DELETE FROM {table} WHERE `id` = ?", ws.Id)
		if err != nil {
			return err
		}
		l.Logger.Infof("删除工作空间 [id=%d, namespace=%s]", ws.Id, ws.Namespace)
	}
	return nil
}

// deleteClusterApps 删除集群应用配置
func (l *ClusterDeleteLogic) deleteClusterApps(ctx context.Context, session sqlx.Session, clusterUuid string) error {
	apps, err := l.svcCtx.OnecClusterAppModel.SearchNoPage(ctx, "", true,
		"`cluster_uuid` = ?", clusterUuid)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil
		}
		return err
	}

	for _, app := range apps {
		_, err = l.svcCtx.OnecClusterAppModel.TransOnSql(ctx, session, app.Id,
			"DELETE FROM {table} WHERE `id` = ?", app.Id)
		if err != nil {
			return err
		}
		l.Logger.Infof("删除集群应用配置 [id=%d, appCode=%s]", app.Id, app.AppCode)
	}
	return nil
}

// deleteClusterNodes 删除集群节点
func (l *ClusterDeleteLogic) deleteClusterNodes(ctx context.Context, session sqlx.Session, clusterUuid string) error {
	nodes, err := l.svcCtx.OnecClusterNodeModel.SearchNoPage(ctx, "", true,
		"`cluster_uuid` = ?", clusterUuid)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil
		}
		return err
	}

	for _, node := range nodes {
		_, err = l.svcCtx.OnecClusterNodeModel.TransOnSql(ctx, session, node.Id,
			"DELETE FROM {table} WHERE `id` = ?", node.Id)
		if err != nil {
			return err
		}
		l.Logger.Infof("删除集群节点 [id=%d, name=%s]", node.Id, node.Name)
	}
	return nil
}

// deleteClusterNetwork 删除集群网络配置
func (l *ClusterDeleteLogic) deleteClusterNetwork(ctx context.Context, session sqlx.Session, clusterUuid string) error {
	network, err := l.svcCtx.OnecClusterNetworkModel.FindOneByClusterUuid(ctx, clusterUuid)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil
		}
		return err
	}

	_, err = l.svcCtx.OnecClusterNetworkModel.TransOnSql(ctx, session, network.Id,
		"DELETE FROM {table} WHERE `id` = ?", network.Id)
	if err != nil {
		return err
	}
	l.Logger.Infof("删除集群网络配置 [id=%d]", network.Id)
	return nil
}

// deleteClusterResource 删除集群资源配置
func (l *ClusterDeleteLogic) deleteClusterResource(ctx context.Context, session sqlx.Session, clusterUuid string) error {
	resource, err := l.svcCtx.OnecClusterResourceModel.FindOneByClusterUuid(ctx, clusterUuid)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil
		}
		return err
	}

	_, err = l.svcCtx.OnecClusterResourceModel.TransOnSql(ctx, session, resource.Id,
		"DELETE FROM {table} WHERE `id` = ?", resource.Id)
	if err != nil {
		return err
	}
	l.Logger.Infof("删除集群资源配置 [id=%d]", resource.Id)
	return nil
}

// deleteClusterAuth 删除集群认证信息
func (l *ClusterDeleteLogic) deleteClusterAuth(ctx context.Context, session sqlx.Session, clusterUuid string) error {
	auth, err := l.svcCtx.OnecClusterAuthModel.FindOneByClusterUuid(ctx, clusterUuid)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil
		}
		return err
	}

	_, err = l.svcCtx.OnecClusterAuthModel.TransOnSql(ctx, session, auth.Id,
		"DELETE FROM {table} WHERE `id` = ?", auth.Id)
	if err != nil {
		return err
	}
	l.Logger.Infof("删除集群认证信息 [id=%d]", auth.Id)
	return nil
}
