package workload

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	k8sTypes "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateAdvancedConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateAdvancedConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateAdvancedConfigLogic {
	return &UpdateAdvancedConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateAdvancedConfigLogic) UpdateAdvancedConfig(req *types.CommUpdateAdvancedConfigRequest) (resp string, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	resourceType := strings.ToUpper(versionDetail.ResourceType)

	// 获取原高级配置
	var oldConfig *k8sTypes.AdvancedConfigResponse
	switch resourceType {
	case "DEPLOYMENT":
		oldConfig, err = client.Deployment().GetAdvancedConfig(versionDetail.Namespace, versionDetail.ResourceName)
	case "STATEFULSET":
		oldConfig, err = client.StatefulSet().GetAdvancedConfig(versionDetail.Namespace, versionDetail.ResourceName)
	case "DAEMONSET":
		oldConfig, err = client.DaemonSet().GetAdvancedConfig(versionDetail.Namespace, versionDetail.ResourceName)
	case "CRONJOB":
		oldConfig, err = client.CronJob().GetAdvancedConfig(versionDetail.Namespace, versionDetail.ResourceName)
	default:
		return "", fmt.Errorf("资源类型 %s 不支持修改高级配置", resourceType)
	}

	if err != nil {
		l.Errorf("获取原高级配置失败: %v", err)
		// 继续执行，oldConfig 可能为 nil
	}

	// 转换请求
	updateReq := convertApiAdvancedConfigRequestToK8s(versionDetail.ResourceName, versionDetail.Namespace, req)

	// 执行更新
	switch resourceType {
	case "DEPLOYMENT":
		err = client.Deployment().UpdateAdvancedConfig(updateReq)
	case "STATEFULSET":
		err = client.StatefulSet().UpdateAdvancedConfig(updateReq)
	case "DAEMONSET":
		err = client.DaemonSet().UpdateAdvancedConfig(updateReq)
	case "CRONJOB":
		err = client.CronJob().UpdateAdvancedConfig(updateReq)
	}

	// 比较并生成变更详情
	changeDetail := compareAdvancedConfigChanges(oldConfig, req)

	// 如果没有变更，不记录日志
	if changeDetail == "" {
		if err != nil {
			l.Errorf("修改高级配置失败: %v", err)
			return "", fmt.Errorf("修改高级配置失败")
		}
		return "修改高级配置成功(无变更)", nil
	}

	if err != nil {
		l.Errorf("修改高级配置失败: %v", err)
		recordAuditLog(l.ctx, l.svcCtx, versionDetail, "修改高级配置",
			fmt.Sprintf("%s %s/%s 修改高级配置失败, %s, 错误: %v", resourceType, versionDetail.Namespace, versionDetail.ResourceName, changeDetail, err), 2)
		return "", fmt.Errorf("修改高级配置失败")
	}

	recordAuditLog(l.ctx, l.svcCtx, versionDetail, "修改高级配置",
		fmt.Sprintf("%s %s/%s 修改高级配置成功, %s", resourceType, versionDetail.Namespace, versionDetail.ResourceName, changeDetail), 1)
	return "修改高级配置成功", nil
}

// convertApiAdvancedConfigRequestToK8s 将 API 请求转换为 k8s 请求
func convertApiAdvancedConfigRequestToK8s(name, namespace string, req *types.CommUpdateAdvancedConfigRequest) *k8sTypes.UpdateAdvancedConfigRequest {
	if req == nil {
		return nil
	}

	result := &k8sTypes.UpdateAdvancedConfigRequest{
		Name:       name,
		Namespace:  namespace,
		Containers: make([]k8sTypes.ContainerAdvancedConfig, 0, len(req.Containers)),
	}

	// 转换 PodConfig
	if req.PodConfig != nil {
		result.PodConfig = convertApiPodAdvancedConfigToK8s(req.PodConfig)
	}

	// 转换 Containers
	for _, container := range req.Containers {
		result.Containers = append(result.Containers, convertApiContainerAdvancedConfigToK8s(&container))
	}

	return result
}

// convertApiPodAdvancedConfigToK8s 转换 Pod 级别高级配置
func convertApiPodAdvancedConfigToK8s(config *types.CommPodAdvancedConfig) *k8sTypes.PodAdvancedConfig {
	if config == nil {
		return nil
	}

	result := &k8sTypes.PodAdvancedConfig{
		RestartPolicy:      config.RestartPolicy,
		DNSPolicy:          config.DNSPolicy,
		Hostname:           config.Hostname,
		Subdomain:          config.Subdomain,
		ServiceAccountName: config.ServiceAccountName,
		PriorityClassName:  config.PriorityClassName,
		PreemptionPolicy:   config.PreemptionPolicy,
		SchedulerName:      config.SchedulerName,
		ImagePullSecrets:   config.ImagePullSecrets,
	}

	// 处理非零值字段转换为指针
	if config.TerminationGracePeriodSeconds != 0 {
		result.TerminationGracePeriodSeconds = &config.TerminationGracePeriodSeconds
	}
	if config.HostNetwork {
		result.HostNetwork = &config.HostNetwork
	}
	if config.HostPID {
		result.HostPID = &config.HostPID
	}
	if config.HostIPC {
		result.HostIPC = &config.HostIPC
	}
	if config.HostUsers {
		result.HostUsers = &config.HostUsers
	}
	if config.AutomountServiceAccountToken {
		result.AutomountServiceAccountToken = &config.AutomountServiceAccountToken
	}
	if config.Priority != 0 {
		result.Priority = &config.Priority
	}
	if config.ShareProcessNamespace {
		result.ShareProcessNamespace = &config.ShareProcessNamespace
	}
	if config.ActiveDeadlineSeconds != 0 {
		result.ActiveDeadlineSeconds = &config.ActiveDeadlineSeconds
	}
	if config.RuntimeClassName != "" {
		result.RuntimeClassName = &config.RuntimeClassName
	}

	// 转换 DNSConfig
	if config.DNSConfig != nil {
		result.DNSConfig = convertApiDNSConfigToK8s(config.DNSConfig)
	}

	// 转换 HostAliases
	if len(config.HostAliases) > 0 {
		result.HostAliases = make([]k8sTypes.HostAlias, 0, len(config.HostAliases))
		for _, ha := range config.HostAliases {
			result.HostAliases = append(result.HostAliases, k8sTypes.HostAlias{
				IP:        ha.IP,
				Hostnames: ha.Hostnames,
			})
		}
	}

	// 转换 SecurityContext
	if config.SecurityContext != nil {
		result.SecurityContext = convertApiPodSecurityContextToK8s(config.SecurityContext)
	}

	// 转换 OS
	if config.OS != nil {
		result.OS = &k8sTypes.PodOS{
			Name: config.OS.Name,
		}
	}

	return result
}

// convertApiDNSConfigToK8s 转换 DNS 配置
func convertApiDNSConfigToK8s(config *types.CommDNSConfig) *k8sTypes.DNSConfig {
	if config == nil {
		return nil
	}

	result := &k8sTypes.DNSConfig{
		Nameservers: config.Nameservers,
		Searches:    config.Searches,
	}

	if len(config.Options) > 0 {
		result.Options = make([]k8sTypes.DNSConfigOption, 0, len(config.Options))
		for _, opt := range config.Options {
			option := k8sTypes.DNSConfigOption{
				Name: opt.Name,
			}
			if opt.Value != "" {
				option.Value = &opt.Value
			}
			result.Options = append(result.Options, option)
		}
	}

	return result
}

// convertApiPodSecurityContextToK8s 转换 Pod 安全上下文
func convertApiPodSecurityContextToK8s(ctx *types.CommPodSecurityContext) *k8sTypes.PodSecurityContext {
	if ctx == nil {
		return nil
	}

	result := &k8sTypes.PodSecurityContext{
		FSGroupChangePolicy: ctx.FSGroupChangePolicy,
		SupplementalGroups:  ctx.SupplementalGroups,
	}

	if ctx.RunAsUser != 0 {
		result.RunAsUser = &ctx.RunAsUser
	}
	if ctx.RunAsGroup != 0 {
		result.RunAsGroup = &ctx.RunAsGroup
	}
	if ctx.RunAsNonRoot {
		result.RunAsNonRoot = &ctx.RunAsNonRoot
	}
	if ctx.FSGroup != 0 {
		result.FSGroup = &ctx.FSGroup
	}

	// 转换 SeccompProfile
	if ctx.SeccompProfile != nil {
		result.SeccompProfile = &k8sTypes.SeccompProfile{
			Type:             ctx.SeccompProfile.Type,
			LocalhostProfile: ctx.SeccompProfile.LocalhostProfile,
		}
	}

	// 转换 SELinuxOptions
	if ctx.SELinuxOptions != nil {
		result.SELinuxOptions = &k8sTypes.SELinuxOptions{
			User:  ctx.SELinuxOptions.User,
			Role:  ctx.SELinuxOptions.Role,
			Type:  ctx.SELinuxOptions.Type,
			Level: ctx.SELinuxOptions.Level,
		}
	}

	// 转换 AppArmorProfile
	if ctx.AppArmorProfile != nil {
		result.AppArmorProfile = &k8sTypes.AppArmorProfile{
			Type:             ctx.AppArmorProfile.Type,
			LocalhostProfile: ctx.AppArmorProfile.LocalhostProfile,
		}
	}

	// 转换 Sysctls
	if len(ctx.Sysctls) > 0 {
		result.Sysctls = make([]k8sTypes.Sysctl, 0, len(ctx.Sysctls))
		for _, s := range ctx.Sysctls {
			result.Sysctls = append(result.Sysctls, k8sTypes.Sysctl{
				Name:  s.Name,
				Value: s.Value,
			})
		}
	}

	// 转换 WindowsOptions
	if ctx.WindowsOptions != nil {
		result.WindowsOptions = &k8sTypes.WindowsSecurityContextOptions{
			GMSACredentialSpecName: ctx.WindowsOptions.GMSACredentialSpecName,
			GMSACredentialSpec:     ctx.WindowsOptions.GMSACredentialSpec,
			RunAsUserName:          ctx.WindowsOptions.RunAsUserName,
		}
		if ctx.WindowsOptions.HostProcess {
			result.WindowsOptions.HostProcess = &ctx.WindowsOptions.HostProcess
		}
	}

	return result
}

// convertApiContainerAdvancedConfigToK8s 转换容器级别高级配置
func convertApiContainerAdvancedConfigToK8s(config *types.CommContainerAdvancedConfig) k8sTypes.ContainerAdvancedConfig {
	if config == nil {
		return k8sTypes.ContainerAdvancedConfig{}
	}

	result := k8sTypes.ContainerAdvancedConfig{
		ContainerName:   config.ContainerName,
		ContainerType:   k8sTypes.ContainerType(config.ContainerType),
		ImagePullPolicy: config.ImagePullPolicy,
		Command:         config.Command,
		Args:            config.Args,
		WorkingDir:      config.WorkingDir,
	}

	// 处理布尔字段转换为指针
	if config.Stdin {
		result.Stdin = &config.Stdin
	}
	if config.StdinOnce {
		result.StdinOnce = &config.StdinOnce
	}
	if config.TTY {
		result.TTY = &config.TTY
	}

	// 转换 Lifecycle
	if config.Lifecycle != nil {
		result.Lifecycle = convertApiLifecycleToK8s(config.Lifecycle)
	}

	// 转换 SecurityContext
	if config.SecurityContext != nil {
		result.SecurityContext = convertApiContainerSecurityContextToK8s(config.SecurityContext)
	}

	return result
}

// convertApiLifecycleToK8s 转换生命周期钩子
func convertApiLifecycleToK8s(lifecycle *types.CommLifecycle) *k8sTypes.Lifecycle {
	if lifecycle == nil {
		return nil
	}

	result := &k8sTypes.Lifecycle{}

	if lifecycle.PostStart != nil {
		result.PostStart = convertApiLifecycleHandlerToK8s(lifecycle.PostStart)
	}
	if lifecycle.PreStop != nil {
		result.PreStop = convertApiLifecycleHandlerToK8s(lifecycle.PreStop)
	}

	return result
}

// convertApiLifecycleHandlerToK8s 转换生命周期处理器
func convertApiLifecycleHandlerToK8s(handler *types.CommLifecycleHandler) *k8sTypes.LifecycleHandler {
	if handler == nil {
		return nil
	}

	result := &k8sTypes.LifecycleHandler{
		Type: handler.Type,
	}

	if handler.Exec != nil {
		result.Exec = &k8sTypes.ExecAction{
			Command: handler.Exec.Command,
		}
	}

	if handler.HTTPGet != nil {
		result.HTTPGet = &k8sTypes.HTTPGetAction{
			Path:   handler.HTTPGet.Path,
			Port:   handler.HTTPGet.Port,
			Host:   handler.HTTPGet.Host,
			Scheme: handler.HTTPGet.Scheme,
		}
		if len(handler.HTTPGet.HTTPHeaders) > 0 {
			result.HTTPGet.HTTPHeaders = make([]k8sTypes.HTTPHeader, 0, len(handler.HTTPGet.HTTPHeaders))
			for _, h := range handler.HTTPGet.HTTPHeaders {
				result.HTTPGet.HTTPHeaders = append(result.HTTPGet.HTTPHeaders, k8sTypes.HTTPHeader{
					Name:  h.Name,
					Value: h.Value,
				})
			}
		}
	}

	if handler.TCPSocket != nil {
		result.TCPSocket = &k8sTypes.TCPSocketAction{
			Port: handler.TCPSocket.Port,
			Host: handler.TCPSocket.Host,
		}
	}

	if handler.Sleep != nil {
		result.Sleep = &k8sTypes.SleepAction{
			Seconds: handler.Sleep.Seconds,
		}
	}

	return result
}

// convertApiContainerSecurityContextToK8s 转换容器安全上下文
func convertApiContainerSecurityContextToK8s(ctx *types.CommContainerSecurityContext) *k8sTypes.ContainerSecurityContext {
	if ctx == nil {
		return nil
	}

	result := &k8sTypes.ContainerSecurityContext{
		ProcMount: ctx.ProcMount,
	}

	if ctx.RunAsUser != 0 {
		result.RunAsUser = &ctx.RunAsUser
	}
	if ctx.RunAsGroup != 0 {
		result.RunAsGroup = &ctx.RunAsGroup
	}
	if ctx.RunAsNonRoot {
		result.RunAsNonRoot = &ctx.RunAsNonRoot
	}
	if ctx.ReadOnlyRootFilesystem {
		result.ReadOnlyRootFilesystem = &ctx.ReadOnlyRootFilesystem
	}
	if ctx.Privileged {
		result.Privileged = &ctx.Privileged
	}
	if ctx.AllowPrivilegeEscalation {
		result.AllowPrivilegeEscalation = &ctx.AllowPrivilegeEscalation
	}

	// 转换 Capabilities
	if ctx.Capabilities != nil {
		result.Capabilities = &k8sTypes.Capabilities{
			Add:  ctx.Capabilities.Add,
			Drop: ctx.Capabilities.Drop,
		}
	}

	// 转换 SeccompProfile
	if ctx.SeccompProfile != nil {
		result.SeccompProfile = &k8sTypes.SeccompProfile{
			Type:             ctx.SeccompProfile.Type,
			LocalhostProfile: ctx.SeccompProfile.LocalhostProfile,
		}
	}

	// 转换 SELinuxOptions
	if ctx.SELinuxOptions != nil {
		result.SELinuxOptions = &k8sTypes.SELinuxOptions{
			User:  ctx.SELinuxOptions.User,
			Role:  ctx.SELinuxOptions.Role,
			Type:  ctx.SELinuxOptions.Type,
			Level: ctx.SELinuxOptions.Level,
		}
	}

	// 转换 AppArmorProfile
	if ctx.AppArmorProfile != nil {
		result.AppArmorProfile = &k8sTypes.AppArmorProfile{
			Type:             ctx.AppArmorProfile.Type,
			LocalhostProfile: ctx.AppArmorProfile.LocalhostProfile,
		}
	}

	return result
}

// ==================== 变更比较函数 ====================

// compareAdvancedConfigChanges 比较高级配置变更，返回变更详情
func compareAdvancedConfigChanges(oldConfig *k8sTypes.AdvancedConfigResponse, newReq *types.CommUpdateAdvancedConfigRequest) string {
	if newReq == nil {
		return ""
	}

	var changes []string

	// 比较 PodConfig
	if newReq.PodConfig != nil {
		podChanges := comparePodAdvancedConfig(oldConfig, newReq.PodConfig)
		if podChanges != "" {
			changes = append(changes, podChanges)
		}
	}

	// 比较 Containers
	if len(newReq.Containers) > 0 {
		containerChanges := compareContainerAdvancedConfigs(oldConfig, newReq.Containers)
		if containerChanges != "" {
			changes = append(changes, containerChanges)
		}
	}

	if len(changes) == 0 {
		return ""
	}

	return "高级配置变更: " + strings.Join(changes, "; ")
}

// comparePodAdvancedConfig 比较 Pod 级别高级配置
func comparePodAdvancedConfig(oldConfig *k8sTypes.AdvancedConfigResponse, newConfig *types.CommPodAdvancedConfig) string {
	if newConfig == nil {
		return ""
	}

	var changes []string

	var oldPodConfig k8sTypes.PodAdvancedConfig
	if oldConfig != nil {
		oldPodConfig = oldConfig.PodConfig
	}

	// 比较关键字段
	if oldPodConfig.RestartPolicy != newConfig.RestartPolicy && newConfig.RestartPolicy != "" {
		changes = append(changes, fmt.Sprintf("RestartPolicy: %s -> %s",
			defaultStrVal(oldPodConfig.RestartPolicy, "未设置"), newConfig.RestartPolicy))
	}

	if oldPodConfig.DNSPolicy != newConfig.DNSPolicy && newConfig.DNSPolicy != "" {
		changes = append(changes, fmt.Sprintf("DNSPolicy: %s -> %s",
			defaultStrVal(oldPodConfig.DNSPolicy, "未设置"), newConfig.DNSPolicy))
	}

	if oldPodConfig.ServiceAccountName != newConfig.ServiceAccountName && newConfig.ServiceAccountName != "" {
		changes = append(changes, fmt.Sprintf("ServiceAccountName: %s -> %s",
			defaultStrVal(oldPodConfig.ServiceAccountName, "未设置"), newConfig.ServiceAccountName))
	}

	oldTermination := int64(0)
	if oldPodConfig.TerminationGracePeriodSeconds != nil {
		oldTermination = *oldPodConfig.TerminationGracePeriodSeconds
	}
	if oldTermination != newConfig.TerminationGracePeriodSeconds && newConfig.TerminationGracePeriodSeconds != 0 {
		changes = append(changes, fmt.Sprintf("TerminationGracePeriodSeconds: %d -> %d", oldTermination, newConfig.TerminationGracePeriodSeconds))
	}

	// 比较 HostNetwork
	oldHostNetwork := false
	if oldPodConfig.HostNetwork != nil {
		oldHostNetwork = *oldPodConfig.HostNetwork
	}
	if oldHostNetwork != newConfig.HostNetwork {
		changes = append(changes, fmt.Sprintf("HostNetwork: %v -> %v", oldHostNetwork, newConfig.HostNetwork))
	}

	// 比较 SecurityContext
	oldHasSecCtx := oldPodConfig.SecurityContext != nil
	newHasSecCtx := newConfig.SecurityContext != nil
	if oldHasSecCtx != newHasSecCtx {
		if newHasSecCtx {
			changes = append(changes, "新增Pod安全上下文")
		} else {
			changes = append(changes, "移除Pod安全上下文")
		}
	}

	if len(changes) == 0 {
		return ""
	}

	return "Pod配置: " + strings.Join(changes, ", ")
}

// compareContainerAdvancedConfigs 比较容器级别高级配置
func compareContainerAdvancedConfigs(oldConfig *k8sTypes.AdvancedConfigResponse, newContainers []types.CommContainerAdvancedConfig) string {
	if len(newContainers) == 0 {
		return ""
	}

	// 构建旧配置 map
	oldContainerMap := make(map[string]k8sTypes.ContainerAdvancedConfig)
	if oldConfig != nil {
		for _, c := range oldConfig.Containers {
			oldContainerMap[c.ContainerName] = c
		}
	}

	var containerChanges []string

	for _, newC := range newContainers {
		oldC, exists := oldContainerMap[newC.ContainerName]
		var changes []string

		if !exists {
			changes = append(changes, "新增配置")
		} else {
			// 比较 ImagePullPolicy
			if oldC.ImagePullPolicy != newC.ImagePullPolicy && newC.ImagePullPolicy != "" {
				changes = append(changes, fmt.Sprintf("ImagePullPolicy: %s -> %s",
					defaultStrVal(oldC.ImagePullPolicy, "未设置"), newC.ImagePullPolicy))
			}

			// 比较 Command
			if !stringSliceEqual(oldC.Command, newC.Command) && len(newC.Command) > 0 {
				changes = append(changes, "Command已变更")
			}

			// 比较 Args
			if !stringSliceEqual(oldC.Args, newC.Args) && len(newC.Args) > 0 {
				changes = append(changes, "Args已变更")
			}

			// 比较 WorkingDir
			if oldC.WorkingDir != newC.WorkingDir && newC.WorkingDir != "" {
				changes = append(changes, fmt.Sprintf("WorkingDir: %s -> %s",
					defaultStrVal(oldC.WorkingDir, "未设置"), newC.WorkingDir))
			}

			// 比较 Lifecycle
			oldHasLifecycle := oldC.Lifecycle != nil
			newHasLifecycle := newC.Lifecycle != nil
			if oldHasLifecycle != newHasLifecycle {
				if newHasLifecycle {
					changes = append(changes, "新增生命周期钩子")
				} else {
					changes = append(changes, "移除生命周期钩子")
				}
			}

			// 比较 SecurityContext
			oldHasSecCtx := oldC.SecurityContext != nil
			newHasSecCtx := newC.SecurityContext != nil
			if oldHasSecCtx != newHasSecCtx {
				if newHasSecCtx {
					changes = append(changes, "新增安全上下文")
				} else {
					changes = append(changes, "移除安全上下文")
				}
			}
		}

		if len(changes) > 0 {
			containerType := newC.ContainerType
			if containerType == "" {
				containerType = "main"
			}
			containerChanges = append(containerChanges, fmt.Sprintf("容器[%s](%s): %s",
				newC.ContainerName, containerType, strings.Join(changes, ", ")))
		}
	}

	if len(containerChanges) == 0 {
		return ""
	}

	return strings.Join(containerChanges, "; ")
}

// defaultStrVal 如果字符串为空返回默认值
func defaultStrVal(s, defaultVal string) string {
	if s == "" {
		return defaultVal
	}
	return s
}

// stringSliceEqual 比较两个字符串切片是否相等
func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}
