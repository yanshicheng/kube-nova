package managerservicelogic

import (
	"context"
	"errors"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/common/utils"
	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectClusterAddLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectClusterAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectClusterAddLogic {
	return &ProjectClusterAddLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ProjectClusterAdd 添加项目集群配额
func (l *ProjectClusterAddLogic) ProjectClusterAdd(in *pb.AddOnecProjectClusterReq) (*pb.AddOnecProjectClusterResp, error) {
	// 参数校验
	if in.ClusterUuid == "" {
		return nil, errorx.Msg("集群UUID不能为空")
	}
	if in.ProjectId == 0 {
		return nil, errorx.Msg("项目ID不能为空")
	}

	// 检查项目是否存在
	_, err := l.svcCtx.OnecProjectModel.FindOne(l.ctx, in.ProjectId)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, errorx.Msg("项目不存在")
		}
		l.Error("查询项目失败", err)
		return nil, errorx.Msg("查询项目失败")
	}

	// 检查集群是否存在
	_, err = l.svcCtx.OnecClusterModel.FindOneByUuid(l.ctx, in.ClusterUuid)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, errorx.Msg("集群不存在")
		}
		l.Error("查询集群失败", err)
		return nil, errorx.Msg("查询集群失败")
	}

	// 检查是否已存在相同的项目集群配额
	existing, err := l.svcCtx.OnecProjectClusterModel.FindOneByClusterUuidProjectId(l.ctx, in.ClusterUuid, in.ProjectId)
	if err == nil && existing != nil {
		return nil, errorx.Msg("该项目在此集群已存在配额配置")
	}

	// 获取集群资源信息
	clusterResource, err := l.svcCtx.OnecClusterResourceModel.FindOneByClusterUuid(l.ctx, in.ClusterUuid)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, errorx.Msg("集群资源信息不存在")
		}
		l.Error("查询集群资源信息失败", err)
		return nil, errorx.Msg("查询集群资源信息失败")
	}

	// 检查配额是否超出集群资源
	if err := l.checkQuotaOvercommit(clusterResource, in); err != nil {
		l.Errorf("配额检查失败: %v", err)
		return nil, errorx.Msg(err.Error())
	}

	// 创建项目集群配额
	projectCluster := model.OnecProjectCluster{
		ProjectId:                 in.ProjectId,
		ClusterUuid:               in.ClusterUuid,
		CpuAllocated:              "0",
		CpuLimit:                  in.CpuLimit,
		CpuOvercommitRatio:        in.CpuOvercommitRatio,
		CpuCapacity:               in.CpuCapacity,
		MemAllocated:              "0Gi",
		MemLimit:                  in.MemLimit,
		MemOvercommitRatio:        in.MemOvercommitRatio,
		MemCapacity:               in.MemCapacity,
		StorageLimit:              in.StorageLimit,
		StorageAllocated:          "0Gi",
		GpuCapacity:               in.GpuCapacity,
		GpuLimit:                  in.GpuLimit,
		GpuOvercommitRatio:        in.GpuOvercommitRatio,
		GpuAllocated:              "0",
		PodsAllocated:             0,
		PodsLimit:                 in.PodsLimit,
		IngressesLimit:            in.IngressesLimit,
		IngressesAllocated:        0,
		ServiceLimit:              in.ServiceLimit,
		ServiceAllocated:          0,
		NodeportsLimit:            in.NodeportsLimit,
		NodeportsAllocated:        0,
		PvcLimit:                  in.PvcLimit,
		PvcAllocated:              0,
		ConfigmapLimit:            in.ConfigmapLimit,
		ConfigmapAllocated:        0,
		SecretLimit:               in.SecretLimit,
		SecretAllocated:           0,
		LoadbalancersAllocated:    0,
		LoadbalancersLimit:        in.LoadbalancersLimit,
		DaemonsetsLimit:           in.DaemonsetsLimit,
		DaemonsetsAllocated:       0,
		DeploymentsLimit:          in.DeploymentsLimit,
		DeploymentsAllocated:      0,
		StatefulsetsLimit:         in.StatefulsetsLimit,
		StatefulsetsAllocated:     0,
		EphemeralStorageLimit:     in.EphemeralStorageLimit,
		EphemeralStorageAllocated: "0Gi",
		JobsLimit:                 in.JobsLimit,
		JobsAllocated:             0,
		CronjobsLimit:             in.CronjobsLimit,
		CronjobsAllocated:         0,
		CreatedBy:                 in.CreatedBy,
		UpdatedBy:                 in.UpdatedBy,
	}

	// 判断是否创建成功
	res, err := l.svcCtx.OnecProjectClusterModel.Insert(l.ctx, &projectCluster)
	if err != nil {
		l.Error("创建项目集群配额失败", err)
		return nil, errorx.Msg("创建项目集群配额失败")
	}
	_, err = res.LastInsertId()
	if err != nil {
		l.Error("创建项目集群配额失败", err)
		return nil, errorx.Msg("创建项目集群配额失败")
	}

	// 更新集群资源信息
	err = l.svcCtx.OnecClusterModel.SyncClusterResourceByResourceId(l.ctx, clusterResource.Id)
	if err != nil {
		l.Error("更新集群资源信息失败", err)
		return nil, errorx.Msg("更新集群资源信息失败")
	}

	return &pb.AddOnecProjectClusterResp{}, nil
}

// checkQuotaOvercommit 检查配额是否超出集群物理资源
// 校验规则：所有项目的 limit（实际分配）总和 不能超过 集群的 physical_capacity
func (l *ProjectClusterAddLogic) checkQuotaOvercommit(clusterResource *model.OnecClusterResource, in *pb.AddOnecProjectClusterReq) error {
	// CPU检查 - 检查 cpu_limit 是否超出物理容量
	cpuLimit, err := utils.CPUToCores(in.CpuLimit)
	if err != nil {
		l.Errorf("CPU转换失败: %v", err)
		return fmt.Errorf("CPU转换失败: %v", err)
	}
	newCpuAllocatedTotal := clusterResource.CpuAllocatedTotal + cpuLimit
	if newCpuAllocatedTotal > clusterResource.CpuPhysicalCapacity {
		l.Errorf("CPU配额超出物理容量，物理容量: %.2f核，分配后总量: %.2f核",
			clusterResource.CpuPhysicalCapacity, newCpuAllocatedTotal)
		return fmt.Errorf("CPU配额超出物理容量，物理容量: %.2f核，分配后总量: %.2f核",
			clusterResource.CpuPhysicalCapacity, newCpuAllocatedTotal)
	}

	// 内存检查 - 检查 mem_limit 是否超出物理容量
	memLimit, err := utils.MemoryToGiB(in.MemLimit)
	if err != nil {
		l.Errorf("内存转换失败: %v", err)
		return fmt.Errorf("内存转换失败: %v", err)
	}
	newMemAllocatedTotal := clusterResource.MemAllocatedTotal + memLimit
	if newMemAllocatedTotal > clusterResource.MemPhysicalCapacity {
		l.Errorf("内存配额超出物理容量，物理容量: %.2f GiB，分配后总量: %.2f GiB",
			clusterResource.MemPhysicalCapacity, newMemAllocatedTotal)
		return fmt.Errorf("内存配额超出物理容量，物理容量: %.2f GiB，分配后总量: %.2f GiB",
			clusterResource.MemPhysicalCapacity, newMemAllocatedTotal)
	}

	// 存储检查 - 检查 storage_limit 是否超出物理容量
	storageLimit, err := utils.MemoryToGiB(in.StorageLimit)
	if err != nil {
		l.Errorf("存储转换失败: %v", err)
		return fmt.Errorf("存储转换失败: %v", err)
	}
	newStorageAllocatedTotal := clusterResource.StorageAllocatedTotal + storageLimit
	if newStorageAllocatedTotal > clusterResource.StoragePhysicalCapacity {
		l.Errorf("存储配额超出物理容量，物理容量: %.2f GiB，分配后总量: %.2f GiB",
			clusterResource.StoragePhysicalCapacity, newStorageAllocatedTotal)
		return fmt.Errorf("存储配额超出物理容量，物理容量: %.2f GiB，分配后总量: %.2f GiB",
			clusterResource.StoragePhysicalCapacity, newStorageAllocatedTotal)
	}

	// GPU检查 - 检查 gpu_limit 是否超出物理容量
	gpuLimit, err := utils.GPUToCount(in.GpuLimit)
	if err != nil {
		l.Errorf("GPU转换失败: %v", err)
		return fmt.Errorf("GPU转换失败: %v", err)
	}
	newGpuAllocatedTotal := clusterResource.GpuAllocatedTotal + gpuLimit
	if newGpuAllocatedTotal > clusterResource.GpuPhysicalCapacity {
		l.Errorf("GPU配额超出物理容量，物理容量: %.2f个，分配后总量: %.2f个",
			clusterResource.GpuPhysicalCapacity, newGpuAllocatedTotal)
		return fmt.Errorf("GPU配额超出物理容量，物理容量: %.2f个，分配后总量: %.2f个",
			clusterResource.GpuPhysicalCapacity, newGpuAllocatedTotal)
	}

	// Pod检查 - 检查 pods_limit 是否超出物理容量
	newPodsAllocatedTotal := clusterResource.PodsAllocatedTotal + in.PodsLimit
	if newPodsAllocatedTotal > clusterResource.PodsPhysicalCapacity {
		l.Errorf("Pod配额超出物理容量，物理容量: %d个，分配后总量: %d个",
			clusterResource.PodsPhysicalCapacity, newPodsAllocatedTotal)
		return fmt.Errorf("Pod配额超出物理容量，物理容量: %d个，分配后总量: %d个",
			clusterResource.PodsPhysicalCapacity, newPodsAllocatedTotal)
	}

	return nil
}
