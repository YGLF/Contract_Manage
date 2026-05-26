package party

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"contract-manage/pkg/microplatform/httpx"
	"contract-manage/pkg/microplatform/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Party struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	UnifiedSocialCode string    `json:"unified_social_code"`
	OwnerDepartment   string    `json:"owner_department,omitempty"`
	ContactName       string    `json:"contact_name"`
	ContactPhone      string    `json:"contact_phone"`
	CreditRating      string    `json:"credit_rating"`
	CreditSource      string    `json:"credit_source"`
	CreditUpdatedAt   time.Time `json:"credit_updated_at"`
	CooperationCount  int       `json:"cooperation_count"`
	LastContractDate  time.Time `json:"last_contract_date,omitempty"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"created_at"`
}

type PartyRecord struct {
	ID                string `gorm:"primaryKey;size:64"`
	Name              string `gorm:"size:255;index;not null"`
	UnifiedSocialCode string `gorm:"size:64;uniqueIndex;not null"`
	OwnerDepartment   string `gorm:"size:64;index"`
	ContactName       string `gorm:"size:64"`
	ContactPhone      string `gorm:"size:32"`
	CreditRating      string `gorm:"size:32;index"`
	CreditSource      string `gorm:"size:64"`
	CreditUpdatedAt   time.Time
	CooperationCount  int
	LastContractDate  *time.Time
	Status            string `gorm:"size:32;index;not null"`
	CreatedAt         time.Time
}

type CreditSnapshot struct {
	ID          string    `json:"id"`
	PartyID     string    `json:"party_id"`
	Rating      string    `json:"rating"`
	Source      string    `json:"source"`
	RiskFlag    string    `json:"risk_flag"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type CreditSnapshotRecord struct {
	ID          string `gorm:"primaryKey;size:64"`
	PartyID     string `gorm:"size:64;index;not null"`
	Rating      string `gorm:"size:32;index;not null"`
	Source      string `gorm:"size:64;index;not null"`
	RiskFlag    string `gorm:"size:32;index"`
	Description string `gorm:"type:text"`
	CreatedAt   time.Time
}

type CooperationHistory struct {
	ID            string    `json:"id"`
	PartyID       string    `json:"party_id"`
	ContractID    string    `json:"contract_id"`
	ContractTitle string    `json:"contract_title"`
	ContractNo    string    `json:"contract_no"`
	SignedAt      time.Time `json:"signed_at"`
	Status        string    `json:"status"`
}

type CooperationHistoryRecord struct {
	ID            string    `gorm:"primaryKey;size:64"`
	PartyID       string    `gorm:"size:64;index;not null"`
	ContractID    string    `gorm:"size:64;index;not null"`
	ContractTitle string    `gorm:"size:255;not null"`
	ContractNo    string    `gorm:"size:64;index"`
	SignedAt      time.Time `gorm:"index"`
	Status        string    `gorm:"size:32;index"`
}

type Service struct {
	mu        sync.RWMutex
	parties   map[string]Party
	snapshots map[string][]CreditSnapshot
	history   map[string][]CooperationHistory
	seq       int
	db        *gorm.DB
}

func New() *Service {
	return &Service{
		parties:   make(map[string]Party),
		snapshots: make(map[string][]CreditSnapshot),
		history:   make(map[string][]CooperationHistory),
	}
}

func NewWithDB(db *gorm.DB) *Service {
	service := New()
	service.db = db
	if db != nil {
		_ = db.AutoMigrate(&PartyRecord{})
		_ = db.AutoMigrate(&CreditSnapshotRecord{})
		_ = db.AutoMigrate(&CooperationHistoryRecord{})
	}
	return service
}

func (s *Service) RegisterRoutes(router gin.IRouter) {
	router.GET("/parties", s.list)
	router.POST("/parties", s.create)
	router.GET("/parties/:id", s.get)
	router.PUT("/parties/:id", s.update)
	router.DELETE("/parties/:id", s.remove)
	router.POST("/parties/:id/credit-snapshots", s.addCreditSnapshot)
	router.GET("/parties/:id/credit-snapshots/latest", s.latestCreditSnapshot)
	router.GET("/parties/:id/credit-check", s.creditCheck)
	router.POST("/parties/:id/cooperation-history", s.addCooperationHistory)
	router.GET("/parties/:id/cooperation-summary", s.cooperationSummary)
}

func (s *Service) list(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "party.read") {
		return
	}
	if s.db != nil {
		var rows []PartyRecord
		query := s.db.Order("created_at desc")
		if department, limited := scopedDepartment(c); limited {
			query = query.Where("owner_department = ?", department)
		}
		if name := strings.TrimSpace(c.Query("name")); name != "" {
			query = query.Where("name LIKE ?", "%"+name+"%")
		}
		if status := strings.TrimSpace(c.Query("status")); status != "" {
			query = query.Where("status = ?", status)
		}
		if err := query.Find(&rows).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to list parties")
			return
		}
		result := make([]Party, 0, len(rows))
		for _, row := range rows {
			result = append(result, convertPartyRecord(row))
		}
		httpx.Success(c, result)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Party, 0, len(s.parties))
	for _, item := range s.parties {
		if !canAccessDepartment(c, item.OwnerDepartment) {
			continue
		}
		if name := strings.TrimSpace(c.Query("name")); name != "" && !strings.Contains(item.Name, name) {
			continue
		}
		if status := strings.TrimSpace(c.Query("status")); status != "" && item.Status != status {
			continue
		}
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	httpx.Success(c, result)
}

func (s *Service) create(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "party.write") {
		return
	}
	var req struct {
		Name              string `json:"name"`
		UnifiedSocialCode string `json:"unified_social_code"`
		ContactName       string `json:"contact_name"`
		ContactPhone      string `json:"contact_phone"`
		CreditRating      string `json:"credit_rating"`
		CreditSource      string `json:"credit_source"`
		Status            string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid party payload")
		return
	}
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.UnifiedSocialCode) == "" {
		httpx.Error(c, http.StatusBadRequest, "name and unified_social_code are required")
		return
	}
	if strings.TrimSpace(req.Status) == "" {
		req.Status = "active"
	}

	s.mu.Lock()
	s.seq++
	item := Party{
		ID:                fmt.Sprintf("party-%04d", s.seq),
		Name:              req.Name,
		UnifiedSocialCode: req.UnifiedSocialCode,
		OwnerDepartment:   operatorDepartment(c),
		ContactName:       req.ContactName,
		ContactPhone:      req.ContactPhone,
		CreditRating:      req.CreditRating,
		CreditSource:      req.CreditSource,
		CreditUpdatedAt:   time.Now(),
		Status:            req.Status,
		CreatedAt:         time.Now(),
	}

	if s.db != nil {
		row := PartyRecord{
			ID:                item.ID,
			Name:              item.Name,
			UnifiedSocialCode: item.UnifiedSocialCode,
			OwnerDepartment:   item.OwnerDepartment,
			ContactName:       item.ContactName,
			ContactPhone:      item.ContactPhone,
			CreditRating:      item.CreditRating,
			CreditSource:      item.CreditSource,
			CreditUpdatedAt:   item.CreditUpdatedAt,
			CooperationCount:  item.CooperationCount,
			Status:            item.Status,
			CreatedAt:         item.CreatedAt,
		}
		s.mu.Unlock()
		if err := s.db.Create(&row).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to create party")
			return
		}
	} else {
		s.parties[item.ID] = item
		s.mu.Unlock()
	}

	httpx.Created(c, item)
}

func (s *Service) get(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "party.read") {
		return
	}
	if s.db != nil {
		var row PartyRecord
		if err := s.db.First(&row, "id = ?", c.Param("id")).Error; err != nil {
			httpx.Error(c, http.StatusNotFound, "party not found")
			return
		}
		if !canAccessDepartment(c, row.OwnerDepartment) {
			httpx.Error(c, http.StatusForbidden, "department data scope denied")
			return
		}
		httpx.Success(c, convertPartyRecord(row))
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.parties[c.Param("id")]
	if !ok {
		httpx.Error(c, http.StatusNotFound, "party not found")
		return
	}
	if !canAccessDepartment(c, item.OwnerDepartment) {
		httpx.Error(c, http.StatusForbidden, "department data scope denied")
		return
	}
	httpx.Success(c, item)
}

func (s *Service) update(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "party.write") {
		return
	}
	var req struct {
		Name         string `json:"name"`
		ContactName  string `json:"contact_name"`
		ContactPhone string `json:"contact_phone"`
		CreditRating string `json:"credit_rating"`
		CreditSource string `json:"credit_source"`
		Status       string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid party update payload")
		return
	}

	if s.db != nil {
		var row PartyRecord
		if err := s.db.First(&row, "id = ?", c.Param("id")).Error; err != nil {
			httpx.Error(c, http.StatusNotFound, "party not found")
			return
		}
		if !canAccessDepartment(c, row.OwnerDepartment) {
			httpx.Error(c, http.StatusForbidden, "department data scope denied")
			return
		}
		if strings.TrimSpace(req.Name) != "" {
			row.Name = req.Name
		}
		if req.ContactName != "" {
			row.ContactName = req.ContactName
		}
		if req.ContactPhone != "" {
			row.ContactPhone = req.ContactPhone
		}
		if req.CreditRating != "" {
			row.CreditRating = req.CreditRating
			row.CreditUpdatedAt = time.Now()
		}
		if req.CreditSource != "" {
			row.CreditSource = req.CreditSource
		}
		if req.Status != "" {
			row.Status = req.Status
		}
		if err := s.db.Save(&row).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to update party")
			return
		}
		httpx.Success(c, convertPartyRecord(row))
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.parties[c.Param("id")]
	if !ok {
		httpx.Error(c, http.StatusNotFound, "party not found")
		return
	}
	if !canAccessDepartment(c, item.OwnerDepartment) {
		httpx.Error(c, http.StatusForbidden, "department data scope denied")
		return
	}
	if strings.TrimSpace(req.Name) != "" {
		item.Name = req.Name
	}
	if req.ContactName != "" {
		item.ContactName = req.ContactName
	}
	if req.ContactPhone != "" {
		item.ContactPhone = req.ContactPhone
	}
	if req.CreditRating != "" {
		item.CreditRating = req.CreditRating
		item.CreditUpdatedAt = time.Now()
	}
	if req.CreditSource != "" {
		item.CreditSource = req.CreditSource
	}
	if req.Status != "" {
		item.Status = req.Status
	}
	s.parties[item.ID] = item
	httpx.Success(c, item)
}

func (s *Service) remove(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "party.write") {
		return
	}
	if s.db != nil {
		var row PartyRecord
		if err := s.db.First(&row, "id = ?", c.Param("id")).Error; err != nil {
			httpx.Error(c, http.StatusNotFound, "party not found")
			return
		}
		if !canAccessDepartment(c, row.OwnerDepartment) {
			httpx.Error(c, http.StatusForbidden, "department data scope denied")
			return
		}
		err := s.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("party_id = ?", c.Param("id")).Delete(&CreditSnapshotRecord{}).Error; err != nil {
				return err
			}
			if err := tx.Where("party_id = ?", c.Param("id")).Delete(&CooperationHistoryRecord{}).Error; err != nil {
				return err
			}
			return tx.Where("id = ?", c.Param("id")).Delete(&PartyRecord{}).Error
		})
		if err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to delete party")
			return
		}
		httpx.Success(c, gin.H{"id": c.Param("id")})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.parties[c.Param("id")]; !ok {
		httpx.Error(c, http.StatusNotFound, "party not found")
		return
	}
	if !canAccessDepartment(c, s.parties[c.Param("id")].OwnerDepartment) {
		httpx.Error(c, http.StatusForbidden, "department data scope denied")
		return
	}
	delete(s.parties, c.Param("id"))
	delete(s.snapshots, c.Param("id"))
	delete(s.history, c.Param("id"))
	httpx.Success(c, gin.H{"id": c.Param("id")})
}

func (s *Service) addCreditSnapshot(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "party.credit.write") {
		return
	}
	var req struct {
		Rating      string `json:"rating"`
		Source      string `json:"source"`
		RiskFlag    string `json:"risk_flag"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid credit snapshot payload")
		return
	}
	if strings.TrimSpace(req.Rating) == "" || strings.TrimSpace(req.Source) == "" {
		httpx.Error(c, http.StatusBadRequest, "rating and source are required")
		return
	}

	s.mu.Lock()
	s.seq++
	snapshot := CreditSnapshot{
		ID:          fmt.Sprintf("pcs-%04d", s.seq),
		PartyID:     c.Param("id"),
		Rating:      req.Rating,
		Source:      req.Source,
		RiskFlag:    req.RiskFlag,
		Description: req.Description,
		CreatedAt:   time.Now(),
	}

	if s.db != nil {
		err := s.db.Transaction(func(tx *gorm.DB) error {
			var party PartyRecord
			if err := tx.First(&party, "id = ?", c.Param("id")).Error; err != nil {
				return err
			}
			if !canAccessDepartment(c, party.OwnerDepartment) {
				return gorm.ErrInvalidData
			}
			record := CreditSnapshotRecord{
				ID:          snapshot.ID,
				PartyID:     snapshot.PartyID,
				Rating:      snapshot.Rating,
				Source:      snapshot.Source,
				RiskFlag:    snapshot.RiskFlag,
				Description: snapshot.Description,
				CreatedAt:   snapshot.CreatedAt,
			}
			if err := tx.Create(&record).Error; err != nil {
				return err
			}
			party.CreditRating = snapshot.Rating
			party.CreditSource = snapshot.Source
			party.CreditUpdatedAt = snapshot.CreatedAt
			return tx.Save(&party).Error
		})
		s.mu.Unlock()
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				httpx.Error(c, http.StatusNotFound, "party not found")
				return
			}
			if err == gorm.ErrInvalidData {
				httpx.Error(c, http.StatusForbidden, "department data scope denied")
				return
			}
			httpx.Error(c, http.StatusInternalServerError, "failed to add credit snapshot")
			return
		}
	} else {
		party, ok := s.parties[c.Param("id")]
		if !ok {
			s.mu.Unlock()
			httpx.Error(c, http.StatusNotFound, "party not found")
			return
		}
		if !canAccessDepartment(c, party.OwnerDepartment) {
			s.mu.Unlock()
			httpx.Error(c, http.StatusForbidden, "department data scope denied")
			return
		}
		s.snapshots[snapshot.PartyID] = append(s.snapshots[snapshot.PartyID], snapshot)
		party.CreditRating = snapshot.Rating
		party.CreditSource = snapshot.Source
		party.CreditUpdatedAt = snapshot.CreatedAt
		s.parties[party.ID] = party
		s.mu.Unlock()
	}

	httpx.Created(c, snapshot)
}

func (s *Service) latestCreditSnapshot(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "party.read") {
		return
	}
	if s.db != nil {
		var party PartyRecord
		if err := s.db.First(&party, "id = ?", c.Param("id")).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				httpx.Error(c, http.StatusNotFound, "party not found")
				return
			}
			httpx.Error(c, http.StatusInternalServerError, "failed to load latest credit snapshot")
			return
		}
		if !canAccessDepartment(c, party.OwnerDepartment) {
			httpx.Error(c, http.StatusForbidden, "department data scope denied")
			return
		}
		var row CreditSnapshotRecord
		if err := s.db.Where("party_id = ?", c.Param("id")).Order("created_at desc").First(&row).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				httpx.Success(c, gin.H{})
				return
			}
			httpx.Error(c, http.StatusInternalServerError, "failed to load latest credit snapshot")
			return
		}
		httpx.Success(c, CreditSnapshot{
			ID:          row.ID,
			PartyID:     row.PartyID,
			Rating:      row.Rating,
			Source:      row.Source,
			RiskFlag:    row.RiskFlag,
			Description: row.Description,
			CreatedAt:   row.CreatedAt,
		})
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	items := s.snapshots[c.Param("id")]
	if party, ok := s.parties[c.Param("id")]; ok && !canAccessDepartment(c, party.OwnerDepartment) {
		httpx.Error(c, http.StatusForbidden, "department data scope denied")
		return
	}
	if len(items) == 0 {
		httpx.Success(c, gin.H{})
		return
	}
	httpx.Success(c, items[len(items)-1])
}

func (s *Service) addCooperationHistory(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "party.write") {
		return
	}
	var req struct {
		ContractID    string    `json:"contract_id"`
		ContractTitle string    `json:"contract_title"`
		ContractNo    string    `json:"contract_no"`
		SignedAt      time.Time `json:"signed_at"`
		Status        string    `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid cooperation history payload")
		return
	}
	if strings.TrimSpace(req.ContractID) == "" || strings.TrimSpace(req.ContractTitle) == "" {
		httpx.Error(c, http.StatusBadRequest, "contract_id and contract_title are required")
		return
	}
	if req.SignedAt.IsZero() {
		req.SignedAt = time.Now()
	}

	s.mu.Lock()
	s.seq++
	item := CooperationHistory{
		ID:            fmt.Sprintf("pch-%04d", s.seq),
		PartyID:       c.Param("id"),
		ContractID:    req.ContractID,
		ContractTitle: req.ContractTitle,
		ContractNo:    req.ContractNo,
		SignedAt:      req.SignedAt,
		Status:        req.Status,
	}

	if s.db != nil {
		err := s.db.Transaction(func(tx *gorm.DB) error {
			var party PartyRecord
			if err := tx.First(&party, "id = ?", c.Param("id")).Error; err != nil {
				return err
			}
			if !canAccessDepartment(c, party.OwnerDepartment) {
				return gorm.ErrInvalidData
			}
			record := CooperationHistoryRecord{
				ID:            item.ID,
				PartyID:       item.PartyID,
				ContractID:    item.ContractID,
				ContractTitle: item.ContractTitle,
				ContractNo:    item.ContractNo,
				SignedAt:      item.SignedAt,
				Status:        item.Status,
			}
			if err := tx.Create(&record).Error; err != nil {
				return err
			}
			party.CooperationCount++
			lastDate := item.SignedAt
			party.LastContractDate = &lastDate
			return tx.Save(&party).Error
		})
		s.mu.Unlock()
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				httpx.Error(c, http.StatusNotFound, "party not found")
				return
			}
			if err == gorm.ErrInvalidData {
				httpx.Error(c, http.StatusForbidden, "department data scope denied")
				return
			}
			httpx.Error(c, http.StatusInternalServerError, "failed to add cooperation history")
			return
		}
	} else {
		party, ok := s.parties[c.Param("id")]
		if !ok {
			s.mu.Unlock()
			httpx.Error(c, http.StatusNotFound, "party not found")
			return
		}
		if !canAccessDepartment(c, party.OwnerDepartment) {
			s.mu.Unlock()
			httpx.Error(c, http.StatusForbidden, "department data scope denied")
			return
		}
		s.history[item.PartyID] = append(s.history[item.PartyID], item)
		party.CooperationCount++
		party.LastContractDate = item.SignedAt
		s.parties[party.ID] = party
		s.mu.Unlock()
	}

	httpx.Created(c, item)
}

func (s *Service) cooperationSummary(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "party.read") {
		return
	}
	if s.db != nil {
		var party PartyRecord
		if err := s.db.First(&party, "id = ?", c.Param("id")).Error; err != nil {
			httpx.Error(c, http.StatusNotFound, "party not found")
			return
		}
		if !canAccessDepartment(c, party.OwnerDepartment) {
			httpx.Error(c, http.StatusForbidden, "department data scope denied")
			return
		}
		var rows []CooperationHistoryRecord
		if err := s.db.Where("party_id = ?", c.Param("id")).Order("signed_at desc").Find(&rows).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to load cooperation history")
			return
		}
		history := make([]CooperationHistory, 0, len(rows))
		for _, row := range rows {
			history = append(history, CooperationHistory{
				ID:            row.ID,
				PartyID:       row.PartyID,
				ContractID:    row.ContractID,
				ContractTitle: row.ContractTitle,
				ContractNo:    row.ContractNo,
				SignedAt:      row.SignedAt,
				Status:        row.Status,
			})
		}
		httpx.Success(c, gin.H{
			"party_id":           party.ID,
			"cooperation_count":  party.CooperationCount,
			"last_contract_date": party.LastContractDate,
			"history":            history,
		})
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	party, ok := s.parties[c.Param("id")]
	if !ok {
		httpx.Error(c, http.StatusNotFound, "party not found")
		return
	}
	if !canAccessDepartment(c, party.OwnerDepartment) {
		httpx.Error(c, http.StatusForbidden, "department data scope denied")
		return
	}
	httpx.Success(c, gin.H{
		"party_id":           party.ID,
		"cooperation_count":  party.CooperationCount,
		"last_contract_date": party.LastContractDate,
		"history":            s.history[party.ID],
	})
}

func (s *Service) creditCheck(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "party.read") {
		return
	}
	partyItem, snapshot, exists, err := s.loadPartyAndLatestSnapshot(c.Param("id"))
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "failed to load party credit status")
		return
	}
	if !exists {
		httpx.Error(c, http.StatusNotFound, "party not found")
		return
	}

	blocked, reasons := evaluateCreditAccess(partyItem, snapshot)
	httpx.Success(c, gin.H{
		"party_id":      partyItem.ID,
		"allowed":       !blocked,
		"blocked":       blocked,
		"reasons":       reasons,
		"party_status":  partyItem.Status,
		"credit_rating": partyItem.CreditRating,
		"risk_flag":     snapshot.RiskFlag,
	})
}

func convertPartyRecord(row PartyRecord) Party {
	item := Party{
		ID:                row.ID,
		Name:              row.Name,
		UnifiedSocialCode: row.UnifiedSocialCode,
		OwnerDepartment:   row.OwnerDepartment,
		ContactName:       row.ContactName,
		ContactPhone:      row.ContactPhone,
		CreditRating:      row.CreditRating,
		CreditSource:      row.CreditSource,
		CreditUpdatedAt:   row.CreditUpdatedAt,
		CooperationCount:  row.CooperationCount,
		Status:            row.Status,
		CreatedAt:         row.CreatedAt,
	}
	if row.LastContractDate != nil {
		item.LastContractDate = *row.LastContractDate
	}
	return item
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

func (s *Service) loadPartyAndLatestSnapshot(partyID string) (Party, CreditSnapshot, bool, error) {
	if s.db != nil {
		var row PartyRecord
		if err := s.db.First(&row, "id = ?", partyID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return Party{}, CreditSnapshot{}, false, nil
			}
			return Party{}, CreditSnapshot{}, false, err
		}
		var snapshotRow CreditSnapshotRecord
		err := s.db.Where("party_id = ?", partyID).Order("created_at desc").First(&snapshotRow).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return Party{}, CreditSnapshot{}, false, err
		}
		snapshot := CreditSnapshot{}
		if err != gorm.ErrRecordNotFound {
			snapshot = CreditSnapshot{
				ID:          snapshotRow.ID,
				PartyID:     snapshotRow.PartyID,
				Rating:      snapshotRow.Rating,
				Source:      snapshotRow.Source,
				RiskFlag:    snapshotRow.RiskFlag,
				Description: snapshotRow.Description,
				CreatedAt:   snapshotRow.CreatedAt,
			}
		}
		return convertPartyRecord(row), snapshot, true, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	partyItem, ok := s.parties[partyID]
	if !ok {
		return Party{}, CreditSnapshot{}, false, nil
	}
	var snapshot CreditSnapshot
	items := s.snapshots[partyID]
	if len(items) > 0 {
		snapshot = items[len(items)-1]
	}
	return partyItem, snapshot, true, nil
}

func evaluateCreditAccess(partyItem Party, snapshot CreditSnapshot) (bool, []string) {
	reasons := make([]string, 0)
	if partyItem.Status != "active" {
		reasons = append(reasons, "party status is not active")
	}
	rating := strings.ToUpper(strings.TrimSpace(snapshot.Rating))
	if rating == "" {
		rating = strings.ToUpper(strings.TrimSpace(partyItem.CreditRating))
	}
	switch rating {
	case "D", "E", "BLACKLIST":
		reasons = append(reasons, "credit rating is restricted")
	}
	flag := strings.ToLower(strings.TrimSpace(snapshot.RiskFlag))
	switch flag {
	case "high", "blocked", "blacklist", "frozen":
		reasons = append(reasons, "credit risk flag blocks contract intake")
	}
	return len(reasons) > 0, reasons
}
