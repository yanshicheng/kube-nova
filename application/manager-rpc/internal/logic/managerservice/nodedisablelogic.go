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

type NodeDisableLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewNodeDisableLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NodeDisableLogic {
	return &NodeDisableLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 污点相关
func (l *NodeDisableLogic) NodeDisable(in *pb.ClusterNodeDisableReq) (*pb.ClusterNodeDisableResp, error) {
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
	var actionDetail string
	if in.Status == 2 {
		err = nodeOperator.DisableScheduling(node.Name)
		actionDetail = fmt.Sprintf("节点 %s 禁用调度", node.Name)
	} else {
		err = nodeOperator.EnableScheduling(node.Name)
		actionDetail = fmt.Sprintf("节点 %s 启用调度", node.Name)
	}

	auditStatus := int64(1)
	if err != nil {
		auditStatus = 0
		l.Errorf("设置节点调度状态失败: %v", err)
	}

	_, _ = l.svcCtx.OnecProjectAuditLog.Insert(l.ctx, &model.OnecProjectAuditLog{
		ClusterName:  cluster.Name,
		ClusterUuid:  cluster.Uuid,
		Title:        "节点调度设置",
		ActionDetail: actionDetail,
		Status:       auditStatus,
		OperatorName: in.UpdatedBy,
	})

	if err != nil {
		return nil, errorx.Msg("设置节点调度状态失败")
	}

	node.Unschedulable = in.Status
	node.UpdatedBy = in.UpdatedBy
	err = l.svcCtx.OnecClusterNodeModel.Update(l.ctx, node)
	if err != nil {
		l.Errorf("更新节点状态失败: %v", err)
		return nil, errorx.Msg("更新节点状态失败")
	}

	return &pb.ClusterNodeDisableResp{}, nil
}
