package portalservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeptSearchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeptSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeptSearchLogic {
	return &DeptSearchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// DeptSearch 搜索部门（返回树状结构）
func (l *DeptSearchLogic) DeptSearch(in *pb.SearchSysDeptReq) (*pb.SearchSysDeptResp, error) {
	// 查询所有部门数据
	allDepts, err := l.svcCtx.SysDept.SearchNoPage(l.ctx, "sort", true, "", nil)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		l.Errorf("查询所有部门失败: %v", err)
		return nil, errorx.Msg("查询部门列表失败")
	}

	// 如果没有数据，返回空结果
	if len(allDepts) == 0 {
		l.Infof("搜索部门完成，未找到任何部门数据")
		return &pb.SearchSysDeptResp{
			Data: []*pb.SysDeptTree{},
		}, nil
	}

	// 转换为protobuf格式
	pbDepts := l.convertToPbDeptList(allDepts)

	// 构建树状结构
	deptTree := l.buildDeptTree(pbDepts)

	return &pb.SearchSysDeptResp{
		Data: deptTree,
	}, nil
}

// convertToPbDeptList 将数据库模型列表转换为protobuf格式列表
func (l *DeptSearchLogic) convertToPbDeptList(depts []*model.SysDept) []*pb.SysDeptTree {
	var pbDepts []*pb.SysDeptTree
	for _, dept := range depts {
		pbDept := &pb.SysDeptTree{
			Id:        dept.Id,
			Name:      dept.Name,
			ParentId:  dept.ParentId,
			Remark:    dept.Remark,
			Leader:    dept.Leader,
			Phone:     dept.Phone,
			Email:     dept.Email,
			Status:    dept.Status,
			Sort:      dept.Sort,
			CreatedBy: dept.CreatedBy,
			UpdatedBy: dept.UpdatedBy,
			CreatedAt: dept.CreatedAt.Unix(),
			UpdatedAt: dept.UpdatedAt.Unix(),
			Children:  []*pb.SysDeptTree{}, // 初始化子节点切片
		}
		pbDepts = append(pbDepts, pbDept)
	}
	return pbDepts
}

// buildDeptTree 构建部门树状结构
func (l *DeptSearchLogic) buildDeptTree(depts []*pb.SysDeptTree) []*pb.SysDeptTree {
	// 创建ID到部门的映射，便于快速查找
	deptMap := make(map[uint64]*pb.SysDeptTree)
	var rootDepts []*pb.SysDeptTree

	// 第一步：建立ID映射
	for _, dept := range depts {
		deptMap[dept.Id] = dept
	}

	// 第二步：构建父子关系
	for _, dept := range depts {
		if dept.ParentId == 0 {
			// 顶级部门
			rootDepts = append(rootDepts, dept)
		} else {
			// 子部门，找到其父部门并加入到父部门的children中
			if parent, exists := deptMap[dept.ParentId]; exists {
				parent.Children = append(parent.Children, dept)
			} else {
				// 父部门不存在，可能数据有问题，将其作为顶级部门处理
				l.Errorf("部门 %d (名称: %s) 的父部门 %d 不存在，将其作为顶级部门处理", dept.Id, dept.Name, dept.ParentId)
				dept.ParentId = 0 // 重置父ID
				rootDepts = append(rootDepts, dept)
			}
		}
	}

	// 第三步：对每个层级的部门按sort字段排序
	l.sortDeptTree(rootDepts)

	return rootDepts
}

// sortDeptTree 递归排序部门树（按sort字段升序）
func (l *DeptSearchLogic) sortDeptTree(depts []*pb.SysDeptTree) {
	if len(depts) <= 1 {
		return
	}

	// 使用冒泡排序按sort字段排序
	for i := 0; i < len(depts)-1; i++ {
		for j := 0; j < len(depts)-1-i; j++ {
			if depts[j].Sort > depts[j+1].Sort {
				depts[j], depts[j+1] = depts[j+1], depts[j]
			}
		}
	}

	// 递归排序子节点
	for _, dept := range depts {
		if len(dept.Children) > 0 {
			l.sortDeptTree(dept.Children)
		}
	}
}
