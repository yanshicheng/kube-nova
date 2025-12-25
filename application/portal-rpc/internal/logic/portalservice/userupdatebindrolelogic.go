package portalservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"

	"github.com/zeromicro/go-zero/core/logx"
)

type UserUpdateBindRoleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUserUpdateBindRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserUpdateBindRoleLogic {
	return &UserUpdateBindRoleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UserUpdateBindRole 用户绑定角色
func (l *UserUpdateBindRoleLogic) UserUpdateBindRole(in *pb.UpdateSysUserBindRoleReq) (*pb.UpdateSysUserBindRoleResp, error) {
	// 参数验证
	if in.Id == 0 {
		l.Error("用户ID不能为空")
		return nil, errorx.Msg("用户ID不能为空")
	}

	// 验证用户是否存在
	_, err := l.svcCtx.SysUser.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("用户不存在，用户ID: %d", in.Id)
			return nil, errorx.Msg("用户不存在")
		}
		l.Errorf("查询用户失败，用户ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("查询用户失败")
	}

	// 预先验证所有角色是否存在，收集有效的角色ID
	validRoleIds := make([]uint64, 0, len(in.RoleIds))
	for _, roleId := range in.RoleIds {
		if roleId <= 0 {
			continue // 跳过无效的角色ID
		}

		_, err := l.svcCtx.SysRole.FindOne(l.ctx, roleId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Errorf("角色不存在，跳过，角色ID: %d", roleId)
				continue
			}
			l.Errorf("查询角色失败，角色ID: %d, 错误: %v", roleId, err)
			return nil, errorx.Msg("查询角色失败")
		}
		validRoleIds = append(validRoleIds, roleId)
	}

	// 使用事务处理
	err = l.svcCtx.SysUserRole.TransCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		// 1. 删除该用户的所有角色关联（同时清理缓存）
		if err := l.svcCtx.SysUserRole.DeleteByUserIdWithCache(ctx, session, in.Id); err != nil {
			l.Errorf("删除用户角色关联失败，用户ID: %d, 错误: %v", in.Id, err)
			return err
		}

		// 2. 添加新的角色关联
		for _, roleId := range validRoleIds {
			userRole := &model.SysUserRole{
				UserId: in.Id,
				RoleId: roleId,
			}

			if err := l.svcCtx.SysUserRole.InsertWithSession(ctx, session, userRole); err != nil {
				l.Errorf("插入用户角色关联失败，用户ID: %d, 角色ID: %d, 错误: %v", in.Id, roleId, err)
				return err
			}
		}

		return nil
	})

	if err != nil {
		l.Errorf("用户角色绑定失败，用户ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("用户角色绑定失败")
	}

	l.Infof("用户角色绑定成功，用户ID: %d, 有效角色数量: %d", in.Id, len(validRoleIds))
	return &pb.UpdateSysUserBindRoleResp{}, nil
}
