package services

import (
	"contract-manage/models"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupWorkflowTestDB(t *testing.T) func() {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to connect workflow test database: %v", err)
	}

	err = db.AutoMigrate(
		&models.User{},
		&models.Customer{},
		&models.ContractType{},
		&models.Contract{},
		&models.ContractExecution{},
		&models.ApprovalRecord{},
		&models.Document{},
		&models.ContractLifecycleEvent{},
		&models.StatusChangeRequest{},
		&models.Reminder{},
		&models.AuditLog{},
		&models.ApprovalWorkflow{},
		&models.WorkflowApproval{},
	)
	if err != nil {
		t.Fatalf("failed to migrate workflow test database: %v", err)
	}

	models.DB = db

	userService := NewUserService()
	_, _ = userService.CreateUser(UserCreateInput{
		Username: "admin",
		Email:    "admin@example.com",
		Password: "password123",
		Role:     models.RoleAdmin,
	})
	_, _ = userService.CreateUser(UserCreateInput{
		Username: "manager",
		Email:    "manager@example.com",
		Password: "password123",
		Role:     models.RoleManager,
	})
	_, _ = userService.CreateUser(UserCreateInput{
		Username: "user",
		Email:    "user@example.com",
		Password: "password123",
		Role:     models.RoleUser,
	})

	customerService := NewCustomerService()
	_, _ = customerService.CreateCustomer(CustomerCreateInput{
		Name: "测试客户",
		Type: "customer",
		Code: "C001",
	})

	db.Create(&models.ContractType{
		Name: "采购合同",
		Code: "PO001",
	})

	return func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}
}

func seedWorkflowApprovalScenario(t *testing.T) (*WorkflowService, *models.ApprovalWorkflow) {
	t.Helper()

	service := NewWorkflowService(models.DB)

	contract := models.Contract{
		ContractNo:     "WF001",
		Title:          "工作流测试合同",
		CustomerID:     1,
		ContractTypeID: 1,
		CreatorID:      3,
		Status:         models.StatusPending,
	}
	if err := models.DB.Create(&contract).Error; err != nil {
		t.Fatalf("failed to create contract: %v", err)
	}

	workflow := models.ApprovalWorkflow{
		ContractID:   uint64(contract.ID),
		CurrentLevel: 1,
		MaxLevel:     2,
		Status:       models.WorkflowStatusPending,
		CreatorRole:  string(models.RoleUser),
	}
	if err := models.DB.Create(&workflow).Error; err != nil {
		t.Fatalf("failed to create workflow: %v", err)
	}

	approvals := []models.WorkflowApproval{
		{
			WorkflowID:   workflow.ID,
			ContractID:   uint64(contract.ID),
			ApproverRole: string(models.RoleManager),
			Level:        1,
			Status:       models.WorkflowStatusPending,
		},
		{
			WorkflowID:   workflow.ID,
			ContractID:   uint64(contract.ID),
			ApproverRole: string(models.RoleAdmin),
			Level:        2,
			Status:       models.WorkflowStatusPending,
		},
	}
	if err := models.DB.Create(&approvals).Error; err != nil {
		t.Fatalf("failed to create workflow approvals: %v", err)
	}

	return service, &workflow
}

func TestWorkflowService_ApproveRequiresMatchingRole(t *testing.T) {
	cleanup := setupWorkflowTestDB(t)
	defer cleanup()

	service, workflow := seedWorkflowApprovalScenario(t)

	if err := service.Approve(workflow.ID, 1, 2, string(models.RoleManager), "经理同意"); err != nil {
		t.Fatalf("expected manager to approve first level, got error: %v", err)
	}

	var currentWorkflow models.ApprovalWorkflow
	if err := models.DB.First(&currentWorkflow, workflow.ID).Error; err != nil {
		t.Fatalf("failed to reload workflow: %v", err)
	}
	if currentWorkflow.CurrentLevel != 2 {
		t.Fatalf("expected workflow to advance to level 2, got %d", currentWorkflow.CurrentLevel)
	}
}

func TestWorkflowService_ApproveRejectsWrongRole(t *testing.T) {
	cleanup := setupWorkflowTestDB(t)
	defer cleanup()

	service, workflow := seedWorkflowApprovalScenario(t)

	err := service.Approve(workflow.ID, 1, 1, string(models.RoleAdmin), "管理员越级审批")
	if err == nil {
		t.Fatal("expected approval to fail for mismatched role")
	}
}

func TestWorkflowService_RejectRequiresMatchingRole(t *testing.T) {
	cleanup := setupWorkflowTestDB(t)
	defer cleanup()

	service, workflow := seedWorkflowApprovalScenario(t)

	err := service.Reject(workflow.ID, 1, 1, string(models.RoleAdmin), "管理员越级拒绝")
	if err == nil {
		t.Fatal("expected rejection to fail for mismatched role")
	}

	if err := service.Reject(workflow.ID, 1, 2, string(models.RoleManager), "经理拒绝"); err != nil {
		t.Fatalf("expected manager to reject first level, got error: %v", err)
	}

	var currentWorkflow models.ApprovalWorkflow
	if err := models.DB.First(&currentWorkflow, workflow.ID).Error; err != nil {
		t.Fatalf("failed to reload workflow: %v", err)
	}
	if currentWorkflow.Status != models.WorkflowStatusRejected {
		t.Fatalf("expected workflow to be rejected, got %s", currentWorkflow.Status)
	}
}
