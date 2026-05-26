package main

import (
	"log"
	"os"

	"contract-manage/internal/microservices/performance"
	"contract-manage/pkg/microplatform/app"
	"contract-manage/pkg/microplatform/config"
	platformdb "contract-manage/pkg/microplatform/db"
)

func main() {
	cfg, err := config.Load("performance-service", 8105)
	if err != nil {
		log.Fatal(err)
	}

	server := app.New(cfg)
	riskServiceURL := os.Getenv("RISK_SERVICE_URL")
	if cfg.DBEnabled {
		db, err := platformdb.Open(cfg)
		if err != nil {
			log.Fatal(err)
		}
		service := performance.NewWithDB(db)
		service.SetRiskServiceURL(riskServiceURL)
		service.RegisterRoutes(server.Router.Group("/api/v1"))
	} else {
		service := performance.New()
		service.SetRiskServiceURL(riskServiceURL)
		service.RegisterRoutes(server.Router.Group("/api/v1"))
	}

	log.Fatal(server.Run())
}
