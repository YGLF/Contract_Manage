package notification

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"contract-manage/pkg/microplatform/auditclient"
	"contract-manage/pkg/microplatform/httpx"
	"contract-manage/pkg/microplatform/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Message struct {
	ID         string    `json:"id"`
	Channel    string    `json:"channel"`
	Recipient  string    `json:"recipient"`
	Subject    string    `json:"subject"`
	Body       string    `json:"body"`
	Template   string    `json:"template"`
	Status     string    `json:"status"`
	SourceType string    `json:"source_type"`
	SourceID   string    `json:"source_id"`
	CreatedAt  time.Time `json:"created_at"`
}

type MessageRecord struct {
	ID         string `gorm:"primaryKey;size:64"`
	Channel    string `gorm:"size:32;index;not null"`
	Recipient  string `gorm:"size:128;index;not null"`
	Subject    string `gorm:"size:255"`
	Body       string `gorm:"type:text"`
	Template   string `gorm:"size:64;index"`
	Status     string `gorm:"size:32;index;not null"`
	SourceType string `gorm:"size:64;index"`
	SourceID   string `gorm:"size:128;index"`
	CreatedAt  time.Time
}

type Service struct {
	mu                 sync.RWMutex
	messages           map[string]Message
	seq                int
	db                 *gorm.DB
	audit              *auditclient.Client
	autoSendRiskAlerts bool
}

func New() *Service {
	return &Service{
		messages: make(map[string]Message),
	}
}

func NewWithDB(db *gorm.DB) *Service {
	service := New()
	service.db = db
	if db != nil {
		_ = db.AutoMigrate(&MessageRecord{})
	}
	return service
}

func (s *Service) RegisterRoutes(router gin.IRouter) {
	router.GET("/notifications/messages", s.list)
	router.POST("/notifications/messages", s.create)
	router.POST("/notifications/messages/:id/send", s.send)
}

func (s *Service) SetAutoSendRiskAlerts(enabled bool) {
	s.autoSendRiskAlerts = enabled
}

func (s *Service) SetAuditClient(client *auditclient.Client) {
	s.audit = client
}

func (s *Service) list(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "notification.read") {
		return
	}
	if s.db != nil {
		var rows []MessageRecord
		if err := s.db.Order("created_at desc").Find(&rows).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to list notifications")
			return
		}
		result := make([]Message, 0, len(rows))
		for _, row := range rows {
			result = append(result, Message{
				ID:         row.ID,
				Channel:    row.Channel,
				Recipient:  row.Recipient,
				Subject:    row.Subject,
				Body:       row.Body,
				Template:   row.Template,
				Status:     row.Status,
				SourceType: row.SourceType,
				SourceID:   row.SourceID,
				CreatedAt:  row.CreatedAt,
			})
		}
		httpx.Success(c, result)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Message, 0, len(s.messages))
	for _, item := range s.messages {
		result = append(result, item)
	}
	httpx.Success(c, result)
}

func (s *Service) create(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "notification.send") {
		return
	}
	var req struct {
		Channel    string `json:"channel"`
		Recipient  string `json:"recipient"`
		Subject    string `json:"subject"`
		Body       string `json:"body"`
		Template   string `json:"template"`
		SourceType string `json:"source_type"`
		SourceID   string `json:"source_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		s.recordAudit(c, "notification.create_failed", map[string]interface{}{
			"reason": "invalid notification payload",
		})
		httpx.Error(c, http.StatusBadRequest, "invalid notification payload")
		return
	}
	if strings.TrimSpace(req.Channel) == "" {
		req.Channel = "in_app"
	}
	if strings.TrimSpace(req.Recipient) == "" {
		s.recordAudit(c, "notification.create_failed", map[string]interface{}{
			"reason": "recipient is required",
		})
		httpx.Error(c, http.StatusBadRequest, "recipient is required")
		return
	}

	s.mu.Lock()
	s.seq++
	item := Message{
		ID:         fmt.Sprintf("ntf-%04d", s.seq),
		Channel:    req.Channel,
		Recipient:  req.Recipient,
		Subject:    req.Subject,
		Body:       req.Body,
		Template:   req.Template,
		Status:     "pending",
		SourceType: req.SourceType,
		SourceID:   req.SourceID,
		CreatedAt:  time.Now(),
	}

	if s.db != nil {
		row := MessageRecord{
			ID:         item.ID,
			Channel:    item.Channel,
			Recipient:  item.Recipient,
			Subject:    item.Subject,
			Body:       item.Body,
			Template:   item.Template,
			Status:     item.Status,
			SourceType: item.SourceType,
			SourceID:   item.SourceID,
			CreatedAt:  item.CreatedAt,
		}
		s.mu.Unlock()
		if err := s.db.Create(&row).Error; err != nil {
			s.recordAudit(c, "notification.create_failed", map[string]interface{}{
				"reason":      "failed to create notification",
				"recipient":   item.Recipient,
				"channel":     item.Channel,
				"source_type": item.SourceType,
				"source_id":   item.SourceID,
			})
			httpx.Error(c, http.StatusInternalServerError, "failed to create notification")
			return
		}
		if s.shouldAutoSend(item) {
			row.Status = "sent"
			if err := s.db.Save(&row).Error; err != nil {
				s.recordAudit(c, "notification.auto_send_failed", map[string]interface{}{
					"id":          item.ID,
					"recipient":   item.Recipient,
					"channel":     item.Channel,
					"source_id":   item.SourceID,
					"source_type": item.SourceType,
				})
				httpx.Error(c, http.StatusInternalServerError, "failed to auto send notification")
				return
			}
			item.Status = row.Status
		}
	} else {
		if s.shouldAutoSend(item) {
			item.Status = "sent"
		}
		s.messages[item.ID] = item
		s.mu.Unlock()
	}

	action := "notification.created"
	if item.Status == "sent" {
		action = "notification.auto_sent"
	}
	s.recordAudit(c, action, map[string]interface{}{
		"id":          item.ID,
		"recipient":   item.Recipient,
		"channel":     item.Channel,
		"template":    item.Template,
		"status":      item.Status,
		"source_type": item.SourceType,
		"source_id":   item.SourceID,
	})
	httpx.Created(c, item)
}

func (s *Service) shouldAutoSend(item Message) bool {
	return s.autoSendRiskAlerts && item.SourceType == "risk_event"
}

func (s *Service) send(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "notification.send") {
		return
	}
	if s.db != nil {
		var row MessageRecord
		if err := s.db.First(&row, "id = ?", c.Param("id")).Error; err != nil {
			s.recordAudit(c, "notification.send_failed", map[string]interface{}{
				"id":     c.Param("id"),
				"reason": "notification not found",
			})
			httpx.Error(c, http.StatusNotFound, "notification not found")
			return
		}
		row.Status = "sent"
		if err := s.db.Save(&row).Error; err != nil {
			s.recordAudit(c, "notification.send_failed", map[string]interface{}{
				"id":        row.ID,
				"recipient": row.Recipient,
				"channel":   row.Channel,
				"reason":    "failed to send notification",
			})
			httpx.Error(c, http.StatusInternalServerError, "failed to send notification")
			return
		}
		s.recordAudit(c, "notification.sent", map[string]interface{}{
			"id":          row.ID,
			"recipient":   row.Recipient,
			"channel":     row.Channel,
			"template":    row.Template,
			"source_type": row.SourceType,
			"source_id":   row.SourceID,
		})
		httpx.Success(c, gin.H{"id": row.ID, "status": row.Status})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.messages[c.Param("id")]
	if !ok {
		s.recordAudit(c, "notification.send_failed", map[string]interface{}{
			"id":     c.Param("id"),
			"reason": "notification not found",
		})
		httpx.Error(c, http.StatusNotFound, "notification not found")
		return
	}
	item.Status = "sent"
	s.messages[item.ID] = item
	s.recordAudit(c, "notification.sent", map[string]interface{}{
		"id":          item.ID,
		"recipient":   item.Recipient,
		"channel":     item.Channel,
		"template":    item.Template,
		"source_type": item.SourceType,
		"source_id":   item.SourceID,
	})
	httpx.Success(c, item)
}

func (s *Service) recordAudit(c *gin.Context, action string, payload map[string]interface{}) {
	if s.audit == nil {
		return
	}
	traceID, _ := c.Get(middleware.TraceIDKey)
	trace := ""
	if value, ok := traceID.(string); ok {
		trace = value
	}
	_ = s.audit.Record(
		fmt.Sprintf("audit-notification-%d", time.Now().UnixNano()),
		action,
		middleware.CurrentOperatorID(c, "system"),
		trace,
		payload,
	)
}
