package gateway

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"

	"contract-manage/pkg/microplatform/httpx"
	"contract-manage/pkg/microplatform/middleware"
	"contract-manage/pkg/microplatform/security"

	"github.com/gin-gonic/gin"
)

type Service struct {
	targets   map[string]string
	jwtSecret string
}

func New(targets map[string]string, jwtSecret string) *Service {
	return &Service{targets: targets, jwtSecret: jwtSecret}
}

func (s *Service) RegisterRoutes(router gin.IRouter) {
	router.GET("/gateway/routes", s.routes)
	router.Use(s.securityHeaders())

	identity := router.Group("/api/identity")
	identity.Any("/*path", s.proxy("identity"))

	protected := router.Group("/")
	protected.Use(middleware.Auth(s.jwtSecret))
	protected.Any("/api/audit/*path", s.proxy("audit"))
	protected.Any("/api/contracts/*path", s.proxy("contract"))
	protected.Any("/api/documents/*path", s.proxy("document"))
	protected.Any("/api/performance/*path", s.proxy("performance"))
	protected.Any("/api/workflows/*path", s.proxy("approval-workflow"))
	protected.Any("/api/risk/*path", s.proxy("risk"))
	protected.Any("/api/amendments/*path", s.proxy("amendment"))
	protected.Any("/api/closure/*path", s.proxy("closure"))
	protected.Any("/api/archive/*path", s.proxy("archive"))
	protected.Any("/api/notifications/*path", s.proxy("notification"))
	protected.Any("/api/reports/*path", s.proxy("report"))
	protected.Any("/api/parties/*path", s.proxy("party"))
	protected.Any("/api/search-ai/*path", s.proxy("search-ai"))
}

func (s *Service) routes(c *gin.Context) {
	httpx.Success(c, s.targets)
}

func (s *Service) proxy(serviceName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		targetURL := strings.TrimSpace(s.targets[serviceName])
		if targetURL == "" {
			httpx.Error(c, http.StatusNotImplemented, "service target is not configured")
			return
		}

		parsed, err := url.Parse(targetURL)
		if err != nil {
			httpx.Error(c, http.StatusInternalServerError, "invalid service target")
			return
		}

		proxy := httputil.NewSingleHostReverseProxy(parsed)
		prefix := c.FullPath()
		prefix = strings.TrimSuffix(prefix, "/*path")
		targetBasePath := strings.TrimRight(parsed.Path, "/")
		servicePath := c.Param("path")
		if servicePath == "" {
			servicePath = "/"
		}
		proxy.Director = func(req *http.Request) {
			req.URL.Scheme = parsed.Scheme
			req.URL.Host = parsed.Host
			req.Host = parsed.Host
			req.URL.Path = path.Clean(targetBasePath + "/api/v1" + servicePath)
			req.URL.RawPath = req.URL.Path
			if prefix != "" && strings.HasPrefix(req.URL.Path, prefix) {
				req.URL.Path = path.Clean(targetBasePath + "/api/v1" + servicePath)
			}
			if traceID := c.GetHeader("X-Trace-Id"); traceID != "" {
				req.Header.Set("X-Trace-Id", traceID)
			}
			if value, ok := c.Get(middleware.ClaimsKey); ok {
				if claims, ok := value.(*security.Claims); ok {
					req.Header.Set("X-User-Id", claims.UserID)
					req.Header.Set("X-User-Department", claims.Department)
					req.Header.Set("X-User-Roles", strings.Join(claims.Roles, ","))
					req.Header.Set("X-User-Permissions", strings.Join(claims.Permissions, ","))
					req.Header.Set("X-Data-Scope", claims.DataScope)
				}
			}
		}
		proxy.ErrorHandler = func(writer http.ResponseWriter, request *http.Request, err error) {
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusBadGateway)
			_, _ = writer.Write([]byte(`{"success":false,"error":"upstream service unavailable"}`))
		}
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

func (s *Service) securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "same-origin")
		c.Header("Cache-Control", "no-store")
		c.Next()
	}
}
