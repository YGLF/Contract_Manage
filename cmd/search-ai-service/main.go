package main

import (
	"log"
	"os"

	"contract-manage/internal/microservices/searchai"
	"contract-manage/pkg/microplatform/app"
	"contract-manage/pkg/microplatform/auditclient"
	"contract-manage/pkg/microplatform/config"
)

func main() {
	cfg, err := config.Load("search-ai-service", 8114)
	if err != nil {
		log.Fatal(err)
	}

	server := app.New(cfg)
	service := searchai.New()
	service.SetDependencies(
		os.Getenv("CONTRACT_SERVICE_URL"),
		os.Getenv("RISK_SERVICE_URL"),
		os.Getenv("REPORT_SERVICE_URL"),
		os.Getenv("PARTY_SERVICE_URL"),
		os.Getenv("PERFORMANCE_SERVICE_URL"),
		os.Getenv("ARCHIVE_SERVICE_URL"),
		os.Getenv("AI_MODEL_ENDPOINT"),
		auditclient.New(os.Getenv("AUDIT_SERVICE_URL"), "search-ai-service"),
	)
	service.RegisterRoutes(server.Router.Group("/api/v1"))

	log.Fatal(server.Run())
}
