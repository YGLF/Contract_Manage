package searchai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"contract-manage/pkg/microplatform/auditclient"
	"contract-manage/pkg/microplatform/httpx"
	"contract-manage/pkg/microplatform/middleware"

	"github.com/gin-gonic/gin"
)

type Answer struct {
	Intent       string                 `json:"intent"`
	Summary      string                 `json:"summary"`
	SourceData   map[string]interface{} `json:"source_data"`
	ModelOutput  string                 `json:"model_output,omitempty"`
	NeedsConfirm bool                   `json:"needs_confirm"`
}

type Service struct {
	contractServiceURL    string
	riskServiceURL        string
	reportServiceURL      string
	partyServiceURL       string
	performanceServiceURL string
	archiveServiceURL     string
	modelEndpoint         string
	audit                 *auditclient.Client
	httpClient            *http.Client
}

func New() *Service {
	return &Service{
		httpClient: &http.Client{Timeout: 8 * time.Second},
	}
}

func (s *Service) SetDependencies(contractURL, riskURL, reportURL, partyURL, performanceURL, archiveURL, modelEndpoint string, audit *auditclient.Client) {
	s.contractServiceURL = strings.TrimRight(strings.TrimSpace(contractURL), "/")
	s.riskServiceURL = strings.TrimRight(strings.TrimSpace(riskURL), "/")
	s.reportServiceURL = strings.TrimRight(strings.TrimSpace(reportURL), "/")
	s.partyServiceURL = strings.TrimRight(strings.TrimSpace(partyURL), "/")
	s.performanceServiceURL = strings.TrimRight(strings.TrimSpace(performanceURL), "/")
	s.archiveServiceURL = strings.TrimRight(strings.TrimSpace(archiveURL), "/")
	s.modelEndpoint = strings.TrimRight(strings.TrimSpace(modelEndpoint), "/")
	s.audit = audit
}

func (s *Service) RegisterRoutes(router gin.IRouter) {
	router.POST("/search-ai/ask", s.ask)
}

func (s *Service) ask(c *gin.Context) {
	if !middleware.EnforcePermissionIfPresent(c, "search_ai.ask") {
		return
	}
	var req struct {
		Question string `json:"question"`
		UserID   string `json:"user_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		s.recordAudit(c, "search_ai.ask_failed", middleware.CurrentOperatorID(c, "system"), map[string]interface{}{
			"reason": "invalid ai ask payload",
		})
		httpx.Error(c, http.StatusBadRequest, "invalid ai ask payload")
		return
	}
	if strings.TrimSpace(req.Question) == "" {
		s.recordAudit(c, "search_ai.ask_failed", middleware.CurrentOperatorID(c, "system"), map[string]interface{}{
			"reason": "question is required",
		})
		httpx.Error(c, http.StatusBadRequest, "question is required")
		return
	}
	if strings.TrimSpace(req.UserID) == "" {
		req.UserID = middleware.CurrentOperatorID(c, "system")
	}

	intent := detectIntent(req.Question)
	answer, err := s.buildAnswer(intent, req.Question)
	if err != nil {
		s.recordAudit(c, "search_ai.ask_failed", req.UserID, map[string]interface{}{
			"question": req.Question,
			"intent":   intent,
			"reason":   err.Error(),
		})
		httpx.Error(c, http.StatusBadGateway, err.Error())
		return
	}

	s.recordAudit(c, "search_ai.ask", req.UserID, map[string]interface{}{
		"question":         req.Question,
		"intent":           answer.Intent,
		"needs_confirm":    answer.NeedsConfirm,
		"used_model":       strings.TrimSpace(s.modelEndpoint) != "",
		"source_data_keys": mapKeys(answer.SourceData),
	})

	httpx.Success(c, answer)
}

func detectIntent(question string) string {
	lowered := strings.ToLower(question)
	switch {
	case strings.Contains(lowered, "风险") || strings.Contains(lowered, "预警") || strings.Contains(lowered, "risk"):
		return "risk_overview"
	case strings.Contains(lowered, "履约") || strings.Contains(lowered, "执行") || strings.Contains(lowered, "performance"):
		return "performance_overview"
	case strings.Contains(lowered, "相对方") || strings.Contains(lowered, "信用") || strings.Contains(lowered, "party") || strings.Contains(lowered, "credit"):
		return "party_credit_overview"
	case strings.Contains(lowered, "归档") || strings.Contains(lowered, "借阅") || strings.Contains(lowered, "销毁") || strings.Contains(lowered, "archive"):
		return "archive_overview"
	case strings.Contains(lowered, "报表") || strings.Contains(lowered, "驾驶舱") || strings.Contains(lowered, "dashboard"):
		return "dashboard_summary"
	default:
		return "contract_overview"
	}
}

func (s *Service) buildAnswer(intent, question string) (Answer, error) {
	switch intent {
	case "risk_overview":
		data, err := s.fetchMap(s.riskServiceURL + "/api/v1/risk/events")
		if err != nil {
			return Answer{}, fmt.Errorf("failed to query risk service")
		}
		return Answer{
			Intent:       intent,
			Summary:      "已汇总当前风险事件列表，重点关注未关闭的高风险事项。",
			SourceData:   data,
			ModelOutput:  s.callModelOrFallback(question, data),
			NeedsConfirm: false,
		}, nil
	case "performance_overview":
		data, err := s.fetchMap(s.performanceServiceURL + "/api/v1/contracts/ctr-0001/performance-summary")
		if err != nil {
			return Answer{}, fmt.Errorf("failed to query performance service")
		}
		return Answer{
			Intent:       intent,
			Summary:      "已汇总履约执行摘要，可用于识别延期、异常和当前履约完成度。",
			SourceData:   data,
			ModelOutput:  s.callModelOrFallback(question, data),
			NeedsConfirm: false,
		}, nil
	case "party_credit_overview":
		data, err := s.fetchMap(s.partyServiceURL + "/api/v1/parties")
		if err != nil {
			return Answer{}, fmt.Errorf("failed to query party service")
		}
		return Answer{
			Intent:       intent,
			Summary:      "已汇总相对方主数据与信用概况，涉及准入限制仍以业务规则校验结果为准。",
			SourceData:   data,
			ModelOutput:  s.callModelOrFallback(question, data),
			NeedsConfirm: false,
		}, nil
	case "archive_overview":
		data, err := s.fetchMap(s.archiveServiceURL + "/api/v1/archive/cases")
		if err != nil {
			return Answer{}, fmt.Errorf("failed to query archive service")
		}
		return Answer{
			Intent:       intent,
			Summary:      "已汇总档案状态、借阅和销毁相关信息，实际动作仍需审批流确认。",
			SourceData:   data,
			ModelOutput:  s.callModelOrFallback(question, data),
			NeedsConfirm: true,
		}, nil
	case "dashboard_summary":
		data, err := s.fetchMap(s.reportServiceURL + "/api/v1/reports/dashboard")
		if err != nil {
			return Answer{}, fmt.Errorf("failed to query report service")
		}
		return Answer{
			Intent:       intent,
			Summary:      "已汇总驾驶舱指标，可用于查看合同、审批、风险与归档总体态势。",
			SourceData:   data,
			ModelOutput:  s.callModelOrFallback(question, data),
			NeedsConfirm: false,
		}, nil
	default:
		data, err := s.fetchMap(s.contractServiceURL + "/api/v1/contracts")
		if err != nil {
			return Answer{}, fmt.Errorf("failed to query contract service")
		}
		return Answer{
			Intent:       intent,
			Summary:      "已汇总合同列表基础信息，当前仅提供受控查询与摘要，不直接执行业务写操作。",
			SourceData:   data,
			ModelOutput:  s.callModelOrFallback(question, data),
			NeedsConfirm: true,
		}, nil
	}
}

func (s *Service) fetchMap(endpoint string) (map[string]interface{}, error) {
	if strings.TrimSpace(endpoint) == "" || strings.HasPrefix(endpoint, "/api/") {
		return map[string]interface{}{}, nil
	}
	resp, err := s.httpClient.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) callModelOrFallback(question string, sourceData map[string]interface{}) string {
	if s.modelEndpoint == "" {
		return "910B 推理接口未配置，当前返回规则摘要结果。"
	}

	payload, err := json.Marshal(map[string]interface{}{
		"question":    question,
		"source_data": sourceData,
	})
	if err != nil {
		return "910B 推理请求构造失败，当前返回规则摘要结果。"
	}

	resp, err := s.httpClient.Post(s.modelEndpoint, "application/json", bytes.NewReader(payload))
	if err != nil {
		return "910B 推理接口不可达，当前返回规则摘要结果。"
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Sprintf("910B 推理接口返回异常：%s", strings.TrimSpace(string(body)))
	}

	var result struct {
		Output string `json:"output"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || strings.TrimSpace(result.Output) == "" {
		return "910B 推理输出解析失败，当前返回规则摘要结果。"
	}
	return result.Output
}

func (s *Service) recordAudit(c *gin.Context, action, operatorID string, payload map[string]interface{}) {
	if s.audit == nil {
		return
	}
	traceID, _ := c.Get(middleware.TraceIDKey)
	trace := ""
	if value, ok := traceID.(string); ok {
		trace = value
	}
	_ = s.audit.Record(
		fmt.Sprintf("audit-ai-%d", time.Now().UnixNano()),
		action,
		operatorID,
		trace,
		payload,
	)
}

func mapKeys(data map[string]interface{}) []string {
	if len(data) == 0 {
		return nil
	}
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	return keys
}
