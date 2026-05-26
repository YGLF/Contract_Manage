package main

import (
	"log"
	"os"

	"contract-manage/internal/microservices/gateway"
	"contract-manage/pkg/microplatform/app"
	"contract-manage/pkg/microplatform/config"
)

func main() {
	cfg, err := config.Load("gateway-service", 8100)
	if err != nil {
		log.Fatal(err)
	}

	server := app.New(cfg)
	targets := map[string]string{
		"identity":          os.Getenv("IDENTITY_SERVICE_URL"),
		"audit":             os.Getenv("AUDIT_SERVICE_URL"),
		"contract":          os.Getenv("CONTRACT_SERVICE_URL"),
		"document":          os.Getenv("DOCUMENT_SERVICE_URL"),
		"performance":       os.Getenv("PERFORMANCE_SERVICE_URL"),
		"approval-workflow": os.Getenv("APPROVAL_WORKFLOW_SERVICE_URL"),
		"risk":              os.Getenv("RISK_SERVICE_URL"),
		"amendment":         os.Getenv("AMENDMENT_SERVICE_URL"),
		"closure":           os.Getenv("CLOSURE_SERVICE_URL"),
		"archive":           os.Getenv("ARCHIVE_SERVICE_URL"),
		"notification":      os.Getenv("NOTIFICATION_SERVICE_URL"),
		"report":            os.Getenv("REPORT_SERVICE_URL"),
		"party":             os.Getenv("PARTY_SERVICE_URL"),
		"search-ai":         os.Getenv("SEARCH_AI_SERVICE_URL"),
	}
	gateway.New(targets, cfg.JWTSecret).RegisterRoutes(server.Router)

	log.Fatal(server.Run())
}
