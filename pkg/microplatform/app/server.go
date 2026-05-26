package app

import (
	"fmt"
	"net/http"
	"os"

	"contract-manage/pkg/microplatform/auditclient"
	"contract-manage/pkg/microplatform/config"
	"contract-manage/pkg/microplatform/middleware"

	"github.com/gin-gonic/gin"
)

type Server struct {
	Config config.ServiceConfig
	Router *gin.Engine
}

func New(cfg config.ServiceConfig) *Server {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.Logger())
	router.Use(middleware.Trace())
	middleware.SetDeniedAuditClient(auditclient.New(os.Getenv("AUDIT_SERVICE_URL"), cfg.ServiceName))

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service": cfg.ServiceName,
			"status":  "ok",
		})
	})

	return &Server{
		Config: cfg,
		Router: router,
	}
}

func (s *Server) Run() error {
	return s.Router.Run(fmt.Sprintf(":%d", s.Config.HTTPPort))
}
