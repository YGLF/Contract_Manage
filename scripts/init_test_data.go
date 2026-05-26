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

func main() {
	dsn := "root:rootroots@tcp(192.168.112.1:3306)/contract_manage?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("连接数据库失败:", err)
	}

	fmt.Println("=== 安信合同管理系统 - 初始化测试数据 ===\n")

	clearTestData(db)
	createUsers(db)
	createCustomers(db)
	createContractTypes(db)
	createContracts(db)
	createWorkflows(db)
	createNotifications(db)
	createAuditLogs(db)

	fmt.Println("\n=== 测试数据创建完成 ===")
	printSummary()
}

func clearTestData(db *gorm.DB) {
	fmt.Println("[1/8] 清理旧测试数据...")
	db.Exec("DELETE FROM audit_logs")
	db.Exec("DELETE FROM notifications")
	db.Exec("DELETE FROM workflow_approvals")
	db.Exec("DELETE FROM approval_workflows")
	db.Exec("DELETE FROM contract_lifecycle_events")
	db.Exec("DELETE FROM contract_executions")
	db.Exec("DELETE FROM contract_documents")
	db.Exec("DELETE FROM status_change_requests")
	db.Exec("DELETE FROM contracts WHERE id > 0")
	db.Exec("DELETE FROM customers WHERE id > 0")
	db.Exec("DELETE FROM contract_types WHERE id > 0")
	fmt.Println("      ✓ 清理完成")
}

func createUsers(db *gorm.DB) {
	fmt.Println("[2/8] 创建测试用户...")
	users := []struct {
		username   string
		password   string
		fullName   string
		role       string
		email      string
		department string
	}{
		{"admin", "admin@123456", "系统管理员", "admin", "admin@anxin.com", "信息技术部"},
		{"auditadmin", "auditadmin@123456", "审计管理员", "audit_admin", "audit@anxin.com", "审计部"},
		{"sales01", "123456", "张三丰", "sales", "zhang.sales@anxin.com", "销售部"},
		{"sales02", "123456", "李四海", "sales", "li.sales@anxin.com", "销售部"},
		{"sales03", "123456", "王五湖", "sales", "wang.sales@anxin.com", "销售部"},
		{"sales_director", "123456", "赵明", "sales_director", "zhao.director@anxin.com", "销售部"},
		{"tech_director", "123456", "孙建国", "tech_director", "sun.tech@anxin.com", "技术部"},
		{"finance_director", "123456", "周财务", "finance_director", "zhou.finance@anxin.com", "财务部"},
		{"contract_admin", "123456", "吴合同", "contract_admin", "wu.contract@anxin.com", "法务部"},
	}

	for _, u := range users {
		hash, _ := bcrypt.GenerateFromPassword([]byte(u.password), bcrypt.DefaultCost)
		passwordHash := fmt.Sprintf("%x", hash)
		db.Exec(`INSERT INTO users (username, email, hashed_password, password_hash, full_name, role, department, is_active, account_status, created_at, updated_at) 
			VALUES (?, ?, ?, ?, ?, ?, ?, 1, 'permanent', ?, ?)`,
			u.username, u.email, string(hash), passwordHash, u.fullName, u.role, u.department, time.Now(), nil)
		fmt.Printf("      ✓ 用户: %s (%s)\n", u.username, u.fullName)
	}
}

func createCustomers(db *gorm.DB) {
	fmt.Println("[3/8] 创建测试客户...")
	customers := []struct {
		name    string
		code    string
		contact string
		phone   string
		address string
	}{
		{"深圳市腾讯科技有限公司", "CUST001", "马化腾", "13800138000", "深圳市南山区科技园"},
		{"阿里巴巴（中国）有限公司", "CUST002", "马云", "13900139000", "杭州市滨江区阿里巴巴园区"},
		{"北京百度网讯科技有限公司", "CUST003", "李彦宏", "13700137000", "北京市海淀区百度科技园"},
		{"华为技术有限公司", "CUST004", "任正非", "13600136000", "深圳市龙岗区华为总部"},
		{"京东集团", "CUST005", "刘强东", "13500135000", "北京市亦庄经济开发区"},
		{"小米科技有限责任公司", "CUST006", "雷军", "13400134000", "北京市海淀区小米科技园"},
		{"美团", "CUST007", "王兴", "13300133000", "北京市朝阳区望京"},
		{"字节跳动科技有限公司", "CUST008", "张一鸣", "13200132000", "北京市海淀区知春路"},
		{"网易公司", "CUST009", "丁磊", "13100131000", "杭州市滨江区网易大厦"},
		{"拼多多", "CUST010", "黄峥", "13000130000", "上海市长宁区拼多多总部"},
	}

	for _, c := range customers {
		db.Exec(`INSERT INTO customers (name, code, contact_person, phone, address, creator_id, created_at, updated_at) 
			VALUES (?, ?, ?, ?, ?, 1, ?, ?)`,
			c.name, c.code, c.contact, c.phone, c.address, time.Now(), nil)
	}
	fmt.Printf("      ✓ 创建 %d 个客户\n", len(customers))
}

func createContractTypes(db *gorm.DB) {
	fmt.Println("[4/8] 创建合同类型...")
	types := []struct {
		name string
		desc string
	}{
		{"软件开发合同", "软件系统开发服务"},
		{"系统集成合同", "系统集成项目服务"},
		{"技术服务合同", "技术咨询和运维服务"},
		{"设备采购合同", "设备采购供应"},
		{"租赁合同", "设备或场地租赁"},
		{"采购合同", "物资采购供应"},
		{"咨询合同", "管理和技术咨询"},
		{"维护合同", "系统维护服务"},
	}

	for _, t := range types {
		db.Exec(`INSERT INTO contract_types (name, description, creator_id, created_at, updated_at) VALUES (?, ?, 1, ?, ?)`,
			t.name, t.desc, time.Now(), nil)
	}
	fmt.Printf("      ✓ 创建 %d 个合同类型\n", len(types))
}

func createContracts(db *gorm.DB) {
	fmt.Println("[5/8] 创建测试合同...")
	contracts := []struct {
		no         string
		title      string
		customerID int
		typeID     int
		amount     float64
		status     string
		creatorID  int
	}{
		// 草稿状态
		{"CT2024-001", "企业内部管理系统开发合同", 1, 1, 180000.00, "draft", 3},
		{"CT2024-002", "云计算平台集成项目", 2, 2, 350000.00, "draft", 4},
		// 待审批状态 - 等待销售总监
		{"CT2024-003", "移动APP开发服务合同", 3, 1, 220000.00, "pending", 3},
		{"CT2024-004", "网络安全升级项目", 4, 2, 280000.00, "pending", 4},
		{"CT2024-005", "数据分析平台合同", 5, 1, 160000.00, "pending", 3},
		// 待审批状态 - 等待技术总监（销售已通过）
		{"CT2024-006", "智能办公系统合同", 6, 3, 450000.00, "pending", 4},
		{"CT2024-007", "数据库优化服务", 7, 3, 120000.00, "pending", 3},
		// 待审批状态 - 等待财务总监（销售+技术已通过）
		{"CT2024-008", "企业ERP系统采购", 8, 4, 680000.00, "pending", 4},
		{"CT2024-009", "服务器设备采购", 9, 4, 520000.00, "pending", 3},
		// 已生效状态（已完成审批）
		{"CT2024-010", "网站改版升级合同", 10, 1, 95000.00, "active", 3},
		{"CT2024-011", "视频会议系统集成", 1, 2, 185000.00, "active", 4},
		// 执行中状态
		{"CT2024-012", "电商平台开发合同", 2, 1, 420000.00, "in_progress", 3},
		{"CT2024-013", "智能仓储系统", 3, 2, 380000.00, "in_progress", 4},
		// 待付款状态
		{"CT2024-014", "办公设备采购合同", 4, 4, 156000.00, "pending_pay", 3},
		{"CT2024-015", "软件授权许可合同", 5, 1, 280000.00, "pending_pay", 4},
		// 已完成状态
		{"CT2024-016", "IT运维服务合同", 6, 8, 96000.00, "completed", 3},
		{"CT2024-017", "技术咨询顾问合同", 7, 7, 75000.00, "completed", 4},
		// 已归档状态
		{"CT2024-018", "去年项目结算合同", 8, 1, 125000.00, "archived", 3},
		{"CT2024-019", "历史合同补充协议", 9, 3, 45000.00, "archived", 4},
		// 已终止状态
		{"CT2024-020", "项目终止补充协议", 10, 1, 0, "terminated", 3},
	}

	for _, c := range contracts {
		db.Exec(`INSERT INTO contracts (contract_no, title, customer_id, contract_type_id, amount, currency, status, creator_id, created_at, updated_at) 
			VALUES (?, ?, ?, ?, ?, 'CNY', ?, ?, ?, ?)`,
			c.no, c.title, c.customerID, c.typeID, c.amount, c.status, c.creatorID, time.Now(), nil)

		// 为已生效/执行中/已完成/已归档的合同创建执行记录
		if c.status == "in_progress" || c.status == "completed" || c.status == "archived" {
			var contractID uint
			db.Raw("SELECT LAST_INSERT_ID()").Scan(&contractID)
			createContractExecutions(db, contractID, c.status)
		}
	}
	fmt.Printf("      ✓ 创建 %d 个合同\n", len(contracts))
}

func createContractExecutions(db *gorm.DB, contractID uint, status string) {
	executions := []struct {
		phase   string
		content string
	}{
		{"启动", "项目启动会议，确认需求"},
		{"设计", "系统设计和方案确认"},
		{"开发", "功能开发和单元测试"},
		{"测试", "系统测试和问题修复"},
	}

	for _, e := range executions {
		execStatus := "completed"
		if status == "in_progress" && e.phase == "测试" {
			execStatus = "in_progress"
		}
		db.Exec(`INSERT INTO contract_executions (contract_id, phase, content, status, operator_id, executed_at, created_at) 
			VALUES (?, ?, ?, ?, 1, ?, ?)`,
			contractID, e.phase, e.content, execStatus, time.Now(), time.Now())
	}
}

func createWorkflows(db *gorm.DB) {
	fmt.Println("[6/8] 创建审批工作流...")

	// 待销售总监审批
	createWorkflow(db, 3, 3, 1, "pending")
	createWorkflow(db, 4, 4, 1, "pending")
	createWorkflow(db, 5, 3, 1, "pending")

	// 待技术总监审批（销售已通过）
	createWorkflowWithLevel(db, 6, 4, 2, 1)
	createWorkflowWithLevel(db, 7, 3, 2, 1)

	// 待财务总监审批（销售+技术已通过）
	createWorkflowWithLevel(db, 8, 4, 3, 2)
	createWorkflowWithLevel(db, 9, 3, 3, 2)

	// 已完成审批（已生效）
	createCompletedWorkflow(db, 10)
	createCompletedWorkflow(db, 11)

	fmt.Println("      ✓ 创建审批工作流")
}

func createWorkflow(db *gorm.DB, contractID, creatorID, currentLevel int, status string) {
	db.Exec(`INSERT INTO approval_workflows (contract_id, creator_id, current_level, max_level, status, creator_role, created_at, updated_at) 
		VALUES (?, ?, 1, 3, 'pending', 'sales', ?, ?)`,
		contractID, creatorID, time.Now(), time.Now())

	var workflowID uint
	db.Raw("SELECT LAST_INSERT_ID()").Scan(&workflowID)

	db.Exec(`INSERT INTO workflow_approvals (workflow_id, contract_id, approver_role, level, status, created_at) VALUES 
		(?, ?, 'sales_director', 1, 'pending', ?),
		(?, ?, 'tech_director', 2, 'pending', ?),
		(?, ?, 'finance_director', 3, 'pending', ?)`,
		workflowID, contractID, time.Now(),
		workflowID, contractID, time.Now(),
		workflowID, contractID, time.Now())
}

func createWorkflowWithLevel(db *gorm.DB, contractID, creatorID, currentLevel, approvedLevels int) {
	db.Exec(`INSERT INTO approval_workflows (contract_id, creator_id, current_level, max_level, status, creator_role, created_at, updated_at) 
		VALUES (?, ?, ?, 3, 'pending', 'sales', ?, ?)`,
		contractID, creatorID, currentLevel, time.Now(), time.Now())

	var workflowID uint
	db.Raw("SELECT LAST_INSERT_ID()").Scan(&workflowID)

	// 插入审批节点，部分已通过
	for level := 1; level <= 3; level++ {
		nodeStatus := "pending"
		approverID := 0
		comment := ""
		if level <= approvedLevels {
			nodeStatus = "approved"
			approverID = level
			comment = "同意"
		}
		if approverID > 0 {
			db.Exec(`INSERT INTO workflow_approvals (workflow_id, contract_id, approver_role, level, status, approver_id, comment, approved_at, created_at) 
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				workflowID, contractID, getRoleByLevel(level), level, nodeStatus, approverID, comment, time.Now(), time.Now())
		} else {
			db.Exec(`INSERT INTO workflow_approvals (workflow_id, contract_id, approver_role, level, status, created_at) 
				VALUES (?, ?, ?, ?, 'pending', ?)`,
				workflowID, contractID, getRoleByLevel(level), time.Now())
		}
	}
}

func createCompletedWorkflow(db *gorm.DB, contractID int) {
	db.Exec(`INSERT INTO approval_workflows (contract_id, creator_id, current_level, max_level, status, creator_role, created_at, updated_at) 
		VALUES (?, 3, 3, 3, 'completed', 'sales', ?, ?)`,
		contractID, time.Now(), time.Now())

	var workflowID uint
	db.Raw("SELECT LAST_INSERT_ID()").Scan(&workflowID)

	for level := 1; level <= 3; level++ {
		db.Exec(`INSERT INTO workflow_approvals (workflow_id, contract_id, approver_role, level, status, approver_id, comment, approved_at, created_at) 
			VALUES (?, ?, ?, ?, 'approved', ?, '同意', ?, ?)`,
			workflowID, contractID, getRoleByLevel(level), level, time.Now(), time.Now(), time.Now())
	}
}

func getRoleByLevel(level int) string {
	switch level {
	case 1:
		return "sales_director"
	case 2:
		return "tech_director"
	case 3:
		return "finance_director"
	default:
		return ""
	}
}

func createNotifications(db *gorm.DB) {
	fmt.Println("[7/8] 创建通知消息...")
	notifications := []struct {
		userID     int
		contractID int
		title      string
		content    string
		notifType  string
		isRead     bool
	}{
		{3, 3, "合同待审批", "您有一个合同等待审批：CT2024-003", "approval_reminder", false},
		{3, 4, "合同待审批", "您有一个合同等待审批：CT2024-004", "approval_reminder", false},
		{3, 5, "合同待审批", "您有一个合同等待审批：CT2024-005", "approval_reminder", false},
		{6, 3, "合同待审批", "您有一个合同等待审批：CT2024-003", "approval_reminder", false},
		{7, 6, "合同待审批", "您有一个合同等待审批：CT2024-006", "approval_reminder", false},
		{8, 8, "合同待审批", "您有一个合同等待审批：CT2024-008", "approval_reminder", false},
		{3, 10, "审批通过", "您的合同CT2024-010已通过全部审批", "approved", true},
		{3, 12, "执行提醒", "合同CT2024-012进入执行阶段", "info", false},
		{4, 11, "审批通过", "您的合同CT2024-011已通过全部审批", "approved", true},
		{5, 1, "系统消息", "欢迎使用安信合同管理系统", "system", true},
	}

	for _, n := range notifications {
		db.Exec(`INSERT INTO notifications (user_id, contract_id, role, type, title, content, is_read, created_at) 
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			n.userID, n.contractID, "sales", n.notifType, n.title, n.content, n.isRead, time.Now())
	}
	fmt.Printf("      ✓ 创建 %d 条通知\n", len(notifications))
}

func createAuditLogs(db *gorm.DB) {
	fmt.Println("[8/8] 创建审计日志...")
	actions := []struct {
		userID   int
		action   string
		resource string
		detail   string
		ip       string
	}{
		{1, "登录系统", "auth", "用户登录成功", "192.168.1.100"},
		{3, "创建合同", "contract", "创建合同 CT2024-001", "192.168.1.101"},
		{3, "创建合同", "contract", "创建合同 CT2024-003", "192.168.1.101"},
		{4, "创建合同", "contract", "创建合同 CT2024-004", "192.168.1.102"},
		{6, "审批通过", "workflow", "审批通过合同 CT2024-003 第1级", "192.168.1.103"},
		{7, "审批通过", "workflow", "审批通过合同 CT2024-006 第2级", "192.168.1.104"},
		{8, "审批通过", "workflow", "审批通过合同 CT2024-008 第3级", "192.168.1.105"},
		{1, "创建用户", "user", "创建用户 sales02", "192.168.1.100"},
		{1, "创建客户", "customer", "创建客户 深圳市腾讯科技有限公司", "192.168.1.100"},
		{2, "查看审计日志", "audit", "查看系统操作日志", "192.168.1.106"},
	}

	for _, a := range actions {
		db.Exec(`INSERT INTO audit_logs (user_id, action, resource, detail, ip_address, created_at) 
			VALUES (?, ?, ?, ?, ?, ?)`,
			a.userID, a.action, a.resource, a.detail, a.ip, time.Now())
	}
	fmt.Printf("      ✓ 创建 %d 条审计日志\n", len(actions))
}

func printSummary() {
	fmt.Println("\n=== 数据统计 ===")
	fmt.Println("用户: 9个 (admin, auditadmin, sales01~03, 销售/技术/财务总监, 合同管理员)")
	fmt.Println("客户: 10个 (知名企业)")
	fmt.Println("合同类型: 8个")
	fmt.Println("合同: 20个 (包含各种状态)")
	fmt.Println("工作流: 13个")
	fmt.Println("通知: 10条")
	fmt.Println("审计日志: 10条")

	fmt.Println("\n=== 测试账号 ===")
	fmt.Println("账号          密码            角色")
	fmt.Println("-----------------------------------")
	fmt.Println("admin         admin@123456    超级管理员")
	fmt.Println("auditadmin    auditadmin@123456 审计管理员")
	fmt.Println("sales01       123456         销售人员")
	fmt.Println("sales02       123456         销售人员")
	fmt.Println("sales_director 123456        销售总监")
	fmt.Println("tech_director 123456         技术总监")
	fmt.Println("finance_director 123456      财务总监")
	fmt.Println("contract_admin 123456        合同管理员")
}
