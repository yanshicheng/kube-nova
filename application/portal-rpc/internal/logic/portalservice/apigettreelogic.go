package portalservicelogic

import (
	"context"
	"database/sql"
	"errors"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type APIGetTreeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAPIGetTreeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *APIGetTreeLogic {
	return &APIGetTreeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *APIGetTreeLogic) APIGetTree(in *pb.GetSysAPITreeReq) (*pb.GetSysAPITreeResp, error) {
	// 查询所有API数据，按父级ID和ID排序构建层次结构
	apis, err := l.svcCtx.SysApi.SearchNoPage(l.ctx, "parent_id, id", true, "", []interface{}{}...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// 没有数据时返回空树
			return &pb.GetSysAPITreeResp{
				Data: []*pb.SysAPITreeNode{},
			}, nil
		}
		l.Logger.Errorf("查询API权限数据失败: %v", err)
		return nil, err
	}

	// 创建临时节点数组，用于构建树形结构
	tempNodes := make([]tempAPINode, 0, len(apis))
	for _, api := range apis {
		tempNodes = append(tempNodes, tempAPINode{
			Id:           api.Id,
			ParentId:     api.ParentId,
			Name:         api.Name,
			IsPermission: api.IsPermission,
			Method:       api.Method,
		})
	}

	// 从根节点开始构建API权限树（parentId = 0）
	tree := l.buildAPITreeFromTemp(tempNodes, 0)

	return &pb.GetSysAPITreeResp{
		Data: tree,
	}, nil
}

// 辅助结构，用于构建API树
type tempAPINode struct {
	Id           uint64
	ParentId     uint64
	Name         string
	IsPermission int64
	Method       string
}

// buildAPITreeFromTemp 从临时节点构建API权限树
func (l *APIGetTreeLogic) buildAPITreeFromTemp(tempNodes []tempAPINode, parentId uint64) []*pb.SysAPITreeNode {
	tree := make([]*pb.SysAPITreeNode, 0)

	// 遍历所有临时节点，找出当前父级的直接子节点
	for _, tempNode := range tempNodes {
		if tempNode.ParentId == parentId {
			// 创建当前API节点
			node := &pb.SysAPITreeNode{
				Id:           tempNode.Id,
				Name:         tempNode.Name,
				IsPermission: tempNode.IsPermission,
				Method:       tempNode.Method,
			}

			// 递归构建子节点
			children := l.buildAPITreeFromTemp(tempNodes, tempNode.Id)
			if len(children) > 0 {
				node.Children = children
			}

			tree = append(tree, node)
		}
	}

	return tree
}
