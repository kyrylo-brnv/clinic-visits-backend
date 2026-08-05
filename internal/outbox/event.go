package outbox

import (
	"encoding/json"
	"fmt"
)

const (
	AggregateTypeVisit    = "visit"
	EventTypeVisitCreated = "visit.created"
)

type Event struct {
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       []byte
}

func NewEvent(
	aggregateType string,
	aggregateID string,
	eventType string,
	payload any,
) (Event, error) {
	serializedPayload, err := json.Marshal(payload)
	if err != nil {
		return Event{}, fmt.Errorf("serialize outbox event payload: %w", err)
	}

	return Event{
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		EventType:     eventType,
		Payload:       serializedPayload,
	}, nil
}
