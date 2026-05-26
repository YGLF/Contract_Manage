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

type ApprovalWorkflow struct {
	ID           uint
	ContractID   uint
	CreatorID    uint
	CurrentLevel int
	MaxLevel     int
	Status       string
	CreatorRole  string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type WorkflowApproval struct {
	ID           uint
	WfID         uint       `gorm:"column:workflow_id"`
	ContractID   uint       `gorm:"column:contract_id"`
	ApproverID   *uint      `gorm:"column:approver_id"`
	ApproverRole string     `gorm:"column:approver_role"`
	Level        int        `gorm:"column:level"`
	Status       string     `gorm:"column:status"`
	Comment      string     `gorm:"column:comment"`
	ApprovedAt   *time.Time `gorm:"column:approved_at"`
	CreatedAt    time.Time  `gorm:"column:created_at"`
}

func main() {
	dsn := mustOperationalDSN()
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("连接数据库失败:", err)
	}

	fmt.Println("=== 重新创建工作流测试数据 ===\n")

	// 获取工作流
	var workflows []ApprovalWorkflow
	db.Find(&workflows)

	// 清空旧的审批节点
	db.Exec("DELETE FROM workflow_approvals")

	for _, workflow := range workflows {
		fmt.Printf("为工作流#%d 创建审批节点...\n", workflow.ID)

		// 创建3级审批节点
		nodes := []WorkflowApproval{
			{
				WfID:         workflow.ID,
				ContractID:   workflow.ContractID,
				ApproverRole: "sales_director",
				Level:        1,
				Status:       "pending",
				CreatedAt:    time.Now(),
			},
			{
				WfID:         workflow.ID,
				ContractID:   workflow.ContractID,
				ApproverRole: "tech_director",
				Level:        2,
				Status:       "pending",
				CreatedAt:    time.Now(),
			},
			{
				WfID:         workflow.ID,
				ContractID:   workflow.ContractID,
				ApproverRole: "finance_director",
				Level:        3,
				Status:       "pending",
				CreatedAt:    time.Now(),
			},
		}

		// 合同2已完成第一级
		if workflow.ContractID == 2 {
			now := time.Now()
			nodes[0].Status = "approved"
			nodes[0].ApproverID = uintPtr(1)
			nodes[0].Comment = "同意提交审批"
			nodes[0].ApprovedAt = &now
		}

		// 合同4已完成两级
		if workflow.ContractID == 4 {
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

		for _, node := range nodes {
			if err := db.Create(&node).Error; err != nil {
				fmt.Printf("  错误: %v\n", err)
			} else {
				fmt.Printf("  ✓ 创建节点: role=%s, level=%d, status=%s\n",
					node.ApproverRole, node.Level, node.Status)
			}
		}
	}

	fmt.Println("\n=== 验证数据 ===")
	var count int64
	db.Model(&WorkflowApproval{}).Count(&count)
	fmt.Printf("审批节点总数: %d\n", count)

	// 统计每个角色的待审批
	var salesPending, techPending, financePending int64
	db.Model(&WorkflowApproval{}).Where("approver_role = ? AND status = ?", "sales_director", "pending").Count(&salesPending)
	db.Model(&WorkflowApproval{}).Where("approver_role = ? AND status = ?", "tech_director", "pending").Count(&techPending)
	db.Model(&WorkflowApproval{}).Where("approver_role = ? AND status = ?", "finance_director", "pending").Count(&financePending)
	fmt.Printf("待审批统计: 销售总监=%d, 技术总监=%d, 财务总监=%d\n", salesPending, techPending, financePending)
}

func uintPtr(i uint) *uint {
	return &i
}
