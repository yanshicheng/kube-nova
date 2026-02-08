package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/yanshicheng/kube-nova/common/utils"
	"github.com/zeromicro/go-zero/core/logx"
	corev1 "k8s.io/api/core/v1"
)

type ServiceUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新 Service
func NewServiceUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceUpdateLogic {
	return &ServiceUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceUpdateLogic) ServiceUpdate(req *types.ServiceRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}

	// 获取集群以及命名空间
	workloadInfo, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{Id: req.WorkloadId})
	if err != nil {
		l.Errorf("获取项目工作空间详情失败: %v", err)
		return "", fmt.Errorf("获取项目工作空间详情失败")
	}

	// 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workloadInfo.Data.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	// 初始化 Service 客户端
	serviceClient := client.Services()

	// 获取现有的 Service
	existingSvc, err := serviceClient.Get(workloadInfo.Data.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取现有 Service 失败: %v", err)
		return "", fmt.Errorf("获取现有 Service 失败")
	}

	// 提取现有 Service 的端口信息用于对比
	oldPorts := l.extractServicePorts(existingSvc)

	// 构建新的 Service 对象
	createLogic := NewServiceCreateLogic(l.ctx, l.svcCtx)
	updatedSvc := createLogic.buildServiceFromRequest(req, workloadInfo.Data.Namespace)

	// 保留必要的元数据
	updatedSvc.ResourceVersion = existingSvc.ResourceVersion
	updatedSvc.UID = existingSvc.UID
	updatedSvc.CreationTimestamp = existingSvc.CreationTimestamp

	// 对于某些不可变字段保留原值
	if existingSvc.Spec.ClusterIP != "" && existingSvc.Spec.ClusterIP != "None" {
		if updatedSvc.Spec.ClusterIP == "" {
			updatedSvc.Spec.ClusterIP = existingSvc.Spec.ClusterIP
		}
	}

	// 对比变更
	var changeDetails []string

	// 类型变更
	if string(existingSvc.Spec.Type) != req.Type {
		changeDetails = append(changeDetails, fmt.Sprintf("类型: %s → %s", existingSvc.Spec.Type, req.Type))
	}

	// 端口变更
	portDiff := CompareServicePorts(oldPorts, req.Ports)
	portChangeDetail := BuildPortDiffDetail(portDiff)
	if portChangeDetail != "端口无变更" && portChangeDetail != "" {
		changeDetails = append(changeDetails, portChangeDetail)
	}

	// 选择器变更
	selectorDiff := CompareStringMaps(existingSvc.Spec.Selector, req.Selector)
	if HasMapChanges(selectorDiff) {
		selectorChangeDetail := BuildMapDiffDetail(selectorDiff, true)
		changeDetails = append(changeDetails, fmt.Sprintf("选择器变更: %s", selectorChangeDetail))
	}

	// Labels 变更
	labelsDiff := CompareStringMaps(existingSvc.Labels, req.Labels)
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
		ClusterUuid: workloadInfo.Data.ClusterUuid,
		Namespace:   workloadInfo.Data.Namespace,
	})
	if err != nil {
		l.Errorf("获取项目详情失败: %v", err)
		return "", fmt.Errorf("获取项目详情失败")
	} else {
		// 注入注解
		utils.AddAnnotations(&updatedSvc.ObjectMeta, &utils.AnnotationsInfo{
			ServiceName:   updatedSvc.Name,
			ProjectName:   projectDetail.ProjectNameCn,
			WorkspaceName: projectDetail.WorkspaceNameCn,
			ProjectUuid:   projectDetail.ProjectUuid,
		})
	}

	// 执行更新
	_, updateErr := serviceClient.Update(updatedSvc)
	if updateErr != nil {
		l.Errorf("更新 Service 失败: %v", updateErr)
		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			WorkspaceId:  req.WorkloadId,
			Title:        "更新 Service",
			ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 更新 Service %s 失败, 错误原因: %v, 变更内容: %s", username, workloadInfo.Data.Namespace, req.Name, updateErr, changeDetailStr),
			Status:       0,
		})
		return "", fmt.Errorf("更新 Service 失败: %v", updateErr)
	}

	// 记录成功的审计日志
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		WorkspaceId:  req.WorkloadId,
		Title:        "更新 Service",
		ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 成功更新 Service %s, %s", username, workloadInfo.Data.Namespace, req.Name, changeDetailStr),
		Status:       1,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	l.Infof("成功更新 Service: %s", req.Name)
	return "更新成功", nil
}

// extractServicePorts 从 k8s Service 提取端口信息
func (l *ServiceUpdateLogic) extractServicePorts(svc *corev1.Service) []types.ServicePort {
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
func (l *ServiceUpdateLogic) buildPortInfo(ports []types.ServicePort) string {
	if len(ports) == 0 {
		return "无"
	}
	var portStrs []string
	for _, p := range ports {
		portStrs = append(portStrs, fmt.Sprintf("%d->%s", p.Port, p.TargetPort))
	}
	return strings.Join(portStrs, ", ")
}
