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

	fmt.Println("=== 验证测试数据 ===\n")

	// 查询所有合同
	type Contract struct {
		ID         uint
		ContractNo string
		Title      string
		Status     string
		Amount     float64
	}

	var contracts []Contract
	db.Raw("SELECT id, contract_no, title, status, amount FROM contracts WHERE contract_no LIKE 'TEST-%'").Scan(&contracts)

	fmt.Println("合同列表:")
	for _, c := range contracts {
		fmt.Printf("  - %s: %s (状态: %s, ¥%.2f)\n", c.ContractNo, c.Title, c.Status, c.Amount)
	}

	// 查询工作流
	type Workflow struct {
		ID           uint
		ContractID   uint
		CurrentLevel int
		MaxLevel     int
		Status       string
	}

	var workflows []Workflow
	db.Raw("SELECT id, contract_id, current_level, max_level, status FROM approval_workflows").Scan(&workflows)

	fmt.Println("\n工作流列表:")
	for _, w := range workflows {
		fmt.Printf("  - ID=%d: contract_id=%d, level=%d/%d, status=%s\n",
			w.ID, w.ContractID, w.CurrentLevel, w.MaxLevel, w.Status)
	}

	// 查询审批节点
	type Node struct {
		ID           uint
		WfID         uint
		ContractID   uint
		ApproverRole string
		Level        int
		Status       string
	}

	var nodes []Node
	db.Raw("SELECT id, workflow_id, contract_id, approver_role, level, status FROM workflow_approvals ORDER BY workflow_id, level").Scan(&nodes)

	fmt.Println("\n审批节点:")
	for _, n := range nodes {
		fmt.Printf("  - 工作流#%d, 级别%d (%s): %s\n",
			n.WfID, n.Level, n.ApproverRole, n.Status)
	}

	// 统计
	fmt.Println("\n=== 各角色待审批统计 ===")
	var salesCount, techCount, financeCount int64
	db.Raw(`
		SELECT COUNT(*) FROM workflow_approvals wa 
		JOIN approval_workflows w ON w.id = wa.workflow_id
		WHERE wa.status = 'pending' AND wa.approver_role = 'sales_director' 
		AND w.status = 'pending' AND w.current_level = wa.level
	`).Scan(&salesCount)

	db.Raw(`
		SELECT COUNT(*) FROM workflow_approvals wa 
		JOIN approval_workflows w ON w.id = wa.workflow_id
		WHERE wa.status = 'pending' AND wa.approver_role = 'tech_director' 
		AND w.status = 'pending' AND w.current_level = wa.level
	`).Scan(&techCount)

	db.Raw(`
		SELECT COUNT(*) FROM workflow_approvals wa 
		JOIN approval_workflows w ON w.id = wa.workflow_id
		WHERE wa.status = 'pending' AND wa.approver_role = 'finance_director' 
		AND w.status = 'pending' AND w.current_level = wa.level
	`).Scan(&financeCount)

	fmt.Printf("销售总监待审批: %d 条\n", salesCount)
	fmt.Printf("技术总监待审批: %d 条\n", techCount)
	fmt.Printf("财务总监待审批: %d 条\n", financeCount)
}
