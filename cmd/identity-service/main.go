package main

import (
	"log"
	"os"

	"contract-manage/internal/microservices/identity"
	"contract-manage/pkg/microplatform/app"
	"contract-manage/pkg/microplatform/auditclient"
	"contract-manage/pkg/microplatform/config"
	platformdb "contract-manage/pkg/microplatform/db"
)

func main() {
	cfg, err := config.Load("identity-service", 8101)
	if err != nil {
		log.Fatal(err)
	}

	server := app.New(cfg)
	audit := auditclient.New(os.Getenv("AUDIT_SERVICE_URL"), "identity-service")
	if cfg.DBEnabled {
		db, err := platformdb.Open(cfg)
		if err != nil {
			log.Fatal(err)
		}
		service := identity.NewWithDB(cfg.JWTSecret, db)
		service.SetAuditClient(audit)
		service.RegisterRoutes(server.Router.Group("/api/v1"))
	} else {
		service := identity.New(cfg.JWTSecret)
		service.SetAuditClient(audit)
		service.RegisterRoutes(server.Router.Group("/api/v1"))
	}

	log.Fatal(server.Run())
}
