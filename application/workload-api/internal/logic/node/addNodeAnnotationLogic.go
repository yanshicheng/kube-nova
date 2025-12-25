package node

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type AddNodeAnnotationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAddNodeAnnotationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddNodeAnnotationLogic {
	return &AddNodeAnnotationLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddNodeAnnotationLogic) AddNodeAnnotation(req *types.NodeAnnotationRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}

	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	// 添加注解
	nodeOperator := client.Node()
	if err = nodeOperator.AddAnnotations(req.NodeName, req.Key, req.Value); err != nil {
		l.Errorf("添加节点注解失败: %v", err)
		// 记录失败审计日志
		l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "节点添加注解",
			ActionDetail: fmt.Sprintf("用户 %s 为节点 %s 添加注解失败, Key: %s, Value: %s, 错误: %v", username, req.NodeName, req.Key, req.Value, err),
			Status:       0,
		})
		return "", fmt.Errorf("添加节点注解失败")
	}

	l.Infof("成功添加节点注解: node=%s, key=%s", req.NodeName, req.Key)
	// 记录成功审计日志
	l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "节点添加注解",
		ActionDetail: fmt.Sprintf("用户 %s 为节点 %s 添加注解成功, Key: %s, Value: %s", username, req.NodeName, req.Key, req.Value),
		Status:       1,
	})
	return "添加注解成功", nil
}
