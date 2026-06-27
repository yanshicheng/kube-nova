package channelvars

import "strings"

// StandardVariableFields 返回内置渠道类型的标准变量规格。
func StandardVariableFields(groupCode, channelType string) []VariableField {
	groupCode = strings.TrimSpace(groupCode)
	channelType = strings.TrimSpace(channelType)
	var fields []VariableField
	switch groupCode {
	case GroupCodeRepo:
		fields = gitRepositoryVariables()
	case GroupImageRepo:
		fields = imageRegistryVariables()
	case GroupArtifactRepo:
		fields = artifactRepositoryVariables()
	case GroupCodeScan:
		if channelType == "sonarqube" {
			fields = sonarVariables()
		}
	case GroupBuildEngine:
		switch channelType {
		case "jenkins":
			fields = jenkinsVariables()
		case "tekton":
			fields = tektonVariables()
		}
	case GroupDeployTarget:
		switch channelType {
		case "kubernetes":
			fields = kubernetesVariables()
		case "host":
			fields = hostVariables()
		case "host_group":
			fields = hostGroupVariables()
		case "kube-nova":
			fields = kubeNovaVariables()
		}
	}
	return MergeVariableFields(withVariableContext(groupCode, channelType, fields))
}

func DynamicVariable(field, name, provider string, deps ...VariableDependency) VariableField {
	return VariableField{
		Field:            field,
		Name:             name,
		Kind:             "dynamic",
		UIControl:        "select",
		Provider:         provider,
		Dependencies:     deps,
		AllowManualInput: true,
		ResolveEndpoint:  hasEndpointDependency(deps),
	}
}

func AddressVariable(field, name, provider, builder string, fallback []string, deps ...VariableDependency) VariableField {
	return VariableField{
		Field:                 field,
		Name:                  name,
		Kind:                  "address",
		UIControl:             "cascadePicker",
		Provider:              provider,
		Dependencies:          addressEndpointDependencies(deps),
		AllowManualInput:      true,
		AddressBuilder:        builder,
		OutputMode:            "provider",
		FallbackAddressFields: fallback,
		ResolveEndpoint:       true,
	}
}

func OptionVariable(field, name string, options ...VariableOption) VariableField {
	return VariableField{
		Field:            field,
		Name:             name,
		Kind:             "option",
		UIControl:        "radio",
		Options:          options,
		AllowManualInput: true,
	}
}

func OutputVariable(field, name, provider string, deps ...VariableDependency) VariableField {
	return VariableField{
		Field:            field,
		Name:             name,
		Kind:             "output",
		UIControl:        "output",
		Provider:         provider,
		Dependencies:     deps,
		AllowManualInput: true,
		OutputMode:       "provider",
		ResolveEndpoint:  hasEndpointDependency(deps),
	}
}

func CompositeVariable(field, name, provider string, deps ...VariableDependency) VariableField {
	return VariableField{
		Field:            field,
		Name:             name,
		Kind:             "composite",
		UIControl:        "deployConfig",
		Provider:         provider,
		Dependencies:     deps,
		AllowManualInput: true,
		ResolveEndpoint:  hasEndpointDependency(deps),
	}
}

func DepChannel() VariableDependency {
	return VariableDependency{Field: FieldEndpoint, Source: "channel", Name: "渠道实例", Required: true}
}

func DepParam(field, name string, required bool) VariableDependency {
	return VariableDependency{Field: field, Source: "param", Name: name, Required: required}
}

func hasEndpointDependency(deps []VariableDependency) bool {
	for _, dep := range deps {
		if strings.TrimSpace(dep.Source) == "channel" && strings.TrimSpace(dep.Field) == FieldEndpoint {
			return true
		}
	}
	return false
}

func gitRepositoryVariables() []VariableField {
	return []VariableField{
		DynamicVariable(FieldDynamicProject, "仓库项目", "git.project", DepChannel()).withFallback(FieldAddressProjectURL),
		DynamicVariable(FieldDynamicBranch, "分支", "git.branch", DepChannel(), DepParam(FieldDynamicProject, "仓库项目", true)).withFallback(FieldAddressProjectURL),
		DynamicVariable(FieldDynamicTag, "Tag", "git.tag", DepChannel(), DepParam(FieldDynamicProject, "仓库项目", true)).withFallback(FieldAddressProjectURL),
		AddressVariable(FieldAddressProjectURL, "仓库地址", "git.projectUrl", "channelType + endpoint + project + protocol", nil, DepChannel()),
	}
}

func imageRegistryVariables() []VariableField {
	fallbacks := []string{FieldAddressProjectURL, FieldAddressImageURL, FieldAddressImageTagURL}
	return []VariableField{
		DynamicVariable(FieldDynamicProject, "镜像项目", "image.project", DepChannel()).withFallback(fallbacks...),
		DynamicVariable(FieldDynamicImage, "镜像", "image.image", DepChannel(), DepParam(FieldDynamicProject, "镜像项目", true)).withFallback(fallbacks...),
		DynamicVariable(FieldDynamicTag, "镜像标签", "image.tag", DepChannel(), DepParam(FieldDynamicProject, "镜像项目", true), DepParam(FieldDynamicImage, "镜像", true)).withFallback(FieldAddressImageURL, FieldAddressImageTagURL),
		DynamicVariable(FieldDynamicImageTag, "镜像加标签", "image.imageTag", DepChannel(), DepParam(FieldDynamicProject, "镜像项目", true)).withFallback(fallbacks...),
		AddressVariable(FieldAddressProjectURL, "镜像项目地址", "image.projectUrl", "endpoint + project", nil, DepChannel()),
		AddressVariable(FieldAddressImageURL, "镜像地址", "image.imageUrl", "endpoint + project + image", nil, DepChannel()),
		AddressVariable(FieldAddressImageTagURL, "镜像标签地址", "image.imageTagUrl", "endpoint + project + image + tag", nil, DepChannel()),
	}
}

func artifactRepositoryVariables() []VariableField {
	fallbacks := []string{FieldAddressRepositoryURL, FieldAddressArtifactURL, FieldAddressArtifactVersionURL}
	return []VariableField{
		DynamicVariable(FieldDynamicRepository, "制品仓库", "artifact.repository", DepChannel()).withFallback(fallbacks...),
		DynamicVariable(FieldDynamicArtifactName, "制品名称", "artifact.artifactName", DepChannel(), DepParam(FieldDynamicRepository, "制品仓库", true)).withFallback(fallbacks...),
		DynamicVariable(FieldDynamicArtifactVersion, "制品版本", "artifact.artifactVersion", DepChannel(), DepParam(FieldDynamicRepository, "制品仓库", true), DepParam(FieldDynamicArtifactName, "制品名称", true)).withFallback(FieldAddressArtifactURL, FieldAddressArtifactVersionURL),
		AddressVariable(FieldAddressRepositoryURL, "制品仓库地址", "artifact.repositoryUrl", "endpoint + repository", nil, DepChannel()),
		AddressVariable(FieldAddressArtifactURL, "制品地址", "artifact.artifactUrl", "endpoint + repository + artifact", nil, DepChannel()),
		AddressVariable(FieldAddressArtifactVersionURL, "制品版本地址", "artifact.artifactVersionUrl", "endpoint + repository + artifact + version", nil, DepChannel()),
	}
}

func sonarVariables() []VariableField {
	return []VariableField{
		DynamicVariable(FieldDynamicProjectName, "项目名称", "scan.projectName", DepChannel()),
		DynamicVariable(FieldDynamicProjectKey, "项目 Key", "scan.projectKey", DepChannel()),
	}
}

func kubernetesVariables() []VariableField {
	return []VariableField{
		DynamicVariable(FieldDynamicNamespace, "命名空间", "kubernetes.namespace", DepChannel()),
		OptionVariable(FieldDynamicWorkloadType, "部署类型",
			VariableOption{Label: "Deployment", Value: "deployment"},
			VariableOption{Label: "StatefulSet", Value: "statefulset"},
			VariableOption{Label: "DaemonSet", Value: "daemonset"},
			VariableOption{Label: "CronJob", Value: "cronjob"},
		),
		DynamicVariable(FieldDynamicResourceName, "资源名称", "kubernetes.resourceName", DepChannel(), DepParam(FieldDynamicNamespace, "命名空间", true), DepParam(FieldDynamicWorkloadType, "部署类型", true)),
		DynamicVariable(FieldDynamicContainerName, "容器名称", "kubernetes.containerName", DepChannel(), DepParam(FieldDynamicNamespace, "命名空间", true), DepParam(FieldDynamicWorkloadType, "部署类型", true), DepParam(FieldDynamicResourceName, "资源名称", true)),
		DynamicVariable(FieldDynamicImageName, "镜像名称", "kubernetes.imageName", DepChannel(), DepParam(FieldDynamicNamespace, "命名空间", true), DepParam(FieldDynamicWorkloadType, "部署类型", true), DepParam(FieldDynamicResourceName, "资源名称", true), DepParam(FieldDynamicContainerName, "容器名称", true)),
		CompositeVariable(FieldAddressDeploymentConfig, "Kubernetes 部署配置", "kubernetes.deploymentConfig", DepChannel()),
	}
}

func hostGroupVariables() []VariableField {
	return []VariableField{
		AddressVariable(FieldAddressGroupConfig, "主机组配置", "hostGroup.groupConfig", "endpoint + hostGroup + outputFormat", nil, DepChannel()),
	}
}

func hostVariables() []VariableField {
	return []VariableField{
		AddressVariable(FieldAddressHostConfig, "主机配置", "host.hostConfig", "endpoint + host + outputFormat", nil, DepChannel()),
	}
}

func kubeNovaVariables() []VariableField {
	return []VariableField{
		CompositeVariable(FieldAddressDeployConfig, "kube-nova 部署配置", "kubeNova.deployConfig", DepChannel()),
	}
}

func jenkinsVariables() []VariableField {
	return []VariableField{
		DynamicVariable("dynamic.folder", "文件夹", "build.folder", DepChannel()),
		DynamicVariable("dynamic.job", "Job", "build.job", DepChannel(), DepParam("dynamic.folder", "文件夹", false)),
		DynamicVariable("dynamic.buildNumber", "构建号", "build.number", DepChannel(), DepParam("dynamic.job", "Job", true)),
	}
}

func tektonVariables() []VariableField {
	return []VariableField{
		DynamicVariable("dynamic.pipeline", "Pipeline", "build.pipeline", DepChannel()),
		DynamicVariable("dynamic.task", "Task", "build.task", DepChannel()),
		DynamicVariable("dynamic.pipelineRun", "PipelineRun", "build.pipelineRun", DepChannel(), DepParam("dynamic.pipeline", "Pipeline", true)),
	}
}

func (field VariableField) withFallback(fallbacks ...string) VariableField {
	field.FallbackAddressFields = append(field.FallbackAddressFields, fallbacks...)
	field.ResolveEndpoint = true
	return field
}

func withVariableContext(groupCode, channelType string, fields []VariableField) []VariableField {
	result := make([]VariableField, 0, len(fields))
	for i, item := range fields {
		item.GroupCode = groupCode
		if channelType != "" {
			item.ChannelTypes = []string{channelType}
		}
		item.SortOrder = int64((i + 1) * 10)
		if item.Kind != "default" {
			item.AllowManualInput = true
		}
		item.AllOf = DependencyAllOf(item)
		item.AnyOf = DependencyAnyOf(item)
		result = append(result, item)
	}
	return result
}
