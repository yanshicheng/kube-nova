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

type RoleSearchApiLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRoleSearchApiLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleSearchApiLogic {
	return &RoleSearchApiLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 查询关联了哪些 api权限
func (l *RoleSearchApiLogic) RoleSearchApi(in *pb.SearchSysRoleApiReq) (*pb.SearchSysRoleApiResp, error) {

	// 参数验证
	if in.RoleId <= 0 {
		l.Errorf("查询角色关联API权限失败：角色ID无效")
		return nil, errorx.Msg("角色ID无效")
	}

	// 验证角色是否存在
	_, err := l.svcCtx.SysRole.FindOne(l.ctx, in.RoleId)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("查询角色关联API权限失败：角色不存在, roleId: %d", in.RoleId)
			return nil, errorx.Msg("角色不存在")
		}
		l.Errorf("查询角色信息失败: %v", err)
		return nil, errorx.Msg("查询角色信息失败")
	}

	// 查询角色关联的API ID列表
	roleApis, err := l.svcCtx.SysRoleApi.SearchNoPage(l.ctx, "", true, "`role_id` = ?", in.RoleId)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		l.Errorf("查询角色API权限关联失败: %v", err)
		return nil, errorx.Msg("查询角色API权限关联失败")
	}

	// 提取API ID列表并验证有效性
	var apiIds []uint64
	validApiIds := make([]uint64, 0, len(roleApis))

	for _, roleApi := range roleApis {
		apiIds = append(apiIds, roleApi.ApiId)

		// 验证API是否还存在且为权限类型（清理无效关联）
		api, err := l.svcCtx.SysApi.FindOne(l.ctx, roleApi.ApiId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Errorf("API不存在，关联数据可能已过期, apiId: %d", roleApi.ApiId)
				// 可以选择在这里删除无效的关联关系
				if delErr := l.svcCtx.SysRoleApi.DeleteSoft(l.ctx, roleApi.Id); delErr != nil {
					l.Errorf("删除无效的角色API权限关联失败: %v", delErr)
				}
				continue
			}
			l.Errorf("查询API信息失败: %v", err)
			return nil, errorx.Msg("查询API信息失败")
		}

		// 检查API是否为权限类型（isPermission=1）
		if api.IsPermission != 1 {
			l.Errorf("API不是权限类型，关联数据可能有误, apiId: %d, isPermission: %d", roleApi.ApiId, api.IsPermission)
			// 可以选择在这里删除非权限类型的关联关系
			if delErr := l.svcCtx.SysRoleApi.DeleteSoft(l.ctx, roleApi.Id); delErr != nil {
				l.Errorf("删除非权限类型的角色API关联失败: %v", delErr)
			}
			continue
		}

		validApiIds = append(validApiIds, roleApi.ApiId)
	}

	return &pb.SearchSysRoleApiResp{
		ApiIds: validApiIds, // 只返回有效的API权限ID
	}, nil
}
