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

	// 检查用户的角色
	fmt.Println("=== 用户角色列表 ===")
	type User struct {
		ID       uint
		Username string
		Role     string
	}
	var users []User
	db.Raw("SELECT id, username, role FROM users").Scan(&users)
	for _, u := range users {
		fmt.Printf("  - %s: %s\n", u.Username, u.Role)
	}
}
