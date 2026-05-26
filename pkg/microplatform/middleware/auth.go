package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"contract-manage/pkg/microplatform/auditclient"
	"contract-manage/pkg/microplatform/httpx"
	"contract-manage/pkg/microplatform/security"

	"github.com/gin-gonic/gin"
)

const ClaimsKey = "auth_claims"

var deniedAudit struct {
	mu     sync.RWMutex
	client *auditclient.Client
}

type RequestIdentity struct {
	UserID      string
	Department  string
	DataScope   string
	Roles       []string
	Permissions []string
}

func SetDeniedAuditClient(client *auditclient.Client) {
	deniedAudit.mu.Lock()
	defer deniedAudit.mu.Unlock()
	deniedAudit.client = client
}

func Auth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			recordDeniedAudit(c, "auth.missing_authorization", "missing authorization header", http.StatusUnauthorized, nil)
			httpx.Error(c, http.StatusUnauthorized, "missing authorization header")
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			recordDeniedAudit(c, "auth.invalid_authorization", "invalid authorization header", http.StatusUnauthorized, nil)
			httpx.Error(c, http.StatusUnauthorized, "invalid authorization header")
			c.Abort()
			return
		}

		claims, err := security.ParseToken(secret, parts[1])
		if err != nil {
			recordDeniedAudit(c, "auth.invalid_token", "invalid token", http.StatusUnauthorized, nil)
			httpx.Error(c, http.StatusUnauthorized, "invalid token")
			c.Abort()
			return
		}

		c.Set(ClaimsKey, claims)
		c.Next()
	}
}

func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}

	return func(c *gin.Context) {
		value, ok := c.Get(ClaimsKey)
		if !ok {
			recordDeniedAudit(c, "auth.missing_context", "missing auth context", http.StatusUnauthorized, map[string]interface{}{
				"required_roles": roles,
			})
			httpx.Error(c, http.StatusUnauthorized, "missing auth context")
			c.Abort()
			return
		}

		claims, ok := value.(*security.Claims)
		if !ok {
			recordDeniedAudit(c, "auth.invalid_context", "invalid auth context", http.StatusUnauthorized, map[string]interface{}{
				"required_roles": roles,
			})
			httpx.Error(c, http.StatusUnauthorized, "invalid auth context")
			c.Abort()
			return
		}

		for _, role := range claims.Roles {
			if _, exists := allowed[role]; exists {
				c.Next()
				return
			}
		}

		recordDeniedAudit(c, "auth.role_denied", "insufficient role", http.StatusForbidden, map[string]interface{}{
			"required_roles": roles,
			"user_roles":     claims.Roles,
		})
		httpx.Error(c, http.StatusForbidden, "insufficient role")
		c.Abort()
	}
}

func RequirePermission(permissions ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(permissions))
	for _, permission := range permissions {
		allowed[permission] = struct{}{}
	}

	return func(c *gin.Context) {
		claims, ok := claimsFromContext(c)
		if !ok {
			recordDeniedAudit(c, "auth.missing_context", "missing auth context", http.StatusUnauthorized, map[string]interface{}{
				"required_permissions": permissions,
			})
			httpx.Error(c, http.StatusUnauthorized, "missing auth context")
			c.Abort()
			return
		}

		for _, permission := range claims.Permissions {
			if permission == "*" {
				c.Next()
				return
			}
			if _, exists := allowed[permission]; exists {
				c.Next()
				return
			}
		}

		recordDeniedAudit(c, "auth.permission_denied", "insufficient permission", http.StatusForbidden, map[string]interface{}{
			"required_permissions": permissions,
			"user_permissions":     claims.Permissions,
		})
		httpx.Error(c, http.StatusForbidden, "insufficient permission")
		c.Abort()
	}
}

func RequireDepartmentAccess(paramKeys ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := claimsFromContext(c)
		if !ok {
			recordDeniedAudit(c, "auth.missing_context", "missing auth context", http.StatusUnauthorized, map[string]interface{}{
				"department_param_keys": paramKeys,
			})
			httpx.Error(c, http.StatusUnauthorized, "missing auth context")
			c.Abort()
			return
		}
		if hasRole(claims, "admin") || claims.DataScope == "all" {
			c.Next()
			return
		}
		if claims.DataScope == "" || claims.DataScope == "department" {
			for _, key := range paramKeys {
				value := strings.TrimSpace(c.Query(key))
				if value == "" {
					value = strings.TrimSpace(c.Param(key))
				}
				if value != "" && value != claims.Department {
					recordDeniedAudit(c, "auth.department_scope_denied", "department data scope denied", http.StatusForbidden, map[string]interface{}{
						"department_param_keys": paramKeys,
						"requested_department":  value,
						"user_department":       claims.Department,
					})
					httpx.Error(c, http.StatusForbidden, "department data scope denied")
					c.Abort()
					return
				}
			}
		}
		c.Next()
	}
}

func claimsFromContext(c *gin.Context) (*security.Claims, bool) {
	value, ok := c.Get(ClaimsKey)
	if !ok {
		return nil, false
	}
	claims, ok := value.(*security.Claims)
	return claims, ok
}

func hasRole(claims *security.Claims, role string) bool {
	for _, item := range claims.Roles {
		if item == role {
			return true
		}
	}
	return false
}

func IdentityFromContextOrHeaders(c *gin.Context) (RequestIdentity, bool) {
	if claims, ok := claimsFromContext(c); ok {
		return RequestIdentity{
			UserID:      claims.UserID,
			Department:  claims.Department,
			DataScope:   claims.DataScope,
			Roles:       claims.Roles,
			Permissions: claims.Permissions,
		}, true
	}

	permissions := splitCSVHeader(c.GetHeader("X-User-Permissions"))
	roles := splitCSVHeader(c.GetHeader("X-User-Roles"))
	userID := strings.TrimSpace(c.GetHeader("X-User-Id"))
	department := strings.TrimSpace(c.GetHeader("X-User-Department"))
	dataScope := strings.TrimSpace(c.GetHeader("X-Data-Scope"))
	if userID == "" && department == "" && dataScope == "" && len(roles) == 0 && len(permissions) == 0 {
		return RequestIdentity{}, false
	}
	return RequestIdentity{
		UserID:      userID,
		Department:  department,
		DataScope:   dataScope,
		Roles:       roles,
		Permissions: permissions,
	}, true
}

func EnforcePermissionIfPresent(c *gin.Context, permissions ...string) bool {
	identity, ok := IdentityFromContextOrHeaders(c)
	if !ok {
		return true
	}
	for _, permission := range identity.Permissions {
		if permission == "*" {
			return true
		}
		for _, required := range permissions {
			if permission == required {
				return true
			}
		}
	}
	httpx.Error(c, http.StatusForbidden, "insufficient permission")
	recordDeniedAudit(c, "auth.permission_denied", "insufficient permission", http.StatusForbidden, map[string]interface{}{
		"required_permissions": permissions,
		"user_permissions":     identity.Permissions,
	})
	c.Abort()
	return false
}

func CurrentOperatorID(c *gin.Context, fallback string) string {
	identity, ok := IdentityFromContextOrHeaders(c)
	if ok && strings.TrimSpace(identity.UserID) != "" {
		return identity.UserID
	}
	return fallback
}

func splitCSVHeader(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, item := range parts {
		cleaned := strings.TrimSpace(item)
		if cleaned != "" {
			result = append(result, cleaned)
		}
	}
	return result
}

func recordDeniedAudit(c *gin.Context, action, reason string, statusCode int, payload map[string]interface{}) {
	deniedAudit.mu.RLock()
	client := deniedAudit.client
	deniedAudit.mu.RUnlock()
	if client == nil || !client.Enabled() {
		return
	}

	identity, _ := IdentityFromContextOrHeaders(c)
	traceID := c.GetHeader("X-Trace-Id")
	if traceID == "" {
		if value, ok := c.Get(TraceIDKey); ok {
			if v, ok := value.(string); ok {
				traceID = v
			}
		}
	}

	auditPayload := map[string]interface{}{
		"reason":         reason,
		"status_code":    statusCode,
		"request_path":   c.Request.URL.Path,
		"request_method": c.Request.Method,
		"department":     identity.Department,
		"data_scope":     identity.DataScope,
	}
	if len(identity.Roles) > 0 {
		auditPayload["roles"] = identity.Roles
	}
	if len(identity.Permissions) > 0 {
		auditPayload["permissions"] = identity.Permissions
	}
	for key, value := range payload {
		auditPayload[key] = value
	}

	operatorID := strings.TrimSpace(identity.UserID)
	if operatorID == "" {
		operatorID = "anonymous"
	}

	_ = client.Record(
		"deny-"+action+"-"+time.Now().Format("20060102150405.000000000"),
		action,
		operatorID,
		traceID,
		auditPayload,
	)
}
