package approvalworkflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"contract-manage/pkg/microplatform/httpx"
	"contract-manage/pkg/microplatform/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Request struct {
	ID          string                 `json:"id"`
	ContractID  string                 `json:"contract_id"`
	Department  string                 `json:"department,omitempty"`
	RequestType string                 `json:"request_type"`
	ResourceID  string                 `json:"resource_id,omitempty"`
	Status      string                 `json:"status"`
	RequestedBy string                 `json:"requested_by"`
	Payload     map[string]interface{} `json:"payload,omitempty"`
	ApprovedBy  string                 `json:"approved_by,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	ApprovedAt  time.Time              `json:"approved_at,omitempty"`
	ExpiresAt   time.Time              `json:"expires_at,omitempty"`
	ConsumedAt  time.Time              `json:"consumed_at,omitempty"`
	ConsumedBy  string                 `json:"consumed_by,omitempty"`
}

type RequestRecord struct {
	ID          string `gorm:"primaryKey;size:64"`
	ContractID  string `gorm:"size:64;index;not null"`
	Department  string `gorm:"size:64;index"`
	RequestType string `gorm:"size:64;index;not null"`
	ResourceID  string `gorm:"size:64;index"`
	Status      string `gorm:"size:32;index;not null"`
	RequestedBy string `gorm:"size:64;index;not null"`
	PayloadRaw  string `gorm:"type:text"`
	ApprovedBy  string `gorm:"size:64"`
	CreatedAt   time.Time
	ApprovedAt  *time.Time
	ExpiresAt   *time.Time `gorm:"index"`
	ConsumedAt  *time.Time `gorm:"index"`
	ConsumedBy  string     `gorm:"size:64"`
}

type Service struct {
	mu                    sync.RWMutex
	requests              map[string]Request
	seq                   int
	db                    *gorm.DB
	contractServiceURL    string
	performanceServiceURL string
	archiveServiceURL     string
	reportServiceURL      string
}

var allowedRequestTypes = map[string]struct{}{
	"status_change":   {},
	"plan_adjustment": {},
	"archive_borrow":  {},
	"archive_destroy": {},
	"report_export":   {},
}

func New() *Service {
	return &Service{
		requests: make(map[string]Request),
	}
}

func NewWithDB(db *gorm.DB) *Service {
	service := New()
	service.db = db
	if db != nil {
		_ = db.AutoMigrate(&RequestRecord{})
	}
	return service
}

func (s *Service) RegisterRoutes(router gin.IRouter) {
	router.GET("/approval-requests", s.list)
	router.GET("/approval-requests/:id", s.get)
	router.POST("/approval-requests", s.create)
	router.POST("/approval-requests/:id/approve", s.approve)
	router.POST("/approval-requests/:id/reject", s.reject)
	router.POST("/approval-requests/:id/consume", s.consume)
}

func (s *Service) SetServiceURLs(contractURL, performanceURL, archiveURL, reportURL string) {
	s.contractServiceURL = strings.TrimRight(strings.TrimSpace(contractURL), "/")
	s.performanceServiceURL = strings.TrimRight(strings.TrimSpace(performanceURL), "/")
	s.archiveServiceURL = strings.TrimRight(strings.TrimSpace(archiveURL), "/")
	s.reportServiceURL = strings.TrimRight(strings.TrimSpace(reportURL), "/")
}

func (s *Service) list(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "approval.read") {
		return
	}
	if s.db != nil {
		var rows []RequestRecord
		query := s.db.Order("created_at desc")
		if department, limited := scopedDepartment(c); limited {
			query = query.Where("department = ?", department)
		}
		if err := query.Find(&rows).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to list approval requests")
			return
		}
		result := make([]Request, 0, len(rows))
		for _, row := range rows {
			request := Request{
				ID:          row.ID,
				ContractID:  row.ContractID,
				Department:  row.Department,
				RequestType: row.RequestType,
				ResourceID:  row.ResourceID,
				Status:      row.Status,
				RequestedBy: row.RequestedBy,
				CreatedAt:   row.CreatedAt,
			}
			request.Payload = parsePayload(row.PayloadRaw)
			if row.ApprovedBy != "" {
				request.ApprovedBy = row.ApprovedBy
			}
			if row.ApprovedAt != nil {
				request.ApprovedAt = *row.ApprovedAt
			}
			if row.ExpiresAt != nil {
				request.ExpiresAt = *row.ExpiresAt
			}
			if row.ConsumedAt != nil {
				request.ConsumedAt = *row.ConsumedAt
			}
			if row.ConsumedBy != "" {
				request.ConsumedBy = row.ConsumedBy
			}
			result = append(result, request)
		}
		httpx.Success(c, result)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Request, 0, len(s.requests))
	for _, req := range s.requests {
		if !canAccessDepartment(c, req.Department) {
			continue
		}
		result = append(result, req)
	}
	httpx.Success(c, result)
}

func (s *Service) get(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "approval.read") {
		return
	}
	if s.db != nil {
		var row RequestRecord
		if err := s.db.First(&row, "id = ?", c.Param("id")).Error; err != nil {
			httpx.Error(c, http.StatusNotFound, "approval request not found")
			return
		}
		if !canAccessDepartment(c, row.Department) {
			httpx.Error(c, http.StatusForbidden, "department data scope denied")
			return
		}
		request := Request{
			ID:          row.ID,
			ContractID:  row.ContractID,
			Department:  row.Department,
			RequestType: row.RequestType,
			ResourceID:  row.ResourceID,
			Status:      row.Status,
			RequestedBy: row.RequestedBy,
			Payload:     parsePayload(row.PayloadRaw),
			CreatedAt:   row.CreatedAt,
		}
		if row.ApprovedBy != "" {
			request.ApprovedBy = row.ApprovedBy
		}
		if row.ApprovedAt != nil {
			request.ApprovedAt = *row.ApprovedAt
		}
		if row.ExpiresAt != nil {
			request.ExpiresAt = *row.ExpiresAt
		}
		if row.ConsumedAt != nil {
			request.ConsumedAt = *row.ConsumedAt
		}
		if row.ConsumedBy != "" {
			request.ConsumedBy = row.ConsumedBy
		}
		httpx.Success(c, request)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.requests[c.Param("id")]
	if !ok {
		httpx.Error(c, http.StatusNotFound, "approval request not found")
		return
	}
	if !canAccessDepartment(c, record.Department) {
		httpx.Error(c, http.StatusForbidden, "department data scope denied")
		return
	}
	httpx.Success(c, record)
}

func (s *Service) create(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "approval.request") {
		return
	}
	var req struct {
		ContractID  string                 `json:"contract_id"`
		RequestType string                 `json:"request_type"`
		ResourceID  string                 `json:"resource_id"`
		RequestedBy string                 `json:"requested_by"`
		Payload     map[string]interface{} `json:"payload"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid approval request payload")
		return
	}
	if _, ok := allowedRequestTypes[req.RequestType]; !ok {
		httpx.Error(c, http.StatusBadRequest, "unsupported approval request type")
		return
	}
	if err := validateRequest(req.RequestType, req.ContractID, req.ResourceID, req.Payload); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	s.mu.Lock()
	s.seq++
	record := Request{
		ID:          fmt.Sprintf("apr-%04d", s.seq),
		ContractID:  req.ContractID,
		Department:  operatorDepartment(c),
		RequestType: req.RequestType,
		ResourceID:  req.ResourceID,
		Status:      "pending",
		RequestedBy: req.RequestedBy,
		Payload:     req.Payload,
		CreatedAt:   time.Now(),
	}
	if record.RequestType == "report_export" {
		record.ExpiresAt = resolveReportExportExpiry(record.CreatedAt, record.Payload)
	}
	payloadRaw, _ := json.Marshal(record.Payload)

	if s.db != nil {
		dbRecord := RequestRecord{
			ID:          record.ID,
			ContractID:  record.ContractID,
			Department:  record.Department,
			RequestType: record.RequestType,
			ResourceID:  record.ResourceID,
			Status:      record.Status,
			RequestedBy: record.RequestedBy,
			PayloadRaw:  string(payloadRaw),
			CreatedAt:   record.CreatedAt,
			ExpiresAt:   timePointerIfSet(record.ExpiresAt),
		}
		s.mu.Unlock()
		if err := s.db.Create(&dbRecord).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to create approval request")
			return
		}
	} else {
		s.requests[record.ID] = record
		s.mu.Unlock()
	}

	httpx.Created(c, record)
}

func (s *Service) approve(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "approval.process") {
		return
	}
	var req struct {
		ApprovedBy string `json:"approved_by"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid approval payload")
		return
	}

	if s.db != nil {
		var row RequestRecord
		if err := s.db.First(&row, "id = ?", c.Param("id")).Error; err != nil {
			httpx.Error(c, http.StatusNotFound, "approval request not found")
			return
		}
		if !canAccessDepartment(c, row.Department) {
			httpx.Error(c, http.StatusForbidden, "department data scope denied")
			return
		}
		now := time.Now()
		row.Status = "approved"
		row.ApprovedBy = req.ApprovedBy
		row.ApprovedAt = &now
		request := Request{
			ID:          row.ID,
			ContractID:  row.ContractID,
			Department:  row.Department,
			RequestType: row.RequestType,
			ResourceID:  row.ResourceID,
			Status:      row.Status,
			RequestedBy: row.RequestedBy,
			Payload:     parsePayload(row.PayloadRaw),
			ApprovedBy:  req.ApprovedBy,
			CreatedAt:   row.CreatedAt,
			ApprovedAt:  now,
		}
		if row.ExpiresAt != nil {
			request.ExpiresAt = *row.ExpiresAt
		}
		if row.ConsumedAt != nil {
			request.ConsumedAt = *row.ConsumedAt
		}
		if row.ConsumedBy != "" {
			request.ConsumedBy = row.ConsumedBy
		}
		if err := s.executeApproval(request); err != nil {
			httpx.Error(c, http.StatusBadGateway, "approval callback failed")
			return
		}
		if err := s.db.Save(&row).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to approve request")
			return
		}
		httpx.Success(c, request)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.requests[c.Param("id")]
	if !ok {
		httpx.Error(c, http.StatusNotFound, "approval request not found")
		return
	}
	if !canAccessDepartment(c, record.Department) {
		httpx.Error(c, http.StatusForbidden, "department data scope denied")
		return
	}

	record.Status = "approved"
	record.ApprovedBy = req.ApprovedBy
	record.ApprovedAt = time.Now()
	if err := s.executeApproval(record); err != nil {
		httpx.Error(c, http.StatusBadGateway, "approval callback failed")
		return
	}
	s.requests[record.ID] = record
	httpx.Success(c, record)
}

func (s *Service) reject(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "approval.process") {
		return
	}
	var req struct {
		ApprovedBy string `json:"approved_by"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid approval payload")
		return
	}

	if s.db != nil {
		var row RequestRecord
		if err := s.db.First(&row, "id = ?", c.Param("id")).Error; err != nil {
			httpx.Error(c, http.StatusNotFound, "approval request not found")
			return
		}
		if !canAccessDepartment(c, row.Department) {
			httpx.Error(c, http.StatusForbidden, "department data scope denied")
			return
		}
		now := time.Now()
		row.Status = "rejected"
		row.ApprovedBy = req.ApprovedBy
		row.ApprovedAt = &now
		if err := s.db.Save(&row).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to reject request")
			return
		}
		httpx.Success(c, Request{
			ID:          row.ID,
			ContractID:  row.ContractID,
			Department:  row.Department,
			RequestType: row.RequestType,
			ResourceID:  row.ResourceID,
			Status:      row.Status,
			RequestedBy: row.RequestedBy,
			Payload:     parsePayload(row.PayloadRaw),
			ApprovedBy:  req.ApprovedBy,
			CreatedAt:   row.CreatedAt,
			ApprovedAt:  now,
			ExpiresAt:   timeValueOrZero(row.ExpiresAt),
			ConsumedAt:  timeValueOrZero(row.ConsumedAt),
			ConsumedBy:  row.ConsumedBy,
		})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.requests[c.Param("id")]
	if !ok {
		httpx.Error(c, http.StatusNotFound, "approval request not found")
		return
	}
	if !canAccessDepartment(c, record.Department) {
		httpx.Error(c, http.StatusForbidden, "department data scope denied")
		return
	}

	record.Status = "rejected"
	record.ApprovedBy = req.ApprovedBy
	record.ApprovedAt = time.Now()
	s.requests[record.ID] = record
	httpx.Success(c, record)
}

func (s *Service) consume(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "approval.process") {
		return
	}
	operatorID := middleware.CurrentOperatorID(c, "system")
	if s.db != nil {
		var row RequestRecord
		if err := s.db.First(&row, "id = ?", c.Param("id")).Error; err != nil {
			httpx.Error(c, http.StatusNotFound, "approval request not found")
			return
		}
		if !canAccessDepartment(c, row.Department) {
			httpx.Error(c, http.StatusForbidden, "department data scope denied")
			return
		}
		if err := validateConsumableApproval(row); err != nil {
			httpx.Error(c, http.StatusConflict, err.Error())
			return
		}
		now := time.Now()
		row.ConsumedAt = &now
		row.ConsumedBy = operatorID
		if err := s.db.Save(&row).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to consume approval request")
			return
		}
		httpx.Success(c, Request{
			ID:          row.ID,
			ContractID:  row.ContractID,
			Department:  row.Department,
			RequestType: row.RequestType,
			ResourceID:  row.ResourceID,
			Status:      row.Status,
			RequestedBy: row.RequestedBy,
			Payload:     parsePayload(row.PayloadRaw),
			ApprovedBy:  row.ApprovedBy,
			CreatedAt:   row.CreatedAt,
			ApprovedAt:  timeValueOrZero(row.ApprovedAt),
			ExpiresAt:   timeValueOrZero(row.ExpiresAt),
			ConsumedAt:  now,
			ConsumedBy:  row.ConsumedBy,
		})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.requests[c.Param("id")]
	if !ok {
		httpx.Error(c, http.StatusNotFound, "approval request not found")
		return
	}
	if !canAccessDepartment(c, record.Department) {
		httpx.Error(c, http.StatusForbidden, "department data scope denied")
		return
	}
	if err := validateConsumableRequest(record); err != nil {
		httpx.Error(c, http.StatusConflict, err.Error())
		return
	}
	record.ConsumedAt = time.Now()
	record.ConsumedBy = operatorID
	s.requests[record.ID] = record
	httpx.Success(c, record)
}

func validateRequest(requestType, contractID, resourceID string, payload map[string]interface{}) error {
	switch requestType {
	case "status_change":
		if strings.TrimSpace(contractID) == "" {
			return fmt.Errorf("contract_id is required")
		}
		status, _ := payload["status"].(string)
		if strings.TrimSpace(status) == "" {
			return fmt.Errorf("payload.status is required")
		}
	case "plan_adjustment":
		if strings.TrimSpace(contractID) == "" {
			return fmt.Errorf("contract_id is required")
		}
		nodes, ok := payload["nodes"].([]interface{})
		if !ok || len(nodes) == 0 {
			return fmt.Errorf("payload.nodes must contain at least one node")
		}
	case "archive_borrow", "archive_destroy":
		if strings.TrimSpace(resourceID) == "" {
			return fmt.Errorf("resource_id is required")
		}
	case "report_export":
		view, _ := payload["view"].(string)
		if strings.TrimSpace(view) == "" {
			return fmt.Errorf("payload.view is required")
		}
	}
	return nil
}

func parsePayload(raw string) map[string]interface{} {
	if strings.TrimSpace(raw) == "" {
		return map[string]interface{}{}
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return map[string]interface{}{}
	}
	return payload
}

func (s *Service) executeApproval(request Request) error {
	switch request.RequestType {
	case "status_change":
		if s.contractServiceURL == "" {
			return fmt.Errorf("contract service url is not configured")
		}
		return s.postJSON(
			fmt.Sprintf("%s/api/v1/contracts/%s/status", s.contractServiceURL, request.ContractID),
			map[string]interface{}{
				"status":      request.Payload["status"],
				"operator_id": request.ApprovedBy,
				"description": "approved by approval-workflow-service",
			},
		)
	case "plan_adjustment":
		if s.performanceServiceURL == "" {
			return fmt.Errorf("performance service url is not configured")
		}
		return s.postJSON(
			fmt.Sprintf("%s/api/v1/contracts/%s/plan-versions", s.performanceServiceURL, request.ContractID),
			map[string]interface{}{
				"nodes": request.Payload["nodes"],
			},
		)
	case "archive_borrow":
		if s.archiveServiceURL == "" {
			return fmt.Errorf("archive service url is not configured")
		}
		return s.postJSON(
			fmt.Sprintf("%s/api/v1/archive/cases/%s/borrow", s.archiveServiceURL, request.ResourceID),
			map[string]interface{}{
				"approval_request_id": request.ID,
				"approved_by":         request.ApprovedBy,
			},
		)
	case "archive_destroy":
		if s.archiveServiceURL == "" {
			return fmt.Errorf("archive service url is not configured")
		}
		return s.postJSON(
			fmt.Sprintf("%s/api/v1/archive/cases/%s/destroy", s.archiveServiceURL, request.ResourceID),
			map[string]interface{}{
				"approval_request_id": request.ID,
				"approved_by":         request.ApprovedBy,
			},
		)
	case "report_export":
		return nil
	default:
		return fmt.Errorf("unsupported approval request type")
	}
}

func (s *Service) postJSON(endpoint string, payload map[string]interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := http.Post(endpoint, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("callback failed: %s", strings.TrimSpace(string(body)))
	}
	return nil
}

func scopedDepartment(c *gin.Context) (string, bool) {
	identity, ok := middleware.IdentityFromContextOrHeaders(c)
	if !ok {
		return "", false
	}
	if identity.DataScope == "department" && strings.TrimSpace(identity.Department) != "" {
		return identity.Department, true
	}
	return "", false
}

func canAccessDepartment(c *gin.Context, ownerDepartment string) bool {
	department, limited := scopedDepartment(c)
	if !limited {
		return true
	}
	return ownerDepartment == "" || ownerDepartment == department
}

func operatorDepartment(c *gin.Context) string {
	identity, ok := middleware.IdentityFromContextOrHeaders(c)
	if !ok {
		return ""
	}
	return strings.TrimSpace(identity.Department)
}

func resolveReportExportExpiry(createdAt time.Time, payload map[string]interface{}) time.Time {
	const defaultMinutes = 30
	minutes := defaultMinutes
	if raw, ok := payload["expires_in_minutes"]; ok {
		switch value := raw.(type) {
		case float64:
			if int(value) > 0 {
				minutes = int(value)
			}
		case int:
			if value > 0 {
				minutes = value
			}
		}
	}
	return createdAt.Add(time.Duration(minutes) * time.Minute)
}

func timePointerIfSet(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copy := value
	return &copy
}

func timeValueOrZero(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}

func validateConsumableApproval(row RequestRecord) error {
	if row.RequestType != "report_export" {
		return fmt.Errorf("approval request type is not report_export")
	}
	if row.Status != "approved" {
		return fmt.Errorf("approval request has not been approved")
	}
	if row.ConsumedAt != nil {
		return fmt.Errorf("approval request has already been consumed")
	}
	if row.ExpiresAt != nil && row.ExpiresAt.Before(time.Now()) {
		return fmt.Errorf("approval request has expired")
	}
	return nil
}

func validateConsumableRequest(record Request) error {
	if record.RequestType != "report_export" {
		return fmt.Errorf("approval request type is not report_export")
	}
	if record.Status != "approved" {
		return fmt.Errorf("approval request has not been approved")
	}
	if !record.ConsumedAt.IsZero() {
		return fmt.Errorf("approval request has already been consumed")
	}
	if !record.ExpiresAt.IsZero() && record.ExpiresAt.Before(time.Now()) {
		return fmt.Errorf("approval request has expired")
	}
	return nil
}
