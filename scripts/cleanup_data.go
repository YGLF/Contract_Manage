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

	// 清理测试数据
	db.Exec("DELETE FROM approval_workflows WHERE contract_id >= 5")
	db.Exec("DELETE FROM workflow_approvals WHERE contract_id >= 5")
	db.Exec("DELETE FROM contracts WHERE contract_no LIKE 'CT-2026-%'")
	db.Exec("DELETE FROM customers WHERE code IN ('HW001', 'ALY001', 'TX001', 'JD001', 'BD001')")
	db.Exec("DELETE FROM contract_types WHERE code IN ('SOFTWARE', 'INTEGRATION', 'CONSULTING', 'MAINTENANCE', 'PROCUREMENT')")
	db.Exec("DELETE FROM users WHERE username IN ('manager_wang', 'director_li', 'admin_zhang', 'sales_zhao', 'finance_chen')")

	fmt.Println("已清理测试数据")
}
