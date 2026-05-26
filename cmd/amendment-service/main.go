package main

import (
	"log"
	"os"

	"contract-manage/internal/microservices/amendment"
	"contract-manage/pkg/microplatform/app"
	"contract-manage/pkg/microplatform/config"
	platformdb "contract-manage/pkg/microplatform/db"
)

func main() {
	cfg, err := config.Load("amendment-service", 8108)
	if err != nil {
		log.Fatal(err)
	}

	server := app.New(cfg)
	contractServiceURL := os.Getenv("CONTRACT_SERVICE_URL")
	if cfg.DBEnabled {
		db, err := platformdb.Open(cfg)
		if err != nil {
			log.Fatal(err)
		}
		service := amendment.NewWithDB(db)
		service.SetContractServiceURL(contractServiceURL)
		service.RegisterRoutes(server.Router.Group("/api/v1"))
	} else {
		service := amendment.New()
		service.SetContractServiceURL(contractServiceURL)
		service.RegisterRoutes(server.Router.Group("/api/v1"))
	}

	log.Fatal(server.Run())
}
