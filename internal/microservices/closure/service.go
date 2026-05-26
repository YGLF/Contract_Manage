package closure

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

type Request struct {
	ID            string    `json:"id"`
	ContractID    string    `json:"contract_id"`
	Department    string    `json:"department,omitempty"`
	RequestType   string    `json:"request_type"`
	Reason        string    `json:"reason"`
	Status        string    `json:"status"`
	RequestedBy   string    `json:"requested_by"`
	RiskChecked   bool      `json:"risk_checked"`
	PerformanceOK bool      `json:"performance_ok"`
	EvidenceReady bool      `json:"evidence_ready"`
	CreatedAt     time.Time `json:"created_at"`
}

type RequestRecord struct {
	ID            string `gorm:"primaryKey;size:64"`
	ContractID    string `gorm:"size:64;index;not null"`
	Department    string `gorm:"size:64;index"`
	RequestType   string `gorm:"size:32;index;not null"`
	Reason        string `gorm:"type:text"`
	Status        string `gorm:"size:32;index;not null"`
	RequestedBy   string `gorm:"size:64;index;not null"`
	RiskChecked   bool   `gorm:"not null"`
	PerformanceOK bool   `gorm:"not null"`
	EvidenceReady bool   `gorm:"not null"`
	CreatedAt     time.Time
}

type Service struct {
	mu             sync.RWMutex
	requests       map[string]Request
	seq            int
	db             *gorm.DB
	archiveURL     string
	audit          *auditclient.Client
	performanceURL string
	riskURL        string
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
		_ = outbox.AutoMigrate(db)
	}
	return service
}

func (s *Service) SetArchiveURL(url string) {
	s.archiveURL = strings.TrimRight(url, "/")
}

func (s *Service) SetDependencyURLs(performanceURL, riskURL string) {
	s.performanceURL = strings.TrimRight(strings.TrimSpace(performanceURL), "/")
	s.riskURL = strings.TrimRight(strings.TrimSpace(riskURL), "/")
}

func (s *Service) SetAuditClient(client *auditclient.Client) {
	s.audit = client
}

func (s *Service) RegisterRoutes(router gin.IRouter) {
	router.GET("/closure/requests", s.list)
	router.POST("/closure/requests", s.create)
	router.POST("/closure/requests/:id/complete", s.complete)
}

func (s *Service) list(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "closure.read") {
		return
	}
	if s.db != nil {
		var rows []RequestRecord
		query := s.db.Order("created_at desc")
		if department, limited := scopedDepartment(c); limited {
			query = query.Where("department = ?", department)
		}
		if err := query.Find(&rows).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to list closure requests")
			return
		}

		result := make([]Request, 0, len(rows))
		for _, row := range rows {
			result = append(result, Request{
				ID:            row.ID,
				ContractID:    row.ContractID,
				Department:    row.Department,
				RequestType:   row.RequestType,
				Reason:        row.Reason,
				Status:        row.Status,
				RequestedBy:   row.RequestedBy,
				RiskChecked:   row.RiskChecked,
				PerformanceOK: row.PerformanceOK,
				EvidenceReady: row.EvidenceReady,
				CreatedAt:     row.CreatedAt,
			})
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

func (s *Service) create(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "closure.request") {
		return
	}
	var req struct {
		ContractID    string `json:"contract_id"`
		RequestType   string `json:"request_type"`
		Reason        string `json:"reason"`
		RequestedBy   string `json:"requested_by"`
		RiskChecked   bool   `json:"risk_checked"`
		PerformanceOK bool   `json:"performance_ok"`
		EvidenceReady bool   `json:"evidence_ready"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid closure payload")
		return
	}
	if strings.TrimSpace(req.RequestedBy) == "" {
		req.RequestedBy = middleware.CurrentOperatorID(c, "system")
	}
	if strings.TrimSpace(req.ContractID) == "" || strings.TrimSpace(req.RequestType) == "" || strings.TrimSpace(req.RequestedBy) == "" {
		httpx.Error(c, http.StatusBadRequest, "contract_id, request_type and requested_by are required")
		return
	}
	if err := s.validateClosureConditions(req.ContractID, req.EvidenceReady); err != nil {
		httpx.Error(c, http.StatusConflict, err.Error())
		return
	}

	s.mu.Lock()
	s.seq++
	item := Request{
		ID:            fmt.Sprintf("cls-%04d", s.seq),
		ContractID:    req.ContractID,
		Department:    operatorDepartment(c),
		RequestType:   req.RequestType,
		Reason:        req.Reason,
		Status:        "pending",
		RequestedBy:   req.RequestedBy,
		RiskChecked:   true,
		PerformanceOK: true,
		EvidenceReady: req.EvidenceReady,
		CreatedAt:     time.Now(),
	}

	if s.db != nil {
		row := RequestRecord{
			ID:            item.ID,
			ContractID:    item.ContractID,
			Department:    item.Department,
			RequestType:   item.RequestType,
			Reason:        item.Reason,
			Status:        item.Status,
			RequestedBy:   item.RequestedBy,
			RiskChecked:   item.RiskChecked,
			PerformanceOK: item.PerformanceOK,
			EvidenceReady: item.EvidenceReady,
			CreatedAt:     item.CreatedAt,
		}
		s.mu.Unlock()
		if err := s.db.Create(&row).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to create closure request")
			return
		}
		_ = outbox.Append(s.db, events.Event{
			EventID:       fmt.Sprintf("evt-closure-%s", item.ID),
			EventType:     "closure.requested",
			OccurredAt:    item.CreatedAt,
			TraceID:       c.GetHeader("X-Trace-Id"),
			Source:        "closure-service",
			AggregateType: "closure_request",
			AggregateID:   item.ID,
			OperatorID:    item.RequestedBy,
			Payload: map[string]interface{}{
				"contract_id":    item.ContractID,
				"request_type":   item.RequestType,
				"risk_checked":   item.RiskChecked,
				"performance_ok": item.PerformanceOK,
				"evidence_ready": item.EvidenceReady,
			},
		})
	} else {
		s.requests[item.ID] = item
		s.mu.Unlock()
	}

	httpx.Created(c, item)
}

func (s *Service) complete(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "closure.process") {
		return
	}
	traceIDHeader := c.GetHeader("X-Trace-Id")
	if s.db != nil {
		var row RequestRecord
		if err := s.db.First(&row, "id = ?", c.Param("id")).Error; err != nil {
			httpx.Error(c, http.StatusNotFound, "closure request not found")
			return
		}
		if !canAccessDepartment(c, row.Department) {
			httpx.Error(c, http.StatusForbidden, "department data scope denied")
			return
		}
		if err := s.validateClosureConditions(row.ContractID, row.EvidenceReady); err != nil {
			httpx.Error(c, http.StatusConflict, err.Error())
			return
		}
		operatorID := middleware.CurrentOperatorID(c, row.RequestedBy)
		row.Status = "completed"
		if err := s.db.Save(&row).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to complete closure request")
			return
		}
		_ = outbox.Append(s.db, events.Event{
			EventID:       fmt.Sprintf("evt-closure-complete-%s-%d", row.ID, time.Now().UnixNano()),
			EventType:     "closure.completed",
			OccurredAt:    time.Now(),
			TraceID:       traceIDHeader,
			Source:        "closure-service",
			AggregateType: "closure_request",
			AggregateID:   row.ID,
			OperatorID:    operatorID,
			Payload: map[string]interface{}{
				"contract_id":  row.ContractID,
				"request_type": row.RequestType,
				"status":       row.Status,
			},
		})
		_ = s.createArchiveCase(row.ContractID, traceIDHeader)
		if s.audit != nil {
			_ = s.audit.Record("audit-"+row.ID, "closure.completed", operatorID, traceIDHeader, map[string]interface{}{
				"contract_id":  row.ContractID,
				"request_type": row.RequestType,
			})
		}
		httpx.Success(c, Request{
			ID:            row.ID,
			ContractID:    row.ContractID,
			Department:    row.Department,
			RequestType:   row.RequestType,
			Reason:        row.Reason,
			Status:        row.Status,
			RequestedBy:   row.RequestedBy,
			RiskChecked:   row.RiskChecked,
			PerformanceOK: row.PerformanceOK,
			EvidenceReady: row.EvidenceReady,
			CreatedAt:     row.CreatedAt,
		})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.requests[c.Param("id")]
	if !ok {
		httpx.Error(c, http.StatusNotFound, "closure request not found")
		return
	}
	if !canAccessDepartment(c, item.Department) {
		httpx.Error(c, http.StatusForbidden, "department data scope denied")
		return
	}
	if err := s.validateClosureConditions(item.ContractID, item.EvidenceReady); err != nil {
		httpx.Error(c, http.StatusConflict, err.Error())
		return
	}
	operatorID := middleware.CurrentOperatorID(c, item.RequestedBy)
	item.Status = "completed"
	s.requests[item.ID] = item
	_ = s.createArchiveCase(item.ContractID, traceIDHeader)
	if s.audit != nil {
		_ = s.audit.Record("audit-"+item.ID, "closure.completed", operatorID, traceIDHeader, map[string]interface{}{
			"contract_id":  item.ContractID,
			"request_type": item.RequestType,
		})
	}
	httpx.Success(c, item)
}

func (s *Service) createArchiveCase(contractID, traceID string) error {
	if s.archiveURL == "" {
		return nil
	}

	body := map[string]interface{}{
		"contract_id":  contractID,
		"archive_type": "electronic",
	}
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, s.archiveURL+"/api/v1/archive/cases", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if traceID != "" {
		req.Header.Set("X-Trace-Id", traceID)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (s *Service) validateClosureConditions(contractID string, evidenceReady bool) error {
	if !evidenceReady {
		return fmt.Errorf("evidence materials are not ready")
	}
	if s.performanceURL != "" {
		performanceOK, err := s.fetchPerformanceOK(contractID)
		if err != nil {
			return fmt.Errorf("failed to validate performance summary")
		}
		if !performanceOK {
			return fmt.Errorf("performance execution is not complete")
		}
	}
	if s.riskURL != "" {
		openCount, err := s.fetchOpenRiskCount(contractID)
		if err != nil {
			return fmt.Errorf("failed to validate open risks")
		}
		if openCount > 0 {
			return fmt.Errorf("open risk events must be closed before closure")
		}
	}
	return nil
}

func (s *Service) fetchPerformanceOK(contractID string) (bool, error) {
	resp, err := http.Get(s.performanceURL + "/api/v1/contracts/" + contractID + "/performance-summary")
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("unexpected performance response: %s", strings.TrimSpace(string(body)))
	}
	var result struct {
		Success bool `json:"success"`
		Data    struct {
			PerformanceOK bool `json:"performance_ok"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, err
	}
	return result.Success && result.Data.PerformanceOK, nil
}

func (s *Service) fetchOpenRiskCount(contractID string) (int, error) {
	resp, err := http.Get(s.riskURL + "/api/v1/risk/events?contract_id=" + contractID + "&status=open")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("unexpected risk response: %s", strings.TrimSpace(string(body)))
	}
	var result struct {
		Success bool `json:"success"`
		Data    []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}
	return len(result.Data), nil
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
