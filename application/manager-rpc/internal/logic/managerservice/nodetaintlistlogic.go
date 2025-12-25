package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type NodeTaintListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewNodeTaintListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NodeTaintListLogic {
	return &NodeTaintListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 获取污点列表
func (l *NodeTaintListLogic) NodeTaintList(in *pb.ClusterNodeTaintsListReq) (*pb.ClusterNodeTaintsListResp, error) {
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
	taints, err := nodeOperator.GetTaints(node.Name)
	if err != nil {
		l.Errorf("获取节点污点失败: %v", err)
		return nil, errorx.Msg("获取节点污点失败")
	}

	var list []*pb.ClusterNodeTaint
	for _, taint := range taints {
		list = append(list, &pb.ClusterNodeTaint{
			Key:      taint.Key,
			Value:    taint.Value,
			Effect:   taint.Effect,
			IsDelete: taint.IsDelete,
		})
	}

	return &pb.ClusterNodeTaintsListResp{Taints: list}, nil
}
