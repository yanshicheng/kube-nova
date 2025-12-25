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

type DeptDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeptDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeptDelLogic {
	return &DeptDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// DeptDel 删除部门
func (l *DeptDelLogic) DeptDel(in *pb.DelSysDeptReq) (*pb.DelSysDeptResp, error) {

	// 参数验证
	if in.Id <= 0 {
		l.Errorf("删除部门失败：部门ID无效")
		return nil, errorx.Msg("部门ID无效")
	}

	// 检查部门是否存在
	existingDept, err := l.svcCtx.SysDept.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("删除部门失败：部门不存在, deptId: %d", in.Id)
			return nil, errorx.Msg("部门不存在")
		}
		l.Errorf("查询部门信息失败: %v", err)
		return nil, errorx.Msg("查询部门信息失败")
	}

	// 检查是否有子部门
	subDepts, err := l.svcCtx.SysDept.SearchNoPage(l.ctx, "", true, "`parent_id` = ?", in.Id)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		l.Errorf("检查子部门失败: %v", err)
		return nil, errorx.Msg("检查子部门失败")
	}
	if len(subDepts) > 0 {
		l.Errorf("删除部门失败：存在子部门，不能删除, deptId: %d, subDeptCount: %d", in.Id, len(subDepts))
		return nil, errorx.Msg("该部门下存在子部门，请先删除子部门")
	}

	// 检查是否有用户关联到此部门
	userDepts, err := l.svcCtx.SysUserDept.SearchNoPage(l.ctx, "", true, "`dept_id` = ?", in.Id)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		l.Errorf("检查部门用户关联失败: %v", err)
		return nil, errorx.Msg("检查部门用户关联失败")
	}
	if len(userDepts) > 0 {
		l.Errorf("删除部门失败：有用户属于此部门，不能删除, deptId: %d, userCount: %d", in.Id, len(userDepts))
		return nil, errorx.Msg("该部门下存在用户，请先转移用户到其他部门")
	}

	// 检查是否有用户直接设置了此部门ID
	users, err := l.svcCtx.SysUser.SearchNoPage(l.ctx, "", true, "`dept_id` = ?", in.Id)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		l.Errorf("检查部门关联用户失败: %v", err)
		return nil, errorx.Msg("检查部门关联用户失败")
	}
	if len(users) > 0 {
		l.Errorf("删除部门失败：有用户直接关联此部门，不能删除, deptId: %d, userCount: %d", in.Id, len(users))
		return nil, errorx.Msg("该部门下存在用户，请先转移用户到其他部门")
	}

	// 执行软删除
	err = l.svcCtx.SysDept.DeleteSoft(l.ctx, in.Id)
	if err != nil {
		l.Errorf("删除部门失败: %v", err)
		return nil, errorx.Msg("删除部门失败")
	}

	l.Infof("删除部门成功，部门ID: %d, 部门名称: %s", in.Id, existingDept.Name)
	return &pb.DelSysDeptResp{}, nil
}
