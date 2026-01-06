package managerservicelogic

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/common/utils"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/mr"
)

type GetProjectResourceRankingLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetProjectResourceRankingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProjectResourceRankingLogic {
	return &GetProjectResourceRankingLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 项目排行项 - 按项目+集群维度
type projectRankingItem struct {
	ProjectClusterId uint64
	ProjectId        uint64
	ProjectName      string
	ProjectUuid      string
	IsSystem         bool
	ClusterUuid      string
	ClusterName      string
	WorkspaceCount   int64

	// CPU 相关 - 全部转换为 float64 数值
	CpuAllocated float64 // 已分配
	CpuLimit     float64 // 配额
	CpuCapacity  float64 // 超分容量（用于排序）

	// 内存相关 (GiB)
	MemAllocated float64
	MemLimit     float64
	MemCapacity  float64 // 超分容量（用于排序）

	// GPU 相关
	GpuAllocated float64
	GpuLimit     float64
	GpuCapacity  float64 // 超分容量（用于排序）

	// 存储相关 (GiB)
	StorageAllocated float64
	StorageLimit     float64

	// Pod 相关
	PodsAllocated int64
	PodsLimit     int64
}

// GetProjectResourceRanking 获取项目资源排行
func (l *GetProjectResourceRankingLogic) GetProjectResourceRanking(in *pb.ProjectResourceRankingReq) (*pb.ProjectResourceRankingResp, error) {
	// 设置默认值
	topN := in.TopN
	if topN <= 0 {
		topN = 10
	}

	// 解析排序字段和方向，格式: "fieldName:direction" 或 "fieldName"
	sortBy, sortDirection := l.parseSortBy(in.SortBy)

	// 打印日志确认参数
	l.Infof("项目排行参数: sortBy=%s, sortDirection=%s, topN=%d, clusterUuid=%s", sortBy, sortDirection, topN, in.ClusterUuid)

	// 获取项目-集群维度的数据
	items, err := l.getProjectClusterData(in.ClusterUuid)
	if err != nil {
		l.Errorf("获取项目排行数据失败: %v", err)
		return nil, errorx.Msg("获取项目排行数据失败")
	}

	// 按指定字段排序
	l.sortItems(items, sortBy, sortDirection)

	// 计算总量（用于占比计算）
	totalCapacity := l.calculateTotalCapacity(items, sortBy)

	// 限制数量
	total := int64(len(items))
	if int64(len(items)) > topN {
		items = items[:topN]
	}

	// 构建响应
	resp := &pb.ProjectResourceRankingResp{
		Items: make([]*pb.ProjectRankingItem, 0, len(items)),
		Total: total,
	}

	for idx, item := range items {
		rankItem := l.buildRankingItem(int64(idx+1), item, sortBy, totalCapacity)
		resp.Items = append(resp.Items, rankItem)
	}

	return resp, nil
}

// 解析排序字段和方向
func (l *GetProjectResourceRankingLogic) parseSortBy(sortByStr string) (string, string) {
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
		"primaryClusterName": true,
		"cpuAllocated":       true, "cpuCapacity": true,
		"memAllocatedGib": true, "memCapacityGib": true,
		"gpuAllocated": true, "gpuCapacity": true,
		"storageLimitGib": true, "podsLimit": true,
		// 兼容旧的简写
		"cpu": true, "mem": true, "gpu": true, "storage": true, "pod": true,
	}

	if !validFields[sortBy] {
		sortBy = "cpuCapacity"
	}

	return sortBy, sortDirection
}

// getProjectClusterData 获取项目-集群维度的数据
func (l *GetProjectResourceRankingLogic) getProjectClusterData(clusterUuid string) ([]*projectRankingItem, error) {
	// 构建查询条件
	var queryStr string
	var args []any
	hasCluster := clusterUuid != "" && clusterUuid != "all"
	if hasCluster {
		queryStr = "cluster_uuid = ?"
		args = []any{clusterUuid}
	}

	projectClusters, err := l.svcCtx.OnecProjectClusterModel.SearchNoPage(l.ctx, "", false, queryStr, args...)
	if err != nil && err.Error() != "record not found" {
		return nil, fmt.Errorf("查询项目集群数据失败: %v", err)
	}

	// 收集需要查询的项目ID和集群UUID
	projectIds := make(map[uint64]bool)
	clusterUuids := make(map[string]bool)
	projectClusterIds := make([]uint64, 0)

	// 每个 ProjectCluster 记录作为一行
	items := make([]*projectRankingItem, 0, len(projectClusters))

	for _, pc := range projectClusters {
		// 过滤默认项目 ID = 3
		if pc.ProjectId == DefaultProjectId {
			continue
		}

		projectIds[pc.ProjectId] = true
		clusterUuids[pc.ClusterUuid] = true
		projectClusterIds = append(projectClusterIds, pc.Id)

		// 使用 utils 函数转换字符串为数值
		cpuAllocated, _ := utils.CPUToCores(pc.CpuAllocated)
		cpuLimit, _ := utils.CPUToCores(pc.CpuLimit)
		cpuCapacity, _ := utils.CPUToCores(pc.CpuCapacity)

		memAllocated, _ := utils.MemoryToGiB(pc.MemAllocated)
		memLimit, _ := utils.MemoryToGiB(pc.MemLimit)
		memCapacity, _ := utils.MemoryToGiB(pc.MemCapacity)

		gpuAllocated, _ := utils.GPUToCount(pc.GpuAllocated)
		gpuLimit, _ := utils.GPUToCount(pc.GpuLimit)
		gpuCapacity, _ := utils.GPUToCount(pc.GpuCapacity)

		storageAllocated, _ := utils.MemoryToGiB(pc.StorageAllocated)
		storageLimit, _ := utils.MemoryToGiB(pc.StorageLimit)

		item := &projectRankingItem{
			ProjectClusterId: pc.Id,
			ProjectId:        pc.ProjectId,
			ClusterUuid:      pc.ClusterUuid,
			// CPU
			CpuAllocated: cpuAllocated,
			CpuLimit:     cpuLimit,
			CpuCapacity:  cpuCapacity,
			// 内存
			MemAllocated: memAllocated,
			MemLimit:     memLimit,
			MemCapacity:  memCapacity,
			// GPU
			GpuAllocated: gpuAllocated,
			GpuLimit:     gpuLimit,
			GpuCapacity:  gpuCapacity,
			// 存储
			StorageAllocated: storageAllocated,
			StorageLimit:     storageLimit,
			// Pod
			PodsAllocated: pc.PodsAllocated,
			PodsLimit:     pc.PodsLimit,
		}

		items = append(items, item)
	}

	// 批量查询项目信息和集群信息
	projectInfos := l.batchGetProjectInfos(l.mapKeysToUint64Slice(projectIds))
	clusterInfos := l.batchGetClusterInfos(l.mapKeysToStringSlice(clusterUuids))

	// 查询每个项目-集群组合的工作空间数量
	workspaceCounts := l.countWorkspacesByProjectCluster(projectClusterIds)

	// 填充名称信息
	for _, item := range items {
		if info, exists := projectInfos[item.ProjectId]; exists {
			item.ProjectName = info.Name
			item.ProjectUuid = info.Uuid
			item.IsSystem = info.IsSystem
		}
		if info, exists := clusterInfos[item.ClusterUuid]; exists {
			item.ClusterName = info.Name
		}
		if count, exists := workspaceCounts[item.ProjectClusterId]; exists {
			item.WorkspaceCount = count
		}
	}

	return items, nil
}

// 排序 - 支持多字段和排序方向
func (l *GetProjectResourceRankingLogic) sortItems(items []*projectRankingItem, sortBy string, sortDirection string) {
	isDesc := sortDirection == "desc"

	sort.Slice(items, func(i, j int) bool {
		var result bool

		switch sortBy {
		case "primaryClusterName":
			// 字符串排序
			if isDesc {
				result = items[i].ClusterName > items[j].ClusterName
			} else {
				result = items[i].ClusterName < items[j].ClusterName
			}
			return result
		case "cpuAllocated":
			if isDesc {
				return items[i].CpuAllocated > items[j].CpuAllocated
			}
			return items[i].CpuAllocated < items[j].CpuAllocated
		case "cpuCapacity", "cpu":
			if isDesc {
				return items[i].CpuCapacity > items[j].CpuCapacity
			}
			return items[i].CpuCapacity < items[j].CpuCapacity
		case "memAllocatedGib":
			if isDesc {
				return items[i].MemAllocated > items[j].MemAllocated
			}
			return items[i].MemAllocated < items[j].MemAllocated
		case "memCapacityGib", "mem":
			if isDesc {
				return items[i].MemCapacity > items[j].MemCapacity
			}
			return items[i].MemCapacity < items[j].MemCapacity
		case "gpuAllocated":
			if isDesc {
				return items[i].GpuAllocated > items[j].GpuAllocated
			}
			return items[i].GpuAllocated < items[j].GpuAllocated
		case "gpuCapacity", "gpu":
			if isDesc {
				return items[i].GpuCapacity > items[j].GpuCapacity
			}
			return items[i].GpuCapacity < items[j].GpuCapacity
		case "storageLimitGib", "storage":
			if isDesc {
				return items[i].StorageLimit > items[j].StorageLimit
			}
			return items[i].StorageLimit < items[j].StorageLimit
		case "podsLimit", "pod":
			if isDesc {
				return items[i].PodsLimit > items[j].PodsLimit
			}
			return items[i].PodsLimit < items[j].PodsLimit
		default:
			// 默认按 CpuCapacity 排序
			if isDesc {
				return items[i].CpuCapacity > items[j].CpuCapacity
			}
			return items[i].CpuCapacity < items[j].CpuCapacity
		}
	})
}

// 计算总量（用于占比计算）
func (l *GetProjectResourceRankingLogic) calculateTotalCapacity(items []*projectRankingItem, sortBy string) float64 {
	var total float64
	for _, item := range items {
		switch sortBy {
		case "mem", "memCapacityGib", "memAllocatedGib":
			total += item.MemCapacity
		case "gpu", "gpuCapacity", "gpuAllocated":
			total += item.GpuCapacity
		case "storage", "storageLimitGib":
			total += item.StorageLimit
		case "pod", "podsLimit":
			total += float64(item.PodsLimit)
		default: // cpu, cpuCapacity, cpuAllocated
			total += item.CpuCapacity
		}
	}
	return total
}

// 获取排序字段的值（用于占比计算）
func (l *GetProjectResourceRankingLogic) getSortValue(item *projectRankingItem, sortBy string) float64 {
	switch sortBy {
	case "mem", "memCapacityGib", "memAllocatedGib":
		return item.MemCapacity
	case "gpu", "gpuCapacity", "gpuAllocated":
		return item.GpuCapacity
	case "storage", "storageLimitGib":
		return item.StorageLimit
	case "pod", "podsLimit":
		return float64(item.PodsLimit)
	default: // cpu, cpuCapacity, cpuAllocated
		return item.CpuCapacity
	}
}

// countWorkspacesByProjectCluster 统计每个项目-集群组合的工作空间数量
func (l *GetProjectResourceRankingLogic) countWorkspacesByProjectCluster(projectClusterIds []uint64) map[uint64]int64 {
	result := make(map[uint64]int64)
	if len(projectClusterIds) == 0 {
		return result
	}

	ids := make([]string, 0, len(projectClusterIds))
	for _, id := range projectClusterIds {
		ids = append(ids, fmt.Sprintf("%d", id))
	}
	wsQueryStr := fmt.Sprintf("project_cluster_id IN (%s)", joinStringsForProject(ids, ","))

	workspaces, err := l.svcCtx.OnecProjectWorkspaceModel.SearchNoPage(l.ctx, "", false, wsQueryStr)
	if err != nil {
		return result
	}

	for _, ws := range workspaces {
		result[ws.ProjectClusterId]++
	}

	return result
}

func joinStringsForProject(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

type projectFullInfo struct {
	Name     string
	Uuid     string
	IsSystem bool
}

type clusterInfo struct {
	Name string
}

func (l *GetProjectResourceRankingLogic) batchGetProjectInfos(ids []uint64) map[uint64]*projectFullInfo {
	result := make(map[uint64]*projectFullInfo)
	if len(ids) == 0 {
		return result
	}

	mr.MapReduceVoid(
		func(source chan<- uint64) {
			for _, id := range ids {
				source <- id
			}
		},
		func(id uint64, writer mr.Writer[*projectFullInfoResult], cancel func(error)) {
			project, err := l.svcCtx.OnecProjectModel.FindOne(l.ctx, id)
			if err == nil {
				writer.Write(&projectFullInfoResult{
					Id:       id,
					Name:     project.Name,
					Uuid:     project.Uuid,
					IsSystem: project.IsSystem == 1,
				})
			}
		},
		func(pipe <-chan *projectFullInfoResult, cancel func(error)) {
			for item := range pipe {
				result[item.Id] = &projectFullInfo{
					Name:     item.Name,
					Uuid:     item.Uuid,
					IsSystem: item.IsSystem,
				}
			}
		},
	)

	return result
}

type projectFullInfoResult struct {
	Id       uint64
	Name     string
	Uuid     string
	IsSystem bool
}

func (l *GetProjectResourceRankingLogic) batchGetClusterInfos(uuids []string) map[string]*clusterInfo {
	result := make(map[string]*clusterInfo)
	if len(uuids) == 0 {
		return result
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
					Uuid: uuid,
					Name: cluster.Name,
				})
			}
		},
		func(pipe <-chan *clusterInfoResult, cancel func(error)) {
			for item := range pipe {
				result[item.Uuid] = &clusterInfo{
					Name: item.Name,
				}
			}
		},
	)

	return result
}

type clusterInfoResult struct {
	Uuid string
	Name string
}

func (l *GetProjectResourceRankingLogic) mapKeysToUint64Slice(m map[uint64]bool) []uint64 {
	result := make([]uint64, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}

func (l *GetProjectResourceRankingLogic) mapKeysToStringSlice(m map[string]bool) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}

// 构建排行项 - 返回所有字段
func (l *GetProjectResourceRankingLogic) buildRankingItem(rank int64, item *projectRankingItem, sortBy string, totalCapacity float64) *pb.ProjectRankingItem {
	rankItem := &pb.ProjectRankingItem{
		Rank:               rank,
		ProjectId:          item.ProjectId,
		ProjectName:        item.ProjectName,
		ProjectUuid:        item.ProjectUuid,
		IsSystem:           item.IsSystem,
		ClusterCount:       1,
		WorkspaceCount:     item.WorkspaceCount,
		PrimaryClusterUuid: item.ClusterUuid,
		PrimaryClusterName: item.ClusterName,

		// CPU 三个字段
		CpuAllocated: item.CpuAllocated, // 已分配
		CpuLimit:     item.CpuLimit,     // 配额
		CpuCapacity:  item.CpuCapacity,  // 超分容量

		// 内存 三个字段 (GiB)
		MemAllocatedGib: item.MemAllocated,
		MemLimitGib:     item.MemLimit,
		MemCapacityGib:  item.MemCapacity,

		// GPU 三个字段
		GpuAllocated: item.GpuAllocated,
		GpuLimit:     item.GpuLimit,
		GpuCapacity:  item.GpuCapacity,

		// 存储 (GiB)
		StorageAllocatedGib: item.StorageAllocated,
		StorageLimitGib:     item.StorageLimit,

		// Pod
		PodsAllocated: item.PodsAllocated,
		PodsLimit:     item.PodsLimit,
	}

	// 计算使用率 (Allocated / Limit)
	if item.CpuLimit > 0 {
		rankItem.CpuUsageRate = (item.CpuAllocated / item.CpuLimit) * 100
	}
	if item.MemLimit > 0 {
		rankItem.MemUsageRate = (item.MemAllocated / item.MemLimit) * 100
	}
	if item.GpuLimit > 0 {
		rankItem.GpuUsageRate = (item.GpuAllocated / item.GpuLimit) * 100
	}
	if item.StorageLimit > 0 {
		rankItem.StorageUsageRate = (item.StorageAllocated / item.StorageLimit) * 100
	}
	if item.PodsLimit > 0 {
		rankItem.PodsUsageRate = (float64(item.PodsAllocated) / float64(item.PodsLimit)) * 100
	}

	// 计算占比（当前项 / 总量）
	if totalCapacity > 0 {
		sortValue := l.getSortValue(item, sortBy)
		rankItem.OverallUsageRate = (sortValue / totalCapacity) * 100
	}

	return rankItem
}
