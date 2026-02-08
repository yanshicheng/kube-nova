package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/yanshicheng/kube-nova/common/utils"
	corev1 "k8s.io/api/core/v1"

	"github.com/zeromicro/go-zero/core/logx"
)

type ApplicationServiceUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 服务级 service 更新
func NewApplicationServiceUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApplicationServiceUpdateLogic {
	return &ApplicationServiceUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ApplicationServiceUpdateLogic) ApplicationServiceUpdate(req *types.ApplicationServiceRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}

	// 参数校验
	if req.IsAllSvc && req.IsAppSvc {
		return "", fmt.Errorf("IsAllSvc 和 IsAppSvc 不能同时为 true")
	}

	var selector map[string]string
	var serviceLevel string

	// 根据不同情况确定 selector
	switch {
	case req.IsAllSvc:
		// 全部版本
		selector, err = l.getAppLabels(req.ApplicationId)
		if err != nil {
			return "", fmt.Errorf("获取应用共同 labels 失败: %w", err)
		}
		serviceLevel = "全部版本"

	case req.IsAppSvc:
		// 应用级别
		if len(req.Labels) == 0 {
			return "", fmt.Errorf("应用级 Service 必须提供 labels")
		}
		selector = req.Labels
		serviceLevel = "应用级别"

	default:
		// 版本级别
		if len(req.Selector) == 0 {
			return "", fmt.Errorf("版本级 Service 必须提供 selector")
		}
		selector = req.Selector
		serviceLevel = "版本级别"
	}

	// 将确定的 selector 赋值回 req
	req.Selector = selector

	// 获取工作空间信息
	workspace, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{
		Id: req.WorkloadId,
	})
	if err != nil {
		return "", fmt.Errorf("获取工作空间失败: %w", err)
	}

	// 获取 K8s 客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workspace.Data.ClusterUuid)
	if err != nil {
		return "", fmt.Errorf("获取集群客户端失败: %w", err)
	}

	serviceOperator := client.Services()

	// 获取现有的 Service
	existingService, err := serviceOperator.Get(workspace.Data.Namespace, req.Name)
	if err != nil {
		return "", fmt.Errorf("获取现有 Service 失败: %w，请确认 Service 是否存在", err)
	}

	// 提取现有 Service 的端口信息用于对比
	oldPorts := l.extractServicePorts(existingService)

	// 构建新的 Service 对象
	createLogic := NewApplicationServiceLogic(l.ctx, l.svcCtx)
	updatedService := createLogic.buildK8sService(req, workspace.Data.Namespace)

	// 保留必要的元数据
	updatedService.ResourceVersion = existingService.ResourceVersion
	updatedService.UID = existingService.UID
	updatedService.CreationTimestamp = existingService.CreationTimestamp

	// 对于某些不可变字段保留原值
	if existingService.Spec.ClusterIP != "" && existingService.Spec.ClusterIP != "None" {
		if updatedService.Spec.ClusterIP == "" {
			updatedService.Spec.ClusterIP = existingService.Spec.ClusterIP
		}
	}

	// 获取应用信息用于审计
	app, err := l.svcCtx.ManagerRpc.ApplicationGetById(l.ctx, &managerservice.GetOnecProjectApplicationByIdReq{
		Id: req.ApplicationId,
	})
	if err != nil {
		return "", fmt.Errorf("获取应用失败: %w", err)
	}

	// 对比变更
	var changeDetails []string

	// 类型变更
	if string(existingService.Spec.Type) != req.Type {
		changeDetails = append(changeDetails, fmt.Sprintf("类型: %s → %s", existingService.Spec.Type, req.Type))
	}

	// 端口变更
	portDiff := CompareServicePorts(oldPorts, req.Ports)
	portChangeDetail := BuildPortDiffDetail(portDiff)
	if portChangeDetail != "端口无变更" && portChangeDetail != "" {
		changeDetails = append(changeDetails, portChangeDetail)
	}

	// 选择器变更
	selectorDiff := CompareStringMaps(existingService.Spec.Selector, req.Selector)
	if HasMapChanges(selectorDiff) {
		selectorChangeDetail := BuildMapDiffDetail(selectorDiff, true)
		changeDetails = append(changeDetails, fmt.Sprintf("选择器变更: %s", selectorChangeDetail))
	}

	// Labels 变更
	labelsDiff := CompareStringMaps(existingService.Labels, req.Labels)
	if HasMapChanges(labelsDiff) {
		labelsChangeDetail := BuildMapDiffDetail(labelsDiff, false)
		changeDetails = append(changeDetails, fmt.Sprintf("Labels变更: %s", labelsChangeDetail))
	}

	changeDetailStr := "无变更"
	if len(changeDetails) > 0 {
		changeDetailStr = strings.Join(changeDetails, "; ")
	}

	// 获取项目详情
	projectDetail, err := l.svcCtx.ManagerRpc.GetClusterNsDetail(l.ctx, &managerservice.GetClusterNsDetailReq{
		ClusterUuid: workspace.Data.ClusterUuid,
		Namespace:   workspace.Data.Namespace,
	})
	if err != nil {
		l.Errorf("获取项目详情失败: %v", err)
		return "", fmt.Errorf("获取项目详情失败")
	} else {
		// 注入注解
		utils.AddAnnotations(&updatedService.ObjectMeta, &utils.AnnotationsInfo{
			ServiceName:   updatedService.Name,
			ProjectName:   projectDetail.ProjectNameCn,
			WorkspaceName: projectDetail.WorkspaceNameCn,
			ProjectUuid:   projectDetail.ProjectUuid,
		})
	}

	// 更新 Service
	result, updateErr := serviceOperator.Update(updatedService)
	if updateErr != nil {
		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ApplicationId: req.ApplicationId,
			Title:         "更新应用 Service",
			ActionDetail:  fmt.Sprintf("用户 %s 为应用 %s 更新 Service %s 失败, 级别: %s, 错误原因: %v, 变更内容: %s", username, app.Data.NameCn, req.Name, serviceLevel, updateErr, changeDetailStr),
			Status:        0,
		})
		return "", fmt.Errorf("更新 Service 失败: %w", updateErr)
	}

	// 记录成功的审计日志
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ApplicationId: req.ApplicationId,
		Title:         "更新应用 Service",
		ActionDetail:  fmt.Sprintf("用户 %s 为应用 %s 成功更新 Service %s, 级别: %s, %s", username, app.Data.NameCn, result.Name, serviceLevel, changeDetailStr),
		Status:        1,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	return fmt.Sprintf("Service %s 更新成功", result.Name), nil
}

// extractServicePorts 从 k8s Service 提取端口信息
func (l *ApplicationServiceUpdateLogic) extractServicePorts(svc *corev1.Service) []types.ServicePort {
	var ports []types.ServicePort
	for _, p := range svc.Spec.Ports {
		ports = append(ports, types.ServicePort{
			Name:       p.Name,
			Protocol:   string(p.Protocol),
			Port:       p.Port,
			TargetPort: p.TargetPort.String(),
			NodePort:   p.NodePort,
		})
	}
	return ports
}

// buildPortInfo 构建端口信息字符串用于审计日志
func (l *ApplicationServiceUpdateLogic) buildPortInfo(ports []types.ServicePort) string {
	if len(ports) == 0 {
		return "无"
	}
	var portStrs []string
	for _, p := range ports {
		portStrs = append(portStrs, fmt.Sprintf("%d->%s", p.Port, p.TargetPort))
	}
	return strings.Join(portStrs, ", ")
}

// getAppLabels 获取应用所有版本的共同 app label
func (l *ApplicationServiceUpdateLogic) getAppLabels(appId uint64) (map[string]string, error) {
	createLogic := NewApplicationServiceLogic(l.ctx, l.svcCtx)
	return createLogic.getAppLabels(appId)
}
