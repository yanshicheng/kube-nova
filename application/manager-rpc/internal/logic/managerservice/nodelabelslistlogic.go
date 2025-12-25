package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type NodeLabelsListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewNodeLabelsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NodeLabelsListLogic {
	return &NodeLabelsListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// labels 相关
func (l *NodeLabelsListLogic) NodeLabelsList(in *pb.ClusterNodeLabelsListReq) (*pb.ClusterNodeLabelsListResp, error) {
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
	labels, err := nodeOperator.GetLabels(node.Name)
	if err != nil {
		l.Errorf("获取节点标签失败: %v", err)
		return nil, errorx.Msg("获取节点标签失败")
	}

	var list []*pb.ClusterNodeLabel
	for _, label := range labels {
		list = append(list, &pb.ClusterNodeLabel{
			Key:      label.Key,
			Value:    label.Value,
			IsDelete: label.IsDelete,
		})
	}

	return &pb.ClusterNodeLabelsListResp{Labels: list}, nil
}
