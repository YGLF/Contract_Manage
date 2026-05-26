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

	fmt.Println("=== 修复工作流数据 ===")

	// 更新工作流的合同ID映射 (旧ID -> 新ID)
	// 根据合同编号匹配
	updates := map[uint]uint{
		3:  23, // 移动APP开发服务合同
		4:  24, // 网络安全升级项目
		5:  25, // 数据分析平台合同
		6:  26, // 智能办公系统合同
		7:  27, // 数据库优化服务
		8:  28, // 企业ERP系统采购
		9:  29, // 服务器设备采购
		10: 30, // 网站改版升级合同
		11: 31, // 视频会议系统集成
	}

	for oldContractID, newContractID := range updates {
		// 找到对应的工作流
		var workflow struct {
			ID         uint
			ContractID uint
		}

		// 找到旧合同ID的工作流

		if err := db.Raw("SELECT id, contract_id FROM approval_workflows WHERE contract_id = ?", oldContractID).Scan(&workflow).Error; err == nil {
			fmt.Printf("找到工作流#%d (旧合同ID=%d)\n", workflow.ID, oldContractID)

			// 更新工作流的合同ID
			db.Exec("UPDATE approval_workflows SET contract_id = ? WHERE id = ?", newContractID, workflow.ID)

			// 更新审批节点的合同ID
			db.Exec("UPDATE workflow_approvals SET contract_id = ? WHERE workflow_id = ?", newContractID, workflow.ID)

			fmt.Printf("  -> 已更新合同ID为 %d\n", newContractID)
		}
	}

	// 检查剩余的工作流
	var workflows []struct {
		ID         uint
		ContractID uint
	}
	db.Raw("SELECT id, contract_id FROM approval_workflows").Scan(&workflows)
	fmt.Printf("\n当前工作流合同ID: ")
	for _, w := range workflows {
		fmt.Printf("%d ", w.ContractID)
	}
	fmt.Println()
}
