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

	// 检查合同表
	var contracts []struct {
		ID    uint
		Title string
	}
	db.Raw("SELECT id, title FROM contracts WHERE id IN (3,4,5)").Scan(&contracts)

	fmt.Println("合同数据:")
	for _, c := range contracts {
		fmt.Printf("  ID=%d, Title=%s\n", c.ID, c.Title)
	}

	if len(contracts) == 0 {
		fmt.Println("  没有找到合同数据!")
	}
}
