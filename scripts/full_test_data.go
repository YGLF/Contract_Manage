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
		log.Fatal("连接数据库失败:", err)
	}

	fmt.Println("============================================")
	fmt.Println("  合同管理系统 - 完整测试数据初始化")
	fmt.Println("============================================\n")

	// 1. 创建测试用户
	fmt.Println("【1】创建测试用户...")
	password, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)

	users := []struct {
		Username string
		FullName string
		Email    string
		Role     string
	}{
		{"manager_wang", "王经理", "manager_wang@example.com", "sales"},
		{"director_li", "李总监", "director_li@example.com", "director"},
		{"admin_zhang", "张主管", "admin_zhang@example.com", "admin"},
		{"sales_zhao", "赵销售", "sales_zhao@example.com", "sales"},
		{"finance_chen", "陈财务", "finance_chen@example.com", "finance"},
		{"sales_director", "孙销售总监", "sales_director@example.com", "sales_director"},
		{"tech_director", "周技术总监", "tech_director@example.com", "tech_director"},
		{"finance_director", "吴财务总监", "finance_director@example.com", "finance_director"},
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
			fmt.Printf("   ✓ 创建用户: %s (角色: %s)\n", u.Username, u.Role)
		} else {
			fmt.Printf("   - 用户已存在: %s\n", u.Username)
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
		{"运维服务合同", "MAINTENANCE", "系统运维服务合同"},
		{"采购合同", "PROCUREMENT", "设备采购及安装合同"},
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
			fmt.Printf("   ✓ 创建合同类型: %s (%s)\n", t.Name, t.Code)
		} else {
			fmt.Printf("   - 合同类型已存在: %s\n", t.Name)
		}
	}

	// 获取合同类型ID
	var swType, intType, conType ContractType
	db.Where("code = ?", "SOFTWARE").First(&swType)
	db.Where("code = ?", "INTEGRATION").First(&intType)
	db.Where("code = ?", "CONSULTING").First(&conType)

	// 3. 创建客户
	fmt.Println("\n【3】创建客户...")
	customers := []struct {
		Name          string
		Code          string
		ContactPerson string
		ContactPhone  string
		ContactEmail  string
		Address       string
	}{
		{"华为技术有限公司", "HW001", "任正非", "13800138001", "ren@huawei.com", "深圳市龙岗区坂田华为基地"},
		{"阿里巴巴集团", "ALY001", "马云", "13800138002", "jack@alibaba.com", "杭州市余杭区阿里巴巴园区"},
		{"腾讯科技", "TX001", "马化腾", "13800138003", "pony@tencent.com", "深圳市南山区腾讯大厦"},
		{"京东集团", "JD001", "刘强东", "13800138004", "liu@jd.com", "北京市亦庄经济技术开发区"},
		{"百度在线", "BD001", "李彦宏", "13800138005", "li@baidu.com", "北京市海淀区百度科技园"},
	}

	for _, c := range customers {
		var existing Customer
		if db.Where("code = ?", c.Code).First(&existing).Error != nil {
			customer := Customer{
				Name:          c.Name,
				Code:          c.Code,
				ContactPerson: c.ContactPerson,
				ContactPhone:  c.ContactPhone,
				ContactEmail:  c.ContactEmail,
				Address:       c.Address,
				IsActive:      true,
			}
			db.Create(&customer)
			fmt.Printf("   ✓ 创建客户: %s (%s)\n", c.Name, c.Code)
		} else {
			fmt.Printf("   - 客户已存在: %s\n", c.Name)
		}
	}

	// 获取客户ID
	var hwCustomer, alyCustomer, txCustomer Customer
	db.Where("code = ?", "HW001").First(&hwCustomer)
	db.Where("code = ?", "ALY001").First(&alyCustomer)
	db.Where("code = ?", "TX001").First(&txCustomer)

	// 获取销售人员ID
	var salesUser User
	db.Where("username = ?", "sales_zhao").First(&salesUser)
	if salesUser.ID == 0 {
		db.Where("role = ?", "sales").First(&salesUser)
	}

	// 4. 创建合同
	fmt.Println("\n【4】创建合同...")
	contracts := []struct {
		No         string
		Title      string
		Amount     float64
		CustomerID uint
		TypeID     uint
		Content    string
	}{
		{"CT-2026-0001", "企业ERP系统开发合同", 800000.00, hwCustomer.ID, swType.ID, "开发企业ERP管理系统，包括财务模块、采购模块、销售模块等"},
		{"CT-2026-0002", "云计算平台集成合同", 1200000.00, alyCustomer.ID, intType.ID, "搭建企业级云计算平台，包含服务器、存储、网络等基础设施建设"},
		{"CT-2026-0003", "数字化转型咨询合同", 500000.00, txCustomer.ID, conType.ID, "提供企业数字化转型咨询服务，梳理业务流程，制定数字化方案"},
		{"CT-2026-0004", "智能办公系统合同", 350000.00, hwCustomer.ID, swType.ID, "开发智能办公系统，包含考勤、审批、文档管理等功能"},
		{"CT-2026-0005", "数据中心建设合同", 2000000.00, alyCustomer.ID, intType.ID, "建设企业级数据中心，包含机房装修、服务器部署、网络布线等"},
	}

	for _, c := range contracts {
		var existing Contract
		if db.Where("contract_no = ?", c.No).First(&existing).Error != nil {
			contract := Contract{
				ContractNo:     c.No,
				Title:          c.Title,
				Amount:         c.Amount,
				CustomerID:     c.CustomerID,
				ContractTypeID: c.TypeID,
				Status:         "draft",
				Content:        c.Content,
				CreatorID:      salesUser.ID,
			}
			db.Create(&contract)
			fmt.Printf("   ✓ 创建合同: %s - %s (金额: ¥%.2f)\n", c.No, c.Title, c.Amount)

			// 5. 创建审批工作流
			fmt.Printf("   └ 创建3级审批工作流...\n")
			workflow := ApprovalWorkflow{
				ContractID:   uint64(contract.ID),
				CreatorID:    uint64(salesUser.ID),
				CurrentLevel: 1,
				MaxLevel:     3,
				Status:       "pending",
				CreatorRole:  "sales",
			}
			db.Create(&workflow)

			// 创建审批节点
			nodes := []WorkflowApproval{
				{WfID: workflow.ID, ContractID: uint64(contract.ID), ApproverRole: "sales", Level: 1, Status: "pending"},
				{WfID: workflow.ID, ContractID: uint64(contract.ID), ApproverRole: "director", Level: 2, Status: "pending"},
				{WfID: workflow.ID, ContractID: uint64(contract.ID), ApproverRole: "admin", Level: 3, Status: "pending"},
			}
			for _, node := range nodes {
				db.Create(&node)
			}
			fmt.Printf("   └ 审批节点: 销售 -> 总监 -> 主管\n")
		} else {
			fmt.Printf("   - 合同已存在: %s\n", c.No)
		}
	}

	// 6. 创建已完成部分审批的合同
	fmt.Println("\n【5】创建部分审批中的合同...")
	var pendingContract Contract
	db.Where("contract_no = ?", "CT-2026-0002").First(&pendingContract)
	if pendingContract.ID > 0 {
		var workflow ApprovalWorkflow
		db.Where("contract_id = ?", pendingContract.ID).First(&workflow)
		if workflow.ID > 0 {
			now := time.Now()
			db.Model(&WorkflowApproval{}).Where("workflow_id = ? AND level = 1", workflow.ID).Updates(map[string]interface{}{
				"status":       "approved",
				"approver_ref": 2,
				"comment":      "项目需求明确，同意进入下一阶段",
				"approved_at":  now,
			})
			db.Model(&ApprovalWorkflow{}).Where("id = ?", workflow.ID).Update("current_level", 2)
			fmt.Printf("   ✓ 合同 %s: 第1级审批已完成，等待总监审批\n", pendingContract.ContractNo)
		}
	}

	var pendingContract2 Contract
	db.Where("contract_no = ?", "CT-2026-0004").First(&pendingContract2)
	if pendingContract2.ID > 0 {
		var workflow ApprovalWorkflow
		db.Where("contract_id = ?", pendingContract2.ID).First(&workflow)
		if workflow.ID > 0 {
			now := time.Now()
			db.Model(&WorkflowApproval{}).Where("workflow_id = ? AND level = 1", workflow.ID).Updates(map[string]interface{}{
				"status":       "approved",
				"approver_ref": 2,
				"comment":      "方案可行",
				"approved_at":  now,
			})
			db.Model(&WorkflowApproval{}).Where("workflow_id = ? AND level = 2", workflow.ID).Updates(map[string]interface{}{
				"status":       "approved",
				"approver_ref": 3,
				"comment":      "预算合理，同意",
				"approved_at":  now,
			})
			db.Model(&ApprovalWorkflow{}).Where("id = ?", workflow.ID).Update("current_level", 3)
			fmt.Printf("   ✓ 合同 %s: 前2级审批已完成，等待主管审批\n", pendingContract2.ContractNo)
		}
	}

	fmt.Println("\n============================================")
	fmt.Println("  测试数据初始化完成!")
	fmt.Println("============================================")
	fmt.Println("\n【测试账号】(密码: 123456)")
	fmt.Println("  销售人员: sales_zhao, manager_wang")
	fmt.Println("  总监: director_li")
	fmt.Println("  主管: admin_zhang")
	fmt.Println("  财务: finance_chen")
	fmt.Println("\n【合同状态】")
	fmt.Println("  CT-2026-0001: 待销售审批")
	fmt.Println("  CT-2026-0002: 待总监审批 (第1级已完成)")
	fmt.Println("  CT-2026-0003: 待销售审批")
	fmt.Println("  CT-2026-0004: 待主管审批 (前2级已完成)")
	fmt.Println("  CT-2026-0005: 待销售审批")
	fmt.Println("\n【API端点】")
	fmt.Println("  登录: POST /api/auth/login")
	fmt.Println("  合同列表: GET /api/contracts")
	fmt.Println("  待审批: GET /api/workflow/pending")
	fmt.Println("  审批通过: POST /api/workflow/approve")
	fmt.Println("  审批拒绝: POST /api/workflow/reject")
}
