package main

import (
	"os"
	"testing"
)

func TestLoadRuntimeConfigRequiresAtLeastOneDownstream(t *testing.T) {
	t.Setenv("NOTIFICATION_SERVICE_URL", "")
	t.Setenv("REPORT_SERVICE_URL", "")
	t.Setenv("OUTBOX_DISPATCH_LIMIT", "")

	_, _, _, err := loadRuntimeConfig()
	if err == nil {
		t.Fatal("expected error when no downstream URL is configured")
	}
}

func TestLoadRuntimeConfigRejectsInvalidURLAndLimit(t *testing.T) {
	t.Setenv("NOTIFICATION_SERVICE_URL", "ftp://example.com")
	t.Setenv("REPORT_SERVICE_URL", "")
	t.Setenv("OUTBOX_DISPATCH_LIMIT", "0")

	_, _, _, err := loadRuntimeConfig()
	if err == nil {
		t.Fatal("expected invalid URL error")
	}

	t.Setenv("NOTIFICATION_SERVICE_URL", "https://notify.internal")
	_, _, _, err = loadRuntimeConfig()
	if err == nil {
		t.Fatal("expected invalid limit error")
	}
}

func TestLoadRuntimeConfigNormalizesURLsAndLimit(t *testing.T) {
	t.Setenv("NOTIFICATION_SERVICE_URL", "https://notify.internal/")
	t.Setenv("REPORT_SERVICE_URL", "http://report.internal/api/")
	t.Setenv("OUTBOX_DISPATCH_LIMIT", "25")

	notifyURL, reportURL, limit, err := loadRuntimeConfig()
	if err != nil {
		t.Fatalf("loadRuntimeConfig returned error: %v", err)
	}
	if notifyURL != "https://notify.internal" {
		t.Fatalf("unexpected notify URL: %s", notifyURL)
	}
	if reportURL != "http://report.internal/api" {
		t.Fatalf("unexpected report URL: %s", reportURL)
	}
	if limit != 25 {
		t.Fatalf("unexpected dispatch limit: %d", limit)
	}
}

func TestNormalizeHTTPURLAllowsEmptyValue(t *testing.T) {
	const envKey = "TEMP_OUTBOX_URL"
	_ = os.Unsetenv(envKey)
	t.Setenv(envKey, "")

	value, err := normalizeHTTPURL(envKey)
	if err != nil {
		t.Fatalf("normalizeHTTPURL returned error: %v", err)
	}
	if value != "" {
		t.Fatalf("expected empty value, got %q", value)
	}
}
