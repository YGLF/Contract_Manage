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

	fmt.Println("=== 修复审批节点 workflow_id ===\n")

	// 先删除错误的审批节点
	db.Exec("DELETE FROM workflow_approvals")

	// 获取所有工作流
	type Workflow struct {
		ID         uint
		ContractID uint
	}

	var workflows []Workflow
	db.Raw("SELECT id, contract_id FROM approval_workflows").Scan(&workflows)

	for _, w := range workflows {
		fmt.Printf("修复工作流#%d (contract_id=%d)\n", w.ID, w.ContractID)

		// 检查是否已有节点
		var count int
		db.Raw("SELECT COUNT(*) FROM workflow_approvals WHERE workflow_id = ?", w.ID).Scan(&count)
		if count > 0 {
			fmt.Println("  已有节点，跳过")
			continue
		}

		// 插入节点 - 注意要使用正确的工作流ID
		err := db.Exec(`
			INSERT INTO workflow_approvals (workflow_id, contract_id, approver_role, level, status, created_at)
			VALUES (?, ?, 'sales_director', 1, 'pending', ?),
			       (?, ?, 'tech_director', 2, 'pending', ?),
			       (?, ?, 'finance_director', 3, 'pending', ?)
		`, w.ID, w.ContractID, time.Now(), w.ID, w.ContractID, time.Now(), w.ID, w.ContractID, time.Now()).Error

		if err != nil {
			fmt.Printf("  错误: %v\n", err)
		} else {
			fmt.Println("  ✓ 节点创建成功")
		}
	}

	// 更新已通过的节点状态
	// TEST-2024-1002 (workflow_id=2) - 销售总监已通过
	db.Exec(`UPDATE workflow_approvals SET status = 'approved', approver_id = 1, comment = '同意提交', approved_at = ? WHERE workflow_id = 2 AND level = 1`, time.Now())
	db.Exec(`UPDATE approval_workflows SET current_level = 2 WHERE id = 2`)

	// TEST-2024-1003 (workflow_id=3) - 销售和技术都已通过
	db.Exec(`UPDATE workflow_approvals SET status = 'approved', approver_id = 1, comment = '同意', approved_at = ? WHERE workflow_id = 3 AND level = 1`, time.Now())
	db.Exec(`UPDATE workflow_approvals SET status = 'approved', approver_id = 2, comment = '技术方案可行', approved_at = ? WHERE workflow_id = 3 AND level = 2`, time.Now())
	db.Exec(`UPDATE approval_workflows SET current_level = 3 WHERE id = 3`)

	fmt.Println("\n=== 最终验证 ===")

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
		fmt.Printf("  工作流#%d, 级别%d (%s): %s\n",
			n.WfID, n.Level, n.ApproverRole, n.Status)
	}

	// 统计
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

	fmt.Printf("\n各角色待审批:\n")
	fmt.Printf("  销售总监: %d 条\n", salesCount)
	fmt.Printf("  技术总监: %d 条\n", techCount)
	fmt.Printf("  财务总监: %d 条\n", financeCount)

	fmt.Println("\n=== 请重启后端服务后测试 ===")
	fmt.Println("测试账号 (密码: 123456):")
	fmt.Println("  sales_director - 销售总监")
	fmt.Println("  tech_director  - 技术总监")
	fmt.Println("  finance_director - 财务总监")
}
