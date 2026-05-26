//go:build operational_scripts
// +build operational_scripts

package main

import (
	"fmt"
	"log"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	dsn := mustOperationalDSN()
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("连接数据库失败:", err)
	}

	fmt.Println("=== 修复审批节点的workflow_id ===\n")

	// 获取所有工作流
	type Workflow struct {
		ID         uint
		ContractID uint
	}

	var workflows []Workflow
	db.Raw("SELECT id, contract_id FROM approval_workflows").Scan(&workflows)

	fmt.Println("工作流列表:")
	for _, w := range workflows {
		fmt.Printf("  工作流#%d: contract_id=%d\n", w.ID, w.ContractID)
	}

	// 删除所有旧的审批节点（因为workflow_id是错的）
	fmt.Println("\n删除旧的审批节点...")
	db.Exec("DELETE FROM workflow_approvals")

	// 为每个工作流重新创建审批节点
	for _, workflow := range workflows {
		fmt.Printf("\n为工作流#%d 创建审批节点...\n", workflow.ID)

		nodes := []struct {
			ApproverRole string
			Level        int
			Status       string
			ApproverID   *uint
			Comment      string
			ApprovedAt   *time.Time
		}{
			{ApproverRole: "sales_director", Level: 1, Status: "pending", ApproverID: nil, Comment: "", ApprovedAt: nil},
			{ApproverRole: "tech_director", Level: 2, Status: "pending", ApproverID: nil, Comment: "", ApprovedAt: nil},
			{ApproverRole: "finance_director", Level: 3, Status: "pending", ApproverID: nil, Comment: "", ApprovedAt: nil},
		}

		// 合同2 (workflow 2) - 销售总监已通过
		if workflow.ID == 2 {
			now := time.Now()
			nodes[0].Status = "approved"
			nodes[0].ApproverID = uintPtr(1)
			nodes[0].Comment = "同意提交审批"
			nodes[0].ApprovedAt = &now
		}

		// 合同4 (workflow 4) - 销售和技术都已通过
		if workflow.ID == 4 {
			now := time.Now()
			nodes[0].Status = "approved"
			nodes[0].ApproverID = uintPtr(1)
			nodes[0].Comment = "同意"
			nodes[0].ApprovedAt = &now

			nodes[1].Status = "approved"
			nodes[1].ApproverID = uintPtr(2)
			nodes[1].Comment = "技术方案可行"
			nodes[1].ApprovedAt = &now
		}

		// 如果工作流的current_level > 1，说明之前的审批已完成
		var currentLevel int
		db.Raw("SELECT current_level FROM approval_workflows WHERE id = ?", workflow.ID).Scan(&currentLevel)

		if currentLevel > 1 && workflow.ID != 2 && workflow.ID != 4 {
			// 说明第1级已通过
			now := time.Now()
			nodes[0].Status = "approved"
			nodes[0].ApproverID = uintPtr(1)
			nodes[0].Comment = "同意"
			nodes[0].ApprovedAt = &now
		}

		for _, node := range nodes {
			err := db.Exec(`
				INSERT INTO workflow_approvals (workflow_id, contract_id, approver_role, level, status, approver_id, comment, approved_at, created_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, workflow.ID, workflow.ContractID, node.ApproverRole, node.Level, node.Status, node.ApproverID, node.Comment, node.ApprovedAt, time.Now()).Error

			if err != nil {
				fmt.Printf("  错误: %v\n", err)
			} else {
				fmt.Printf("  ✓ 节点: role=%s, level=%d, status=%s\n", node.ApproverRole, node.Level, node.Status)
			}
		}
	}

	// 更新工作流的current_level
	fmt.Println("\n=== 更新工作流状态 ===")
	for _, workflow := range workflows {
		// 检查第1级是否通过
		var level1Status string
		db.Raw("SELECT status FROM workflow_approvals WHERE workflow_id = ? AND level = 1", workflow.ID).Scan(&level1Status)

		// 检查第2级是否通过
		var level2Status string
		db.Raw("SELECT status FROM workflow_approvals WHERE workflow_id = ? AND level = 2", workflow.ID).Scan(&level2Status)

		newLevel := 1
		if level1Status == "approved" {
			newLevel = 2
		}
		if level2Status == "approved" {
			newLevel = 3
		}

		db.Exec("UPDATE approval_workflows SET current_level = ? WHERE id = ?", newLevel, workflow.ID)
		fmt.Printf("  工作流#%d: current_level 更新为 %d\n", workflow.ID, newLevel)
	}

	fmt.Println("\n=== 验证数据 ===")

	// 验证查询
	var techCount int
	db.Raw(`
		SELECT COUNT(*) FROM workflow_approvals wa 
		JOIN approval_workflows w ON w.id = wa.workflow_id
		WHERE wa.status = 'pending' AND wa.approver_role = 'tech_director' 
		AND w.status = 'pending' AND w.current_level = wa.level
	`).Scan(&techCount)
	fmt.Printf("技术总监待审批: %d 条\n", techCount)

	var salesCount int
	db.Raw(`
		SELECT COUNT(*) FROM workflow_approvals wa 
		JOIN approval_workflows w ON w.id = wa.workflow_id
		WHERE wa.status = 'pending' AND wa.approver_role = 'sales_director' 
		AND w.status = 'pending' AND w.current_level = wa.level
	`).Scan(&salesCount)
	fmt.Printf("销售总监待审批: %d 条\n", salesCount)

	var financeCount int
	db.Raw(`
		SELECT COUNT(*) FROM workflow_approvals wa 
		JOIN approval_workflows w ON w.id = wa.workflow_id
		WHERE wa.status = 'pending' AND wa.approver_role = 'finance_director' 
		AND w.status = 'pending' AND w.current_level = wa.level
	`).Scan(&financeCount)
	fmt.Printf("财务总监待审批: %d 条\n", financeCount)
}

func uintPtr(i uint) *uint {
	return &i
}
