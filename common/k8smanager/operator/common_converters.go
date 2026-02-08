package operator

import (
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ============================================================================
// 通用 PodSpec 转换函数
// 适用于：Deployment, DaemonSet, StatefulSet, CronJob
// 注意：StatefulSet 的存储配置需要单独处理 VolumeClaimTemplates
// ============================================================================

// ============================================================================
// 镜像管理转换
// ============================================================================

// ConvertPodSpecToContainerImages 从 PodSpec 提取容器镜像信息
// 适用于：Deployment, DaemonSet, StatefulSet, CronJob
func ConvertPodSpecToContainerImages(spec *corev1.PodSpec) *types.ContainerInfoList {
	if spec == nil {
		return &types.ContainerInfoList{
			InitContainers:      []types.ContainerInfo{},
			Containers:          []types.ContainerInfo{},
			EphemeralContainers: []types.ContainerInfo{},
		}
	}

	result := &types.ContainerInfoList{
		InitContainers:      make([]types.ContainerInfo, 0, len(spec.InitContainers)),
		Containers:          make([]types.ContainerInfo, 0, len(spec.Containers)),
		EphemeralContainers: make([]types.ContainerInfo, 0, len(spec.EphemeralContainers)),
	}

	for _, c := range spec.InitContainers {
		result.InitContainers = append(result.InitContainers, types.ContainerInfo{
			Name:  c.Name,
			Image: c.Image,
		})
	}

	for _, c := range spec.Containers {
		result.Containers = append(result.Containers, types.ContainerInfo{
			Name:  c.Name,
			Image: c.Image,
		})
	}

	for _, c := range spec.EphemeralContainers {
		result.EphemeralContainers = append(result.EphemeralContainers, types.ContainerInfo{
			Name:  c.Name,
			Image: c.Image,
		})
	}

	return result
}

// ApplyImagesToPodSpec 应用镜像更新到 PodSpec
// 适用于：Deployment, DaemonSet, StatefulSet, CronJob
// 返回变更列表用于记录 change-cause
func ApplyImagesToPodSpec(spec *corev1.PodSpec, containers []types.ContainerImageInfo) ([]string, error) {
	if spec == nil {
		return nil, fmt.Errorf("PodSpec 不能为空")
	}

	// 构建容器名到索引的映射
	initContainerMap := make(map[string]int)
	for i, c := range spec.InitContainers {
		initContainerMap[c.Name] = i
	}

	containerMap := make(map[string]int)
	for i, c := range spec.Containers {
		containerMap[c.Name] = i
	}

	ephemeralContainerMap := make(map[string]int)
	for i, c := range spec.EphemeralContainers {
		ephemeralContainerMap[c.Name] = i
	}

	var changes []string

	for _, img := range containers {
		switch img.ContainerType {
		case types.ContainerTypeInit:
			if idx, exists := initContainerMap[img.ContainerName]; exists {
				container := &spec.InitContainers[idx]
				if container.Image != img.Image {
					changes = append(changes, fmt.Sprintf("%s=%s", img.ContainerName, extractImageTag(img.Image)))
					container.Image = img.Image
				}
			}
		case types.ContainerTypeMain, "":
			if idx, exists := containerMap[img.ContainerName]; exists {
				container := &spec.Containers[idx]
				if container.Image != img.Image {
					changes = append(changes, fmt.Sprintf("%s=%s", img.ContainerName, extractImageTag(img.Image)))
					container.Image = img.Image
				}
			}
		case types.ContainerTypeEphemeral:
			if idx, exists := ephemeralContainerMap[img.ContainerName]; exists {
				container := &spec.EphemeralContainers[idx]
				if container.Image != img.Image {
					changes = append(changes, fmt.Sprintf("%s=%s", img.ContainerName, extractImageTag(img.Image)))
					container.Image = img.Image
				}
			}
		}
	}

	return changes, nil
}

// ============================================================================
// 资源配额转换
// ============================================================================

// ConvertPodSpecToResources 从 PodSpec 提取资源配额
// 适用于：Deployment, DaemonSet, StatefulSet, CronJob
func ConvertPodSpecToResources(spec *corev1.PodSpec) *types.ResourcesResponse {
	if spec == nil {
		return &types.ResourcesResponse{Containers: []types.ContainerResources{}}
	}

	response := &types.ResourcesResponse{
		Containers: make([]types.ContainerResources, 0),
	}

	// Init 容器
	for _, c := range spec.InitContainers {
		response.Containers = append(response.Containers, types.ContainerResources{
			ContainerName: c.Name,
			ContainerType: types.ContainerTypeInit,
			Resources:     convertK8sResourcesToType(c.Resources),
		})
	}

	// 主容器
	for _, c := range spec.Containers {
		response.Containers = append(response.Containers, types.ContainerResources{
			ContainerName: c.Name,
			ContainerType: types.ContainerTypeMain,
			Resources:     convertK8sResourcesToType(c.Resources),
		})
	}

	// Ephemeral 容器
	for _, c := range spec.EphemeralContainers {
		response.Containers = append(response.Containers, types.ContainerResources{
			ContainerName: c.Name,
			ContainerType: types.ContainerTypeEphemeral,
			Resources:     convertK8sResourcesToType(c.Resources),
		})
	}

	return response
}

// ApplyResourcesToPodSpec 应用资源配额更新到 PodSpec
// 适用于：Deployment, DaemonSet, StatefulSet, CronJob
func ApplyResourcesToPodSpec(spec *corev1.PodSpec, containers []types.ContainerResources) error {
	if spec == nil {
		return fmt.Errorf("PodSpec 不能为空")
	}

	resourcesMap := make(map[string]types.ResourceRequirements)
	containerTypeMap := make(map[string]types.ContainerType)
	for _, c := range containers {
		resourcesMap[c.ContainerName] = c.Resources
		containerTypeMap[c.ContainerName] = c.ContainerType
	}

	// 更新 Init 容器
	for i := range spec.InitContainers {
		name := spec.InitContainers[i].Name
		if resources, exists := resourcesMap[name]; exists {
			if containerTypeMap[name] != "" && containerTypeMap[name] != types.ContainerTypeInit {
				return fmt.Errorf("容器 '%s' 类型不匹配，期望 init，实际 %s", name, containerTypeMap[name])
			}
			k8sRes, err := convertTypeToK8sResources(resources)
			if err != nil {
				return fmt.Errorf("容器 '%s' 资源配置无效: %v", name, err)
			}
			spec.InitContainers[i].Resources = k8sRes
		}
	}

	// 更新主容器
	for i := range spec.Containers {
		name := spec.Containers[i].Name
		if resources, exists := resourcesMap[name]; exists {
			if containerTypeMap[name] != "" && containerTypeMap[name] != types.ContainerTypeMain {
				return fmt.Errorf("容器 '%s' 类型不匹配，期望 main，实际 %s", name, containerTypeMap[name])
			}
			k8sRes, err := convertTypeToK8sResources(resources)
			if err != nil {
				return fmt.Errorf("容器 '%s' 资源配置无效: %v", name, err)
			}
			spec.Containers[i].Resources = k8sRes
		}
	}

	return nil
}

// convertK8sResourcesToType K8s 资源需求 -> 自定义类型
func convertK8sResourcesToType(k8sRes corev1.ResourceRequirements) types.ResourceRequirements {
	result := types.ResourceRequirements{
		Limits:   types.ResourceList{},
		Requests: types.ResourceList{},
	}

	if k8sRes.Limits != nil {
		if cpu := k8sRes.Limits.Cpu(); cpu != nil && !cpu.IsZero() {
			result.Limits.Cpu = cpu.String()
		}
		if mem := k8sRes.Limits.Memory(); mem != nil && !mem.IsZero() {
			result.Limits.Memory = mem.String()
		}
	}

	if k8sRes.Requests != nil {
		if cpu := k8sRes.Requests.Cpu(); cpu != nil && !cpu.IsZero() {
			result.Requests.Cpu = cpu.String()
		}
		if mem := k8sRes.Requests.Memory(); mem != nil && !mem.IsZero() {
			result.Requests.Memory = mem.String()
		}
	}

	return result
}

// convertTypeToK8sResources 自定义类型 -> K8s 资源需求
func convertTypeToK8sResources(res types.ResourceRequirements) (corev1.ResourceRequirements, error) {
	k8sRes := corev1.ResourceRequirements{
		Limits:   corev1.ResourceList{},
		Requests: corev1.ResourceList{},
	}

	if res.Limits.Cpu != "" {
		cpuLimit, err := resource.ParseQuantity(res.Limits.Cpu)
		if err != nil {
			return k8sRes, fmt.Errorf("解析 CPU 限制失败: %v", err)
		}
		k8sRes.Limits[corev1.ResourceCPU] = cpuLimit
	}

	if res.Limits.Memory != "" {
		memLimit, err := resource.ParseQuantity(res.Limits.Memory)
		if err != nil {
			return k8sRes, fmt.Errorf("解析内存限制失败: %v", err)
		}
		k8sRes.Limits[corev1.ResourceMemory] = memLimit
	}

	if res.Requests.Cpu != "" {
		cpuReq, err := resource.ParseQuantity(res.Requests.Cpu)
		if err != nil {
			return k8sRes, fmt.Errorf("解析 CPU 请求失败: %v", err)
		}
		k8sRes.Requests[corev1.ResourceCPU] = cpuReq
	}

	if res.Requests.Memory != "" {
		memReq, err := resource.ParseQuantity(res.Requests.Memory)
		if err != nil {
			return k8sRes, fmt.Errorf("解析内存请求失败: %v", err)
		}
		k8sRes.Requests[corev1.ResourceMemory] = memReq
	}

	return k8sRes, nil
}

// ============================================================================
// 健康检查转换
// ============================================================================

// ConvertPodSpecToProbes 从 PodSpec 提取健康检查配置
// 适用于：Deployment, DaemonSet, StatefulSet, CronJob
func ConvertPodSpecToProbes(spec *corev1.PodSpec) *types.ProbesResponse {
	if spec == nil {
		return &types.ProbesResponse{Containers: []types.ContainerProbes{}}
	}

	response := &types.ProbesResponse{
		Containers: make([]types.ContainerProbes, 0),
	}

	// Init 容器
	for _, c := range spec.InitContainers {
		cp := types.ContainerProbes{
			ContainerName: c.Name,
			ContainerType: types.ContainerTypeInit,
		}
		if c.LivenessProbe != nil {
			cp.LivenessProbe = convertK8sProbeToType(c.LivenessProbe)
		}
		if c.ReadinessProbe != nil {
			cp.ReadinessProbe = convertK8sProbeToType(c.ReadinessProbe)
		}
		if c.StartupProbe != nil {
			cp.StartupProbe = convertK8sProbeToType(c.StartupProbe)
		}
		response.Containers = append(response.Containers, cp)
	}

	// 主容器
	for _, c := range spec.Containers {
		cp := types.ContainerProbes{
			ContainerName: c.Name,
			ContainerType: types.ContainerTypeMain,
		}
		if c.LivenessProbe != nil {
			cp.LivenessProbe = convertK8sProbeToType(c.LivenessProbe)
		}
		if c.ReadinessProbe != nil {
			cp.ReadinessProbe = convertK8sProbeToType(c.ReadinessProbe)
		}
		if c.StartupProbe != nil {
			cp.StartupProbe = convertK8sProbeToType(c.StartupProbe)
		}
		response.Containers = append(response.Containers, cp)
	}

	// Ephemeral 容器
	for _, c := range spec.EphemeralContainers {
		cp := types.ContainerProbes{
			ContainerName: c.Name,
			ContainerType: types.ContainerTypeEphemeral,
		}
		if c.LivenessProbe != nil {
			cp.LivenessProbe = convertK8sProbeToType(c.LivenessProbe)
		}
		if c.ReadinessProbe != nil {
			cp.ReadinessProbe = convertK8sProbeToType(c.ReadinessProbe)
		}
		if c.StartupProbe != nil {
			cp.StartupProbe = convertK8sProbeToType(c.StartupProbe)
		}
		response.Containers = append(response.Containers, cp)
	}

	return response
}

// ApplyProbesToPodSpec 应用健康检查更新到 PodSpec
// 适用于：Deployment, DaemonSet, StatefulSet, CronJob
func ApplyProbesToPodSpec(spec *corev1.PodSpec, containers []types.ContainerProbes) error {
	if spec == nil {
		return fmt.Errorf("PodSpec 不能为空")
	}

	probesMap := make(map[string]types.ContainerProbes)
	for _, c := range containers {
		probesMap[c.ContainerName] = c
	}

	// 更新 Init 容器
	for i := range spec.InitContainers {
		name := spec.InitContainers[i].Name
		if probes, exists := probesMap[name]; exists {
			if probes.ContainerType != "" && probes.ContainerType != types.ContainerTypeInit {
				return fmt.Errorf("容器 '%s' 类型不匹配，期望 init，实际 %s", name, probes.ContainerType)
			}
			spec.InitContainers[i].LivenessProbe = convertTypeToK8sProbe(probes.LivenessProbe)
			spec.InitContainers[i].ReadinessProbe = convertTypeToK8sProbe(probes.ReadinessProbe)
			spec.InitContainers[i].StartupProbe = convertTypeToK8sProbe(probes.StartupProbe)
		}
	}

	// 更新主容器
	for i := range spec.Containers {
		name := spec.Containers[i].Name
		if probes, exists := probesMap[name]; exists {
			if probes.ContainerType != "" && probes.ContainerType != types.ContainerTypeMain {
				return fmt.Errorf("容器 '%s' 类型不匹配，期望 main，实际 %s", name, probes.ContainerType)
			}
			spec.Containers[i].LivenessProbe = convertTypeToK8sProbe(probes.LivenessProbe)
			spec.Containers[i].ReadinessProbe = convertTypeToK8sProbe(probes.ReadinessProbe)
			spec.Containers[i].StartupProbe = convertTypeToK8sProbe(probes.StartupProbe)
		}
	}

	return nil
}

// convertK8sProbeToType K8s Probe -> 自定义类型
func convertK8sProbeToType(probe *corev1.Probe) *types.Probe {
	if probe == nil {
		return nil
	}

	result := &types.Probe{
		InitialDelaySeconds: probe.InitialDelaySeconds,
		TimeoutSeconds:      probe.TimeoutSeconds,
		PeriodSeconds:       probe.PeriodSeconds,
		SuccessThreshold:    probe.SuccessThreshold,
		FailureThreshold:    probe.FailureThreshold,
	}

	if probe.HTTPGet != nil {
		result.Type = "httpGet"
		headers := make([]types.HTTPHeader, 0, len(probe.HTTPGet.HTTPHeaders))
		for _, h := range probe.HTTPGet.HTTPHeaders {
			headers = append(headers, types.HTTPHeader{
				Name:  h.Name,
				Value: h.Value,
			})
		}
		result.HttpGet = &types.HTTPGetAction{
			Path:        probe.HTTPGet.Path,
			Port:        probe.HTTPGet.Port.IntVal,
			Host:        probe.HTTPGet.Host,
			Scheme:      string(probe.HTTPGet.Scheme),
			HTTPHeaders: headers,
		}
	} else if probe.TCPSocket != nil {
		result.Type = "tcpSocket"
		result.TcpSocket = &types.TCPSocketAction{
			Port: probe.TCPSocket.Port.IntVal,
			Host: probe.TCPSocket.Host,
		}
	} else if probe.Exec != nil {
		result.Type = "exec"
		result.Exec = &types.ExecAction{
			Command: probe.Exec.Command,
		}
	} else if probe.GRPC != nil {
		result.Type = "grpc"
		service := ""
		if probe.GRPC.Service != nil {
			service = *probe.GRPC.Service
		}
		result.Grpc = &types.GRPCAction{
			Port:    probe.GRPC.Port,
			Service: service,
		}
	}

	return result
}

// convertTypeToK8sProbe 自定义类型 -> K8s Probe
func convertTypeToK8sProbe(probe *types.Probe) *corev1.Probe {
	if probe == nil {
		return nil
	}

	result := &corev1.Probe{
		InitialDelaySeconds: probe.InitialDelaySeconds,
		TimeoutSeconds:      probe.TimeoutSeconds,
		PeriodSeconds:       probe.PeriodSeconds,
		SuccessThreshold:    probe.SuccessThreshold,
		FailureThreshold:    probe.FailureThreshold,
	}

	switch probe.Type {
	case "httpGet":
		if probe.HttpGet != nil {
			headers := make([]corev1.HTTPHeader, 0, len(probe.HttpGet.HTTPHeaders))
			for _, h := range probe.HttpGet.HTTPHeaders {
				headers = append(headers, corev1.HTTPHeader{
					Name:  h.Name,
					Value: h.Value,
				})
			}
			result.HTTPGet = &corev1.HTTPGetAction{
				Path:        probe.HttpGet.Path,
				Port:        intstr.FromInt32(probe.HttpGet.Port),
				Host:        probe.HttpGet.Host,
				Scheme:      corev1.URIScheme(probe.HttpGet.Scheme),
				HTTPHeaders: headers,
			}
		}
	case "tcpSocket":
		if probe.TcpSocket != nil {
			result.TCPSocket = &corev1.TCPSocketAction{
				Port: intstr.FromInt32(probe.TcpSocket.Port),
				Host: probe.TcpSocket.Host,
			}
		}
	case "exec":
		if probe.Exec != nil {
			result.Exec = &corev1.ExecAction{
				Command: probe.Exec.Command,
			}
		}
	case "grpc":
		if probe.Grpc != nil {
			service := probe.Grpc.Service
			result.GRPC = &corev1.GRPCAction{
				Port:    probe.Grpc.Port,
				Service: &service,
			}
		}
	}

	return result
}

// ============================================================================
// 环境变量转换
// ============================================================================

// ConvertPodSpecToEnvVars 从 PodSpec 提取环境变量
// 适用于：Deployment, DaemonSet, StatefulSet, CronJob
func ConvertPodSpecToEnvVars(spec *corev1.PodSpec) *types.EnvVarsResponse {
	if spec == nil {
		return &types.EnvVarsResponse{Containers: []types.ContainerEnvVars{}}
	}

	response := &types.EnvVarsResponse{
		Containers: make([]types.ContainerEnvVars, 0),
	}

	for _, c := range spec.InitContainers {
		response.Containers = append(response.Containers, types.ContainerEnvVars{
			ContainerName: c.Name,
			ContainerType: types.ContainerTypeInit,
			Env:           convertK8sEnvVarsToType(c.Env),
		})
	}

	for _, c := range spec.Containers {
		response.Containers = append(response.Containers, types.ContainerEnvVars{
			ContainerName: c.Name,
			ContainerType: types.ContainerTypeMain,
			Env:           convertK8sEnvVarsToType(c.Env),
		})
	}

	for _, c := range spec.EphemeralContainers {
		response.Containers = append(response.Containers, types.ContainerEnvVars{
			ContainerName: c.Name,
			ContainerType: types.ContainerTypeEphemeral,
			Env:           convertK8sEnvVarsToType(c.Env),
		})
	}

	return response
}

// ApplyEnvVarsToPodSpec 应用环境变量更新到 PodSpec
// 适用于：Deployment, DaemonSet, StatefulSet, CronJob
func ApplyEnvVarsToPodSpec(spec *corev1.PodSpec, containers []types.ContainerEnvVars) error {
	if spec == nil {
		return fmt.Errorf("PodSpec 不能为空")
	}

	envVarsMap := make(map[string][]types.EnvVar)
	containerTypeMap := make(map[string]types.ContainerType)
	for _, c := range containers {
		envVarsMap[c.ContainerName] = c.Env
		containerTypeMap[c.ContainerName] = c.ContainerType
	}

	for i := range spec.InitContainers {
		name := spec.InitContainers[i].Name
		if envVars, exists := envVarsMap[name]; exists {
			if containerTypeMap[name] != "" && containerTypeMap[name] != types.ContainerTypeInit {
				return fmt.Errorf("容器 '%s' 类型不匹配，期望 init，实际 %s", name, containerTypeMap[name])
			}
			spec.InitContainers[i].Env = convertTypeToK8sEnvVars(envVars)
		}
	}

	for i := range spec.Containers {
		name := spec.Containers[i].Name
		if envVars, exists := envVarsMap[name]; exists {
			if containerTypeMap[name] != "" && containerTypeMap[name] != types.ContainerTypeMain {
				return fmt.Errorf("容器 '%s' 类型不匹配，期望 main，实际 %s", name, containerTypeMap[name])
			}
			spec.Containers[i].Env = convertTypeToK8sEnvVars(envVars)
		}
	}

	return nil
}

// convertK8sEnvVarsToType K8s EnvVar -> 自定义类型
func convertK8sEnvVarsToType(k8sEnvVars []corev1.EnvVar) []types.EnvVar {
	envVars := make([]types.EnvVar, 0, len(k8sEnvVars))

	for _, env := range k8sEnvVars {
		envVar := types.EnvVar{
			Name: env.Name,
			Source: types.EnvVarSource{
				Type:  "value",
				Value: env.Value,
			},
		}

		if env.ValueFrom != nil {
			if env.ValueFrom.ConfigMapKeyRef != nil {
				envVar.Source.Type = "configMapKeyRef"
				envVar.Source.ConfigMapKeyRef = &types.ConfigMapKeySelector{
					Name:     env.ValueFrom.ConfigMapKeyRef.Name,
					Key:      env.ValueFrom.ConfigMapKeyRef.Key,
					Optional: env.ValueFrom.ConfigMapKeyRef.Optional != nil && *env.ValueFrom.ConfigMapKeyRef.Optional,
				}
			} else if env.ValueFrom.SecretKeyRef != nil {
				envVar.Source.Type = "secretKeyRef"
				envVar.Source.SecretKeyRef = &types.SecretKeySelector{
					Name:     env.ValueFrom.SecretKeyRef.Name,
					Key:      env.ValueFrom.SecretKeyRef.Key,
					Optional: env.ValueFrom.SecretKeyRef.Optional != nil && *env.ValueFrom.SecretKeyRef.Optional,
				}
			} else if env.ValueFrom.FieldRef != nil {
				envVar.Source.Type = "fieldRef"
				envVar.Source.FieldRef = &types.ObjectFieldSelector{
					FieldPath: env.ValueFrom.FieldRef.FieldPath,
				}
			} else if env.ValueFrom.ResourceFieldRef != nil {
				envVar.Source.Type = "resourceFieldRef"
				divisor := ""
				if !env.ValueFrom.ResourceFieldRef.Divisor.IsZero() {
					divisor = env.ValueFrom.ResourceFieldRef.Divisor.String()
				}
				envVar.Source.ResourceFieldRef = &types.ResourceFieldSelector{
					ContainerName: env.ValueFrom.ResourceFieldRef.ContainerName,
					Resource:      env.ValueFrom.ResourceFieldRef.Resource,
					Divisor:       divisor,
				}
			}
		}

		envVars = append(envVars, envVar)
	}

	return envVars
}

// convertTypeToK8sEnvVars 自定义类型 -> K8s EnvVar
func convertTypeToK8sEnvVars(envVars []types.EnvVar) []corev1.EnvVar {
	k8sEnvVars := make([]corev1.EnvVar, 0, len(envVars))

	for _, env := range envVars {
		k8sEnvVar := corev1.EnvVar{Name: env.Name}

		switch env.Source.Type {
		case "value", "":
			k8sEnvVar.Value = env.Source.Value
		case "configMapKeyRef":
			if env.Source.ConfigMapKeyRef != nil {
				k8sEnvVar.ValueFrom = &corev1.EnvVarSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: env.Source.ConfigMapKeyRef.Name},
						Key:                  env.Source.ConfigMapKeyRef.Key,
						Optional:             &env.Source.ConfigMapKeyRef.Optional,
					},
				}
			}
		case "secretKeyRef":
			if env.Source.SecretKeyRef != nil {
				k8sEnvVar.ValueFrom = &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: env.Source.SecretKeyRef.Name},
						Key:                  env.Source.SecretKeyRef.Key,
						Optional:             &env.Source.SecretKeyRef.Optional,
					},
				}
			}
		case "fieldRef":
			if env.Source.FieldRef != nil {
				k8sEnvVar.ValueFrom = &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{FieldPath: env.Source.FieldRef.FieldPath},
				}
			}
		case "resourceFieldRef":
			if env.Source.ResourceFieldRef != nil {
				resourceFieldRef := &corev1.ResourceFieldSelector{
					ContainerName: env.Source.ResourceFieldRef.ContainerName,
					Resource:      env.Source.ResourceFieldRef.Resource,
				}
				if env.Source.ResourceFieldRef.Divisor != "" {
					if divisor, err := resource.ParseQuantity(env.Source.ResourceFieldRef.Divisor); err == nil {
						resourceFieldRef.Divisor = divisor
					}
				}
				k8sEnvVar.ValueFrom = &corev1.EnvVarSource{ResourceFieldRef: resourceFieldRef}
			}
		}

		k8sEnvVars = append(k8sEnvVars, k8sEnvVar)
	}

	return k8sEnvVars
}

// ============================================================================
// 辅助函数
// ============================================================================

// extractImageTag 提取镜像标签
func extractImageTag(image string) string {
	if strings.Contains(image, "@") {
		parts := strings.Split(image, "@")
		if len(parts) == 2 {
			digest := parts[1]
			if len(digest) > 15 {
				return digest[:15] + "..."
			}
			return digest
		}
	}
	if strings.Contains(image, ":") {
		parts := strings.Split(image, ":")
		return parts[len(parts)-1]
	}
	return "latest"
}

// validateImageFormat 验证镜像格式
func validateImageFormat(image string) error {
	if image == "" {
		return fmt.Errorf("镜像不能为空")
	}
	if strings.HasPrefix(image, ":") || strings.HasPrefix(image, "@") {
		return fmt.Errorf("镜像格式无效")
	}
	return nil
}
