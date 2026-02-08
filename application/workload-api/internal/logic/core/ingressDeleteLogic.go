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

type IngressDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除 Ingress
func NewIngressDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *IngressDeleteLogic {
	return &IngressDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *IngressDeleteLogic) IngressDelete(req *types.DefaultNameRequest) (resp string, err error) {
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

	// 初始化 Ingress 客户端
	ingressClient := client.Ingresses()

	// 获取 Ingress 详情用于审计
	existingIngress, _ := ingressClient.Get(workloadInfo.Data.Namespace, req.Name)
	var hostsInfo string
	var tlsEnabled bool
	if existingIngress != nil {
		var hosts []string
		for _, rule := range existingIngress.Spec.Rules {
			if rule.Host != "" {
				hosts = append(hosts, rule.Host)
			}
		}
		hostsInfo = strings.Join(hosts, ", ")
		if hostsInfo == "" {
			hostsInfo = "无"
		}
		tlsEnabled = len(existingIngress.Spec.TLS) > 0
	}

	// 删除 Ingress
	deleteErr := ingressClient.Delete(workloadInfo.Data.Namespace, req.Name)
	if deleteErr != nil {
		l.Errorf("删除 Ingress 失败: %v", deleteErr)
		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			WorkspaceId:  req.WorkloadId,
			Title:        "删除 Ingress",
			ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 删除 Ingress %s 失败, 主机: %s, 错误原因: %v", username, workloadInfo.Data.Namespace, req.Name, hostsInfo, deleteErr),
			Status:       0,
		})
		return "", fmt.Errorf("删除 Ingress 失败: %v", deleteErr)
	}

	// 记录成功的审计日志
	tlsStatus := "未启用"
	if tlsEnabled {
		tlsStatus = "已启用"
	}
	auditDetail := fmt.Sprintf("用户 %s 在命名空间 %s 成功删除 Ingress %s, 主机: %s, TLS: %s", username, workloadInfo.Data.Namespace, req.Name, hostsInfo, tlsStatus)

	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		WorkspaceId:  req.WorkloadId,
		Title:        "删除 Ingress",
		ActionDetail: auditDetail,
		Status:       1,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	l.Infof("成功删除 Ingress: %s/%s", workloadInfo.Data.Namespace, req.Name)
	return fmt.Sprintf("成功删除 Ingress: %s", req.Name), nil
}
