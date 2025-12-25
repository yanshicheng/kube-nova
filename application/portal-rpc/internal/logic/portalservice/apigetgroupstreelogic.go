package portalservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type APIGetGroupsTreeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAPIGetGroupsTreeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *APIGetGroupsTreeLogic {
	return &APIGetGroupsTreeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// APIGetGroupsTree 获取所有分组的树状数据
func (l *APIGetGroupsTreeLogic) APIGetGroupsTree(in *pb.GetGroupsTreeReq) (*pb.GetGroupsTreeResp, error) {
	// 构建查询条件，根据isPermission参数过滤
	var query string
	var args []interface{}

	// 如果传入了isPermission参数，则只查询指定类型的数据
	// isPermission = 0: 只查询分组
	// isPermission = 1: 只查询权限
	// 如果不传或传其他值，查询所有数据
	if in.IsPermission == 0 || in.IsPermission == 1 {
		query = "`is_permission` = ?"
		args = append(args, in.IsPermission)
	} else {
		// 查询所有数据（分组和权限都要）
		query = ""
		args = nil
	}

	// 查询所有符合条件的API数据，按照父级ID和排序字段排序
	allAPIs, err := l.svcCtx.SysApi.SearchNoPage(l.ctx, "parent_id, id", true, query, args...)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		l.Errorf("查询API分组数据失败: %v", err)
		return nil, errorx.Msg("查询API分组数据失败")
	}

	// 如果没有数据，返回空结果
	if len(allAPIs) == 0 {
		l.Infof("获取API分组树状数据完成，未找到任何数据")
		return &pb.GetGroupsTreeResp{
			Data: []*pb.GroupTreeNode{},
		}, nil
	}

	// 转换为GroupTreeNode格式
	treeNodes := l.convertToGroupTreeNodes(allAPIs)

	// 构建树状结构
	groupTree := l.buildGroupTree(treeNodes)

	return &pb.GetGroupsTreeResp{
		Data: groupTree,
	}, nil
}

// convertToGroupTreeNodes 将API数据转换为GroupTreeNode格式
func (l *APIGetGroupsTreeLogic) convertToGroupTreeNodes(apis []*model.SysApi) []*pb.GroupTreeNode {
	var nodes []*pb.GroupTreeNode
	for _, api := range apis {
		node := &pb.GroupTreeNode{
			Id:       api.Id,
			Name:     api.Name,
			Pid:      api.ParentId,
			Children: []*pb.GroupTreeNode{}, // 初始化子节点切片
		}
		nodes = append(nodes, node)
	}
	return nodes
}

// buildGroupTree 构建分组树状结构
func (l *APIGetGroupsTreeLogic) buildGroupTree(nodes []*pb.GroupTreeNode) []*pb.GroupTreeNode {
	// 创建ID到节点的映射，便于快速查找
	nodeMap := make(map[uint64]*pb.GroupTreeNode)
	var rootNodes []*pb.GroupTreeNode

	// 第一步：建立ID映射
	for _, node := range nodes {
		nodeMap[node.Id] = node
	}

	// 第二步：构建父子关系
	for _, node := range nodes {
		if node.Pid == 0 {
			// 顶级节点
			rootNodes = append(rootNodes, node)
		} else {
			// 子节点，找到其父节点并加入到父节点的children中
			if parent, exists := nodeMap[node.Pid]; exists {
				parent.Children = append(parent.Children, node)
			} else {
				// 父节点不存在，可能数据有问题，将其作为顶级节点处理
				l.Errorf("API分组 %d (名称: %s) 的父节点 %d 不存在，将其作为顶级节点处理",
					node.Id, node.Name, node.Pid)
				node.Pid = 0 // 重置父ID
				rootNodes = append(rootNodes, node)
			}
		}
	}

	// 第三步：对每个层级的节点按ID排序（简单排序）
	l.sortGroupTree(rootNodes)

	return rootNodes
}

// sortGroupTree 递归排序分组树（按ID升序）
func (l *APIGetGroupsTreeLogic) sortGroupTree(nodes []*pb.GroupTreeNode) {
	if len(nodes) <= 1 {
		return
	}

	// 使用冒泡排序按ID排序
	for i := 0; i < len(nodes)-1; i++ {
		for j := 0; j < len(nodes)-1-i; j++ {
			if nodes[j].Id > nodes[j+1].Id {
				nodes[j], nodes[j+1] = nodes[j+1], nodes[j]
			}
		}
	}

	// 递归排序子节点
	for _, node := range nodes {
		if len(node.Children) > 0 {
			l.sortGroupTree(node.Children)
		}
	}
}
