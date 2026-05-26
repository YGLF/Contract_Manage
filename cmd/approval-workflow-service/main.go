package main

import (
	"log"
	"os"

	"contract-manage/internal/microservices/approvalworkflow"
	"contract-manage/pkg/microplatform/app"
	"contract-manage/pkg/microplatform/config"
	platformdb "contract-manage/pkg/microplatform/db"
)

func main() {
	cfg, err := config.Load("approval-workflow-service", 8106)
	if err != nil {
		log.Fatal(err)
	}

	server := app.New(cfg)
	contractServiceURL := os.Getenv("CONTRACT_SERVICE_URL")
	performanceServiceURL := os.Getenv("PERFORMANCE_SERVICE_URL")
	archiveServiceURL := os.Getenv("ARCHIVE_SERVICE_URL")
	reportServiceURL := os.Getenv("REPORT_SERVICE_URL")
	if cfg.DBEnabled {
		db, err := platformdb.Open(cfg)
		if err != nil {
			log.Fatal(err)
		}
		service := approvalworkflow.NewWithDB(db)
		service.SetServiceURLs(contractServiceURL, performanceServiceURL, archiveServiceURL, reportServiceURL)
		service.RegisterRoutes(server.Router.Group("/api/v1"))
	} else {
		service := approvalworkflow.New()
		service.SetServiceURLs(contractServiceURL, performanceServiceURL, archiveServiceURL, reportServiceURL)
		service.RegisterRoutes(server.Router.Group("/api/v1"))
	}

	log.Fatal(server.Run())
}
