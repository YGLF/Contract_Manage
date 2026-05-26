package outbox

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"
)

type Dispatcher struct {
	db          *gorm.DB
	httpClient  *http.Client
	notifyURL   string
	reportURL   string
}

func NewDispatcher(db *gorm.DB, notifyURL, reportURL string) *Dispatcher {
	return &Dispatcher{
		db:        db,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		notifyURL: strings.TrimRight(notifyURL, "/"),
		reportURL: strings.TrimRight(reportURL, "/"),
	}
}

func (d *Dispatcher) DispatchPending(limit int) (int, error) {
	if d.db == nil {
		return 0, fmt.Errorf("dispatcher database is nil")
	}

	var messages []Message
	if err := d.db.Where("status = ?", "pending").Order("created_at asc").Limit(limit).Find(&messages).Error; err != nil {
		return 0, err
	}

	dispatched := 0
	for _, message := range messages {
		if err := d.dispatch(message); err != nil {
			continue
		}
		if err := d.db.Model(&Message{}).Where("id = ?", message.ID).Update("status", "dispatched").Error; err != nil {
			return dispatched, err
		}
		dispatched++
	}

	return dispatched, nil
}

func (d *Dispatcher) dispatch(message Message) error {
	payloadMap := map[string]interface{}{}
	_ = json.Unmarshal([]byte(message.PayloadRaw), &payloadMap)

	if d.notifyURL != "" {
		if notificationPayloads := d.buildNotificationPayloads(message, payloadMap); len(notificationPayloads) > 0 {
			for _, payload := range notificationPayloads {
				if err := d.postJSON(d.notifyURL+"/api/v1/notifications/messages", payload, message.TraceID); err != nil {
					return err
				}
			}
		}
	}

	if d.reportURL != "" {
		if metrics := d.buildReportMetrics(message); len(metrics) > 0 {
			for _, payload := range metrics {
				if err := d.postJSON(d.reportURL+"/api/v1/reports/metrics", payload, message.TraceID); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (d *Dispatcher) buildNotificationPayloads(message Message, payload map[string]interface{}) []map[string]interface{} {
	switch message.EventType {
	case "risk.created":
		return []map[string]interface{}{
			{
				"id":          "dispatch-" + message.ID,
				"channel":     "inbox",
				"recipient":   "risk-manager",
				"subject":     "风险预警通知",
				"body":        fmt.Sprintf("合同 %v 触发风险规则 %v，严重级别 %v。", payload["contract_id"], payload["rule_code"], payload["severity"]),
				"template":    "risk_alert",
				"source_type": message.AggregateType,
				"source_id":   message.AggregateID,
			},
		}
	case "closure.requested":
		return []map[string]interface{}{
			{
				"id":          "dispatch-" + message.ID,
				"channel":     "inbox",
				"recipient":   "contract-admin",
				"subject":     "结案申请通知",
				"body":        fmt.Sprintf("合同 %v 发起了 %v 申请，请处理。", payload["contract_id"], payload["request_type"]),
				"template":    "closure_requested",
				"source_type": message.AggregateType,
				"source_id":   message.AggregateID,
			},
		}
	case "closure.completed":
		return []map[string]interface{}{
			{
				"id":          "dispatch-" + message.ID,
				"channel":     "inbox",
				"recipient":   "archive-admin",
				"subject":     "结案完成归档通知",
				"body":        fmt.Sprintf("合同 %v 已完成结案，可进入归档流程。", payload["contract_id"]),
				"template":    "closure_completed",
				"source_type": message.AggregateType,
				"source_id":   message.AggregateID,
			},
		}
	case "contract.status_changed":
		return []map[string]interface{}{
			{
				"id":          "dispatch-" + message.ID,
				"channel":     "inbox",
				"recipient":   "contract-owner",
				"subject":     "合同状态变更通知",
				"body":        fmt.Sprintf("合同状态已从 %v 变更为 %v。", payload["from_status"], payload["to_status"]),
				"template":    "contract_status_changed",
				"source_type": message.AggregateType,
				"source_id":   message.AggregateID,
			},
		}
	default:
		return nil
	}
}

func (d *Dispatcher) buildReportMetrics(message Message) []map[string]interface{} {
	eventMetric := map[string]interface{}{
		"id":                "metric-" + message.ID,
		"metric_name":       "events." + strings.ReplaceAll(message.EventType, ".", "_"),
		"metric_value":      1,
		"dimension":         message.AggregateType,
		"source_event_type": message.EventType,
	}

	result := []map[string]interface{}{eventMetric}
	switch message.EventType {
	case "contract.created":
		result = append(result, map[string]interface{}{
			"id":                "metric-contract-created-" + message.ID,
			"metric_name":       "contracts.created.total",
			"metric_value":      1,
			"dimension":         "contract",
			"source_event_type": message.EventType,
		})
	case "risk.created":
		result = append(result, map[string]interface{}{
			"id":                "metric-risk-created-" + message.ID,
			"metric_name":       "risks.open.total",
			"metric_value":      1,
			"dimension":         "risk",
			"source_event_type": message.EventType,
		})
	case "risk.closed":
		result = append(result, map[string]interface{}{
			"id":                "metric-risk-closed-" + message.ID,
			"metric_name":       "risks.closed.total",
			"metric_value":      1,
			"dimension":         "risk",
			"source_event_type": message.EventType,
		})
	case "closure.completed":
		result = append(result, map[string]interface{}{
			"id":                "metric-closure-completed-" + message.ID,
			"metric_name":       "closures.completed.total",
			"metric_value":      1,
			"dimension":         "closure",
			"source_event_type": message.EventType,
		})
	}
	return result
}

func (d *Dispatcher) postJSON(url string, payload map[string]interface{}, traceID string) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if traceID != "" {
		req.Header.Set("X-Trace-Id", traceID)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("outbox downstream returned status %d for %s: %s", resp.StatusCode, url, strings.TrimSpace(string(body)))
	}
	return nil
}
