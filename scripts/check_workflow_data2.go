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

	fmt.Println("=== 直接查询数据库 ===")

	rows, err := db.Raw("SELECT id, workflow_id, contract_id, approver_role, level, status FROM workflow_approvals").Rows()
	if err != nil {
		fmt.Printf("错误: %v\n", err)
		return
	}
	defer rows.Close()

	fmt.Println("ID | workflow_id | contract_id | approver_role | level | status")
	fmt.Println("---+-------------+-------------+---------------+-------+--------")
	for rows.Next() {
		var id, workflow_id, contract_id, level int
		var approver_role, status string
		rows.Scan(&id, &workflow_id, &contract_id, &approver_role, &level, &status)
		fmt.Printf("%d | %d | %d | %s | %d | %s\n", id, workflow_id, contract_id, approver_role, level, status)
	}

	fmt.Println("\n=== 查询待审批（带current_level检查）===")
	rows2, err := db.Raw(`
		SELECT wa.id, wa.workflow_id, wa.contract_id, wa.approver_role, wa.level, wa.status, w.current_level
		FROM workflow_approvals wa 
		JOIN approval_workflows w ON w.id = wa.workflow_id
		WHERE wa.status = 'pending' AND wa.approver_role = 'sales_director' 
		AND w.status = 'pending' AND w.current_level = wa.level
	`).Rows()
	if err != nil {
		fmt.Printf("错误: %v\n", err)
		return
	}
	defer rows2.Close()

	fmt.Println("销售总监待审批:")
	count := 0
	for rows2.Next() {
		var id, workflow_id, contract_id, level, current_level int
		var approver_role, status string
		rows2.Scan(&id, &workflow_id, &contract_id, &approver_role, &level, &status, &current_level)
		fmt.Printf("  - ID=%d, workflow_id=%d, contract_id=%d, level=%d, current_level=%d\n",
			id, workflow_id, contract_id, level, current_level)
		count++
	}
	fmt.Printf("总计: %d 条\n", count)
}
