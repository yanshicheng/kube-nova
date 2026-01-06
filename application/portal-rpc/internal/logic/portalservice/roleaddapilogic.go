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

// RoleAddApi 角色 API 权限关联方法
func (l *RoleAddApiLogic) RoleAddApi(in *pb.AddSysRoleApiReq) (*pb.AddSysRoleApiResp, error) {
	// 参数验证
	if in.RoleId <= 0 {
		l.Errorf("设置角色 API 权限关联失败：角色 ID 无效")
		return nil, errorx.Msg("角色 ID 无效")
	}

	// 验证角色是否存在
	_, err := l.svcCtx.SysRole.FindOne(l.ctx, in.RoleId)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("设置角色 API 权限关联失败：角色不存在, roleId: %d", in.RoleId)
			return nil, errorx.Msg("角色不存在")
		}
		l.Errorf("查询角色信息失败: %v", err)
		return nil, errorx.Msg("查询角色信息失败")
	}

	// 验证所有 API ID 是否存在且为权限类型
	validApiIds := make([]uint64, 0, len(in.ApiIds))
	for _, apiId := range in.ApiIds {
		if apiId <= 0 {
			continue // 跳过无效的 API ID
		}

		api, err := l.svcCtx.SysApi.FindOne(l.ctx, apiId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Errorf("API 不存在，跳过, apiId: %d", apiId)
				continue
			}
			l.Errorf("查询 API 信息失败: %v", err)
			return nil, errorx.Msg("查询 API 信息失败")
		}

		// 只关联权限类型的 API，isPermission 为 1 表示权限类型
		if api.IsPermission != 1 {
			l.Errorf("API 不是权限类型，跳过, apiId: %d, isPermission: %d", apiId, api.IsPermission)
			continue
		}

		validApiIds = append(validApiIds, apiId)
	}

	// 使用事务处理角色 API 关联
	err = l.svcCtx.SysRoleApi.TransCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		// 删除该角色的所有现有 API 关联，同时清理缓存
		if err := l.svcCtx.SysRoleApi.DeleteByRoleIdWithCache(ctx, session, in.RoleId); err != nil {
			l.Errorf("删除角色 API 权限关联失败: %v", err)
			return err
		}

		// 添加新的 API 权限关联
		for _, apiId := range validApiIds {
			roleApi := &model.SysRoleApi{
				RoleId: in.RoleId,
				ApiId:  apiId,
			}
			if err := l.svcCtx.SysRoleApi.InsertWithSession(ctx, session, roleApi); err != nil {
				l.Errorf("添加角色 API 权限关联失败: %v", err)
				return err
			}
		}

		return nil
	})

	if err != nil {
		l.Errorf("设置角色 API 权限关联失败: %v", err)
		return nil, errorx.Msg("设置角色 API 权限关联失败")
	}

	// 重新加载权限策略，确保权限变更立即生效
	if l.svcCtx.AuthzManager != nil {
		if err := l.svcCtx.AuthzManager.ReloadPolicy(l.ctx); err != nil {
			l.Errorf("重新加载权限策略失败: %v", err)
		} else {
			l.Info("权限策略重新加载成功")
		}
	}

	l.Infof("设置角色 API 权限关联成功，角色 ID: %d, 有效 API 权限数量: %d", in.RoleId, len(validApiIds))
	return &pb.AddSysRoleApiResp{}, nil
}
