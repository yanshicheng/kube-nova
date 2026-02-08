package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除 Service
func NewServiceDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceDeleteLogic {
	return &ServiceDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceDeleteLogic) ServiceDelete(req *types.DefaultNameRequest) (resp string, err error) {
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

	// 获取 Service 详情用于审计
	existingSvc, _ := serviceClient.Get(workloadInfo.Data.Namespace, req.Name)
	var svcType string
	var portsInfo string
	var clusterIP string
	if existingSvc != nil {
		svcType = string(existingSvc.Spec.Type)
		clusterIP = existingSvc.Spec.ClusterIP

		var portStrs []string
		for _, p := range existingSvc.Spec.Ports {
			portStrs = append(portStrs, fmt.Sprintf("%d→%s", p.Port, p.TargetPort.String()))
		}
		portsInfo = strings.Join(portStrs, ", ")
		if portsInfo == "" {
			portsInfo = "无"
		}
	}

	// 删除 Service
	deleteErr := serviceClient.Delete(workloadInfo.Data.Namespace, req.Name)
	if deleteErr != nil {
		l.Errorf("删除 Service 失败: %v", deleteErr)
		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			WorkspaceId:  req.WorkloadId,
			Title:        "删除 Service",
			ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 删除 Service %s 失败, 类型: %s, 错误原因: %v", username, workloadInfo.Data.Namespace, req.Name, svcType, deleteErr),
			Status:       0,
		})
		return "", fmt.Errorf("删除 Service 失败")
	}

	// 记录成功的审计日志
	auditDetail := fmt.Sprintf("用户 %s 在命名空间 %s 成功删除 Service %s, 类型: %s, ClusterIP: %s, 端口: %s", username, workloadInfo.Data.Namespace, req.Name, svcType, clusterIP, portsInfo)

	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		WorkspaceId:  req.WorkloadId,
		Title:        "删除 Service",
		ActionDetail: auditDetail,
		Status:       1,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	l.Infof("成功删除 Service: %s", req.Name)
	return "删除成功", nil
}
