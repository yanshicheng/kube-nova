package managerservicelogic

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	"github.com/yanshicheng/kube-nova/common/utils"
	"github.com/zeromicro/go-zero/core/logx"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ProjectWorkspaceAddLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectWorkspaceAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectWorkspaceAddLogic {
	return &ProjectWorkspaceAddLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ProjectWorkspaceAdd 添加项目工作空间
func (l *ProjectWorkspaceAddLogic) ProjectWorkspaceAdd(in *pb.AddOnecProjectWorkspaceReq) (*pb.AddOnecProjectWorkspaceResp, error) {
	l.Infof("开始添加项目工作空间，名称: %s, 命名空间: %s", in.Name, in.Namespace)

	// 查询项目集群配额是否存在
	projectCluster, err := l.svcCtx.OnecProjectClusterModel.FindOne(l.ctx, in.ProjectClusterId)
	if err != nil {
		l.Errorf("查询项目集群配额失败，ID: %d, 错误: %v", in.ProjectClusterId, err)
		return nil, errorx.Msg("项目集群配额不存在")
	}
	// 查询项目是否存在
	project, err := l.svcCtx.OnecProjectModel.FindOne(l.ctx, projectCluster.ProjectId)
	if err != nil {
		l.Errorf("查询项目失败，ID: %d, 错误: %v", projectCluster.ProjectId, err)
		return nil, errorx.Msg("项目不存在")
	}
	l.Infof("项目信息: %s", project.Name)

	// 检查命名空间是否已存在
	existing, _ := l.svcCtx.OnecProjectWorkspaceModel.FindOneByProjectClusterIdNamespace(l.ctx, in.ProjectClusterId, in.Namespace)
	if existing != nil {
		l.Errorf("工作空间已存在，项目集群ID: %d, 命名空间: %s", in.ProjectClusterId, in.Namespace)
		return nil, errorx.Msg("该命名空间已存在")
	}

	// 检查资源是否充足（检查项目集群配额）
	if err := l.checkResourceAvailable(projectCluster, in); err != nil {
		l.Errorf("资源检查失败: %v", err)
		return nil, errorx.Msg(err.Error())
	}

	// 添加项目工作空间
	workspace := model.OnecProjectWorkspace{
		ProjectClusterId:          in.ProjectClusterId,
		ClusterUuid:               projectCluster.ClusterUuid,
		Name:                      in.Name,
		Namespace:                 in.Namespace,
		CpuAllocated:              in.CpuAllocated,
		MemAllocated:              in.MemAllocated,
		StorageAllocated:          in.StorageAllocated,
		GpuAllocated:              in.GpuAllocated,
		PodsAllocated:             in.PodsAllocated,
		ConfigmapAllocated:        in.ConfigmapAllocated,
		SecretAllocated:           in.SecretAllocated,
		PvcAllocated:              in.PvcAllocated,
		EphemeralStorageAllocated: in.EphemeralStorageAllocated,
		ServiceAllocated:          in.ServiceAllocated,
		LoadbalancersAllocated:    in.LoadbalancersAllocated,
		NodeportsAllocated:        in.NodeportsAllocated,
		DeploymentsAllocated:      in.DeploymentsAllocated,
		JobsAllocated:             in.JobsAllocated,
		CronjobsAllocated:         in.CronjobsAllocated,
		DaemonsetsAllocated:       in.DaemonsetsAllocated,
		StatefulsetsAllocated:     in.StatefulsetsAllocated,
		IngressesAllocated:        in.IngressesAllocated,

		PodMaxCpu:              in.PodMaxCpu,
		PodMaxMemory:           in.PodMaxMemory,
		PodMinCpu:              in.PodMinCpu,
		PodMinMemory:           in.PodMinMemory,
		PodMaxEphemeralStorage: in.PodMaxEphemeralStorage,
		PodMinEphemeralStorage: in.PodMinEphemeralStorage,

		ContainerMaxCpu:              in.ContainerMaxCpu,
		ContainerMaxMemory:           in.ContainerMaxMemory,
		ContainerMinCpu:              in.ContainerMinCpu,
		ContainerMinMemory:           in.ContainerMinMemory,
		ContainerMaxEphemeralStorage: in.ContainerMaxEphemeralStorage,
		ContainerMinEphemeralStorage: in.ContainerMinEphemeralStorage,

		ContainerDefaultCpu:                     in.ContainerDefaultCpu,
		ContainerDefaultMemory:                  in.ContainerDefaultMemory,
		ContainerDefaultEphemeralStorage:        in.ContainerDefaultEphemeralStorage,
		ContainerDefaultRequestCpu:              in.ContainerDefaultRequestCpu,
		ContainerDefaultRequestMemory:           in.ContainerDefaultRequestMemory,
		ContainerDefaultRequestEphemeralStorage: in.ContainerDefaultRequestEphemeralStorage,
		AppCreateTime:                           time.Now(),
		Status:                                  1,
	}
	works, err := l.svcCtx.OnecProjectWorkspaceModel.Insert(l.ctx, &workspace)
	if err != nil {
		l.Errorf("添加项目工作空间失败: %v", err)
		return nil, errorx.Msg("添加项目工作空间失败")
	}
	id, err := works.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("获取项目工作空间ID失败: %v", err)
	}
	// 同步项目资源 信息
	err = l.svcCtx.OnecProjectModel.SyncAllProjectClusters(l.ctx, projectCluster.Id)
	if err != nil {
		l.Errorf("同步项目资源信息失败: %v", err)
	}
	// 创建namespace
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, projectCluster.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群失败: %v", err)
		return nil, errorx.Msg("获取集群失败")
	}
	if err := isValidNamespaceName(in.Namespace); err != nil {
		l.Errorf("命名空间名称格式无效: %s", in.Namespace)
		return nil, fmt.Errorf("命名空间名称格式无效，必须符合DNS-1123标签规范：小写字母、数字、连字符，以字母或数字开头和结尾，最长63个字符")
	}
	// 构建 Namespace 对象
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   in.Namespace,
			Labels: map[string]string{},
		},
	}
	// 注入注解
	utils.AddAnnotations(&namespace.ObjectMeta, &utils.AnnotationsInfo{
		ProjectUuid:   project.Uuid,
		ProjectName:   project.Name,
		WorkspaceName: workspace.Name,
		ServiceName:   namespace.Name,
	})
	// 调用 Namespace 操作器创建命名空间
	createdNs, err := client.Namespaces().Create(namespace)
	if err != nil {
		l.Errorf("创建命名空间失败: namespace=%s, error=%v", in.Namespace, err)

		// 检查是否是因为命名空间已存在
		if existingNs, getErr := client.Namespaces().Get(in.Namespace); getErr == nil && existingNs != nil {
			l.Infof("命名空间已存在: %s", in.Namespace)
			return nil, fmt.Errorf("命名空间已存在: %s", in.Namespace)
		}

		return nil, fmt.Errorf("创建命名空间失败: %v", err)
	}

	l.Infof("成功创建命名空间: name=%s, uid=%s, creationTime=%s",
		createdNs.Name, createdNs.UID, createdNs.CreationTimestamp)
	// 创建 资源限制
	req := &types.ResourceQuotaRequest{
		Namespace:                 in.Namespace,
		Labels:                    make(map[string]string),
		Annotations:               createdNs.Annotations,
		Name:                      fmt.Sprintf("ikubeops-%s", in.Namespace),
		CPUAllocated:              in.CpuAllocated,
		MemoryAllocated:           in.MemAllocated,
		StorageAllocated:          in.StorageAllocated,
		GPUAllocated:              in.GpuAllocated,
		EphemeralStorageAllocated: in.EphemeralStorageAllocated,
		// 对象数量配额: in.
		PodsAllocated:          in.PodsAllocated,
		ConfigMapsAllocated:    in.ConfigmapAllocated,
		SecretsAllocated:       in.SecretAllocated,
		PVCsAllocated:          in.PvcAllocated,
		ServicesAllocated:      in.ServiceAllocated,
		LoadBalancersAllocated: in.LoadbalancersAllocated,
		NodePortsAllocated:     in.NodeportsAllocated,
		// 工作负载配额: in.
		DeploymentsAllocated:  in.DeploymentsAllocated,
		JobsAllocated:         in.JobsAllocated,
		CronJobsAllocated:     in.CronjobsAllocated,
		DaemonSetsAllocated:   in.DaemonsetsAllocated,
		StatefulSetsAllocated: in.StatefulsetsAllocated,
		IngressesAllocated:    in.IngressesAllocated,
	}
	// 记录详细的配额信息用于日志
	l.Debugf("ResourceQuota配置详情: CPU=%v, Memory=%vGiB, Storage=%vGiB, Pods=%v",
		req.CPUAllocated, req.MemoryAllocated, req.StorageAllocated, req.PodsAllocated)

	// 调用 Namespace 操作器创建或更新 ResourceQuota
	err = client.Namespaces().CreateOrUpdateResourceQuota(req)
	if err != nil {
		l.Errorf("创建或更新ResourceQuota失败: name=%s, namespace=%s, error=%v",
			in.Name, in.Namespace, err)
		return nil, fmt.Errorf("创建或更新ResourceQuota失败: %v", err)
	}
	// 创建 limitRange
	reqLimitRange := &types.LimitRangeRequest{
		Namespace:   in.Namespace,
		Labels:      make(map[string]string),
		Annotations: createdNs.Annotations,
		Name:        fmt.Sprintf("ikubeops-%s", in.Namespace),
		// Pod级别限制
		PodMaxCPU:              in.PodMaxCpu, // 核心数
		PodMaxMemory:           in.PodMaxMemory,
		PodMaxEphemeralStorage: in.PodMaxEphemeralStorage,
		PodMinCPU:              in.PodMinCpu,
		PodMinMemory:           in.PodMinMemory,
		PodMinEphemeralStorage: in.PodMinEphemeralStorage,
		// Container级别限制
		ContainerMaxCPU:              in.ContainerMaxCpu,
		ContainerMaxMemory:           in.ContainerMaxMemory,
		ContainerMaxEphemeralStorage: in.ContainerMaxEphemeralStorage,
		ContainerMinCPU:              in.ContainerMinCpu,
		ContainerMinMemory:           in.ContainerMinMemory,
		ContainerMinEphemeralStorage: in.ContainerMinEphemeralStorage,
		// Container默认限制（limits）
		ContainerDefaultCPU:              in.ContainerDefaultCpu,
		ContainerDefaultMemory:           in.ContainerDefaultMemory,
		ContainerDefaultEphemeralStorage: in.ContainerDefaultEphemeralStorage,
		// Container默认请求（requests）
		ContainerDefaultRequestCPU:              in.ContainerDefaultRequestCpu,
		ContainerDefaultRequestMemory:           in.ContainerDefaultRequestMemory,
		ContainerDefaultRequestEphemeralStorage: in.ContainerDefaultRequestEphemeralStorage,
	}
	// 调用 Namespace 操作器创建或更新 LimitRange
	err = client.Namespaces().CreateOrUpdateLimitRange(reqLimitRange)
	if err != nil {
		l.Errorf("创建或更新LimitRange失败: name=%s, namespace=%s, error=%v",
			in.Name, in.Namespace, err)
		return nil, fmt.Errorf("创建或更新LimitRange失败: %v", err)
	}
	// 调用 OnecProjectModel 的同步方法
	err = l.svcCtx.OnecProjectModel.SyncProjectClusterResourceAllocation(l.ctx, projectCluster.Id)
	if err != nil {
		l.Errorf("更新项目集群资源失败，项目集群ID: %d, 错误: %v", projectCluster.Id, err)
	} else {
		l.Infof("更新项目集群资源成功，项目集群ID: %d", projectCluster.Id)

		// 3. 清除项目集群缓存 - 需要项目集群ID
		if err := l.svcCtx.OnecProjectClusterModel.DeleteCache(l.ctx, projectCluster.Id); err != nil {
			l.Errorf("清除项目集群缓存失败，项目集群ID: %d, 错误: %v", projectCluster.Id, err)
		}
	}
	return &pb.AddOnecProjectWorkspaceResp{
		Id: uint64(id),
	}, nil
}

// checkResourceAvailable 检查资源是否充足（完整版本）
func (l *ProjectWorkspaceAddLogic) checkResourceAvailable(projectCluster *model.OnecProjectCluster, in *pb.AddOnecProjectWorkspaceReq) error {
	// CPU资源检查 - 使用 cpu_capacity（超分后容量）
	cpu, err := utils.CPUToCores(in.CpuAllocated)
	if err != nil {
		return fmt.Errorf("CPU转换错误: %v", err)
	}
	oldCpuAllocated, err := utils.CPUToCores(projectCluster.CpuAllocated)
	if err != nil {
		return fmt.Errorf("CPU转换错误: %v", err)
	}
	oldCpuCapacity, err := utils.CPUToCores(projectCluster.CpuCapacity)
	if err != nil {
		return fmt.Errorf("CPU转换错误: %v", err)
	}
	newCpuTotalAllocated := oldCpuAllocated + cpu
	if newCpuTotalAllocated > oldCpuCapacity {
		l.Errorf("CPU资源不足，容量: %v，申请后总分配: %.2f核", projectCluster.CpuCapacity, newCpuTotalAllocated)
		return fmt.Errorf("CPU资源不足，容量: %v，申请后总分配: %.2f核", projectCluster.CpuCapacity, newCpuTotalAllocated)
	}
	mem, err := utils.MemoryToGiB(in.MemAllocated)
	if err != nil {
		return fmt.Errorf("内存转换错误: %v", err)
	}
	// 内存资源检查 - 使用 mem_capacity（超分后容量）
	oldMemAllocated, err := utils.MemoryToGiB(projectCluster.MemAllocated)
	if err != nil {
		return fmt.Errorf("内存转换错误: %v", err)
	}
	oldMemCapacity, err := utils.MemoryToGiB(projectCluster.MemCapacity)
	if err != nil {
		return fmt.Errorf("内存转换错误: %v", err)
	}
	newMemTotalAllocated := oldMemAllocated + mem
	if newMemTotalAllocated > oldMemCapacity {
		l.Errorf("内存资源不足，容量: %v，申请后总分配: %.2f GiB", projectCluster.MemCapacity, newMemTotalAllocated)
		return fmt.Errorf("内存资源不足，容量: %v，申请后总分配: %.2f GiB", projectCluster.MemCapacity, newMemTotalAllocated)
	}

	storage, err := utils.MemoryToGiB(in.StorageAllocated)
	if err != nil {
		return fmt.Errorf("存储转换错误: %v", err)
	}
	// 存储资源检查 - 使用 storage_limit（不支持超分）
	oldStorageAllocated, err := utils.MemoryToGiB(projectCluster.StorageAllocated)
	if err != nil {
		return fmt.Errorf("存储转换错误: %v", err)
	}
	oldStorageLimit, err := utils.MemoryToGiB(projectCluster.StorageLimit)
	if err != nil {
		return fmt.Errorf("存储转换错误: %v", err)
	}
	newStorageTotalAllocated := oldStorageAllocated + storage
	if newStorageTotalAllocated > oldStorageLimit {
		l.Errorf("存储资源不足，限制: %v，申请后总分配: %d GiB", projectCluster.StorageLimit, newStorageTotalAllocated)
		return fmt.Errorf("存储资源不足，限制: %v，申请后总分配: %f GiB", projectCluster.StorageLimit, newStorageTotalAllocated)
	}

	// GPU资源检查 - 使用 gpu_capacity（超分后容量）
	gpu, err := utils.GPUToCount(in.GpuAllocated)
	if err != nil {
		return fmt.Errorf("GPU转换错误: %v", err)
	}
	oldGpuAllocated, err := utils.GPUToCount(projectCluster.GpuAllocated)
	if err != nil {
		return fmt.Errorf("GPU转换错误: %v", err)
	}
	oldGpuCapacity, err := utils.GPUToCount(projectCluster.GpuCapacity)
	if err != nil {
		return fmt.Errorf("GPU转换错误: %v", err)
	}
	newGpuTotalAllocated := oldGpuAllocated + gpu
	if newGpuTotalAllocated > oldGpuCapacity {
		l.Errorf("GPU资源不足，容量: %v个，申请后总分配: %.2f个", projectCluster.GpuCapacity, newGpuTotalAllocated)
		return fmt.Errorf("GPU资源不足，容量: %v个，申请后总分配: %.2f个", projectCluster.GpuCapacity, newGpuTotalAllocated)
	}

	// Pod资源检查 - 使用 pods_limit
	newPodsTotalAllocated := projectCluster.PodsAllocated + in.PodsAllocated
	if newPodsTotalAllocated > projectCluster.PodsLimit {
		l.Errorf("Pod配额不足，限制: %d个，申请后总分配: %d个", projectCluster.PodsLimit, newPodsTotalAllocated)
		return fmt.Errorf("pod配额不足，限制: %d个，申请后总分配: %d个", projectCluster.PodsLimit, newPodsTotalAllocated)
	}

	// ConfigMap检查 - 使用 configmap_limit
	newConfigmapTotalAllocated := projectCluster.ConfigmapAllocated + in.ConfigmapAllocated
	if newConfigmapTotalAllocated > projectCluster.ConfigmapLimit {
		l.Errorf("ConfigMap配额不足，限制: %d个，申请后总分配: %d个", projectCluster.ConfigmapLimit, newConfigmapTotalAllocated)
		return fmt.Errorf("ConfigMap配额不足，限制: %d个，申请后总分配: %d个", projectCluster.ConfigmapLimit, newConfigmapTotalAllocated)
	}

	// Secret检查 - 使用 secret_limit
	newSecretTotalAllocated := projectCluster.SecretAllocated + in.SecretAllocated
	if newSecretTotalAllocated > projectCluster.SecretLimit {
		l.Errorf("Secret配额不足，限制: %d个，申请后总分配: %d个", projectCluster.SecretLimit, newSecretTotalAllocated)
		return fmt.Errorf("secret配额不足，限制: %d个，申请后总分配: %d个", projectCluster.SecretLimit, newSecretTotalAllocated)
	}

	// PVC检查 - 使用 pvc_limit
	newPvcTotalAllocated := projectCluster.PvcAllocated + in.PvcAllocated
	if newPvcTotalAllocated > projectCluster.PvcLimit {
		l.Errorf("PVC配额不足，限制: %d个，申请后总分配: %d个", projectCluster.PvcLimit, newPvcTotalAllocated)
		return fmt.Errorf("PVC配额不足，限制: %d个，申请后总分配: %d个", projectCluster.PvcLimit, newPvcTotalAllocated)
	}

	// EphemeralStorage检查 - 使用 ephemeral_storage_limit
	ephStorage, err := utils.MemoryToGiB(in.EphemeralStorageAllocated)
	if err != nil {
		return fmt.Errorf("临时存储转换错误: %v", err)
	}
	oldEphStorageAllocated, err := utils.MemoryToGiB(projectCluster.EphemeralStorageAllocated)
	if err != nil {
		return fmt.Errorf("临时存储转换错误: %v", err)
	}
	oldEphStorageLimit, err := utils.MemoryToGiB(projectCluster.EphemeralStorageLimit)
	if err != nil {
		return fmt.Errorf("临时存储转换错误: %v", err)
	}
	newEphemeralStorageTotalAllocated := oldEphStorageAllocated + ephStorage
	if newEphemeralStorageTotalAllocated > oldEphStorageLimit {
		l.Errorf("临时存储配额不足，限制: %v，申请后总分配: %d GiB", projectCluster.EphemeralStorageLimit, newEphemeralStorageTotalAllocated)
		return fmt.Errorf("临时存储配额不足，限制: %v，申请后总分配: %d GiB", projectCluster.EphemeralStorageLimit, newEphemeralStorageTotalAllocated)
	}

	// Service检查 - 使用 service_limit
	newServiceTotalAllocated := projectCluster.ServiceAllocated + in.ServiceAllocated
	if newServiceTotalAllocated > projectCluster.ServiceLimit {
		l.Errorf("Service配额不足，限制: %d个，申请后总分配: %d个", projectCluster.ServiceLimit, newServiceTotalAllocated)
		return fmt.Errorf("service配额不足，限制: %d个，申请后总分配: %d个", projectCluster.ServiceLimit, newServiceTotalAllocated)
	}

	// LoadBalancer检查 - 使用 loadbalancers_limit
	newLoadbalancersTotalAllocated := projectCluster.LoadbalancersAllocated + in.LoadbalancersAllocated
	if newLoadbalancersTotalAllocated > projectCluster.LoadbalancersLimit {
		l.Errorf("LoadBalancer配额不足，限制: %d个，申请后总分配: %d个", projectCluster.LoadbalancersLimit, newLoadbalancersTotalAllocated)
		return fmt.Errorf("LoadBalancer配额不足，限制: %d个，申请后总分配: %d个", projectCluster.LoadbalancersLimit, newLoadbalancersTotalAllocated)
	}

	// NodePort检查 - 使用 nodeports_limit
	newNodeportsTotalAllocated := projectCluster.NodeportsAllocated + in.NodeportsAllocated
	if newNodeportsTotalAllocated > projectCluster.NodeportsLimit {
		l.Errorf("NodePort配额不足，限制: %d个，申请后总分配: %d个", projectCluster.NodeportsLimit, newNodeportsTotalAllocated)
		return fmt.Errorf("NodePort配额不足，限制: %d个，申请后总分配: %d个", projectCluster.NodeportsLimit, newNodeportsTotalAllocated)
	}

	// Deployment检查 - 使用 deployments_limit
	newDeploymentsTotalAllocated := projectCluster.DeploymentsAllocated + in.DeploymentsAllocated
	if newDeploymentsTotalAllocated > projectCluster.DeploymentsLimit {
		l.Errorf("Deployment配额不足，限制: %d个，申请后总分配: %d个", projectCluster.DeploymentsLimit, newDeploymentsTotalAllocated)
		return fmt.Errorf("deployment配额不足，限制: %d个，申请后总分配: %d个", projectCluster.DeploymentsLimit, newDeploymentsTotalAllocated)
	}

	// Jobs检查 - 使用 jobs_limit
	newJobsTotalAllocated := projectCluster.JobsAllocated + in.JobsAllocated
	if newJobsTotalAllocated > projectCluster.JobsLimit {
		l.Errorf("Job配额不足，限制: %d个，申请后总分配: %d个", projectCluster.JobsLimit, newJobsTotalAllocated)
		return fmt.Errorf("job配额不足，限制: %d个，申请后总分配: %d个", projectCluster.JobsLimit, newJobsTotalAllocated)
	}

	// CronJobs检查 - 使用 cronjobs_limit
	newCronjobsTotalAllocated := projectCluster.CronjobsAllocated + in.CronjobsAllocated
	if newCronjobsTotalAllocated > projectCluster.CronjobsLimit {
		l.Errorf("CronJob配额不足，限制: %d个，申请后总分配: %d个", projectCluster.CronjobsLimit, newCronjobsTotalAllocated)
		return fmt.Errorf("CronJob配额不足，限制: %d个，申请后总分配: %d个", projectCluster.CronjobsLimit, newCronjobsTotalAllocated)
	}

	// DaemonSets检查 - 使用 daemonsets_limit
	newDaemonsetsTotalAllocated := projectCluster.DaemonsetsAllocated + in.DaemonsetsAllocated
	if newDaemonsetsTotalAllocated > projectCluster.DaemonsetsLimit {
		l.Errorf("DaemonSet配额不足，限制: %d个，申请后总分配: %d个", projectCluster.DaemonsetsLimit, newDaemonsetsTotalAllocated)
		return fmt.Errorf("DaemonSet配额不足，限制: %d个，申请后总分配: %d个", projectCluster.DaemonsetsLimit, newDaemonsetsTotalAllocated)
	}

	// StatefulSets检查 - 使用 statefulsets_limit
	newStatefulsetsTotalAllocated := projectCluster.StatefulsetsAllocated + in.StatefulsetsAllocated
	if newStatefulsetsTotalAllocated > projectCluster.StatefulsetsLimit {
		l.Errorf("StatefulSet配额不足，限制: %d个，申请后总分配: %d个", projectCluster.StatefulsetsLimit, newStatefulsetsTotalAllocated)
		return fmt.Errorf("StatefulSet配额不足，限制: %d个，申请后总分配: %d个", projectCluster.StatefulsetsLimit, newStatefulsetsTotalAllocated)
	}

	// Ingresses检查 - 使用 ingresses_limit
	newIngressesTotalAllocated := projectCluster.IngressesAllocated + in.IngressesAllocated
	if newIngressesTotalAllocated > projectCluster.IngressesLimit {
		l.Errorf("Ingress配额不足，限制: %d个，申请后总分配: %d个", projectCluster.IngressesLimit, newIngressesTotalAllocated)
		return fmt.Errorf("ingress配额不足，限制: %d个，申请后总分配: %d个", projectCluster.IngressesLimit, newIngressesTotalAllocated)
	}

	return nil
}

// 创建 namespace
// isValidNamespaceName 验证命名空间名称是否符合 Kubernetes 的 DNS-1123 标签规范
func isValidNamespaceName(name string) error {
	if len(name) == 0 || len(name) > 63 {
		return fmt.Errorf("命名空间名称格式无效，必须符合DNS-1123标签规范：小写字母、数字、连字符，以字母或数字开头和结尾，最长63个字符")
	}

	// DNS-1123 标签规范：小写字母、数字、连字符
	// 必须以字母或数字开头和结尾
	// 不能有连续的连字符
	pattern := `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	matched, err := regexp.MatchString(pattern, name)
	if err != nil || !matched {
		return fmt.Errorf("命名空间名称格式无效，必须符合DNS-1123标签规范")
	}
	// 防止创建系统保留的命名空间
	protectedNamespaces := []string{
		"kube-system",
		"kube-public",
		"kube-node-lease",
		"default",
	}
	for _, protected := range protectedNamespaces {
		if name == protected {
			return fmt.Errorf("不能创建系统保留的命名空间: %s", name)
		}
	}
	return nil
}
