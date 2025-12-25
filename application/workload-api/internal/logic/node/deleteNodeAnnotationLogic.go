package node

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteNodeAnnotationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteNodeAnnotationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteNodeAnnotationLogic {
	return &DeleteNodeAnnotationLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteNodeAnnotationLogic) DeleteNodeAnnotation(req *types.NodeAnnotationDeleteRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}

	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	// 删除注解
	nodeOperator := client.Node()
	if err = nodeOperator.DeleteAnnotations(req.NodeName, req.Key); err != nil {
		l.Errorf("删除节点注解失败: %v", err)
		// 记录失败审计日志
		l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "节点删除注解",
			ActionDetail: fmt.Sprintf("用户 %s 删除节点 %s 的注解失败, Key: %s, 错误: %v", username, req.NodeName, req.Key, err),
			Status:       0,
		})
		return "", fmt.Errorf("删除节点注解失败")
	}

	l.Infof("成功删除节点注解: node=%s, key=%s", req.NodeName, req.Key)
	// 记录成功审计日志
	l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "节点删除注解",
		ActionDetail: fmt.Sprintf("用户 %s 删除节点 %s 的注解成功, Key: %s", username, req.NodeName, req.Key),
		Status:       1,
	})
	return "删除注解成功", nil
}
