package networkpolicy

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type NetworkPolicyDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除 NetworkPolicy
func NewNetworkPolicyDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NetworkPolicyDeleteLogic {
	return &NetworkPolicyDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *NetworkPolicyDeleteLogic) NetworkPolicyDelete(req *types.NetworkPolicyNameRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}

	// 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	// 获取 NetworkPolicy 操作器
	networkPolicyOp := client.NetworkPolicies()

	// 调用 Delete 方法
	deleteErr := networkPolicyOp.Delete(req.Namespace, req.Name)
	if deleteErr != nil {
		l.Errorf("删除 NetworkPolicy 失败: %v", deleteErr)
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid: req.ClusterUuid,
			Title:       "删除 NetworkPolicy",
			ActionDetail: fmt.Sprintf("用户 %s 删除 NetworkPolicy 失败: %s/%s, 错误: %v",
				username, req.Namespace, req.Name, deleteErr),
			Status: 0,
		})
		return "", fmt.Errorf("删除 NetworkPolicy 失败: %v", deleteErr)
	}

	// 记录成功的审计日志
	_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid: req.ClusterUuid,
		Title:       "删除 NetworkPolicy",
		ActionDetail: fmt.Sprintf("用户 %s 删除 NetworkPolicy: %s/%s",
			username, req.Namespace, req.Name),
		Status: 1,
	})

	l.Infof("用户: %s, 成功删除 NetworkPolicy %s/%s", username, req.Namespace, req.Name)

	return fmt.Sprintf("NetworkPolicy %s/%s 删除成功", req.Namespace, req.Name), nil
}
