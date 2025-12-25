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

type NodeAddLabelsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewNodeAddLabelsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NodeAddLabelsLogic {
	return &NodeAddLabelsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 添加 labels
func (l *NodeAddLabelsLogic) NodeAddLabels(in *pb.ClusterNodeAddLabelsReq) (*pb.ClusterNodeDisableResp, error) {
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
	err = nodeOperator.AddLabels(node.Name, in.Key, in.Value)

	auditStatus := int64(1)
	if err != nil {
		auditStatus = 0
		l.Errorf("添加节点标签失败: %v", err)
	}

	_, _ = l.svcCtx.OnecProjectAuditLog.Insert(l.ctx, &model.OnecProjectAuditLog{
		ClusterName:  cluster.Name,
		ClusterUuid:  cluster.Uuid,
		Title:        "节点标签添加",
		ActionDetail: fmt.Sprintf("节点 %s 添加标签 %s=%s", node.Name, in.Key, in.Value),
		Status:       auditStatus,
		OperatorName: in.UpdatedBy,
	})

	if err != nil {
		return nil, errorx.Msg("添加节点标签失败")
	}

	return &pb.ClusterNodeDisableResp{}, nil
}
