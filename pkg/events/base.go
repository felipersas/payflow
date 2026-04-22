package events

import (
	"time"

	"github.com/google/uuid"
)

// BaseEvent é o contrato compartilhado entre todos os serviços.
// Todo evento do PayFlow deve conter estes campos.
type BaseEvent struct {
	EventID   string    `json:"event_id"`
	EventType string    `json:"event_type"`
	Timestamp time.Time `json:"timestamp"`
	Version   int       `json:"version"`
}

func NewBaseEvent(eventType string, version int) BaseEvent {
	return BaseEvent{
		EventID:   uuid.Must(uuid.NewV7()).String(),
		EventType: eventType,
		Timestamp: time.Now().UTC(),
		Version:   version,
	}
}
