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

// CasbinAdapter 从现有数据库表加载 Casbin 策略
type CasbinAdapter struct {
	sysRole    model2.SysRoleModel    // 角色数据模型
	sysApi     model2.SysApiModel     // API 数据模型
	sysRoleApi model2.SysRoleApiModel // 角色 API 关联数据模型
}

// NewCasbinAdapter 创建适配器
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

// LoadPolicy 从数据库加载策略到 Casbin
// 策略格式: p, role_code, api_path, http_method
// 示例: p, ADMIN, /portal/v1/user/*, POST
func (a *CasbinAdapter) LoadPolicy(m model.Model) error {
	ctx := context.Background()

	// 查询所有角色
	roles, err := a.sysRole.SearchNoPage(ctx, "", true, "")
	if err != nil {
		return fmt.Errorf("查询角色失败: %w", err)
	}

	logx.Infof("[Adapter] 开始加载策略，共 %d 个角色", len(roles))

	policyCount := 0

	// 遍历每个角色
	for _, role := range roles {
		// 查询角色的 API 权限关联
		roleApis, err := a.sysRoleApi.SearchNoPage(ctx, "", true, "role_id = ?", role.Id)
		if err != nil {
			logx.Errorf("[Adapter] 查询角色 %s (ID=%d) 的 API 关联失败: %v",
				role.Code, role.Id, err)
			continue
		}

		if len(roleApis) == 0 {
			logx.Infof("[Adapter] 角色 %s 没有关联任何 API 权限", role.Code)
			continue
		}

		// 提取 API ID 列表
		apiIds := make([]uint64, 0, len(roleApis))
		for _, ra := range roleApis {
			apiIds = append(apiIds, ra.ApiId)
		}

		// 批量查询 API 详情
		apis, err := a.batchGetApis(ctx, apiIds)
		if err != nil {
			logx.Errorf("[Adapter] 批量查询 API 失败: %v", err)
			continue
		}

		// 构建 Casbin 策略
		for _, api := range apis {
			// 只处理权限类型的 API，is_permission 为 1 表示权限类型
			// is_permission 为 0 表示分组，不需要加载到 Casbin
			if api.IsPermission != 1 {
				continue
			}

			// 策略格式: p, role_code, path, method
			line := fmt.Sprintf("p, %s, %s, %s", role.Code, api.Path, api.Method)

			// 加载到 Casbin 模型
			if err := persist.LoadPolicyLine(line, m); err != nil {
				logx.Errorf("[Adapter] 加载策略失败: %s, 错误: %v", line, err)
				continue
			}

			policyCount++
		}

		logx.Infof("[Adapter] 角色 %s 加载了 %d 个 API 权限", role.Code, len(apis))
	}

	logx.Infof("[Adapter] 策略加载完成，共 %d 条策略规则", policyCount)
	return nil
}

// SavePolicy 保存策略
// 策略通过 sys_role_api 表管理，不需要保存到单独的 casbin_rule 表
func (a *CasbinAdapter) SavePolicy(m model.Model) error {
	return nil
}

// AddPolicy 添加单条策略
func (a *CasbinAdapter) AddPolicy(sec string, ptype string, rule []string) error {
	return nil
}

// RemovePolicy 删除单条策略
func (a *CasbinAdapter) RemovePolicy(sec string, ptype string, rule []string) error {
	return nil
}

// RemoveFilteredPolicy 删除过滤策略
func (a *CasbinAdapter) RemoveFilteredPolicy(sec string, ptype string, fieldIndex int, fieldValues ...string) error {
	return nil
}

// batchGetApis 批量查询 API
func (a *CasbinAdapter) batchGetApis(ctx context.Context, apiIds []uint64) ([]*model2.SysApi, error) {
	if len(apiIds) == 0 {
		return nil, nil
	}

	// 构建 IN 查询条件
	placeholders := make([]string, len(apiIds))
	args := make([]interface{}, len(apiIds))
	for i, id := range apiIds {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf("id IN (%s)", strings.Join(placeholders, ","))
	return a.sysApi.SearchNoPage(ctx, "", true, query, args...)
}

// 确保实现了 persist.Adapter 接口
var _ persist.Adapter = (*CasbinAdapter)(nil)
