package main

import (
	"log"
	"os"

	"contract-manage/internal/microservices/closure"
	"contract-manage/pkg/microplatform/app"
	"contract-manage/pkg/microplatform/auditclient"
	"contract-manage/pkg/microplatform/config"
	platformdb "contract-manage/pkg/microplatform/db"
)

func main() {
	cfg, err := config.Load("closure-service", 8109)
	if err != nil {
		log.Fatal(err)
	}

	server := app.New(cfg)
	archiveURL := os.Getenv("ARCHIVE_SERVICE_URL")
	performanceURL := os.Getenv("PERFORMANCE_SERVICE_URL")
	riskURL := os.Getenv("RISK_SERVICE_URL")
	audit := auditclient.New(os.Getenv("AUDIT_SERVICE_URL"), "closure-service")
	if cfg.DBEnabled {
		db, err := platformdb.Open(cfg)
		if err != nil {
			log.Fatal(err)
		}
		service := closure.NewWithDB(db)
		service.SetArchiveURL(archiveURL)
		service.SetDependencyURLs(performanceURL, riskURL)
		service.SetAuditClient(audit)
		service.RegisterRoutes(server.Router.Group("/api/v1"))
	} else {
		service := closure.New()
		service.SetArchiveURL(archiveURL)
		service.SetDependencyURLs(performanceURL, riskURL)
		service.SetAuditClient(audit)
		service.RegisterRoutes(server.Router.Group("/api/v1"))
	}

	log.Fatal(server.Run())
}
