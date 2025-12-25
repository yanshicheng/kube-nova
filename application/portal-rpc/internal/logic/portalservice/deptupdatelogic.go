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

type DeptUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeptUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeptUpdateLogic {
	return &DeptUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// DeptUpdate 更新部门信息
func (l *DeptUpdateLogic) DeptUpdate(in *pb.UpdateSysDeptReq) (*pb.UpdateSysDeptResp, error) {
	// 参数验证
	if in.Id <= 0 {
		l.Errorf("更新部门失败：部门ID无效")
		return nil, errorx.Msg("部门ID无效")
	}

	if in.Name == "" {
		l.Errorf("更新部门失败：部门名称不能为空")
		return nil, errorx.Msg("部门名称不能为空")
	}

	// 查询原部门信息
	existingDept, err := l.svcCtx.SysDept.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("更新部门失败：部门不存在, deptId: %d", in.Id)
			return nil, errorx.Msg("部门不存在")
		}
		l.Errorf("查询部门信息失败: %v", err)
		return nil, errorx.Msg("查询部门信息失败")
	}

	// 防止将部门设置为自己的子部门（循环引用检查）
	if in.ParentId == in.Id {
		l.Errorf("更新部门失败：不能将部门设置为自己的父部门, deptId: %d", in.Id)
		return nil, errorx.Msg("不能将部门设置为自己的父部门")
	}

	// 如果修改了父级部门，需要检查是否会造成循环引用
	if in.ParentId != existingDept.ParentId && in.ParentId > 0 {
		// 检查新的父级部门是否存在
		_, err := l.svcCtx.SysDept.FindOne(l.ctx, in.ParentId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Errorf("更新部门失败：新的父级部门不存在, parentId: %d", in.ParentId)
				return nil, errorx.Msg("新的父级部门不存在")
			}
			l.Errorf("查询新的父级部门失败: %v", err)
			return nil, errorx.Msg("查询新的父级部门失败")
		}

		// TODO: 这里可以添加更复杂的循环引用检查逻辑
		// 检查新的父级部门是否是当前部门的子部门
		if err := l.checkCircularReference(in.Id, in.ParentId); err != nil {
			l.Errorf("更新部门失败：检测到循环引用, deptId: %d, parentId: %d", in.Id, in.ParentId)
			return nil, errorx.Msg("不能将部门移动到其子部门下，这会造成循环引用")
		}
	}

	// 检查同级部门名称是否重复（排除自己）
	if in.Name != existingDept.Name || in.ParentId != existingDept.ParentId {
		args := []interface{}{in.ParentId, in.Name, in.Id}
		query := "`parent_id` = ? AND `name` = ? AND `id` != ?"

		existingDepts, err := l.svcCtx.SysDept.SearchNoPage(l.ctx, "", true, query, args...)
		if err != nil && !errors.Is(err, model.ErrNotFound) {
			l.Errorf("检查部门名称重复失败: %v", err)
			return nil, errorx.Msg("检查部门名称重复失败")
		}
		if len(existingDepts) > 0 {
			l.Errorf("更新部门失败：同级部门名称已存在, name: %s, parentId: %d", in.Name, in.ParentId)
			return nil, errorx.Msg("同级部门名称已存在")
		}
	}

	// 更新部门信息
	updatedDept := &model.SysDept{
		Id:        in.Id,
		Name:      in.Name,
		ParentId:  in.ParentId,
		Remark:    in.Remark,
		Leader:    in.Leader,
		Phone:     in.Phone,
		Email:     in.Email,
		Status:    in.Status,
		Sort:      in.Sort,
		UpdatedBy: in.UpdatedBy,
	}

	err = l.svcCtx.SysDept.Update(l.ctx, updatedDept)
	if err != nil {
		l.Errorf("更新部门失败: %v", err)
		return nil, errorx.Msg("更新部门失败")
	}

	l.Infof("更新部门成功，部门ID: %d, 部门名称: %s, 更新人: %s", in.Id, in.Name, in.UpdatedBy)
	return &pb.UpdateSysDeptResp{}, nil
}

// checkCircularReference 检查循环引用
// 检查 parentId 是否是 deptId 的子部门
func (l *DeptUpdateLogic) checkCircularReference(deptId, parentId uint64) error {
	// 获取 parentId 的所有父级部门，如果其中包含 deptId，则存在循环引用
	currentId := parentId
	visited := make(map[uint64]bool)

	for currentId > 0 {
		// 防止无限循环
		if visited[currentId] {
			break
		}
		visited[currentId] = true

		// 如果找到了目标部门，说明存在循环引用
		if currentId == deptId {
			return errorx.Msg("循环引用")
		}

		// 查找当前部门的父部门
		dept, err := l.svcCtx.SysDept.FindOne(l.ctx, currentId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				break
			}
			return err
		}
		currentId = dept.ParentId
	}

	return nil
}
