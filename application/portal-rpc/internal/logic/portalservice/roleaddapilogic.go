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

type RoleAddApiLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRoleAddApiLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleAddApiLogic {
	return &RoleAddApiLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// RoleAddApi 角色API权限关联方法
func (l *RoleAddApiLogic) RoleAddApi(in *pb.AddSysRoleApiReq) (*pb.AddSysRoleApiResp, error) {
	// 参数验证
	if in.RoleId <= 0 {
		l.Errorf("设置角色API权限关联失败：角色ID无效")
		return nil, errorx.Msg("角色ID无效")
	}

	// 验证角色是否存在
	_, err := l.svcCtx.SysRole.FindOne(l.ctx, in.RoleId)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("设置角色API权限关联失败：角色不存在, roleId: %d", in.RoleId)
			return nil, errorx.Msg("角色不存在")
		}
		l.Errorf("查询角色信息失败: %v", err)
		return nil, errorx.Msg("查询角色信息失败")
	}

	// 验证所有API ID是否存在且为权限类型
	validApiIds := make([]uint64, 0, len(in.ApiIds))
	for _, apiId := range in.ApiIds {
		if apiId <= 0 {
			continue // 跳过无效的API ID
		}

		api, err := l.svcCtx.SysApi.FindOne(l.ctx, apiId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Errorf("API不存在，跳过, apiId: %d", apiId)
				continue
			}
			l.Errorf("查询API信息失败: %v", err)
			return nil, errorx.Msg("查询API信息失败")
		}

		// 只关联权限类型的API（isPermission=1）
		if api.IsPermission != 1 {
			l.Errorf("API不是权限类型，跳过, apiId: %d, isPermission: %d", apiId, api.IsPermission)
			continue
		}

		validApiIds = append(validApiIds, apiId)
	}

	// 使用事务处理角色API关联
	err = l.svcCtx.SysRoleApi.TransCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		// 1. 删除该角色的所有现有API关联（同时清理缓存）
		if err := l.svcCtx.SysRoleApi.DeleteByRoleIdWithCache(ctx, session, in.RoleId); err != nil {
			l.Errorf("删除角色API权限关联失败: %v", err)
			return err
		}

		// 2. 添加新的API权限关联
		for _, apiId := range validApiIds {
			roleApi := &model.SysRoleApi{
				RoleId: in.RoleId,
				ApiId:  apiId,
			}
			if err := l.svcCtx.SysRoleApi.InsertWithSession(ctx, session, roleApi); err != nil {
				l.Errorf("添加角色API权限关联失败: %v", err)
				return err
			}
		}

		return nil
	})

	if err != nil {
		l.Errorf("设置角色API权限关联失败: %v", err)
		return nil, errorx.Msg("设置角色API权限关联失败")
	}

	l.Infof("设置角色API权限关联成功，角色ID: %d, 有效API权限数量: %d", in.RoleId, len(validApiIds))
	return &pb.AddSysRoleApiResp{}, nil
}
