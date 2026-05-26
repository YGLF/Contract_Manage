package outbox

import (
	"encoding/json"
	"time"

	"contract-manage/pkg/microplatform/events"

	"gorm.io/gorm"
)

type Message struct {
	ID            string    `gorm:"primaryKey;size:128"`
	EventType     string    `gorm:"size:128;index;not null"`
	AggregateType string    `gorm:"size:64;index;not null"`
	AggregateID   string    `gorm:"size:128;index;not null"`
	TraceID       string    `gorm:"size:128;index"`
	OperatorID    string    `gorm:"size:64;index"`
	Source        string    `gorm:"size:128;index;not null"`
	PayloadRaw    string    `gorm:"type:text;not null"`
	Status        string    `gorm:"size:32;index;not null"`
	OccurredAt    time.Time `gorm:"index;not null"`
	CreatedAt     time.Time
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&Message{})
}

func Append(db *gorm.DB, evt events.Event) error {
	payload := evt.Payload
	if payload == nil {
		payload = map[string]interface{}{}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	message := Message{
		ID:            evt.EventID,
		EventType:     evt.EventType,
		AggregateType: evt.AggregateType,
		AggregateID:   evt.AggregateID,
		TraceID:       evt.TraceID,
		OperatorID:    evt.OperatorID,
		Source:        evt.Source,
		PayloadRaw:    string(raw),
		Status:        "pending",
		OccurredAt:    evt.OccurredAt,
		CreatedAt:     time.Now(),
	}

	return db.Create(&message).Error
}
