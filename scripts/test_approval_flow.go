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

	fmt.Println("=== 测试前准备：重置测试数据 ===\n")

	// 删除旧的测试数据
	db.Exec("DELETE FROM workflow_approvals")
	db.Exec("DELETE FROM approval_workflows")
	db.Exec("DELETE FROM contracts WHERE contract_no LIKE 'TEST-%'")

	// 创建测试合同
	result := db.Exec(`
		INSERT INTO contracts (contract_no, title, amount, customer_id, status, creator_id, created_at, updated_at)
		VALUES (?, ?, ?, 1, 'pending', 5, ?, ?)
	`, "TEST-2024-0001", "测试合同A项目", 100000.00, time.Now(), time.Now())

	if result.Error != nil {
		log.Fatal("创建合同失败:", result.Error)
	}

	var contractID uint
	db.Raw("SELECT LAST_INSERT_ID()").Scan(&contractID)
	fmt.Printf("✓ 创建测试合同: TEST-2024-0001 - 测试合同A项目 (ID: %d)\n", contractID)

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

	fmt.Printf("✓ 创建3级审批节点\n")

	fmt.Println("\n========================================")
	fmt.Println("=== 测试数据已准备好 ===")
	fmt.Println("========================================")
	fmt.Printf("\n测试合同编号: TEST-2024-0001\n")
	fmt.Printf("合同ID: %d, 工作流ID: %d\n", contractID, workflowID)
	fmt.Printf("当前审批级别: 1 (等待销售总监审批)\n")

	fmt.Println("\n=== 测试步骤 ===")
	fmt.Println("1. 用 sales01 账号登录合同管理，可以看到 TEST-2024-0001")
	fmt.Println("2. 点击查看 → 审批记录标签 → 提交审批")
	fmt.Println("3. 用 sales_director 账号登录审批管理，同意")
	fmt.Println("4. 用 tech_director 账号登录审批管理，同意")
	fmt.Println("5. 用 finance_director 账号登录审批管理，同意 → 归档")
	fmt.Println("\n=== 测试拒绝流程 ===")
	fmt.Println("1-2. 同上步骤1-2")
	fmt.Println("3. 用 tech_director 账号登录审批管理，拒绝")
	fmt.Println("4. 用 sales01 账号重新提交，应该重用原工作流")
}
