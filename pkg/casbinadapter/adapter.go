// adapter.go
package casbinadapter

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/casbin/casbin/v2/model"
	//"github.com/casbin/casbin/v2/persist"
	_ "github.com/go-sql-driver/mysql"
)

// CustomAdapter 实现了 casbin.Adapter 接口，从自定义表加载策略
type CustomAdapter struct {
	db *sql.DB
}

// NewCustomAdapter 创建一个 CustomAdapter 实例
func NewCustomAdapter(dsn string) (*CustomAdapter, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("无法连接到数据库: %v", err)
	}
	// 测试数据库连接
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("无法连接到数据库: %v", err)
	}
	return &CustomAdapter{db: db}, nil
}

// LoadPolicy 加载策略到 Casbin
func (a *CustomAdapter) LoadPolicy(model model.Model) error {
	query := `
        SELECT r.role_code, p.uri, p.action
        FROM sys_role_permission rp
        JOIN sys_role r ON rp.role_id = r.id
        JOIN sys_permission p ON rp.permission_id = p.id
        WHERE rp.delete_time IS NULL AND r.delete_time IS NULL AND p.delete_time IS NULL
    `

	rows, err := a.db.Query(query)
	if err != nil {
		return fmt.Errorf("查询策略失败: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var role, uri, action string
		if err := rows.Scan(&role, &uri, &action); err != nil {
			return fmt.Errorf("扫描行失败: %v", err)
		}

		// 将策略添加到 Casbin 的模型中
		line := fmt.Sprintf("p, %s, %s, %s", role, uri, action)
		fields := strings.Split(line, ", ")
		if len(fields) != 4 || fields[0] != "p" {
			log.Printf("跳过无效的策略行: %s", line)
			continue
		}
		err := model.AddPolicy(fields[0], fields[1], fields[2:])
		if err != nil {
			return err
		}
	}

	return nil
}

// SavePolicy 实现 Adapter 接口，但这里不支持保存策略
func (a *CustomAdapter) SavePolicy(model model.Model) error {
	return fmt.Errorf("不支持保存策略")
}

// AddPolicy 实现 Adapter 接口，但这里不支持动态添加策略
func (a *CustomAdapter) AddPolicy(sec string, ptype string, rule []string) error {
	return fmt.Errorf("不支持添加策略")
}

// RemovePolicy 实现 Adapter 接口，但这里不支持动态移除策略
func (a *CustomAdapter) RemovePolicy(sec string, ptype string, rule []string) error {
	return fmt.Errorf("不支持移除策略")
}

// RemoveFilteredPolicy 实现 Adapter 接口，但这里不支持过滤移除策略
func (a *CustomAdapter) RemoveFilteredPolicy(sec string, ptype string, fieldIndex int, fieldValues ...string) error {
	return fmt.Errorf("不支持移除过滤策略")
}
