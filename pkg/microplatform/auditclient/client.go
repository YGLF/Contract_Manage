package auditclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	service    string
	httpClient *http.Client
}

func New(baseURL, service string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		service: service,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (c *Client) Enabled() bool {
	return c != nil && c.baseURL != ""
}

func (c *Client) Record(id, action, operatorID, traceID string, payload map[string]interface{}) error {
	if !c.Enabled() {
		return nil
	}

	body := map[string]interface{}{
		"id":          id,
		"service":     c.service,
		"action":      action,
		"operator_id": operatorID,
		"payload":     payload,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/api/v1/audit/logs", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if traceID != "" {
		req.Header.Set("X-Trace-Id", traceID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("audit service returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return nil
}
