package portalservicelogic

import (
	"context"
	"errors"
	"strings"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type RoleUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRoleUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleUpdateLogic {
	return &RoleUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// RoleUpdate 更新角色信息
func (l *RoleUpdateLogic) RoleUpdate(in *pb.UpdateSysRoleReq) (*pb.UpdateSysRoleResp, error) {

	// 参数验证
	if in.Id <= 0 {
		l.Errorf("更新角色失败：角色ID无效")
		return nil, errorx.Msg("角色ID无效")
	}

	if in.Name == "" {
		l.Errorf("更新角色失败：角色名称不能为空")
		return nil, errorx.Msg("角色名称不能为空")
	}

	if in.Code == "" {
		l.Errorf("更新角色失败：角色编码不能为空")
		return nil, errorx.Msg("角色编码不能为空")
	}

	// 查询原角色信息
	existingRole, err := l.svcCtx.SysRole.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("更新角色失败：角色不存在, roleId: %d", in.Id)
			return nil, errorx.Msg("角色不存在")
		}
		l.Errorf("查询角色信息失败: %v", err)
		return nil, errorx.Msg("查询角色信息失败")
	}

	// 角色编码格式验证和规范化
	roleCode := strings.ToLower(strings.TrimSpace(in.Code))
	if len(roleCode) < 2 || len(roleCode) > 50 {
		l.Errorf("更新角色失败：角色编码长度必须在2-50字符之间")
		return nil, errorx.Msg("角色编码长度必须在2-50字符之间")
	}

	// 检查角色编码是否重复（排除自己）
	if roleCode != existingRole.Code {
		checkRole, err := l.svcCtx.SysRole.FindOneByCode(l.ctx, roleCode)
		if err != nil && !errors.Is(err, model.ErrNotFound) {
			l.Errorf("检查角色编码重复失败: %v", err)
			return nil, errorx.Msg("检查角色编码重复失败")
		}
		if err == nil && checkRole.Id != in.Id {
			l.Errorf("更新角色失败：角色编码已存在, code: %s", roleCode)
			return nil, errorx.Msg("角色编码已存在")
		}
	}

	// 检查角色名称是否重复（排除自己）
	if in.Name != existingRole.Name {
		existingRoles, err := l.svcCtx.SysRole.SearchNoPage(l.ctx, "", true, "`name` = ? AND `id` != ?", in.Name, in.Id)
		if err != nil && !errors.Is(err, model.ErrNotFound) {
			l.Errorf("检查角色名称重复失败: %v", err)
			return nil, errorx.Msg("检查角色名称重复失败")
		}
		if len(existingRoles) > 0 {
			l.Errorf("更新角色失败：角色名称已存在, name: %s", in.Name)
			return nil, errorx.Msg("角色名称已存在")
		}
	}

	// 更新角色信息
	updatedRole := &model.SysRole{
		Id:        in.Id,
		Name:      in.Name,
		Code:      roleCode, // 使用规范化后的编码
		Remark:    in.Remark,
		UpdatedBy: in.UpdatedBy,
	}

	err = l.svcCtx.SysRole.Update(l.ctx, updatedRole)
	if err != nil {
		l.Errorf("更新角色失败: %v", err)
		return nil, errorx.Msg("更新角色失败")
	}

	l.Infof("更新角色成功，角色ID: %d, 角色名称: %s, 角色编码: %s, 更新人: %s",
		in.Id, in.Name, roleCode, in.UpdatedBy)
	return &pb.UpdateSysRoleResp{}, nil
}
