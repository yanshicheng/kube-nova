package portalservicelogic

import (
	"context"
	"database/sql"
	"errors"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

type MenuGetSimpleTreeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewMenuGetSimpleTreeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MenuGetSimpleTreeLogic {
	return &MenuGetSimpleTreeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 辅助结构，包含父级ID信息用于构建树
type tempMenuNode struct {
	Id       uint64
	ParentId uint64
	Title    string
}

func (l *MenuGetSimpleTreeLogic) MenuGetSimpleTree(in *pb.GetSysMenuSimpleTreeReq) (*pb.GetSysMenuSimpleTreeResp, error) {
	// 参数验证
	if in.PlatformId == 0 {
		l.Errorf("获取简单菜单树失败：平台ID不能为空")
		return nil, errorx.Msg("平台ID不能为空")
	}

	// 构建查询条件，排除按钮类型菜单（menu_type=3）
	queryConditions := "platform_id = ? AND menu_type != 3"
	args := []interface{}{in.PlatformId}

	// 如果指定了状态过滤条件
	if in.Status != -1 {
		queryConditions += " AND status = ?"
		args = append(args, in.Status)
	} else {
		// 默认只返回启用状态的菜单
		queryConditions += " AND status = ?"
		args = append(args, 1)
	}

	// 从数据库查询所有符合条件的菜单数据，按父级ID和排序字段排序
	menus, err := l.svcCtx.SysMenu.SearchNoPage(l.ctx, "parent_id, sort", true, queryConditions, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// 没有数据时返回空树
			return &pb.GetSysMenuSimpleTreeResp{
				Data: []*pb.SysMenuSimpleTreeNode{},
			}, nil
		}
		l.Logger.Errorf("查询菜单数据失败: platformId=%d, error=%v", in.PlatformId, err)
		return nil, err
	}

	// 将数据库结果转换为临时节点结构，包含构建树所需的父级信息
	tempNodes := make([]tempMenuNode, 0, len(menus))
	for _, menu := range menus {
		tempNodes = append(tempNodes, tempMenuNode{
			Id:       menu.Id,
			ParentId: menu.ParentId,
			Title:    menu.Title,
		})
	}

	// 从根节点开始构建树形结构（parentId = 0）
	tree := l.buildSimpleMenuTreeFromTemp(tempNodes, 0)

	return &pb.GetSysMenuSimpleTreeResp{
		Data: tree,
	}, nil
}

// buildSimpleMenuTreeFromTemp 从包含父级信息的临时节点构建树
func (l *MenuGetSimpleTreeLogic) buildSimpleMenuTreeFromTemp(tempNodes []tempMenuNode, parentId uint64) []*pb.SysMenuSimpleTreeNode {
	tree := make([]*pb.SysMenuSimpleTreeNode, 0)

	// 遍历所有临时节点，找出当前父级的直接子节点
	for _, tempNode := range tempNodes {
		if tempNode.ParentId == parentId {
			// 创建当前节点
			node := &pb.SysMenuSimpleTreeNode{
				Id:    tempNode.Id,
				Title: tempNode.Title,
			}

			// 递归构建子节点
			children := l.buildSimpleMenuTreeFromTemp(tempNodes, tempNode.Id)
			if len(children) > 0 {
				node.Children = children
			}

			// 将节点添加到树中
			tree = append(tree, node)
		}
	}

	return tree
}
