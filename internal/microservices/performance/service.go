package performance

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

type Plan struct {
	ID         string    `json:"id"`
	ContractID string    `json:"contract_id"`
	Department string    `json:"department,omitempty"`
	Version    int       `json:"version"`
	NodeName   string    `json:"node_name"`
	NodeType   string    `json:"node_type"`
	DueDate    time.Time `json:"due_date"`
	CreatedAt  time.Time `json:"created_at"`
}

type PlanRecord struct {
	ID         string    `gorm:"primaryKey;size:64"`
	ContractID string    `gorm:"size:64;index;not null"`
	Department string    `gorm:"size:64;index"`
	Version    int       `gorm:"not null"`
	NodeName   string    `gorm:"size:255;not null"`
	NodeType   string    `gorm:"size:64;not null"`
	DueDate    time.Time `gorm:"not null"`
	CreatedAt  time.Time
}

type ExecutionRecord struct {
	ID         string    `json:"id"`
	ContractID string    `json:"contract_id"`
	Department string    `json:"department,omitempty"`
	PlanID     string    `json:"plan_id"`
	ActualAt   time.Time `json:"actual_at"`
	Result     string    `json:"result"`
	Remark     string    `json:"remark"`
	OperatorID string    `json:"operator_id"`
	CreatedAt  time.Time `json:"created_at"`
}

type ExecutionDBRecord struct {
	ID         string    `gorm:"primaryKey;size:64"`
	ContractID string    `gorm:"size:64;index;not null"`
	Department string    `gorm:"size:64;index"`
	PlanID     string    `gorm:"size:64;index;not null"`
	ActualAt   time.Time `gorm:"not null"`
	Result     string    `gorm:"size:64;not null"`
	Remark     string    `gorm:"type:text"`
	OperatorID string    `gorm:"size:64;index"`
	CreatedAt  time.Time
}

type Service struct {
	mu             sync.RWMutex
	plans          map[string][]Plan
	executions     map[string][]ExecutionRecord
	seq            int
	db             *gorm.DB
	riskServiceURL string
}

type VersionBatch struct {
	ContractID string    `json:"contract_id"`
	Version    int       `json:"version"`
	Plans      []Plan    `json:"plans"`
	CreatedAt  time.Time `json:"created_at"`
}

type Summary struct {
	ContractID      string `json:"contract_id"`
	LatestVersion   int    `json:"latest_version"`
	PlanCount       int    `json:"plan_count"`
	CompletedCount  int    `json:"completed_count"`
	InProgressCount int    `json:"in_progress_count"`
	DelayedCount    int    `json:"delayed_count"`
	ExceptionCount  int    `json:"exception_count"`
	PerformanceOK   bool   `json:"performance_ok"`
}

func New() *Service {
	return &Service{
		plans:      make(map[string][]Plan),
		executions: make(map[string][]ExecutionRecord),
	}
}

func NewWithDB(db *gorm.DB) *Service {
	service := New()
	service.db = db
	if db != nil {
		_ = db.AutoMigrate(&PlanRecord{})
		_ = db.AutoMigrate(&ExecutionDBRecord{})
	}
	return service
}

func (s *Service) SetRiskServiceURL(rawURL string) {
	s.riskServiceURL = strings.TrimRight(strings.TrimSpace(rawURL), "/")
}

func (s *Service) RegisterRoutes(router gin.IRouter) {
	router.GET("/contracts/:id/plans", s.list)
	router.POST("/contracts/:id/plans", s.create)
	router.GET("/contracts/:id/plan-versions/latest", s.latestVersion)
	router.POST("/contracts/:id/plan-versions", s.createVersion)
	router.GET("/contracts/:id/executions", s.listExecutions)
	router.POST("/contracts/:id/executions", s.createExecution)
	router.GET("/contracts/:id/performance-summary", s.performanceSummary)
}

func (s *Service) list(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "performance.read") {
		return
	}
	if s.db != nil {
		var rows []PlanRecord
		query := s.db.Where("contract_id = ?", c.Param("id"))
		if department, limited := scopedDepartment(c); limited {
			query = query.Where("department = ?", department)
		}
		if err := query.Order("version asc, created_at asc").Find(&rows).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to list plans")
			return
		}
		result := make([]Plan, 0, len(rows))
		for _, row := range rows {
			result = append(result, Plan{
				ID:         row.ID,
				ContractID: row.ContractID,
				Department: row.Department,
				Version:    row.Version,
				NodeName:   row.NodeName,
				NodeType:   row.NodeType,
				DueDate:    row.DueDate,
				CreatedAt:  row.CreatedAt,
			})
		}
		httpx.Success(c, result)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Plan, 0, len(s.plans[c.Param("id")]))
	for _, plan := range s.plans[c.Param("id")] {
		if !canAccessDepartment(c, plan.Department) {
			continue
		}
		result = append(result, plan)
	}
	httpx.Success(c, result)
}

func (s *Service) create(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "performance.write") {
		return
	}
	var req struct {
		NodeName string    `json:"node_name"`
		NodeType string    `json:"node_type"`
		DueDate  time.Time `json:"due_date"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid plan payload")
		return
	}

	s.mu.Lock()
	s.seq++
	contractID := c.Param("id")
	version := len(s.plans[contractID]) + 1
	s.mu.Unlock()

	if s.db != nil {
		var count int64
		if err := s.db.Model(&PlanRecord{}).Where("contract_id = ?", contractID).Count(&count).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to calculate plan version")
			return
		}
		version = int(count) + 1
	}

	plan := Plan{
		ID:         fmt.Sprintf("plan-%04d", s.seq),
		ContractID: contractID,
		Department: operatorDepartment(c),
		Version:    version,
		NodeName:   req.NodeName,
		NodeType:   req.NodeType,
		DueDate:    req.DueDate,
		CreatedAt:  time.Now(),
	}

	if s.db != nil {
		record := PlanRecord{
			ID:         plan.ID,
			ContractID: plan.ContractID,
			Department: plan.Department,
			Version:    plan.Version,
			NodeName:   plan.NodeName,
			NodeType:   plan.NodeType,
			DueDate:    plan.DueDate,
			CreatedAt:  plan.CreatedAt,
		}
		if err := s.db.Create(&record).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to create plan")
			return
		}
	} else {
		s.mu.Lock()
		s.plans[contractID] = append(s.plans[contractID], plan)
		s.mu.Unlock()
	}

	httpx.Created(c, plan)
}

func (s *Service) latestVersion(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "performance.read") {
		return
	}
	contractID := c.Param("id")

	if s.db != nil {
		var latest PlanRecord
		if err := s.db.Where("contract_id = ?", contractID).Order("version desc, created_at desc").First(&latest).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				httpx.Success(c, VersionBatch{ContractID: contractID, Version: 0, Plans: []Plan{}})
				return
			}
			httpx.Error(c, http.StatusInternalServerError, "failed to load latest plan version")
			return
		}

		if !canAccessDepartment(c, latest.Department) {
			httpx.Error(c, http.StatusForbidden, "department data scope denied")
			return
		}

		var rows []PlanRecord
		if err := s.db.Where("contract_id = ? AND version = ?", contractID, latest.Version).Order("created_at asc").Find(&rows).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to load latest version plans")
			return
		}

		result := VersionBatch{
			ContractID: contractID,
			Version:    latest.Version,
			Plans:      make([]Plan, 0, len(rows)),
			CreatedAt:  latest.CreatedAt,
		}
		for _, row := range rows {
			result.Plans = append(result.Plans, Plan{
				ID:         row.ID,
				ContractID: row.ContractID,
				Department: row.Department,
				Version:    row.Version,
				NodeName:   row.NodeName,
				NodeType:   row.NodeType,
				DueDate:    row.DueDate,
				CreatedAt:  row.CreatedAt,
			})
		}
		httpx.Success(c, result)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	plans := s.plans[contractID]
	if len(plans) == 0 {
		httpx.Success(c, VersionBatch{ContractID: contractID, Version: 0, Plans: []Plan{}})
		return
	}
	if !canAccessDepartment(c, plans[0].Department) {
		httpx.Error(c, http.StatusForbidden, "department data scope denied")
		return
	}
	latestVersion := plans[len(plans)-1].Version
	result := VersionBatch{
		ContractID: contractID,
		Version:    latestVersion,
		Plans:      make([]Plan, 0),
	}
	for _, plan := range plans {
		if plan.Version == latestVersion {
			if result.CreatedAt.IsZero() {
				result.CreatedAt = plan.CreatedAt
			}
			result.Plans = append(result.Plans, plan)
		}
	}
	httpx.Success(c, result)
}

func (s *Service) createVersion(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "performance.write") {
		return
	}
	var req struct {
		Nodes []struct {
			NodeName string    `json:"node_name"`
			NodeType string    `json:"node_type"`
			DueDate  time.Time `json:"due_date"`
		} `json:"nodes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid version payload")
		return
	}
	if len(req.Nodes) == 0 {
		httpx.Error(c, http.StatusBadRequest, "at least one node is required")
		return
	}

	contractID := c.Param("id")
	nextVersion := 1

	if s.db != nil {
		var latest PlanRecord
		if err := s.db.Where("contract_id = ?", contractID).Order("version desc").First(&latest).Error; err == nil {
			nextVersion = latest.Version + 1
		} else if err != gorm.ErrRecordNotFound {
			httpx.Error(c, http.StatusInternalServerError, "failed to calculate next version")
			return
		}

		now := time.Now()
		result := VersionBatch{
			ContractID: contractID,
			Version:    nextVersion,
			Plans:      make([]Plan, 0, len(req.Nodes)),
			CreatedAt:  now,
		}
		for _, node := range req.Nodes {
			s.seq++
			plan := Plan{
				ID:         fmt.Sprintf("plan-%04d", s.seq),
				ContractID: contractID,
				Department: operatorDepartment(c),
				Version:    nextVersion,
				NodeName:   node.NodeName,
				NodeType:   node.NodeType,
				DueDate:    node.DueDate,
				CreatedAt:  now,
			}
			result.Plans = append(result.Plans, plan)
		}

		err := s.db.Transaction(func(tx *gorm.DB) error {
			for _, plan := range result.Plans {
				record := PlanRecord{
					ID:         plan.ID,
					ContractID: plan.ContractID,
					Department: plan.Department,
					Version:    plan.Version,
					NodeName:   plan.NodeName,
					NodeType:   plan.NodeType,
					DueDate:    plan.DueDate,
					CreatedAt:  plan.CreatedAt,
				}
				if err := tx.Create(&record).Error; err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to create plan version")
			return
		}
		httpx.Created(c, result)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.plans[contractID] {
		if existing.Version >= nextVersion {
			nextVersion = existing.Version + 1
		}
	}
	now := time.Now()
	result := VersionBatch{
		ContractID: contractID,
		Version:    nextVersion,
		Plans:      make([]Plan, 0, len(req.Nodes)),
		CreatedAt:  now,
	}
	for _, node := range req.Nodes {
		s.seq++
		plan := Plan{
			ID:         fmt.Sprintf("plan-%04d", s.seq),
			ContractID: contractID,
			Department: operatorDepartment(c),
			Version:    nextVersion,
			NodeName:   node.NodeName,
			NodeType:   node.NodeType,
			DueDate:    node.DueDate,
			CreatedAt:  now,
		}
		s.plans[contractID] = append(s.plans[contractID], plan)
		result.Plans = append(result.Plans, plan)
	}
	httpx.Created(c, result)
}

func (s *Service) listExecutions(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "performance.read") {
		return
	}
	contractID := c.Param("id")

	if s.db != nil {
		var rows []ExecutionDBRecord
		query := s.db.Where("contract_id = ?", contractID)
		if department, limited := scopedDepartment(c); limited {
			query = query.Where("department = ?", department)
		}
		if err := query.Order("created_at desc").Find(&rows).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to list executions")
			return
		}
		result := make([]ExecutionRecord, 0, len(rows))
		for _, row := range rows {
			result = append(result, ExecutionRecord{
				ID:         row.ID,
				ContractID: row.ContractID,
				Department: row.Department,
				PlanID:     row.PlanID,
				ActualAt:   row.ActualAt,
				Result:     row.Result,
				Remark:     row.Remark,
				OperatorID: row.OperatorID,
				CreatedAt:  row.CreatedAt,
			})
		}
		httpx.Success(c, result)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]ExecutionRecord, 0, len(s.executions[contractID]))
	for _, row := range s.executions[contractID] {
		if !canAccessDepartment(c, row.Department) {
			continue
		}
		result = append(result, row)
	}
	httpx.Success(c, result)
}

func (s *Service) createExecution(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "performance.write") {
		return
	}
	var req struct {
		PlanID     string    `json:"plan_id"`
		ActualAt   time.Time `json:"actual_at"`
		Result     string    `json:"result"`
		Remark     string    `json:"remark"`
		OperatorID string    `json:"operator_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid execution payload")
		return
	}
	if req.PlanID == "" || req.Result == "" || req.ActualAt.IsZero() {
		httpx.Error(c, http.StatusBadRequest, "plan_id, result and actual_at are required")
		return
	}
	if strings.TrimSpace(req.OperatorID) == "" {
		req.OperatorID = middleware.CurrentOperatorID(c, "system")
	}
	if !isAllowedExecutionResult(req.Result) {
		httpx.Error(c, http.StatusBadRequest, "result must be one of completed, in_progress, delayed, exception")
		return
	}

	contractID := c.Param("id")
	if !s.planExists(contractID, req.PlanID) {
		httpx.Error(c, http.StatusBadRequest, "plan_id does not exist for contract")
		return
	}
	record := ExecutionRecord{
		ID:         fmt.Sprintf("exec-%04d", s.nextSeq()),
		ContractID: contractID,
		Department: operatorDepartment(c),
		PlanID:     req.PlanID,
		ActualAt:   req.ActualAt,
		Result:     req.Result,
		Remark:     req.Remark,
		OperatorID: req.OperatorID,
		CreatedAt:  time.Now(),
	}

	if s.db != nil {
		dbRecord := ExecutionDBRecord{
			ID:         record.ID,
			ContractID: record.ContractID,
			Department: record.Department,
			PlanID:     record.PlanID,
			ActualAt:   record.ActualAt,
			Result:     record.Result,
			Remark:     record.Remark,
			OperatorID: record.OperatorID,
			CreatedAt:  record.CreatedAt,
		}
		if err := s.db.Create(&dbRecord).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "failed to create execution")
			return
		}
	} else {
		s.mu.Lock()
		s.executions[contractID] = append([]ExecutionRecord{record}, s.executions[contractID]...)
		s.mu.Unlock()
	}

	_ = s.syncRiskEvent(record)

	httpx.Created(c, record)
}

func (s *Service) performanceSummary(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "performance.read") {
		return
	}
	contractID := c.Param("id")
	if s.db != nil {
		summary, err := s.buildSummaryFromDB(c, contractID)
		if err != nil {
			if err == gorm.ErrInvalidData {
				httpx.Error(c, http.StatusForbidden, "department data scope denied")
				return
			}
			httpx.Error(c, http.StatusInternalServerError, "failed to calculate performance summary")
			return
		}
		httpx.Success(c, summary)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	summary, ok := s.buildSummaryFromMemory(c, contractID)
	if !ok {
		httpx.Error(c, http.StatusForbidden, "department data scope denied")
		return
	}
	httpx.Success(c, summary)
}

func (s *Service) buildSummaryFromDB(c *gin.Context, contractID string) (Summary, error) {
	summary := Summary{ContractID: contractID}

	var latest PlanRecord
	if err := s.db.Where("contract_id = ?", contractID).Order("version desc, created_at desc").First(&latest).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return summary, nil
		}
		return Summary{}, err
	}
	if !canAccessDepartment(c, latest.Department) {
		return Summary{}, gorm.ErrInvalidData
	}

	var plans []PlanRecord
	if err := s.db.Where("contract_id = ? AND version = ? AND department = ?", contractID, latest.Version, latest.Department).Find(&plans).Error; err != nil {
		return Summary{}, err
	}

	var executions []ExecutionDBRecord
	if err := s.db.Where("contract_id = ? AND department = ?", contractID, latest.Department).Order("created_at desc").Find(&executions).Error; err != nil {
		return Summary{}, err
	}

	summary.LatestVersion = latest.Version
	summary.PlanCount = len(plans)
	computeSummaryCounts(&summary, collectLatestExecutionResults(executions), collectPlanIDs(plans))
	return summary, nil
}

func (s *Service) buildSummaryFromMemory(c *gin.Context, contractID string) (Summary, bool) {
	plans := s.plans[contractID]
	executions := s.executions[contractID]
	summary := Summary{ContractID: contractID}
	if len(plans) == 0 {
		return summary, true
	}
	if !canAccessDepartment(c, plans[0].Department) {
		return Summary{}, false
	}

	latestVersion := 0
	for _, plan := range plans {
		if plan.Version > latestVersion {
			latestVersion = plan.Version
		}
	}
	currentPlans := make([]Plan, 0)
	for _, plan := range plans {
		if plan.Version == latestVersion {
			currentPlans = append(currentPlans, plan)
		}
	}

	summary.LatestVersion = latestVersion
	summary.PlanCount = len(currentPlans)
	computeSummaryCounts(&summary, collectLatestExecutionResultsMemory(executions), collectPlanIDsMemory(currentPlans))
	return summary, true
}

func collectPlanIDs(rows []PlanRecord) []string {
	result := make([]string, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.ID)
	}
	return result
}

func collectPlanIDsMemory(rows []Plan) []string {
	result := make([]string, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.ID)
	}
	return result
}

func collectLatestExecutionResults(rows []ExecutionDBRecord) map[string]string {
	result := make(map[string]string, len(rows))
	for _, row := range rows {
		if _, ok := result[row.PlanID]; ok {
			continue
		}
		result[row.PlanID] = row.Result
	}
	return result
}

func collectLatestExecutionResultsMemory(rows []ExecutionRecord) map[string]string {
	result := make(map[string]string, len(rows))
	for _, row := range rows {
		if _, ok := result[row.PlanID]; ok {
			continue
		}
		result[row.PlanID] = row.Result
	}
	return result
}

func computeSummaryCounts(summary *Summary, executionResults map[string]string, planIDs []string) {
	completed := 0
	for _, planID := range planIDs {
		switch executionResults[planID] {
		case "completed":
			summary.CompletedCount++
			completed++
		case "in_progress":
			summary.InProgressCount++
		case "delayed":
			summary.DelayedCount++
		case "exception":
			summary.ExceptionCount++
		}
	}
	summary.PerformanceOK = summary.PlanCount > 0 && completed == summary.PlanCount && summary.DelayedCount == 0 && summary.ExceptionCount == 0
}

func (s *Service) nextSeq() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	return s.seq
}

func isAllowedExecutionResult(result string) bool {
	switch result {
	case "completed", "in_progress", "delayed", "exception":
		return true
	default:
		return false
	}
}

func (s *Service) planExists(contractID, planID string) bool {
	if s.db != nil {
		var count int64
		if err := s.db.Model(&PlanRecord{}).Where("contract_id = ? AND id = ?", contractID, planID).Count(&count).Error; err != nil {
			return false
		}
		return count > 0
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, plan := range s.plans[contractID] {
		if plan.ID == planID {
			return true
		}
	}
	return false
}

func (s *Service) syncRiskEvent(record ExecutionRecord) error {
	if s.riskServiceURL == "" {
		return nil
	}

	payload := map[string]string{
		"contract_id": record.ContractID,
		"description": record.Remark,
	}
	switch record.Result {
	case "delayed":
		payload["rule_code"] = "performance_delayed"
		payload["severity"] = "medium"
		if strings.TrimSpace(payload["description"]) == "" {
			payload["description"] = "performance node delayed"
		}
	case "exception":
		payload["rule_code"] = "performance_exception"
		payload["severity"] = "high"
		if strings.TrimSpace(payload["description"]) == "" {
			payload["description"] = "performance node exception"
		}
	default:
		return nil
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := http.Post(s.riskServiceURL+"/api/v1/risk/events", "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("risk sync failed: %s", strings.TrimSpace(string(body)))
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
