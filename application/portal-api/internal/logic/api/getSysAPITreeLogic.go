package api

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetSysAPITreeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetSysAPITreeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSysAPITreeLogic {
	return &GetSysAPITreeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetSysAPITreeLogic) GetSysAPITree(req *types.GetSysAPITreeRequest) (resp []types.SysAPITreeNode, err error) {
	// 调用 RPC 服务获取API权限树
	rpcReq := &pb.GetSysAPITreeReq{
		// 可以添加过滤条件，如只获取权限或只获取分组
	}

	rpcResp, err := l.svcCtx.PortalRpc.APIGetTree(l.ctx, rpcReq)
	if err != nil {
		logx.Errorf("调用 APIGetTree RPC 失败: %v", err)
		return nil, err
	}

	return convertPbAPITreeToTypes(rpcResp.Data), nil
}

// 转换 protobuf API权限树节点为 types 节点
func convertPbAPITreeToTypes(pbNodes []*pb.SysAPITreeNode) []types.SysAPITreeNode {
	if len(pbNodes) == 0 {
		return []types.SysAPITreeNode{}
	}

	result := make([]types.SysAPITreeNode, 0, len(pbNodes))
	for _, node := range pbNodes {
		typeNode := types.SysAPITreeNode{
			Id:           node.Id,
			Name:         node.Name,
			IsPermission: node.IsPermission,
		}

		// 递归转换子节点
		if len(node.Children) > 0 {
			typeNode.Children = convertPbAPITreeToTypes(node.Children)
		} else {
			typeNode.Children = []types.SysAPITreeNode{}
		}

		result = append(result, typeNode)
	}

	return result
}
