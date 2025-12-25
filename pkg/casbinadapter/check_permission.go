// check_permission.go
package casbinadapter

import (
	"log"
)

// CheckPermission 检查用户是否有权限
// userRoles: 当前用户的角色列表
// obj: 资源路径
// act: 动作
func (c *CasbinService) CheckPermission(userRoles []string, obj string, act string) bool {
	for _, role := range userRoles {
		allowed, err := c.Enforcer.Enforce(role, obj, act)
		if err != nil {
			log.Printf("权限检查失败: %v", err)
			continue
		}
		if allowed {
			return true
		}
	}
	return false
}
