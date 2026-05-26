package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"contract-manage/pkg/microplatform/auditclient"
	"contract-manage/pkg/microplatform/events"
	"contract-manage/pkg/microplatform/httpx"
	"contract-manage/pkg/microplatform/middleware"
	"contract-manage/pkg/microplatform/outbox"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Contract struct {
	ID                   string         `json:"id"`
	ContractNo           string         `json:"contract_no"`
	Title                string         `json:"title"`
	CounterpartyID       string         `json:"counterparty_id"`
	OwnerDepartment      string         `json:"owner_department,omitempty"`
	Status               string         `json:"status"`
	DocumentIDs          []string       `json:"document_ids"`
	LatestAmendmentID    string         `json:"latest_amendment_id,omitempty"`
	LatestAmendmentTitle string         `json:"latest_amendment_title,omitempty"`
	CreatedAt            time.Time      `json:"created_at"`
	Lifecycle            []events.Event `json:"lifecycle"`
}

type ContractRecord struct {
	ID                   string `gorm:"primaryKey;size:64"`
	ContractNo           string `gorm:"size:64;uniqueIndex;not null"`
	Title                string `gorm:"size:255;not null"`
	CounterpartyID       string `gorm:"size:64;index"`
	OwnerDepartment      string `gorm:"size:64;index"`
	Status               string `gorm:"size:32;index"`
	DocumentIDs          string `gorm:"type:text"`
	LatestAmendmentID    string `gorm:"size:64;index"`
	LatestAmendmentTitle string `gorm:"size:255"`
	CreatedAt            time.Time
}

type LifecycleRecord struct {
	ID          string    `gorm:"primaryKey;size:64"`
	ContractID  string    `gorm:"size:64;index;not null"`
	EventType   string    `gorm:"size:64;index;not null"`
	FromStatus  string    `gorm:"size:32"`
	ToStatus    string    `gorm:"size:32"`
	OperatorID  string    `gorm:"size:64;index"`
	TraceID     string    `gorm:"size:128;index"`
	OccurredAt  time.Time `gorm:"index"`
	Description string    `gorm:"type:text"`
}

type Service struct {
	mu                 sync.RWMutex
	contracts          map[string]Contract
	seq                int
	db                 *gorm.DB
	audit              *auditclient.Client
	documentServiceURL string
	partyServiceURL    string
}

func New() *Service {
	return &Service{
		contracts: make(map[string]Contract),
	}
}

func NewWithDB(db *gorm.DB) *Service {
	service := New()
	service.db = db
	if db != nil {
		_ = db.AutoMigrate(&ContractRecord{})
		_ = db.AutoMigrate(&LifecycleRecord{})
		_ = outbox.AutoMigrate(db)
	}
	return service
}

func (s *Service) SetAuditClient(client *auditclient.Client) {
	s.audit = client
}

func (s *Service) SetDocumentServiceURL(rawURL string) {
	s.documentServiceURL = strings.TrimRight(strings.TrimSpace(rawURL), "/")
}

func (s *Service) SetPartyServiceURL(rawURL string) {
	s.partyServiceURL = strings.TrimRight(strings.TrimSpace(rawURL), "/")
}

func (s *Service) RegisterRoutes(router gin.IRouter) {
	router.GET("/contracts", s.list)
	router.POST("/contracts", s.create)
	router.POST("/contracts/intake", s.intake)
	router.GET("/contracts/:id", s.get)
	router.PUT("/contracts/:id", s.update)
	router.DELETE("/contracts/:id", s.remove)
	router.POST("/contracts/:id/attachments", s.attachDocument)
	router.POST("/contracts/:id/amendments/apply", s.applyAmendment)
	router.POST("/contracts/:id/status", s.updateStatus)
	router.GET("/contracts/:id/lifecycle", s.lifecycle)
}

func (s *Service) list(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "contract.read") {
		return
	}
	if s.db != nil {
		var rows []ContractRecord
		query := s.db.Order("created_at desc")
		if department, limited := scopedDepartment(c); limited {
			query = query.Where("owner_department = ?", department)
		}
		if err := query.Find(&rows).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to list contracts")
			return
		}

		result := make([]Contract, 0, len(rows))
		for _, row := range rows {
			result = append(result, Contract{
				ID:                   row.ID,
				ContractNo:           row.ContractNo,
				Title:                row.Title,
				CounterpartyID:       row.CounterpartyID,
				OwnerDepartment:      row.OwnerDepartment,
				Status:               row.Status,
				DocumentIDs:          splitDocumentIDs(row.DocumentIDs),
				LatestAmendmentID:    row.LatestAmendmentID,
				LatestAmendmentTitle: row.LatestAmendmentTitle,
				CreatedAt:            row.CreatedAt,
			})
		}
		httpx.Success(c, result)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Contract, 0, len(s.contracts))
	for _, contract := range s.contracts {
		if !canAccessDepartment(c, contract.OwnerDepartment) {
			continue
		}
		result = append(result, contract)
	}
	httpx.Success(c, result)
}

func (s *Service) get(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "contract.read") {
		return
	}
	if s.db != nil {
		var row ContractRecord
		if err := s.db.First(&row, "id = ?", c.Param("id")).Error; err != nil {
			httpx.Error(c, http.StatusNotFound, "contract not found")
			return
		}
		if !canAccessDepartment(c, row.OwnerDepartment) {
			httpx.Error(c, http.StatusForbidden, "department data scope denied")
			return
		}
		httpx.Success(c, Contract{
			ID:                   row.ID,
			ContractNo:           row.ContractNo,
			Title:                row.Title,
			CounterpartyID:       row.CounterpartyID,
			OwnerDepartment:      row.OwnerDepartment,
			Status:               row.Status,
			DocumentIDs:          splitDocumentIDs(row.DocumentIDs),
			LatestAmendmentID:    row.LatestAmendmentID,
			LatestAmendmentTitle: row.LatestAmendmentTitle,
			CreatedAt:            row.CreatedAt,
		})
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	contract, ok := s.contracts[c.Param("id")]
	if !ok {
		httpx.Error(c, http.StatusNotFound, "contract not found")
		return
	}
	if !canAccessDepartment(c, contract.OwnerDepartment) {
		httpx.Error(c, http.StatusForbidden, "department data scope denied")
		return
	}
	httpx.Success(c, contract)
}

func (s *Service) create(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "contract.create") {
		return
	}
	var req struct {
		Title          string   `json:"title"`
		CounterpartyID string   `json:"counterparty_id"`
		DocumentIDs    []string `json:"document_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid contract payload")
		return
	}

	traceID, _ := c.Get(middleware.TraceIDKey)
	trace := ""
	if traceID != nil {
		trace = traceID.(string)
	}
	operatorID := middleware.CurrentOperatorID(c, "system")
	if err := s.validateCounterparty(req.CounterpartyID); err != nil {
		httpx.Error(c, http.StatusBadGateway, err.Error())
		return
	}
	contract, _, err := s.createContractRecord(req.Title, req.CounterpartyID, req.DocumentIDs, operatorDepartment(c), "contract.created", trace)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "failed to create contract")
		return
	}
	_ = s.syncCounterpartyHistory(contract)

	if s.audit != nil {
		_ = s.audit.Record("audit-"+contract.ID, "contract.created", operatorID, trace, map[string]interface{}{
			"contract_id": contract.ID,
			"contract_no": contract.ContractNo,
			"title":       contract.Title,
			"status":      contract.Status,
		})
	}

	httpx.Created(c, contract)
}

func (s *Service) intake(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "contract.create") {
		return
	}
	var req struct {
		Title          string   `json:"title"`
		CounterpartyID string   `json:"counterparty_id"`
		DocumentIDs    []string `json:"document_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid intake payload")
		return
	}

	if strings.TrimSpace(req.Title) == "" {
		httpx.Error(c, http.StatusBadRequest, "title is required")
		return
	}
	if len(req.DocumentIDs) == 0 {
		httpx.Error(c, http.StatusBadRequest, "at least one committed document is required")
		return
	}
	for _, documentID := range req.DocumentIDs {
		if strings.TrimSpace(documentID) == "" {
			httpx.Error(c, http.StatusBadRequest, "document id cannot be empty")
			return
		}
	}

	if s.documentServiceURL == "" {
		httpx.Error(c, http.StatusFailedDependency, "document service url is not configured")
		return
	}
	if err := s.validateCounterparty(req.CounterpartyID); err != nil {
		httpx.Error(c, http.StatusBadGateway, err.Error())
		return
	}

	for _, documentID := range req.DocumentIDs {
		doc, err := s.fetchTempDocument(documentID)
		if err != nil {
			httpx.Error(c, http.StatusBadGateway, fmt.Sprintf("failed to validate temp document %s", documentID))
			return
		}
		if doc.Status != "committed" {
			httpx.Error(c, http.StatusConflict, fmt.Sprintf("temp document %s is not committed", documentID))
			return
		}
	}

	traceID, _ := c.Get(middleware.TraceIDKey)
	trace := ""
	if traceID != nil {
		trace = traceID.(string)
	}
	operatorID := middleware.CurrentOperatorID(c, "system")

	contract, _, err := s.createContractRecord(req.Title, req.CounterpartyID, req.DocumentIDs, operatorDepartment(c), "contract.intake.created", trace)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "failed to create contract")
		return
	}

	if err := s.bindDocuments(contract.ID, req.DocumentIDs); err != nil {
		_ = s.deleteContract(contract.ID)
		httpx.Error(c, http.StatusBadGateway, "failed to bind committed documents")
		return
	}
	_ = s.syncCounterpartyHistory(contract)

	if s.audit != nil {
		_ = s.audit.Record("audit-intake-"+contract.ID, "contract.intake.completed", operatorID, trace, map[string]interface{}{
			"contract_id":  contract.ID,
			"contract_no":  contract.ContractNo,
			"title":        contract.Title,
			"status":       contract.Status,
			"document_ids": contract.DocumentIDs,
		})
	}

	httpx.Created(c, contract)
}

func (s *Service) attachDocument(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "contract.update") {
		return
	}
	var req struct {
		DocumentID string `json:"document_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid attachment payload")
		return
	}

	if req.DocumentID == "" {
		httpx.Error(c, http.StatusBadRequest, "document_id is required")
		return
	}

	if s.db != nil {
		var row ContractRecord
		if err := s.db.First(&row, "id = ?", c.Param("id")).Error; err != nil {
			httpx.Error(c, http.StatusNotFound, "contract not found")
			return
		}
		if !canAccessDepartment(c, row.OwnerDepartment) {
			httpx.Error(c, http.StatusForbidden, "department data scope denied")
			return
		}

		if row.DocumentIDs == "" {
			row.DocumentIDs = req.DocumentID
		} else {
			row.DocumentIDs = row.DocumentIDs + "," + req.DocumentID
		}

		if err := s.db.Save(&row).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to attach document")
			return
		}

		httpx.Success(c, gin.H{
			"contract_id":  row.ID,
			"document_ids": row.DocumentIDs,
		})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	contract, ok := s.contracts[c.Param("id")]
	if !ok {
		httpx.Error(c, http.StatusNotFound, "contract not found")
		return
	}
	if !canAccessDepartment(c, contract.OwnerDepartment) {
		httpx.Error(c, http.StatusForbidden, "department data scope denied")
		return
	}

	contract.DocumentIDs = append(contract.DocumentIDs, req.DocumentID)
	s.contracts[contract.ID] = contract

	httpx.Success(c, gin.H{
		"contract_id":  contract.ID,
		"document_ids": contract.DocumentIDs,
	})
}

func (s *Service) update(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "contract.update") {
		return
	}
	var req struct {
		Title          string   `json:"title"`
		CounterpartyID string   `json:"counterparty_id"`
		DocumentIDs    []string `json:"document_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid contract payload")
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		httpx.Error(c, http.StatusBadRequest, "title is required")
		return
	}
	if err := s.validateCounterparty(req.CounterpartyID); err != nil {
		httpx.Error(c, http.StatusBadGateway, err.Error())
		return
	}

	traceID, _ := c.Get(middleware.TraceIDKey)
	trace := ""
	if traceID != nil {
		trace = traceID.(string)
	}
	operatorID := middleware.CurrentOperatorID(c, "system")

	if s.db != nil {
		var row ContractRecord
		if err := s.db.First(&row, "id = ?", c.Param("id")).Error; err != nil {
			httpx.Error(c, http.StatusNotFound, "contract not found")
			return
		}
		if !canAccessDepartment(c, row.OwnerDepartment) {
			httpx.Error(c, http.StatusForbidden, "department data scope denied")
			return
		}

		previousDocumentIDs := splitDocumentIDs(row.DocumentIDs)
		nextDocumentIDs := req.DocumentIDs
		if nextDocumentIDs == nil {
			nextDocumentIDs = previousDocumentIDs
		}
		if err := s.syncDocumentBindings(row.ID, previousDocumentIDs, nextDocumentIDs); err != nil {
			httpx.Error(c, http.StatusBadGateway, err.Error())
			return
		}

		row.Title = req.Title
		row.CounterpartyID = req.CounterpartyID
		row.DocumentIDs = strings.Join(nextDocumentIDs, ",")
		if err := s.db.Save(&row).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to update contract")
			return
		}

		description := "contract updated"
		_ = s.appendLifecycleRecord(row.ID, "contract.updated", row.Status, row.Status, operatorID, trace, description)
		_ = outbox.Append(s.db, events.Event{
			EventID:       fmt.Sprintf("evt-update-%s-%d", row.ID, time.Now().UnixNano()),
			EventType:     "contract.updated",
			OccurredAt:    time.Now(),
			TraceID:       trace,
			Source:        "contract-service",
			AggregateType: "contract",
			AggregateID:   row.ID,
			OperatorID:    operatorID,
			Payload: map[string]interface{}{
				"title":           row.Title,
				"counterparty_id": row.CounterpartyID,
				"document_ids":    nextDocumentIDs,
			},
		})
		if s.audit != nil {
			_ = s.audit.Record("audit-update-"+row.ID, "contract.updated", operatorID, trace, map[string]interface{}{
				"contract_id":     row.ID,
				"title":           row.Title,
				"counterparty_id": row.CounterpartyID,
				"document_ids":    nextDocumentIDs,
			})
		}

		httpx.Success(c, Contract{
			ID:                   row.ID,
			ContractNo:           row.ContractNo,
			Title:                row.Title,
			CounterpartyID:       row.CounterpartyID,
			OwnerDepartment:      row.OwnerDepartment,
			Status:               row.Status,
			DocumentIDs:          nextDocumentIDs,
			LatestAmendmentID:    row.LatestAmendmentID,
			LatestAmendmentTitle: row.LatestAmendmentTitle,
			CreatedAt:            row.CreatedAt,
		})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	contract, ok := s.contracts[c.Param("id")]
	if !ok {
		httpx.Error(c, http.StatusNotFound, "contract not found")
		return
	}
	if !canAccessDepartment(c, contract.OwnerDepartment) {
		httpx.Error(c, http.StatusForbidden, "department data scope denied")
		return
	}
	nextDocumentIDs := contract.DocumentIDs
	if req.DocumentIDs != nil {
		nextDocumentIDs = req.DocumentIDs
		if err := s.syncDocumentBindings(contract.ID, contract.DocumentIDs, nextDocumentIDs); err != nil {
			httpx.Error(c, http.StatusBadGateway, err.Error())
			return
		}
		contract.DocumentIDs = nextDocumentIDs
	}
	contract.Title = req.Title
	contract.CounterpartyID = req.CounterpartyID
	s.contracts[contract.ID] = contract
	httpx.Success(c, contract)
}

func (s *Service) remove(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "contract.delete") {
		return
	}
	traceID, _ := c.Get(middleware.TraceIDKey)
	trace := ""
	if traceID != nil {
		trace = traceID.(string)
	}
	operatorID := middleware.CurrentOperatorID(c, "system")

	if s.db != nil {
		var row ContractRecord
		if err := s.db.First(&row, "id = ?", c.Param("id")).Error; err != nil {
			httpx.Error(c, http.StatusNotFound, "contract not found")
			return
		}
		if !canAccessDepartment(c, row.OwnerDepartment) {
			httpx.Error(c, http.StatusForbidden, "department data scope denied")
			return
		}
		documentIDs := splitDocumentIDs(row.DocumentIDs)
		if len(documentIDs) > 0 && s.documentServiceURL != "" {
			if err := s.releaseDocuments(row.ID, documentIDs); err != nil {
				httpx.Error(c, http.StatusBadGateway, "failed to release bound documents")
				return
			}
		}
		if s.audit != nil {
			_ = s.audit.Record("audit-delete-"+row.ID, "contract.deleted", operatorID, trace, map[string]interface{}{
				"contract_id":  row.ID,
				"contract_no":  row.ContractNo,
				"title":        row.Title,
				"counterparty": row.CounterpartyID,
				"document_ids": documentIDs,
			})
		}
		if err := s.deleteContract(row.ID); err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to delete contract")
			return
		}
		httpx.Success(c, gin.H{
			"contract_id": row.ID,
			"deleted":     true,
		})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	contract, ok := s.contracts[c.Param("id")]
	if !ok {
		httpx.Error(c, http.StatusNotFound, "contract not found")
		return
	}
	if !canAccessDepartment(c, contract.OwnerDepartment) {
		httpx.Error(c, http.StatusForbidden, "department data scope denied")
		return
	}
	if len(contract.DocumentIDs) > 0 && s.documentServiceURL != "" {
		if err := s.releaseDocuments(contract.ID, contract.DocumentIDs); err != nil {
			httpx.Error(c, http.StatusBadGateway, "failed to release bound documents")
			return
		}
	}
	delete(s.contracts, c.Param("id"))
	httpx.Success(c, gin.H{
		"contract_id": c.Param("id"),
		"deleted":     true,
	})
}

func (s *Service) updateStatus(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "contract.status.change") {
		return
	}
	var req struct {
		Status      string `json:"status"`
		OperatorID  string `json:"operator_id"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid status payload")
		return
	}
	if req.Status == "" {
		httpx.Error(c, http.StatusBadRequest, "status is required")
		return
	}

	traceID, _ := c.Get(middleware.TraceIDKey)
	trace := ""
	if traceID != nil {
		trace = traceID.(string)
	}
	if strings.TrimSpace(req.OperatorID) == "" {
		req.OperatorID = middleware.CurrentOperatorID(c, "system")
	}

	if s.db != nil {
		var row ContractRecord
		if err := s.db.First(&row, "id = ?", c.Param("id")).Error; err != nil {
			httpx.Error(c, http.StatusNotFound, "contract not found")
			return
		}
		if !canAccessDepartment(c, row.OwnerDepartment) {
			httpx.Error(c, http.StatusForbidden, "department data scope denied")
			return
		}
		fromStatus := row.Status
		row.Status = req.Status
		if err := s.db.Save(&row).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to update contract status")
			return
		}
		_ = s.appendLifecycleRecord(row.ID, "contract.status_changed", fromStatus, row.Status, req.OperatorID, trace, req.Description)
		_ = outbox.Append(s.db, events.Event{
			EventID:       fmt.Sprintf("evt-status-%s-%d", row.ID, time.Now().UnixNano()),
			EventType:     "contract.status_changed",
			OccurredAt:    time.Now(),
			TraceID:       trace,
			Source:        "contract-service",
			AggregateType: "contract",
			AggregateID:   row.ID,
			OperatorID:    req.OperatorID,
			Payload: map[string]interface{}{
				"from_status": fromStatus,
				"to_status":   row.Status,
				"description": req.Description,
			},
		})
		if s.audit != nil {
			_ = s.audit.Record("audit-status-"+row.ID, "contract.status_changed", req.OperatorID, trace, map[string]interface{}{
				"contract_id": row.ID,
				"from_status": fromStatus,
				"to_status":   row.Status,
			})
		}
		httpx.Success(c, gin.H{
			"contract_id": row.ID,
			"status":      row.Status,
		})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	contract, ok := s.contracts[c.Param("id")]
	if !ok {
		httpx.Error(c, http.StatusNotFound, "contract not found")
		return
	}
	if !canAccessDepartment(c, contract.OwnerDepartment) {
		httpx.Error(c, http.StatusForbidden, "department data scope denied")
		return
	}
	fromStatus := contract.Status
	contract.Status = req.Status
	s.contracts[contract.ID] = contract
	httpx.Success(c, gin.H{
		"contract_id": contract.ID,
		"from_status": fromStatus,
		"to_status":   contract.Status,
	})
}

func (s *Service) applyAmendment(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "contract.amendment.apply") {
		return
	}
	var req struct {
		AmendmentID    string `json:"amendment_id"`
		AmendmentTitle string `json:"amendment_title"`
		OperatorID     string `json:"operator_id"`
		Description    string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid amendment apply payload")
		return
	}
	if strings.TrimSpace(req.AmendmentID) == "" {
		httpx.Error(c, http.StatusBadRequest, "amendment_id is required")
		return
	}

	traceID, _ := c.Get(middleware.TraceIDKey)
	trace := ""
	if traceID != nil {
		trace = traceID.(string)
	}
	if strings.TrimSpace(req.OperatorID) == "" {
		req.OperatorID = middleware.CurrentOperatorID(c, "system")
	}

	if s.db != nil {
		var row ContractRecord
		if err := s.db.First(&row, "id = ?", c.Param("id")).Error; err != nil {
			httpx.Error(c, http.StatusNotFound, "contract not found")
			return
		}
		if !canAccessDepartment(c, row.OwnerDepartment) {
			httpx.Error(c, http.StatusForbidden, "department data scope denied")
			return
		}
		row.LatestAmendmentID = req.AmendmentID
		row.LatestAmendmentTitle = req.AmendmentTitle
		if err := s.db.Save(&row).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to apply amendment")
			return
		}
		_ = s.appendLifecycleRecord(row.ID, "contract.amendment_applied", row.Status, row.Status, req.OperatorID, trace, req.Description)
		if s.audit != nil {
			_ = s.audit.Record("audit-amendment-"+row.ID, "contract.amendment_applied", req.OperatorID, trace, map[string]interface{}{
				"contract_id":            row.ID,
				"latest_amendment_id":    row.LatestAmendmentID,
				"latest_amendment_title": row.LatestAmendmentTitle,
			})
		}
		httpx.Success(c, gin.H{
			"contract_id":            row.ID,
			"latest_amendment_id":    row.LatestAmendmentID,
			"latest_amendment_title": row.LatestAmendmentTitle,
		})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	contract, ok := s.contracts[c.Param("id")]
	if !ok {
		httpx.Error(c, http.StatusNotFound, "contract not found")
		return
	}
	if !canAccessDepartment(c, contract.OwnerDepartment) {
		httpx.Error(c, http.StatusForbidden, "department data scope denied")
		return
	}
	contract.LatestAmendmentID = req.AmendmentID
	contract.LatestAmendmentTitle = req.AmendmentTitle
	s.contracts[contract.ID] = contract
	httpx.Success(c, gin.H{
		"contract_id":            contract.ID,
		"latest_amendment_id":    contract.LatestAmendmentID,
		"latest_amendment_title": contract.LatestAmendmentTitle,
	})
}

func (s *Service) lifecycle(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "contract.read") {
		return
	}
	if s.db != nil {
		var contractRow ContractRecord
		if err := s.db.First(&contractRow, "id = ?", c.Param("id")).Error; err != nil {
			httpx.Error(c, http.StatusNotFound, "contract not found")
			return
		}
		if !canAccessDepartment(c, contractRow.OwnerDepartment) {
			httpx.Error(c, http.StatusForbidden, "department data scope denied")
			return
		}
		var rows []LifecycleRecord
		if err := s.db.Where("contract_id = ?", c.Param("id")).Order("occurred_at asc").Find(&rows).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to list lifecycle")
			return
		}
		httpx.Success(c, rows)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	contract, ok := s.contracts[c.Param("id")]
	if !ok {
		httpx.Error(c, http.StatusNotFound, "contract not found")
		return
	}
	if !canAccessDepartment(c, contract.OwnerDepartment) {
		httpx.Error(c, http.StatusForbidden, "department data scope denied")
		return
	}
	httpx.Success(c, contract.Lifecycle)
}

func (s *Service) appendLifecycleRecord(contractID, eventType, fromStatus, toStatus, operatorID, traceID, description string) error {
	if s.db == nil {
		return nil
	}
	return s.db.Create(&LifecycleRecord{
		ID:          fmt.Sprintf("life-%s-%d", contractID, time.Now().UnixNano()),
		ContractID:  contractID,
		EventType:   eventType,
		FromStatus:  fromStatus,
		ToStatus:    toStatus,
		OperatorID:  operatorID,
		TraceID:     traceID,
		OccurredAt:  time.Now(),
		Description: description,
	}).Error
}

func splitDocumentIDs(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		cleaned := strings.TrimSpace(strings.Trim(part, "[]"))
		if cleaned != "" {
			result = append(result, cleaned)
		}
	}
	return result
}

type tempDocumentResponse struct {
	Success bool         `json:"success"`
	Data    TempDocument `json:"data"`
	Error   string       `json:"error"`
}

type TempDocument struct {
	ID              string    `json:"id"`
	FileName        string    `json:"file_name"`
	TempPath        string    `json:"temp_path"`
	Hash            string    `json:"hash"`
	Size            int64     `json:"size"`
	Status          string    `json:"status"`
	BoundContractID string    `json:"bound_contract_id"`
	CreatedAt       time.Time `json:"created_at"`
}

func (s *Service) createContractRecord(title, counterpartyID string, documentIDs []string, ownerDepartment, eventType, traceID string) (Contract, string, error) {
	s.mu.Lock()
	s.seq++
	id := fmt.Sprintf("ctr-%04d", s.seq)
	s.mu.Unlock()

	if strings.TrimSpace(traceID) == "" {
		traceID = fmt.Sprintf("trace-%d", time.Now().UnixNano())
	}
	evt := events.Event{
		EventID:       fmt.Sprintf("evt-%s", id),
		EventType:     eventType,
		OccurredAt:    time.Now(),
		TraceID:       traceID,
		Source:        "contract-service",
		AggregateType: "contract",
		AggregateID:   id,
		OperatorID:    "system",
		Payload: map[string]interface{}{
			"title": title,
		},
	}

	contract := Contract{
		ID:              id,
		ContractNo:      fmt.Sprintf("CM-%s", time.Now().Format("20060102150405")),
		Title:           title,
		CounterpartyID:  counterpartyID,
		OwnerDepartment: ownerDepartment,
		Status:          "registered",
		DocumentIDs:     documentIDs,
		CreatedAt:       time.Now(),
		Lifecycle:       []events.Event{evt},
	}

	if s.db != nil {
		record := ContractRecord{
			ID:              contract.ID,
			ContractNo:      contract.ContractNo,
			Title:           contract.Title,
			CounterpartyID:  contract.CounterpartyID,
			OwnerDepartment: contract.OwnerDepartment,
			Status:          contract.Status,
			DocumentIDs:     strings.Join(contract.DocumentIDs, ","),
			CreatedAt:       contract.CreatedAt,
		}
		if err := s.db.Create(&record).Error; err != nil {
			return Contract{}, "", err
		}
		_ = s.appendLifecycleRecord(contract.ID, eventType, "", contract.Status, "system", traceID, "contract created")
		_ = outbox.Append(s.db, evt)
	} else {
		s.mu.Lock()
		s.contracts[id] = contract
		s.mu.Unlock()
	}

	return contract, traceID, nil
}

func (s *Service) fetchTempDocument(documentID string) (TempDocument, error) {
	endpoint, err := url.JoinPath(s.documentServiceURL, "api/v1/documents/temp", path.Clean(documentID))
	if err != nil {
		return TempDocument{}, err
	}
	resp, err := http.Get(endpoint)
	if err != nil {
		return TempDocument{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return TempDocument{}, fmt.Errorf("document service returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result tempDocumentResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return TempDocument{}, err
	}
	if !result.Success {
		return TempDocument{}, fmt.Errorf("document service validation failed: %s", result.Error)
	}
	return result.Data, nil
}

func (s *Service) bindDocuments(contractID string, documentIDs []string) error {
	payload, err := json.Marshal(gin.H{
		"contract_id":  contractID,
		"document_ids": documentIDs,
	})
	if err != nil {
		return err
	}
	resp, err := http.Post(s.documentServiceURL+"/api/v1/documents/bind", "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("document bind failed: %s", strings.TrimSpace(string(body)))
	}
	return nil
}

func (s *Service) syncDocumentBindings(contractID string, previousDocumentIDs, nextDocumentIDs []string) error {
	if s.documentServiceURL == "" {
		return nil
	}
	toRelease := diffDocumentIDs(previousDocumentIDs, nextDocumentIDs)
	toBind := diffDocumentIDs(nextDocumentIDs, previousDocumentIDs)

	if len(toRelease) > 0 {
		if err := s.releaseDocuments(contractID, toRelease); err != nil {
			return err
		}
	}
	if len(toBind) > 0 {
		for _, documentID := range toBind {
			doc, err := s.fetchTempDocument(documentID)
			if err != nil {
				return fmt.Errorf("failed to validate temp document %s", documentID)
			}
			if doc.Status != "committed" {
				return fmt.Errorf("temp document %s is not committed", documentID)
			}
		}
		if err := s.bindDocuments(contractID, toBind); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) releaseDocuments(contractID string, documentIDs []string) error {
	payload, err := json.Marshal(gin.H{
		"contract_id":  contractID,
		"document_ids": documentIDs,
	})
	if err != nil {
		return err
	}
	resp, err := http.Post(s.documentServiceURL+"/api/v1/documents/release", "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("document release failed: %s", strings.TrimSpace(string(body)))
	}
	return nil
}

func diffDocumentIDs(left, right []string) []string {
	if len(left) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(right))
	for _, item := range right {
		cleaned := strings.TrimSpace(item)
		if cleaned != "" {
			seen[cleaned] = struct{}{}
		}
	}
	result := make([]string, 0)
	for _, item := range left {
		cleaned := strings.TrimSpace(item)
		if cleaned == "" {
			continue
		}
		if _, ok := seen[cleaned]; !ok {
			result = append(result, cleaned)
		}
	}
	return result
}

func (s *Service) deleteContract(contractID string) error {
	if s.db != nil {
		return s.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("contract_id = ?", contractID).Delete(&LifecycleRecord{}).Error; err != nil {
				return err
			}
			if err := tx.Where("aggregate_id = ? AND aggregate_type = ?", contractID, "contract").Delete(&outbox.Message{}).Error; err != nil {
				return err
			}
			return tx.Where("id = ?", contractID).Delete(&ContractRecord{}).Error
		})
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.contracts, contractID)
	return nil
}

type partyEnvelope struct {
	Success bool   `json:"success"`
	Data    Party  `json:"data"`
	Error   string `json:"error"`
}

type Party struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Status           string    `json:"status"`
	CooperationCount int       `json:"cooperation_count"`
	LastContractDate time.Time `json:"last_contract_date"`
}

type partyCreditCheckEnvelope struct {
	Success bool `json:"success"`
	Data    struct {
		Allowed bool     `json:"allowed"`
		Blocked bool     `json:"blocked"`
		Reasons []string `json:"reasons"`
	} `json:"data"`
	Error string `json:"error"`
}

func (s *Service) validateCounterparty(counterpartyID string) error {
	if strings.TrimSpace(counterpartyID) == "" {
		return fmt.Errorf("counterparty_id is required")
	}
	if s.partyServiceURL == "" {
		return nil
	}
	resp, err := http.Get(s.partyServiceURL + "/api/v1/parties/" + path.Clean(counterpartyID))
	if err != nil {
		return fmt.Errorf("failed to validate counterparty")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("counterparty is not available")
	}
	var result partyEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode counterparty response")
	}
	if !result.Success {
		return fmt.Errorf("counterparty validation failed")
	}
	if strings.TrimSpace(result.Data.Status) != "active" {
		return fmt.Errorf("counterparty is not active")
	}
	creditResp, err := http.Get(s.partyServiceURL + "/api/v1/parties/" + path.Clean(counterpartyID) + "/credit-check")
	if err != nil {
		return fmt.Errorf("failed to validate counterparty credit")
	}
	defer creditResp.Body.Close()
	if creditResp.StatusCode == http.StatusOK {
		var creditResult partyCreditCheckEnvelope
		if err := json.NewDecoder(creditResp.Body).Decode(&creditResult); err == nil && creditResult.Success && !creditResult.Data.Allowed {
			return fmt.Errorf("counterparty credit is restricted: %s", strings.Join(creditResult.Data.Reasons, "; "))
		}
	}
	return nil
}

func (s *Service) syncCounterpartyHistory(contract Contract) error {
	if s.partyServiceURL == "" || strings.TrimSpace(contract.CounterpartyID) == "" {
		return nil
	}
	payload, err := json.Marshal(gin.H{
		"contract_id":    contract.ID,
		"contract_title": contract.Title,
		"contract_no":    contract.ContractNo,
		"signed_at":      contract.CreatedAt,
		"status":         contract.Status,
	})
	if err != nil {
		return err
	}
	resp, err := http.Post(
		s.partyServiceURL+"/api/v1/parties/"+path.Clean(contract.CounterpartyID)+"/cooperation-history",
		"application/json",
		bytes.NewReader(payload),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to sync counterparty cooperation history")
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
