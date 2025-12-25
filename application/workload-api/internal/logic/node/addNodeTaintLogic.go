package node

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type AddNodeTaintLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAddNodeTaintLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddNodeTaintLogic {
	return &AddNodeTaintLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddNodeTaintLogic) AddNodeTaint(req *types.NodeTaintRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}

	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	// 添加污点
	nodeOperator := client.Node()
	if err = nodeOperator.AddTaint(req.NodeName, req.Key, req.Value, req.Effect); err != nil {
		l.Errorf("添加节点污点失败: %v", err)
		// 记录失败审计日志
		l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "节点添加污点",
			ActionDetail: fmt.Sprintf("用户 %s 为节点 %s 添加污点失败, Key: %s, Value: %s, Effect: %s, 错误: %v", username, req.NodeName, req.Key, req.Value, req.Effect, err),
			Status:       0,
		})
		return "", fmt.Errorf("添加节点污点失败")
	}

	l.Infof("成功添加节点污点: node=%s, key=%s, effect=%s", req.NodeName, req.Key, req.Effect)
	// 记录成功审计日志
	l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "节点添加污点",
		ActionDetail: fmt.Sprintf("用户 %s 为节点 %s 添加污点成功, Key: %s, Value: %s, Effect: %s", username, req.NodeName, req.Key, req.Value, req.Effect),
		Status:       1,
	})
	return "添加污点成功", nil
}
