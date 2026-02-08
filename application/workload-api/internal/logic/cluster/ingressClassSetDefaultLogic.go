package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type IngressClassSetDefaultLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 设置默认 IngressClass
func NewIngressClassSetDefaultLogic(ctx context.Context, svcCtx *svc.ServiceContext) *IngressClassSetDefaultLogic {
	return &IngressClassSetDefaultLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *IngressClassSetDefaultLogic) IngressClassSetDefault(req *types.SetDefaultIngressClassRequest) (resp string, err error) {
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

	// 查找当前默认的 IngressClass
	icList, _ := icOp.List("", "")
	var oldDefaultIC string
	if icList != nil {
		for _, ic := range icList.Items {
			if ic.IsDefault && ic.Name != req.Name {
				oldDefaultIC = ic.Name
				break
			}
		}
	}

	// 获取目标 IngressClass 的控制器信息
	targetIC, _ := icOp.Get(req.Name)
	var controller string
	if targetIC != nil {
		controller = targetIC.Spec.Controller
	}

	err = icOp.SetDefault(req.Name)
	if err != nil {
		l.Errorf("设置默认 IngressClass 失败: %v", err)
		// 记录失败审计日志
		l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "设置默认 IngressClass",
			ActionDetail: fmt.Sprintf("用户 %s 设置默认 IngressClass %s 失败, 错误: %v", username, req.Name, err),
			Status:       0,
		})
		return "", fmt.Errorf("设置默认 IngressClass 失败")
	}

	l.Infof("用户: %s, 成功设置默认 IngressClass: %s", username, req.Name)

	// 构建详细审计信息
	var changeInfo string
	if oldDefaultIC != "" {
		changeInfo = fmt.Sprintf(", 原默认: %s → 新默认: %s", oldDefaultIC, req.Name)
	} else {
		changeInfo = fmt.Sprintf(", 新默认: %s", req.Name)
	}

	auditDetail := fmt.Sprintf("用户 %s 成功设置默认 IngressClass%s, 控制器: %s",
		username, changeInfo, controller)

	// 记录成功审计日志
	l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "设置默认 IngressClass",
		ActionDetail: auditDetail,
		Status:       1,
	})
	return "设置默认 IngressClass 成功", nil
}
