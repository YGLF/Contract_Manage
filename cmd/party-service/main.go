package main

import (
	"log"

	"contract-manage/internal/microservices/party"
	"contract-manage/pkg/microplatform/app"
	"contract-manage/pkg/microplatform/config"
	platformdb "contract-manage/pkg/microplatform/db"
)

func main() {
	cfg, err := config.Load("party-service", 8113)
	if err != nil {
		log.Fatal(err)
	}

	server := app.New(cfg)
	if cfg.DBEnabled {
		db, err := platformdb.Open(cfg)
		if err != nil {
			log.Fatal(err)
		}
		party.NewWithDB(db).RegisterRoutes(server.Router.Group("/api/v1"))
	} else {
		party.New().RegisterRoutes(server.Router.Group("/api/v1"))
	}

	log.Fatal(server.Run())
}
