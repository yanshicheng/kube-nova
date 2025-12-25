package node

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type EvictNodeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewEvictNodeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EvictNodeLogic {
	return &EvictNodeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *EvictNodeLogic) EvictNode(req *types.NodeEvictRequest) (resp string, err error) {
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

	// 驱逐节点
	nodeOperator := client.Node()
	if err = nodeOperator.DrainNode(req.NodeName); err != nil {
		l.Errorf("驱逐节点失败: %v", err)
		// 记录失败审计日志
		l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "驱逐节点",
			ActionDetail: fmt.Sprintf("用户 %s 驱逐节点 %s 失败, 错误: %v", username, req.NodeName, err),
			Status:       0,
		})
		return "", fmt.Errorf("驱逐节点失败")
	}

	l.Infof("成功驱逐节点: node=%s", req.NodeName)
	// 记录成功审计日志
	l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "驱逐节点",
		ActionDetail: fmt.Sprintf("用户 %s 驱逐节点 %s 成功, 节点上的所有 Pod 已被安全迁移", username, req.NodeName),
		Status:       1,
	})
	return "驱逐节点成功", nil
}
