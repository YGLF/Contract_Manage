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

	sql := `
	CREATE TABLE IF NOT EXISTS approval_records (
		id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
		contract_id BIGINT UNSIGNED NOT NULL,
		approver_id BIGINT UNSIGNED NOT NULL,
		level INT DEFAULT 1,
		approver_role VARCHAR(20),
		status VARCHAR(20) DEFAULT 'pending',
		comment TEXT,
		approved_at DATETIME,
		created_at DATETIME,
		due_at DATETIME,
		is_expired TINYINT(1) DEFAULT 0,
		INDEX idx_contract_id (contract_id),
		INDEX idx_approver_id (approver_id),
		INDEX idx_status (status)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
	`

	if err := db.Exec(sql).Error; err != nil {
		log.Fatal("创建表失败:", err)
	}

	fmt.Println("✓ approval_records 表检查完成")

	// 检查其他工作流相关表
	tables := []string{"approval_workflows", "workflow_approvals"}
	for _, table := range tables {
		var count int64
		db.Raw("SELECT COUNT(*) FROM " + table).Scan(&count)
		fmt.Printf("✓ %s 表存在，记录数: %d\n", table, count)
	}
}
