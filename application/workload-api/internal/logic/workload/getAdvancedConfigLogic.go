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

type GetAdvancedConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetAdvancedConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAdvancedConfigLogic {
	return &GetAdvancedConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetAdvancedConfigLogic) GetAdvancedConfig(req *types.DefaultIdRequest) (resp *types.CommAdvancedConfigResponse, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	resourceType := strings.ToUpper(versionDetail.ResourceType)

	var config *k8sTypes.AdvancedConfigResponse

	switch resourceType {
	case "DEPLOYMENT":
		config, err = client.Deployment().GetAdvancedConfig(versionDetail.Namespace, versionDetail.ResourceName)
	case "STATEFULSET":
		config, err = client.StatefulSet().GetAdvancedConfig(versionDetail.Namespace, versionDetail.ResourceName)
	case "DAEMONSET":
		config, err = client.DaemonSet().GetAdvancedConfig(versionDetail.Namespace, versionDetail.ResourceName)
	case "CRONJOB":
		config, err = client.CronJob().GetAdvancedConfig(versionDetail.Namespace, versionDetail.ResourceName)
	default:
		return nil, fmt.Errorf("资源类型 %s 不支持查询高级配置", resourceType)
	}

	if err != nil {
		l.Errorf("获取高级配置失败: %v", err)
		return nil, fmt.Errorf("获取高级配置失败")
	}

	resp = convertK8sAdvancedConfigToApiAdvancedConfig(config)
	return resp, nil
}

// convertK8sAdvancedConfigToApiAdvancedConfig 将 k8sTypes.AdvancedConfigResponse 转换为 types.CommAdvancedConfigResponse
func convertK8sAdvancedConfigToApiAdvancedConfig(config *k8sTypes.AdvancedConfigResponse) *types.CommAdvancedConfigResponse {
	if config == nil {
		return &types.CommAdvancedConfigResponse{
			PodConfig:  types.CommPodAdvancedConfig{},
			Containers: []types.CommContainerAdvancedConfig{},
		}
	}

	result := &types.CommAdvancedConfigResponse{
		PodConfig:  convertK8sPodAdvancedConfigToApi(&config.PodConfig),
		Containers: make([]types.CommContainerAdvancedConfig, 0, len(config.Containers)),
	}

	for _, container := range config.Containers {
		result.Containers = append(result.Containers, convertK8sContainerAdvancedConfigToApi(&container))
	}

	return result
}

// convertK8sPodAdvancedConfigToApi 转换 Pod 级别高级配置
func convertK8sPodAdvancedConfigToApi(config *k8sTypes.PodAdvancedConfig) types.CommPodAdvancedConfig {
	if config == nil {
		return types.CommPodAdvancedConfig{}
	}

	result := types.CommPodAdvancedConfig{
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

	// 处理指针字段
	if config.TerminationGracePeriodSeconds != nil {
		result.TerminationGracePeriodSeconds = *config.TerminationGracePeriodSeconds
	}
	if config.HostNetwork != nil {
		result.HostNetwork = *config.HostNetwork
	}
	if config.HostPID != nil {
		result.HostPID = *config.HostPID
	}
	if config.HostIPC != nil {
		result.HostIPC = *config.HostIPC
	}
	if config.HostUsers != nil {
		result.HostUsers = *config.HostUsers
	}
	if config.AutomountServiceAccountToken != nil {
		result.AutomountServiceAccountToken = *config.AutomountServiceAccountToken
	}
	if config.Priority != nil {
		result.Priority = *config.Priority
	}
	if config.ShareProcessNamespace != nil {
		result.ShareProcessNamespace = *config.ShareProcessNamespace
	}
	if config.ActiveDeadlineSeconds != nil {
		result.ActiveDeadlineSeconds = *config.ActiveDeadlineSeconds
	}
	if config.RuntimeClassName != nil {
		result.RuntimeClassName = *config.RuntimeClassName
	}

	// 转换 DNSConfig
	if config.DNSConfig != nil {
		result.DNSConfig = convertK8sDNSConfigToApi(config.DNSConfig)
	}

	// 转换 HostAliases
	if len(config.HostAliases) > 0 {
		result.HostAliases = make([]types.CommHostAlias, 0, len(config.HostAliases))
		for _, ha := range config.HostAliases {
			result.HostAliases = append(result.HostAliases, types.CommHostAlias{
				IP:        ha.IP,
				Hostnames: ha.Hostnames,
			})
		}
	}

	// 转换 SecurityContext
	if config.SecurityContext != nil {
		result.SecurityContext = convertK8sPodSecurityContextToApi(config.SecurityContext)
	}

	// 转换 OS
	if config.OS != nil {
		result.OS = &types.CommPodOS{
			Name: config.OS.Name,
		}
	}

	return result
}

// convertK8sDNSConfigToApi 转换 DNS 配置
func convertK8sDNSConfigToApi(config *k8sTypes.DNSConfig) *types.CommDNSConfig {
	if config == nil {
		return nil
	}

	result := &types.CommDNSConfig{
		Nameservers: config.Nameservers,
		Searches:    config.Searches,
	}

	if len(config.Options) > 0 {
		result.Options = make([]types.CommDNSConfigOption, 0, len(config.Options))
		for _, opt := range config.Options {
			option := types.CommDNSConfigOption{
				Name: opt.Name,
			}
			if opt.Value != nil {
				option.Value = *opt.Value
			}
			result.Options = append(result.Options, option)
		}
	}

	return result
}

// convertK8sPodSecurityContextToApi 转换 Pod 安全上下文
func convertK8sPodSecurityContextToApi(ctx *k8sTypes.PodSecurityContext) *types.CommPodSecurityContext {
	if ctx == nil {
		return nil
	}

	result := &types.CommPodSecurityContext{
		FSGroupChangePolicy: ctx.FSGroupChangePolicy,
		SupplementalGroups:  ctx.SupplementalGroups,
	}

	if ctx.RunAsUser != nil {
		result.RunAsUser = *ctx.RunAsUser
	}
	if ctx.RunAsGroup != nil {
		result.RunAsGroup = *ctx.RunAsGroup
	}
	if ctx.RunAsNonRoot != nil {
		result.RunAsNonRoot = *ctx.RunAsNonRoot
	}
	if ctx.FSGroup != nil {
		result.FSGroup = *ctx.FSGroup
	}

	// 转换 SeccompProfile
	if ctx.SeccompProfile != nil {
		result.SeccompProfile = &types.CommSeccompProfile{
			Type:             ctx.SeccompProfile.Type,
			LocalhostProfile: ctx.SeccompProfile.LocalhostProfile,
		}
	}

	// 转换 SELinuxOptions
	if ctx.SELinuxOptions != nil {
		result.SELinuxOptions = &types.CommSELinuxOptions{
			User:  ctx.SELinuxOptions.User,
			Role:  ctx.SELinuxOptions.Role,
			Type:  ctx.SELinuxOptions.Type,
			Level: ctx.SELinuxOptions.Level,
		}
	}

	// 转换 AppArmorProfile
	if ctx.AppArmorProfile != nil {
		result.AppArmorProfile = &types.CommAppArmorProfile{
			Type:             ctx.AppArmorProfile.Type,
			LocalhostProfile: ctx.AppArmorProfile.LocalhostProfile,
		}
	}

	// 转换 Sysctls
	if len(ctx.Sysctls) > 0 {
		result.Sysctls = make([]types.CommSysctl, 0, len(ctx.Sysctls))
		for _, s := range ctx.Sysctls {
			result.Sysctls = append(result.Sysctls, types.CommSysctl{
				Name:  s.Name,
				Value: s.Value,
			})
		}
	}

	// 转换 WindowsOptions
	if ctx.WindowsOptions != nil {
		result.WindowsOptions = &types.CommWindowsSecurityContextOptions{
			GMSACredentialSpecName: ctx.WindowsOptions.GMSACredentialSpecName,
			GMSACredentialSpec:     ctx.WindowsOptions.GMSACredentialSpec,
			RunAsUserName:          ctx.WindowsOptions.RunAsUserName,
		}
		if ctx.WindowsOptions.HostProcess != nil {
			result.WindowsOptions.HostProcess = *ctx.WindowsOptions.HostProcess
		}
	}

	return result
}

// convertK8sContainerAdvancedConfigToApi 转换容器级别高级配置
func convertK8sContainerAdvancedConfigToApi(config *k8sTypes.ContainerAdvancedConfig) types.CommContainerAdvancedConfig {
	if config == nil {
		return types.CommContainerAdvancedConfig{}
	}

	result := types.CommContainerAdvancedConfig{
		ContainerName:   config.ContainerName,
		ContainerType:   string(config.ContainerType),
		ImagePullPolicy: config.ImagePullPolicy,
		Command:         config.Command,
		Args:            config.Args,
		WorkingDir:      config.WorkingDir,
	}

	// 处理指针字段
	if config.Stdin != nil {
		result.Stdin = *config.Stdin
	}
	if config.StdinOnce != nil {
		result.StdinOnce = *config.StdinOnce
	}
	if config.TTY != nil {
		result.TTY = *config.TTY
	}

	// 转换 Lifecycle
	if config.Lifecycle != nil {
		result.Lifecycle = convertK8sLifecycleToApi(config.Lifecycle)
	}

	// 转换 SecurityContext
	if config.SecurityContext != nil {
		result.SecurityContext = convertK8sContainerSecurityContextToApi(config.SecurityContext)
	}

	return result
}

// convertK8sLifecycleToApi 转换生命周期钩子
func convertK8sLifecycleToApi(lifecycle *k8sTypes.Lifecycle) *types.CommLifecycle {
	if lifecycle == nil {
		return nil
	}

	result := &types.CommLifecycle{}

	if lifecycle.PostStart != nil {
		result.PostStart = convertK8sLifecycleHandlerToApi(lifecycle.PostStart)
	}
	if lifecycle.PreStop != nil {
		result.PreStop = convertK8sLifecycleHandlerToApi(lifecycle.PreStop)
	}

	return result
}

// convertK8sLifecycleHandlerToApi 转换生命周期处理器
func convertK8sLifecycleHandlerToApi(handler *k8sTypes.LifecycleHandler) *types.CommLifecycleHandler {
	if handler == nil {
		return nil
	}

	result := &types.CommLifecycleHandler{
		Type: handler.Type,
	}

	if handler.Exec != nil {
		result.Exec = &types.CommExecAction{
			Command: handler.Exec.Command,
		}
	}

	if handler.HTTPGet != nil {
		result.HTTPGet = &types.CommHTTPGetAction{
			Path:   handler.HTTPGet.Path,
			Port:   handler.HTTPGet.Port,
			Host:   handler.HTTPGet.Host,
			Scheme: handler.HTTPGet.Scheme,
		}
		if len(handler.HTTPGet.HTTPHeaders) > 0 {
			result.HTTPGet.HTTPHeaders = make([]types.CommHTTPHeader, 0, len(handler.HTTPGet.HTTPHeaders))
			for _, h := range handler.HTTPGet.HTTPHeaders {
				result.HTTPGet.HTTPHeaders = append(result.HTTPGet.HTTPHeaders, types.CommHTTPHeader{
					Name:  h.Name,
					Value: h.Value,
				})
			}
		}
	}

	if handler.TCPSocket != nil {
		result.TCPSocket = &types.CommTCPSocketAction{
			Port: handler.TCPSocket.Port,
			Host: handler.TCPSocket.Host,
		}
	}

	if handler.Sleep != nil {
		result.Sleep = &types.CommSleepAction{
			Seconds: handler.Sleep.Seconds,
		}
	}

	return result
}

// convertK8sContainerSecurityContextToApi 转换容器安全上下文
func convertK8sContainerSecurityContextToApi(ctx *k8sTypes.ContainerSecurityContext) *types.CommContainerSecurityContext {
	if ctx == nil {
		return nil
	}

	result := &types.CommContainerSecurityContext{
		ProcMount: ctx.ProcMount,
	}

	if ctx.RunAsUser != nil {
		result.RunAsUser = *ctx.RunAsUser
	}
	if ctx.RunAsGroup != nil {
		result.RunAsGroup = *ctx.RunAsGroup
	}
	if ctx.RunAsNonRoot != nil {
		result.RunAsNonRoot = *ctx.RunAsNonRoot
	}
	if ctx.ReadOnlyRootFilesystem != nil {
		result.ReadOnlyRootFilesystem = *ctx.ReadOnlyRootFilesystem
	}
	if ctx.Privileged != nil {
		result.Privileged = *ctx.Privileged
	}
	if ctx.AllowPrivilegeEscalation != nil {
		result.AllowPrivilegeEscalation = *ctx.AllowPrivilegeEscalation
	}

	// 转换 Capabilities
	if ctx.Capabilities != nil {
		result.Capabilities = &types.CommCapabilities{
			Add:  ctx.Capabilities.Add,
			Drop: ctx.Capabilities.Drop,
		}
	}

	// 转换 SeccompProfile
	if ctx.SeccompProfile != nil {
		result.SeccompProfile = &types.CommSeccompProfile{
			Type:             ctx.SeccompProfile.Type,
			LocalhostProfile: ctx.SeccompProfile.LocalhostProfile,
		}
	}

	// 转换 SELinuxOptions
	if ctx.SELinuxOptions != nil {
		result.SELinuxOptions = &types.CommSELinuxOptions{
			User:  ctx.SELinuxOptions.User,
			Role:  ctx.SELinuxOptions.Role,
			Type:  ctx.SELinuxOptions.Type,
			Level: ctx.SELinuxOptions.Level,
		}
	}

	// 转换 AppArmorProfile
	if ctx.AppArmorProfile != nil {
		result.AppArmorProfile = &types.CommAppArmorProfile{
			Type:             ctx.AppArmorProfile.Type,
			LocalhostProfile: ctx.AppArmorProfile.LocalhostProfile,
		}
	}

	return result
}
