//go:build operational_scripts
// +build operational_scripts

package main

import (
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"time"
)

type User struct {
	ID             uint      `gorm:"primaryKey"`
	Username       string    `gorm:"column:username;uniqueIndex"`
	Email          string    `gorm:"column:email;uniqueIndex"`
	HashedPassword string    `gorm:"column:hashed_password"`
	FullName       string    `gorm:"column:full_name"`
	Role           string    `gorm:"column:role"`
	Phone          string    `gorm:"column:phone"`
	CreatedAt      time.Time `gorm:"column:created_at"`
}

type ContractType struct {
	ID          uint      `gorm:"primaryKey"`
	Name        string    `gorm:"size:100;unique;not null"`
	Code        string    `gorm:"size:50;unique"`
	Description string    `gorm:"type:text"`
	CreatedAt   time.Time `gorm:"column:created_at"`
}

type Customer struct {
	ID            uint       `gorm:"primaryKey"`
	Name          string     `gorm:"size:200;not null;index"`
	Type          string     `gorm:"size:20;default:customer"`
	Code          string     `gorm:"size:50;uniqueIndex"`
	ContactPerson string     `gorm:"size:100"`
	ContactPhone  string     `gorm:"size:20"`
	ContactEmail  string     `gorm:"size:100"`
	Address       string     `gorm:"type:text"`
	CreditRating  string     `gorm:"size:20"`
	IsActive      bool       `gorm:"default:true"`
	CreatedAt     time.Time  `gorm:"column:created_at"`
	UpdatedAt     *time.Time `gorm:"column:updated_at"`
}

type Contract struct {
	ID             uint       `gorm:"primaryKey"`
	ContractNo     string     `gorm:"size:50;uniqueIndex;not null"`
	Title          string     `gorm:"size:200;not null;index"`
	CustomerID     uint       `gorm:"index"`
	ContractTypeID uint       `gorm:"index"`
	Amount         float64    `json:"amount"`
	Currency       string     `gorm:"size:10;default:CNY"`
	Status         string     `gorm:"size:20;default:draft"`
	SignDate       *time.Time `json:"sign_date"`
	StartDate      *time.Time `json:"start_date"`
	EndDate        *time.Time `json:"end_date"`
	PaymentTerms   string     `gorm:"type:text"`
	Content        string     `gorm:"type:text"`
	Notes          string     `gorm:"type:text"`
	CreatorID      uint       `gorm:"index"`
	CreatedAt      time.Time  `gorm:"column:created_at"`
	UpdatedAt      *time.Time `gorm:"column:updated_at"`
}

type ApprovalWorkflow struct {
	ID           uint64    `json:"id" gorm:"primaryKey"`
	ContractID   uint64    `json:"contract_id" gorm:"column:contract_id;index;not null"`
	CreatorID    uint64    `json:"creator_id" gorm:"column:creator_id;index"`
	CurrentLevel int       `json:"current_level" gorm:"column:current_level;default:1"`
	MaxLevel     int       `json:"max_level" gorm:"column:max_level;default:3"`
	Status       string    `json:"status" gorm:"column:status;type:varchar(20);default:'pending';index"`
	CreatorRole  string    `json:"creator_role" gorm:"column:creator_role;type:varchar(20);not null"`
	Hash         string    `json:"hash" gorm:"column:hash;type:varchar(64)"`
	CreatedAt    time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt    time.Time `json:"updated_at" gorm:"column:updated_at"`
}

type WorkflowApproval struct {
	ID           uint64     `json:"id" gorm:"primaryKey"`
	WfID         uint64     `json:"wf_id" gorm:"column:workflow_id;index;not null"`
	ContractID   uint64     `json:"contract_id" gorm:"column:contract_id;index;not null"`
	ApproverRef  *uint64    `json:"approver_ref" gorm:"column:approver_id"`
	ApproverRole string     `json:"approver_role" gorm:"column:approver_role;type:varchar(20);not null"`
	Level        int        `json:"level" gorm:"column:level;not null"`
	Status       string     `json:"status" gorm:"column:status;type:varchar(20);default:'pending'"`
	Comment      string     `json:"comment" gorm:"column:comment;type:text"`
	Hash         string     `json:"hash" gorm:"column:hash;type:varchar(64)"`
	ApprovedAt   *time.Time `json:"approved_at" gorm:"column:approved_at"`
	CreatedAt    time.Time  `json:"created_at" gorm:"column:created_at"`
}

func main() {
	dsn := mustOperationalDSN()
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Println("连接数据库失败:", err)
		return
	}

	fmt.Println("==========================================")
	fmt.Println("  合同管理系统 - 完整测试数据")
	fmt.Println("==========================================\n")

	// 1. 创建用户
	fmt.Println("【1】创建测试用户...")
	password, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)

	users := []struct {
		Username string
		FullName string
		Email    string
		Role     string
	}{
		{"sales_zhao", "赵销售", "sales_zhao@example.com", "sales"},
		{"sales_director", "孙销售总监", "sales_director@example.com", "sales_director"},
		{"tech_director", "周技术总监", "tech_director@example.com", "tech_director"},
		{"finance_director", "吴财务总监", "finance_director@example.com", "finance_director"},
		{"admin_zhang", "张主管", "admin_zhang@example.com", "admin"},
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
			db.Create(&user)
			fmt.Printf("   ✓ 创建用户: %s (%s)\n", u.Username, u.Role)
		} else {
			// 更新email确保不为空
			var emailEmpty bool
			if existing.Email == nil || *existing.Email == "" {
				emailEmpty = true
			}
			if emailEmpty {
				db.Model(&existing).Update("email", u.Email)
				fmt.Printf("   ✓ 更新用户email: %s\n", u.Username)
			} else {
				fmt.Printf("   - 用户已存在: %s\n", u.Username)
			}
		}
	}

	// 2. 创建合同类型
	fmt.Println("\n【2】创建合同类型...")
	types := []struct {
		Name        string
		Code        string
		Description string
	}{
		{"软件开发合同", "SOFTWARE", "软件开发及技术服务合同"},
		{"系统集成合同", "INTEGRATION", "系统集成服务合同"},
		{"咨询服务合同", "CONSULTING", "技术咨询服务合同"},
	}

	for _, t := range types {
		var existing ContractType
		if db.Where("code = ?", t.Code).First(&existing).Error != nil {
			ct := ContractType{
				Name:        t.Name,
				Code:        t.Code,
				Description: t.Description,
			}
			db.Create(&ct)
			fmt.Printf("   ✓ 创建类型: %s\n", t.Name)
		} else {
			fmt.Printf("   - 类型已存在: %s\n", t.Name)
		}
	}

	// 3. 创建客户
	fmt.Println("\n【3】创建客户...")
	customers := []struct {
		Name          string
		Code          string
		ContactPerson string
	}{
		{"华为技术有限公司", "HW001", "任正非"},
		{"阿里巴巴集团", "ALY001", "马云"},
		{"腾讯科技", "TX001", "马化腾"},
	}

	for _, c := range customers {
		var existing Customer
		if db.Where("code = ?", c.Code).First(&existing).Error != nil {
			customer := Customer{
				Name:          c.Name,
				Code:          c.Code,
				ContactPerson: c.ContactPerson,
				ContactPhone:  "13800138000",
				ContactEmail:  "test@example.com",
				Address:       "测试地址",
				IsActive:      true,
			}
			db.Create(&customer)
			fmt.Printf("   ✓ 创建客户: %s\n", c.Name)
		} else {
			fmt.Printf("   - 客户已存在: %s\n", c.Name)
		}
	}

	// 获取数据ID
	var salesUser User
	var hwCustomer Customer
	var swType ContractType
	db.Where("username = ?", "sales_zhao").First(&salesUser)
	db.Where("code = ?", "HW001").First(&hwCustomer)
	db.Where("code = ?", "SOFTWARE").First(&swType)

	// 4. 创建合同
	fmt.Println("\n【4】创建合同...")
	contractNo := "CT2026040002"
	var existing Contract
	if db.Where("contract_no = ?", contractNo).First(&existing).Error != nil {
		contract := Contract{
			ContractNo:     contractNo,
			Title:          "企业ERP系统开发合同",
			Amount:         800000.00,
			CustomerID:     hwCustomer.ID,
			ContractTypeID: swType.ID,
			Status:         "pending",
			Content:        "开发企业ERP管理系统",
			CreatorID:      salesUser.ID,
		}
		db.Create(&contract)
		fmt.Printf("   ✓ 创建合同: %s - %s (金额: %.2f)\n", contractNo, contract.Title, contract.Amount)

		// 5. 创建工作流
		fmt.Println("\n【5】创建审批工作流...")
		workflow := ApprovalWorkflow{
			ContractID:   uint64(contract.ID),
			CreatorID:    uint64(salesUser.ID),
			CurrentLevel: 1,
			MaxLevel:     3,
			Status:       "pending",
			CreatorRole:  "sales",
		}
		db.Create(&workflow)
		fmt.Printf("   ✓ 创建工作流 (ID: %d)\n", workflow.ID)

		// 创建审批节点
		nodes := []WorkflowApproval{
			{WfID: workflow.ID, ContractID: uint64(contract.ID), ApproverRole: "sales_director", Level: 1, Status: "pending"},
			{WfID: workflow.ID, ContractID: uint64(contract.ID), ApproverRole: "tech_director", Level: 2, Status: "pending"},
			{WfID: workflow.ID, ContractID: uint64(contract.ID), ApproverRole: "finance_director", Level: 3, Status: "pending"},
		}
		for _, node := range nodes {
			db.Create(&node)
		}
		fmt.Printf("   ✓ 创建审批节点: 销售总监 -> 技术总监 -> 财务总监\n")
	} else {
		fmt.Printf("   - 合同已存在: %s\n", contractNo)
	}

	fmt.Println("\n==========================================")
	fmt.Println("  测试数据创建完成!")
	fmt.Println("==========================================")
	fmt.Println("\n【测试账号】(密码: 123456)")
	fmt.Println("  销售人员: sales_zhao")
	fmt.Println("  销售总监: sales_director (第1级审批)")
	fmt.Println("  技术总监: tech_director (第2级审批)")
	fmt.Println("  财务总监: finance_director (第3级审批)")
	fmt.Println("\n【合同信息】")
	fmt.Printf("  合同编号: %s\n", contractNo)
	fmt.Println("  状态: 待销售总监审批")
	fmt.Println("\n【API测试】")
	fmt.Println("  登录: POST /api/auth/login")
	fmt.Println("  待审批列表: GET /api/workflow/pending")
}
