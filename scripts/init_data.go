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

	fmt.Println("=== 重新创建测试数据 ===\n")

	// 清理旧数据
	db.Exec("DELETE FROM workflow_approvals")
	db.Exec("DELETE FROM approval_workflows")
	db.Exec("DELETE FROM contracts WHERE contract_no LIKE 'TEST-%'")

	// 创建4个测试合同
	contracts := []struct {
		No     string
		Title  string
		Amount float64
	}{
		{"TEST-2024-1001", "软件开发服务合同", 150000.00},
		{"TEST-2024-1002", "系统集成项目合同", 280000.00},
		{"TEST-2024-1003", "技术服务咨询合同", 80000.00},
		{"TEST-2024-1004", "设备采购合同", 350000.00},
	}

	for i, c := range contracts {
		// 创建合同
		db.Exec(`INSERT INTO contracts (contract_no, title, amount, customer_id, status, creator_id, created_at, updated_at) VALUES (?, ?, ?, 1, 'pending', 5, ?, ?)`,
			c.No, c.Title, c.Amount, time.Now(), time.Now())

		var contractID uint
		db.Raw("SELECT LAST_INSERT_ID()").Scan(&contractID)

		// 创建工作流
		db.Exec(`INSERT INTO approval_workflows (contract_id, creator_id, current_level, max_level, status, creator_role, created_at, updated_at) VALUES (?, 5, 1, 3, 'pending', 'sales', ?, ?)`,
			contractID, time.Now(), time.Now())

		var workflowID uint
		db.Raw("SELECT LAST_INSERT_ID()").Scan(&workflowID)

		// 创建审批节点
		db.Exec(`INSERT INTO workflow_approvals (workflow_id, contract_id, approver_role, level, status, created_at) VALUES (?, ?, 'sales_director', 1, 'pending', ?), (?, ?, 'tech_director', 2, 'pending', ?), (?, ?, 'finance_director', 3, 'pending', ?)`,
			workflowID, contractID, time.Now(),
			workflowID, contractID, time.Now(),
			workflowID, contractID, time.Now())

		// 第2个合同 - 销售总监已审批
		if i == 1 {
			db.Exec(`UPDATE workflow_approvals SET status='approved', approver_id=1, comment='同意', approved_at=? WHERE workflow_id=? AND level=1`, time.Now(), workflowID)
			db.Exec(`UPDATE approval_workflows SET current_level=2 WHERE id=?`, workflowID)
		}

		// 第3个合同 - 销售+技术已审批
		if i == 2 {
			db.Exec(`UPDATE workflow_approvals SET status='approved', approver_id=1, comment='同意', approved_at=? WHERE workflow_id=? AND level=1`, time.Now(), workflowID)
			db.Exec(`UPDATE workflow_approvals SET status='approved', approver_id=2, comment='技术可行', approved_at=? WHERE workflow_id=? AND level=2`, time.Now(), workflowID)
			db.Exec(`UPDATE approval_workflows SET current_level=3 WHERE id=?`, workflowID)
		}

		fmt.Printf("✓ %s - %s (工作流#%d)\n", c.No, c.Title, workflowID)
	}

	// 验证
	fmt.Println("\n=== 待审批统计 ===")
	var sales, tech, finance int64
	db.Raw(`SELECT COUNT(*) FROM workflow_approvals wa JOIN approval_workflows w ON w.id=wa.workflow_id WHERE wa.status='pending' AND wa.approver_role='sales_director' AND w.status='pending' AND w.current_level=wa.level`).Scan(&sales)
	db.Raw(`SELECT COUNT(*) FROM workflow_approvals wa JOIN approval_workflows w ON w.id=wa.workflow_id WHERE wa.status='pending' AND wa.approver_role='tech_director' AND w.status='pending' AND w.current_level=wa.level`).Scan(&tech)
	db.Raw(`SELECT COUNT(*) FROM workflow_approvals wa JOIN approval_workflows w ON w.id=wa.workflow_id WHERE wa.status='pending' AND wa.approver_role='finance_director' AND w.status='pending' AND w.current_level=wa.level`).Scan(&finance)

	fmt.Printf("销售总监: %d 条\n", sales)
	fmt.Printf("技术总监: %d 条\n", tech)
	fmt.Printf("财务总监: %d 条\n", finance)

	fmt.Println("\n=== 测试账号 (密码: 123456) ===")
	fmt.Println("admin           - 管理员 (查看所有)")
	fmt.Println("sales01         - 销售人员")
	fmt.Println("sales_director  - 销售总监")
	fmt.Println("tech_director   - 技术总监")
	fmt.Println("finance_director - 财务总监")
}
