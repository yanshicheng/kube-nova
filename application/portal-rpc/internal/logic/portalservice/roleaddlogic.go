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

type RoleAddLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRoleAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleAddLogic {
	return &RoleAddLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// -----------------------角色表-----------------------
func (l *RoleAddLogic) RoleAdd(in *pb.AddSysRoleReq) (*pb.AddSysRoleResp, error) {

	// 参数验证
	if in.Name == "" {
		l.Errorf("添加角色失败：角色名称不能为空")
		return nil, errorx.Msg("角色名称不能为空")
	}

	if in.Code == "" {
		l.Errorf("添加角色失败：角色编码不能为空")
		return nil, errorx.Msg("角色编码不能为空")
	}

	// 角色编码格式验证和规范化
	roleCode := strings.ToLower(strings.TrimSpace(in.Code))
	if len(roleCode) < 2 || len(roleCode) > 50 {
		l.Errorf("添加角色失败：角色编码长度必须在2-50字符之间")
		return nil, errorx.Msg("角色编码长度必须在2-50字符之间")
	}

	// 检查角色编码是否重复
	_, err := l.svcCtx.SysRole.FindOneByCode(l.ctx, roleCode)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		l.Errorf("检查角色编码重复失败: %v", err)
		return nil, errorx.Msg("检查角色编码重复失败")
	}
	if err == nil {
		l.Errorf("添加角色失败：角色编码已存在, code: %s", roleCode)
		return nil, errorx.Msg("角色编码已存在")
	}

	// 检查角色名称是否重复
	existingRoles, err := l.svcCtx.SysRole.SearchNoPage(l.ctx, "", true, "`name` = ?", in.Name)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		l.Errorf("检查角色名称重复失败: %v", err)
		return nil, errorx.Msg("检查角色名称重复失败")
	}
	if len(existingRoles) > 0 {
		l.Errorf("添加角色失败：角色名称已存在, name: %s", in.Name)
		return nil, errorx.Msg("角色名称已存在")
	}

	// 构造角色数据
	role := &model.SysRole{
		Name:      in.Name,
		Code:      roleCode, // 使用规范化后的编码
		Remark:    in.Remark,
		CreatedBy: in.CreatedBy,
		UpdatedBy: in.UpdatedBy,
	}

	// 保存到数据库
	_, err = l.svcCtx.SysRole.Insert(l.ctx, role)
	if err != nil {
		l.Errorf("添加角色失败: %v", err)
		return nil, errorx.Msg("添加角色失败")
	}

	l.Infof("添加角色成功，角色名称: %s, 角色编码: %s, 创建人: %s",
		in.Name, roleCode, in.CreatedBy)
	return &pb.AddSysRoleResp{}, nil
}
