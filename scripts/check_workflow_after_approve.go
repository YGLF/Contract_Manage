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

	fmt.Println("=== 检查工作流状态 ===\n")

	// 查询所有工作流及其current_level
	type Workflow struct {
		ID           uint
		ContractID   uint
		CurrentLevel int
		MaxLevel     int
		Status       string
	}

	var workflows []Workflow
	db.Raw("SELECT id, contract_id, current_level, max_level, status FROM approval_workflows").Scan(&workflows)

	fmt.Println("工作流状态:")
	for _, w := range workflows {
		fmt.Printf("  工作流#%d: contract_id=%d, current_level=%d, max_level=%d, status=%s\n",
			w.ID, w.ContractID, w.CurrentLevel, w.MaxLevel, w.Status)
	}

	// 查询所有审批节点
	type Node struct {
		ID           uint
		WfID         uint
		Level        int
		ApproverRole string
		Status       string
	}

	var nodes []Node
	db.Raw("SELECT id, workflow_id, level, approver_role, status FROM workflow_approvals").Scan(&nodes)

	fmt.Println("\n审批节点:")
	for _, n := range nodes {
		fmt.Printf("  节点#%d: workflow_id=%d, level=%d, role=%s, status=%s\n",
			n.ID, n.WfID, n.Level, n.ApproverRole, n.Status)
	}

	// 模拟技术总监的查询
	fmt.Println("\n=== 模拟技术总监查询 ===")
	type QueryResult struct {
		ID           uint
		WfID         uint
		ContractID   uint
		ApproverRole string
		Level        int
		Status       string
		CurrentLevel int
	}

	var techResults []QueryResult
	db.Raw(`
		SELECT wa.id, wa.workflow_id, wa.contract_id, wa.approver_role, wa.level, wa.status, w.current_level
		FROM workflow_approvals wa 
		JOIN approval_workflows w ON w.id = wa.workflow_id
		WHERE wa.status = 'pending' AND wa.approver_role = 'tech_director' 
		AND w.status = 'pending' AND w.current_level = wa.level
	`).Scan(&techResults)

	fmt.Printf("技术总监可见记录: %d 条\n", len(techResults))
	for _, r := range techResults {
		fmt.Printf("  节点#%d: workflow=%d, contract=%d, level=%d, current_level=%d, status=%s\n",
			r.ID, r.WfID, r.ContractID, r.Level, r.CurrentLevel, r.Status)
	}
}
