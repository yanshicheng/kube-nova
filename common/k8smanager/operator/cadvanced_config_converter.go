package operator

import (
	"fmt"

	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ============================================================================
// 高级配置转换函数
// 适用于：Deployment, DaemonSet, StatefulSet, CronJob
// 注意：CronJob/Job 的 RestartPolicy 有特殊限制 (OnFailure/Never)
//       StatefulSet 的存储配置需要单独处理
// ============================================================================

// ConvertPodSpecToAdvancedConfig 从 PodSpec 提取高级配置
// 适用于：Deployment, DaemonSet, StatefulSet, CronJob
func ConvertPodSpecToAdvancedConfig(spec *corev1.PodSpec) *types.AdvancedConfigResponse {
	if spec == nil {
		return &types.AdvancedConfigResponse{
			PodConfig:  types.PodAdvancedConfig{},
			Containers: []types.ContainerAdvancedConfig{},
		}
	}

	response := &types.AdvancedConfigResponse{
		PodConfig:  convertPodSpecToPodAdvancedConfig(spec),
		Containers: make([]types.ContainerAdvancedConfig, 0),
	}

	// Init 容器
	for _, c := range spec.InitContainers {
		response.Containers = append(response.Containers,
			convertContainerToAdvancedConfig(c, types.ContainerTypeInit))
	}

	// 主容器
	for _, c := range spec.Containers {
		response.Containers = append(response.Containers,
			convertContainerToAdvancedConfig(c, types.ContainerTypeMain))
	}

	// Ephemeral 容器
	for _, c := range spec.EphemeralContainers {
		response.Containers = append(response.Containers,
			convertEphemeralContainerToAdvancedConfig(c))
	}

	return response
}

// ApplyAdvancedConfigToPodSpec 应用高级配置到 PodSpec
// 适用于：Deployment, DaemonSet, StatefulSet, CronJob
// 注意：调用者需要根据资源类型处理 RestartPolicy 的限制
func ApplyAdvancedConfigToPodSpec(spec *corev1.PodSpec, req *types.UpdateAdvancedConfigRequest) error {
	if spec == nil {
		return fmt.Errorf("PodSpec 不能为空")
	}
	if req == nil {
		return nil
	}

	// 应用 Pod 级别配置
	if req.PodConfig != nil {
		applyPodAdvancedConfigToPodSpec(spec, req.PodConfig)
	}

	// 构建容器映射
	containerConfigMap := make(map[string]types.ContainerAdvancedConfig)
	for _, c := range req.Containers {
		containerConfigMap[c.ContainerName] = c
	}

	// 更新 Init 容器
	for i := range spec.InitContainers {
		name := spec.InitContainers[i].Name
		if config, exists := containerConfigMap[name]; exists {
			if config.ContainerType != "" && config.ContainerType != types.ContainerTypeInit {
				return fmt.Errorf("容器 '%s' 类型不匹配，期望 init，实际 %s", name, config.ContainerType)
			}
			applyContainerAdvancedConfig(&spec.InitContainers[i], config)
		}
	}

	// 更新主容器
	for i := range spec.Containers {
		name := spec.Containers[i].Name
		if config, exists := containerConfigMap[name]; exists {
			if config.ContainerType != "" && config.ContainerType != types.ContainerTypeMain {
				return fmt.Errorf("容器 '%s' 类型不匹配，期望 main，实际 %s", name, config.ContainerType)
			}
			applyContainerAdvancedConfig(&spec.Containers[i], config)
		}
	}

	return nil
}

// convertPodSpecToPodAdvancedConfig 转换 PodSpec -> PodAdvancedConfig
func convertPodSpecToPodAdvancedConfig(spec *corev1.PodSpec) types.PodAdvancedConfig {
	config := types.PodAdvancedConfig{
		TerminationGracePeriodSeconds: spec.TerminationGracePeriodSeconds,
		RestartPolicy:                 string(spec.RestartPolicy),
		DNSPolicy:                     string(spec.DNSPolicy),
		Hostname:                      spec.Hostname,
		Subdomain:                     spec.Subdomain,
		ServiceAccountName:            spec.ServiceAccountName,
		AutomountServiceAccountToken:  spec.AutomountServiceAccountToken,
		PriorityClassName:             spec.PriorityClassName,
		Priority:                      spec.Priority,
		PreemptionPolicy:              string(ptrToPreemptionPolicy(spec.PreemptionPolicy)),
		RuntimeClassName:              spec.RuntimeClassName,
		ShareProcessNamespace:         spec.ShareProcessNamespace,
		SchedulerName:                 spec.SchedulerName,
		ActiveDeadlineSeconds:         spec.ActiveDeadlineSeconds,
	}

	// 主机命名空间
	if spec.HostNetwork {
		config.HostNetwork = &spec.HostNetwork
	}
	if spec.HostPID {
		config.HostPID = &spec.HostPID
	}
	if spec.HostIPC {
		config.HostIPC = &spec.HostIPC
	}
	if spec.HostUsers != nil {
		config.HostUsers = spec.HostUsers
	}

	// DNS 配置
	if spec.DNSConfig != nil {
		config.DNSConfig = convertK8sDNSConfigToType(spec.DNSConfig)
	}

	// HostAliases
	if len(spec.HostAliases) > 0 {
		config.HostAliases = make([]types.HostAlias, 0, len(spec.HostAliases))
		for _, ha := range spec.HostAliases {
			config.HostAliases = append(config.HostAliases, types.HostAlias{
				IP:        ha.IP,
				Hostnames: ha.Hostnames,
			})
		}
	}

	// ImagePullSecrets
	if len(spec.ImagePullSecrets) > 0 {
		config.ImagePullSecrets = make([]string, 0, len(spec.ImagePullSecrets))
		for _, s := range spec.ImagePullSecrets {
			config.ImagePullSecrets = append(config.ImagePullSecrets, s.Name)
		}
	}

	// 安全上下文
	if spec.SecurityContext != nil {
		config.SecurityContext = convertK8sPodSecurityContextToType(spec.SecurityContext)
	}

	// OS
	if spec.OS != nil {
		config.OS = &types.PodOS{Name: string(spec.OS.Name)}
	}

	return config
}

// applyPodAdvancedConfigToPodSpec 应用 PodAdvancedConfig 到 PodSpec
func applyPodAdvancedConfigToPodSpec(spec *corev1.PodSpec, config *types.PodAdvancedConfig) {
	if config == nil {
		return
	}

	spec.TerminationGracePeriodSeconds = config.TerminationGracePeriodSeconds

	if config.RestartPolicy != "" {
		spec.RestartPolicy = corev1.RestartPolicy(config.RestartPolicy)
	}

	if config.DNSPolicy != "" {
		spec.DNSPolicy = corev1.DNSPolicy(config.DNSPolicy)
	}

	spec.DNSConfig = convertTypeDNSConfigToK8s(config.DNSConfig)
	spec.Hostname = config.Hostname
	spec.Subdomain = config.Subdomain

	if config.HostNetwork != nil {
		spec.HostNetwork = *config.HostNetwork
	}
	if config.HostPID != nil {
		spec.HostPID = *config.HostPID
	}
	if config.HostIPC != nil {
		spec.HostIPC = *config.HostIPC
	}
	spec.HostUsers = config.HostUsers

	// HostAliases
	if config.HostAliases != nil {
		spec.HostAliases = make([]corev1.HostAlias, 0, len(config.HostAliases))
		for _, ha := range config.HostAliases {
			spec.HostAliases = append(spec.HostAliases, corev1.HostAlias{
				IP:        ha.IP,
				Hostnames: ha.Hostnames,
			})
		}
	}

	spec.ServiceAccountName = config.ServiceAccountName
	spec.AutomountServiceAccountToken = config.AutomountServiceAccountToken

	// ImagePullSecrets
	if config.ImagePullSecrets != nil {
		spec.ImagePullSecrets = make([]corev1.LocalObjectReference, 0, len(config.ImagePullSecrets))
		for _, name := range config.ImagePullSecrets {
			spec.ImagePullSecrets = append(spec.ImagePullSecrets, corev1.LocalObjectReference{Name: name})
		}
	}

	spec.PriorityClassName = config.PriorityClassName
	spec.Priority = config.Priority

	if config.PreemptionPolicy != "" {
		pp := corev1.PreemptionPolicy(config.PreemptionPolicy)
		spec.PreemptionPolicy = &pp
	}

	spec.RuntimeClassName = config.RuntimeClassName
	spec.SecurityContext = convertTypePodSecurityContextToK8s(config.SecurityContext)
	spec.ShareProcessNamespace = config.ShareProcessNamespace

	if config.OS != nil {
		spec.OS = &corev1.PodOS{Name: corev1.OSName(config.OS.Name)}
	} else {
		spec.OS = nil
	}

	spec.SchedulerName = config.SchedulerName
	spec.ActiveDeadlineSeconds = config.ActiveDeadlineSeconds
}

// convertContainerToAdvancedConfig 转换普通容器 -> ContainerAdvancedConfig
func convertContainerToAdvancedConfig(c corev1.Container, containerType types.ContainerType) types.ContainerAdvancedConfig {
	config := types.ContainerAdvancedConfig{
		ContainerName:   c.Name,
		ContainerType:   containerType,
		ImagePullPolicy: string(c.ImagePullPolicy),
		Command:         c.Command,
		Args:            c.Args,
		WorkingDir:      c.WorkingDir,
	}

	if c.Stdin {
		config.Stdin = &c.Stdin
	}
	if c.StdinOnce {
		config.StdinOnce = &c.StdinOnce
	}
	if c.TTY {
		config.TTY = &c.TTY
	}

	if c.Lifecycle != nil {
		config.Lifecycle = convertK8sLifecycleToType(c.Lifecycle)
	}

	if c.SecurityContext != nil {
		config.SecurityContext = convertK8sContainerSecurityContextToType(c.SecurityContext)
	}

	return config
}

// convertEphemeralContainerToAdvancedConfig 转换 Ephemeral 容器
func convertEphemeralContainerToAdvancedConfig(c corev1.EphemeralContainer) types.ContainerAdvancedConfig {
	config := types.ContainerAdvancedConfig{
		ContainerName:   c.Name,
		ContainerType:   types.ContainerTypeEphemeral,
		ImagePullPolicy: string(c.ImagePullPolicy),
		Command:         c.Command,
		Args:            c.Args,
		WorkingDir:      c.WorkingDir,
	}

	if c.Stdin {
		config.Stdin = &c.Stdin
	}
	if c.StdinOnce {
		config.StdinOnce = &c.StdinOnce
	}
	if c.TTY {
		config.TTY = &c.TTY
	}

	if c.Lifecycle != nil {
		config.Lifecycle = convertK8sLifecycleToType(c.Lifecycle)
	}

	if c.SecurityContext != nil {
		config.SecurityContext = convertK8sContainerSecurityContextToType(c.SecurityContext)
	}

	return config
}

// applyContainerAdvancedConfig 应用高级配置到容器
func applyContainerAdvancedConfig(c *corev1.Container, config types.ContainerAdvancedConfig) {
	if config.ImagePullPolicy != "" {
		c.ImagePullPolicy = corev1.PullPolicy(config.ImagePullPolicy)
	}

	if config.Command != nil {
		c.Command = config.Command
	}
	if config.Args != nil {
		c.Args = config.Args
	}
	c.WorkingDir = config.WorkingDir

	if config.Stdin != nil {
		c.Stdin = *config.Stdin
	}
	if config.StdinOnce != nil {
		c.StdinOnce = *config.StdinOnce
	}
	if config.TTY != nil {
		c.TTY = *config.TTY
	}

	c.Lifecycle = convertTypeLifecycleToK8s(config.Lifecycle)
	c.SecurityContext = convertTypeContainerSecurityContextToK8s(config.SecurityContext)
}

// ============================================================================
// DNS 配置转换
// ============================================================================

func convertK8sDNSConfigToType(dns *corev1.PodDNSConfig) *types.DNSConfig {
	if dns == nil {
		return nil
	}
	config := &types.DNSConfig{
		Nameservers: dns.Nameservers,
		Searches:    dns.Searches,
	}
	if len(dns.Options) > 0 {
		config.Options = make([]types.DNSConfigOption, 0, len(dns.Options))
		for _, opt := range dns.Options {
			config.Options = append(config.Options, types.DNSConfigOption{
				Name:  opt.Name,
				Value: opt.Value,
			})
		}
	}
	return config
}

func convertTypeDNSConfigToK8s(dns *types.DNSConfig) *corev1.PodDNSConfig {
	if dns == nil {
		return nil
	}
	config := &corev1.PodDNSConfig{
		Nameservers: dns.Nameservers,
		Searches:    dns.Searches,
	}
	if len(dns.Options) > 0 {
		config.Options = make([]corev1.PodDNSConfigOption, 0, len(dns.Options))
		for _, opt := range dns.Options {
			config.Options = append(config.Options, corev1.PodDNSConfigOption{
				Name:  opt.Name,
				Value: opt.Value,
			})
		}
	}
	return config
}

// ============================================================================
// 生命周期钩子转换
// ============================================================================

func convertK8sLifecycleToType(lc *corev1.Lifecycle) *types.Lifecycle {
	if lc == nil {
		return nil
	}
	return &types.Lifecycle{
		PostStart: convertK8sLifecycleHandlerToType(lc.PostStart),
		PreStop:   convertK8sLifecycleHandlerToType(lc.PreStop),
	}
}

func convertK8sLifecycleHandlerToType(h *corev1.LifecycleHandler) *types.LifecycleHandler {
	if h == nil {
		return nil
	}
	handler := &types.LifecycleHandler{}
	if h.Exec != nil {
		handler.Type = "exec"
		handler.Exec = &types.ExecAction{Command: h.Exec.Command}
	} else if h.HTTPGet != nil {
		handler.Type = "httpGet"
		headers := make([]types.HTTPHeader, 0, len(h.HTTPGet.HTTPHeaders))
		for _, hdr := range h.HTTPGet.HTTPHeaders {
			headers = append(headers, types.HTTPHeader{Name: hdr.Name, Value: hdr.Value})
		}
		handler.HTTPGet = &types.HTTPGetAction{
			Path:        h.HTTPGet.Path,
			Port:        h.HTTPGet.Port.IntVal,
			Host:        h.HTTPGet.Host,
			Scheme:      string(h.HTTPGet.Scheme),
			HTTPHeaders: headers,
		}
	} else if h.TCPSocket != nil {
		handler.Type = "tcpSocket"
		handler.TCPSocket = &types.TCPSocketAction{
			Port: h.TCPSocket.Port.IntVal,
			Host: h.TCPSocket.Host,
		}
	} else if h.Sleep != nil {
		handler.Type = "sleep"
		handler.Sleep = &types.SleepAction{Seconds: h.Sleep.Seconds}
	}
	return handler
}

func convertTypeLifecycleToK8s(lc *types.Lifecycle) *corev1.Lifecycle {
	if lc == nil {
		return nil
	}
	return &corev1.Lifecycle{
		PostStart: convertTypeLifecycleHandlerToK8s(lc.PostStart),
		PreStop:   convertTypeLifecycleHandlerToK8s(lc.PreStop),
	}
}

func convertTypeLifecycleHandlerToK8s(h *types.LifecycleHandler) *corev1.LifecycleHandler {
	if h == nil {
		return nil
	}
	handler := &corev1.LifecycleHandler{}
	switch h.Type {
	case "exec":
		if h.Exec != nil {
			handler.Exec = &corev1.ExecAction{Command: h.Exec.Command}
		}
	case "httpGet":
		if h.HTTPGet != nil {
			headers := make([]corev1.HTTPHeader, 0, len(h.HTTPGet.HTTPHeaders))
			for _, hdr := range h.HTTPGet.HTTPHeaders {
				headers = append(headers, corev1.HTTPHeader{Name: hdr.Name, Value: hdr.Value})
			}
			handler.HTTPGet = &corev1.HTTPGetAction{
				Path:        h.HTTPGet.Path,
				Port:        intstrFromInt32(h.HTTPGet.Port),
				Host:        h.HTTPGet.Host,
				Scheme:      corev1.URIScheme(h.HTTPGet.Scheme),
				HTTPHeaders: headers,
			}
		}
	case "tcpSocket":
		if h.TCPSocket != nil {
			handler.TCPSocket = &corev1.TCPSocketAction{
				Port: intstrFromInt32(h.TCPSocket.Port),
				Host: h.TCPSocket.Host,
			}
		}
	case "sleep":
		if h.Sleep != nil {
			handler.Sleep = &corev1.SleepAction{Seconds: h.Sleep.Seconds}
		}
	}
	return handler
}

// ============================================================================
// 安全上下文转换
// ============================================================================

func convertK8sPodSecurityContextToType(sc *corev1.PodSecurityContext) *types.PodSecurityContext {
	if sc == nil {
		return nil
	}
	psc := &types.PodSecurityContext{
		RunAsUser:           sc.RunAsUser,
		RunAsGroup:          sc.RunAsGroup,
		RunAsNonRoot:        sc.RunAsNonRoot,
		SupplementalGroups:  sc.SupplementalGroups,
		FSGroup:             sc.FSGroup,
		FSGroupChangePolicy: string(ptrToFSGroupChangePolicy(sc.FSGroupChangePolicy)),
	}
	if sc.SeccompProfile != nil {
		psc.SeccompProfile = convertK8sSeccompProfileToType(sc.SeccompProfile)
	}
	if sc.SELinuxOptions != nil {
		psc.SELinuxOptions = convertK8sSELinuxOptionsToType(sc.SELinuxOptions)
	}
	if sc.AppArmorProfile != nil {
		psc.AppArmorProfile = convertK8sAppArmorProfileToType(sc.AppArmorProfile)
	}
	if len(sc.Sysctls) > 0 {
		psc.Sysctls = make([]types.Sysctl, 0, len(sc.Sysctls))
		for _, s := range sc.Sysctls {
			psc.Sysctls = append(psc.Sysctls, types.Sysctl{Name: s.Name, Value: s.Value})
		}
	}
	if sc.WindowsOptions != nil {
		psc.WindowsOptions = convertK8sWindowsOptionsToType(sc.WindowsOptions)
	}
	return psc
}

func convertTypePodSecurityContextToK8s(psc *types.PodSecurityContext) *corev1.PodSecurityContext {
	if psc == nil {
		return nil
	}
	sc := &corev1.PodSecurityContext{
		RunAsUser:          psc.RunAsUser,
		RunAsGroup:         psc.RunAsGroup,
		RunAsNonRoot:       psc.RunAsNonRoot,
		SupplementalGroups: psc.SupplementalGroups,
		FSGroup:            psc.FSGroup,
	}
	if psc.FSGroupChangePolicy != "" {
		policy := corev1.PodFSGroupChangePolicy(psc.FSGroupChangePolicy)
		sc.FSGroupChangePolicy = &policy
	}
	sc.SeccompProfile = convertTypeSeccompProfileToK8s(psc.SeccompProfile)
	sc.SELinuxOptions = convertTypeSELinuxOptionsToK8s(psc.SELinuxOptions)
	sc.AppArmorProfile = convertTypeAppArmorProfileToK8s(psc.AppArmorProfile)
	if len(psc.Sysctls) > 0 {
		sc.Sysctls = make([]corev1.Sysctl, 0, len(psc.Sysctls))
		for _, s := range psc.Sysctls {
			sc.Sysctls = append(sc.Sysctls, corev1.Sysctl{Name: s.Name, Value: s.Value})
		}
	}
	sc.WindowsOptions = convertTypeWindowsOptionsToK8s(psc.WindowsOptions)
	return sc
}

func convertK8sContainerSecurityContextToType(sc *corev1.SecurityContext) *types.ContainerSecurityContext {
	if sc == nil {
		return nil
	}
	csc := &types.ContainerSecurityContext{
		RunAsUser:                sc.RunAsUser,
		RunAsGroup:               sc.RunAsGroup,
		RunAsNonRoot:             sc.RunAsNonRoot,
		ReadOnlyRootFilesystem:   sc.ReadOnlyRootFilesystem,
		Privileged:               sc.Privileged,
		AllowPrivilegeEscalation: sc.AllowPrivilegeEscalation,
	}
	if sc.ProcMount != nil {
		csc.ProcMount = string(*sc.ProcMount)
	}
	if sc.Capabilities != nil {
		csc.Capabilities = &types.Capabilities{}
		for _, c := range sc.Capabilities.Add {
			csc.Capabilities.Add = append(csc.Capabilities.Add, string(c))
		}
		for _, c := range sc.Capabilities.Drop {
			csc.Capabilities.Drop = append(csc.Capabilities.Drop, string(c))
		}
	}
	csc.SeccompProfile = convertK8sSeccompProfileToType(sc.SeccompProfile)
	csc.SELinuxOptions = convertK8sSELinuxOptionsToType(sc.SELinuxOptions)
	csc.AppArmorProfile = convertK8sAppArmorProfileToType(sc.AppArmorProfile)
	return csc
}

func convertTypeContainerSecurityContextToK8s(csc *types.ContainerSecurityContext) *corev1.SecurityContext {
	if csc == nil {
		return nil
	}
	sc := &corev1.SecurityContext{
		RunAsUser:                csc.RunAsUser,
		RunAsGroup:               csc.RunAsGroup,
		RunAsNonRoot:             csc.RunAsNonRoot,
		ReadOnlyRootFilesystem:   csc.ReadOnlyRootFilesystem,
		Privileged:               csc.Privileged,
		AllowPrivilegeEscalation: csc.AllowPrivilegeEscalation,
	}
	if csc.ProcMount != "" {
		pm := corev1.ProcMountType(csc.ProcMount)
		sc.ProcMount = &pm
	}
	if csc.Capabilities != nil {
		sc.Capabilities = &corev1.Capabilities{}
		for _, c := range csc.Capabilities.Add {
			sc.Capabilities.Add = append(sc.Capabilities.Add, corev1.Capability(c))
		}
		for _, c := range csc.Capabilities.Drop {
			sc.Capabilities.Drop = append(sc.Capabilities.Drop, corev1.Capability(c))
		}
	}
	sc.SeccompProfile = convertTypeSeccompProfileToK8s(csc.SeccompProfile)
	sc.SELinuxOptions = convertTypeSELinuxOptionsToK8s(csc.SELinuxOptions)
	sc.AppArmorProfile = convertTypeAppArmorProfileToK8s(csc.AppArmorProfile)
	return sc
}

// ============================================================================
// 安全配置辅助转换
// ============================================================================

func convertK8sSeccompProfileToType(sp *corev1.SeccompProfile) *types.SeccompProfile {
	if sp == nil {
		return nil
	}
	profile := &types.SeccompProfile{Type: string(sp.Type)}
	if sp.LocalhostProfile != nil {
		profile.LocalhostProfile = *sp.LocalhostProfile
	}
	return profile
}

func convertTypeSeccompProfileToK8s(sp *types.SeccompProfile) *corev1.SeccompProfile {
	if sp == nil {
		return nil
	}
	profile := &corev1.SeccompProfile{Type: corev1.SeccompProfileType(sp.Type)}
	if sp.LocalhostProfile != "" {
		profile.LocalhostProfile = &sp.LocalhostProfile
	}
	return profile
}

func convertK8sSELinuxOptionsToType(se *corev1.SELinuxOptions) *types.SELinuxOptions {
	if se == nil {
		return nil
	}
	return &types.SELinuxOptions{User: se.User, Role: se.Role, Type: se.Type, Level: se.Level}
}

func convertTypeSELinuxOptionsToK8s(se *types.SELinuxOptions) *corev1.SELinuxOptions {
	if se == nil {
		return nil
	}
	return &corev1.SELinuxOptions{User: se.User, Role: se.Role, Type: se.Type, Level: se.Level}
}

func convertK8sAppArmorProfileToType(ap *corev1.AppArmorProfile) *types.AppArmorProfile {
	if ap == nil {
		return nil
	}
	profile := &types.AppArmorProfile{Type: string(ap.Type)}
	if ap.LocalhostProfile != nil {
		profile.LocalhostProfile = *ap.LocalhostProfile
	}
	return profile
}

func convertTypeAppArmorProfileToK8s(ap *types.AppArmorProfile) *corev1.AppArmorProfile {
	if ap == nil {
		return nil
	}
	profile := &corev1.AppArmorProfile{Type: corev1.AppArmorProfileType(ap.Type)}
	if ap.LocalhostProfile != "" {
		profile.LocalhostProfile = &ap.LocalhostProfile
	}
	return profile
}

func convertK8sWindowsOptionsToType(wo *corev1.WindowsSecurityContextOptions) *types.WindowsSecurityContextOptions {
	if wo == nil {
		return nil
	}
	opts := &types.WindowsSecurityContextOptions{HostProcess: wo.HostProcess}
	if wo.GMSACredentialSpecName != nil {
		opts.GMSACredentialSpecName = *wo.GMSACredentialSpecName
	}
	if wo.GMSACredentialSpec != nil {
		opts.GMSACredentialSpec = *wo.GMSACredentialSpec
	}
	if wo.RunAsUserName != nil {
		opts.RunAsUserName = *wo.RunAsUserName
	}
	return opts
}

func convertTypeWindowsOptionsToK8s(wo *types.WindowsSecurityContextOptions) *corev1.WindowsSecurityContextOptions {
	if wo == nil {
		return nil
	}
	opts := &corev1.WindowsSecurityContextOptions{HostProcess: wo.HostProcess}
	if wo.GMSACredentialSpecName != "" {
		opts.GMSACredentialSpecName = &wo.GMSACredentialSpecName
	}
	if wo.GMSACredentialSpec != "" {
		opts.GMSACredentialSpec = &wo.GMSACredentialSpec
	}
	if wo.RunAsUserName != "" {
		opts.RunAsUserName = &wo.RunAsUserName
	}
	return opts
}

// ============================================================================
// 辅助函数
// ============================================================================

func ptrToPreemptionPolicy(pp *corev1.PreemptionPolicy) corev1.PreemptionPolicy {
	if pp == nil {
		return ""
	}
	return *pp
}

func ptrToFSGroupChangePolicy(p *corev1.PodFSGroupChangePolicy) corev1.PodFSGroupChangePolicy {
	if p == nil {
		return ""
	}
	return *p
}

func intstrFromInt32(val int32) intstr.IntOrString {
	return intstr.FromInt32(val)
}
