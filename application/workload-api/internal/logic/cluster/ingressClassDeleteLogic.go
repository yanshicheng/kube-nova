package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type IngressClassDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除 IngressClass
func NewIngressClassDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *IngressClassDeleteLogic {
	return &IngressClassDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *IngressClassDeleteLogic) IngressClassDelete(req *types.IngressClassDeleteRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}

	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	icOp := client.IngressClasses()

	// 获取 IngressClass 详情用于审计
	existingIC, _ := icOp.Get(req.Name)
	var controller string
	var isDefault bool
	if existingIC != nil {
		controller = existingIC.Spec.Controller
		if existingIC.Annotations != nil {
			if val, ok := existingIC.Annotations["ingressclass.kubernetes.io/is-default-class"]; ok && val == "true" {
				isDefault = true
			}
		}
	}

	// 获取使用情况
	usage, _ := icOp.GetUsage(req.Name)
	var usageInfo string
	if usage != nil && usage.IngressCount > 0 {
		usageInfo = fmt.Sprintf(", 关联 Ingress: %d 个, 涉及命名空间: %d 个",
			usage.IngressCount, usage.NamespaceCount)
	}

	err = icOp.Delete(req.Name)
	if err != nil {
		l.Errorf("删除 IngressClass 失败: %v", err)
		// 记录失败审计日志
		l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "删除 IngressClass",
			ActionDetail: fmt.Sprintf("用户 %s 删除 IngressClass %s 失败, 错误: %v", username, req.Name, err),
			Status:       0,
		})
		return "", fmt.Errorf("删除 IngressClass 失败")
	}

	l.Infof("用户: %s, 成功删除 IngressClass: %s", username, req.Name)

	// 构建详细审计信息
	var defaultInfo string
	if isDefault {
		defaultInfo = ", 默认: 是"
	}

	auditDetail := fmt.Sprintf("用户 %s 成功删除 IngressClass %s, 控制器: %s%s%s",
		username, req.Name, controller, defaultInfo, usageInfo)

	// 记录成功审计日志
	l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "删除 IngressClass",
		ActionDetail: auditDetail,
		Status:       1,
	})
	return "删除 IngressClass 成功", nil
}
