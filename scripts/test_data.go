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

	fmt.Println("=== 创建工作流表 ===\n")

	// 创建 approval_workflows 表
	db.Exec(`
		CREATE TABLE IF NOT EXISTS approval_workflows (
			id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
			contract_id BIGINT UNSIGNED NOT NULL,
			creator_id BIGINT UNSIGNED,
			current_level INT DEFAULT 1,
			max_level INT DEFAULT 3,
			status VARCHAR(20) DEFAULT 'pending',
			creator_role VARCHAR(20) NOT NULL,
			hash VARCHAR(64),
			created_at DATETIME,
			updated_at DATETIME,
			INDEX idx_contract_id (contract_id),
			INDEX idx_creator_id (creator_id),
			INDEX idx_status (status)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
	`)
	fmt.Println("✓ approval_workflows 表创建成功")

	// 创建 workflow_approvals 表
	db.Exec(`
		CREATE TABLE IF NOT EXISTS workflow_approvals (
			id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
			workflow_id BIGINT UNSIGNED NOT NULL,
			contract_id BIGINT UNSIGNED NOT NULL,
			approver_id BIGINT UNSIGNED,
			approver_role VARCHAR(20) NOT NULL,
			level INT NOT NULL,
			status VARCHAR(20) DEFAULT 'pending',
			comment TEXT,
			hash VARCHAR(64),
			approved_at DATETIME,
			created_at DATETIME,
			INDEX idx_workflow_id (workflow_id),
			INDEX idx_contract_id (contract_id),
			INDEX idx_approver_role (approver_role),
			INDEX idx_status (status)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
	`)
	fmt.Println("✓ workflow_approvals 表创建成功")

	// 删除旧的测试数据
	db.Exec("DELETE FROM workflow_approvals")
	db.Exec("DELETE FROM approval_workflows")
	db.Exec("DELETE FROM contracts WHERE contract_no LIKE 'TEST-%'")

	testContracts := []struct {
		No     string
		Title  string
		Amount float64
	}{
		{"TEST-2024-1001", "软件开发服务合同", 150000.00},
		{"TEST-2024-1002", "系统集成项目合同", 280000.00},
		{"TEST-2024-1003", "技术服务咨询合同", 80000.00},
		{"TEST-2024-1004", "设备采购合同", 350000.00},
	}

	for i, tc := range testContracts {
		// 创建合同
		db.Exec(`
			INSERT INTO contracts (contract_no, title, amount, customer_id, status, creator_id, created_at, updated_at)
			VALUES (?, ?, ?, 1, 'pending', 5, ?, ?)
		`, tc.No, tc.Title, tc.Amount, time.Now(), time.Now())

		var contractID uint
		db.Raw("SELECT LAST_INSERT_ID()").Scan(&contractID)
		fmt.Printf("\n✓ 创建合同: %s - %s (¥%.2f)\n", tc.No, tc.Title, tc.Amount)

		// 创建工作流
		db.Exec(`
			INSERT INTO approval_workflows (contract_id, creator_id, current_level, max_level, status, creator_role, created_at, updated_at)
			VALUES (?, 5, 1, 3, 'pending', 'sales', ?, ?)
		`, contractID, time.Now(), time.Now())

		var workflowID uint
		db.Raw("SELECT LAST_INSERT_ID()").Scan(&workflowID)
		fmt.Printf("✓ 创建工作流 (ID: %d)\n", workflowID)

		// 创建审批节点
		db.Exec(`
			INSERT INTO workflow_approvals (workflow_id, contract_id, approver_role, level, status, created_at)
			VALUES 
			(?, ?, 'sales_director', 1, 'pending', ?),
			(?, ?, 'tech_director', 2, 'pending', ?),
			(?, ?, 'finance_director', 3, 'pending', ?)
		`,
			workflowID, contractID, time.Now(),
			workflowID, contractID, time.Now(),
			workflowID, contractID, time.Now())

		// 第2个合同：销售总监已审批
		if i == 1 {
			now := time.Now()
			db.Exec(`UPDATE workflow_approvals SET status = 'approved', approver_id = 1, comment = '同意提交', approved_at = ? WHERE workflow_id = ? AND level = 1`, now, workflowID)
			db.Exec(`UPDATE approval_workflows SET current_level = 2 WHERE id = ?`, workflowID)
			fmt.Printf("  └─ 销售总监已审批，流转到技术总监\n")
		}

		// 第3个合同：销售和技术都已审批
		if i == 2 {
			now := time.Now()
			db.Exec(`UPDATE workflow_approvals SET status = 'approved', approver_id = 1, comment = '同意', approved_at = ? WHERE workflow_id = ? AND level = 1`, now, workflowID)
			db.Exec(`UPDATE workflow_approvals SET status = 'approved', approver_id = 2, comment = '技术方案可行', approved_at = ? WHERE workflow_id = ? AND level = 2`, now, workflowID)
			db.Exec(`UPDATE approval_workflows SET current_level = 3 WHERE id = ?`, workflowID)
			fmt.Printf("  └─ 销售、技术总监已审批，流转到财务总监\n")
		}
	}

	fmt.Println("\n=== 验证数据 ===")

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

	fmt.Printf("\n各角色待审批数量:")
	fmt.Printf("\n  销售总监: %d 条", salesCount)
	fmt.Printf("\n  技术总监: %d 条", techCount)
	fmt.Printf("\n  财务总监: %d 条\n", financeCount)

	fmt.Println("\n=== 测试账号 (密码: 123456) ===")
	fmt.Println("  sales01        - 销售人员")
	fmt.Println("  sales_director - 销售总监")
	fmt.Println("  tech_director  - 技术总监")
	fmt.Println("  finance_director - 财务总监")
}
