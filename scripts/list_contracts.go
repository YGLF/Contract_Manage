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

	// 检查所有合同
	var contracts []struct {
		ID         uint
		ContractNo string
		Title      string
		Status     string
	}
	db.Raw("SELECT id, contract_no, title, status FROM contracts ORDER BY id DESC LIMIT 20").Scan(&contracts)

	fmt.Println("合同数据 (最新20条):")
	for _, c := range contracts {
		fmt.Printf("  ID=%d, No=%s, Title=%s, Status=%s\n", c.ID, c.ContractNo, c.Title, c.Status)
	}

	fmt.Printf("\n总数: %d\n", len(contracts))
}
