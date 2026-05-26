package audit

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"contract-manage/pkg/microplatform/httpx"
	"contract-manage/pkg/microplatform/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Record struct {
	ID         string                 `json:"id"`
	Service    string                 `json:"service"`
	Action     string                 `json:"action"`
	TraceID    string                 `json:"trace_id"`
	OperatorID string                 `json:"operator_id"`
	Payload    map[string]interface{} `json:"payload"`
	CreatedAt  time.Time              `json:"created_at"`
}

type RecordModel struct {
	ID         string `gorm:"primaryKey;size:64"`
	Service    string `gorm:"size:64;index;not null"`
	Action     string `gorm:"size:128;index;not null"`
	TraceID    string `gorm:"size:128;index"`
	OperatorID string `gorm:"size:64;index"`
	PayloadRaw string `gorm:"type:text"`
	CreatedAt  time.Time
}

type Service struct {
	mu      sync.RWMutex
	records []Record
	db      *gorm.DB
}

func New() *Service {
	return &Service{
		records: make([]Record, 0, 32),
	}
}

func NewWithDB(db *gorm.DB) *Service {
	service := New()
	service.db = db
	if db != nil {
		_ = db.AutoMigrate(&RecordModel{})
	}
	return service
}

func (s *Service) RegisterRoutes(router gin.IRouter) {
	router.GET("/audit/logs", s.list)
	router.POST("/audit/logs", s.create)
}

func (s *Service) list(c *gin.Context) {
	if s.db != nil {
		var rows []RecordModel
		query := s.db.Order("created_at desc")
		if service := strings.TrimSpace(c.Query("service")); service != "" {
			query = query.Where("service = ?", service)
		}
		if action := strings.TrimSpace(c.Query("action")); action != "" {
			query = query.Where("action = ?", action)
		}
		if operatorID := strings.TrimSpace(c.Query("operator_id")); operatorID != "" {
			query = query.Where("operator_id = ?", operatorID)
		}
		if traceID := strings.TrimSpace(c.Query("trace_id")); traceID != "" {
			query = query.Where("trace_id = ?", traceID)
		}
		if err := query.Find(&rows).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to list audit logs")
			return
		}

		result := make([]Record, 0, len(rows))
		for _, row := range rows {
			payload := map[string]interface{}{}
			if strings.TrimSpace(row.PayloadRaw) != "" {
				_ = json.Unmarshal([]byte(row.PayloadRaw), &payload)
			}
			result = append(result, Record{
				ID:         row.ID,
				Service:    row.Service,
				Action:     row.Action,
				TraceID:    row.TraceID,
				OperatorID: row.OperatorID,
				Payload:    payload,
				CreatedAt:  row.CreatedAt,
			})
		}
		httpx.Success(c, result)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Record, 0, len(s.records))
	for _, record := range s.records {
		if service := strings.TrimSpace(c.Query("service")); service != "" && record.Service != service {
			continue
		}
		if action := strings.TrimSpace(c.Query("action")); action != "" && record.Action != action {
			continue
		}
		if operatorID := strings.TrimSpace(c.Query("operator_id")); operatorID != "" && record.OperatorID != operatorID {
			continue
		}
		if traceID := strings.TrimSpace(c.Query("trace_id")); traceID != "" && record.TraceID != traceID {
			continue
		}
		result = append(result, record)
	}
	httpx.Success(c, result)
}

func (s *Service) create(c *gin.Context) {
	var req struct {
		ID         string                 `json:"id"`
		Service    string                 `json:"service"`
		Action     string                 `json:"action"`
		OperatorID string                 `json:"operator_id"`
		Payload    map[string]interface{} `json:"payload"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid audit payload")
		return
	}

	traceID, _ := c.Get(middleware.TraceIDKey)
	trace := ""
	if traceID != nil {
		trace = traceID.(string)
	}
	payloadRaw, _ := json.Marshal(req.Payload)
	record := Record{
		ID:         req.ID,
		Service:    req.Service,
		Action:     req.Action,
		TraceID:    trace,
		OperatorID: req.OperatorID,
		Payload:    req.Payload,
		CreatedAt:  time.Now(),
	}

	if s.db != nil {
		model := RecordModel{
			ID:         record.ID,
			Service:    record.Service,
			Action:     record.Action,
			TraceID:    record.TraceID,
			OperatorID: record.OperatorID,
			PayloadRaw: string(payloadRaw),
			CreatedAt:  record.CreatedAt,
		}
		if err := s.db.Create(&model).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to create audit log")
			return
		}
	} else {
		s.mu.Lock()
		s.records = append(s.records, record)
		s.mu.Unlock()
	}

	httpx.Created(c, record)
}
