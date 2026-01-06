package managerservicelogic

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/mr"
)

type GetClusterResourceRankingLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetClusterResourceRankingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetClusterResourceRankingLogic {
	return &GetClusterResourceRankingLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 集群排行项 - 基于 onec_cluster_resource 表
type clusterRankingItem struct {
	ClusterUuid    string
	ClusterName    string
	Environment    string
	Region         string
	Provider       string
	ProjectCount   int64
	WorkspaceCount int64
	// 物理资源（来自 onec_cluster_resource）
	CpuPhysical     float64 // cpu_physical_capacity
	MemPhysical     float64 // mem_physical_capacity
	GpuPhysical     float64 // gpu_physical_capacity
	StoragePhysical float64 // storage_physical_capacity
	PodsPhysical    int64   // pods_physical_capacity
	// 超分资源（来自 onec_cluster_resource）
	CpuCapacity float64 // cpu_capacity_total (超分后总容量)
	MemCapacity float64 // mem_capacity_total
	GpuCapacity float64 // gpu_capacity_total
	// 分配资源（来自 onec_cluster_resource）
	CpuAllocated     float64 // cpu_allocated_total
	MemAllocated     float64 // mem_allocated_total
	GpuAllocated     float64 // gpu_allocated_total
	StorageAllocated float64 // storage_allocated_total
	PodsAllocated    int64   // pods_allocated_total
}

// 获取集群资源排行
func (l *GetClusterResourceRankingLogic) GetClusterResourceRanking(in *pb.ClusterResourceRankingReq) (*pb.ClusterResourceRankingResp, error) {
	// 设置默认值
	topN := in.TopN
	if topN <= 0 {
		topN = 10
	}

	// 解析排序字段和方向
	sortBy, sortDirection := l.parseSortBy(in.SortBy)

	l.Infof("集群排行参数: sortBy=%s, sortDirection=%s, topN=%d, projectId=%d",
		sortBy, sortDirection, topN, in.ProjectId)

	// 从 onec_cluster_resource 表获取数据
	items, err := l.getClusterResourceData(in.ProjectId)
	if err != nil {
		l.Errorf("获取集群资源数据失败: %v", err)
		return nil, errorx.Msg("获取集群排行数据失败")
	}

	// 排序
	l.sortItems(items, sortBy, sortDirection)

	// 限制数量
	total := int64(len(items))
	if int64(len(items)) > topN {
		items = items[:topN]
	}

	// 构建响应
	resp := &pb.ClusterResourceRankingResp{
		Items: make([]*pb.ClusterRankingItem, 0, len(items)),
		Total: total,
	}

	for idx, item := range items {
		rankItem := l.buildRankingItem(int64(idx+1), item)
		resp.Items = append(resp.Items, rankItem)
	}

	return resp, nil
}

// 解析排序字段和方向
func (l *GetClusterResourceRankingLogic) parseSortBy(sortByStr string) (string, string) {
	if sortByStr == "" {
		return "cpuCapacity", "desc"
	}

	parts := strings.Split(sortByStr, ":")
	sortBy := parts[0]
	sortDirection := "desc"
	if len(parts) > 1 && (parts[1] == "asc" || parts[1] == "desc") {
		sortDirection = parts[1]
	}

	// 验证排序字段是否有效
	validFields := map[string]bool{
		"cpuCapacity": true, "cpuLimit": true, "cpuAllocated": true,
		"memCapacityGib": true, "memLimitGib": true, "memAllocatedGib": true,
		"gpuCapacity": true, "gpuLimit": true, "gpuAllocated": true,
		"storageLimitGib": true, "storageAllocatedGib": true,
		"podsLimit": true, "podsAllocated": true,
		// 兼容旧的简写
		"cpu": true, "mem": true, "gpu": true, "storage": true, "pod": true,
	}

	if !validFields[sortBy] {
		sortBy = "cpuCapacity"
	}

	return sortBy, sortDirection
}

// 从 onec_cluster_resource 表获取集群资源数据
func (l *GetClusterResourceRankingLogic) getClusterResourceData(projectId uint64) ([]*clusterRankingItem, error) {
	// 查询所有集群资源记录
	clusterResources, err := l.svcCtx.OnecClusterResourceModel.SearchNoPage(l.ctx, "", false, "")
	if err != nil && err.Error() != "record not found" {
		return nil, err
	}

	if len(clusterResources) == 0 {
		return []*clusterRankingItem{}, nil
	}

	// 收集所有集群 UUID
	uuids := make([]string, 0, len(clusterResources))
	for _, cr := range clusterResources {
		uuids = append(uuids, cr.ClusterUuid)
	}

	// 批量获取集群信息
	clusterInfos := l.batchGetClusterInfos(uuids)

	// 如果指定了 projectId，需要过滤只包含该项目的集群
	var filteredUuids map[string]bool
	if projectId > 0 {
		filteredUuids = l.getClusterUuidsByProject(projectId)
	}

	// 获取每个集群的项目数和工作空间数（排除默认项目）
	projectCounts, workspaceCounts := l.getClusterCounts(uuids)

	// 构建结果
	items := make([]*clusterRankingItem, 0, len(clusterResources))
	for _, cr := range clusterResources {
		// 如果指定了 projectId，过滤不包含该项目的集群
		if filteredUuids != nil && !filteredUuids[cr.ClusterUuid] {
			continue
		}

		item := &clusterRankingItem{
			ClusterUuid: cr.ClusterUuid,
			// 物理资源
			CpuPhysical:     cr.CpuPhysicalCapacity,
			MemPhysical:     cr.MemPhysicalCapacity,
			GpuPhysical:     cr.GpuPhysicalCapacity,
			StoragePhysical: cr.StoragePhysicalCapacity,
			PodsPhysical:    cr.PodsPhysicalCapacity,
			// 超分资源（capacity_total 是超分后的总容量）
			CpuCapacity: cr.CpuCapacityTotal,
			MemCapacity: cr.MemCapacityTotal,
			GpuCapacity: cr.GpuCapacityTotal,
			// 分配资源（allocated_total 是实际分配的资源）
			CpuAllocated:     cr.CpuAllocatedTotal,
			MemAllocated:     cr.MemAllocatedTotal,
			GpuAllocated:     cr.GpuAllocatedTotal,
			StorageAllocated: cr.StorageAllocatedTotal,
			PodsAllocated:    cr.PodsAllocatedTotal,
			// 统计数量
			ProjectCount:   projectCounts[cr.ClusterUuid],
			WorkspaceCount: workspaceCounts[cr.ClusterUuid],
		}

		// 填充集群信息
		if info, exists := clusterInfos[cr.ClusterUuid]; exists {
			item.ClusterName = info.Name
			item.Environment = info.Environment
			item.Region = info.Region
			item.Provider = info.Provider
		}

		items = append(items, item)
	}

	return items, nil
}

// 根据项目ID获取关联的集群UUID列表
func (l *GetClusterResourceRankingLogic) getClusterUuidsByProject(projectId uint64) map[string]bool {
	result := make(map[string]bool)

	projectClusters, err := l.svcCtx.OnecProjectClusterModel.SearchNoPage(l.ctx, "", false, "project_id = ?", projectId)
	if err != nil {
		return result
	}

	for _, pc := range projectClusters {
		result[pc.ClusterUuid] = true
	}

	return result
}

// 获取每个集群的项目数和工作空间数（排除默认项目 ID=3）
func (l *GetClusterResourceRankingLogic) getClusterCounts(uuids []string) (map[string]int64, map[string]int64) {
	projectCounts := make(map[string]int64)
	workspaceCounts := make(map[string]int64)

	// 查询所有非默认项目的 project_cluster 记录
	projectClusters, err := l.svcCtx.OnecProjectClusterModel.SearchNoPage(
		l.ctx, "", false, "project_id != ?", DefaultProjectId)
	if err != nil {
		return projectCounts, workspaceCounts
	}

	// 统计每个集群的项目数，同时收集 project_cluster_id
	pcIdToClusterUuid := make(map[uint64]string)
	for _, pc := range projectClusters {
		projectCounts[pc.ClusterUuid]++
		pcIdToClusterUuid[pc.Id] = pc.ClusterUuid
	}

	// 查询工作空间数量
	if len(pcIdToClusterUuid) > 0 {
		// 构建 project_cluster_id 列表
		pcIds := make([]string, 0, len(pcIdToClusterUuid))
		for id := range pcIdToClusterUuid {
			pcIds = append(pcIds, strconv.Itoa(int(id)))
		}

		wsQueryStr := "project_cluster_id IN (" + strings.Join(pcIds, ",") + ")"
		workspaces, err := l.svcCtx.OnecProjectWorkspaceModel.SearchNoPage(l.ctx, "", false, wsQueryStr)
		if err == nil {
			for _, ws := range workspaces {
				if clusterUuid, exists := pcIdToClusterUuid[ws.ProjectClusterId]; exists {
					workspaceCounts[clusterUuid]++
				}
			}
		}
	}

	return projectCounts, workspaceCounts
}

// 集群信息结构
type clusterInfos struct {
	Name        string
	Environment string
	Region      string
	Provider    string
}

// 批量获取集群信息
func (l *GetClusterResourceRankingLogic) batchGetClusterInfos(uuids []string) map[string]*clusterInfos {
	result := make(map[string]*clusterInfos)
	if len(uuids) == 0 {
		return result
	}

	type clusterInfoResult struct {
		Uuid        string
		Name        string
		Environment string
		Region      string
		Provider    string
	}

	mr.MapReduceVoid(
		func(source chan<- string) {
			for _, uuid := range uuids {
				source <- uuid
			}
		},
		func(uuid string, writer mr.Writer[*clusterInfoResult], cancel func(error)) {
			cluster, err := l.svcCtx.OnecClusterModel.FindOneByUuid(l.ctx, uuid)
			if err == nil {
				writer.Write(&clusterInfoResult{
					Uuid:        uuid,
					Name:        cluster.Name,
					Environment: cluster.Environment,
					Region:      cluster.Region,
					Provider:    cluster.Provider,
				})
			}
		},
		func(pipe <-chan *clusterInfoResult, cancel func(error)) {
			for item := range pipe {
				result[item.Uuid] = &clusterInfos{
					Name:        item.Name,
					Environment: item.Environment,
					Region:      item.Region,
					Provider:    item.Provider,
				}
			}
		},
	)

	return result
}

// 排序
func (l *GetClusterResourceRankingLogic) sortItems(items []*clusterRankingItem, sortBy string, sortDirection string) {
	isDesc := sortDirection == "desc"

	sort.Slice(items, func(i, j int) bool {
		var vi, vj float64

		switch sortBy {
		// 物理资源
		case "cpuCapacity", "cpu":
			vi, vj = items[i].CpuPhysical, items[j].CpuPhysical
		case "memCapacityGib", "mem":
			vi, vj = items[i].MemPhysical, items[j].MemPhysical
		case "gpuCapacity", "gpu":
			vi, vj = items[i].GpuPhysical, items[j].GpuPhysical
		// 超分资源
		case "cpuLimit":
			vi, vj = items[i].CpuCapacity, items[j].CpuCapacity
		case "memLimitGib":
			vi, vj = items[i].MemCapacity, items[j].MemCapacity
		case "gpuLimit":
			vi, vj = items[i].GpuCapacity, items[j].GpuCapacity
		// 分配资源
		case "cpuAllocated":
			vi, vj = items[i].CpuAllocated, items[j].CpuAllocated
		case "memAllocatedGib":
			vi, vj = items[i].MemAllocated, items[j].MemAllocated
		case "gpuAllocated":
			vi, vj = items[i].GpuAllocated, items[j].GpuAllocated
		// 存储
		case "storageLimitGib", "storage":
			vi, vj = items[i].StoragePhysical, items[j].StoragePhysical
		case "storageAllocatedGib":
			vi, vj = items[i].StorageAllocated, items[j].StorageAllocated
		// Pod
		case "podsLimit", "pod":
			vi, vj = float64(items[i].PodsPhysical), float64(items[j].PodsPhysical)
		case "podsAllocated":
			vi, vj = float64(items[i].PodsAllocated), float64(items[j].PodsAllocated)
		default:
			vi, vj = items[i].CpuPhysical, items[j].CpuPhysical
		}

		if isDesc {
			return vi > vj
		}
		return vi < vj
	})
}

// 构建排行项
func (l *GetClusterResourceRankingLogic) buildRankingItem(rank int64, item *clusterRankingItem) *pb.ClusterRankingItem {
	rankItem := &pb.ClusterRankingItem{
		Rank:           rank,
		ClusterUuid:    item.ClusterUuid,
		ClusterName:    item.ClusterName,
		Environment:    item.Environment,
		Region:         item.Region,
		Provider:       item.Provider,
		ProjectCount:   item.ProjectCount,
		WorkspaceCount: item.WorkspaceCount,
		// 物理资源 -> Capacity 字段
		CpuCapacity:    item.CpuPhysical,
		MemCapacityGib: item.MemPhysical,
		GpuCapacity:    item.GpuPhysical,
		// 超分资源 -> Limit 字段
		CpuLimit:        item.CpuCapacity,
		MemLimitGib:     item.MemCapacity,
		GpuLimit:        item.GpuCapacity,
		StorageLimitGib: item.StoragePhysical, // 存储没有超分，用物理值
		PodsLimit:       item.PodsPhysical,    // Pod没有超分，用物理值
		// 分配资源 -> Allocated 字段
		CpuAllocated:        item.CpuAllocated,
		MemAllocatedGib:     item.MemAllocated,
		GpuAllocated:        item.GpuAllocated,
		StorageAllocatedGib: item.StorageAllocated,
		PodsAllocated:       item.PodsAllocated,
	}

	// 计算分配率（分配/超分）
	if item.CpuCapacity > 0 {
		rankItem.CpuAllocationRate = (item.CpuAllocated / item.CpuCapacity) * 100
	}
	if item.MemCapacity > 0 {
		rankItem.MemAllocationRate = (item.MemAllocated / item.MemCapacity) * 100
	}
	if item.GpuCapacity > 0 {
		rankItem.GpuAllocationRate = (item.GpuAllocated / item.GpuCapacity) * 100
	}
	if item.StoragePhysical > 0 {
		rankItem.StorageAllocationRate = (item.StorageAllocated / item.StoragePhysical) * 100
	}
	if item.PodsPhysical > 0 {
		rankItem.PodsAllocationRate = (float64(item.PodsAllocated) / float64(item.PodsPhysical)) * 100
	}

	// 计算超分率（超分/物理）
	if item.CpuPhysical > 0 {
		rankItem.CpuOversellRate = (item.CpuCapacity / item.CpuPhysical) * 100
	}
	if item.MemPhysical > 0 {
		rankItem.MemOversellRate = (item.MemCapacity / item.MemPhysical) * 100
	}
	if item.GpuPhysical > 0 {
		rankItem.GpuOversellRate = (item.GpuCapacity / item.GpuPhysical) * 100
	}

	return rankItem
}
