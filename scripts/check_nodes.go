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

	fmt.Println("=== 检查审批节点原始数据 ===\n")

	type Node struct {
		ID           uint
		WfID         uint   `gorm:"column:workflow_id"`
		ContractID   uint   `gorm:"column:contract_id"`
		ApproverRole string `gorm:"column:approver_role"`
		Level        int    `gorm:"column:level"`
		Status       string `gorm:"column:status"`
	}

	var nodes []Node
	db.Table("workflow_approvals").Find(&nodes)

	fmt.Println("ID | workflow_id | contract_id | approver_role | level | status")
	fmt.Println("---+-------------+-------------+---------------+-------+--------")
	for _, n := range nodes {
		fmt.Printf("%d | %d | %d | %s | %d | %s\n",
			n.ID, n.WfID, n.ContractID, n.ApproverRole, n.Level, n.Status)
	}
}
