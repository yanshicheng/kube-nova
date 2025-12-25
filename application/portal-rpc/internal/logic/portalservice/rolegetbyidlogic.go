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

type RoleGetByIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRoleGetByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleGetByIdLogic {
	return &RoleGetByIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// RoleGetById 根据ID获取角色信息
func (l *RoleGetByIdLogic) RoleGetById(in *pb.GetSysRoleByIdReq) (*pb.GetSysRoleByIdResp, error) {

	// 参数验证
	if in.Id <= 0 {
		l.Errorf("查询角色失败：角色ID无效")
		return nil, errorx.Msg("角色ID无效")
	}

	// 查询角色信息
	role, err := l.svcCtx.SysRole.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("查询角色失败：角色不存在, roleId: %d", in.Id)
			return nil, errorx.Msg("角色不存在")
		}
		l.Errorf("查询角色信息失败: %v", err)
		return nil, errorx.Msg("查询角色信息失败")
	}

	// 转换为响应格式
	pbRole := l.convertToPbRole(role)

	return &pb.GetSysRoleByIdResp{
		Data: pbRole,
	}, nil
}

// convertToPbRole 将数据库模型转换为protobuf格式
func (l *RoleGetByIdLogic) convertToPbRole(role *model.SysRole) *pb.SysRole {
	return &pb.SysRole{
		Id:        role.Id,
		Name:      role.Name,
		Code:      role.Code,
		Remark:    role.Remark,
		CreatedBy: role.CreatedBy,
		UpdatedBy: role.UpdatedBy,
		CreatedAt: role.CreatedAt.Unix(),
		UpdatedAt: role.UpdatedAt.Unix(),
	}
}
