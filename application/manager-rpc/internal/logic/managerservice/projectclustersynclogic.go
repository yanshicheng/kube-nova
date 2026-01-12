package managerservicelogic

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/common/utils"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ProjectClusterSyncLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectClusterSyncLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectClusterSyncLogic {
	return &ProjectClusterSyncLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ProjectClusterSync 同步项目集群配额（计算资源使用量）
func (l *ProjectClusterSyncLogic) ProjectClusterSync(in *pb.ProjectClusterSyncReq) (*pb.ProjectClusterSyncResp, error) {
	if in.Id <= 0 {
		l.Errorf("项目集群配额ID不能为空")
		return nil, errorx.Msg("项目集群配额ID不能为空")
	}

	projectResource, err := l.svcCtx.OnecProjectClusterModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("查询项目集群配额失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("项目集群配额不存在")
	}

	projectId := projectResource.ProjectId
	resourceId := in.Id
	svcCtx := l.svcCtx

	go func() {
		ctx := context.Background()
		logger := logx.WithContext(ctx)

		logger.Infof("开始异步同步项目集群配额 [ID:%d]", resourceId)

		err := svcCtx.OnecProjectModel.SyncAllProjectClusters(ctx, projectId)
		if err != nil {
			logger.Errorf("同步项目集群配额失败，ID: %d, 错误: %v", resourceId, err)
			return
		}

		logger.Infof("项目集群配额同步成功，ID: %d", resourceId)
	}()

	return &pb.ProjectClusterSyncResp{}, nil
}

// WorkspaceAllocatedStats 工作空间分配统计结构
type WorkspaceAllocatedStats struct {
	CpuAllocated              float64
	MemAllocated              float64
	StorageAllocated          float64
	GpuAllocated              float64
	PodsAllocated             int64
	ConfigmapAllocated        int64
	SecretAllocated           int64
	PvcAllocated              int64
	EphemeralStorageAllocated float64
	ServiceAllocated          int64
	LoadbalancersAllocated    int64
	NodeportsAllocated        int64
	DeploymentsAllocated      int64
	JobsAllocated             int64
	CronjobsAllocated         int64
	DaemonsetsAllocated       int64
	StatefulsetsAllocated     int64
	IngressesAllocated        int64
}

// calculateWorkspaceAllocatedStats 计算工作空间资源分配总量
func (l *ProjectClusterSyncLogic) calculateWorkspaceAllocatedStats(workspaces []*model.OnecProjectWorkspace) (*WorkspaceAllocatedStats, error) {
	stats := &WorkspaceAllocatedStats{}

	for _, ws := range workspaces {
		cpu, err := utils.CPUToCores(ws.CpuAllocated)
		if err != nil {
			return nil, fmt.Errorf("数值转换错误: %v", err.Error())
		}
		mem, err := utils.MemoryToGiB(ws.MemAllocated)
		if err != nil {
			return nil, fmt.Errorf("数值转换错误: %v", err.Error())
		}
		storage, err := utils.MemoryToGiB(ws.StorageAllocated)
		if err != nil {
			return nil, fmt.Errorf("数值转换错误: %v", err.Error())
		}
		gpu, err := utils.CPUToCores(ws.GpuAllocated)
		if err != nil {
			return nil, fmt.Errorf("数值转换错误: %v", err.Error())
		}
		ephStorage, err := utils.MemoryToGiB(ws.EphemeralStorageAllocated)
		if err != nil {
			return nil, fmt.Errorf("数值转换错误: %v", err.Error())
		}
		stats.CpuAllocated += cpu
		stats.MemAllocated += mem
		stats.StorageAllocated += storage
		stats.GpuAllocated += gpu
		stats.PodsAllocated += ws.PodsAllocated
		stats.ConfigmapAllocated += ws.ConfigmapAllocated
		stats.SecretAllocated += ws.SecretAllocated
		stats.PvcAllocated += ws.PvcAllocated
		stats.EphemeralStorageAllocated += ephStorage
		stats.ServiceAllocated += ws.ServiceAllocated
		stats.LoadbalancersAllocated += ws.LoadbalancersAllocated
		stats.NodeportsAllocated += ws.NodeportsAllocated
		stats.DeploymentsAllocated += ws.DeploymentsAllocated
		stats.JobsAllocated += ws.JobsAllocated
		stats.CronjobsAllocated += ws.CronjobsAllocated
		stats.DaemonsetsAllocated += ws.DaemonsetsAllocated
		stats.StatefulsetsAllocated += ws.StatefulsetsAllocated
		stats.IngressesAllocated += ws.IngressesAllocated
	}

	return stats, nil
}

// syncClusterResourceInTransaction 在事务中同步集群资源总量
func (l *ProjectClusterSyncLogic) syncClusterResourceInTransaction(ctx context.Context, session sqlx.Session, clusterUuid string) error {
	// 查询该集群的所有项目配额
	projectClusters, err := l.svcCtx.OnecProjectClusterModel.SearchNoPage(l.ctx, "", true, "cluster_uuid = ?", clusterUuid)
	if err != nil {
		return fmt.Errorf("查询集群项目配额失败: %v", err)
	}

	// 计算集群级别的资源统计
	clusterStats, err := l.calculateClusterResourceStats(projectClusters)
	if err != nil {
		return fmt.Errorf("计算集群资源统计失败: %v", err)
	}

	// 更新集群资源表
	updateClusterResourceSQL := `UPDATE onec_cluster_resource SET 
		cpu_allocated_total = ?, cpu_capacity_total = ?,
		mem_allocated_total = ?, mem_capacity_total = ?,
		storage_allocated_total = ?,
		gpu_allocated_total = ?, gpu_capacity_total = ?,
		pods_allocated_total = ?
	WHERE cluster_uuid = ? AND is_deleted = 0`

	// 获取集群资源记录ID用于缓存
	clusterResource, err := l.svcCtx.OnecClusterResourceModel.FindOneByClusterUuid(l.ctx, clusterUuid)
	if err != nil {
		return fmt.Errorf("查询集群资源记录失败: %v", err)
	}

	_, err = l.svcCtx.OnecClusterResourceModel.TransOnSql(ctx, session, clusterResource.Id, updateClusterResourceSQL,
		clusterStats.CpuAllocatedTotal, clusterStats.CpuCapacityTotal,
		clusterStats.MemAllocatedTotal, clusterStats.MemCapacityTotal,
		clusterStats.StorageAllocatedTotal,
		clusterStats.GpuAllocatedTotal, clusterStats.GpuCapacityTotal,
		clusterStats.PodsAllocatedTotal,
		clusterUuid,
	)

	return err
}

// ClusterResourceStats 集群资源统计结构
type ClusterResourceStats struct {
	CpuAllocatedTotal     float64
	CpuCapacityTotal      float64
	MemAllocatedTotal     float64
	MemCapacityTotal      float64
	StorageAllocatedTotal float64
	GpuAllocatedTotal     float64
	GpuCapacityTotal      float64
	PodsAllocatedTotal    int64
}

// calculateClusterResourceStats 计算集群资源统计
func (l *ProjectClusterSyncLogic) calculateClusterResourceStats(projectClusters []*model.OnecProjectCluster) (*ClusterResourceStats, error) {
	stats := &ClusterResourceStats{}

	for _, pc := range projectClusters {
		// 累加各项目的限额/容量（这代表分配给项目的总资源）
		cpuTotal, err := utils.CPUToCores(pc.CpuAllocated)
		if err != nil {
			return nil, fmt.Errorf("CPU转换失败: %v", err)
		}
		cpuCapacity, err := utils.CPUToCores(pc.CpuCapacity)
		if err != nil {
			return nil, fmt.Errorf("CPU转换失败: %v", err)
		}
		memTotal, err := utils.MemoryToGiB(pc.MemAllocated)
		if err != nil {
			return nil, fmt.Errorf("内存转换失败: %v", err)
		}
		memCapacity, err := utils.MemoryToGiB(pc.MemCapacity)
		if err != nil {
			return nil, fmt.Errorf("内存转换失败: %v", err)
		}
		storageTotal, err := utils.MemoryToGiB(pc.StorageAllocated)
		if err != nil {
			return nil, fmt.Errorf("存储转换失败: %v", err)
		}
		gpuTotal, err := utils.CPUToCores(pc.GpuAllocated)
		if err != nil {
			return nil, fmt.Errorf("GPU转换失败: %v", err)
		}
		gpuCapacity, err := utils.CPUToCores(pc.GpuCapacity)
		if err != nil {
			return nil, fmt.Errorf("GPU转换失败: %v", err)
		}
		stats.CpuAllocatedTotal += cpuTotal         // CPU实际分配总量
		stats.CpuCapacityTotal += cpuCapacity       // CPU超分后总容量
		stats.MemAllocatedTotal += memTotal         // 内存实际分配总量
		stats.MemCapacityTotal += memCapacity       // 内存超分后总容量
		stats.StorageAllocatedTotal += storageTotal // 存储分配总量
		stats.GpuAllocatedTotal += gpuTotal         // GPU实际分配总量
		stats.GpuCapacityTotal += gpuCapacity       // GPU超分后总容量
		stats.PodsAllocatedTotal += pc.PodsLimit    // Pod分配总量
	}

	return stats, nil
}

// SyncAllProjectClusters 同步所有项目集群配额（批量同步）
func (l *ProjectClusterSyncLogic) SyncAllProjectClusters(clusterUuid string) error {
	// 查询指定集群的所有项目配额
	projectClusters, err := l.svcCtx.OnecProjectClusterModel.SearchNoPage(l.ctx, "", true, "cluster_uuid = ?", clusterUuid)
	if err != nil {
		return fmt.Errorf("查询项目集群配额失败: %v", err)
	}

	// 逐个同步每个项目集群
	for _, pc := range projectClusters {
		req := &pb.ProjectClusterSyncReq{Id: pc.Id}
		_, err := l.ProjectClusterSync(req)
		if err != nil {
			l.Errorf("同步项目集群配额失败，ID: %d, 错误: %v", pc.Id, err)
			// 继续处理其他项目集群，不中断
			continue
		}
	}

	l.Infof("批量同步集群 %s 的项目配额完成，总计: %d", clusterUuid, len(projectClusters))
	return nil
}

// 辅助函数：转换ProjectCluster到Proto（保持原有函数）
func convertProjectClusterToProto(pc *model.OnecProjectCluster, clusterName string) *pb.OnecProjectCluster {
	return &pb.OnecProjectCluster{
		Id:                        pc.Id,
		ClusterUuid:               pc.ClusterUuid,
		ProjectId:                 pc.ProjectId,
		CpuLimit:                  pc.CpuLimit,
		CpuOvercommitRatio:        pc.CpuOvercommitRatio,
		CpuCapacity:               pc.CpuCapacity,
		CpuAllocated:              pc.CpuAllocated,
		MemLimit:                  pc.MemLimit,
		MemOvercommitRatio:        pc.MemOvercommitRatio,
		MemCapacity:               pc.MemCapacity,
		MemAllocated:              pc.MemAllocated,
		StorageLimit:              pc.StorageLimit,
		StorageAllocated:          pc.StorageAllocated,
		GpuLimit:                  pc.GpuLimit,
		GpuOvercommitRatio:        pc.GpuOvercommitRatio,
		GpuCapacity:               pc.GpuCapacity,
		GpuAllocated:              pc.GpuAllocated,
		PodsLimit:                 pc.PodsLimit,
		PodsAllocated:             pc.PodsAllocated,
		ConfigmapLimit:            pc.ConfigmapLimit,
		ConfigmapAllocated:        pc.ConfigmapAllocated,
		SecretLimit:               pc.SecretLimit,
		SecretAllocated:           pc.SecretAllocated,
		PvcLimit:                  pc.PvcLimit,
		PvcAllocated:              pc.PvcAllocated,
		EphemeralStorageLimit:     pc.EphemeralStorageLimit,
		EphemeralStorageAllocated: pc.EphemeralStorageAllocated,
		ServiceLimit:              pc.ServiceLimit,
		ServiceAllocated:          pc.ServiceAllocated,
		LoadbalancersLimit:        pc.LoadbalancersLimit,
		LoadbalancersAllocated:    pc.LoadbalancersAllocated,
		NodeportsLimit:            pc.NodeportsLimit,
		NodeportsAllocated:        pc.NodeportsAllocated,
		DeploymentsLimit:          pc.DeploymentsLimit,
		DeploymentsAllocated:      pc.DeploymentsAllocated,
		JobsLimit:                 pc.JobsLimit,
		JobsAllocated:             pc.JobsAllocated,
		CronjobsLimit:             pc.CronjobsLimit,
		CronjobsAllocated:         pc.CronjobsAllocated,
		DaemonsetsLimit:           pc.DaemonsetsLimit,
		DaemonsetsAllocated:       pc.DaemonsetsAllocated,
		StatefulsetsLimit:         pc.StatefulsetsLimit,
		StatefulsetsAllocated:     pc.StatefulsetsAllocated,
		IngressesLimit:            pc.IngressesLimit,
		IngressesAllocated:        pc.IngressesAllocated,
		CreatedBy:                 pc.CreatedBy,
		UpdatedBy:                 pc.UpdatedBy,
		CreatedAt:                 pc.CreatedAt.Unix(),
		UpdatedAt:                 pc.UpdatedAt.Unix(),
		ClusterName:               clusterName,
	}
}
