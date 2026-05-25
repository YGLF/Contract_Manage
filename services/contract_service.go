package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"contract-manage/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ContractService struct{}

func NewContractService() *ContractService {
	return &ContractService{}
}

var (
	ErrControlledContractStatusChange = errors.New("合同关键状态为受控字段，请通过审批流或状态变更申请修改")
	ErrUnsupportedStatusChangeTarget  = errors.New("目标状态不支持通过当前入口变更")
)

type contractStatusTransitionOptions struct {
	EventType   models.LifecycleEventType
	Description string
}

func normalizeContractStatus(status string) string {
	return strings.TrimSpace(status)
}

func (s *ContractService) isValidContractStatus(status string) bool {
	switch models.ContractStatus(normalizeContractStatus(status)) {
	case models.StatusDraft,
		models.StatusPending,
		models.StatusApproved,
		models.StatusActive,
		models.StatusInProgress,
		models.StatusPendingPay,
		models.StatusCompleted,
		models.StatusTerminated,
		models.StatusArchived:
		return true
	default:
		return false
	}
}

func (s *ContractService) isApprovalManagedStatus(status string) bool {
	switch models.ContractStatus(normalizeContractStatus(status)) {
	case models.StatusDraft, models.StatusPending, models.StatusApproved, models.StatusActive:
		return true
	default:
		return false
	}
}

func (s *ContractService) defaultLifecycleForStatus(status models.ContractStatus) (models.LifecycleEventType, string) {
	switch status {
	case models.StatusPending:
		return models.LifecycleSubmitted, "合同提交审批"
	case models.StatusApproved:
		return models.LifecycleApproved, "合同审批通过"
	case models.StatusActive:
		return models.LifecycleActivated, "合同审批通过并生效"
	case models.StatusInProgress:
		return models.LifecycleProgress, "合同进入执行中"
	case models.StatusPendingPay:
		return models.LifecyclePayment, "合同进入待付款"
	case models.StatusCompleted:
		return models.LifecycleCompleted, "合同已完成"
	case models.StatusTerminated:
		return models.LifecycleTerminated, "合同已终止"
	case models.StatusArchived:
		return models.LifecycleArchived, "合同已归档"
	default:
		return "status_changed", "合同状态变更"
	}
}

func (s *ContractService) addLifecycleEventTx(tx *gorm.DB, contractID uint, input LifecycleEventInput, operatorID uint) (*models.ContractLifecycleEvent, error) {
	event := models.ContractLifecycleEvent{
		ContractID:  contractID,
		EventType:   models.LifecycleEventType(input.EventType),
		FromStatus:  input.FromStatus,
		ToStatus:    input.ToStatus,
		Amount:      input.Amount,
		Description: input.Description,
		OperatorID:  operatorID,
	}

	if err := tx.Create(&event).Error; err != nil {
		return nil, err
	}
	return &event, nil
}

func (s *ContractService) getContractForUpdate(tx *gorm.DB, contractID uint) (*models.Contract, error) {
	var contract models.Contract
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&contract, contractID).Error; err != nil {
		return nil, err
	}
	return &contract, nil
}

func (s *ContractService) transitionContractStatusTx(tx *gorm.DB, contractID uint, newStatus models.ContractStatus, operatorID uint, options contractStatusTransitionOptions) (*models.Contract, error) {
	if !s.isValidContractStatus(string(newStatus)) {
		return nil, fmt.Errorf("非法合同状态: %s", newStatus)
	}

	contract, err := s.getContractForUpdate(tx, contractID)
	if err != nil {
		return nil, err
	}

	if contract.Status == newStatus {
		return nil, fmt.Errorf("合同已处于%s状态", newStatus)
	}

	oldStatus := string(contract.Status)
	if err := tx.Model(contract).Update("status", newStatus).Error; err != nil {
		return nil, err
	}
	contract.Status = newStatus

	eventType := options.EventType
	description := options.Description
	if eventType == "" || description == "" {
		defaultEventType, defaultDescription := s.defaultLifecycleForStatus(newStatus)
		if eventType == "" {
			eventType = defaultEventType
		}
		if description == "" {
			description = defaultDescription
		}
	}

	if eventType != "" {
		if _, err := s.addLifecycleEventTx(tx, contractID, LifecycleEventInput{
			EventType:   string(eventType),
			FromStatus:  oldStatus,
			ToStatus:    string(newStatus),
			Description: description,
		}, operatorID); err != nil {
			return nil, err
		}
	}

	return contract, nil
}

func (s *ContractService) validateStatusChangeRequestTransition(fromStatus models.ContractStatus, toStatus models.ContractStatus) error {
	switch fromStatus {
	case models.StatusActive:
		switch toStatus {
		case models.StatusInProgress, models.StatusPendingPay, models.StatusCompleted, models.StatusTerminated, models.StatusArchived:
			return nil
		}
	case models.StatusInProgress:
		switch toStatus {
		case models.StatusPendingPay, models.StatusCompleted, models.StatusTerminated, models.StatusArchived:
			return nil
		}
	case models.StatusPendingPay:
		switch toStatus {
		case models.StatusCompleted, models.StatusTerminated, models.StatusArchived:
			return nil
		}
	case models.StatusCompleted, models.StatusTerminated:
		if toStatus == models.StatusArchived {
			return nil
		}
	}

	return fmt.Errorf("不允许从%s发起到%s的状态变更申请", fromStatus, toStatus)
}

type JSONTime struct {
	time.Time
}

func (t *JSONTime) UnmarshalJSON(data []byte) error {
	str := strings.Trim(string(data), `"`)
	if str == "" || str == "null" {
		t.Time = time.Time{}
		return nil
	}

	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}

	for _, format := range formats {
		if parsed, err := time.Parse(format, str); err == nil {
			t.Time = parsed
			return nil
		}
	}
	return errors.New("invalid date format")
}

type ContractCreateInput struct {
	Title          string    `json:"title" binding:"required"`
	CustomerID     uint      `json:"customer_id" binding:"required"`
	ContractTypeID uint      `json:"contract_type_id" binding:"required"`
	Amount         float64   `json:"amount"`
	Currency       string    `json:"currency"`
	SignDate       *JSONTime `json:"sign_date"`
	StartDate      *JSONTime `json:"start_date"`
	EndDate        *JSONTime `json:"end_date"`
	PaymentTerms   string    `json:"payment_terms"`
	Content        string    `json:"content"`
	Notes          string    `json:"notes"`
}

func (s *ContractService) generateContractNo() string {
	today := time.Now()
	prefix := fmt.Sprintf("CT%s", today.Format("200601"))

	var lastContract models.Contract
	models.DB.Where("contract_no LIKE ?", prefix+"%").Order("contract_no DESC").First(&lastContract)

	var newNo string
	if lastContract.ID != 0 {
		lastNo := lastContract.ContractNo[len(lastContract.ContractNo)-4:]
		var num int
		fmt.Sscanf(lastNo, "%d", &num)
		newNo = fmt.Sprintf("%04d", num+1)
	} else {
		newNo = "0001"
	}

	return prefix + newNo
}

func (s *ContractService) GetContractByID(id uint) (*models.Contract, error) {
	var contract models.Contract
	if err := models.DB.Preload("Customer").Preload("Creator").Preload("ContractType").First(&contract, id).Error; err != nil {
		return nil, err
	}
	return &contract, nil
}

func (s *ContractService) GetContractByNo(contractNo string) (*models.Contract, error) {
	var contract models.Contract
	if err := models.DB.Where("contract_no = ?", contractNo).First(&contract).Error; err != nil {
		return nil, err
	}
	return &contract, nil
}

func (s *ContractService) GetContracts(skip, limit int, customerID, contractTypeID uint, status string) ([]models.Contract, error) {
	var contracts []models.Contract
	query := models.DB.Preload("Customer").Preload("Creator").Preload("ContractType")

	if customerID > 0 {
		query = query.Where("customer_id = ?", customerID)
	}
	if contractTypeID > 0 {
		query = query.Where("contract_type_id = ?", contractTypeID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Order("created_at DESC").Offset(skip).Limit(limit).Find(&contracts).Error; err != nil {
		return nil, err
	}
	return contracts, nil
}

func (s *ContractService) CreateContract(input ContractCreateInput, creatorID uint) (*models.Contract, error) {
	contract := models.Contract{
		ContractNo:     s.generateContractNo(),
		Title:          input.Title,
		CustomerID:     input.CustomerID,
		ContractTypeID: input.ContractTypeID,
		Amount:         input.Amount,
		Currency:       input.Currency,
		PaymentTerms:   input.PaymentTerms,
		Content:        input.Content,
		Notes:          input.Notes,
		CreatorID:      creatorID,
		Status:         models.StatusDraft,
	}

	if input.SignDate != nil && !input.SignDate.Time.IsZero() {
		contract.SignDate = &input.SignDate.Time
	}
	if input.StartDate != nil && !input.StartDate.Time.IsZero() {
		contract.StartDate = &input.StartDate.Time
	}
	if input.EndDate != nil && !input.EndDate.Time.IsZero() {
		contract.EndDate = &input.EndDate.Time
	}

	if contract.Currency == "" {
		contract.Currency = "CNY"
	}

	if err := models.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&contract).Error; err != nil {
			return err
		}

		if _, err := s.addLifecycleEventTx(tx, contract.ID, LifecycleEventInput{
			EventType:   string(models.LifecycleCreated),
			Description: "合同创建",
		}, creatorID); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return &contract, nil
}

type ContractUpdateInput struct {
	Title          string                `json:"title"`
	CustomerID     uint                  `json:"customer_id"`
	ContractTypeID uint                  `json:"contract_type_id"`
	Amount         float64               `json:"amount"`
	Currency       string                `json:"currency"`
	Status         models.ContractStatus `json:"status"`
	SignDate       *JSONTime             `json:"sign_date"`
	StartDate      *JSONTime             `json:"start_date"`
	EndDate        *JSONTime             `json:"end_date"`
	PaymentTerms   string                `json:"payment_terms"`
	Content        string                `json:"content"`
	Notes          string                `json:"notes"`
}

func (s *ContractService) UpdateContract(id uint, input ContractUpdateInput) (*models.Contract, error) {
	contract, err := s.GetContractByID(id)
	if err != nil {
		return nil, err
	}

	if normalizeContractStatus(string(input.Status)) != "" {
		return nil, ErrControlledContractStatusChange
	}

	updates := map[string]interface{}{}
	if input.Title != "" {
		updates["title"] = input.Title
	}
	if input.CustomerID > 0 {
		updates["customer_id"] = input.CustomerID
	}
	if input.ContractTypeID > 0 {
		updates["contract_type_id"] = input.ContractTypeID
	}
	if input.Amount > 0 {
		updates["amount"] = input.Amount
	}
	if input.Currency != "" {
		updates["currency"] = input.Currency
	}
	if input.SignDate != nil && !input.SignDate.Time.IsZero() {
		updates["sign_date"] = input.SignDate.Time
	}
	if input.StartDate != nil && !input.StartDate.Time.IsZero() {
		updates["start_date"] = input.StartDate.Time
	}
	if input.EndDate != nil && !input.EndDate.Time.IsZero() {
		updates["end_date"] = input.EndDate.Time
	}
	if input.PaymentTerms != "" {
		updates["payment_terms"] = input.PaymentTerms
	}
	if input.Content != "" {
		updates["content"] = input.Content
	}
	if input.Notes != "" {
		updates["notes"] = input.Notes
	}

	if err := models.DB.Model(contract).Updates(updates).Error; err != nil {
		return nil, err
	}
	return contract, nil
}

func (s *ContractService) DeleteContract(id uint) error {
	result := models.DB.Delete(&models.Contract{}, id)
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return result.Error
}

func (s *ContractService) GetContractExecutionByID(id uint) (*models.ContractExecution, error) {
	var execution models.ContractExecution
	if err := models.DB.First(&execution, id).Error; err != nil {
		return nil, err
	}
	return &execution, nil
}

func (s *ContractService) GetContractExecutions(contractID uint) ([]models.ContractExecution, error) {
	var executions []models.ContractExecution
	if err := models.DB.Where("contract_id = ?", contractID).Order("created_at DESC").Find(&executions).Error; err != nil {
		return nil, err
	}
	return executions, nil
}

type ContractExecutionCreateInput struct {
	ContractID    uint      `json:"contract_id"`
	Stage         string    `json:"stage"`
	StageDate     *JSONTime `json:"stage_date"`
	Progress      float64   `json:"progress"`
	PaymentAmount float64   `json:"payment_amount"`
	PaymentDate   *JSONTime `json:"payment_date"`
	Description   string    `json:"description"`
}

func (s *ContractService) CreateContractExecution(input ContractExecutionCreateInput, operatorID uint) (*models.ContractExecution, error) {
	execution := models.ContractExecution{
		ContractID:    input.ContractID,
		Stage:         input.Stage,
		Progress:      input.Progress,
		PaymentAmount: input.PaymentAmount,
		Description:   input.Description,
		OperatorID:    operatorID,
	}

	if input.StageDate != nil && !input.StageDate.Time.IsZero() {
		execution.StageDate = &input.StageDate.Time
	}
	if input.PaymentDate != nil && !input.PaymentDate.Time.IsZero() {
		execution.PaymentDate = &input.PaymentDate.Time
	}

	if err := models.DB.Create(&execution).Error; err != nil {
		return nil, err
	}
	return &execution, nil
}

func (s *ContractService) DeleteExecution(id uint) error {
	return models.DB.Delete(&models.ContractExecution{}, id).Error
}

func (s *ContractService) GetDocumentByID(id uint) (*models.Document, error) {
	var document models.Document
	if err := models.DB.First(&document, id).Error; err != nil {
		return nil, err
	}
	return &document, nil
}

func (s *ContractService) GetDocuments(contractID uint) ([]models.Document, error) {
	var documents []models.Document
	if err := models.DB.Where("contract_id = ?", contractID).Order("created_at DESC").Find(&documents).Error; err != nil {
		return nil, err
	}
	return documents, nil
}

type DocumentCreateInput struct {
	ContractID uint   `json:"contract_id"`
	Name       string `json:"name" binding:"required"`
	FilePath   string `json:"file_path" binding:"required"`
	FileSize   int    `json:"file_size" binding:"required"`
	FileType   string `json:"file_type"`
}

func (s *ContractService) CreateDocument(input DocumentCreateInput, uploaderID uint) (*models.Document, error) {
	document := models.Document{
		ContractID: input.ContractID,
		Name:       input.Name,
		FilePath:   input.FilePath,
		FileSize:   input.FileSize,
		FileType:   input.FileType,
		Version:    "1.0",
		UploaderID: uploaderID,
	}

	if err := models.DB.Create(&document).Error; err != nil {
		return nil, err
	}
	return &document, nil
}

func (s *ContractService) DeleteDocument(id uint) error {
	result := models.DB.Delete(&models.Document{}, id)
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return result.Error
}

type LifecycleEventInput struct {
	EventType   string  `json:"event_type"`
	FromStatus  string  `json:"from_status"`
	ToStatus    string  `json:"to_status"`
	Amount      float64 `json:"amount"`
	Description string  `json:"description"`
}

func (s *ContractService) AddLifecycleEvent(contractID uint, input LifecycleEventInput, operatorID uint) (*models.ContractLifecycleEvent, error) {
	return s.addLifecycleEventTx(models.DB, contractID, input, operatorID)
}

func (s *ContractService) GetLifecycleEvents(contractID uint) ([]models.ContractLifecycleEvent, error) {
	var events []models.ContractLifecycleEvent
	if err := models.DB.Where("contract_id = ?", contractID).Order("created_at ASC").Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}

func (s *ContractService) UpdateContractStatus(contractID uint, newStatus string, operatorID uint) (*models.Contract, error) {
	normalizedStatus := normalizeContractStatus(newStatus)
	if !s.isValidContractStatus(normalizedStatus) {
		return nil, fmt.Errorf("非法合同状态: %s", newStatus)
	}

	if s.isApprovalManagedStatus(normalizedStatus) || s.IsStatusChangeRequireApproval(normalizedStatus) {
		return nil, ErrControlledContractStatusChange
	}

	return nil, ErrUnsupportedStatusChangeTarget
}

func (s *ContractService) ArchiveContract(contractID uint, operatorID uint) (*models.Contract, error) {
	return nil, ErrControlledContractStatusChange
}

var StatusChangeRequireApproval = []string{
	"archived",
	"terminated",
	"in_progress",
	"pending_pay",
	"completed",
}

func (s *ContractService) IsStatusChangeRequireApproval(newStatus string) bool {
	for _, status := range StatusChangeRequireApproval {
		if status == newStatus {
			return true
		}
	}
	return false
}

type StatusChangeRequestInput struct {
	ToStatus string `json:"to_status" binding:"required"`
	Reason   string `json:"reason"`
}

func (s *ContractService) CreateStatusChangeRequest(contractID uint, input StatusChangeRequestInput, requesterID uint) (*models.StatusChangeRequest, error) {
	toStatus := normalizeContractStatus(input.ToStatus)
	if !s.isValidContractStatus(toStatus) {
		return nil, fmt.Errorf("非法合同状态: %s", input.ToStatus)
	}
	if !s.IsStatusChangeRequireApproval(toStatus) {
		return nil, ErrUnsupportedStatusChangeTarget
	}

	var request models.StatusChangeRequest
	if err := models.DB.Transaction(func(tx *gorm.DB) error {
		contract, err := s.getContractForUpdate(tx, contractID)
		if err != nil {
			return err
		}
		if contract.Status == models.ContractStatus(toStatus) {
			return fmt.Errorf("合同已处于%s状态", toStatus)
		}
		if err := s.validateStatusChangeRequestTransition(contract.Status, models.ContractStatus(toStatus)); err != nil {
			return err
		}

		var existingRequest models.StatusChangeRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("contract_id = ? AND status = ?", contractID, "pending").
			First(&existingRequest).Error; err == nil {
			return fmt.Errorf("该合同已有待审核的状态变更申请")
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		request = models.StatusChangeRequest{
			ContractID:  contractID,
			FromStatus:  string(contract.Status),
			ToStatus:    toStatus,
			Reason:      input.Reason,
			RequesterID: requesterID,
			Status:      "pending",
		}

		return tx.Create(&request).Error
	}); err != nil {
		return nil, err
	}

	return &request, nil
}

func (s *ContractService) GetStatusChangeRequests(contractID uint) ([]models.StatusChangeRequest, error) {
	var requests []models.StatusChangeRequest
	if err := models.DB.Preload("Requester").Preload("Approver").Where("contract_id = ?", contractID).Order("created_at DESC").Find(&requests).Error; err != nil {
		return nil, err
	}
	return requests, nil
}

func (s *ContractService) GetPendingStatusChangeRequests(role string) ([]models.StatusChangeRequest, error) {
	var requests []models.StatusChangeRequest
	query := models.DB.Preload("Contract.Customer").Preload("Requester").Order("created_at DESC")

	if role == "manager" || role == "admin" {
		query = query.Where("status = ?", "pending")
	}

	if err := query.Find(&requests).Error; err != nil {
		return nil, err
	}
	return requests, nil
}

func (s *ContractService) ApproveStatusChangeRequest(requestID uint, approverID uint, comment string) (*models.StatusChangeRequest, error) {
	var request models.StatusChangeRequest
	if err := models.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&request, requestID).Error; err != nil {
			return err
		}

		if request.Status != "pending" {
			return fmt.Errorf("该申请已被处理")
		}

		contract, err := s.getContractForUpdate(tx, request.ContractID)
		if err != nil {
			return err
		}
		if string(contract.Status) != request.FromStatus {
			return fmt.Errorf("合同当前状态已变更，请重新发起申请")
		}

		now := time.Now()
		request.Status = "approved"
		request.ApproverID = &approverID
		request.Comment = comment
		request.ApprovedAt = &now

		if err := tx.Save(&request).Error; err != nil {
			return err
		}

		_, err = s.transitionContractStatusTx(tx, request.ContractID, models.ContractStatus(request.ToStatus), approverID, contractStatusTransitionOptions{})
		return err
	}); err != nil {
		return nil, err
	}

	return &request, nil
}

func (s *ContractService) RejectStatusChangeRequest(requestID uint, approverID uint, comment string) (*models.StatusChangeRequest, error) {
	var request models.StatusChangeRequest
	if err := models.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&request, requestID).Error; err != nil {
			return err
		}

		if request.Status != "pending" {
			return fmt.Errorf("该申请已被处理")
		}

		now := time.Now()
		request.Status = "rejected"
		request.ApproverID = &approverID
		request.Comment = comment
		request.ApprovedAt = &now

		return tx.Save(&request).Error
	}); err != nil {
		return nil, err
	}

	return &request, nil
}
