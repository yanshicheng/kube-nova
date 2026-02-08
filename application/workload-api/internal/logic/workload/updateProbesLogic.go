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

type UpdateProbesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateProbesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateProbesLogic {
	return &UpdateProbesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateProbesLogic) UpdateProbes(req *types.CommUpdateProbesRequest) (resp string, err error) {
	versionDetail, controller, err := getResourceController(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取资源控制器失败: %v", err)
		return "", err
	}

	resourceType := strings.ToUpper(versionDetail.ResourceType)

	// 获取原健康检查配置
	var oldProbes *k8sTypes.ProbesResponse
	switch resourceType {
	case "DEPLOYMENT":
		oldProbes, err = controller.Deployment.GetProbes(versionDetail.Namespace, versionDetail.ResourceName)
	case "STATEFULSET":
		oldProbes, err = controller.StatefulSet.GetProbes(versionDetail.Namespace, versionDetail.ResourceName)
	case "DAEMONSET":
		oldProbes, err = controller.DaemonSet.GetProbes(versionDetail.Namespace, versionDetail.ResourceName)
	case "CRONJOB":
		oldProbes, err = controller.CronJob.GetProbes(versionDetail.Namespace, versionDetail.ResourceName)
	default:
		return "", fmt.Errorf("资源类型 %s 不支持修改健康检查", resourceType)
	}

	if err != nil {
		l.Errorf("获取原健康检查配置失败: %v", err)
		// 继续执行
	}

	// 转换所有容器的探针配置
	newContainers := make([]k8sTypes.ContainerProbes, 0, len(req.Containers))
	for _, container := range req.Containers {
		containerProbe := k8sTypes.ContainerProbes{
			ContainerName: container.ContainerName,
			ContainerType: k8sTypes.ContainerType(container.ContainerType),
		}

		if container.LivenessProbe != nil {
			containerProbe.LivenessProbe = convertToK8sProbe(container.LivenessProbe)
		}
		if container.ReadinessProbe != nil {
			containerProbe.ReadinessProbe = convertToK8sProbe(container.ReadinessProbe)
		}
		if container.StartupProbe != nil {
			containerProbe.StartupProbe = convertToK8sProbe(container.StartupProbe)
		}

		newContainers = append(newContainers, containerProbe)
	}

	updateReq := &k8sTypes.UpdateProbesRequest{
		Name:       versionDetail.ResourceName,
		Namespace:  versionDetail.Namespace,
		Containers: newContainers,
	}

	// 执行更新
	switch resourceType {
	case "DEPLOYMENT":
		err = controller.Deployment.UpdateProbes(updateReq)
	case "STATEFULSET":
		err = controller.StatefulSet.UpdateProbes(updateReq)
	case "DAEMONSET":
		err = controller.DaemonSet.UpdateProbes(updateReq)
	case "CRONJOB":
		err = controller.CronJob.UpdateProbes(updateReq)
	}

	// 生成变更详情
	changeDetail := l.buildProbesChangeDetail(oldProbes, newContainers)

	// 如果没有变更，不记录日志
	if changeDetail == "" {
		if err != nil {
			l.Errorf("修改健康检查失败: %v", err)
			return "", fmt.Errorf("修改健康检查失败")
		}
		return "健康检查无变更", nil
	}

	if err != nil {
		l.Errorf("修改健康检查失败: %v", err)
		recordAuditLog(l.ctx, l.svcCtx, versionDetail, "修改健康检查",
			fmt.Sprintf("%s %s/%s 修改健康检查失败, %s, 错误: %v", resourceType, versionDetail.Namespace, versionDetail.ResourceName, changeDetail, err), 2)
		return "", fmt.Errorf("修改健康检查失败")
	}

	recordAuditLog(l.ctx, l.svcCtx, versionDetail, "修改健康检查",
		fmt.Sprintf("%s %s/%s 修改健康检查成功, %s", resourceType, versionDetail.Namespace, versionDetail.ResourceName, changeDetail), 1)
	return "修改健康检查成功", nil
}

// buildProbesChangeDetail 构建健康检查变更详情
// 返回空字符串表示没有变更
func (l *UpdateProbesLogic) buildProbesChangeDetail(oldProbes *k8sTypes.ProbesResponse, newContainers []k8sTypes.ContainerProbes) string {
	if oldProbes == nil {
		// 无法获取原配置时，简单列出新配置
		var parts []string
		for _, c := range newContainers {
			probeDesc := l.describeContainerProbes(c)
			if probeDesc != "" {
				parts = append(parts, fmt.Sprintf("容器[%s(%s)]: %s", c.ContainerName, c.ContainerType, probeDesc))
			}
		}
		if len(parts) == 0 {
			return ""
		}
		return fmt.Sprintf("(无法获取原配置) %s", strings.Join(parts, "; "))
	}

	// 构建旧配置的容器映射
	oldContainerMap := make(map[string]k8sTypes.ContainerProbes)
	for _, c := range oldProbes.Containers {
		oldContainerMap[c.ContainerName] = c
	}

	var allChanges []string

	// 遍历新配置，对比变更
	for _, newContainer := range newContainers {
		oldContainer, exists := oldContainerMap[newContainer.ContainerName]

		containerChanges := l.compareContainerProbes(newContainer.ContainerName, string(newContainer.ContainerType), exists, oldContainer, newContainer)
		if containerChanges != "" {
			allChanges = append(allChanges, containerChanges)
		}
	}

	if len(allChanges) == 0 {
		return ""
	}

	return strings.Join(allChanges, "; ")
}

// describeContainerProbes 描述容器的探针配置
func (l *UpdateProbesLogic) describeContainerProbes(c k8sTypes.ContainerProbes) string {
	var probes []string
	if c.LivenessProbe != nil {
		probes = append(probes, fmt.Sprintf("存活探针(%s)", c.LivenessProbe.Type))
	}
	if c.ReadinessProbe != nil {
		probes = append(probes, fmt.Sprintf("就绪探针(%s)", c.ReadinessProbe.Type))
	}
	if c.StartupProbe != nil {
		probes = append(probes, fmt.Sprintf("启动探针(%s)", c.StartupProbe.Type))
	}
	return strings.Join(probes, ", ")
}

// compareContainerProbes 对比单个容器的健康检查变更
func (l *UpdateProbesLogic) compareContainerProbes(containerName, containerType string, oldExists bool, oldProbes, newProbes k8sTypes.ContainerProbes) string {
	var changes []string

	// 对比存活探针
	livenessChange := l.compareProbe("存活探针", oldExists, oldProbes.LivenessProbe, newProbes.LivenessProbe)
	if livenessChange != "" {
		changes = append(changes, livenessChange)
	}

	// 对比就绪探针
	readinessChange := l.compareProbe("就绪探针", oldExists, oldProbes.ReadinessProbe, newProbes.ReadinessProbe)
	if readinessChange != "" {
		changes = append(changes, readinessChange)
	}

	// 对比启动探针
	startupChange := l.compareProbe("启动探针", oldExists, oldProbes.StartupProbe, newProbes.StartupProbe)
	if startupChange != "" {
		changes = append(changes, startupChange)
	}

	if len(changes) == 0 {
		return ""
	}

	return fmt.Sprintf("容器[%s(%s)]: %s", containerName, containerType, strings.Join(changes, ", "))
}

// compareProbe 对比单个探针的变更
func (l *UpdateProbesLogic) compareProbe(probeName string, oldExists bool, oldProbe, newProbe *k8sTypes.Probe) string {
	// 如果新旧都为空，无变更
	if oldProbe == nil && newProbe == nil {
		return ""
	}

	// 新增探针
	if oldProbe == nil && newProbe != nil {
		return fmt.Sprintf("%s: 新增(%s)", probeName, l.formatProbeConfig(newProbe))
	}

	// 删除探针
	if oldProbe != nil && newProbe == nil {
		return fmt.Sprintf("%s: 删除(原为%s)", probeName, oldProbe.Type)
	}

	// 对比探针配置的变更
	probeChanges := l.compareProbeConfig(oldProbe, newProbe)
	if len(probeChanges) == 0 {
		return ""
	}

	return fmt.Sprintf("%s: %s", probeName, strings.Join(probeChanges, ", "))
}

// compareProbeConfig 对比探针配置的具体变更
func (l *UpdateProbesLogic) compareProbeConfig(oldProbe, newProbe *k8sTypes.Probe) []string {
	var changes []string

	// 对比类型
	if oldProbe.Type != newProbe.Type {
		changes = append(changes, fmt.Sprintf("类型: %s -> %s", oldProbe.Type, newProbe.Type))
	}

	// 对比时间参数
	if oldProbe.InitialDelaySeconds != newProbe.InitialDelaySeconds {
		changes = append(changes, fmt.Sprintf("初始延迟: %d -> %d", oldProbe.InitialDelaySeconds, newProbe.InitialDelaySeconds))
	}
	if oldProbe.TimeoutSeconds != newProbe.TimeoutSeconds {
		changes = append(changes, fmt.Sprintf("超时时间: %d -> %d", oldProbe.TimeoutSeconds, newProbe.TimeoutSeconds))
	}
	if oldProbe.PeriodSeconds != newProbe.PeriodSeconds {
		changes = append(changes, fmt.Sprintf("检查周期: %d -> %d", oldProbe.PeriodSeconds, newProbe.PeriodSeconds))
	}
	if oldProbe.SuccessThreshold != newProbe.SuccessThreshold {
		changes = append(changes, fmt.Sprintf("成功阈值: %d -> %d", oldProbe.SuccessThreshold, newProbe.SuccessThreshold))
	}
	if oldProbe.FailureThreshold != newProbe.FailureThreshold {
		changes = append(changes, fmt.Sprintf("失败阈值: %d -> %d", oldProbe.FailureThreshold, newProbe.FailureThreshold))
	}

	// 对比 HTTP 配置
	if newProbe.Type == "httpGet" {
		httpChanges := l.compareHTTPGetAction(oldProbe.HttpGet, newProbe.HttpGet)
		changes = append(changes, httpChanges...)
	}

	// 对比 TCP 配置
	if newProbe.Type == "tcpSocket" {
		tcpChanges := l.compareTCPSocketAction(oldProbe.TcpSocket, newProbe.TcpSocket)
		changes = append(changes, tcpChanges...)
	}

	// 对比 Exec 配置
	if newProbe.Type == "exec" {
		execChanges := l.compareExecAction(oldProbe.Exec, newProbe.Exec)
		changes = append(changes, execChanges...)
	}

	// 对比 gRPC 配置
	if newProbe.Type == "grpc" {
		grpcChanges := l.compareGRPCAction(oldProbe.Grpc, newProbe.Grpc)
		changes = append(changes, grpcChanges...)
	}

	return changes
}

// compareHTTPGetAction 对比 HTTP 探针配置
func (l *UpdateProbesLogic) compareHTTPGetAction(oldAction, newAction *k8sTypes.HTTPGetAction) []string {
	var changes []string

	if oldAction == nil && newAction == nil {
		return changes
	}
	if oldAction == nil {
		oldAction = &k8sTypes.HTTPGetAction{}
	}
	if newAction == nil {
		newAction = &k8sTypes.HTTPGetAction{}
	}

	if oldAction.Path != newAction.Path {
		changes = append(changes, fmt.Sprintf("HTTP路径: %s -> %s", oldAction.Path, newAction.Path))
	}
	if oldAction.Port != newAction.Port {
		changes = append(changes, fmt.Sprintf("HTTP端口: %d -> %d", oldAction.Port, newAction.Port))
	}
	if oldAction.Scheme != newAction.Scheme {
		changes = append(changes, fmt.Sprintf("HTTP协议: %s -> %s", oldAction.Scheme, newAction.Scheme))
	}

	return changes
}

// compareTCPSocketAction 对比 TCP 探针配置
func (l *UpdateProbesLogic) compareTCPSocketAction(oldAction, newAction *k8sTypes.TCPSocketAction) []string {
	var changes []string

	if oldAction == nil && newAction == nil {
		return changes
	}
	if oldAction == nil {
		oldAction = &k8sTypes.TCPSocketAction{}
	}
	if newAction == nil {
		newAction = &k8sTypes.TCPSocketAction{}
	}

	if oldAction.Port != newAction.Port {
		changes = append(changes, fmt.Sprintf("TCP端口: %d -> %d", oldAction.Port, newAction.Port))
	}

	return changes
}

// compareExecAction 对比 Exec 探针配置
func (l *UpdateProbesLogic) compareExecAction(oldAction, newAction *k8sTypes.ExecAction) []string {
	var changes []string

	if oldAction == nil && newAction == nil {
		return changes
	}

	oldCmd := ""
	newCmd := ""
	if oldAction != nil {
		oldCmd = strings.Join(oldAction.Command, " ")
	}
	if newAction != nil {
		newCmd = strings.Join(newAction.Command, " ")
	}

	if oldCmd != newCmd {
		changes = append(changes, fmt.Sprintf("命令: '%s' -> '%s'", oldCmd, newCmd))
	}

	return changes
}

// compareGRPCAction 对比 gRPC 探针配置
func (l *UpdateProbesLogic) compareGRPCAction(oldAction, newAction *k8sTypes.GRPCAction) []string {
	var changes []string

	if oldAction == nil && newAction == nil {
		return changes
	}
	if oldAction == nil {
		oldAction = &k8sTypes.GRPCAction{}
	}
	if newAction == nil {
		newAction = &k8sTypes.GRPCAction{}
	}

	if oldAction.Port != newAction.Port {
		changes = append(changes, fmt.Sprintf("gRPC端口: %d -> %d", oldAction.Port, newAction.Port))
	}
	if oldAction.Service != newAction.Service {
		changes = append(changes, fmt.Sprintf("gRPC服务: %s -> %s", oldAction.Service, newAction.Service))
	}

	return changes
}

// formatProbeConfig 格式化探针配置用于日志显示
func (l *UpdateProbesLogic) formatProbeConfig(probe *k8sTypes.Probe) string {
	if probe == nil {
		return "无"
	}

	var details []string
	details = append(details, probe.Type)

	switch probe.Type {
	case "httpGet":
		if probe.HttpGet != nil {
			details = append(details, fmt.Sprintf("%s:%d%s", probe.HttpGet.Scheme, probe.HttpGet.Port, probe.HttpGet.Path))
		}
	case "tcpSocket":
		if probe.TcpSocket != nil {
			details = append(details, fmt.Sprintf("port:%d", probe.TcpSocket.Port))
		}
	case "exec":
		if probe.Exec != nil && len(probe.Exec.Command) > 0 {
			cmd := strings.Join(probe.Exec.Command, " ")
			if len(cmd) > 30 {
				cmd = cmd[:30] + "..."
			}
			details = append(details, cmd)
		}
	case "grpc":
		if probe.Grpc != nil {
			details = append(details, fmt.Sprintf("port:%d", probe.Grpc.Port))
		}
	}

	return strings.Join(details, " ")
}

// convertToK8sProbe 将 API 类型转换为 k8sTypes.Probe
func convertToK8sProbe(probe *types.CommProbe) *k8sTypes.Probe {
	if probe == nil {
		return nil
	}

	result := &k8sTypes.Probe{
		Type:                probe.Type,
		InitialDelaySeconds: probe.InitialDelaySeconds,
		TimeoutSeconds:      probe.TimeoutSeconds,
		PeriodSeconds:       probe.PeriodSeconds,
		SuccessThreshold:    probe.SuccessThreshold,
		FailureThreshold:    probe.FailureThreshold,
	}

	if probe.HttpGet != nil {
		headers := make([]k8sTypes.HTTPHeader, 0, len(probe.HttpGet.HTTPHeaders))
		for _, h := range probe.HttpGet.HTTPHeaders {
			headers = append(headers, k8sTypes.HTTPHeader{
				Name:  h.Name,
				Value: h.Value,
			})
		}

		result.HttpGet = &k8sTypes.HTTPGetAction{
			Path:        probe.HttpGet.Path,
			Port:        probe.HttpGet.Port,
			Host:        probe.HttpGet.Host,
			Scheme:      probe.HttpGet.Scheme,
			HTTPHeaders: headers,
		}
	}

	if probe.TcpSocket != nil {
		result.TcpSocket = &k8sTypes.TCPSocketAction{
			Port: probe.TcpSocket.Port,
			Host: probe.TcpSocket.Host,
		}
	}

	if probe.Exec != nil {
		result.Exec = &k8sTypes.ExecAction{
			Command: probe.Exec.Command,
		}
	}

	if probe.Grpc != nil {
		result.Grpc = &k8sTypes.GRPCAction{
			Port:    probe.Grpc.Port,
			Service: probe.Grpc.Service,
		}
	}

	return result
}
