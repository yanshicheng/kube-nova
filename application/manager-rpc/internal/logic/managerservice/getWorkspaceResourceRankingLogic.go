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

type GetWorkspaceResourceRankingLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetWorkspaceResourceRankingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetWorkspaceResourceRankingLogic {
	return &GetWorkspaceResourceRankingLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 工作空间排行项 - 工作空间表只有 Allocated 字段，没有 Limit 和 Capacity
type workspaceRankingItem struct {
	WorkspaceId      uint64
	WorkspaceName    string
	Namespace        string
	ProjectClusterId uint64
	ProjectId        uint64
	ProjectName      string
	ClusterUuid      string
	ClusterName      string
	CpuAllocated     float64
	MemAllocated     float64
	GpuAllocated     float64
	StorageAllocated float64
	PodsAllocated    int64
}

// GetWorkspaceResourceRanking 获取工作空间资源排行
func (l *GetWorkspaceResourceRankingLogic) GetWorkspaceResourceRanking(in *pb.WorkspaceResourceRankingReq) (*pb.WorkspaceResourceRankingResp, error) {
	// 设置默认值
	topN := in.TopN
	if topN <= 0 {
		topN = 10
	}

	// 解析排序字段和方向，格式: "fieldName:direction" 或 "fieldName"
	sortBy, sortDirection := l.parseSortBy(in.SortBy)

	// 打印日志确认参数
	l.Infof("工作空间排行参数: sortBy=%s, sortDirection=%s, topN=%d, clusterUuid=%s, projectId=%d",
		sortBy, sortDirection, topN, in.ClusterUuid, in.ProjectId)

	// 聚合数据
	items, totalAllocated, err := l.aggregateWorkspaceData(in.ClusterUuid, in.ProjectId, sortBy)
	if err != nil {
		l.Errorf("聚合工作空间排行数据失败: %v", err)
		return nil, errorx.Msg("获取工作空间排行数据失败")
	}

	// 按指定字段排序
	l.sortItems(items, sortBy, sortDirection)

	// 限制数量
	total := int64(len(items))
	if int64(len(items)) > topN {
		items = items[:topN]
	}

	// 构建响应
	resp := &pb.WorkspaceResourceRankingResp{
		Items: make([]*pb.WorkspaceRankingItem, 0, len(items)),
		Total: total,
	}

	for idx, item := range items {
		rankItem := l.buildRankingItem(int64(idx+1), item, sortBy, totalAllocated)
		resp.Items = append(resp.Items, rankItem)
	}

	return resp, nil
}

// 解析排序字段和方向
func (l *GetWorkspaceResourceRankingLogic) parseSortBy(sortByStr string) (string, string) {
	if sortByStr == "" {
		return "cpuAllocated", "desc"
	}

	parts := strings.Split(sortByStr, ":")
	sortBy := parts[0]
	sortDirection := "desc"
	if len(parts) > 1 && (parts[1] == "asc" || parts[1] == "desc") {
		sortDirection = parts[1]
	}

	// 验证排序字段是否有效
	validFields := map[string]bool{
		"projectName": true, "workspaceName": true,
		"cpuAllocated": true, "memAllocatedGib": true,
		"gpuAllocated": true, "storageAllocatedGib": true, "podsAllocated": true,
		// 兼容旧的简写
		"cpu": true, "mem": true, "gpu": true, "storage": true, "pod": true,
	}

	if !validFields[sortBy] {
		sortBy = "cpuAllocated"
	}

	return sortBy, sortDirection
}

// 聚合工作空间数据
func (l *GetWorkspaceResourceRankingLogic) aggregateWorkspaceData(clusterUuid string, projectId uint64, sortBy string) ([]*workspaceRankingItem, float64, error) {
	// 首先获取 ProjectCluster 的映射关系
	projectClusterMap := make(map[uint64]*projectClusterInfo)

	// 构建 ProjectCluster 查询条件
	pcQueryStr, pcArgs := l.buildProjectClusterQueryCondition(clusterUuid, projectId)

	projectClusters, err := l.svcCtx.OnecProjectClusterModel.SearchNoPage(l.ctx, "", false, pcQueryStr, pcArgs...)
	if err != nil && err.Error() != "record not found" {
		return nil, 0, fmt.Errorf("查询项目集群数据失败: %v", err)
	}

	// 收集 ProjectClusterId 和相关信息
	projectClusterIds := make([]uint64, 0, len(projectClusters))
	projectIds := make(map[uint64]bool)
	clusterUuids := make(map[string]bool)

	for _, pc := range projectClusters {
		// 过滤默认项目 ID = 3
		if pc.ProjectId == DefaultProjectId {
			continue
		}

		projectClusterMap[pc.Id] = &projectClusterInfo{
			ProjectClusterId: pc.Id,
			ProjectId:        pc.ProjectId,
			ClusterUuid:      pc.ClusterUuid,
		}
		projectClusterIds = append(projectClusterIds, pc.Id)
		projectIds[pc.ProjectId] = true
		clusterUuids[pc.ClusterUuid] = true
	}

	if len(projectClusterIds) == 0 {
		return []*workspaceRankingItem{}, 0, nil
	}

	// 查询工作空间数据
	wsQueryStr := l.buildWorkspaceQueryCondition(projectClusterIds)
	workspaces, err := l.svcCtx.OnecProjectWorkspaceModel.SearchNoPage(l.ctx, "", false, wsQueryStr)
	if err != nil && err.Error() != "record not found" {
		return nil, 0, fmt.Errorf("查询工作空间数据失败: %v", err)
	}

	// 批量查询项目和集群名称
	projectNames := l.batchGetProjectNames(l.mapKeysToUint64Slice(projectIds))
	clusterNames := l.batchGetClusterNames(l.mapKeysToStringSlice(clusterUuids))

	// 构建排行项
	items := make([]*workspaceRankingItem, 0, len(workspaces))
	var totalAllocated float64

	for _, ws := range workspaces {
		// 使用 utils 函数转换字符串为数值
		cpuAllocated, _ := utils.CPUToCores(ws.CpuAllocated)
		memAllocated, _ := utils.MemoryToGiB(ws.MemAllocated)
		gpuAllocated, _ := utils.GPUToCount(ws.GpuAllocated)
		storageAllocated, _ := utils.MemoryToGiB(ws.StorageAllocated)

		// 获取关联的 ProjectCluster 信息
		pcInfo, exists := projectClusterMap[ws.ProjectClusterId]
		if !exists {
			continue
		}

		item := &workspaceRankingItem{
			WorkspaceId:      ws.Id,
			WorkspaceName:    ws.Name,
			Namespace:        ws.Namespace,
			ProjectClusterId: ws.ProjectClusterId,
			ProjectId:        pcInfo.ProjectId,
			ClusterUuid:      ws.ClusterUuid,
			CpuAllocated:     cpuAllocated,
			MemAllocated:     memAllocated,
			GpuAllocated:     gpuAllocated,
			StorageAllocated: storageAllocated,
			PodsAllocated:    ws.PodsAllocated,
		}

		// 填充名称
		if name, ok := projectNames[pcInfo.ProjectId]; ok {
			item.ProjectName = name
		}
		if name, ok := clusterNames[ws.ClusterUuid]; ok {
			item.ClusterName = name
		}

		items = append(items, item)
		totalAllocated += l.getAllocatedBySortBy(sortBy, cpuAllocated, memAllocated, gpuAllocated, storageAllocated, float64(ws.PodsAllocated))
	}

	return items, totalAllocated, nil
}

type projectClusterInfo struct {
	ProjectClusterId uint64
	ProjectId        uint64
	ClusterUuid      string
}

func (l *GetWorkspaceResourceRankingLogic) buildProjectClusterQueryCondition(clusterUuid string, projectId uint64) (string, []any) {
	var queryStr string
	var args []any

	hasCluster := clusterUuid != "" && clusterUuid != "all"
	hasProject := projectId > 0

	if hasCluster && hasProject {
		queryStr = "cluster_uuid = ? AND project_id = ?"
		args = []any{clusterUuid, projectId}
	} else if hasCluster {
		queryStr = "cluster_uuid = ?"
		args = []any{clusterUuid}
	} else if hasProject {
		queryStr = "project_id = ?"
		args = []any{projectId}
	}

	return queryStr, args
}

func (l *GetWorkspaceResourceRankingLogic) buildWorkspaceQueryCondition(projectClusterIds []uint64) string {
	if len(projectClusterIds) == 0 {
		return ""
	}

	ids := make([]string, 0, len(projectClusterIds))
	for _, id := range projectClusterIds {
		ids = append(ids, fmt.Sprintf("%d", id))
	}
	return fmt.Sprintf("project_cluster_id IN (%s)", joinStrings(ids, ","))
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

func (l *GetWorkspaceResourceRankingLogic) batchGetProjectNames(ids []uint64) map[uint64]string {
	result := make(map[uint64]string)
	if len(ids) == 0 {
		return result
	}

	mr.MapReduceVoid(
		func(source chan<- uint64) {
			for _, id := range ids {
				source <- id
			}
		},
		func(id uint64, writer mr.Writer[*wsProjectNameResult], cancel func(error)) {
			project, err := l.svcCtx.OnecProjectModel.FindOne(l.ctx, id)
			if err == nil {
				writer.Write(&wsProjectNameResult{Id: id, Name: project.Name})
			}
		},
		func(pipe <-chan *wsProjectNameResult, cancel func(error)) {
			for item := range pipe {
				result[item.Id] = item.Name
			}
		},
	)

	return result
}

type wsProjectNameResult struct {
	Id   uint64
	Name string
}

func (l *GetWorkspaceResourceRankingLogic) batchGetClusterNames(uuids []string) map[string]string {
	result := make(map[string]string)
	if len(uuids) == 0 {
		return result
	}

	mr.MapReduceVoid(
		func(source chan<- string) {
			for _, uuid := range uuids {
				source <- uuid
			}
		},
		func(uuid string, writer mr.Writer[*wsClusterNameResult], cancel func(error)) {
			cluster, err := l.svcCtx.OnecClusterModel.FindOneByUuid(l.ctx, uuid)
			if err == nil {
				writer.Write(&wsClusterNameResult{Uuid: uuid, Name: cluster.Name})
			}
		},
		func(pipe <-chan *wsClusterNameResult, cancel func(error)) {
			for item := range pipe {
				result[item.Uuid] = item.Name
			}
		},
	)

	return result
}

type wsClusterNameResult struct {
	Uuid string
	Name string
}

func (l *GetWorkspaceResourceRankingLogic) mapKeysToUint64Slice(m map[uint64]bool) []uint64 {
	result := make([]uint64, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}

func (l *GetWorkspaceResourceRankingLogic) mapKeysToStringSlice(m map[string]bool) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}

// 根据排序字段获取已分配值
func (l *GetWorkspaceResourceRankingLogic) getAllocatedBySortBy(sortBy string, cpu, mem, gpu, storage, pod float64) float64 {
	switch sortBy {
	case "mem", "memAllocatedGib":
		return mem
	case "gpu", "gpuAllocated":
		return gpu
	case "storage", "storageAllocatedGib":
		return storage
	case "pod", "podsAllocated":
		return pod
	default: // cpu, cpuAllocated
		return cpu
	}
}

// 按指定字段排序 - 支持多字段和排序方向
func (l *GetWorkspaceResourceRankingLogic) sortItems(items []*workspaceRankingItem, sortBy string, sortDirection string) {
	isDesc := sortDirection == "desc"

	sort.Slice(items, func(i, j int) bool {
		switch sortBy {
		case "projectName":
			// 字符串排序
			if isDesc {
				return items[i].ProjectName > items[j].ProjectName
			}
			return items[i].ProjectName < items[j].ProjectName
		case "workspaceName":
			// 字符串排序
			if isDesc {
				return items[i].WorkspaceName > items[j].WorkspaceName
			}
			return items[i].WorkspaceName < items[j].WorkspaceName
		case "memAllocatedGib", "mem":
			if isDesc {
				return items[i].MemAllocated > items[j].MemAllocated
			}
			return items[i].MemAllocated < items[j].MemAllocated
		case "gpuAllocated", "gpu":
			if isDesc {
				return items[i].GpuAllocated > items[j].GpuAllocated
			}
			return items[i].GpuAllocated < items[j].GpuAllocated
		case "storageAllocatedGib", "storage":
			if isDesc {
				return items[i].StorageAllocated > items[j].StorageAllocated
			}
			return items[i].StorageAllocated < items[j].StorageAllocated
		case "podsAllocated", "pod":
			if isDesc {
				return items[i].PodsAllocated > items[j].PodsAllocated
			}
			return items[i].PodsAllocated < items[j].PodsAllocated
		default: // cpuAllocated, cpu
			if isDesc {
				return items[i].CpuAllocated > items[j].CpuAllocated
			}
			return items[i].CpuAllocated < items[j].CpuAllocated
		}
	})
}

// 构建排行项 - 工作空间没有 Limit/Capacity，UsageRate 无法计算
func (l *GetWorkspaceResourceRankingLogic) buildRankingItem(rank int64, item *workspaceRankingItem, sortBy string, totalAllocated float64) *pb.WorkspaceRankingItem {
	rankItem := &pb.WorkspaceRankingItem{
		Rank:          rank,
		WorkspaceId:   item.WorkspaceId,
		WorkspaceName: item.WorkspaceName,
		WorkspaceUuid: item.Namespace,
		Namespace:     item.Namespace,
		ProjectId:     item.ProjectId,
		ProjectName:   item.ProjectName,
		ClusterUuid:   item.ClusterUuid,
		ClusterName:   item.ClusterName,
		// 工作空间表没有 Limit 字段，设为 0
		CpuLimit:            0,
		CpuAllocated:        item.CpuAllocated,
		CpuUsageRate:        0,
		MemLimitGib:         0,
		MemAllocatedGib:     item.MemAllocated,
		MemUsageRate:        0,
		GpuLimit:            0,
		GpuAllocated:        item.GpuAllocated,
		GpuUsageRate:        0,
		StorageLimitGib:     0,
		StorageAllocatedGib: item.StorageAllocated,
		StorageUsageRate:    0,
		PodsLimit:           0,
		PodsAllocated:       item.PodsAllocated,
		PodsUsageRate:       0,
	}

	// 计算占比
	if totalAllocated > 0 {
		allocated := l.getAllocatedBySortBy(sortBy, item.CpuAllocated, item.MemAllocated, item.GpuAllocated, item.StorageAllocated, float64(item.PodsAllocated))
		rankItem.OverallUsageRate = (allocated / totalAllocated) * 100
	}

	return rankItem
}
