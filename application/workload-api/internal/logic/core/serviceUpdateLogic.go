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

	// 构建端口信息用于审计
	portInfo := l.buildPortInfo(req.Ports)
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
			ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 更新 Service %s 失败, 类型: %s, 端口: %s, 错误原因: %v", username, workloadInfo.Data.Namespace, req.Name, req.Type, portInfo, updateErr),
			Status:       0,
		})
		return "", fmt.Errorf("更新 Service 失败: %v", updateErr)
	}

	// 记录成功的审计日志
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		WorkspaceId:  req.WorkloadId,
		Title:        "更新 Service",
		ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 成功更新 Service %s, 类型: %s, 端口: %s", username, workloadInfo.Data.Namespace, req.Name, req.Type, portInfo),
		Status:       1,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	l.Infof("成功更新 Service: %s", req.Name)
	return "更新成功", nil
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
