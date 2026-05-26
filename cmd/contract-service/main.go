package main

import (
	"log"
	"os"

	"contract-manage/internal/microservices/contract"
	"contract-manage/pkg/microplatform/auditclient"
	"contract-manage/pkg/microplatform/app"
	"contract-manage/pkg/microplatform/config"
	platformdb "contract-manage/pkg/microplatform/db"
)

func main() {
	cfg, err := config.Load("contract-service", 8103)
	if err != nil {
		log.Fatal(err)
	}

	server := app.New(cfg)
	audit := auditclient.New(os.Getenv("AUDIT_SERVICE_URL"), "contract-service")
	documentServiceURL := os.Getenv("DOCUMENT_SERVICE_URL")
	partyServiceURL := os.Getenv("PARTY_SERVICE_URL")
	if cfg.DBEnabled {
		db, err := platformdb.Open(cfg)
		if err != nil {
			log.Fatal(err)
		}
		service := contract.NewWithDB(db)
		service.SetAuditClient(audit)
		service.SetDocumentServiceURL(documentServiceURL)
		service.SetPartyServiceURL(partyServiceURL)
		service.RegisterRoutes(server.Router.Group("/api/v1"))
	} else {
		service := contract.New()
		service.SetAuditClient(audit)
		service.SetDocumentServiceURL(documentServiceURL)
		service.SetPartyServiceURL(partyServiceURL)
		service.RegisterRoutes(server.Router.Group("/api/v1"))
	}

	log.Fatal(server.Run())
}
