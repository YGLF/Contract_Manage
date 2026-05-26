//go:build operational_scripts
// +build operational_scripts

package main

import (
	"fmt"
	"log"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type Contract struct {
	ID         uint
	ContractNo string
	Title      string
	Status     string
}

type ApprovalWorkflow struct {
	ID           uint
	ContractID   uint
	CurrentLevel int
	MaxLevel     int
	Status       string
}

type WorkflowApproval struct {
	ID           uint
	WfID         uint
	ContractID   uint
	ApproverRole string
	Level        int
	Status       string
}

func main() {
	dsn := mustOperationalDSN()
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("连接数据库失败:", err)
	}

	fmt.Println("=== 检查工作流数据 ===\n")

	// 检查合同
	var contracts []Contract
	db.Find(&contracts)
	fmt.Printf("合同数量: %d\n", len(contracts))
	for _, c := range contracts {
		fmt.Printf("  - %s: %s (状态: %s)\n", c.ContractNo, c.Title, c.Status)
	}

	// 检查工作流 (approval_workflows)
	var workflows []ApprovalWorkflow
	db.Find(&workflows)
	fmt.Printf("\n工作流数量: %d\n", len(workflows))
	for _, w := range workflows {
		fmt.Printf("  - 工作流#%d: contract_id=%d, current_level=%d, max_level=%d, status=%s\n",
			w.ID, w.ContractID, w.CurrentLevel, w.MaxLevel, w.Status)
	}

	// 检查审批节点 (workflow_approvals)
	var nodes []WorkflowApproval
	db.Find(&nodes)
	fmt.Printf("\n审批节点数量: %d\n", len(nodes))
	for _, n := range nodes {
		fmt.Printf("  - 节点#%d: workflow_id=%d, role=%s, level=%d, status=%s\n",
			n.ID, n.WfID, n.ApproverRole, n.Level, n.Status)
	}

	// 检查待审批的记录
	fmt.Println("\n=== 待审批查询测试 ===")
	var pendingNodes []WorkflowApproval
	db.Where("status = ?", "pending").Find(&pendingNodes)
	fmt.Printf("待审批节点数: %d\n", len(pendingNodes))

	// 按角色分组
	roleGroups := make(map[string][]WorkflowApproval)
	for _, n := range pendingNodes {
		roleGroups[n.ApproverRole] = append(roleGroups[n.ApproverRole], n)
	}
	for role, nodes := range roleGroups {
		fmt.Printf("  %s: %d 条\n", role, len(nodes))
	}

	// 模拟查询：销售总监可见的待审批
	fmt.Println("\n=== 模拟查询：销售总监 ===")
	var salesPending []WorkflowApproval
	db.Table("workflow_approvals").
		Joins("JOIN approval_workflows ON approval_workflows.id = workflow_approvals.workflow_id").
		Where("workflow_approvals.status = ?", "pending").
		Where("workflow_approvals.approver_role = ?", "sales_director").
		Where("approval_workflows.status = ?", "pending").
		Where("workflow_approvals.level = ?", 1).
		Find(&salesPending)
	fmt.Printf("销售总监可见: %d 条\n", len(salesPending))
}
