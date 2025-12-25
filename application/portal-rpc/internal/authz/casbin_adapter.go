package authz

import (
	"context"
	"fmt"
	"strings"

	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
	model2 "github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/zeromicro/go-zero/core/logx"
)

// CasbinAdapter 从现有数据库表加载Casbin策略
// 使用 sys_role, sys_api, sys_role_api 三张表
type CasbinAdapter struct {
	sysRole    model2.SysRoleModel
	sysApi     model2.SysApiModel
	sysRoleApi model2.SysRoleApiModel
}

// NewCasbinAdapter 创建Adapter
func NewCasbinAdapter(
	sysRole model2.SysRoleModel,
	sysApi model2.SysApiModel,
	sysRoleApi model2.SysRoleApiModel,
) *CasbinAdapter {
	return &CasbinAdapter{
		sysRole:    sysRole,
		sysApi:     sysApi,
		sysRoleApi: sysRoleApi,
	}
}

// LoadPolicy 从数据库加载策略到Casbin
// 策略格式: p, role_code, api_path, http_method
func (a *CasbinAdapter) LoadPolicy(m model.Model) error {
	ctx := context.Background()

	// 1. 查询所有角色
	roles, err := a.sysRole.SearchNoPage(ctx, "", true, "")
	if err != nil {
		return fmt.Errorf("查询角色失败: %w", err)
	}

	logx.Infof("[Adapter] 开始加载策略，共 %d 个角色", len(roles))

	policyCount := 0

	// 2. 遍历每个角色
	for _, role := range roles {
		// 查询角色的API权限关联
		roleApis, err := a.sysRoleApi.SearchNoPage(ctx, "", true, "role_id = ?", role.Id)
		if err != nil {
			logx.Errorf("[Adapter] 查询角色 %s (ID=%d) 的API关联失败: %v",
				role.Code, role.Id, err)
			continue
		}

		if len(roleApis) == 0 {
			logx.Infof("[Adapter] 角色 %s 没有关联任何API权限", role.Code)
			continue
		}

		// 3. 提取API ID列表
		apiIds := make([]uint64, 0, len(roleApis))
		for _, ra := range roleApis {
			apiIds = append(apiIds, ra.ApiId)
		}

		// 4. 批量查询API详情
		apis, err := a.batchGetApis(ctx, apiIds)
		if err != nil {
			logx.Errorf("[Adapter] 批量查询API失败: %v", err)
			continue
		}

		// 5. 构建Casbin策略
		for _, api := range apis {
			// 只处理权限类型的API（is_permission = 1）
			// is_permission = 0 表示分组，不需要加载到Casbin
			if api.IsPermission != 1 {
				continue
			}

			// 策略格式: p, role_code, path, method
			// 示例: p, ADMIN, /portal/v1/user/*, POST
			line := fmt.Sprintf("p, %s, %s, %s", role.Code, api.Path, api.Method)

			// 加载到Casbin模型
			if err := persist.LoadPolicyLine(line, m); err != nil {
				logx.Errorf("[Adapter] 加载策略失败: %s, error: %v", line, err)
				continue
			}

			policyCount++
		}

		logx.Infof("[Adapter] 角色 %s 加载了 %d 个API权限", role.Code, len(apis))
	}

	logx.Infof("[Adapter] 策略加载完成，共 %d 条策略规则", policyCount)
	return nil
}

// SavePolicy 保存策略（不需要实现，通过数据库表管理）
func (a *CasbinAdapter) SavePolicy(m model.Model) error {
	// 策略通过 sys_role_api 表管理，不需要保存到单独的casbin_rule表
	return nil
}

// AddPolicy 添加单条策略（不需要实现）
func (a *CasbinAdapter) AddPolicy(sec string, ptype string, rule []string) error {
	return nil
}

// RemovePolicy 删除单条策略（不需要实现）
func (a *CasbinAdapter) RemovePolicy(sec string, ptype string, rule []string) error {
	return nil
}

// RemoveFilteredPolicy 删除过滤策略（不需要实现）
func (a *CasbinAdapter) RemoveFilteredPolicy(sec string, ptype string, fieldIndex int, fieldValues ...string) error {
	return nil
}

// batchGetApis 批量查询API
func (a *CasbinAdapter) batchGetApis(ctx context.Context, apiIds []uint64) ([]*model2.SysApi, error) {
	if len(apiIds) == 0 {
		return nil, nil
	}

	// 构建 IN 查询
	// 示例: id IN (1, 2, 3)
	placeholders := make([]string, len(apiIds))
	args := make([]interface{}, len(apiIds))
	for i, id := range apiIds {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf("id IN (%s)", strings.Join(placeholders, ","))
	return a.sysApi.SearchNoPage(ctx, "", true, query, args...)
}

// 确保实现了persist.Adapter接口
var _ persist.Adapter = (*CasbinAdapter)(nil)
