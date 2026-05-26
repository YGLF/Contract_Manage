//go:build operational_scripts
// +build operational_scripts

package main

import (
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	dsn := mustOperationalDSN()
	db, _ := gorm.Open(mysql.Open(dsn), &gorm.Config{})

	// 查找合同
	var contracts []struct {
		ID     uint
		No     string
		Title  string
		Status string
	}
	db.Raw("SELECT id, contract_no as no, title, status FROM contracts WHERE contract_no LIKE '%CT2026%' OR contract_no LIKE '%202604%'").Scan(&contracts)

	fmt.Println("合同列表:")
	for _, c := range contracts {
		fmt.Printf("  ID: %d, No: %s, Title: %s, Status: %s\n", c.ID, c.No, c.Title, c.Status)
	}

	// 查找工作流
	var workflows []struct {
		ID           uint
		ContractID   uint
		Status       string
		CurrentLevel int
	}
	db.Raw("SELECT id, contract_id, status, current_level FROM approval_workflows").Scan(&workflows)

	fmt.Println("\n工作流列表:")
	for _, w := range workflows {
		fmt.Printf("  ID: %d, ContractID: %d, Status: %s, CurrentLevel: %d\n", w.ID, w.ContractID, w.Status, w.CurrentLevel)
	}

	// 查找审批节点
	var nodes []struct {
		ID     uint
		WfID   uint
		Level  int
		Status string
		Role   string
	}
	db.Raw("SELECT id, workflow_id as wf_id, level, status, approver_role FROM workflow_approvals").Scan(&nodes)

	fmt.Println("\n审批节点:")
	for _, n := range nodes {
		fmt.Printf("  ID: %d, WfID: %d, Level: %d, Status: %s, Role: %s\n", n.ID, n.WfID, n.Level, n.Status, n.Role)
	}

	// 查找用户
	var users []struct {
		ID       uint
		Username string
		Role     string
	}
	db.Raw("SELECT id, username, role FROM users WHERE role LIKE '%director%' OR username = 'sales_zhao'").Scan(&users)

	fmt.Println("\n相关用户:")
	for _, u := range users {
		fmt.Printf("  ID: %d, Username: %s, Role: %s\n", u.ID, u.Username, u.Role)
	}
}
