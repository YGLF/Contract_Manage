package main

import (
	"log"
	"os"

	"contract-manage/internal/microservices/notification"
	"contract-manage/pkg/microplatform/app"
	"contract-manage/pkg/microplatform/auditclient"
	"contract-manage/pkg/microplatform/config"
	platformdb "contract-manage/pkg/microplatform/db"
)

func main() {
	cfg, err := config.Load("notification-service", 8111)
	if err != nil {
		log.Fatal(err)
	}

	server := app.New(cfg)
	audit := auditclient.New(os.Getenv("AUDIT_SERVICE_URL"), "notification-service")
	autoSendRiskAlerts := os.Getenv("AUTO_SEND_RISK_ALERTS") == "true"
	if cfg.DBEnabled {
		db, err := platformdb.Open(cfg)
		if err != nil {
			log.Fatal(err)
		}
		service := notification.NewWithDB(db)
		service.SetAuditClient(audit)
		service.SetAutoSendRiskAlerts(autoSendRiskAlerts)
		service.RegisterRoutes(server.Router.Group("/api/v1"))
	} else {
		service := notification.New()
		service.SetAuditClient(audit)
		service.SetAutoSendRiskAlerts(autoSendRiskAlerts)
		service.RegisterRoutes(server.Router.Group("/api/v1"))
	}

	log.Fatal(server.Run())
}
