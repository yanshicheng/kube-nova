// casbin_service.go
package casbinadapter

import (
	"fmt"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
)

// CasbinService 封装了 Casbin 的 Enforcer
type CasbinService struct {
	Enforcer *casbin.Enforcer
}

// NewCasbinService 初始化 CasbinService
func NewCasbinService(modelPath, dsn string) (*CasbinService, error) {
	// 加载 Casbin 模型
	m, err := model.NewModelFromFile(modelPath)
	if err != nil {
		return nil, fmt.Errorf("无法加载模型文件: %v", err)
	}

	// 创建自定义适配器
	adapter, err := NewCustomAdapter(dsn)
	if err != nil {
		return nil, fmt.Errorf("无法创建适配器: %v", err)
	}

	// 创建 Enforcer
	enforcer, err := casbin.NewEnforcer(m, adapter)
	if err != nil {
		return nil, fmt.Errorf("无法创建 enforcer: %v", err)
	}

	// 加载策略
	if err := enforcer.LoadPolicy(); err != nil {
		return nil, fmt.Errorf("无法加载策略: %v", err)
	}

	return &CasbinService{Enforcer: enforcer}, nil
}
