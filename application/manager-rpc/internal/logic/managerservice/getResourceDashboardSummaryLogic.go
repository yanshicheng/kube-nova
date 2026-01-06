package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/common/utils"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/mr"
)

type GetResourceDashboardSummaryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetResourceDashboardSummaryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetResourceDashboardSummaryLogic {
	return &GetResourceDashboardSummaryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 汇总数据结构
type summaryAggregateData struct {
	ClusterCount   int64
	ProjectCount   int64
	WorkspaceCount int64

	// Physical = 集群物理资源（只统计关联的集群）
	CpuPhysical     float64
	MemPhysical     float64
	GpuPhysical     float64
	StoragePhysical float64
	PodsPhysical    int64

	// Limit = 分配给项目的原始配额总和
	CpuLimit     float64
	MemLimit     float64
	GpuLimit     float64
	StorageLimit float64
	PodsLimit    int64

	// Capacity = 超分后容量
	CpuCapacity float64
	MemCapacity float64
	GpuCapacity float64

	// 关联的集群UUID列表（用于查询物理资源）
	RelatedClusterUuids map[string]bool

	// 名称映射
	ClusterInfoMap map[string]string
	ProjectInfoMap map[uint64]*summaryProjectInfo
}

type summaryProjectInfo struct {
	Name string
	Uuid string
}

// GetResourceDashboardSummary 获取资源仪表盘汇总数据
func (l *GetResourceDashboardSummaryLogic) GetResourceDashboardSummary(in *pb.ResourceDashboardSummaryReq) (*pb.ResourceDashboardSummaryResp, error) {
	// 聚合数据
	aggData, err := l.aggregateSummaryData(in.ClusterUuid, in.ProjectId)
	if err != nil {
		l.Errorf("聚合汇总数据失败: %v", err)
		return nil, errorx.Msg("获取仪表盘数据失败")
	}

	// 构建响应
	resp := &pb.ResourceDashboardSummaryResp{
		CurrentFilter:      l.buildFilterCondition(in.ClusterUuid, in.ProjectId, aggData),
		SummaryCards:       l.buildSummaryCards(aggData),
		AllocationOverview: l.buildAllocationOverview(aggData),
		OversellSavings:    l.buildOversellSavings(aggData),
	}

	return resp, nil
}

// 聚合汇总数据
func (l *GetResourceDashboardSummaryLogic) aggregateSummaryData(clusterUuid string, projectId uint64) (*summaryAggregateData, error) {
	aggData := &summaryAggregateData{
		RelatedClusterUuids: make(map[string]bool),
		ClusterInfoMap:      make(map[string]string),
		ProjectInfoMap:      make(map[uint64]*summaryProjectInfo),
	}

	// 构建查询条件
	queryStr, args := l.buildQueryCondition(clusterUuid, projectId)

	// 用于收集唯一的集群和项目
	projectIds := make(map[uint64]bool)

	hasCluster := clusterUuid != "" && clusterUuid != "all"
	hasProject := projectId > 0

	// 并发查询
	var projectClustersErr, workspacesErr error

	err := mr.Finish(
		// 查询项目集群数据并聚合（获取 Limit, Capacity，同时收集关联的集群UUID）
		func() error {
			projectClusters, err := l.svcCtx.OnecProjectClusterModel.SearchNoPage(l.ctx, "", false, queryStr, args...)
			if err != nil && err.Error() != "record not found" {
				projectClustersErr = err
				return nil
			}

			for _, pc := range projectClusters {
				// 收集关联的集群UUID（用于后续查询物理资源）
				aggData.RelatedClusterUuids[pc.ClusterUuid] = true
				projectIds[pc.ProjectId] = true

				// 转换资源值
				cpuLimit, _ := utils.CPUToCores(pc.CpuLimit)
				cpuCapacity, _ := utils.CPUToCores(pc.CpuCapacity)
				memLimit, _ := utils.MemoryToGiB(pc.MemLimit)
				memCapacity, _ := utils.MemoryToGiB(pc.MemCapacity)
				gpuLimit, _ := utils.GPUToCount(pc.GpuLimit)
				gpuCapacity, _ := utils.GPUToCount(pc.GpuCapacity)
				storageLimit, _ := utils.MemoryToGiB(pc.StorageLimit)

				// 累加 Limit（原始配额）
				aggData.CpuLimit += cpuLimit
				aggData.MemLimit += memLimit
				aggData.GpuLimit += gpuLimit
				aggData.StorageLimit += storageLimit
				aggData.PodsLimit += pc.PodsLimit

				// 累加 Capacity（超分后容量）
				aggData.CpuCapacity += cpuCapacity
				aggData.MemCapacity += memCapacity
				aggData.GpuCapacity += gpuCapacity
			}
			return nil
		},
		// 查询工作空间数量
		func() error {
			workspaces, err := l.svcCtx.OnecProjectWorkspaceModel.SearchNoPage(l.ctx, "", false, queryStr, args...)
			if err != nil && err.Error() != "record not found" {
				workspacesErr = err
				return nil
			}
			aggData.WorkspaceCount = int64(len(workspaces))
			return nil
		},
	)

	if err != nil {
		return nil, err
	}
	if projectClustersErr != nil {
		return nil, projectClustersErr
	}
	if workspacesErr != nil {
		l.Errorf("查询工作空间失败: %v", workspacesErr)
	}

	// 设置集群和项目数量
	aggData.ClusterCount = int64(len(aggData.RelatedClusterUuids))
	aggData.ProjectCount = int64(len(projectIds))

	// 查询物理资源（只统计关联的集群）
	if err := l.loadPhysicalResources(aggData, clusterUuid, hasCluster, hasProject); err != nil {
		l.Errorf("查询物理资源失败: %v", err)
	}

	// 如果选择了特定集群但没有项目绑定，集群数量应该是1
	if hasCluster && aggData.ClusterCount == 0 {
		aggData.ClusterCount = 1
	}

	// 查询筛选条件对应的名称
	if hasCluster {
		cluster, err := l.svcCtx.OnecClusterModel.FindOneByUuid(l.ctx, clusterUuid)
		if err == nil {
			aggData.ClusterInfoMap[clusterUuid] = cluster.Name
		}
	}

	if hasProject {
		project, err := l.svcCtx.OnecProjectModel.FindOne(l.ctx, projectId)
		if err == nil {
			aggData.ProjectInfoMap[projectId] = &summaryProjectInfo{Name: project.Name, Uuid: project.Uuid}
		}
	}

	return aggData, nil
}

// 加载物理资源（只统计关联的集群）
func (l *GetResourceDashboardSummaryLogic) loadPhysicalResources(aggData *summaryAggregateData, clusterUuid string, hasCluster, hasProject bool) error {
	if hasCluster {
		// 指定了集群，只查询该集群的物理资源
		clusterResource, err := l.svcCtx.OnecClusterResourceModel.FindOneByClusterUuid(l.ctx, clusterUuid)
		if err != nil && err.Error() != "record not found" {
			return err
		}
		if clusterResource != nil {
			aggData.CpuPhysical = clusterResource.CpuPhysicalCapacity
			aggData.MemPhysical = clusterResource.MemPhysicalCapacity
			aggData.GpuPhysical = clusterResource.GpuPhysicalCapacity
			aggData.StoragePhysical = clusterResource.StoragePhysicalCapacity
			aggData.PodsPhysical = clusterResource.PodsPhysicalCapacity
		}
	} else if hasProject && len(aggData.RelatedClusterUuids) > 0 {
		// 指定了项目但没指定集群，只统计该项目关联的集群的物理资源
		for uuid := range aggData.RelatedClusterUuids {
			clusterResource, err := l.svcCtx.OnecClusterResourceModel.FindOneByClusterUuid(l.ctx, uuid)
			if err != nil && err.Error() != "record not found" {
				continue
			}
			if clusterResource != nil {
				aggData.CpuPhysical += clusterResource.CpuPhysicalCapacity
				aggData.MemPhysical += clusterResource.MemPhysicalCapacity
				aggData.GpuPhysical += clusterResource.GpuPhysicalCapacity
				aggData.StoragePhysical += clusterResource.StoragePhysicalCapacity
				aggData.PodsPhysical += clusterResource.PodsPhysicalCapacity
			}
		}
	} else {
		// 全局查询，统计所有集群的物理资源
		clusterResources, err := l.svcCtx.OnecClusterResourceModel.SearchNoPage(l.ctx, "", false, "")
		if err != nil && err.Error() != "record not found" {
			return err
		}
		for _, cr := range clusterResources {
			aggData.CpuPhysical += cr.CpuPhysicalCapacity
			aggData.MemPhysical += cr.MemPhysicalCapacity
			aggData.GpuPhysical += cr.GpuPhysicalCapacity
			aggData.StoragePhysical += cr.StoragePhysicalCapacity
			aggData.PodsPhysical += cr.PodsPhysicalCapacity
		}
	}
	return nil
}

// 构建查询条件
func (l *GetResourceDashboardSummaryLogic) buildQueryCondition(clusterUuid string, projectId uint64) (string, []any) {
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

// 构建筛选条件回显
func (l *GetResourceDashboardSummaryLogic) buildFilterCondition(clusterUuid string, projectId uint64, aggData *summaryAggregateData) *pb.FilterCondition {
	filter := &pb.FilterCondition{
		ClusterUuid:            clusterUuid,
		ClusterName:            "全部集群",
		ProjectId:              projectId,
		ProjectName:            "全部项目",
		FilteredClusterCount:   aggData.ClusterCount,
		FilteredProjectCount:   aggData.ProjectCount,
		FilteredWorkspaceCount: aggData.WorkspaceCount,
	}

	if clusterUuid != "" && clusterUuid != "all" {
		if name, exists := aggData.ClusterInfoMap[clusterUuid]; exists {
			filter.ClusterName = name
		}
	}

	if projectId > 0 {
		if info, exists := aggData.ProjectInfoMap[projectId]; exists {
			filter.ProjectName = info.Name
		}
	}

	return filter
}

// 构建顶部统计卡片
func (l *GetResourceDashboardSummaryLogic) buildSummaryCards(aggData *summaryAggregateData) *pb.DashboardSummaryCards {
	cards := &pb.DashboardSummaryCards{
		ClusterTotalCount:   aggData.ClusterCount,
		ProjectTotalCount:   aggData.ProjectCount,
		WorkspaceTotalCount: aggData.WorkspaceCount,
		CpuPhysical:         aggData.CpuPhysical,
		CpuLimit:            aggData.CpuLimit,
		MemPhysical:         aggData.MemPhysical,
		MemLimit:            aggData.MemLimit,
		GpuPhysical:         aggData.GpuPhysical,
		GpuLimit:            aggData.GpuLimit,
		StoragePhysical:     aggData.StoragePhysical,
		StorageLimit:        aggData.StorageLimit,
		PodPhysical:         aggData.PodsPhysical,
		PodLimit:            aggData.PodsLimit,
	}

	// 计算物理资源分配率 = Limit / Physical * 100
	if aggData.CpuPhysical > 0 {
		cards.CpuPhysicalRate = (aggData.CpuLimit / aggData.CpuPhysical) * 100
	}
	if aggData.MemPhysical > 0 {
		cards.MemPhysicalRate = (aggData.MemLimit / aggData.MemPhysical) * 100
	}
	if aggData.GpuPhysical > 0 {
		cards.GpuPhysicalRate = (aggData.GpuLimit / aggData.GpuPhysical) * 100
	}
	if aggData.StoragePhysical > 0 {
		cards.StoragePhysicalRate = (aggData.StorageLimit / aggData.StoragePhysical) * 100
	}
	if aggData.PodsPhysical > 0 {
		cards.PodPhysicalRate = (float64(aggData.PodsLimit) / float64(aggData.PodsPhysical)) * 100
	}

	return cards
}

// 构建资源分配概览
func (l *GetResourceDashboardSummaryLogic) buildAllocationOverview(aggData *summaryAggregateData) *pb.ResourceAllocationOverview {
	return &pb.ResourceAllocationOverview{
		Cpu: &pb.ResourceOverviewItem{
			Physical: aggData.CpuPhysical,
			Limit:    aggData.CpuLimit,
			Capacity: aggData.CpuCapacity,
			Unit:     "核",
		},
		Mem: &pb.ResourceOverviewItem{
			Physical: aggData.MemPhysical,
			Limit:    aggData.MemLimit,
			Capacity: aggData.MemCapacity,
			Unit:     "GiB",
		},
		Gpu: &pb.ResourceOverviewItem{
			Physical: aggData.GpuPhysical,
			Limit:    aggData.GpuLimit,
			Capacity: aggData.GpuCapacity,
			Unit:     "卡",
		},
		Storage: &pb.ResourceOverviewItem{
			Physical: aggData.StoragePhysical,
			Limit:    aggData.StorageLimit,
			Capacity: aggData.StorageLimit,
			Unit:     "GiB",
		},
		Pod: &pb.ResourceOverviewItem{
			Physical: float64(aggData.PodsPhysical),
			Limit:    float64(aggData.PodsLimit),
			Capacity: float64(aggData.PodsLimit),
			Unit:     "个",
		},
	}
}

// 构建超分收益数据
func (l *GetResourceDashboardSummaryLogic) buildOversellSavings(aggData *summaryAggregateData) *pb.ResourceOversellSavings {
	savings := &pb.ResourceOversellSavings{
		Cpu: &pb.OversellSavingItem{
			LimitTotal:    aggData.CpuLimit,
			CapacityTotal: aggData.CpuCapacity,
			Unit:          "核",
		},
		Mem: &pb.OversellSavingItem{
			LimitTotal:    aggData.MemLimit,
			CapacityTotal: aggData.MemCapacity,
			Unit:          "GiB",
		},
		Gpu: &pb.OversellSavingItem{
			LimitTotal:    aggData.GpuLimit,
			CapacityTotal: aggData.GpuCapacity,
			Unit:          "卡",
		},
	}

	// 计算节约量（Capacity - Limit = 超分收益）
	if aggData.CpuLimit > 0 {
		savings.Cpu.SavingAmount = aggData.CpuCapacity - aggData.CpuLimit
		savings.Cpu.SavingRate = ((aggData.CpuCapacity - aggData.CpuLimit) / aggData.CpuLimit) * 100
	}
	if aggData.MemLimit > 0 {
		savings.Mem.SavingAmount = aggData.MemCapacity - aggData.MemLimit
		savings.Mem.SavingRate = ((aggData.MemCapacity - aggData.MemLimit) / aggData.MemLimit) * 100
	}
	if aggData.GpuLimit > 0 {
		savings.Gpu.SavingAmount = aggData.GpuCapacity - aggData.GpuLimit
		savings.Gpu.SavingRate = ((aggData.GpuCapacity - aggData.GpuLimit) / aggData.GpuLimit) * 100
	}

	return savings
}
