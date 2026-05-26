//go:build operational_scripts
// +build operational_scripts

package main

import (
	"fmt"
	"log"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type User struct {
	ID             uint      `gorm:"primaryKey"`
	Username       string    `gorm:"column:username;uniqueIndex;size:50"`
	Email          string    `gorm:"column:email;size:100"`
	HashedPassword string    `gorm:"column:hashed_password;size:200"`
	FullName       string    `gorm:"column:full_name;size:100"`
	Role           string    `gorm:"column:role;size:20;default:'sales'"`
	Phone          string    `gorm:"column:phone;size:20"`
	CreatedAt      time.Time `gorm:"column:created_at"`
}

type Customer struct {
	ID            uint      `gorm:"primaryKey"`
	Name          string    `gorm:"column:name;size:200;not null"`
	ContactPerson string    `gorm:"column:contact_person;size:100"`
	ContactPhone  string    `gorm:"column:contact_phone;size:20"`
	ContactEmail  string    `gorm:"column:contact_email;size:100"`
	Address       string    `gorm:"column:address;type:text"`
	CreatedAt     time.Time `gorm:"column:created_at"`
}

type Contract struct {
	ID         uint      `gorm:"primaryKey"`
	ContractNo string    `gorm:"column:contract_no;size:50;uniqueIndex"`
	Title      string    `gorm:"column:title;size:200;not null"`
	Amount     float64   `gorm:"column:amount;type:decimal(15,2)"`
	CustomerID uint      `gorm:"column:customer_id"`
	Status     string    `gorm:"column:status;size:20;default:'draft'"`
	CreatorID  uint      `gorm:"column:creator_id"`
	CreatedAt  time.Time `gorm:"column:created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at"`
}

type ApprovalWorkflow struct {
	ID           uint      `gorm:"primaryKey"`
	ContractID   uint      `gorm:"column:contract_id;index"`
	CreatorID    uint      `gorm:"column:creator_id;index"`
	CurrentLevel int       `gorm:"column:current_level;default:1"`
	MaxLevel     int       `gorm:"column:max_level;default:3"`
	Status       string    `gorm:"column:status;size:20;default:'pending';index"`
	CreatorRole  string    `gorm:"column:creator_role;size:20"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

type WorkflowApproval struct {
	ID           uint       `gorm:"primaryKey"`
	WfID         uint       `gorm:"column:workflow_id;index"`
	ContractID   uint       `gorm:"column:contract_id;index"`
	ApproverID   *uint      `gorm:"column:approver_id"`
	ApproverRole string     `gorm:"column:approver_role;size:20"`
	Level        int        `gorm:"column:level"`
	Status       string     `gorm:"column:status;size:20;default:'pending'"`
	Comment      string     `gorm:"column:comment;type:text"`
	ApprovedAt   *time.Time `gorm:"column:approved_at"`
	CreatedAt    time.Time  `gorm:"column:created_at"`
}

func main() {
	dsn := mustOperationalDSN()
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("连接数据库失败:", err)
	}

	fmt.Println("=== 添加审批流程测试数据 ===\n")

	// 1. 创建测试用户
	password, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)

	users := []struct {
		Username string
		FullName string
		Email    string
		Role     string
	}{
		{"sales_director", "销售总监", "sales@example.com", "sales_director"},
		{"tech_director", "技术总监", "tech@example.com", "tech_director"},
		{"finance_director", "财务总监", "finance@example.com", "finance_director"},
		{"sales01", "张三", "zhangsan@example.com", "sales"},
		{"sales02", "李四", "lisi@example.com", "sales"},
	}

	for _, u := range users {
		var existing User
		if db.Where("username = ?", u.Username).First(&existing).Error != nil {
			user := User{
				Username:       u.Username,
				Email:          u.Email,
				HashedPassword: string(password),
				FullName:       u.FullName,
				Role:           u.Role,
				Phone:          "13800000000",
			}
			if err := db.Create(&user).Error; err == nil {
				fmt.Printf("✓ 创建用户: %s (角色: %s)\n", u.Username, u.Role)
			}
		} else {
			fmt.Printf("- 用户已存在: %s\n", u.Username)
		}
	}

	// 2. 创建测试客户
	var customer Customer
	if db.Where("name = ?", "测试科技有限公司").First(&customer).Error != nil {
		customer = Customer{
			Name:          "测试科技有限公司",
			ContactPerson: "王总",
			ContactPhone:  "13800138000",
			ContactEmail:  "wang@test.com",
			Address:       "北京市朝阳区测试路100号",
		}
		db.Create(&customer)
		fmt.Printf("✓ 创建客户: %s\n", customer.Name)
	} else {
		fmt.Printf("- 客户已存在: %s (ID: %d)\n", customer.Name, customer.ID)
	}

	// 3. 获取销售用户
	var salesUser User
	db.Where("username = ?", "sales01").First(&salesUser)
	fmt.Printf("- 销售人员: %s (ID: %d)\n", salesUser.Username, salesUser.ID)

	// 4. 创建测试合同及工作流
	contracts := []struct {
		No     string
		Title  string
		Amount float64
	}{
		{"CT-2024-0001", "软件开发项目合同", 500000.00},
		{"CT-2024-0002", "系统集成服务合同", 300000.00},
		{"CT-2024-0003", "技术咨询合同", 150000.00},
		{"CT-2024-0004", "运维服务合同", 200000.00},
	}

	for i, c := range contracts {
		var existing Contract
		if db.Where("contract_no = ?", c.No).First(&existing).Error != nil {
			contract := Contract{
				ContractNo: c.No,
				Title:      c.Title,
				Amount:     c.Amount,
				CustomerID: customer.ID,
				Status:     "pending",
				CreatorID:  salesUser.ID,
			}
			if err := db.Create(&contract).Error; err == nil {
				fmt.Printf("✓ 创建合同: %s - %s (¥%.2f)\n", c.No, c.Title, c.Amount)

				// 创建工作流
				workflow := ApprovalWorkflow{
					ContractID:   contract.ID,
					CreatorID:    salesUser.ID,
					CurrentLevel: 1,
					MaxLevel:     3,
					Status:       "pending",
					CreatorRole:  "sales",
				}
				db.Create(&workflow)
				fmt.Printf("  └─ 创建工作流 (ID: %d)\n", workflow.ID)

				// 创建审批节点
				nodes := []WorkflowApproval{
					{WfID: workflow.ID, ContractID: contract.ID, ApproverRole: "sales_director", Level: 1, Status: "pending"},
					{WfID: workflow.ID, ContractID: contract.ID, ApproverRole: "tech_director", Level: 2, Status: "pending"},
					{WfID: workflow.ID, ContractID: contract.ID, ApproverRole: "finance_director", Level: 3, Status: "pending"},
				}
				for _, node := range nodes {
					db.Create(&node)
				}
				fmt.Printf("  └─ 创建3级审批节点\n")

				// 第2个合同已完成第一级审批
				if i == 1 {
					now := time.Now()
					db.Model(&WorkflowApproval{}).Where("workflow_id = ? AND level = 1", workflow.ID).Updates(map[string]interface{}{
						"status":      "approved",
						"approver_id": 1,
						"comment":     "同意提交审批，流程规范",
						"approved_at": now,
					})
					fmt.Printf("  └─ 第一级审批已完成\n")
				}
			}
		} else {
			fmt.Printf("- 合同已存在: %s\n", c.No)
		}
	}

	// 5. 创建一个已完成两级审批的合同
	var existingCT4 Contract
	if db.Where("contract_no = ?", "CT-2024-0004").First(&existingCT4).Error == nil {
		var workflow ApprovalWorkflow
		if db.Where("contract_id = ?", existingCT4.ID).First(&workflow).Error == nil {
			now := time.Now()
			// 更新第一级
			db.Model(&WorkflowApproval{}).Where("workflow_id = ? AND level = 1", workflow.ID).Updates(map[string]interface{}{
				"status": "approved", "approver_id": 1, "comment": "同意", "approved_at": now,
			})
			// 更新第二级
			db.Model(&WorkflowApproval{}).Where("workflow_id = ? AND level = 2", workflow.ID).Updates(map[string]interface{}{
				"status": "approved", "approver_id": 2, "comment": "技术方案可行", "approved_at": now,
			})
			fmt.Printf("\n✓ 合同 CT-2024-0004: 已完成两级审批，等待财务总监审批\n")
		}
	}

	fmt.Println("\n=== 测试账号 (密码: 123456) ===")
	fmt.Println("  sales_director    - 销售总监 (可审批第1级)")
	fmt.Println("  tech_director     - 技术总监 (可审批第2级)")
	fmt.Println("  finance_director  - 财务总监 (可审批第3级)")
	fmt.Println("  sales01           - 销售人员 (创建合同)")
	fmt.Println("\n=== 测试场景 ===")
	fmt.Println("  CT-2024-0001: 等待销售总监审批")
	fmt.Println("  CT-2024-0002: 等待技术总监审批 (第1级已完成)")
	fmt.Println("  CT-2024-0003: 等待销售总监审批")
	fmt.Println("  CT-2024-0004: 等待财务总监审批 (前2级已完成)")
}
