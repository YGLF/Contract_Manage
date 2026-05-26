package document

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"contract-manage/pkg/microplatform/auditclient"
	"contract-manage/pkg/microplatform/httpx"
	"contract-manage/pkg/microplatform/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type TempDocument struct {
	ID              string    `json:"id"`
	FileName        string    `json:"file_name"`
	TempPath        string    `json:"temp_path"`
	Hash            string    `json:"hash"`
	Size            int64     `json:"size"`
	Status          string    `json:"status"`
	BoundContractID string    `json:"bound_contract_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

type TempDocumentRecord struct {
	ID              string `gorm:"primaryKey;size:64"`
	FileName        string `gorm:"size:255;not null"`
	TempPath        string `gorm:"size:1024;not null"`
	Hash            string `gorm:"size:128;not null"`
	Size            int64  `gorm:"not null"`
	Status          string `gorm:"size:32;index"`
	BoundContractID string `gorm:"size:64;index"`
	CreatedAt       time.Time
}

type Service struct {
	mu        sync.RWMutex
	uploadDir string
	tempDocs  map[string]TempDocument
	seq       int
	db        *gorm.DB
	audit     *auditclient.Client
}

func New(uploadDir string) *Service {
	return &Service{
		uploadDir: uploadDir,
		tempDocs:  make(map[string]TempDocument),
	}
}

func NewWithDB(uploadDir string, db *gorm.DB) *Service {
	service := New(uploadDir)
	service.db = db
	if db != nil {
		_ = db.AutoMigrate(&TempDocumentRecord{})
	}
	return service
}

func (s *Service) SetAuditClient(client *auditclient.Client) {
	s.audit = client
}

func (s *Service) RegisterRoutes(router gin.IRouter) {
	router.POST("/documents/temp", s.uploadTemp)
	router.POST("/documents/commit", s.commit)
	router.POST("/documents/bind", s.bind)
	router.POST("/documents/release", s.release)
	router.GET("/documents/temp", s.list)
	router.GET("/documents/temp/:id", s.getTemp)
	router.GET("/documents/temp/:id/download", s.downloadTemp)
}

func (s *Service) list(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "document.read") {
		return
	}
	if s.db != nil {
		var rows []TempDocumentRecord
		if err := s.db.Order("created_at desc").Find(&rows).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to list temp documents")
			return
		}
		result := make([]TempDocument, 0, len(rows))
		for _, row := range rows {
			result = append(result, TempDocument{
				ID:              row.ID,
				FileName:        row.FileName,
				TempPath:        row.TempPath,
				Hash:            row.Hash,
				Size:            row.Size,
				Status:          row.Status,
				BoundContractID: row.BoundContractID,
				CreatedAt:       row.CreatedAt,
			})
		}
		httpx.Success(c, result)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]TempDocument, 0, len(s.tempDocs))
	for _, doc := range s.tempDocs {
		result = append(result, doc)
	}
	httpx.Success(c, result)
}

func (s *Service) getTemp(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "document.read") {
		return
	}
	if s.db != nil {
		var row TempDocumentRecord
		if err := s.db.First(&row, "id = ?", c.Param("id")).Error; err != nil {
			httpx.Error(c, http.StatusNotFound, "temp document not found")
			return
		}
		httpx.Success(c, TempDocument{
			ID:              row.ID,
			FileName:        row.FileName,
			TempPath:        row.TempPath,
			Hash:            row.Hash,
			Size:            row.Size,
			Status:          row.Status,
			BoundContractID: row.BoundContractID,
			CreatedAt:       row.CreatedAt,
		})
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	doc, ok := s.tempDocs[c.Param("id")]
	if !ok {
		httpx.Error(c, http.StatusNotFound, "temp document not found")
		return
	}
	httpx.Success(c, doc)
}

func (s *Service) downloadTemp(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "document.read") {
		return
	}
	documentID := c.Param("id")
	var doc TempDocument

	if s.db != nil {
		var row TempDocumentRecord
		if err := s.db.First(&row, "id = ?", documentID).Error; err != nil {
			s.recordAudit(c, "document.download_failed", map[string]interface{}{
				"document_id": documentID,
				"reason":      "temp document not found",
			})
			httpx.Error(c, http.StatusNotFound, "temp document not found")
			return
		}
		doc = TempDocument{
			ID:              row.ID,
			FileName:        row.FileName,
			TempPath:        row.TempPath,
			Hash:            row.Hash,
			Size:            row.Size,
			Status:          row.Status,
			BoundContractID: row.BoundContractID,
			CreatedAt:       row.CreatedAt,
		}
	} else {
		s.mu.RLock()
		var ok bool
		doc, ok = s.tempDocs[documentID]
		s.mu.RUnlock()
		if !ok {
			s.recordAudit(c, "document.download_failed", map[string]interface{}{
				"document_id": documentID,
				"reason":      "temp document not found",
			})
			httpx.Error(c, http.StatusNotFound, "temp document not found")
			return
		}
	}

	if _, err := os.Stat(doc.TempPath); err != nil {
		s.recordAudit(c, "document.download_failed", map[string]interface{}{
			"document_id": documentID,
			"file_name":   doc.FileName,
			"reason":      "document file not found",
		})
		httpx.Error(c, http.StatusNotFound, "document file not found")
		return
	}

	contentType := mime.TypeByExtension(filepath.Ext(doc.FileName))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", doc.FileName))
	s.recordAudit(c, "document.downloaded", map[string]interface{}{
		"document_id":       doc.ID,
		"file_name":         doc.FileName,
		"status":            doc.Status,
		"bound_contract_id": doc.BoundContractID,
	})
	c.File(doc.TempPath)
}

func (s *Service) uploadTemp(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "document.upload") {
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "missing file")
		return
	}
	defer file.Close()

	if err := os.MkdirAll(filepath.Join(s.uploadDir, "tmp"), 0o755); err != nil {
		httpx.Error(c, http.StatusInternalServerError, "failed to prepare upload directory")
		return
	}

	s.mu.Lock()
	s.seq++
	id := fmt.Sprintf("doc-temp-%04d", s.seq)
	s.mu.Unlock()

	tempPath := filepath.Join(s.uploadDir, "tmp", id+"-"+header.Filename)
	out, err := os.Create(tempPath)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "failed to create temp file")
		return
	}
	defer out.Close()

	hasher := sha256.New()
	size, err := io.Copy(io.MultiWriter(out, hasher), file)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "failed to persist temp file")
		return
	}

	doc := TempDocument{
		ID:        id,
		FileName:  header.Filename,
		TempPath:  tempPath,
		Hash:      hex.EncodeToString(hasher.Sum(nil)),
		Size:      size,
		Status:    "uploaded",
		CreatedAt: time.Now(),
	}

	if s.db != nil {
		record := TempDocumentRecord{
			ID:              doc.ID,
			FileName:        doc.FileName,
			TempPath:        doc.TempPath,
			Hash:            doc.Hash,
			Size:            doc.Size,
			Status:          doc.Status,
			BoundContractID: doc.BoundContractID,
			CreatedAt:       doc.CreatedAt,
		}
		if err := s.db.Create(&record).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to persist temp document")
			return
		}
	} else {
		s.mu.Lock()
		s.tempDocs[id] = doc
		s.mu.Unlock()
	}

	httpx.Created(c, doc)
}

func (s *Service) commit(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "document.commit") {
		return
	}
	var req struct {
		TempDocumentID string `json:"temp_document_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid commit payload")
		return
	}

	if s.db != nil {
		var row TempDocumentRecord
		if err := s.db.First(&row, "id = ?", req.TempDocumentID).Error; err != nil {
			httpx.Error(c, http.StatusNotFound, "temp document not found")
			return
		}
		row.Status = "committed"
		if err := s.db.Save(&row).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to commit temp document")
			return
		}
		httpx.Success(c, TempDocument{
			ID:              row.ID,
			FileName:        row.FileName,
			TempPath:        row.TempPath,
			Hash:            row.Hash,
			Size:            row.Size,
			Status:          row.Status,
			BoundContractID: row.BoundContractID,
			CreatedAt:       row.CreatedAt,
		})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	doc, ok := s.tempDocs[req.TempDocumentID]
	if !ok {
		httpx.Error(c, http.StatusNotFound, "temp document not found")
		return
	}

	doc.Status = "committed"
	s.tempDocs[req.TempDocumentID] = doc
	httpx.Success(c, doc)
}

func (s *Service) bind(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "document.bind") {
		return
	}
	var req struct {
		DocumentIDs []string `json:"document_ids"`
		ContractID  string   `json:"contract_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid bind payload")
		return
	}
	if req.ContractID == "" || len(req.DocumentIDs) == 0 {
		httpx.Error(c, http.StatusBadRequest, "contract_id and document_ids are required")
		return
	}

	if s.db != nil {
		err := s.db.Transaction(func(tx *gorm.DB) error {
			var rows []TempDocumentRecord
			if err := tx.Where("id IN ?", req.DocumentIDs).Find(&rows).Error; err != nil {
				return err
			}
			if len(rows) != len(req.DocumentIDs) {
				return fmt.Errorf("one or more temp documents not found")
			}
			for _, row := range rows {
				if row.Status != "committed" {
					return fmt.Errorf("only committed temp documents can be bound")
				}
			}
			for _, row := range rows {
				row.Status = "bound"
				row.BoundContractID = req.ContractID
				if err := tx.Save(&row).Error; err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			switch err.Error() {
			case "one or more temp documents not found":
				httpx.Error(c, http.StatusNotFound, err.Error())
			case "only committed temp documents can be bound":
				httpx.Error(c, http.StatusConflict, err.Error())
			default:
				httpx.Error(c, http.StatusInternalServerError, "failed to bind temp documents")
			}
			return
		}
		httpx.Success(c, gin.H{
			"contract_id":  req.ContractID,
			"document_ids": req.DocumentIDs,
			"status":       "bound",
		})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range req.DocumentIDs {
		doc, ok := s.tempDocs[id]
		if !ok {
			httpx.Error(c, http.StatusNotFound, "one or more temp documents not found")
			return
		}
		if doc.Status != "committed" {
			httpx.Error(c, http.StatusConflict, "only committed temp documents can be bound")
			return
		}
	}
	for _, id := range req.DocumentIDs {
		doc := s.tempDocs[id]
		doc.Status = "bound"
		doc.BoundContractID = req.ContractID
		s.tempDocs[id] = doc
	}
	httpx.Success(c, gin.H{
		"contract_id":  req.ContractID,
		"document_ids": req.DocumentIDs,
		"status":       "bound",
	})
}

func (s *Service) release(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "document.bind") {
		return
	}
	var req struct {
		DocumentIDs []string `json:"document_ids"`
		ContractID  string   `json:"contract_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid release payload")
		return
	}
	if len(req.DocumentIDs) == 0 {
		httpx.Error(c, http.StatusBadRequest, "document_ids are required")
		return
	}

	if s.db != nil {
		err := s.db.Transaction(func(tx *gorm.DB) error {
			var rows []TempDocumentRecord
			if err := tx.Where("id IN ?", req.DocumentIDs).Find(&rows).Error; err != nil {
				return err
			}
			if len(rows) != len(req.DocumentIDs) {
				return fmt.Errorf("one or more temp documents not found")
			}
			for _, row := range rows {
				if row.Status != "bound" {
					return fmt.Errorf("only bound temp documents can be released")
				}
				if req.ContractID != "" && row.BoundContractID != req.ContractID {
					return fmt.Errorf("temp document is not bound to the specified contract")
				}
			}
			for _, row := range rows {
				row.Status = "committed"
				row.BoundContractID = ""
				if err := tx.Save(&row).Error; err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			switch err.Error() {
			case "one or more temp documents not found":
				httpx.Error(c, http.StatusNotFound, err.Error())
			case "only bound temp documents can be released", "temp document is not bound to the specified contract":
				httpx.Error(c, http.StatusConflict, err.Error())
			default:
				httpx.Error(c, http.StatusInternalServerError, "failed to release temp documents")
			}
			return
		}
		httpx.Success(c, gin.H{
			"contract_id":  req.ContractID,
			"document_ids": req.DocumentIDs,
			"status":       "committed",
		})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range req.DocumentIDs {
		doc, ok := s.tempDocs[id]
		if !ok {
			httpx.Error(c, http.StatusNotFound, "one or more temp documents not found")
			return
		}
		if doc.Status != "bound" {
			httpx.Error(c, http.StatusConflict, "only bound temp documents can be released")
			return
		}
		if req.ContractID != "" && doc.BoundContractID != req.ContractID {
			httpx.Error(c, http.StatusConflict, "temp document is not bound to the specified contract")
			return
		}
	}
	for _, id := range req.DocumentIDs {
		doc := s.tempDocs[id]
		doc.Status = "committed"
		doc.BoundContractID = ""
		s.tempDocs[id] = doc
	}
	httpx.Success(c, gin.H{
		"contract_id":  req.ContractID,
		"document_ids": req.DocumentIDs,
		"status":       "committed",
	})
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
		fmt.Sprintf("audit-document-%d", time.Now().UnixNano()),
		action,
		middleware.CurrentOperatorID(c, "system"),
		trace,
		payload,
	)
}
