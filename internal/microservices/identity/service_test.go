package identity

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"contract-manage/pkg/microplatform/auditclient"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDBBackedLoginDoesNotAcceptEmbeddedDemoUsers(t *testing.T) {
	router := newTestIdentityRouter(t, newIdentityTestDB(t))

	resp := postLogin(router, "admin", "admin123")
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected DB mode to reject embedded admin credentials, got %d", resp.Code)
	}
}

func TestDBBackedLoginAcceptsActiveDatabaseUser(t *testing.T) {
	db := newIdentityTestDB(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("S3cure-passphrase"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if err := db.Create(&dbUser{
		Username:       "db-manager",
		HashedPassword: string(hash),
		Role:           "manager",
		Department:     "business",
		IsActive:       true,
		AccountStatus:  "permanent",
	}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	router := newTestIdentityRouter(t, db)
	resp := postLogin(router, "db-manager", "S3cure-passphrase")
	if resp.Code != http.StatusOK {
		t.Fatalf("expected DB user login to succeed, got %d body=%s", resp.Code, resp.Body.String())
	}

	var body struct {
		Data struct {
			User struct {
				ID          string   `json:"id"`
				Username    string   `json:"username"`
				Department  string   `json:"department"`
				Roles       []string `json:"roles"`
				Permissions []string `json:"permissions"`
				DataScope   string   `json:"data_scope"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Data.User.ID == "" || body.Data.User.Username != "db-manager" {
		t.Fatalf("unexpected user payload: %+v", body.Data.User)
	}
	if body.Data.User.DataScope != "department" {
		t.Fatalf("expected manager department data scope, got %q", body.Data.User.DataScope)
	}
	if len(body.Data.User.Permissions) == 0 {
		t.Fatalf("expected permissions mapped from role")
	}
}

func TestLoginSuccessRecordsAuditWithoutPassword(t *testing.T) {
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

	db := newIdentityTestDB(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("S3cure-passphrase"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if err := db.Create(&dbUser{
		Username:       "db-manager",
		HashedPassword: string(hash),
		Role:           "manager",
		Department:     "business",
		IsActive:       true,
		AccountStatus:  "permanent",
	}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	router := newTestIdentityRouterWithAudit(t, db, auditclient.New(auditServer.URL, "identity-service"))
	resp := postLogin(router, "db-manager", "S3cure-passphrase")
	if resp.Code != http.StatusOK {
		t.Fatalf("expected login to succeed, got %d body=%s", resp.Code, resp.Body.String())
	}

	if captured["action"] != "identity.login_success" {
		t.Fatalf("expected login success audit action, got %v", captured["action"])
	}
	if captured["operator_id"] == "" || captured["operator_id"] == "anonymous" {
		t.Fatalf("expected authenticated operator id, got %v", captured["operator_id"])
	}
	payload := captured["payload"].(map[string]interface{})
	if payload["username"] != "db-manager" {
		t.Fatalf("expected audited username db-manager, got %v", payload["username"])
	}
	if strings.Contains(string(mustJSON(t, captured)), "S3cure-passphrase") {
		t.Fatalf("audit payload must not contain plaintext password")
	}
}

func TestLoginFailureRecordsAuditWithoutPassword(t *testing.T) {
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

	router := newTestIdentityRouterWithAudit(t, newIdentityTestDB(t), auditclient.New(auditServer.URL, "identity-service"))
	resp := postLogin(router, "admin", "wrong-password")
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected login to fail, got %d body=%s", resp.Code, resp.Body.String())
	}

	if captured["action"] != "identity.login_failed" {
		t.Fatalf("expected login failure audit action, got %v", captured["action"])
	}
	if captured["operator_id"] != "anonymous" {
		t.Fatalf("expected anonymous operator for failed login, got %v", captured["operator_id"])
	}
	payload := captured["payload"].(map[string]interface{})
	if payload["username"] != "admin" {
		t.Fatalf("expected attempted username admin, got %v", payload["username"])
	}
	if strings.Contains(string(mustJSON(t, captured)), "wrong-password") {
		t.Fatalf("audit payload must not contain plaintext password")
	}
}

func newIdentityTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&dbUser{}); err != nil {
		t.Fatalf("migrate users: %v", err)
	}
	return db
}

func newTestIdentityRouter(t *testing.T, db *gorm.DB) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewWithDB("test-secret", db).RegisterRoutes(router.Group("/api/v1"))
	return router
}

func newTestIdentityRouterWithAudit(t *testing.T, db *gorm.DB, audit *auditclient.Client) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	service := NewWithDB("test-secret", db)
	service.SetAuditClient(audit)
	service.RegisterRoutes(router.Group("/api/v1"))
	return router
}

func postLogin(router http.Handler, username, password string) *httptest.ResponseRecorder {
	payload := []byte(`{"username":"` + username + `","password":"` + password + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}

func mustJSON(t *testing.T, value interface{}) []byte {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return data
}
