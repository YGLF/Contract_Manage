package middleware

import (
	"contract-manage/models"
	"contract-manage/services"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAuthorizeRequest_RequiresAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	allowed := AuthorizeRequest(ctx, services.ResourceAuditLogs, services.ActionView)
	if allowed {
		t.Fatal("expected unauthenticated request to be denied")
	}
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, recorder.Code)
	}
}

func TestAuthorizeRequest_AllowsAuditAdminToExport(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("role", string(models.RoleAuditAdmin))

	allowed := AuthorizeRequest(ctx, services.ResourceAuditLogs, services.ActionExport)
	if !allowed {
		t.Fatal("expected audit admin export request to be allowed")
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected recorder code to remain %d, got %d", http.StatusOK, recorder.Code)
	}
}

func TestAuthorizeRequest_DeniesUserAuditDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("role", string(models.RoleUser))

	allowed := AuthorizeRequest(ctx, services.ResourceAuditLogs, services.ActionDelete)
	if allowed {
		t.Fatal("expected audit delete request to be denied")
	}
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, recorder.Code)
	}
}
