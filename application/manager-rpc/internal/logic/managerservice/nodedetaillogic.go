package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type NodeDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewNodeDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NodeDetailLogic {
	return &NodeDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *NodeDetailLogic) NodeDetail(in *pb.ClusterNodeDetailReq) (*pb.OnecClusterNodeResp, error) {
	node, err := l.svcCtx.OnecClusterNodeModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("获取节点详情失败: %v", err)
		return nil, errorx.Msg("获取节点详情失败")
	}

	return &pb.OnecClusterNodeResp{
		Id:              node.Id,
		ClusterUuid:     node.ClusterUuid,
		NodeUuid:        node.NodeUuid,
		Name:            node.Name,
		Hostname:        node.Hostname,
		Roles:           node.Roles,
		OsImage:         node.OsImage,
		NodeIp:          node.NodeIp,
		KernelVersion:   node.KernelVersion,
		OperatingSystem: node.OperatingSystem,
		Architecture:    node.Architecture,
		Cpu:             node.Cpu,
		Memory:          node.Memory,
		Pods:            node.Pods,
		IsGpu:           node.IsGpu,
		Runtime:         node.Runtime,
		JoinAt:          node.JoinAt.Unix(),
		Unschedulable:   node.Unschedulable,
		KubeletVersion:  node.KubeletVersion,
		Status:          node.Status,
		PodCidr:         node.PodCidr,
		PodCidrs:        node.PodCidrs,
		CreatedBy:       node.CreatedBy,
		UpdatedBy:       node.UpdatedBy,
		CreatedAt:       node.CreatedAt.Unix(),
		UpdatedAt:       node.UpdatedAt.Unix(),
	}, nil
}
