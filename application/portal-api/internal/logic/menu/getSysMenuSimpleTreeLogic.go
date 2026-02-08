package menu

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetSysMenuSimpleTreeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetSysMenuSimpleTreeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSysMenuSimpleTreeLogic {
	return &GetSysMenuSimpleTreeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetSysMenuSimpleTreeLogic) GetSysMenuSimpleTree(req *types.GetSysMenuSimpleTreeRequest) (resp []types.SysMenuSimpleTreeNode, err error) {
	// 验证 platformId 参数
	if req.PlatformId == 0 {
		l.Errorf("平台ID不能为空")
		return nil, fmt.Errorf("平台ID不能为空")
	}

	// 调用 RPC 服务获取精简菜单树
	rpcReq := &pb.GetSysMenuSimpleTreeReq{
		PlatformId: req.PlatformId,
		Status:     req.Status,
	}

	rpcResp, err := l.svcCtx.PortalRpc.MenuGetSimpleTree(l.ctx, rpcReq)
	if err != nil {
		logx.Errorf("调用 MenuGetSimpleTree RPC 失败: platformId=%d, error=%v", req.PlatformId, err)
		return nil, err
	}

	return convertPbSimpleTreeToTypes(rpcResp.Data), nil
}

// 转换 protobuf 精简菜单树节点为 types 节点
func convertPbSimpleTreeToTypes(pbNodes []*pb.SysMenuSimpleTreeNode) []types.SysMenuSimpleTreeNode {
	if len(pbNodes) == 0 {
		return []types.SysMenuSimpleTreeNode{}
	}

	result := make([]types.SysMenuSimpleTreeNode, 0, len(pbNodes))
	for _, node := range pbNodes {
		typeNode := types.SysMenuSimpleTreeNode{
			Id:    node.Id,
			Title: node.Title,
		}

		// 递归转换子节点
		if len(node.Children) > 0 {
			typeNode.Children = convertPbSimpleTreeToTypes(node.Children)
		} else {
			typeNode.Children = []types.SysMenuSimpleTreeNode{}
		}

		result = append(result, typeNode)
	}

	return result
}
