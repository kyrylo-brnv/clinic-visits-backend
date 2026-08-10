package outbox

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestAggregateEventVocabulary(t *testing.T) {
	t.Parallel()

	got := map[string][]string{
		AggregateTypeVisit:   {EventTypeVisitCreated, EventTypeVisitUpdated, EventTypeVisitDeleted},
		AggregateTypeDoctor:  {EventTypeDoctorCreated, EventTypeDoctorUpdated, EventTypeDoctorDeleted},
		AggregateTypePatient: {EventTypePatientCreated, EventTypePatientUpdated, EventTypePatientDeleted},
		AggregateTypeClinic:  {EventTypeClinicCreated, EventTypeClinicUpdated, EventTypeClinicDeleted},
	}
	want := map[string][]string{
		"visit":   {"visit.created", "visit.updated", "visit.deleted"},
		"doctor":  {"doctor.created", "doctor.updated", "doctor.deleted"},
		"patient": {"patient.created", "patient.updated", "patient.deleted"},
		"clinic":  {"clinic.created", "clinic.updated", "clinic.deleted"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("aggregate event vocabulary = %#v, want %#v", got, want)
	}
}

func TestNewEventSerializesPayload(t *testing.T) {
	t.Parallel()

	event, err := NewEvent(
		AggregateTypeVisit,
		"11111111-1111-4111-8111-111111111111",
		EventTypeVisitCreated,
		struct {
			VisitID string `json:"visit_id"`
		}{VisitID: "11111111-1111-4111-8111-111111111111"},
	)
	if err != nil {
		t.Fatalf("create event: %v", err)
	}

	if event.AggregateType != AggregateTypeVisit ||
		event.AggregateID != "11111111-1111-4111-8111-111111111111" ||
		event.EventType != EventTypeVisitCreated {
		t.Fatalf("unexpected event metadata: %+v", event)
	}

	var payload struct {
		VisitID string `json:"visit_id"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.VisitID != event.AggregateID {
		t.Fatalf("expected payload visit ID %q, got %q", event.AggregateID, payload.VisitID)
	}
}

func TestNewEventWrapsSerializationFailure(t *testing.T) {
	t.Parallel()

	_, err := NewEvent("aggregate", "id", "event", make(chan struct{}))
	var unsupportedTypeError *json.UnsupportedTypeError
	if !errors.As(err, &unsupportedTypeError) {
		t.Fatalf("expected wrapped JSON serialization error, got %v", err)
	}
}
