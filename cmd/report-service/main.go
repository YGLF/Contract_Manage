package main

import (
	"log"
	"os"

	"contract-manage/internal/microservices/report"
	"contract-manage/pkg/microplatform/app"
	"contract-manage/pkg/microplatform/auditclient"
	"contract-manage/pkg/microplatform/config"
	platformdb "contract-manage/pkg/microplatform/db"
)

func main() {
	cfg, err := config.Load("report-service", 8112)
	if err != nil {
		log.Fatal(err)
	}

	server := app.New(cfg)
	audit := auditclient.New(os.Getenv("AUDIT_SERVICE_URL"), "report-service")
	contractServiceURL := os.Getenv("CONTRACT_SERVICE_URL")
	approvalServiceURL := os.Getenv("APPROVAL_SERVICE_URL")
	riskServiceURL := os.Getenv("RISK_SERVICE_URL")
	archiveServiceURL := os.Getenv("ARCHIVE_SERVICE_URL")
	closureServiceURL := os.Getenv("CLOSURE_SERVICE_URL")
	if cfg.DBEnabled {
		db, err := platformdb.Open(cfg)
		if err != nil {
			log.Fatal(err)
		}
		service := report.NewWithDB(db)
		service.SetAuditClient(audit)
		service.SetServiceURLs(contractServiceURL, approvalServiceURL, riskServiceURL, archiveServiceURL, closureServiceURL)
		service.RegisterRoutes(server.Router.Group("/api/v1"))
	} else {
		service := report.New()
		service.SetAuditClient(audit)
		service.SetServiceURLs(contractServiceURL, approvalServiceURL, riskServiceURL, archiveServiceURL, closureServiceURL)
		service.RegisterRoutes(server.Router.Group("/api/v1"))
	}

	log.Fatal(server.Run())
}
