package operator

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/rsync/types"
	"github.com/yanshicheng/kube-nova/common/k8smanager/cluster"
	"github.com/zeromicro/go-zero/core/logx"
)

const (
	MaxClusterConcurrency   = 5  // 最多同时处理5个集群
	MaxNamespaceConcurrency = 10 // 最多同时处理10个namespace
	MaxResourceConcurrency  = 50 // 最多同时处理50个资源实例
)

// 隐式 判断 ClusterResourceSync 是否满足SyncService 接口
var _ types.SyncService = (*ClusterResourceSync)(nil)

// ClusterResourceSyncConfig 集群资源同步服务配置
type ClusterResourceSyncConfig struct {
	K8sManager                  cluster.Manager
	ClusterModel                model.OnecClusterModel
	ClusterNodeModel            model.OnecClusterNodeModel
	ClusterResourceModel        model.OnecClusterResourceModel
	ClusterNetwork              model.OnecClusterNetworkModel
	ProjectModel                model.OnecProjectModel
	ProjectClusterResourceModel model.OnecProjectClusterModel
	ProjectWorkspaceModel       model.OnecProjectWorkspaceModel
	ProjectApplication          model.OnecProjectApplicationModel
	ProjectApplicationVersion   model.OnecProjectVersionModel
	ProjectAuditModel           model.OnecProjectAuditLogModel
}

// ClusterResourceSync 集群资源同步服务
type ClusterResourceSync struct {
	logx.Logger
	K8sManager                  cluster.Manager
	ClusterModel                model.OnecClusterModel
	ClusterNodeModel            model.OnecClusterNodeModel
	ClusterResourceModel        model.OnecClusterResourceModel
	ClusterNetwork              model.OnecClusterNetworkModel
	ProjectModel                model.OnecProjectModel
	ProjectClusterResourceModel model.OnecProjectClusterModel
	ProjectWorkspaceModel       model.OnecProjectWorkspaceModel
	ProjectApplication          model.OnecProjectApplicationModel
	ProjectApplicationVersion   model.OnecProjectVersionModel
	ProjectAuditModel           model.OnecProjectAuditLogModel
}

// NewClusterResourceSync 创建集群资源同步服务实例
func NewClusterResourceSync(cfg ClusterResourceSyncConfig) *ClusterResourceSync {
	ctx := context.Background()
	return &ClusterResourceSync{
		Logger:                      logx.WithContext(ctx),
		K8sManager:                  cfg.K8sManager,
		ClusterModel:                cfg.ClusterModel,
		ClusterNodeModel:            cfg.ClusterNodeModel,
		ClusterResourceModel:        cfg.ClusterResourceModel,
		ClusterNetwork:              cfg.ClusterNetwork,
		ProjectModel:                cfg.ProjectModel,
		ProjectClusterResourceModel: cfg.ProjectClusterResourceModel,
		ProjectWorkspaceModel:       cfg.ProjectWorkspaceModel,
		ProjectApplication:          cfg.ProjectApplication,
		ProjectApplicationVersion:   cfg.ProjectApplicationVersion,
		ProjectAuditModel:           cfg.ProjectAuditModel,
	}
}

// SyncOneProjectAllResource 同步某一个项目的所有资源信息
func (s *ClusterResourceSync) SyncOneProjectAllResource(ctx context.Context, projectId uint64, operator string, enableAudit bool) error {
	s.Logger.WithContext(ctx).Infof("开始同步项目资源, projectId: %d, operator: %s", projectId, operator)

	project, err := s.ProjectModel.FindOne(ctx, projectId)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("查询项目失败, projectId: %d, error: %v", projectId, err)
		return fmt.Errorf("查询项目失败: %v", err)
	}

	s.Logger.WithContext(ctx).Infof("项目信息: name=%s, uuid=%s, is_system=%d", project.Name, project.Uuid, project.IsSystem)

	err = s.ProjectModel.SyncAllProjectClusters(ctx, projectId)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("同步项目集群资源失败, projectId: %d, error: %v", projectId, err)
		if enableAudit {
			s.writeProjectAuditLog(ctx, 0, 0, 0, operator, "同步失败", "Project", "SYNC",
				fmt.Sprintf("项目[%s]资源同步失败: %v", project.Name, err), 2)
		}
		return fmt.Errorf("同步项目集群资源失败: %v", err)
	}

	if enableAudit {
		s.writeProjectAuditLog(ctx, 0, 0, 0, operator, "同步成功", "Project", "SYNC",
			fmt.Sprintf("项目[%s]资源同步完成", project.Name), 1)
	}

	s.Logger.WithContext(ctx).Infof("项目资源同步完成, projectId: %d", projectId)
	return nil
}

// SyncAllProjectAllResource 同步所有项目的所有资源信息
func (s *ClusterResourceSync) SyncAllProjectAllResource(ctx context.Context, operator string, enableAudit bool) error {
	s.Logger.WithContext(ctx).Infof("开始同步所有项目的资源, operator: %s", operator)

	projects, err := s.ProjectModel.SearchNoPage(ctx, "", false, "", nil)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("查询项目列表失败, error: %v", err)
		return fmt.Errorf("查询项目列表失败: %v", err)
	}

	s.Logger.WithContext(ctx).Infof("共查询到 %d 个项目", len(projects))

	if len(projects) == 0 {
		s.Logger.WithContext(ctx).Infof("没有项目需要同步")
		return nil
	}

	var (
		wg         sync.WaitGroup
		mu         sync.Mutex
		successCnt int
		failCnt    int
		syncErrors []string
		semaphore  = make(chan struct{}, MaxClusterConcurrency)
	)

	for _, project := range projects {
		wg.Add(1)
		go func(proj *model.OnecProject) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			err := s.SyncOneProjectAllResource(ctx, proj.Id, operator, false)

			mu.Lock()
			if err != nil {
				failCnt++
				syncErrors = append(syncErrors, fmt.Sprintf("项目[%s]", proj.Name))
			} else {
				successCnt++
			}
			mu.Unlock()
		}(project)
	}

	wg.Wait()

	s.Logger.WithContext(ctx).Infof("所有项目资源同步完成: 总数=%d, 成功=%d, 失败=%d", len(projects), successCnt, failCnt)

	if enableAudit {
		status := int64(1)
		auditDetail := fmt.Sprintf("同步所有项目资源: 总数=%d, 成功=%d, 失败=%d", len(projects), successCnt, failCnt)
		if failCnt > 0 {
			status = 2
			auditDetail = fmt.Sprintf("%s, 失败项目: %s", auditDetail, strings.Join(syncErrors, ", "))
		}
		s.writeProjectAuditLog(ctx, 0, 0, 0, operator, "批量同步", "Project", "SYNC_ALL", auditDetail, status)
	}

	if failCnt > 0 {
		return fmt.Errorf("部分项目同步失败: 成功=%d, 失败=%d", successCnt, failCnt)
	}

	return nil
}

// SyncOneClusterAllResource 同步某一个集群的资源（节点、版本、网络、资源会强制记录审计日志）
func (s *ClusterResourceSync) SyncOneClusterAllResource(ctx context.Context, clusterUuid string, operator string, enableAudit bool) error {
	s.Logger.WithContext(ctx).Infof("开始同步集群资源, clusterUuid: %s, operator: %s", clusterUuid, operator)

	onecCluster, err := s.ClusterModel.FindOneByUuid(ctx, clusterUuid)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("查询集群失败, clusterUuid: %s, error: %v", clusterUuid, err)
		return fmt.Errorf("查询集群失败: %v", err)
	}

	s.Logger.WithContext(ctx).Infof("集群信息: name=%s, uuid=%s, status=%d", onecCluster.Name, onecCluster.Uuid, onecCluster.Status)

	// 同步节点（强制记录审计日志，enableAudit参数在方法内部被忽略）
	if err = s.SyncClusterNodes(ctx, onecCluster.Uuid, operator, enableAudit); err != nil {
		s.Logger.WithContext(ctx).Errorf("同步集群节点失败, clusterUuid: %s, error: %v", clusterUuid, err)
	}

	// 同步版本（强制记录审计日志，enableAudit参数在方法内部被忽略）
	if err = s.SyncClusterVersion(ctx, onecCluster.Uuid, operator, enableAudit); err != nil {
		s.Logger.WithContext(ctx).Errorf("同步集群版本失败, clusterUuid: %s, error: %v", clusterUuid, err)
	}

	// 同步网络（强制记录审计日志，enableAudit参数在方法内部被忽略）
	if err = s.SyncClusterNetwork(ctx, onecCluster.Uuid, operator, enableAudit); err != nil {
		s.Logger.WithContext(ctx).Errorf("同步集群网络失败, clusterUuid: %s, error: %v", clusterUuid, err)
	}

	// 确保资源记录存在
	if err = s.ensureClusterResourceExists(ctx, onecCluster.Uuid); err != nil {
		s.Logger.WithContext(ctx).Errorf("确保集群资源记录存在失败, clusterUuid: %s, error: %v", clusterUuid, err)
	}

	// 同步资源（强制记录审计日志，enableAudit参数在方法内部被忽略）
	if err = s.SyncClusterResource(ctx, onecCluster.Uuid, operator, enableAudit); err != nil {
		s.Logger.WithContext(ctx).Errorf("同步集群物理资源失败, clusterUuid: %s, error: %v", clusterUuid, err)
	}

	// 同步统计
	if err = s.ClusterModel.SyncClusterResourceByClusterId(ctx, onecCluster.Id); err != nil {
		s.Logger.WithContext(ctx).Errorf("同步集群资源统计失败, clusterId: %d, error: %v", onecCluster.Id, err)
	}

	s.Logger.WithContext(ctx).Infof("集群资源同步完成, clusterId: %d", onecCluster.Id)
	return nil
}

// ensureClusterResourceExists 确保集群资源记录存在
func (s *ClusterResourceSync) ensureClusterResourceExists(ctx context.Context, clusterUuid string) error {
	_, err := s.ClusterResourceModel.FindOneByClusterUuid(ctx, clusterUuid)
	if err == nil {
		return nil
	}

	if errors.Is(err, model.ErrNotFound) {
		s.Logger.WithContext(ctx).Infof("集群资源记录不存在，创建新记录, clusterUuid: %s", clusterUuid)
		_, err = s.ClusterResourceModel.Insert(ctx, &model.OnecClusterResource{ClusterUuid: clusterUuid})
		if err != nil {
			return fmt.Errorf("创建集群资源记录失败: %v", err)
		}
		return nil
	}

	return fmt.Errorf("查询集群资源记录失败: %v", err)
}

// SyncAllClusterAllResource 同步所有集群的资源
func (s *ClusterResourceSync) SyncAllClusterAllResource(ctx context.Context, operator string, enableAudit bool) error {
	s.Logger.WithContext(ctx).Infof("开始同步所有集群的资源, operator: %s", operator)

	clusters, err := s.ClusterModel.GetAllClusters(ctx)
	if err != nil {
		return fmt.Errorf("查询集群列表失败: %v", err)
	}

	if len(clusters) == 0 {
		s.Logger.WithContext(ctx).Infof("没有集群需要同步")
		return nil
	}

	var (
		wg             sync.WaitGroup
		mu             sync.Mutex
		successCnt     int
		failCnt        int
		syncErrors     []string
		successCluster []string
		semaphore      = make(chan struct{}, MaxClusterConcurrency)
	)

	for _, onecCluster := range clusters {
		wg.Add(1)
		go func(cluster *model.OnecCluster) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			err := s.SyncOneClusterAllResource(ctx, cluster.Uuid, operator, false)

			mu.Lock()
			if err != nil {
				failCnt++
				syncErrors = append(syncErrors, cluster.Name)
			} else {
				successCnt++
				successCluster = append(successCluster, cluster.Name)
			}
			mu.Unlock()
		}(onecCluster)
	}

	wg.Wait()

	s.Logger.WithContext(ctx).Infof("所有集群资源同步完成: 总数=%d, 成功=%d, 失败=%d", len(clusters), successCnt, failCnt)

	if enableAudit {
		status := int64(1)
		auditContent := fmt.Sprintf("批量集群资源同步: 总数=%d, 成功=%d", len(clusters), successCnt)
		if failCnt > 0 {
			status = 2
			auditContent = fmt.Sprintf("%s, 失败=%d, 失败集群: %s", auditContent, failCnt, strings.Join(syncErrors, ", "))
		}
		s.writeClusterAuditLog(ctx, "", operator, auditContent, "批量资源同步", status)
	}

	if failCnt > 0 {
		return fmt.Errorf("部分集群同步失败: 成功=%d, 失败=%d", successCnt, failCnt)
	}

	return nil
}

// SyncAll 同步所有资源
func (s *ClusterResourceSync) SyncAll(ctx context.Context, operator string, enableAudit bool) error {
	s.Logger.WithContext(ctx).Infof("开始同步所有资源, operator: %s =========================", operator)
	timeNow := time.Now()

	var errMsgs []string
	var successMsgs []string

	if err := s.SyncAllClusterNamespaces(ctx, operator, false); err != nil {
		errMsgs = append(errMsgs, "命名空间")
	} else {
		successMsgs = append(successMsgs, "命名空间")
	}

	if err := s.SyncAllClustersWorkspaceResources(ctx, operator, false); err != nil {
		errMsgs = append(errMsgs, "工作空间配额")
	} else {
		successMsgs = append(successMsgs, "工作空间配额")
	}

	if err := s.SyncAllProjectAllResource(ctx, operator, false); err != nil {
		errMsgs = append(errMsgs, "项目配额")
	} else {
		successMsgs = append(successMsgs, "项目配额")
	}

	if err := s.SyncAllClusterApplications(ctx, operator, false); err != nil {
		errMsgs = append(errMsgs, "应用资源")
	} else {
		successMsgs = append(successMsgs, "应用资源")
	}

	if err := s.SyncAllClusterAllResource(ctx, operator, false); err != nil {
		errMsgs = append(errMsgs, "集群配额")
	} else {
		successMsgs = append(successMsgs, "集群配额")
	}

	duration := time.Since(timeNow)
	s.Logger.WithContext(ctx).Infof("资源同步耗时: %v", duration)

	if enableAudit {
		status := int64(1)
		auditContent := fmt.Sprintf("全量资源同步完成, 耗时: %v, 成功模块: %s", duration, strings.Join(successMsgs, ", "))
		if len(errMsgs) > 0 {
			status = 2
			auditContent = fmt.Sprintf("全量资源同步部分失败, 耗时: %v, 成功: %s, 失败: %s",
				duration, strings.Join(successMsgs, ", "), strings.Join(errMsgs, ", "))
		}
		s.writeClusterAuditLog(ctx, "", operator, auditContent, "全量资源同步", status)
	}

	if len(errMsgs) > 0 {
		return fmt.Errorf("全量同步部分失败: %v", errMsgs)
	}

	return nil
}

// SyncOneCLuster 同步单个集群的所有资源
func (s *ClusterResourceSync) SyncOneCLuster(ctx context.Context, clusterUuid string, operator string, enableAudit bool) error {
	s.Logger.WithContext(ctx).Infof("开始同步集群 %s, operator: %s", clusterUuid, operator)
	timeNow := time.Now()

	var clusterName string
	cluster, err := s.ClusterModel.FindOneByUuid(ctx, clusterUuid)
	if err != nil {
		clusterName = clusterUuid
	} else {
		clusterName = cluster.Name
	}

	var errMsgs []string
	var successMsgs []string

	if err := s.SyncClusterNamespaces(ctx, clusterUuid, operator, false); err != nil {
		errMsgs = append(errMsgs, "命名空间")
	} else {
		successMsgs = append(successMsgs, "命名空间")
	}

	if err := s.SyncAllWorkspaceResources(ctx, clusterUuid, operator, false); err != nil {
		errMsgs = append(errMsgs, "工作空间配额")
	} else {
		successMsgs = append(successMsgs, "工作空间配额")
	}

	if err := s.SyncClusterApplications(ctx, clusterUuid, operator, false); err != nil {
		errMsgs = append(errMsgs, "项目应用")
	} else {
		successMsgs = append(successMsgs, "项目应用")
	}

	if err := s.SyncOneClusterAllResource(ctx, clusterUuid, operator, false); err != nil {
		errMsgs = append(errMsgs, "配额资源")
	} else {
		successMsgs = append(successMsgs, "配额资源")
	}

	duration := time.Since(timeNow)
	s.Logger.WithContext(ctx).Infof("集群 %s 同步耗时: %v", clusterName, duration)

	if enableAudit {
		status := int64(1)
		auditContent := fmt.Sprintf("集群[%s]全量同步完成, 耗时: %v, 成功模块: %s",
			clusterName, duration, strings.Join(successMsgs, ", "))
		if len(errMsgs) > 0 {
			status = 2
			auditContent = fmt.Sprintf("集群[%s]全量同步部分失败, 耗时: %v, 成功: %s, 失败: %s",
				clusterName, duration, strings.Join(successMsgs, ", "), strings.Join(errMsgs, ", "))
		}
		s.writeClusterAuditLog(ctx, clusterUuid, operator, auditContent, "集群全量同步", status)
	}

	if len(errMsgs) > 0 {
		return fmt.Errorf("集群同步部分失败: %v", errMsgs)
	}

	return nil
}

// ==================== 审计日志方法 ====================

// writeClusterAuditLog 写集群审计日志
func (s *ClusterResourceSync) writeClusterAuditLog(ctx context.Context, clusterUuid, operator, auditContent string, auditType string, status int64) {
	var clusterName string

	if clusterUuid != "" {
		cluster, err := s.ClusterModel.FindOneByUuid(ctx, clusterUuid)
		if err == nil {
			clusterName = cluster.Name
		} else {
			clusterName = clusterUuid
		}
	} else {
		clusterName = "全局操作"
	}

	audit := &model.OnecProjectAuditLog{
		ClusterUuid:  clusterUuid,
		ClusterName:  clusterName,
		OperatorName: operator,
		Title:        auditType,
		ActionDetail: auditContent,
		Status:       status,
	}

	if _, err := s.ProjectAuditModel.Insert(ctx, audit); err != nil {
		s.Logger.WithContext(ctx).Errorf("写入集群审计日志失败: %v", err)
	}
}

// writeProjectAuditLog 写项目审计日志
func (s *ClusterResourceSync) writeProjectAuditLog(ctx context.Context, workspaceId, applicationId, projectClusterId uint64,
	operator, title, resourceType, action, actionDetail string, status int64) {

	audit := &model.OnecProjectAuditLog{
		OperatorName: operator,
		Title:        title,
		ActionDetail: actionDetail,
		Status:       status,
	}

	if projectClusterId > 0 {
		if projectCluster, err := s.ProjectClusterResourceModel.FindOne(ctx, projectClusterId); err == nil {
			audit.ProjectId = projectCluster.ProjectId
			if cluster, err := s.ClusterModel.FindOneByUuid(ctx, projectCluster.ClusterUuid); err == nil {
				audit.ClusterUuid = cluster.Uuid
				audit.ClusterName = cluster.Name
			}
			if project, err := s.ProjectModel.FindOne(ctx, projectCluster.ProjectId); err == nil {
				audit.ProjectName = project.Name
			}
		}
	}

	if workspaceId > 0 {
		if workspace, err := s.ProjectWorkspaceModel.FindOne(ctx, workspaceId); err == nil {
			audit.WorkspaceId = workspaceId
			audit.WorkspaceName = workspace.Name
		}
	}

	if applicationId > 0 {
		if application, err := s.ProjectApplication.FindOne(ctx, applicationId); err == nil {
			audit.ApplicationId = applicationId
			audit.ApplicationName = application.NameCn
		}
	}

	if _, err := s.ProjectAuditModel.Insert(ctx, audit); err != nil {
		s.Logger.WithContext(ctx).Errorf("写入项目审计日志失败: %v", err)
	}
}

// writeNodeChangeAuditLog 写节点变更审计日志
func (s *ClusterResourceSync) writeNodeChangeAuditLog(ctx context.Context, clusterUuid, clusterName, operator string,
	changeType string, nodeName string, changes []FieldChange, status int64) {

	var auditContent string
	var auditTitle string

	switch changeType {
	case "ADD":
		auditTitle = "节点新增"
		auditContent = fmt.Sprintf("集群[%s]新增节点: %s", clusterName, nodeName)
	case "ADD_FAILED":
		auditTitle = "节点新增失败"
		auditContent = fmt.Sprintf("集群[%s]新增节点失败: %s", clusterName, nodeName)
	case "UPDATE":
		auditTitle = "节点更新"
		if len(changes) > 0 {
			changeDesc := s.buildChangeDescription(changes)
			auditContent = fmt.Sprintf("集群[%s]节点[%s]配置变更: %s", clusterName, nodeName, changeDesc)
		} else {
			auditContent = fmt.Sprintf("集群[%s]节点[%s]配置更新", clusterName, nodeName)
		}
	case "UPDATE_FAILED":
		auditTitle = "节点更新失败"
		auditContent = fmt.Sprintf("集群[%s]节点[%s]更新失败", clusterName, nodeName)
	case "DELETE":
		auditTitle = "节点删除"
		auditContent = fmt.Sprintf("集群[%s]删除节点: %s", clusterName, nodeName)
	case "DELETE_FAILED":
		auditTitle = "节点删除失败"
		auditContent = fmt.Sprintf("集群[%s]删除节点失败: %s", clusterName, nodeName)
	default:
		auditTitle = "节点操作"
		auditContent = fmt.Sprintf("集群[%s]节点[%s]操作: %s", clusterName, nodeName, changeType)
	}

	audit := &model.OnecProjectAuditLog{
		ClusterUuid:  clusterUuid,
		ClusterName:  clusterName,
		OperatorName: operator,
		Title:        auditTitle,
		ActionDetail: auditContent,
		Status:       status,
	}

	if _, err := s.ProjectAuditModel.Insert(ctx, audit); err != nil {
		s.Logger.WithContext(ctx).Errorf("写入节点变更审计日志失败: %v", err)
	}
}
