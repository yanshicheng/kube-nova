package api

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetGroupsTreeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetGroupsTreeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetGroupsTreeLogic {
	return &GetGroupsTreeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetGroupsTreeLogic) GetGroupsTree(req *types.GetGroupsTreeRequest) (resp []types.GroupTreeNode, err error) {
	// 调用 RPC 服务获取分组树
	res, err := l.svcCtx.PortalRpc.APIGetGroupsTree(l.ctx, &pb.GetGroupsTreeReq{
		IsPermission: req.IsPermission,
	})
	if err != nil {
		l.Errorf("获取分组树失败: error=%v", err)
		return nil, err
	}

	// 转换分组树
	var convertGroupTree func(*pb.GroupTreeNode) types.GroupTreeNode
	convertGroupTree = func(pbNode *pb.GroupTreeNode) types.GroupTreeNode {
		node := types.GroupTreeNode{
			Id:   pbNode.Id,
			Name: pbNode.Name,
			Pid:  pbNode.Pid,
		}
		for _, child := range pbNode.Children {
			node.Children = append(node.Children, convertGroupTree(child))
		}
		return node
	}

	var groups []types.GroupTreeNode
	for _, group := range res.Data {
		groups = append(groups, convertGroupTree(group))
	}

	return groups, nil
}
