package portalservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type UserSearchRoleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUserSearchRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserSearchRoleLogic {
	return &UserSearchRoleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 通过用户ID 查询用户角色列表
// 通过用户ID 查询用户角色列表
func (l *UserSearchRoleLogic) UserSearchRole(in *pb.SearchSysUserRoleReq) (*pb.SearchSysUserRoleResp, error) {
	// 验证用户ID
	if in.UserId == 0 {
		l.Logger.Error("用户ID不能为空")
		return &pb.SearchSysUserRoleResp{}, nil
	}

	// 查询用户角色关联关系
	userRoles, err := l.svcCtx.SysUserRole.SearchNoPage(
		l.ctx,
		"",            // orderStr
		true,          // isAsc
		"user_id = ?", // queryStr
		in.UserId,
	)
	if err != nil {
		l.Logger.Errorf("查询用户角色关联失败: %v", err)
		return &pb.SearchSysUserRoleResp{}, nil
	}

	// 如果没有找到角色关联，返回空结果
	if len(userRoles) == 0 {
		l.Logger.Infof("用户ID %d 没有关联任何角色", in.UserId)
		return &pb.SearchSysUserRoleResp{
			RoleNames: []string{},
			RoleIds:   []uint64{},
		}, nil
	}

	// 收集角色名称和ID
	var roleNames []string
	var roleIds []uint64

	// 遍历用户角色关联，获取角色详情
	for _, userRole := range userRoles {
		// 根据角色ID查询角色详情
		role, err := l.svcCtx.SysRole.FindOne(l.ctx, userRole.RoleId)
		if err != nil {
			l.Logger.Errorf("查询角色详情失败, roleId: %d, error: %v", userRole.RoleId, err)
			continue // 跳过查询失败的角色，继续处理其他角色
		}

		// 添加角色名称和ID到结果集
		roleNames = append(roleNames, role.Name)
		roleIds = append(roleIds, role.Id)
	}

	l.Logger.Infof("用户ID %d 查询到 %d 个角色", in.UserId, len(roleNames))

	return &pb.SearchSysUserRoleResp{
		RoleNames: roleNames,
		RoleIds:   roleIds,
	}, nil
}
