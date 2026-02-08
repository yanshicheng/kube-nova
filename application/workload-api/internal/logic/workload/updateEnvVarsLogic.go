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

type UpdateEnvVarsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateEnvVarsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateEnvVarsLogic {
	return &UpdateEnvVarsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateEnvVarsLogic) UpdateEnvVars(req *types.UpdateEnvVarsRequest) (resp string, err error) {
	versionDetail, controller, err := getResourceController(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取资源控制器失败: %v", err)
		return "", err
	}

	resourceType := strings.ToUpper(versionDetail.ResourceType)

	// 获取原环境变量配置
	var oldEnvVars *k8sTypes.EnvVarsResponse
	switch resourceType {
	case "DEPLOYMENT":
		oldEnvVars, err = controller.Deployment.GetEnvVars(versionDetail.Namespace, versionDetail.ResourceName)
	case "STATEFULSET":
		oldEnvVars, err = controller.StatefulSet.GetEnvVars(versionDetail.Namespace, versionDetail.ResourceName)
	case "DAEMONSET":
		oldEnvVars, err = controller.DaemonSet.GetEnvVars(versionDetail.Namespace, versionDetail.ResourceName)
	case "JOB":
		oldEnvVars, err = controller.Job.GetEnvVars(versionDetail.Namespace, versionDetail.ResourceName)
	case "CRONJOB":
		oldEnvVars, err = controller.CronJob.GetEnvVars(versionDetail.Namespace, versionDetail.ResourceName)
	default:
		return "", fmt.Errorf("资源类型 %s 不支持修改环境变量", resourceType)
	}

	if err != nil {
		l.Errorf("获取原环境变量配置失败: %v", err)
		// 继续执行，审计日志中记录无法获取原配置
	}

	// 构建新的环境变量列表（转换所有容器）
	newContainers := make([]k8sTypes.ContainerEnvVars, 0, len(req.Containers))
	for _, container := range req.Containers {
		newEnv := make([]k8sTypes.EnvVar, 0, len(container.Env))
		for _, e := range container.Env {
			newEnv = append(newEnv, k8sTypes.EnvVar{
				Name:   e.Name,
				Source: convertToK8sEnvVarSource(e.Source),
			})
		}
		newContainers = append(newContainers, k8sTypes.ContainerEnvVars{
			ContainerName: container.ContainerName,
			ContainerType: k8sTypes.ContainerType(container.ContainerType),
			Env:           newEnv,
		})
	}

	updateReq := &k8sTypes.UpdateEnvVarsRequest{
		Name:       versionDetail.ResourceName,
		Namespace:  versionDetail.Namespace,
		Containers: newContainers,
	}

	// 执行更新
	switch resourceType {
	case "DEPLOYMENT":
		err = controller.Deployment.UpdateEnvVars(updateReq)
	case "STATEFULSET":
		err = controller.StatefulSet.UpdateEnvVars(updateReq)
	case "DAEMONSET":
		err = controller.DaemonSet.UpdateEnvVars(updateReq)
	case "CRONJOB":
		err = controller.CronJob.UpdateEnvVars(updateReq)
	}

	// 生成变更详情
	changeDetail := l.buildEnvVarsChangeDetail(oldEnvVars, newContainers)

	// 如果没有变更，不记录日志
	if changeDetail == "" {
		if err != nil {
			l.Errorf("修改环境变量失败: %v", err)
			return "", fmt.Errorf("修改环境变量失败")
		}
		return "环境变量无变更", nil
	}

	if err != nil {
		l.Errorf("修改环境变量失败: %v", err)
		recordAuditLog(l.ctx, l.svcCtx, versionDetail, "修改环境变量",
			fmt.Sprintf("%s %s/%s 修改环境变量失败, %s, 错误: %v", resourceType, versionDetail.Namespace, versionDetail.ResourceName, changeDetail, err), 2)
		return "", fmt.Errorf("修改环境变量失败")
	}

	recordAuditLog(l.ctx, l.svcCtx, versionDetail, "修改环境变量",
		fmt.Sprintf("%s %s/%s 修改环境变量成功, %s", resourceType, versionDetail.Namespace, versionDetail.ResourceName, changeDetail), 1)
	return "修改环境变量成功", nil
}

// buildEnvVarsChangeDetail 构建环境变量变更详情
// 返回空字符串表示没有变更
func (l *UpdateEnvVarsLogic) buildEnvVarsChangeDetail(oldEnvVars *k8sTypes.EnvVarsResponse, newContainers []k8sTypes.ContainerEnvVars) string {
	if oldEnvVars == nil {
		// 无法获取原配置时，简单列出新配置
		var parts []string
		for _, c := range newContainers {
			parts = append(parts, fmt.Sprintf("容器[%s(%s)]环境变量数量: %d", c.ContainerName, c.ContainerType, len(c.Env)))
		}
		if len(parts) == 0 {
			return ""
		}
		return fmt.Sprintf("(无法获取原配置) %s", strings.Join(parts, "; "))
	}

	// 构建旧配置的容器映射
	oldContainerMap := make(map[string]k8sTypes.ContainerEnvVars)
	for _, c := range oldEnvVars.Containers {
		oldContainerMap[c.ContainerName] = c
	}

	var allChanges []string

	// 遍历新配置，对比变更
	for _, newContainer := range newContainers {
		oldContainer, exists := oldContainerMap[newContainer.ContainerName]

		containerChanges := l.compareContainerEnvVars(newContainer.ContainerName, string(newContainer.ContainerType), exists, oldContainer.Env, newContainer.Env)
		if containerChanges != "" {
			allChanges = append(allChanges, containerChanges)
		}
	}

	if len(allChanges) == 0 {
		return ""
	}

	return strings.Join(allChanges, "; ")
}

// compareContainerEnvVars 对比单个容器的环境变量变更
func (l *UpdateEnvVarsLogic) compareContainerEnvVars(containerName, containerType string, oldExists bool, oldEnv, newEnv []k8sTypes.EnvVar) string {
	if !oldExists {
		if len(newEnv) == 0 {
			return ""
		}
		var envNames []string
		for _, e := range newEnv {
			envNames = append(envNames, e.Name)
		}
		return fmt.Sprintf("容器[%s(%s)]新增环境变量: %s", containerName, containerType, strings.Join(envNames, ", "))
	}

	// 构建旧环境变量映射
	oldEnvMap := make(map[string]k8sTypes.EnvVar)
	for _, e := range oldEnv {
		oldEnvMap[e.Name] = e
	}

	// 构建新环境变量映射
	newEnvMap := make(map[string]k8sTypes.EnvVar)
	for _, e := range newEnv {
		newEnvMap[e.Name] = e
	}

	var changes []string

	// 检查新增和修改的环境变量
	for _, newE := range newEnv {
		oldE, exists := oldEnvMap[newE.Name]
		if !exists {
			changes = append(changes, fmt.Sprintf("新增[%s]=%s", newE.Name, formatEnvVarValue(newE.Source)))
		} else if !envVarSourceEqual(oldE.Source, newE.Source) {
			changes = append(changes, fmt.Sprintf("修改[%s]: %s -> %s", newE.Name, formatEnvVarValue(oldE.Source), formatEnvVarValue(newE.Source)))
		}
	}

	// 检查删除的环境变量
	for _, oldE := range oldEnv {
		if _, exists := newEnvMap[oldE.Name]; !exists {
			changes = append(changes, fmt.Sprintf("删除[%s]", oldE.Name))
		}
	}

	if len(changes) == 0 {
		return ""
	}

	return fmt.Sprintf("容器[%s(%s)]: %s", containerName, containerType, strings.Join(changes, ", "))
}

// formatEnvVarValue 格式化环境变量值用于日志显示
func formatEnvVarValue(source k8sTypes.EnvVarSource) string {
	switch source.Type {
	case "value":
		// 值过长时截断
		if len(source.Value) > 50 {
			return fmt.Sprintf("'%s...'", source.Value[:50])
		}
		return fmt.Sprintf("'%s'", source.Value)
	case "configMapKeyRef":
		if source.ConfigMapKeyRef != nil {
			return fmt.Sprintf("ConfigMap(%s/%s)", source.ConfigMapKeyRef.Name, source.ConfigMapKeyRef.Key)
		}
	case "secretKeyRef":
		if source.SecretKeyRef != nil {
			return fmt.Sprintf("Secret(%s/%s)", source.SecretKeyRef.Name, source.SecretKeyRef.Key)
		}
	case "fieldRef":
		if source.FieldRef != nil {
			return fmt.Sprintf("FieldRef(%s)", source.FieldRef.FieldPath)
		}
	case "resourceFieldRef":
		if source.ResourceFieldRef != nil {
			return fmt.Sprintf("ResourceFieldRef(%s)", source.ResourceFieldRef.Resource)
		}
	}
	return source.Type
}

// envVarSourceEqual 判断两个环境变量来源是否相等
func envVarSourceEqual(a, b k8sTypes.EnvVarSource) bool {
	if a.Type != b.Type {
		return false
	}

	switch a.Type {
	case "value":
		return a.Value == b.Value
	case "configMapKeyRef":
		if a.ConfigMapKeyRef == nil && b.ConfigMapKeyRef == nil {
			return true
		}
		if a.ConfigMapKeyRef == nil || b.ConfigMapKeyRef == nil {
			return false
		}
		return a.ConfigMapKeyRef.Name == b.ConfigMapKeyRef.Name &&
			a.ConfigMapKeyRef.Key == b.ConfigMapKeyRef.Key &&
			a.ConfigMapKeyRef.Optional == b.ConfigMapKeyRef.Optional
	case "secretKeyRef":
		if a.SecretKeyRef == nil && b.SecretKeyRef == nil {
			return true
		}
		if a.SecretKeyRef == nil || b.SecretKeyRef == nil {
			return false
		}
		return a.SecretKeyRef.Name == b.SecretKeyRef.Name &&
			a.SecretKeyRef.Key == b.SecretKeyRef.Key &&
			a.SecretKeyRef.Optional == b.SecretKeyRef.Optional
	case "fieldRef":
		if a.FieldRef == nil && b.FieldRef == nil {
			return true
		}
		if a.FieldRef == nil || b.FieldRef == nil {
			return false
		}
		return a.FieldRef.FieldPath == b.FieldRef.FieldPath
	case "resourceFieldRef":
		if a.ResourceFieldRef == nil && b.ResourceFieldRef == nil {
			return true
		}
		if a.ResourceFieldRef == nil || b.ResourceFieldRef == nil {
			return false
		}
		return a.ResourceFieldRef.ContainerName == b.ResourceFieldRef.ContainerName &&
			a.ResourceFieldRef.Resource == b.ResourceFieldRef.Resource &&
			a.ResourceFieldRef.Divisor == b.ResourceFieldRef.Divisor
	}

	return true
}

// convertToK8sEnvVarSource 将 API 类型转换为 k8sTypes.EnvVarSource
func convertToK8sEnvVarSource(source types.EnvVarSource) k8sTypes.EnvVarSource {
	result := k8sTypes.EnvVarSource{
		Type:  source.Type,
		Value: source.Value,
	}

	if source.ConfigMapKeyRef != nil {
		result.ConfigMapKeyRef = &k8sTypes.ConfigMapKeySelector{
			Name:     source.ConfigMapKeyRef.Name,
			Key:      source.ConfigMapKeyRef.Key,
			Optional: source.ConfigMapKeyRef.Optional,
		}
	}

	if source.SecretKeyRef != nil {
		result.SecretKeyRef = &k8sTypes.SecretKeySelector{
			Name:     source.SecretKeyRef.Name,
			Key:      source.SecretKeyRef.Key,
			Optional: source.SecretKeyRef.Optional,
		}
	}

	if source.FieldRef != nil {
		result.FieldRef = &k8sTypes.ObjectFieldSelector{
			FieldPath: source.FieldRef.FieldPath,
		}
	}

	if source.ResourceFieldRef != nil {
		result.ResourceFieldRef = &k8sTypes.ResourceFieldSelector{
			ContainerName: source.ResourceFieldRef.ContainerName,
			Resource:      source.ResourceFieldRef.Resource,
			Divisor:       source.ResourceFieldRef.Divisor,
		}
	}

	return result
}
