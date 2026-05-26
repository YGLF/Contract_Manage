package risk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"contract-manage/pkg/microplatform/events"
	"contract-manage/pkg/microplatform/httpx"
	"contract-manage/pkg/microplatform/middleware"
	"contract-manage/pkg/microplatform/outbox"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Event struct {
	ID          string    `json:"id"`
	ContractID  string    `json:"contract_id"`
	Department  string    `json:"department,omitempty"`
	RuleCode    string    `json:"rule_code"`
	Severity    string    `json:"severity"`
	Status      string    `json:"status"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type EventRecord struct {
	ID          string `gorm:"primaryKey;size:64"`
	ContractID  string `gorm:"size:64;index;not null"`
	Department  string `gorm:"size:64;index"`
	RuleCode    string `gorm:"size:64;index;not null"`
	Severity    string `gorm:"size:32;index;not null"`
	Status      string `gorm:"size:32;index;not null"`
	Description string `gorm:"type:text"`
	CreatedAt   time.Time
}

type Service struct {
	mu                     sync.RWMutex
	events                 map[string]Event
	seq                    int
	db                     *gorm.DB
	notificationServiceURL string
	defaultRecipient       string
}

func New() *Service {
	return &Service{
		events: make(map[string]Event),
	}
}

func NewWithDB(db *gorm.DB) *Service {
	service := New()
	service.db = db
	if db != nil {
		_ = db.AutoMigrate(&EventRecord{})
		_ = outbox.AutoMigrate(db)
	}
	return service
}

func (s *Service) RegisterRoutes(router gin.IRouter) {
	router.GET("/risk/events", s.list)
	router.POST("/risk/events", s.create)
	router.POST("/risk/events/:id/close", s.close)
}

func (s *Service) SetNotificationConfig(serviceURL, recipient string) {
	s.notificationServiceURL = strings.TrimRight(strings.TrimSpace(serviceURL), "/")
	s.defaultRecipient = strings.TrimSpace(recipient)
}

func (s *Service) list(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "risk.read") {
		return
	}
	if s.db != nil {
		var rows []EventRecord
		query := s.db.Order("created_at desc")
		if department, limited := scopedDepartment(c); limited {
			query = query.Where("department = ?", department)
		}
		if contractID := strings.TrimSpace(c.Query("contract_id")); contractID != "" {
			query = query.Where("contract_id = ?", contractID)
		}
		if status := strings.TrimSpace(c.Query("status")); status != "" {
			query = query.Where("status = ?", status)
		}
		if ruleCode := strings.TrimSpace(c.Query("rule_code")); ruleCode != "" {
			query = query.Where("rule_code = ?", ruleCode)
		}
		if err := query.Find(&rows).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to list risk events")
			return
		}

		result := make([]Event, 0, len(rows))
		for _, row := range rows {
			result = append(result, Event{
				ID:          row.ID,
				ContractID:  row.ContractID,
				Department:  row.Department,
				RuleCode:    row.RuleCode,
				Severity:    row.Severity,
				Status:      row.Status,
				Description: row.Description,
				CreatedAt:   row.CreatedAt,
			})
		}
		httpx.Success(c, result)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Event, 0, len(s.events))
	for _, item := range s.events {
		if !canAccessDepartment(c, item.Department) {
			continue
		}
		if contractID := strings.TrimSpace(c.Query("contract_id")); contractID != "" && item.ContractID != contractID {
			continue
		}
		if status := strings.TrimSpace(c.Query("status")); status != "" && item.Status != status {
			continue
		}
		if ruleCode := strings.TrimSpace(c.Query("rule_code")); ruleCode != "" && item.RuleCode != ruleCode {
			continue
		}
		result = append(result, item)
	}
	httpx.Success(c, result)
}

func (s *Service) create(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "risk.write") {
		return
	}
	var req struct {
		ContractID  string `json:"contract_id"`
		RuleCode    string `json:"rule_code"`
		Severity    string `json:"severity"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid risk payload")
		return
	}
	if strings.TrimSpace(req.ContractID) == "" || strings.TrimSpace(req.RuleCode) == "" || strings.TrimSpace(req.Severity) == "" {
		httpx.Error(c, http.StatusBadRequest, "contract_id, rule_code and severity are required")
		return
	}
	if existing, ok, err := s.findOpenEvent(req.ContractID, req.RuleCode); err != nil {
		httpx.Error(c, http.StatusInternalServerError, "failed to query duplicate risk event")
		return
	} else if ok {
		httpx.Success(c, existing)
		return
	}

	s.mu.Lock()
	s.seq++
	record := Event{
		ID:          fmt.Sprintf("risk-%04d", s.seq),
		ContractID:  req.ContractID,
		Department:  operatorDepartment(c),
		RuleCode:    req.RuleCode,
		Severity:    req.Severity,
		Status:      "open",
		Description: req.Description,
		CreatedAt:   time.Now(),
	}

	if s.db != nil {
		operatorID := middleware.CurrentOperatorID(c, "system")
		dbRecord := EventRecord{
			ID:          record.ID,
			ContractID:  record.ContractID,
			Department:  record.Department,
			RuleCode:    record.RuleCode,
			Severity:    record.Severity,
			Status:      record.Status,
			Description: record.Description,
			CreatedAt:   record.CreatedAt,
		}
		s.mu.Unlock()
		if err := s.db.Create(&dbRecord).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to create risk event")
			return
		}
		_ = outbox.Append(s.db, events.Event{
			EventID:       fmt.Sprintf("evt-risk-%s", record.ID),
			EventType:     "risk.created",
			OccurredAt:    record.CreatedAt,
			TraceID:       c.GetHeader("X-Trace-Id"),
			Source:        "risk-service",
			AggregateType: "risk_event",
			AggregateID:   record.ID,
			OperatorID:    operatorID,
			Payload: map[string]interface{}{
				"contract_id": record.ContractID,
				"department":  record.Department,
				"rule_code":   record.RuleCode,
				"severity":    record.Severity,
				"status":      record.Status,
			},
		})
	} else {
		s.events[record.ID] = record
		s.mu.Unlock()
	}

	_ = s.notifyRiskEvent(record, "created")

	httpx.Created(c, record)
}

func (s *Service) close(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "risk.dispose") {
		return
	}
	if s.db != nil {
		operatorID := middleware.CurrentOperatorID(c, "system")
		var row EventRecord
		if err := s.db.First(&row, "id = ?", c.Param("id")).Error; err != nil {
			httpx.Error(c, http.StatusNotFound, "risk event not found")
			return
		}
		if !canAccessDepartment(c, row.Department) {
			httpx.Error(c, http.StatusForbidden, "department data scope denied")
			return
		}
		row.Status = "closed"
		if err := s.db.Save(&row).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to close risk event")
			return
		}
		_ = outbox.Append(s.db, events.Event{
			EventID:       fmt.Sprintf("evt-risk-close-%s-%d", row.ID, time.Now().UnixNano()),
			EventType:     "risk.closed",
			OccurredAt:    time.Now(),
			TraceID:       c.GetHeader("X-Trace-Id"),
			Source:        "risk-service",
			AggregateType: "risk_event",
			AggregateID:   row.ID,
			OperatorID:    operatorID,
			Payload: map[string]interface{}{
				"contract_id": row.ContractID,
				"department":  row.Department,
				"rule_code":   row.RuleCode,
				"severity":    row.Severity,
				"status":      row.Status,
			},
		})
		_ = s.notifyRiskEvent(Event{
			ID:          row.ID,
			ContractID:  row.ContractID,
			Department:  row.Department,
			RuleCode:    row.RuleCode,
			Severity:    row.Severity,
			Status:      row.Status,
			Description: row.Description,
			CreatedAt:   row.CreatedAt,
		}, "closed")
		httpx.Success(c, Event{
			ID:          row.ID,
			ContractID:  row.ContractID,
			RuleCode:    row.RuleCode,
			Severity:    row.Severity,
			Status:      row.Status,
			Description: row.Description,
			CreatedAt:   row.CreatedAt,
		})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.events[c.Param("id")]
	if !ok {
		httpx.Error(c, http.StatusNotFound, "risk event not found")
		return
	}
	if !canAccessDepartment(c, item.Department) {
		httpx.Error(c, http.StatusForbidden, "department data scope denied")
		return
	}

	item.Status = "closed"
	s.events[item.ID] = item
	_ = s.notifyRiskEvent(item, "closed")
	httpx.Success(c, item)
}

func (s *Service) notifyRiskEvent(event Event, phase string) error {
	if s.notificationServiceURL == "" || s.defaultRecipient == "" {
		return nil
	}

	subject := fmt.Sprintf("合同风险预警[%s]", strings.ToUpper(event.Severity))
	body := fmt.Sprintf("合同 %s 触发规则 %s，当前状态：%s。说明：%s", event.ContractID, event.RuleCode, event.Status, event.Description)
	template := "risk_alert_created"
	if phase == "closed" {
		subject = fmt.Sprintf("合同风险处置完成[%s]", event.RuleCode)
		body = fmt.Sprintf("合同 %s 的风险事件 %s 已关闭。说明：%s", event.ContractID, event.ID, event.Description)
		template = "risk_alert_closed"
	}

	payload, err := json.Marshal(map[string]interface{}{
		"channel":     "in_app",
		"recipient":   s.defaultRecipient,
		"subject":     subject,
		"body":        body,
		"template":    template,
		"source_type": "risk_event",
		"source_id":   event.ID,
	})
	if err != nil {
		return err
	}

	resp, err := http.Post(s.notificationServiceURL+"/api/v1/notifications/messages", "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create risk notification: %s", strings.TrimSpace(string(body)))
	}
	return nil
}

func (s *Service) findOpenEvent(contractID, ruleCode string) (Event, bool, error) {
	if s.db != nil {
		var row EventRecord
		err := s.db.Where("contract_id = ? AND rule_code = ? AND status = ?", contractID, ruleCode, "open").Order("created_at desc").First(&row).Error
		if err == gorm.ErrRecordNotFound {
			return Event{}, false, nil
		}
		if err != nil {
			return Event{}, false, err
		}
		return Event{
			ID:          row.ID,
			ContractID:  row.ContractID,
			Department:  row.Department,
			RuleCode:    row.RuleCode,
			Severity:    row.Severity,
			Status:      row.Status,
			Description: row.Description,
			CreatedAt:   row.CreatedAt,
		}, true, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.events {
		if item.ContractID == contractID && item.RuleCode == ruleCode && item.Status == "open" {
			return item, true, nil
		}
	}
	return Event{}, false, nil
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
