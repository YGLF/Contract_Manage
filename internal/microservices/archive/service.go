package archive

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"contract-manage/pkg/microplatform/httpx"
	"contract-manage/pkg/microplatform/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ArchiveCase struct {
	ID                    string    `json:"id"`
	ContractID            string    `json:"contract_id"`
	Department            string    `json:"department,omitempty"`
	Status                string    `json:"status"`
	ArchiveType           string    `json:"archive_type"`
	BorrowStatus          string    `json:"borrow_status"`
	DestroyState          string    `json:"destroy_state"`
	LastBorrowApprovalID  string    `json:"last_borrow_approval_id,omitempty"`
	LastBorrowApprovedBy  string    `json:"last_borrow_approved_by,omitempty"`
	LastDestroyApprovalID string    `json:"last_destroy_approval_id,omitempty"`
	LastDestroyApprovedBy string    `json:"last_destroy_approved_by,omitempty"`
	CreatedAt             time.Time `json:"created_at"`
}

type ArchiveMaterial struct {
	ID            string    `json:"id"`
	ArchiveCaseID string    `json:"archive_case_id"`
	MaterialType  string    `json:"material_type"`
	DocumentID    string    `json:"document_id"`
	Description   string    `json:"description"`
	CreatedBy     string    `json:"created_by"`
	CreatedAt     time.Time `json:"created_at"`
}

type ArchiveCaseRecord struct {
	ID                    string `gorm:"primaryKey;size:64"`
	ContractID            string `gorm:"size:64;index;not null"`
	Department            string `gorm:"size:64;index"`
	Status                string `gorm:"size:32;index;not null"`
	ArchiveType           string `gorm:"size:32;index;not null"`
	BorrowStatus          string `gorm:"size:32;index;not null"`
	DestroyState          string `gorm:"size:32;index;not null"`
	LastBorrowApprovalID  string `gorm:"size:64;index"`
	LastBorrowApprovedBy  string `gorm:"size:64"`
	LastDestroyApprovalID string `gorm:"size:64;index"`
	LastDestroyApprovedBy string `gorm:"size:64"`
	CreatedAt             time.Time
}

type ArchiveMaterialRecord struct {
	ID            string `gorm:"primaryKey;size:64"`
	ArchiveCaseID string `gorm:"size:64;index;not null"`
	MaterialType  string `gorm:"size:64;index;not null"`
	DocumentID    string `gorm:"size:64;index;not null"`
	Description   string `gorm:"type:text"`
	CreatedBy     string `gorm:"size:64;index;not null"`
	CreatedAt     time.Time
}

type Service struct {
	mu        sync.RWMutex
	items     map[string]ArchiveCase
	materials map[string][]ArchiveMaterial
	seq       int
	db        *gorm.DB
}

func New() *Service {
	return &Service{
		items:     make(map[string]ArchiveCase),
		materials: make(map[string][]ArchiveMaterial),
	}
}

func NewWithDB(db *gorm.DB) *Service {
	service := New()
	service.db = db
	if db != nil {
		_ = db.AutoMigrate(&ArchiveCaseRecord{})
		_ = db.AutoMigrate(&ArchiveMaterialRecord{})
	}
	return service
}

func (s *Service) RegisterRoutes(router gin.IRouter) {
	router.GET("/archive/cases", s.list)
	router.POST("/archive/cases", s.create)
	router.GET("/archive/cases/:id/materials", s.listMaterials)
	router.POST("/archive/cases/:id/materials", s.addMaterial)
	router.POST("/archive/cases/:id/borrow", s.borrow)
	router.POST("/archive/cases/:id/destroy", s.destroy)
}

func (s *Service) list(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "archive.read") {
		return
	}
	if s.db != nil {
		var rows []ArchiveCaseRecord
		query := s.db.Order("created_at desc")
		if department, limited := scopedDepartment(c); limited {
			query = query.Where("department = ?", department)
		}
		if err := query.Find(&rows).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to list archive cases")
			return
		}

		result := make([]ArchiveCase, 0, len(rows))
		for _, row := range rows {
			result = append(result, ArchiveCase{
				ID:                    row.ID,
				ContractID:            row.ContractID,
				Department:            row.Department,
				Status:                row.Status,
				ArchiveType:           row.ArchiveType,
				BorrowStatus:          row.BorrowStatus,
				DestroyState:          row.DestroyState,
				LastBorrowApprovalID:  row.LastBorrowApprovalID,
				LastBorrowApprovedBy:  row.LastBorrowApprovedBy,
				LastDestroyApprovalID: row.LastDestroyApprovalID,
				LastDestroyApprovedBy: row.LastDestroyApprovedBy,
				CreatedAt:             row.CreatedAt,
			})
		}
		httpx.Success(c, result)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]ArchiveCase, 0, len(s.items))
	for _, item := range s.items {
		if !canAccessDepartment(c, item.Department) {
			continue
		}
		result = append(result, item)
	}
	httpx.Success(c, result)
}

func (s *Service) create(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "archive.write") {
		return
	}
	var req struct {
		ContractID  string `json:"contract_id"`
		ArchiveType string `json:"archive_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid archive payload")
		return
	}

	s.mu.Lock()
	s.seq++
	item := ArchiveCase{
		ID:           fmt.Sprintf("arc-%04d", s.seq),
		ContractID:   req.ContractID,
		Department:   operatorDepartment(c),
		Status:       "archived",
		ArchiveType:  req.ArchiveType,
		BorrowStatus: "idle",
		DestroyState: "retained",
		CreatedAt:    time.Now(),
	}

	if s.db != nil {
		row := ArchiveCaseRecord{
			ID:                    item.ID,
			ContractID:            item.ContractID,
			Department:            item.Department,
			Status:                item.Status,
			ArchiveType:           item.ArchiveType,
			BorrowStatus:          item.BorrowStatus,
			DestroyState:          item.DestroyState,
			LastBorrowApprovalID:  item.LastBorrowApprovalID,
			LastBorrowApprovedBy:  item.LastBorrowApprovedBy,
			LastDestroyApprovalID: item.LastDestroyApprovalID,
			LastDestroyApprovedBy: item.LastDestroyApprovedBy,
			CreatedAt:             item.CreatedAt,
		}
		s.mu.Unlock()
		if err := s.db.Create(&row).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to create archive case")
			return
		}
	} else {
		s.items[item.ID] = item
		s.mu.Unlock()
	}

	httpx.Created(c, item)
}

func (s *Service) borrow(c *gin.Context) {
	s.updateCase(c, "borrowed", "", "borrow")
}

func (s *Service) destroy(c *gin.Context) {
	s.updateCase(c, "", "requested", "destroy")
}

func (s *Service) updateCase(c *gin.Context, borrowStatus, destroyState, action string) {
	if action == "borrow" && !middleware.EnforcePermissionIfPresent(c, "archive.borrow") {
		return
	}
	if action == "destroy" && !middleware.EnforcePermissionIfPresent(c, "archive.destroy") {
		return
	}
	var req struct {
		ApprovalRequestID string `json:"approval_request_id"`
		ApprovedBy        string `json:"approved_by"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid archive approval payload")
		return
	}
	if req.ApprovalRequestID == "" || req.ApprovedBy == "" {
		httpx.Error(c, http.StatusBadRequest, "approval_request_id and approved_by are required")
		return
	}

	if s.db != nil {
		var row ArchiveCaseRecord
		if err := s.db.First(&row, "id = ?", c.Param("id")).Error; err != nil {
			httpx.Error(c, http.StatusNotFound, "archive case not found")
			return
		}
		if !canAccessDepartment(c, row.Department) {
			httpx.Error(c, http.StatusForbidden, "department data scope denied")
			return
		}
		if borrowStatus != "" {
			row.BorrowStatus = borrowStatus
			row.LastBorrowApprovalID = req.ApprovalRequestID
			row.LastBorrowApprovedBy = req.ApprovedBy
		}
		if destroyState != "" {
			row.DestroyState = destroyState
			row.LastDestroyApprovalID = req.ApprovalRequestID
			row.LastDestroyApprovedBy = req.ApprovedBy
		}
		if err := s.db.Save(&row).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to update archive case")
			return
		}
		httpx.Success(c, ArchiveCase{
			ID:                    row.ID,
			ContractID:            row.ContractID,
			Department:            row.Department,
			Status:                row.Status,
			ArchiveType:           row.ArchiveType,
			BorrowStatus:          row.BorrowStatus,
			DestroyState:          row.DestroyState,
			LastBorrowApprovalID:  row.LastBorrowApprovalID,
			LastBorrowApprovedBy:  row.LastBorrowApprovedBy,
			LastDestroyApprovalID: row.LastDestroyApprovalID,
			LastDestroyApprovedBy: row.LastDestroyApprovedBy,
			CreatedAt:             row.CreatedAt,
		})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[c.Param("id")]
	if !ok {
		httpx.Error(c, http.StatusNotFound, "archive case not found")
		return
	}
	if !canAccessDepartment(c, item.Department) {
		httpx.Error(c, http.StatusForbidden, "department data scope denied")
		return
	}
	if borrowStatus != "" {
		item.BorrowStatus = borrowStatus
		item.LastBorrowApprovalID = req.ApprovalRequestID
		item.LastBorrowApprovedBy = req.ApprovedBy
	}
	if destroyState != "" {
		item.DestroyState = destroyState
		item.LastDestroyApprovalID = req.ApprovalRequestID
		item.LastDestroyApprovedBy = req.ApprovedBy
	}
	s.items[item.ID] = item

	if action == "borrow" && item.BorrowStatus != "borrowed" {
		httpx.Error(c, http.StatusConflict, "archive borrow must be approved before update")
		return
	}
	if action == "destroy" && item.DestroyState != "requested" {
		httpx.Error(c, http.StatusConflict, "archive destroy must be approved before update")
		return
	}
	httpx.Success(c, item)
}

func (s *Service) listMaterials(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "archive.read") {
		return
	}
	if s.db != nil {
		var caseRow ArchiveCaseRecord
		if err := s.db.First(&caseRow, "id = ?", c.Param("id")).Error; err != nil {
			httpx.Error(c, http.StatusNotFound, "archive case not found")
			return
		}
		if !canAccessDepartment(c, caseRow.Department) {
			httpx.Error(c, http.StatusForbidden, "department data scope denied")
			return
		}
		var rows []ArchiveMaterialRecord
		if err := s.db.Where("archive_case_id = ?", c.Param("id")).Order("created_at desc").Find(&rows).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to list archive materials")
			return
		}
		result := make([]ArchiveMaterial, 0, len(rows))
		for _, row := range rows {
			result = append(result, ArchiveMaterial{
				ID:            row.ID,
				ArchiveCaseID: row.ArchiveCaseID,
				MaterialType:  row.MaterialType,
				DocumentID:    row.DocumentID,
				Description:   row.Description,
				CreatedBy:     row.CreatedBy,
				CreatedAt:     row.CreatedAt,
			})
		}
		httpx.Success(c, result)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if item, ok := s.items[c.Param("id")]; ok && !canAccessDepartment(c, item.Department) {
		httpx.Error(c, http.StatusForbidden, "department data scope denied")
		return
	}
	httpx.Success(c, s.materials[c.Param("id")])
}

func (s *Service) addMaterial(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "archive.write") {
		return
	}
	var req struct {
		MaterialType string `json:"material_type"`
		DocumentID   string `json:"document_id"`
		Description  string `json:"description"`
		CreatedBy    string `json:"created_by"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid archive material payload")
		return
	}
	if req.CreatedBy == "" {
		req.CreatedBy = middleware.CurrentOperatorID(c, "system")
	}
	if req.MaterialType == "" || req.DocumentID == "" || req.CreatedBy == "" {
		httpx.Error(c, http.StatusBadRequest, "material_type, document_id and created_by are required")
		return
	}

	s.mu.Lock()
	s.seq++
	item := ArchiveMaterial{
		ID:            fmt.Sprintf("arm-%04d", s.seq),
		ArchiveCaseID: c.Param("id"),
		MaterialType:  req.MaterialType,
		DocumentID:    req.DocumentID,
		Description:   req.Description,
		CreatedBy:     req.CreatedBy,
		CreatedAt:     time.Now(),
	}

	if s.db != nil {
		s.mu.Unlock()
		var caseRow ArchiveCaseRecord
		if err := s.db.First(&caseRow, "id = ?", c.Param("id")).Error; err != nil {
			httpx.Error(c, http.StatusNotFound, "archive case not found")
			return
		}
		if !canAccessDepartment(c, caseRow.Department) {
			httpx.Error(c, http.StatusForbidden, "department data scope denied")
			return
		}
		row := ArchiveMaterialRecord{
			ID:            item.ID,
			ArchiveCaseID: item.ArchiveCaseID,
			MaterialType:  item.MaterialType,
			DocumentID:    item.DocumentID,
			Description:   item.Description,
			CreatedBy:     item.CreatedBy,
			CreatedAt:     item.CreatedAt,
		}
		if err := s.db.Create(&row).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to add archive material")
			return
		}
		httpx.Created(c, item)
		return
	}

	caseItem, ok := s.items[c.Param("id")]
	if !ok {
		s.mu.Unlock()
		httpx.Error(c, http.StatusNotFound, "archive case not found")
		return
	}
	if !canAccessDepartment(c, caseItem.Department) {
		s.mu.Unlock()
		httpx.Error(c, http.StatusForbidden, "department data scope denied")
		return
	}
	s.materials[c.Param("id")] = append([]ArchiveMaterial{item}, s.materials[c.Param("id")]...)
	s.mu.Unlock()
	httpx.Created(c, item)
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
