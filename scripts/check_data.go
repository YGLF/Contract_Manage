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

	fmt.Println("=== 检查工作流数据 ===\n")

	// 检查工作流
	var workflows []struct {
		ID           uint
		ContractID   uint
		CurrentLevel int
		MaxLevel     int
		Status       string
		CreatorRole  string
	}
	db.Raw("SELECT id, contract_id, current_level, max_level, status, creator_role FROM approval_workflows").Scan(&workflows)

	fmt.Printf("工作流总数: %d\n", len(workflows))
	for _, w := range workflows {
		fmt.Printf("  工作流#%d: 合同ID=%d, 当前级别=%d, 最大级别=%d, 状态=%s, 创建者角色=%s\n",
			w.ID, w.ContractID, w.CurrentLevel, w.MaxLevel, w.Status, w.CreatorRole)
	}

	// 检查审批节点
	fmt.Println("\n=== 检查审批节点 ===")
	var nodes []struct {
		ID           uint
		WorkflowID   uint
		ContractID   uint
		ApproverRole string
		Level        int
		Status       string
	}
	db.Raw("SELECT id, workflow_id, contract_id, approver_role, level, status FROM workflow_approvals ORDER BY workflow_id, level").Scan(&nodes)

	fmt.Printf("审批节点总数: %d\n", len(nodes))
	for _, n := range nodes {
		fmt.Printf("  节点#%d: 工作流#%d, 合同ID=%d, 角色=%s, 级别=%d, 状态=%s\n",
			n.ID, n.WorkflowID, n.ContractID, n.ApproverRole, n.Level, n.Status)
	}

	// 查询管理员可以看到的所有待审批项
	fmt.Println("\n=== 模拟管理员查询 ===")
	var pendingItems []struct {
		ID           uint
		WorkflowID   uint
		ContractID   uint
		ApproverRole string
		Level        int
		Status       string
	}
	db.Raw(`SELECT wa.id, wa.workflow_id, wa.contract_id, wa.approver_role, wa.level, wa.status
		FROM workflow_approvals wa
		JOIN approval_workflows w ON w.id = wa.workflow_id
		WHERE wa.status = 'pending' AND w.status = 'pending'`).Scan(&pendingItems)

	fmt.Printf("管理员可见的待审批项: %d\n", len(pendingItems))
	for _, p := range pendingItems {
		fmt.Printf("  待审批#%d: 工作流#%d, 合同ID=%d, 角色=%s, 级别=%d\n",
			p.ID, p.WorkflowID, p.ContractID, p.ApproverRole, p.Level)
	}
}
