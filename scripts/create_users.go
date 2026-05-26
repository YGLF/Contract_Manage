//go:build operational_scripts
// +build operational_scripts

package main

import (
	"fmt"
	"log"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	dsn := mustOperationalDSN()
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("连接数据库失败:", err)
	}

	password, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)

	users := []struct {
		Username string
		FullName string
		Role     string
	}{
		{"sales_director", "销售总监", "sales_director"},
		{"tech_director", "技术总监", "tech_director"},
		{"finance_director", "财务总监", "finance_director"},
		{"sales01", "张三", "sales"},
		{"sales02", "李四", "sales"},
	}

	fmt.Println("=== 创建测试用户 ===")
	for _, u := range users {
		var existing struct {
			ID uint
		}
		if db.Raw("SELECT id FROM users WHERE username = ?", u.Username).Scan(&existing); existing.ID > 0 {
			fmt.Printf("  - %s 已存在\n", u.Username)
			continue
		}

		db.Exec(`INSERT INTO users (username, hashed_password, full_name, role, phone, created_at) VALUES (?, ?, ?, ?, ?, NOW())`,
			u.Username, string(password), u.FullName, u.Role, "13800000000")
		fmt.Printf("  ✓ 创建用户: %s (%s)\n", u.Username, u.Role)
	}

	fmt.Println("\n=== 用户列表 ===")
	type User struct {
		ID       uint
		Username string
		Role     string
	}
	var usersList []User
	db.Raw("SELECT id, username, role FROM users").Scan(&usersList)
	for _, u := range usersList {
		fmt.Printf("  - %s: %s\n", u.Username, u.Role)
	}
}
