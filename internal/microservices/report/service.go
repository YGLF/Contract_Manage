package report

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"contract-manage/pkg/microplatform/auditclient"
	"contract-manage/pkg/microplatform/httpx"
	"contract-manage/pkg/microplatform/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type MetricSnapshot struct {
	ID              string    `json:"id"`
	MetricName      string    `json:"metric_name"`
	MetricValue     int64     `json:"metric_value"`
	Dimension       string    `json:"dimension"`
	SourceEventType string    `json:"source_event_type"`
	CreatedAt       time.Time `json:"created_at"`
}

type MetricSnapshotRecord struct {
	ID              string `gorm:"primaryKey;size:128"`
	MetricName      string `gorm:"size:128;index;not null"`
	MetricValue     int64  `gorm:"not null"`
	Dimension       string `gorm:"size:128;index"`
	SourceEventType string `gorm:"size:128;index"`
	CreatedAt       time.Time
}

type ContractSummary struct {
	ID                   string    `json:"id"`
	ContractNo           string    `json:"contract_no"`
	Title                string    `json:"title"`
	Status               string    `json:"status"`
	LatestAmendmentID    string    `json:"latest_amendment_id,omitempty"`
	LatestAmendmentTitle string    `json:"latest_amendment_title,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
}

type ApprovalSummary struct {
	ID          string    `json:"id"`
	ContractID  string    `json:"contract_id"`
	RequestType string    `json:"request_type"`
	Status      string    `json:"status"`
	RequestedBy string    `json:"requested_by"`
	CreatedAt   time.Time `json:"created_at"`
}

type RiskSummary struct {
	ID          string    `json:"id"`
	ContractID  string    `json:"contract_id"`
	RuleCode    string    `json:"rule_code"`
	Severity    string    `json:"severity"`
	Status      string    `json:"status"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type ArchiveSummary struct {
	ID           string    `json:"id"`
	ContractID   string    `json:"contract_id"`
	Status       string    `json:"status"`
	ArchiveType  string    `json:"archive_type"`
	BorrowStatus string    `json:"borrow_status"`
	DestroyState string    `json:"destroy_state"`
	CreatedAt    time.Time `json:"created_at"`
}

type ClosureSummary struct {
	ID            string    `json:"id"`
	ContractID    string    `json:"contract_id"`
	RequestType   string    `json:"request_type"`
	Reason        string    `json:"reason"`
	Status        string    `json:"status"`
	RequestedBy   string    `json:"requested_by"`
	RiskChecked   bool      `json:"risk_checked"`
	PerformanceOK bool      `json:"performance_ok"`
	EvidenceReady bool      `json:"evidence_ready"`
	CreatedAt     time.Time `json:"created_at"`
}

type Service struct {
	db                 *gorm.DB
	contractServiceURL string
	approvalServiceURL string
	riskServiceURL     string
	archiveServiceURL  string
	closureServiceURL  string
	audit              *auditclient.Client
	httpClient         *http.Client
}

func New() *Service {
	return &Service{
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

func NewWithDB(db *gorm.DB) *Service {
	service := New()
	service.db = db
	if db != nil {
		_ = db.AutoMigrate(&MetricSnapshotRecord{})
	}
	return service
}

func (s *Service) SetServiceURLs(contractURL, approvalURL, riskURL, archiveURL, closureURL string) {
	s.contractServiceURL = strings.TrimRight(strings.TrimSpace(contractURL), "/")
	s.approvalServiceURL = strings.TrimRight(strings.TrimSpace(approvalURL), "/")
	s.riskServiceURL = strings.TrimRight(strings.TrimSpace(riskURL), "/")
	s.archiveServiceURL = strings.TrimRight(strings.TrimSpace(archiveURL), "/")
	s.closureServiceURL = strings.TrimRight(strings.TrimSpace(closureURL), "/")
}

func (s *Service) SetAuditClient(client *auditclient.Client) {
	s.audit = client
}

func (s *Service) RegisterRoutes(router gin.IRouter) {
	router.GET("/reports/metrics", s.list)
	router.POST("/reports/metrics", s.upsert)
	router.GET("/reports/summary", s.summary)
	router.GET("/reports/dashboard", s.dashboard)
	router.GET("/reports/workbench", s.workbench)
	router.GET("/reports/export", s.export)
}

func (s *Service) list(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "report.read") {
		return
	}
	if s.db == nil {
		httpx.Success(c, []MetricSnapshot{})
		return
	}

	var rows []MetricSnapshotRecord
	if err := s.db.Order("created_at desc").Find(&rows).Error; err != nil {
		httpx.Error(c, http.StatusInternalServerError, "failed to list report metrics")
		return
	}

	result := make([]MetricSnapshot, 0, len(rows))
	for _, row := range rows {
		result = append(result, MetricSnapshot{
			ID:              row.ID,
			MetricName:      row.MetricName,
			MetricValue:     row.MetricValue,
			Dimension:       row.Dimension,
			SourceEventType: row.SourceEventType,
			CreatedAt:       row.CreatedAt,
		})
	}
	httpx.Success(c, result)
}

func (s *Service) upsert(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "report.export") {
		return
	}
	if s.db == nil {
		s.recordAudit(c, "report.metric.upsert_failed", map[string]interface{}{
			"reason": "report database mode is disabled",
		})
		httpx.Error(c, http.StatusNotImplemented, "report database mode is disabled")
		return
	}

	var req struct {
		ID              string `json:"id"`
		MetricName      string `json:"metric_name"`
		MetricValue     int64  `json:"metric_value"`
		Dimension       string `json:"dimension"`
		SourceEventType string `json:"source_event_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		s.recordAudit(c, "report.metric.upsert_failed", map[string]interface{}{
			"reason": "invalid report payload",
		})
		httpx.Error(c, http.StatusBadRequest, "invalid report payload")
		return
	}

	row := MetricSnapshotRecord{
		ID:              req.ID,
		MetricName:      req.MetricName,
		MetricValue:     req.MetricValue,
		Dimension:       req.Dimension,
		SourceEventType: req.SourceEventType,
		CreatedAt:       time.Now(),
	}
	if err := s.db.Save(&row).Error; err != nil {
		s.recordAudit(c, "report.metric.upsert_failed", map[string]interface{}{
			"reason":      "failed to save report metric",
			"metric_name": req.MetricName,
			"dimension":   req.Dimension,
		})
		httpx.Error(c, http.StatusInternalServerError, "failed to save report metric")
		return
	}
	s.recordAudit(c, "report.metric.upserted", map[string]interface{}{
		"id":                row.ID,
		"metric_name":       row.MetricName,
		"metric_value":      row.MetricValue,
		"dimension":         row.Dimension,
		"source_event_type": row.SourceEventType,
	})
	httpx.Success(c, row)
}

func (s *Service) summary(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "report.read") {
		return
	}
	if s.db == nil {
		httpx.Success(c, gin.H{
			"totals": []MetricSnapshot{},
		})
		return
	}

	var rows []MetricSnapshotRecord
	if err := s.db.Order("metric_name asc, created_at desc").Find(&rows).Error; err != nil {
		httpx.Error(c, http.StatusInternalServerError, "failed to build report summary")
		return
	}

	type aggregate struct {
		MetricName string `json:"metric_name"`
		Count      int64  `json:"count"`
	}

	acc := make(map[string]int64)
	for _, row := range rows {
		acc[row.MetricName] += row.MetricValue
	}

	result := make([]aggregate, 0, len(acc))
	for name, count := range acc {
		result = append(result, aggregate{
			MetricName: name,
			Count:      count,
		})
	}

	httpx.Success(c, gin.H{
		"totals": result,
	})
}

func (s *Service) dashboard(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "report.read") {
		return
	}
	payload, err := s.buildDashboardPayload(c)
	if err != nil {
		httpx.Error(c, http.StatusBadGateway, err.Error())
		return
	}
	httpx.Success(c, payload)
}

func (s *Service) workbench(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "report.read") {
		return
	}
	payload, err := s.buildWorkbenchPayload(c)
	if err != nil {
		httpx.Error(c, http.StatusBadGateway, err.Error())
		return
	}
	httpx.Success(c, payload)
}

func (s *Service) export(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "report.export") {
		return
	}
	approvalID := strings.TrimSpace(c.Query("approval_request_id"))
	if approvalID == "" {
		s.recordAudit(c, "report.export_failed", map[string]interface{}{
			"reason": "approval_request_id is required",
		})
		httpx.Error(c, http.StatusBadRequest, "approval_request_id is required")
		return
	}

	view := strings.TrimSpace(c.DefaultQuery("view", "dashboard"))
	if err := s.validateExportApproval(c, approvalID, view); err != nil {
		s.recordAudit(c, "report.export_failed", map[string]interface{}{
			"view":                view,
			"approval_request_id": approvalID,
			"reason":              err.Error(),
		})
		httpx.Error(c, http.StatusForbidden, err.Error())
		return
	}
	var (
		payload interface{}
		err     error
	)
	switch view {
	case "dashboard":
		payload, err = s.buildDashboardPayload(c)
	case "workbench":
		payload, err = s.buildWorkbenchPayload(c)
	case "summary":
		payload, err = s.buildSummaryPayload()
	default:
		s.recordAudit(c, "report.export_failed", map[string]interface{}{
			"view":   view,
			"reason": "unsupported export view",
		})
		httpx.Error(c, http.StatusBadRequest, "unsupported export view")
		return
	}
	if err != nil {
		s.recordAudit(c, "report.export_failed", map[string]interface{}{
			"view":   view,
			"reason": err.Error(),
		})
		httpx.Error(c, http.StatusBadGateway, err.Error())
		return
	}
	if err := s.consumeExportApproval(c, approvalID); err != nil {
		s.recordAudit(c, "report.export_failed", map[string]interface{}{
			"view":                view,
			"approval_request_id": approvalID,
			"reason":              err.Error(),
		})
		httpx.Error(c, http.StatusConflict, err.Error())
		return
	}

	data, err := json.Marshal(payload)
	if err != nil {
		s.recordAudit(c, "report.export_failed", map[string]interface{}{
			"view":   view,
			"reason": "failed to marshal export payload",
		})
		httpx.Error(c, http.StatusInternalServerError, "failed to build export payload")
		return
	}

	filename := fmt.Sprintf("report-%s-%s.json", view, time.Now().Format("20060102150405"))
	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	s.recordAudit(c, "report.exported", map[string]interface{}{
		"view":                view,
		"filename":            filename,
		"size":                len(data),
		"approval_request_id": approvalID,
	})
	c.Data(http.StatusOK, "application/json", data)
}

func (s *Service) buildDashboardPayload(c *gin.Context) (gin.H, error) {
	contracts, approvals, risks, archives, closures, err := s.loadAggregates(c)
	if err != nil {
		return nil, err
	}

	statusBreakdown := make(map[string]int)
	for _, item := range contracts {
		statusBreakdown[item.Status]++
	}

	openRiskCount := 0
	highRiskCount := 0
	for _, item := range risks {
		if item.Status == "open" {
			openRiskCount++
		}
		if strings.EqualFold(item.Severity, "high") && item.Status == "open" {
			highRiskCount++
		}
	}

	pendingApprovals := 0
	for _, item := range approvals {
		if item.Status == "pending" {
			pendingApprovals++
		}
	}

	archivedCount := 0
	pendingDestroyCount := 0
	borrowedCount := 0
	for _, item := range archives {
		if item.Status == "archived" {
			archivedCount++
		}
		if item.DestroyState == "requested" {
			pendingDestroyCount++
		}
		if item.BorrowStatus == "borrowed" {
			borrowedCount++
		}
	}

	pendingClosures := 0
	completedClosures := 0
	for _, item := range closures {
		if item.Status == "completed" {
			completedClosures++
			continue
		}
		pendingClosures++
	}

	return gin.H{
		"overview": gin.H{
			"contract_total":           len(contracts),
			"pending_approvals":        pendingApprovals,
			"open_risks":               openRiskCount,
			"high_risks":               highRiskCount,
			"archived_contracts":       archivedCount,
			"borrowed_archives":        borrowedCount,
			"pending_destroy_archives": pendingDestroyCount,
			"pending_closures":         pendingClosures,
			"completed_closures":       completedClosures,
		},
		"contract_status_breakdown": statusBreakdown,
	}, nil
}

func (s *Service) buildWorkbenchPayload(c *gin.Context) (gin.H, error) {
	contracts, approvals, risks, archives, closures, err := s.loadAggregates(c)
	if err != nil {
		return nil, err
	}

	pendingApprovals := make([]ApprovalSummary, 0)
	for _, item := range approvals {
		if item.Status == "pending" {
			pendingApprovals = append(pendingApprovals, item)
		}
	}
	sort.Slice(pendingApprovals, func(i, j int) bool {
		return pendingApprovals[i].CreatedAt.After(pendingApprovals[j].CreatedAt)
	})

	openRisks := make([]RiskSummary, 0)
	for _, item := range risks {
		if item.Status == "open" {
			openRisks = append(openRisks, item)
		}
	}
	sort.Slice(openRisks, func(i, j int) bool {
		return openRisks[i].CreatedAt.After(openRisks[j].CreatedAt)
	})

	recentContracts := make([]ContractSummary, len(contracts))
	copy(recentContracts, contracts)
	sort.Slice(recentContracts, func(i, j int) bool {
		return recentContracts[i].CreatedAt.After(recentContracts[j].CreatedAt)
	})
	if len(recentContracts) > 5 {
		recentContracts = recentContracts[:5]
	}

	attentionArchives := make([]ArchiveSummary, 0)
	for _, item := range archives {
		if item.BorrowStatus == "borrowed" || item.DestroyState == "requested" {
			attentionArchives = append(attentionArchives, item)
		}
	}

	pendingClosures := make([]ClosureSummary, 0)
	for _, item := range closures {
		if item.Status != "completed" {
			pendingClosures = append(pendingClosures, item)
		}
	}
	sort.Slice(pendingClosures, func(i, j int) bool {
		return pendingClosures[i].CreatedAt.After(pendingClosures[j].CreatedAt)
	})

	return gin.H{
		"pending_approvals":  pendingApprovals,
		"open_risks":         openRisks,
		"recent_contracts":   recentContracts,
		"attention_archives": attentionArchives,
		"pending_closures":   pendingClosures,
	}, nil
}

func (s *Service) buildSummaryPayload() (gin.H, error) {
	if s.db == nil {
		return gin.H{"totals": []MetricSnapshot{}}, nil
	}

	var rows []MetricSnapshotRecord
	if err := s.db.Order("metric_name asc, created_at desc").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to build report summary")
	}

	type aggregate struct {
		MetricName string `json:"metric_name"`
		Count      int64  `json:"count"`
	}

	acc := make(map[string]int64)
	for _, row := range rows {
		acc[row.MetricName] += row.MetricValue
	}

	result := make([]aggregate, 0, len(acc))
	for name, count := range acc {
		result = append(result, aggregate{
			MetricName: name,
			Count:      count,
		})
	}
	return gin.H{"totals": result}, nil
}

func (s *Service) validateExportApproval(c *gin.Context, approvalID, view string) error {
	if strings.TrimSpace(s.approvalServiceURL) == "" {
		return fmt.Errorf("approval service url is not configured")
	}
	req, err := http.NewRequest(http.MethodGet, s.approvalServiceURL+"/api/v1/approval-requests/"+approvalID, nil)
	if err != nil {
		return fmt.Errorf("failed to build approval validation request")
	}
	for key, value := range buildForwardHeaders(c) {
		if strings.TrimSpace(value) != "" {
			req.Header.Set(key, value)
		}
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to validate export approval")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("export approval is not available")
	}

	var result struct {
		Success bool `json:"success"`
		Data    struct {
			ID          string                 `json:"id"`
			RequestType string                 `json:"request_type"`
			Status      string                 `json:"status"`
			RequestedBy string                 `json:"requested_by"`
			Payload     map[string]interface{} `json:"payload"`
			ExpiresAt   string                 `json:"expires_at"`
			ConsumedAt  string                 `json:"consumed_at"`
		} `json:"data"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode export approval")
	}
	if !result.Success {
		return fmt.Errorf("export approval is invalid")
	}
	if result.Data.RequestType != "report_export" {
		return fmt.Errorf("approval request type is not report_export")
	}
	if result.Data.Status != "approved" {
		return fmt.Errorf("export approval has not been approved")
	}
	if strings.TrimSpace(result.Data.ConsumedAt) != "" {
		consumedAt, err := time.Parse(time.RFC3339, result.Data.ConsumedAt)
		if err == nil && !consumedAt.IsZero() && consumedAt.Year() > 1 {
			return fmt.Errorf("export approval has already been consumed")
		}
	}
	if strings.TrimSpace(result.Data.ExpiresAt) != "" {
		expiresAt, err := time.Parse(time.RFC3339, result.Data.ExpiresAt)
		if err == nil && !expiresAt.IsZero() && expiresAt.Year() > 1 && expiresAt.Before(time.Now()) {
			return fmt.Errorf("export approval has expired")
		}
	}
	approvedView, _ := result.Data.Payload["view"].(string)
	if strings.TrimSpace(approvedView) != "" && approvedView != view {
		return fmt.Errorf("export approval scope does not match requested view")
	}
	return nil
}

func (s *Service) consumeExportApproval(c *gin.Context, approvalID string) error {
	if strings.TrimSpace(s.approvalServiceURL) == "" {
		return fmt.Errorf("approval service url is not configured")
	}
	req, err := http.NewRequest(http.MethodPost, s.approvalServiceURL+"/api/v1/approval-requests/"+approvalID+"/consume", strings.NewReader("{}"))
	if err != nil {
		return fmt.Errorf("failed to build approval consume request")
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range buildForwardHeaders(c) {
		if strings.TrimSpace(value) != "" {
			req.Header.Set(key, value)
		}
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to consume export approval")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var result struct {
			Success bool   `json:"success"`
			Error   string `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err == nil && strings.TrimSpace(result.Error) != "" {
			return fmt.Errorf(result.Error)
		}
		return fmt.Errorf("failed to consume export approval")
	}
	return nil
}

func (s *Service) loadAggregates(c *gin.Context) ([]ContractSummary, []ApprovalSummary, []RiskSummary, []ArchiveSummary, []ClosureSummary, error) {
	headers := buildForwardHeaders(c)
	contracts, err := fetchList[ContractSummary](s.httpClient, s.contractServiceURL+"/api/v1/contracts", headers)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to load contracts")
	}
	approvals, err := fetchList[ApprovalSummary](s.httpClient, s.approvalServiceURL+"/api/v1/approval-requests", headers)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to load approval requests")
	}
	risks, err := fetchList[RiskSummary](s.httpClient, s.riskServiceURL+"/api/v1/risk/events", headers)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to load risk events")
	}
	archives, err := fetchList[ArchiveSummary](s.httpClient, s.archiveServiceURL+"/api/v1/archive/cases", headers)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to load archive cases")
	}
	closures, err := fetchList[ClosureSummary](s.httpClient, s.closureServiceURL+"/api/v1/closure/requests", headers)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to load closure requests")
	}
	return contracts, approvals, risks, archives, closures, nil
}

type envelope[T any] struct {
	Success bool   `json:"success"`
	Data    []T    `json:"data"`
	Error   string `json:"error"`
}

func fetchList[T any](client *http.Client, endpoint string, headers map[string]string) ([]T, error) {
	if strings.TrimSpace(endpoint) == "" || strings.HasPrefix(endpoint, "/api/") {
		return []T{}, nil
	}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	for key, value := range headers {
		if strings.TrimSpace(value) != "" {
			req.Header.Set(key, value)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	var result envelope[T]
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, fmt.Errorf(result.Error)
	}
	return result.Data, nil
}

func buildForwardHeaders(c *gin.Context) map[string]string {
	headers := map[string]string{}
	if traceID := c.GetHeader("X-Trace-Id"); traceID != "" {
		headers["X-Trace-Id"] = traceID
	}
	if identity, ok := middleware.IdentityFromContextOrHeaders(c); ok {
		headers["X-User-Id"] = identity.UserID
		headers["X-User-Department"] = identity.Department
		headers["X-Data-Scope"] = identity.DataScope
		headers["X-User-Roles"] = strings.Join(identity.Roles, ",")
		headers["X-User-Permissions"] = strings.Join(identity.Permissions, ",")
	}
	return headers
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
		fmt.Sprintf("audit-report-%d", time.Now().UnixNano()),
		action,
		middleware.CurrentOperatorID(c, "system"),
		trace,
		payload,
	)
}
