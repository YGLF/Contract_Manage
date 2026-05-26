package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"

	"contract-manage/pkg/microplatform/config"
	platformdb "contract-manage/pkg/microplatform/db"
	"contract-manage/pkg/microplatform/outbox"
)

func main() {
	cfg, err := config.Load("outbox-dispatcher", 0)
	if err != nil {
		log.Fatal(err)
	}
	if !cfg.DBEnabled {
		log.Fatal("DB_ENABLED=true is required for outbox-dispatcher")
	}

	db, err := platformdb.Open(cfg)
	if err != nil {
		log.Fatal(err)
	}
	if err := outbox.AutoMigrate(db); err != nil {
		log.Fatal(err)
	}

	notifyURL, reportURL, dispatchLimit, err := loadRuntimeConfig()
	if err != nil {
		log.Fatal(err)
	}

	dispatcher := outbox.NewDispatcher(
		db,
		notifyURL,
		reportURL,
	)

	count, err := dispatcher.DispatchPending(dispatchLimit)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("dispatched %d outbox messages\n", count)
}

func loadRuntimeConfig() (string, string, int, error) {
	notifyURL, err := normalizeHTTPURL("NOTIFICATION_SERVICE_URL")
	if err != nil {
		return "", "", 0, err
	}
	reportURL, err := normalizeHTTPURL("REPORT_SERVICE_URL")
	if err != nil {
		return "", "", 0, err
	}
	if notifyURL == "" && reportURL == "" {
		return "", "", 0, fmt.Errorf("at least one downstream target must be configured for outbox-dispatcher")
	}

	dispatchLimit := 100
	if raw := strings.TrimSpace(os.Getenv("OUTBOX_DISPATCH_LIMIT")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			return "", "", 0, fmt.Errorf("OUTBOX_DISPATCH_LIMIT must be a positive integer")
		}
		dispatchLimit = parsed
	}

	return notifyURL, reportURL, dispatchLimit, nil
}

func normalizeHTTPURL(envKey string) (string, error) {
	raw := strings.TrimSpace(os.Getenv(envKey))
	if raw == "" {
		return "", nil
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("%s is invalid: %w", envKey, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("%s must use http or https", envKey)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("%s must include a host", envKey)
	}

	return strings.TrimRight(parsed.String(), "/"), nil
}
