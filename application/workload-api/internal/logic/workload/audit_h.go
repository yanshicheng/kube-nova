package workload

import (
	"fmt"
	"sort"
	"strings"

	k8sTypes "github.com/yanshicheng/kube-nova/common/k8smanager/types"
)

// AuditRecordEnvValues 控制是否在审计日志中记录环境变量的值
// true: 记录完整的 key=value
// false: 只记录 key（用于保护敏感信息）
var AuditRecordEnvValues = true

// ============================================
// 环境变量变更对比
// ============================================

// CompareEnvVars 对比环境变量变更，生成审计描述
// oldEnvs: 原环境变量列表
// newEnvs: 新环境变量列表
// 返回格式: 环境变量变更: 新增 [KEY1=value1], 修改 [KEY2: old → new], 删除 [KEY3]
func CompareEnvVars(oldEnvs, newEnvs []k8sTypes.EnvVar) string {
	oldMap := make(map[string]k8sTypes.EnvVar)
	newMap := make(map[string]k8sTypes.EnvVar)

	for _, env := range oldEnvs {
		oldMap[env.Name] = env
	}
	for _, env := range newEnvs {
		newMap[env.Name] = env
	}

	var added, modified, deleted []string

	// 检查新增和修改
	for name, newEnv := range newMap {
		if oldEnv, exists := oldMap[name]; exists {
			// 存在，检查是否修改
			oldValue := getEnvVarValue(oldEnv)
			newValue := getEnvVarValue(newEnv)
			if oldValue != newValue {
				if AuditRecordEnvValues {
					modified = append(modified, fmt.Sprintf("%s: %s → %s", name, oldValue, newValue))
				} else {
					modified = append(modified, name)
				}
			}
		} else {
			// 新增
			if AuditRecordEnvValues {
				added = append(added, fmt.Sprintf("%s=%s", name, getEnvVarValue(newEnv)))
			} else {
				added = append(added, name)
			}
		}
	}

	// 检查删除
	for name := range oldMap {
		if _, exists := newMap[name]; !exists {
			deleted = append(deleted, name)
		}
	}

	// 排序保证输出稳定
	sort.Strings(added)
	sort.Strings(modified)
	sort.Strings(deleted)

	return buildChangeDescription("环境变量", added, modified, deleted)
}

// getEnvVarValue 获取环境变量的值描述
func getEnvVarValue(env k8sTypes.EnvVar) string {
	switch env.Source.Type {
	case "value", "":
		return env.Source.Value
	case "configMapKeyRef":
		if env.Source.ConfigMapKeyRef != nil {
			return fmt.Sprintf("ConfigMap(%s.%s)", env.Source.ConfigMapKeyRef.Name, env.Source.ConfigMapKeyRef.Key)
		}
	case "secretKeyRef":
		if env.Source.SecretKeyRef != nil {
			if AuditRecordEnvValues {
				return fmt.Sprintf("Secret(%s.%s)", env.Source.SecretKeyRef.Name, env.Source.SecretKeyRef.Key)
			}
			return "Secret(***)"
		}
	case "fieldRef":
		if env.Source.FieldRef != nil {
			return fmt.Sprintf("FieldRef(%s)", env.Source.FieldRef.FieldPath)
		}
	case "resourceFieldRef":
		if env.Source.ResourceFieldRef != nil {
			return fmt.Sprintf("ResourceFieldRef(%s)", env.Source.ResourceFieldRef.Resource)
		}
	}
	return ""
}

// ============================================
// 镜像变更对比
// ============================================

// CompareContainerImages 对比容器镜像变更
func CompareContainerImages(oldImages, newImages *k8sTypes.ContainerInfoList) string {
	var changes []string

	// 对比主容器
	changes = append(changes, compareContainerList("容器", oldImages.Containers, newImages.Containers)...)
	// 对比初始化容器
	changes = append(changes, compareContainerList("Init容器", oldImages.InitContainers, newImages.InitContainers)...)
	// 对比临时容器
	changes = append(changes, compareContainerList("临时容器", oldImages.EphemeralContainers, newImages.EphemeralContainers)...)

	if len(changes) == 0 {
		return "镜像无变更"
	}
	return "镜像变更: " + strings.Join(changes, "; ")
}

// compareContainerList 对比容器列表的镜像变更
func compareContainerList(containerType string, oldContainers, newContainers []k8sTypes.ContainerInfo) []string {
	var changes []string
	oldMap := make(map[string]string)
	newMap := make(map[string]string)

	for _, c := range oldContainers {
		oldMap[c.Name] = c.Image
	}
	for _, c := range newContainers {
		newMap[c.Name] = c.Image
	}

	for name, newImage := range newMap {
		if oldImage, exists := oldMap[name]; exists {
			if oldImage != newImage {
				changes = append(changes, fmt.Sprintf("%s %s: %s → %s", containerType, name, oldImage, newImage))
			}
		} else {
			changes = append(changes, fmt.Sprintf("%s %s: 新增镜像 %s", containerType, name, newImage))
		}
	}

	return changes
}

// CompareSingleImage 对比单个容器镜像变更
func CompareSingleImage(containerName, oldImage, newImage string) string {
	if oldImage == newImage {
		return fmt.Sprintf("容器 %s 镜像无变更", containerName)
	}
	if containerName == "" {
		containerName = "默认容器"
	}
	return fmt.Sprintf("容器 %s 镜像: %s → %s", containerName, oldImage, newImage)
}

// ============================================
// 更新策略变更对比
// ============================================

// CompareUpdateStrategy 对比更新策略变更
func CompareUpdateStrategy(oldStrategy *k8sTypes.UpdateStrategyResponse, newType string, newRolling *k8sTypes.RollingUpdateConfig) string {
	var changes []string

	// 策略类型变更
	if oldStrategy.Type != newType {
		changes = append(changes, fmt.Sprintf("策略类型: %s → %s", oldStrategy.Type, newType))
	}

	// 滚动更新参数变更
	if newRolling != nil {
		if oldStrategy.RollingUpdate != nil {
			if oldStrategy.RollingUpdate.MaxUnavailable != newRolling.MaxUnavailable {
				changes = append(changes, fmt.Sprintf("MaxUnavailable: %s → %s", oldStrategy.RollingUpdate.MaxUnavailable, newRolling.MaxUnavailable))
			}
			if oldStrategy.RollingUpdate.MaxSurge != newRolling.MaxSurge {
				changes = append(changes, fmt.Sprintf("MaxSurge: %s → %s", oldStrategy.RollingUpdate.MaxSurge, newRolling.MaxSurge))
			}
			if oldStrategy.RollingUpdate.Partition != newRolling.Partition {
				changes = append(changes, fmt.Sprintf("Partition: %d → %d", oldStrategy.RollingUpdate.Partition, newRolling.Partition))
			}
		} else {
			changes = append(changes, fmt.Sprintf("新增滚动更新配置: MaxUnavailable=%s, MaxSurge=%s", newRolling.MaxUnavailable, newRolling.MaxSurge))
		}
	}

	if len(changes) == 0 {
		return "更新策略无变更"
	}
	return "更新策略变更: " + strings.Join(changes, ", ")
}

// ============================================
// 资源配额变更对比
// ============================================

// CompareResources 对比资源配额变更
func CompareResources(containerName string, oldRes, newRes *k8sTypes.ResourcesResponse, newReq k8sTypes.ResourceRequirements) string {
	var changes []string

	// 查找对应容器的旧资源配置
	var oldContainerRes *k8sTypes.ContainerResources
	for _, cr := range oldRes.Containers {
		if cr.ContainerName == containerName {
			oldContainerRes = &cr
			break
		}
	}

	if oldContainerRes == nil {
		return fmt.Sprintf("容器 %s 资源配额: 新增配置 (Limits: CPU=%s, Memory=%s; Requests: CPU=%s, Memory=%s)",
			containerName, newReq.Limits.Cpu, newReq.Limits.Memory, newReq.Requests.Cpu, newReq.Requests.Memory)
	}

	// 对比 Limits
	if oldContainerRes.Resources.Limits.Cpu != newReq.Limits.Cpu {
		changes = append(changes, fmt.Sprintf("CPU Limits: %s → %s", oldContainerRes.Resources.Limits.Cpu, newReq.Limits.Cpu))
	}
	if oldContainerRes.Resources.Limits.Memory != newReq.Limits.Memory {
		changes = append(changes, fmt.Sprintf("Memory Limits: %s → %s", oldContainerRes.Resources.Limits.Memory, newReq.Limits.Memory))
	}

	// 对比 Requests
	if oldContainerRes.Resources.Requests.Cpu != newReq.Requests.Cpu {
		changes = append(changes, fmt.Sprintf("CPU Requests: %s → %s", oldContainerRes.Resources.Requests.Cpu, newReq.Requests.Cpu))
	}
	if oldContainerRes.Resources.Requests.Memory != newReq.Requests.Memory {
		changes = append(changes, fmt.Sprintf("Memory Requests: %s → %s", oldContainerRes.Resources.Requests.Memory, newReq.Requests.Memory))
	}

	if len(changes) == 0 {
		return fmt.Sprintf("容器 %s 资源配额无变更", containerName)
	}
	return fmt.Sprintf("容器 %s 资源配额变更: %s", containerName, strings.Join(changes, ", "))
}

// ============================================
// 健康检查变更对比
// ============================================

// CompareProbes 对比健康检查变更
func CompareProbes(containerName string, oldProbes *k8sTypes.ProbesResponse, newLiveness, newReadiness, newStartup *k8sTypes.Probe) string {
	var changes []string

	// 查找对应容器的旧探针配置
	var oldContainerProbes *k8sTypes.ContainerProbes
	for _, cp := range oldProbes.Containers {
		if cp.ContainerName == containerName {
			oldContainerProbes = &cp
			break
		}
	}

	// 对比存活探针
	if newLiveness != nil {
		if oldContainerProbes == nil || oldContainerProbes.LivenessProbe == nil {
			changes = append(changes, fmt.Sprintf("存活探针: 新增 %s 探针", newLiveness.Type))
		} else {
			probeChanges := compareProbeConfig("存活探针", oldContainerProbes.LivenessProbe, newLiveness)
			if probeChanges != "" {
				changes = append(changes, probeChanges)
			}
		}
	}

	// 对比就绪探针
	if newReadiness != nil {
		if oldContainerProbes == nil || oldContainerProbes.ReadinessProbe == nil {
			changes = append(changes, fmt.Sprintf("就绪探针: 新增 %s 探针", newReadiness.Type))
		} else {
			probeChanges := compareProbeConfig("就绪探针", oldContainerProbes.ReadinessProbe, newReadiness)
			if probeChanges != "" {
				changes = append(changes, probeChanges)
			}
		}
	}

	// 对比启动探针
	if newStartup != nil {
		if oldContainerProbes == nil || oldContainerProbes.StartupProbe == nil {
			changes = append(changes, fmt.Sprintf("启动探针: 新增 %s 探针", newStartup.Type))
		} else {
			probeChanges := compareProbeConfig("启动探针", oldContainerProbes.StartupProbe, newStartup)
			if probeChanges != "" {
				changes = append(changes, probeChanges)
			}
		}
	}

	if len(changes) == 0 {
		return fmt.Sprintf("容器 %s 健康检查无变更", containerName)
	}
	return fmt.Sprintf("容器 %s 健康检查变更: %s", containerName, strings.Join(changes, "; "))
}

// compareProbeConfig 对比单个探针配置
func compareProbeConfig(probeName string, oldProbe, newProbe *k8sTypes.Probe) string {
	var changes []string

	if oldProbe.Type != newProbe.Type {
		changes = append(changes, fmt.Sprintf("类型: %s → %s", oldProbe.Type, newProbe.Type))
	}
	if oldProbe.InitialDelaySeconds != newProbe.InitialDelaySeconds {
		changes = append(changes, fmt.Sprintf("初始延迟: %ds → %ds", oldProbe.InitialDelaySeconds, newProbe.InitialDelaySeconds))
	}
	if oldProbe.TimeoutSeconds != newProbe.TimeoutSeconds {
		changes = append(changes, fmt.Sprintf("超时时间: %ds → %ds", oldProbe.TimeoutSeconds, newProbe.TimeoutSeconds))
	}
	if oldProbe.PeriodSeconds != newProbe.PeriodSeconds {
		changes = append(changes, fmt.Sprintf("检查间隔: %ds → %ds", oldProbe.PeriodSeconds, newProbe.PeriodSeconds))
	}
	if oldProbe.SuccessThreshold != newProbe.SuccessThreshold {
		changes = append(changes, fmt.Sprintf("成功阈值: %d → %d", oldProbe.SuccessThreshold, newProbe.SuccessThreshold))
	}
	if oldProbe.FailureThreshold != newProbe.FailureThreshold {
		changes = append(changes, fmt.Sprintf("失败阈值: %d → %d", oldProbe.FailureThreshold, newProbe.FailureThreshold))
	}

	if len(changes) == 0 {
		return ""
	}
	return fmt.Sprintf("%s: %s", probeName, strings.Join(changes, ", "))
}

// ============================================
// 调度配置变更对比
// ============================================

// CompareSchedulingConfig 对比调度配置变更
// CompareSchedulingConfig 对比调度配置变更
// 注意：移除了无用的第二个参数 scher
func CompareSchedulingConfig(oldConfig *k8sTypes.SchedulingConfig, newReq *k8sTypes.UpdateSchedulingConfigRequest) string {
	// 空值检查
	if newReq == nil {
		return "调度配置无变更"
	}

	// 如果 oldConfig 为 nil，说明无法获取原配置，只能描述新配置
	if oldConfig == nil {
		return buildNewConfigDescription(newReq)
	}

	var changes []string

	// 对比 NodeSelector
	nodeSelChanges := compareMapChanges("NodeSelector", oldConfig.NodeSelector, newReq.NodeSelector)
	if nodeSelChanges != "" {
		changes = append(changes, nodeSelChanges)
	}

	// 对比 NodeName
	if newReq.NodeName != nil && *newReq.NodeName != "" {
		if oldConfig.NodeName != *newReq.NodeName {
			changes = append(changes, fmt.Sprintf("节点名称: %s → %s", oldConfig.NodeName, *newReq.NodeName))
		}
	}

	// 对比 SchedulerName（修复：正确比较字符串值，而不是地址）
	if newReq.SchedulerName != nil && *newReq.SchedulerName != "" {
		if oldConfig.SchedulerName != *newReq.SchedulerName {
			changes = append(changes, fmt.Sprintf("调度器: %s → %s", oldConfig.SchedulerName, *newReq.SchedulerName))
		}
	}

	// 对比 PriorityClassName（修复：正确比较字符串值）
	if newReq.PriorityClassName != nil && *newReq.PriorityClassName != "" {
		if oldConfig.PriorityClassName != *newReq.PriorityClassName {
			changes = append(changes, fmt.Sprintf("优先级类: %s → %s", oldConfig.PriorityClassName, *newReq.PriorityClassName))
		}
	}

	// 对比 Priority
	if newReq.Priority != nil {
		oldPriority := int32(0)
		if oldConfig.Priority != nil {
			oldPriority = *oldConfig.Priority
		}
		if oldPriority != *newReq.Priority {
			changes = append(changes, fmt.Sprintf("优先级: %d → %d", oldPriority, *newReq.Priority))
		}
	}

	// 对比 RuntimeClassName
	if newReq.RuntimeClassName != nil && *newReq.RuntimeClassName != "" {
		oldRuntimeClass := ""
		if oldConfig.RuntimeClassName != nil {
			oldRuntimeClass = *oldConfig.RuntimeClassName
		}
		if oldRuntimeClass != *newReq.RuntimeClassName {
			changes = append(changes, fmt.Sprintf("运行时类: %s → %s", oldRuntimeClass, *newReq.RuntimeClassName))
		}
	}

	// 对比 Tolerations 数量变化
	oldTolCount := len(oldConfig.Tolerations)
	newTolCount := len(newReq.Tolerations)
	if newReq.Tolerations != nil && oldTolCount != newTolCount {
		changes = append(changes, fmt.Sprintf("容忍度数量: %d → %d", oldTolCount, newTolCount))
	}

	// 对比 Affinity
	if newReq.Affinity != nil {
		if oldConfig.Affinity == nil {
			changes = append(changes, "亲和性: 新增配置")
		} else {
			changes = append(changes, "亲和性: 配置已更新")
		}
	}

	// 对比 TopologySpreadConstraints
	if newReq.TopologySpreadConstraints != nil {
		oldTscCount := len(oldConfig.TopologySpreadConstraints)
		newTscCount := len(newReq.TopologySpreadConstraints)
		if oldTscCount != newTscCount {
			changes = append(changes, fmt.Sprintf("拓扑分布约束数量: %d → %d", oldTscCount, newTscCount))
		}
	}

	if len(changes) == 0 {
		return "调度配置无变更"
	}
	return "调度配置变更: " + strings.Join(changes, ", ")
}

// buildNewConfigDescription 当无法获取原配置时，描述新配置内容
func buildNewConfigDescription(newReq *k8sTypes.UpdateSchedulingConfigRequest) string {
	var parts []string

	if len(newReq.NodeSelector) > 0 {
		parts = append(parts, fmt.Sprintf("NodeSelector: %v", newReq.NodeSelector))
	}
	if newReq.NodeName != nil && *newReq.NodeName != "" {
		parts = append(parts, fmt.Sprintf("NodeName: %s", *newReq.NodeName))
	}
	if newReq.SchedulerName != nil && *newReq.SchedulerName != "" {
		parts = append(parts, fmt.Sprintf("调度器: %s", *newReq.SchedulerName))
	}
	if newReq.PriorityClassName != nil && *newReq.PriorityClassName != "" {
		parts = append(parts, fmt.Sprintf("优先级类: %s", *newReq.PriorityClassName))
	}
	if len(newReq.Tolerations) > 0 {
		parts = append(parts, fmt.Sprintf("容忍度数量: %d", len(newReq.Tolerations)))
	}
	if newReq.Affinity != nil {
		parts = append(parts, "配置了亲和性")
	}
	if len(newReq.TopologySpreadConstraints) > 0 {
		parts = append(parts, fmt.Sprintf("拓扑分布约束数量: %d", len(newReq.TopologySpreadConstraints)))
	}

	if len(parts) == 0 {
		return "调度配置变更 (无详细信息)"
	}
	return "调度配置变更 (无法获取原配置): " + strings.Join(parts, ", ")
}

// ============================================
// 存储配置变更对比
// ============================================

// CompareStorageConfig 对比存储配置变更
func CompareStorageConfig(oldConfig, newConfig *k8sTypes.StorageConfig, newReq *k8sTypes.UpdateStorageConfigRequest) string {
	var changes []string

	// 对比 Volumes
	oldVolNames := make(map[string]bool)
	newVolNames := make(map[string]bool)
	for _, v := range oldConfig.Volumes {
		oldVolNames[v.Name] = true
	}
	for _, v := range newReq.Volumes {
		newVolNames[v.Name] = true
	}

	var addedVols, deletedVols []string
	for name := range newVolNames {
		if !oldVolNames[name] {
			addedVols = append(addedVols, name)
		}
	}
	for name := range oldVolNames {
		if !newVolNames[name] {
			deletedVols = append(deletedVols, name)
		}
	}

	if len(addedVols) > 0 {
		sort.Strings(addedVols)
		changes = append(changes, fmt.Sprintf("Volumes 新增: [%s]", strings.Join(addedVols, ", ")))
	}
	if len(deletedVols) > 0 {
		sort.Strings(deletedVols)
		changes = append(changes, fmt.Sprintf("Volumes 删除: [%s]", strings.Join(deletedVols, ", ")))
	}

	// 对比 VolumeMounts 数量
	oldMountCount := 0
	newMountCount := 0
	for _, vm := range oldConfig.VolumeMounts {
		oldMountCount += len(vm.Mounts)
	}
	for _, vm := range newReq.VolumeMounts {
		newMountCount += len(vm.Mounts)
	}
	if oldMountCount != newMountCount {
		changes = append(changes, fmt.Sprintf("VolumeMounts 数量: %d → %d", oldMountCount, newMountCount))
	}

	// 对比 VolumeClaimTemplates
	oldPvcCount := len(oldConfig.VolumeClaimTemplates)
	newPvcCount := len(newReq.VolumeClaimTemplates)
	if oldPvcCount != newPvcCount {
		changes = append(changes, fmt.Sprintf("PVC模板数量: %d → %d", oldPvcCount, newPvcCount))
	}

	if len(changes) == 0 {
		return "存储配置无变更"
	}
	return "存储配置变更: " + strings.Join(changes, ", ")
}

// ============================================
// CronJob 调度配置变更对比
// ============================================

// CompareCronJobSchedule 对比 CronJob 调度配置变更
func CompareCronJobSchedule(oldConfig *k8sTypes.CronJobScheduleConfig, newSchedule, newTimezone, newConcurrencyPolicy string, newStartingDeadline *int64, newSuccessLimit, newFailedLimit *int32) string {
	var changes []string

	if oldConfig.Schedule != newSchedule && newSchedule != "" {
		changes = append(changes, fmt.Sprintf("调度表达式: %s → %s", oldConfig.Schedule, newSchedule))
	}
	if oldConfig.Timezone != newTimezone && newTimezone != "" {
		changes = append(changes, fmt.Sprintf("时区: %s → %s", oldConfig.Timezone, newTimezone))
	}
	if oldConfig.ConcurrencyPolicy != newConcurrencyPolicy && newConcurrencyPolicy != "" {
		changes = append(changes, fmt.Sprintf("并发策略: %s → %s", oldConfig.ConcurrencyPolicy, newConcurrencyPolicy))
	}
	if newStartingDeadline != nil && oldConfig.StartingDeadlineSeconds != *newStartingDeadline {
		changes = append(changes, fmt.Sprintf("启动截止时间: %ds → %ds", oldConfig.StartingDeadlineSeconds, *newStartingDeadline))
	}
	if newSuccessLimit != nil && oldConfig.SuccessfulJobsHistoryLimit != *newSuccessLimit {
		changes = append(changes, fmt.Sprintf("成功历史保留数: %d → %d", oldConfig.SuccessfulJobsHistoryLimit, *newSuccessLimit))
	}
	if newFailedLimit != nil && oldConfig.FailedJobsHistoryLimit != *newFailedLimit {
		changes = append(changes, fmt.Sprintf("失败历史保留数: %d → %d", oldConfig.FailedJobsHistoryLimit, *newFailedLimit))
	}

	if len(changes) == 0 {
		return "CronJob 调度配置无变更"
	}
	return "CronJob 调度配置变更: " + strings.Join(changes, ", ")
}

// ============================================
// Job 并行度配置变更对比
// ============================================

// CompareJobParallelism 对比 Job 并行度配置变更
func CompareJobParallelism(oldConfig *k8sTypes.JobParallelismConfig, newParallelism, newCompletions, newBackoffLimit *int32, newActiveDeadline *int64) string {
	var changes []string

	if newParallelism != nil && oldConfig.Parallelism != *newParallelism {
		changes = append(changes, fmt.Sprintf("并行度: %d → %d", oldConfig.Parallelism, *newParallelism))
	}
	if newCompletions != nil && oldConfig.Completions != *newCompletions {
		changes = append(changes, fmt.Sprintf("完成数: %d → %d", oldConfig.Completions, *newCompletions))
	}
	if newBackoffLimit != nil && oldConfig.BackoffLimit != *newBackoffLimit {
		changes = append(changes, fmt.Sprintf("重试次数: %d → %d", oldConfig.BackoffLimit, *newBackoffLimit))
	}
	if newActiveDeadline != nil && oldConfig.ActiveDeadlineSeconds != *newActiveDeadline {
		changes = append(changes, fmt.Sprintf("超时时间: %ds → %ds", oldConfig.ActiveDeadlineSeconds, *newActiveDeadline))
	}

	if len(changes) == 0 {
		return "Job 并行度配置无变更"
	}
	return "Job 并行度配置变更: " + strings.Join(changes, ", ")
}

// ============================================
// Job Spec 配置变更对比
// ============================================

// CompareJobSpec 对比 Job Spec 配置变更
func CompareJobSpec(oldSpec *k8sTypes.JobSpecConfig, newReq *k8sTypes.UpdateJobSpecRequest) string {
	var changes []string

	if newReq.Parallelism != nil && (oldSpec.Parallelism == nil || *oldSpec.Parallelism != *newReq.Parallelism) {
		oldVal := int32(0)
		if oldSpec.Parallelism != nil {
			oldVal = *oldSpec.Parallelism
		}
		changes = append(changes, fmt.Sprintf("并行度: %d → %d", oldVal, *newReq.Parallelism))
	}
	if newReq.Completions != nil && (oldSpec.Completions == nil || *oldSpec.Completions != *newReq.Completions) {
		oldVal := int32(0)
		if oldSpec.Completions != nil {
			oldVal = *oldSpec.Completions
		}
		changes = append(changes, fmt.Sprintf("完成数: %d → %d", oldVal, *newReq.Completions))
	}
	if newReq.BackoffLimit != nil && (oldSpec.BackoffLimit == nil || *oldSpec.BackoffLimit != *newReq.BackoffLimit) {
		oldVal := int32(0)
		if oldSpec.BackoffLimit != nil {
			oldVal = *oldSpec.BackoffLimit
		}
		changes = append(changes, fmt.Sprintf("重试次数: %d → %d", oldVal, *newReq.BackoffLimit))
	}
	if newReq.ActiveDeadlineSeconds != nil && (oldSpec.ActiveDeadlineSeconds == nil || *oldSpec.ActiveDeadlineSeconds != *newReq.ActiveDeadlineSeconds) {
		oldVal := int64(0)
		if oldSpec.ActiveDeadlineSeconds != nil {
			oldVal = *oldSpec.ActiveDeadlineSeconds
		}
		changes = append(changes, fmt.Sprintf("超时时间: %ds → %ds", oldVal, *newReq.ActiveDeadlineSeconds))
	}
	if newReq.TTLSecondsAfterFinished != nil && (oldSpec.TTLSecondsAfterFinished == nil || *oldSpec.TTLSecondsAfterFinished != *newReq.TTLSecondsAfterFinished) {
		oldVal := int32(0)
		if oldSpec.TTLSecondsAfterFinished != nil {
			oldVal = *oldSpec.TTLSecondsAfterFinished
		}
		changes = append(changes, fmt.Sprintf("完成后保留时间: %ds → %ds", oldVal, *newReq.TTLSecondsAfterFinished))
	}
	if newReq.CompletionMode != nil && oldSpec.CompletionMode != *newReq.CompletionMode {
		changes = append(changes, fmt.Sprintf("完成模式: %s → %s", oldSpec.CompletionMode, *newReq.CompletionMode))
	}
	if newReq.Suspend != nil && oldSpec.Suspend != *newReq.Suspend {
		changes = append(changes, fmt.Sprintf("暂停状态: %v → %v", oldSpec.Suspend, *newReq.Suspend))
	}
	if newReq.PodReplacementPolicy != nil && oldSpec.PodReplacementPolicy != *newReq.PodReplacementPolicy {
		changes = append(changes, fmt.Sprintf("Pod替换策略: %s → %s", oldSpec.PodReplacementPolicy, *newReq.PodReplacementPolicy))
	}

	if len(changes) == 0 {
		return "Job Spec 配置无变更"
	}
	return "Job Spec 配置变更: " + strings.Join(changes, ", ")
}

// ============================================
// 版本回滚描述
// ============================================

// BuildRevisionRollbackDescription 构建版本回滚描述
func BuildRevisionRollbackDescription(resourceType, namespace, name string, currentRevision, targetRevision int64) string {
	return fmt.Sprintf("%s %s/%s 从版本 %d 回滚到版本 %d", resourceType, namespace, name, currentRevision, targetRevision)
}

// BuildConfigRollbackDescription 构建配置回滚描述
func BuildConfigRollbackDescription(resourceType, namespace, name string, configHistoryId int64) string {
	return fmt.Sprintf("%s %s/%s 回滚到配置历史 ID: %d", resourceType, namespace, name, configHistoryId)
}

// ============================================
// 通用辅助函数
// ============================================

// buildChangeDescription 构建变更描述
func buildChangeDescription(itemType string, added, modified, deleted []string) string {
	var parts []string

	if len(added) > 0 {
		parts = append(parts, fmt.Sprintf("新增 [%s]", strings.Join(added, ", ")))
	}
	if len(modified) > 0 {
		parts = append(parts, fmt.Sprintf("修改 [%s]", strings.Join(modified, ", ")))
	}
	if len(deleted) > 0 {
		parts = append(parts, fmt.Sprintf("删除 [%s]", strings.Join(deleted, ", ")))
	}

	if len(parts) == 0 {
		return fmt.Sprintf("%s无变更", itemType)
	}
	return fmt.Sprintf("%s变更: %s", itemType, strings.Join(parts, ", "))
}

// compareMapChanges 对比 map 变更
func compareMapChanges(itemType string, oldMap, newMap map[string]string) string {
	if oldMap == nil {
		oldMap = make(map[string]string)
	}
	if newMap == nil {
		newMap = make(map[string]string)
	}

	var added, modified, deleted []string

	for k, newV := range newMap {
		if oldV, exists := oldMap[k]; exists {
			if oldV != newV {
				modified = append(modified, fmt.Sprintf("%s: %s → %s", k, oldV, newV))
			}
		} else {
			added = append(added, fmt.Sprintf("%s=%s", k, newV))
		}
	}

	for k := range oldMap {
		if _, exists := newMap[k]; !exists {
			deleted = append(deleted, k)
		}
	}

	sort.Strings(added)
	sort.Strings(modified)
	sort.Strings(deleted)

	return buildChangeDescription(itemType, added, modified, deleted)
}

// GetCurrentRevision 获取当前版本号（从版本历史中获取最新的）
func GetCurrentRevision(revisions []k8sTypes.RevisionInfo) int64 {
	if len(revisions) == 0 {
		return 0
	}
	// 假设版本历史按 Revision 排序，取最大的
	var maxRevision int64 = 0
	for _, rev := range revisions {
		if rev.Revision > maxRevision {
			maxRevision = rev.Revision
		}
	}
	return maxRevision
}
