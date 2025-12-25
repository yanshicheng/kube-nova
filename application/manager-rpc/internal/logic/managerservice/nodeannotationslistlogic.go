package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type NodeAnnotationsListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewNodeAnnotationsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NodeAnnotationsListLogic {
	return &NodeAnnotationsListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 获取注解列表
func (l *NodeAnnotationsListLogic) NodeAnnotationsList(in *pb.ClusterNodeAnnotationsListReq) (*pb.ClusterNodeAnnotationsListResp, error) {
	node, err := l.svcCtx.OnecClusterNodeModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("获取节点失败: %v", err)
		return nil, errorx.Msg("获取节点失败")
	}

	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, node.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群失败: %v", err)
		return nil, errorx.Msg("获取集群失败")
	}

	nodeOperator := client.Node()
	annotations, err := nodeOperator.GetAnnotations(node.Name)
	if err != nil {
		l.Errorf("获取节点注解失败: %v", err)
		return nil, errorx.Msg("获取节点注解失败")
	}

	var list []*pb.ClusterNodeAnnotation
	for _, annotation := range annotations {
		list = append(list, &pb.ClusterNodeAnnotation{
			Key:      annotation.Key,
			Value:    annotation.Value,
			IsDelete: annotation.IsDelete,
		})
	}

	return &pb.ClusterNodeAnnotationsListResp{Annotations: list}, nil
}
