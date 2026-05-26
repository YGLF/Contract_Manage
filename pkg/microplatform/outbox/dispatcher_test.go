package outbox

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDispatchPendingMarksMessageDispatchedOnSuccess(t *testing.T) {
	notifyCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		notifyCalls++
		if r.URL.Path != "/api/v1/notifications/messages" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("X-Trace-Id"); got != "trace-001" {
			t.Fatalf("expected trace header trace-001, got %q", got)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	db := newOutboxTestDB(t)
	seedOutboxMessage(t, db, Message{
		ID:            "evt-001",
		EventType:     "risk.created",
		AggregateType: "risk",
		AggregateID:   "risk-001",
		TraceID:       "trace-001",
		OperatorID:    "u-admin",
		Source:        "risk-service",
		PayloadRaw:    mustJSONString(t, map[string]interface{}{"contract_id": "ctr-001", "rule_code": "expiry_warning", "severity": "high"}),
		Status:        "pending",
		OccurredAt:    time.Now(),
		CreatedAt:     time.Now(),
	})

	dispatcher := NewDispatcher(db, server.URL, "")
	dispatched, err := dispatcher.DispatchPending(10)
	if err != nil {
		t.Fatalf("DispatchPending returned error: %v", err)
	}
	if dispatched != 1 {
		t.Fatalf("expected 1 dispatched message, got %d", dispatched)
	}
	if notifyCalls != 1 {
		t.Fatalf("expected 1 downstream call, got %d", notifyCalls)
	}

	assertOutboxStatus(t, db, "evt-001", "dispatched")
}

func TestDispatchPendingDoesNotMarkMessageDispatchedWhenDownstreamReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"temporary failure"}`))
	}))
	defer server.Close()

	db := newOutboxTestDB(t)
	seedOutboxMessage(t, db, Message{
		ID:            "evt-002",
		EventType:     "risk.created",
		AggregateType: "risk",
		AggregateID:   "risk-002",
		TraceID:       "trace-002",
		OperatorID:    "u-admin",
		Source:        "risk-service",
		PayloadRaw:    mustJSONString(t, map[string]interface{}{"contract_id": "ctr-002", "rule_code": "expiry_warning", "severity": "high"}),
		Status:        "pending",
		OccurredAt:    time.Now(),
		CreatedAt:     time.Now(),
	})

	dispatcher := NewDispatcher(db, server.URL, "")
	dispatched, err := dispatcher.DispatchPending(10)
	if err != nil {
		t.Fatalf("DispatchPending returned error: %v", err)
	}
	if dispatched != 0 {
		t.Fatalf("expected 0 dispatched messages, got %d", dispatched)
	}

	assertOutboxStatus(t, db, "evt-002", "pending")
}

func newOutboxTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("migrate outbox: %v", err)
	}
	return db
}

func seedOutboxMessage(t *testing.T, db *gorm.DB, message Message) {
	t.Helper()
	if err := db.Create(&message).Error; err != nil {
		t.Fatalf("seed outbox message: %v", err)
	}
}

func assertOutboxStatus(t *testing.T, db *gorm.DB, id, expected string) {
	t.Helper()
	var message Message
	if err := db.First(&message, "id = ?", id).Error; err != nil {
		t.Fatalf("load outbox message %s: %v", id, err)
	}
	if message.Status != expected {
		t.Fatalf("expected status %q, got %q", expected, message.Status)
	}
}

func mustJSONString(t *testing.T, value interface{}) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return string(data)
}
