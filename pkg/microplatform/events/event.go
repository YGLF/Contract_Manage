package events

import "time"

type Event struct {
	EventID       string                 `json:"event_id"`
	EventType     string                 `json:"event_type"`
	OccurredAt    time.Time              `json:"occurred_at"`
	TraceID       string                 `json:"trace_id"`
	Source        string                 `json:"source"`
	AggregateType string                 `json:"aggregate_type"`
	AggregateID   string                 `json:"aggregate_id"`
	OperatorID    string                 `json:"operator_id"`
	Payload       map[string]interface{} `json:"payload"`
}
