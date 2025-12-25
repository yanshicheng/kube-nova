package monitoring

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type PrometheusRuleDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除 PrometheusRule
func NewPrometheusRuleDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PrometheusRuleDeleteLogic {
	return &PrometheusRuleDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PrometheusRuleDeleteLogic) PrometheusRuleDelete(req *types.MonitoringResourceNameRequest) (resp string, err error) {
	// 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}
	deleteErr := client.PrometheusRule().Delete(req.Namespace, req.Name)
	if deleteErr != nil {
		l.Errorf("删除  失败: %v", deleteErr)
		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "删除 prometheus Rule",
			ActionDetail: fmt.Sprintf(" 在命名空间 %s 删除 prometheus Rule %s 失败, 错误原因: %v", req.Namespace, req.Name, deleteErr),
			Status:       0,
		})
		return "", fmt.Errorf("删除 ConfigMap 失败")
	}

	// 记录成功的审计日志
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "删除 prometheus Rule",
		ActionDetail: fmt.Sprintf(" 在命名空间 %s 成功删除 prometheus Rule %s", req.Namespace, req.Name),
		Status:       1,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}
	return
}
