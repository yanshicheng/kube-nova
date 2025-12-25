package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type NodeInfoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewNodeInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NodeInfoLogic {
	return &NodeInfoLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// node 相关
func (l *NodeInfoLogic) NodeInfo(in *pb.ClusterNodeInfoReq) (*pb.ClusterNodeInfoResp, error) {
	queryStr := "`cluster_uuid` = ?"
	args := []any{in.ClusterUuid}

	orderField := "id"
	if in.OrderField != "" {
		orderField = in.OrderField
	}

	nodes, total, err := l.svcCtx.OnecClusterNodeModel.Search(l.ctx, orderField, in.IsAsc, in.Page, in.PageSize, queryStr, args...)
	if err != nil {
		l.Errorf("查询节点列表失败: %v", err)
		return nil, errorx.Msg("查询节点列表失败")
	}

	var list []*pb.ClusterNodeInfo
	for _, node := range nodes {
		list = append(list, &pb.ClusterNodeInfo{
			Id:            node.Id,
			ClusterUuid:   node.ClusterUuid,
			NodeName:      node.Name,
			NodeIp:        node.NodeIp,
			NodeStatus:    node.Status,
			CreatedAt:     node.CreatedAt.Unix(),
			UpdatedAt:     node.UpdatedAt.Unix(),
			NodeRole:      node.Roles,
			Architecture:  node.Architecture,
			Unschedulable: node.Unschedulable,
		})
	}

	return &pb.ClusterNodeInfoResp{
		Data:  list,
		Total: total,
	}, nil
}
