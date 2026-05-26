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
	CREATE TABLE IF NOT EXISTS notifications (
		id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
		user_id BIGINT UNSIGNED NOT NULL,
		contract_id BIGINT UNSIGNED NOT NULL,
		workflow_id BIGINT UNSIGNED NOT NULL,
		target_role VARCHAR(20),
		notification_type VARCHAR(50),
		title VARCHAR(200),
		content TEXT,
		is_read TINYINT(1) DEFAULT 0,
		read_at DATETIME,
		created_at DATETIME,
		INDEX idx_user_id (user_id),
		INDEX idx_contract_id (contract_id),
		INDEX idx_is_read (is_read)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
	`

	if err := db.Exec(sql).Error; err != nil {
		log.Fatal("创建表失败:", err)
	}

	fmt.Println("✓ notifications 表创建成功")
}
