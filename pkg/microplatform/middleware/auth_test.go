package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"contract-manage/pkg/microplatform/auditclient"
	"contract-manage/pkg/microplatform/security"

	"github.com/gin-gonic/gin"
)

func TestEnforcePermissionIfPresentRecordsDeniedAudit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var captured map[string]interface{}
	auditServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode audit request: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer auditServer.Close()

	SetDeniedAuditClient(auditclient.New(auditServer.URL, "middleware-test"))
	defer SetDeniedAuditClient(nil)

	router := gin.New()
	router.Use(Trace())
	router.GET("/contracts", func(c *gin.Context) {
		c.Request.Header.Set("X-User-Id", "u-audited")
		c.Request.Header.Set("X-User-Permissions", "party.read")
		if !EnforcePermissionIfPresent(c, "contract.read") {
			return
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/contracts", nil)
	req.Header.Set("X-Trace-Id", "trace-permission-denied")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.Code)
	}
	if captured["action"] != "auth.permission_denied" {
		t.Fatalf("expected auth.permission_denied, got %v", captured["action"])
	}
	if captured["operator_id"] != "u-audited" {
		t.Fatalf("expected operator u-audited, got %v", captured["operator_id"])
	}
	payload := captured["payload"].(map[string]interface{})
	if payload["request_path"] != "/contracts" {
		t.Fatalf("expected request_path /contracts, got %v", payload["request_path"])
	}
}

func TestRequireDepartmentAccessRecordsDeniedAudit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var captured map[string]interface{}
	auditServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode audit request: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer auditServer.Close()

	SetDeniedAuditClient(auditclient.New(auditServer.URL, "middleware-test"))
	defer SetDeniedAuditClient(nil)

	token, err := security.IssueToken(
		"secret",
		"u-legal",
		"legal-user",
		"legal",
		[]string{"manager"},
		[]string{"contract.read"},
		"department",
		time.Hour,
	)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	router := gin.New()
	router.Use(Trace())
	router.Use(Auth("secret"))
	router.GET("/departments/:department/contracts", RequireDepartmentAccess("department"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/departments/business/contracts", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Trace-Id", "trace-department-denied")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.Code)
	}
	if captured["action"] != "auth.department_scope_denied" {
		t.Fatalf("expected auth.department_scope_denied, got %v", captured["action"])
	}
	payload := captured["payload"].(map[string]interface{})
	if payload["requested_department"] != "business" {
		t.Fatalf("expected requested department business, got %v", payload["requested_department"])
	}
	if payload["user_department"] != "legal" {
		t.Fatalf("expected user department legal, got %v", payload["user_department"])
	}
}
