package authz

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	model2 "github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

// 定义白名单，这些路径不需要权限验证
var whiteList = []string{
	"/portal/v1/user/info",
	"/portal/v1/user/logout",
	"/portal/v1/auth/login",
	"/portal/v1/menu/roles/tree",
	"/ws/v1/site-messages",
}

// SuperAdminRole 是超级管理员角色代码
const (
	SuperAdminRole = "SUPER_ADMIN"
)

// CasbinRBACManager 基于 Casbin 的 RBAC 权限管理器
type CasbinRBACManager struct {
	enforcer    *casbin.SyncedEnforcer // Casbin 同步执行器，线程安全
	adapter     *CasbinAdapter         // 自定义适配器，从数据库加载策略
	watcher     *RedisWatcher          // Redis 监听器，用于分布式同步
	sysRole     model2.SysRoleModel    // 角色数据模型
	sysApi      model2.SysApiModel     // API 数据模型
	reloadMutex sync.Mutex             // 策略重载互斥锁，防止并发重载
}

// NewCasbinRBACManager 创建 RBAC 管理器
func NewCasbinRBACManager(
	redisConf redis.RedisConf,
	sysRole model2.SysRoleModel,
	sysApi model2.SysApiModel,
	sysRoleApi model2.SysRoleApiModel,
) (*CasbinRBACManager, error) {
	// 创建 Casbin 模型
	// RBAC 模型定义说明:
	//   - r: 请求定义，包含主体、对象、动作
	//   - p: 策略定义，包含主体、对象、动作
	//   - e: 效果定义，有任一允许则通过
	//   - m: 匹配器，支持通配符和 RESTful 路径匹配
	m := model.NewModel()
	m.AddDef("r", "r", "sub, obj, act")
	m.AddDef("p", "p", "sub, obj, act")
	m.AddDef("e", "e", "some(where (p.eft == allow))")

	// 匹配器说明:
	//   - keyMatch2: 支持 RESTful 路径匹配，如 /portal/v1/user/* 可以匹配 /portal/v1/user/123
	//   - r.act == p.act || p.act == "*": 支持方法通配符，* 可以匹配所有方法
	m.AddDef("m", "m", "r.sub == p.sub && keyMatch2(r.obj, p.obj) && (r.act == p.act || p.act == \"*\")")

	// 创建适配器，从现有数据库表加载策略
	adapter := NewCasbinAdapter(sysRole, sysApi, sysRoleApi)

	// 创建 SyncedEnforcer，支持并发安全的权限检查
	enforcer, err := casbin.NewSyncedEnforcer(m, adapter)
	if err != nil {
		return nil, fmt.Errorf("创建 Enforcer 失败: %w", err)
	}

	// 创建 Redis Watcher，用于分布式策略同步
	watcher, err := NewRedisWatcher(redisConf)
	if err != nil {
		return nil, fmt.Errorf("创建 Watcher 失败: %w", err)
	}

	// 设置 Watcher 到 Enforcer
	if err := enforcer.SetWatcher(watcher); err != nil {
		return nil, fmt.Errorf("设置 Watcher 失败: %w", err)
	}

	// 创建管理器实例
	manager := &CasbinRBACManager{
		enforcer: enforcer,
		adapter:  adapter,
		watcher:  watcher,
		sysRole:  sysRole,
		sysApi:   sysApi,
	}

	// 设置更新回调：收到 Redis 通知后重新加载策略
	if err := watcher.SetUpdateCallback(func(msg string) {
		logx.Infof("[RBAC] 收到策略更新通知: %s，准备重新加载策略", msg)
		// 使用内部方法重载，避免重复发送通知
		if err := manager.reloadPolicyInternal(); err != nil {
			logx.Errorf("[RBAC] 重新加载策略失败: %v", err)
		} else {
			logx.Info("[RBAC] 策略重新加载成功")
		}
	}); err != nil {
		return nil, fmt.Errorf("设置更新回调失败: %w", err)
	}

	// 启动 Watcher，开始监听 Redis 频道
	if err := watcher.Start(); err != nil {
		return nil, fmt.Errorf("启动 Watcher 失败: %w", err)
	}

	// 首次加载策略
	if err := enforcer.LoadPolicy(); err != nil {
		return nil, fmt.Errorf("加载策略失败: %w", err)
	}

	logx.Info("[RBAC] Casbin RBAC 管理器初始化成功，支持分布式模式")

	return manager, nil
}

// CheckPermission 检查权限
func (m *CasbinRBACManager) CheckPermission(ctx context.Context, userRoles []string, path string, method string) (bool, error) {
	// 检查白名单
	if m.isInWhiteList(path, method) {
		logx.WithContext(ctx).Infof("[RBAC] 请求在白名单中，允许访问: %s %s", method, path)
		return true, nil
	}

	// 没有角色，直接拒绝
	if len(userRoles) == 0 {
		logx.WithContext(ctx).Infof("[RBAC] 用户没有任何角色，拒绝访问: %s %s", method, path)
		return false, nil
	}

	// 检查是否包含超级管理员
	for _, role := range userRoles {
		if strings.EqualFold(strings.ToUpper(role), SuperAdminRole) {
			logx.WithContext(ctx).Infof("[RBAC] 用户是超级管理员(%s)，允许访问: %s %s",
				role, method, path)
			return true, nil
		}
	}

	// 使用 Casbin 检查权限
	// 遍历所有角色，只要有一个角色有权限就通过
	for _, role := range userRoles {
		allowed, err := m.enforcer.Enforce(role, path, method)
		if err != nil {
			logx.WithContext(ctx).Errorf("[RBAC] Casbin 检查权限失败: role=%s, path=%s, method=%s, error=%v",
				role, path, method, err)
			continue
		}

		if allowed {
			logx.WithContext(ctx).Infof("[RBAC] 角色 %s 有权限访问: %s %s", role, method, path)
			return true, nil
		}
	}

	// 所有角色都没有权限
	logx.WithContext(ctx).Infof("[RBAC] 角色 %v 无权限访问: %s %s", userRoles, method, path)
	return false, nil
}

// reloadPolicyInternal 内部策略重载方法
// 只重载本地策略，不发送 Redis 通知，用于响应其他实例的通知
func (m *CasbinRBACManager) reloadPolicyInternal() error {
	m.reloadMutex.Lock()
	defer m.reloadMutex.Unlock()

	return m.enforcer.LoadPolicy()
}

// ReloadPolicy 重新加载策略，权限变更后调用
func (m *CasbinRBACManager) ReloadPolicy(ctx context.Context) error {
	logx.WithContext(ctx).Info("[RBAC] 开始重新加载策略...")

	// 使用互斥锁防止并发重载
	m.reloadMutex.Lock()
	defer m.reloadMutex.Unlock()

	// 本地重新加载策略
	if err := m.enforcer.LoadPolicy(); err != nil {
		return fmt.Errorf("重新加载策略失败: %w", err)
	}

	// 通知其他实例，通过 Redis 发布订阅
	if err := m.watcher.Update(); err != nil {
		// 通知失败不影响本地重载，只记录日志
		logx.WithContext(ctx).Errorf("[RBAC] 通知其他实例失败: %v", err)
	}

	logx.WithContext(ctx).Info("[RBAC] 策略重新加载完成，已通知其他实例")
	return nil
}

// GetAllPolicies 获取所有策略，用于调试
func (m *CasbinRBACManager) GetAllPolicies() ([][]string, error) {
	return m.enforcer.GetPolicy()
}

// GetRolePolicies 获取指定角色的所有策略
func (m *CasbinRBACManager) GetRolePolicies(roleCode string) ([][]string, error) {
	return m.enforcer.GetFilteredPolicy(0, roleCode)
}

// GetUserPermissions 获取用户的所有权限，用于前端菜单渲染
func (m *CasbinRBACManager) GetUserPermissions(ctx context.Context, userRoles []string) ([]Permission, error) {
	// 检查是否包含超级管理员
	for _, role := range userRoles {
		if strings.EqualFold(role, SuperAdminRole) {
			// 超级管理员返回所有权限
			return m.getAllPermissions(ctx)
		}
	}

	// 合并所有角色的权限，使用 map 去重
	permMap := make(map[string]Permission)

	for _, role := range userRoles {
		policies, err := m.enforcer.GetFilteredPolicy(0, role)
		if err != nil {
			logx.WithContext(ctx).Errorf("[RBAC] 获取角色 %s 的策略失败: %v", role, err)
			continue
		}

		for _, policy := range policies {
			if len(policy) < 3 {
				continue
			}

			path := policy[1]
			method := policy[2]
			key := fmt.Sprintf("%s:%s", path, method)

			permMap[key] = Permission{
				Path:   path,
				Method: method,
			}
		}
	}

	// 转换为列表
	result := make([]Permission, 0, len(permMap))
	for _, perm := range permMap {
		result = append(result, perm)
	}

	logx.WithContext(ctx).Infof("[RBAC] 角色 %v 共有 %d 个权限", userRoles, len(result))
	return result, nil
}

// getAllPermissions 获取所有权限，超级管理员专用
func (m *CasbinRBACManager) getAllPermissions(ctx context.Context) ([]Permission, error) {
	// 查询所有权限类型的 API
	apis, err := m.sysApi.SearchNoPage(ctx, "", true, "is_permission = ?", 1)
	if err != nil {
		return nil, fmt.Errorf("查询所有 API 失败: %w", err)
	}

	result := make([]Permission, 0, len(apis))
	for _, api := range apis {
		result = append(result, Permission{
			Path:   api.Path,
			Method: api.Method,
		})
	}

	return result, nil
}

// BatchCheckPermission 批量检查权限，用于前端按钮权限
func (m *CasbinRBACManager) BatchCheckPermission(
	ctx context.Context,
	userRoles []string,
	permissions []Permission,
) (map[string]bool, error) {
	result := make(map[string]bool, len(permissions))

	for _, perm := range permissions {
		key := fmt.Sprintf("%s:%s", perm.Path, perm.Method)
		allowed, err := m.CheckPermission(ctx, userRoles, perm.Path, perm.Method)
		if err != nil {
			logx.WithContext(ctx).Errorf("[RBAC] 检查权限 %s 失败: %v", key, err)
			result[key] = false
			continue
		}
		result[key] = allowed
	}

	return result, nil
}

// Close 关闭管理器，释放资源
func (m *CasbinRBACManager) Close() {
	if m.watcher != nil {
		m.watcher.Close()
	}
	logx.Info("[RBAC] RBAC 管理器已关闭")
}

// Permission 权限结构
type Permission struct {
	Path   string `json:"path"`   // API 路径
	Method string `json:"method"` // HTTP 方法
}

// isInWhiteList 检查请求是否在白名单中
func (m *CasbinRBACManager) isInWhiteList(path string, method string) bool {
	for _, whitePath := range whiteList {
		if strings.HasPrefix(path, whitePath) {
			return true
		}
	}
	return false
}
