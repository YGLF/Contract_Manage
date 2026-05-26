package main

import (
	"log"
	"os"

	"contract-manage/internal/microservices/document"
	"contract-manage/pkg/microplatform/app"
	"contract-manage/pkg/microplatform/auditclient"
	"contract-manage/pkg/microplatform/config"
	platformdb "contract-manage/pkg/microplatform/db"
)

func main() {
	cfg, err := config.Load("document-service", 8104)
	if err != nil {
		log.Fatal(err)
	}

	server := app.New(cfg)
	audit := auditclient.New(os.Getenv("AUDIT_SERVICE_URL"), "document-service")
	if cfg.DBEnabled {
		db, err := platformdb.Open(cfg)
		if err != nil {
			log.Fatal(err)
		}
		service := document.NewWithDB(cfg.UploadDir, db)
		service.SetAuditClient(audit)
		service.RegisterRoutes(server.Router.Group("/api/v1"))
	} else {
		service := document.New(cfg.UploadDir)
		service.SetAuditClient(audit)
		service.RegisterRoutes(server.Router.Group("/api/v1"))
	}

	log.Fatal(server.Run())
}
