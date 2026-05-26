package main

import (
	"log"
	"os"

	"contract-manage/internal/microservices/risk"
	"contract-manage/pkg/microplatform/app"
	"contract-manage/pkg/microplatform/config"
	platformdb "contract-manage/pkg/microplatform/db"
)

func main() {
	cfg, err := config.Load("risk-service", 8107)
	if err != nil {
		log.Fatal(err)
	}

	server := app.New(cfg)
	notificationServiceURL := os.Getenv("NOTIFICATION_SERVICE_URL")
	riskNotificationRecipient := os.Getenv("RISK_NOTIFICATION_RECIPIENT")
	if cfg.DBEnabled {
		db, err := platformdb.Open(cfg)
		if err != nil {
			log.Fatal(err)
		}
		service := risk.NewWithDB(db)
		service.SetNotificationConfig(notificationServiceURL, riskNotificationRecipient)
		service.RegisterRoutes(server.Router.Group("/api/v1"))
	} else {
		service := risk.New()
		service.SetNotificationConfig(notificationServiceURL, riskNotificationRecipient)
		service.RegisterRoutes(server.Router.Group("/api/v1"))
	}

	log.Fatal(server.Run())
}
