package node

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetNodeListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetNodeListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNodeListLogic {
	return &GetNodeListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNodeListLogic) GetNodeList(req *types.SearchClusterNodeRequest) (resp *types.SearchClusterNodeResponse, err error) {
	rpcResp, err := l.svcCtx.ManagerRpc.NodeInfo(l.ctx, &pb.ClusterNodeInfoReq{
		Page:        req.Page,
		PageSize:    req.PageSize,
		OrderField:  req.OrderField,
		IsAsc:       req.IsAsc,
		ClusterUuid: req.ClusterUuid,
	})
	if err != nil {
		l.Errorf("查询节点列表失败: %v", err)
		return nil, errorx.Msg("查询节点列表失败")
	}

	var items []types.ClusterNodeInfo
	for _, node := range rpcResp.Data {
		items = append(items, types.ClusterNodeInfo{
			Id:            node.Id,
			ClusterUuid:   node.ClusterUuid,
			NodeName:      node.NodeName,
			NodeIp:        node.NodeIp,
			NodeStatus:    node.NodeStatus,
			CreatedAt:     node.CreatedAt,
			UpdatedAt:     node.UpdatedAt,
			NodeRole:      node.NodeRole,
			Architecture:  node.Architecture,
			Unschedulable: node.Unschedulable,
		})
	}

	return &types.SearchClusterNodeResponse{
		Items: items,
		Total: rpcResp.Total,
	}, nil
}
