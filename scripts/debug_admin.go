//go:build operational_scripts
// +build operational_scripts

package main

import (
	"fmt"
	"log"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	dsn := mustOperationalDSN()
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("连接数据库失败:", err)
	}

	// 检查admin用户的角色详情
	fmt.Println("=== admin用户详情 ===")
	var user struct {
		ID       uint
		Username string
		Role     string
		FullName string
	}
	db.Raw("SELECT id, username, role, full_name FROM users WHERE username = 'admin'").Scan(&user)
	fmt.Printf("  ID: %d\n", user.ID)
	fmt.Printf("  Username: %s\n", user.Username)
	fmt.Printf("  Role: '%s'\n", user.Role)
	fmt.Printf("  FullName: %s\n", user.FullName)

	// 测试查询
	fmt.Println("\n=== 测试管理员查询 ===")
	var count int64

	// 查询所有待审批
	db.Raw(`SELECT COUNT(*) FROM workflow_approvals wa 
		JOIN approval_workflows w ON w.id = wa.workflow_id
		WHERE wa.status = 'pending' AND w.status = 'pending' AND w.current_level = wa.level`).Scan(&count)
	fmt.Printf("所有待审批: %d\n", count)

	// 按角色查询
	db.Raw(`SELECT COUNT(*) FROM workflow_approvals wa 
		JOIN approval_workflows w ON w.id = wa.workflow_id
		WHERE wa.status = 'pending' AND wa.approver_role = 'sales_director' 
		AND w.status = 'pending' AND w.current_level = wa.level`).Scan(&count)
	fmt.Printf("销售总监待审批: %d\n", count)
}
