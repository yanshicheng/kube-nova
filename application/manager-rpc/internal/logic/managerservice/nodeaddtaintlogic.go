package managerservicelogic

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type NodeAddTaintLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewNodeAddTaintLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NodeAddTaintLogic {
	return &NodeAddTaintLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 添加污点
func (l *NodeAddTaintLogic) NodeAddTaint(in *pb.ClusterNodeAddTaintReq) (*pb.ClusterNodeAddTaintResp, error) {
	node, err := l.svcCtx.OnecClusterNodeModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("获取节点失败: %v", err)
		return nil, errorx.Msg("获取节点失败")
	}

	cluster, err := l.svcCtx.OnecClusterModel.FindOneByUuid(l.ctx, node.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群信息失败: %v", err)
		return nil, errorx.Msg("获取集群信息失败")
	}

	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, node.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群失败: %v", err)
		return nil, errorx.Msg("获取集群失败")
	}

	nodeOperator := client.Node()
	err = nodeOperator.AddTaint(node.Name, in.Key, in.Value, in.Effect)

	auditStatus := int64(1)
	if err != nil {
		auditStatus = 0
		l.Errorf("添加节点污点失败: %v", err)
	}

	_, _ = l.svcCtx.OnecProjectAuditLog.Insert(l.ctx, &model.OnecProjectAuditLog{
		ClusterName:  cluster.Name,
		ClusterUuid:  cluster.Uuid,
		Title:        "节点污点添加",
		ActionDetail: fmt.Sprintf("节点 %s 添加污点 %s=%s:%s", node.Name, in.Key, in.Value, in.Effect),
		Status:       auditStatus,
		OperatorName: in.UpdatedBy,
	})

	if err != nil {
		return nil, errorx.Msg("添加节点污点失败")
	}

	return &pb.ClusterNodeAddTaintResp{}, nil
}
