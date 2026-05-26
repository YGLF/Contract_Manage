package identity

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"contract-manage/pkg/microplatform/auditclient"
	"contract-manage/pkg/microplatform/httpx"
	"contract-manage/pkg/microplatform/middleware"
	"contract-manage/pkg/microplatform/security"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Service struct {
	jwtSecret string
	users     map[string]User
	db        *gorm.DB
	audit     *auditclient.Client
}

type User struct {
	ID          string   `json:"id"`
	Username    string   `json:"username"`
	Department  string   `json:"department"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
	DataScope   string   `json:"data_scope"`
	Password    string   `json:"-"`
}

type dbUser struct {
	ID                uint       `gorm:"primaryKey"`
	Username          string     `gorm:"column:username"`
	HashedPassword    string     `gorm:"column:hashed_password"`
	Role              string     `gorm:"column:role"`
	CustomPermissions string     `gorm:"column:custom_permissions"`
	Department        string     `gorm:"column:department"`
	IsActive          bool       `gorm:"column:is_active"`
	AccountStatus     string     `gorm:"column:account_status"`
	ValidFrom         *time.Time `gorm:"column:valid_from"`
	ValidTo           *time.Time `gorm:"column:valid_to"`
	ValidHours        int        `gorm:"column:valid_hours"`
}

func (dbUser) TableName() string {
	return "users"
}

func New(jwtSecret string) *Service {
	return &Service{
		jwtSecret: jwtSecret,
		users: map[string]User{
			"admin": {
				ID:          "u-admin",
				Username:    "admin",
				Department:  "platform",
				Roles:       []string{"admin"},
				Permissions: []string{"*"},
				DataScope:   "all",
				Password:    "admin123",
			},
			"auditor": {
				ID:          "u-auditor",
				Username:    "auditor",
				Department:  "audit",
				Roles:       []string{"audit_admin"},
				Permissions: []string{"audit.view", "report.read", "report.export", "contract.read", "approval.read"},
				DataScope:   "all",
				Password:    "audit123",
			},
			"manager": {
				ID:          "u-manager",
				Username:    "manager",
				Department:  "business",
				Roles:       []string{"manager"},
				Permissions: []string{"contract.read", "contract.create", "contract.update", "contract.delete", "contract.status.change", "contract.amendment.apply", "performance.read", "performance.write", "risk.read", "risk.write", "risk.dispose", "archive.read", "archive.write", "archive.borrow", "archive.destroy", "approval.read", "approval.request", "approval.process", "report.read", "search_ai.ask", "notification.read", "notification.send", "party.read", "party.write", "party.credit.write", "document.read", "document.upload", "document.commit", "document.bind", "closure.read", "closure.request", "closure.process"},
				DataScope:   "department",
				Password:    "manager123",
			},
		},
	}
}

func NewWithDB(jwtSecret string, db *gorm.DB) *Service {
	return &Service{
		jwtSecret: jwtSecret,
		db:        db,
	}
}

func (s *Service) SetAuditClient(client *auditclient.Client) {
	s.audit = client
}

func (s *Service) RegisterRoutes(router gin.IRouter) {
	router.POST("/auth/login", s.login)
	router.GET("/auth/me", middleware.Auth(s.jwtSecret), s.me)
	router.GET("/users", middleware.Auth(s.jwtSecret), middleware.RequirePermission("user.manage"), s.listUsers)
}

func (s *Service) login(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid login payload")
		return
	}

	if s.db != nil {
		usr, err := s.authenticateDBUser(req.Username, req.Password)
		if err != nil {
			s.recordLoginAudit(c, "identity.login_failed", "anonymous", strings.TrimSpace(req.Username), "invalid_credentials")
			httpx.Error(c, http.StatusUnauthorized, "invalid credentials")
			return
		}
		s.recordLoginAudit(c, "identity.login_success", usr.ID, usr.Username, "")
		s.respondLoginSuccess(c, usr)
		return
	}

	usr, ok := s.users[req.Username]
	if !ok || usr.Password != req.Password {
		s.recordLoginAudit(c, "identity.login_failed", "anonymous", strings.TrimSpace(req.Username), "invalid_credentials")
		httpx.Error(c, http.StatusUnauthorized, "invalid credentials")
		return
	}

	s.recordLoginAudit(c, "identity.login_success", usr.ID, usr.Username, "")
	s.respondLoginSuccess(c, usr)
}

func (s *Service) recordLoginAudit(c *gin.Context, action, operatorID, username, reason string) {
	if s.audit == nil || !s.audit.Enabled() {
		return
	}

	trace := c.GetHeader("X-Trace-Id")
	if trace == "" {
		if value, exists := c.Get(middleware.TraceIDKey); exists {
			if text, ok := value.(string); ok {
				trace = text
			}
		}
	}

	payload := map[string]interface{}{
		"username":       strings.TrimSpace(username),
		"request_path":   c.Request.URL.Path,
		"request_method": c.Request.Method,
		"remote_addr":    c.ClientIP(),
	}
	if reason != "" {
		payload["reason"] = reason
	}

	if strings.TrimSpace(operatorID) == "" {
		operatorID = "anonymous"
	}

	_ = s.audit.Record(
		fmt.Sprintf("audit-login-%d", time.Now().UnixNano()),
		action,
		operatorID,
		trace,
		payload,
	)
}

func (s *Service) respondLoginSuccess(c *gin.Context, usr User) {
	token, err := security.IssueToken(s.jwtSecret, usr.ID, usr.Username, usr.Department, usr.Roles, usr.Permissions, usr.DataScope, 30*time.Minute)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "failed to issue token")
		return
	}

	httpx.Success(c, gin.H{
		"access_token": token,
		"user": gin.H{
			"id":          usr.ID,
			"username":    usr.Username,
			"department":  usr.Department,
			"roles":       usr.Roles,
			"permissions": usr.Permissions,
			"data_scope":  usr.DataScope,
		},
	})
}

func (s *Service) authenticateDBUser(username, password string) (User, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return User{}, fmt.Errorf("empty credentials")
	}

	var record dbUser
	if err := s.db.Where("username = ?", username).First(&record).Error; err != nil {
		return User{}, err
	}
	if !record.accountValid(time.Now()) {
		return User{}, fmt.Errorf("inactive account")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(record.HashedPassword), []byte(password)); err != nil {
		return User{}, err
	}

	role := strings.TrimSpace(record.Role)
	if role == "" {
		role = "user"
	}
	permissions := permissionsForRole(role)
	permissions = mergePermissions(permissions, parseCustomPermissions(record.CustomPermissions))

	return User{
		ID:          "u-" + strconv.FormatUint(uint64(record.ID), 10),
		Username:    record.Username,
		Department:  strings.TrimSpace(record.Department),
		Roles:       []string{role},
		Permissions: permissions,
		DataScope:   dataScopeForRole(role),
	}, nil
}

func (u dbUser) accountValid(now time.Time) bool {
	status := strings.TrimSpace(u.AccountStatus)
	if status == "" {
		status = "permanent"
	}
	if !u.IsActive || status == "disabled" {
		return false
	}
	switch status {
	case "permanent":
		return true
	case "temporary":
		if u.ValidHours > 0 && u.ValidFrom != nil {
			return now.Before(u.ValidFrom.Add(time.Duration(u.ValidHours) * time.Hour))
		}
		return u.ValidTo != nil && now.Before(*u.ValidTo)
	case "timed":
		if u.ValidFrom != nil && now.Before(*u.ValidFrom) {
			return false
		}
		return u.ValidTo == nil || now.Before(*u.ValidTo)
	default:
		return u.IsActive
	}
}

func permissionsForRole(role string) []string {
	switch role {
	case "admin":
		return []string{"*"}
	case "audit_admin":
		return []string{"audit.view", "report.read", "report.export", "contract.read", "approval.read"}
	case "manager", "sales_director", "tech_director", "finance_director", "contract_admin":
		return []string{"contract.read", "contract.create", "contract.update", "contract.status.change", "contract.amendment.apply", "performance.read", "performance.write", "risk.read", "risk.write", "risk.dispose", "archive.read", "archive.write", "archive.borrow", "archive.destroy", "approval.read", "approval.request", "approval.process", "report.read", "search_ai.ask", "notification.read", "notification.send", "party.read", "party.write", "party.credit.write", "document.read", "document.upload", "document.commit", "document.bind", "closure.read", "closure.request", "closure.process"}
	default:
		return []string{"contract.read", "performance.read", "risk.read", "archive.read", "approval.read", "report.read", "notification.read", "party.read", "document.read", "closure.read"}
	}
}

func dataScopeForRole(role string) string {
	switch role {
	case "admin", "audit_admin":
		return "all"
	default:
		return "department"
	}
}

func parseCustomPermissions(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err == nil {
		return cleanPermissionList(values)
	}
	return cleanPermissionList(strings.Split(raw, ","))
}

func mergePermissions(base, extra []string) []string {
	seen := make(map[string]bool, len(base)+len(extra))
	merged := make([]string, 0, len(base)+len(extra))
	for _, value := range append(base, extra...) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		merged = append(merged, value)
	}
	return merged
}

func cleanPermissionList(values []string) []string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			cleaned = append(cleaned, value)
		}
	}
	return cleaned
}

func (s *Service) me(c *gin.Context) {
	value, _ := c.Get(middleware.ClaimsKey)
	claims := value.(*security.Claims)
	httpx.Success(c, claims)
}

func (s *Service) listUsers(c *gin.Context) {
	identity, hasIdentity := middleware.IdentityFromContextOrHeaders(c)
	users := s.visibleUsers()
	result := make([]gin.H, 0, len(users))
	for _, usr := range users {
		if hasIdentity && identity.DataScope == "department" && identity.Department != "" && usr.Department != identity.Department {
			continue
		}
		result = append(result, gin.H{
			"id":          usr.ID,
			"username":    usr.Username,
			"department":  usr.Department,
			"roles":       usr.Roles,
			"permissions": usr.Permissions,
			"data_scope":  usr.DataScope,
		})
	}
	httpx.Success(c, result)
}

func (s *Service) visibleUsers() []User {
	if s.db == nil {
		users := make([]User, 0, len(s.users))
		for _, usr := range s.users {
			users = append(users, usr)
		}
		return users
	}

	var records []dbUser
	if err := s.db.Find(&records).Error; err != nil {
		return nil
	}
	users := make([]User, 0, len(records))
	now := time.Now()
	for _, record := range records {
		if !record.accountValid(now) {
			continue
		}
		role := strings.TrimSpace(record.Role)
		if role == "" {
			role = "user"
		}
		users = append(users, User{
			ID:          "u-" + strconv.FormatUint(uint64(record.ID), 10),
			Username:    record.Username,
			Department:  strings.TrimSpace(record.Department),
			Roles:       []string{role},
			Permissions: mergePermissions(permissionsForRole(role), parseCustomPermissions(record.CustomPermissions)),
			DataScope:   dataScopeForRole(role),
		})
	}
	return users
}
