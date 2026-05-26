//go:build operational_scripts
// +build operational_scripts

package main

import (
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type User struct {
	ID             uint   `gorm:"primaryKey"`
	Username       string `gorm:"column:username;uniqueIndex"`
	Email          string `gorm:"column:email;uniqueIndex"`
	HashedPassword string `gorm:"column:hashed_password"`
	FullName       string `gorm:"column:full_name"`
	Role           string `gorm:"column:role"`
}

func main() {
	dsn := mustOperationalDSN()
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Println("连接失败:", err)
		return
	}

	// 检查email为空的用户
	var users []User
	db.Where("email = ? OR email IS NULL", "").Find(&users)
	fmt.Printf("找到 %d 个email为空的用户:\n", len(users))
	for _, u := range users {
		fmt.Printf("  ID: %d, Username: %s, Email: '%s', Role: %s\n", u.ID, u.Username, u.Email, u.Role)
	}

	// 检查重复的email
	var duplicates []User
	db.Raw("SELECT * FROM users WHERE email IN (SELECT email FROM users GROUP BY email HAVING COUNT(*) > 1)").Find(&duplicates)
	fmt.Printf("\n找到 %d 个重复email的用户:\n", len(duplicates))
	for _, u := range duplicates {
		fmt.Printf("  ID: %d, Username: %s, Email: '%s'\n", u.ID, u.Username, u.Email)
	}

	// 修复: 为空email的用户设置唯一email
	password, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)

	fixUsers := []struct {
		Username string
		Email    string
		FullName string
		Role     string
	}{
		{"sales_director", "sales_director@example.com", "孙销售总监", "sales_director"},
		{"tech_director", "tech_director@example.com", "周技术总监", "tech_director"},
		{"finance_director", "finance_director@example.com", "吴财务总监", "finance_director"},
	}

	for _, u := range fixUsers {
		var existing User
		if db.Where("username = ?", u.Username).First(&existing).Error == nil {
			// 用户存在，更新email
			db.Model(&existing).Update("email", u.Email)
			fmt.Printf("已更新用户 %s 的email为 %s\n", u.Username, u.Email)
		} else {
			// 创建新用户
			user := User{
				Username:       u.Username,
				Email:          u.Email,
				HashedPassword: string(password),
				FullName:       u.FullName,
				Role:           u.Role,
			}
			if err := db.Create(&user).Error; err != nil {
				fmt.Printf("创建用户 %s 失败: %v\n", u.Username, err)
			} else {
				fmt.Printf("已创建用户 %s\n", u.Username)
			}
		}
	}
}
