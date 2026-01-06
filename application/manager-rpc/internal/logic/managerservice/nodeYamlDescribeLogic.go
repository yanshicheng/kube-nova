package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type NodeYamlDescribeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewNodeYamlDescribeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NodeYamlDescribeLogic {
	return &NodeYamlDescribeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 获取 node  yaml describe
func (l *NodeYamlDescribeLogic) NodeYamlDescribe(in *pb.ClusterNodeYamlDescribeReq) (*pb.ClusterNodeYamlDescribeResp, error) {
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
	descibeYaml, err := nodeOperator.Describe(node.Name)
	if err != nil {
		l.Errorf("获取节点yaml详情失败: %v", err)
		return nil, errorx.Msg("获取节点yaml详情失败")
	}
	return &pb.ClusterNodeYamlDescribeResp{
		Data: descibeYaml,
	}, nil
}
