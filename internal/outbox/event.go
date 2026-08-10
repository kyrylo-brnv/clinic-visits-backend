package outbox

import (
	"encoding/json"
	"fmt"
)

const (
	AggregateTypeVisit   = "visit"
	AggregateTypeDoctor  = "doctor"
	AggregateTypePatient = "patient"
	AggregateTypeClinic  = "clinic"

	EventTypeVisitCreated   = "visit.created"
	EventTypeVisitUpdated   = "visit.updated"
	EventTypeVisitDeleted   = "visit.deleted"
	EventTypeDoctorCreated  = "doctor.created"
	EventTypeDoctorUpdated  = "doctor.updated"
	EventTypeDoctorDeleted  = "doctor.deleted"
	EventTypePatientCreated = "patient.created"
	EventTypePatientUpdated = "patient.updated"
	EventTypePatientDeleted = "patient.deleted"
	EventTypeClinicCreated  = "clinic.created"
	EventTypeClinicUpdated  = "clinic.updated"
	EventTypeClinicDeleted  = "clinic.deleted"
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
