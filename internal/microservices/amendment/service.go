package amendment

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

type Amendment struct {
	ID              string    `json:"id"`
	ContractID      string    `json:"contract_id"`
	Title           string    `json:"title"`
	Reason          string    `json:"reason"`
	SupplementDocID string    `json:"supplement_doc_id"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
}

type AmendmentRecord struct {
	ID              string `gorm:"primaryKey;size:64"`
	ContractID      string `gorm:"size:64;index;not null"`
	Title           string `gorm:"size:255;not null"`
	Reason          string `gorm:"type:text"`
	SupplementDocID string `gorm:"size:64"`
	Status          string `gorm:"size:32;index;not null"`
	CreatedAt       time.Time
}

type Service struct {
	mu                 sync.RWMutex
	amendments         map[string]Amendment
	seq                int
	db                 *gorm.DB
	contractServiceURL string
}

func New() *Service {
	return &Service{
		amendments: make(map[string]Amendment),
	}
}

func NewWithDB(db *gorm.DB) *Service {
	service := New()
	service.db = db
	if db != nil {
		_ = db.AutoMigrate(&AmendmentRecord{})
	}
	return service
}

func (s *Service) RegisterRoutes(router gin.IRouter) {
	router.GET("/amendments", s.list)
	router.POST("/amendments", s.create)
	router.POST("/amendments/:id/approve", s.approve)
}

func (s *Service) SetContractServiceURL(rawURL string) {
	s.contractServiceURL = rawURL
}

func (s *Service) list(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "amendment.read") {
		return
	}
	if s.db != nil {
		var rows []AmendmentRecord
		if err := s.db.Order("created_at desc").Find(&rows).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to list amendments")
			return
		}

		result := make([]Amendment, 0, len(rows))
		for _, row := range rows {
			result = append(result, Amendment{
				ID:              row.ID,
				ContractID:      row.ContractID,
				Title:           row.Title,
				Reason:          row.Reason,
				SupplementDocID: row.SupplementDocID,
				Status:          row.Status,
				CreatedAt:       row.CreatedAt,
			})
		}
		httpx.Success(c, result)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Amendment, 0, len(s.amendments))
	for _, item := range s.amendments {
		result = append(result, item)
	}
	httpx.Success(c, result)
}

func (s *Service) create(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "amendment.write") {
		return
	}
	var req struct {
		ContractID      string `json:"contract_id"`
		Title           string `json:"title"`
		Reason          string `json:"reason"`
		SupplementDocID string `json:"supplement_doc_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid amendment payload")
		return
	}
	if strings.TrimSpace(req.ContractID) == "" || strings.TrimSpace(req.Title) == "" {
		httpx.Error(c, http.StatusBadRequest, "contract_id and title are required")
		return
	}

	s.mu.Lock()
	s.seq++
	item := Amendment{
		ID:              fmt.Sprintf("amd-%04d", s.seq),
		ContractID:      req.ContractID,
		Title:           req.Title,
		Reason:          req.Reason,
		SupplementDocID: req.SupplementDocID,
		Status:          "pending",
		CreatedAt:       time.Now(),
	}

	if s.db != nil {
		row := AmendmentRecord{
			ID:              item.ID,
			ContractID:      item.ContractID,
			Title:           item.Title,
			Reason:          item.Reason,
			SupplementDocID: item.SupplementDocID,
			Status:          item.Status,
			CreatedAt:       item.CreatedAt,
		}
		s.mu.Unlock()
		if err := s.db.Create(&row).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to create amendment")
			return
		}
	} else {
		s.amendments[item.ID] = item
		s.mu.Unlock()
	}

	httpx.Created(c, item)
}

func (s *Service) approve(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "amendment.approve") {
		return
	}
	if s.db != nil {
		var row AmendmentRecord
		if err := s.db.First(&row, "id = ?", c.Param("id")).Error; err != nil {
			httpx.Error(c, http.StatusNotFound, "amendment not found")
			return
		}
		row.Status = "approved"
		if err := s.db.Save(&row).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to approve amendment")
			return
		}
		if err := s.notifyContractAmendment(c, row.ContractID, row.ID, row.Title); err != nil {
			httpx.Error(c, http.StatusBadGateway, "approved amendment but failed to update contract view")
			return
		}
		httpx.Success(c, Amendment{
			ID:              row.ID,
			ContractID:      row.ContractID,
			Title:           row.Title,
			Reason:          row.Reason,
			SupplementDocID: row.SupplementDocID,
			Status:          row.Status,
			CreatedAt:       row.CreatedAt,
		})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.amendments[c.Param("id")]
	if !ok {
		httpx.Error(c, http.StatusNotFound, "amendment not found")
		return
	}

	item.Status = "approved"
	s.amendments[item.ID] = item
	if err := s.notifyContractAmendment(c, item.ContractID, item.ID, item.Title); err != nil {
		httpx.Error(c, http.StatusBadGateway, "approved amendment but failed to update contract view")
		return
	}
	httpx.Success(c, item)
}

func (s *Service) notifyContractAmendment(c *gin.Context, contractID, amendmentID, amendmentTitle string) error {
	if contractID == "" || s.contractServiceURL == "" {
		return nil
	}
	operatorID := middleware.CurrentOperatorID(c, "system")
	payload, err := json.Marshal(gin.H{
		"amendment_id":    amendmentID,
		"amendment_title": amendmentTitle,
		"operator_id":     operatorID,
		"description":     "amendment approved and applied to contract view",
	})
	if err != nil {
		return err
	}
	resp, err := http.Post(
		fmt.Sprintf("%s/api/v1/contracts/%s/amendments/apply", s.contractServiceURL, contractID),
		"application/json",
		bytes.NewReader(payload),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("contract amendment apply failed: %s", string(body))
	}
	return nil
}
