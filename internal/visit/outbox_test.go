package visit

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/smithautotest/clinic-visits/internal/outbox"
)

func TestVisitMutationEventsSerializeVisitSnapshot(t *testing.T) {
	t.Parallel()

	visit := Visit{
		ID:             "11111111-1111-4111-8111-111111111111",
		DoctorID:       "22222222-2222-4222-8222-222222222222",
		PatientID:      "33333333-3333-4333-8333-333333333333",
		ClinicID:       "44444444-4444-4444-8444-444444444444",
		Status:         StatusInProgress,
		VisitStartTime: time.Date(2026, time.August, 5, 9, 0, 0, 0, time.UTC),
		VisitEndTime:   time.Date(2026, time.August, 5, 10, 0, 0, 0, time.UTC),
		CreatedAt:      time.Date(2026, time.August, 1, 8, 0, 0, 0, time.UTC),
		UpdatedAt:      time.Date(2026, time.August, 4, 12, 0, 0, 0, time.UTC),
	}

	tests := []struct {
		name      string
		eventType string
		newEvent  func(Visit) (outbox.Event, error)
	}{
		{name: "updated", eventType: outbox.EventTypeVisitUpdated, newEvent: newUpdatedEvent},
		{name: "deleted", eventType: outbox.EventTypeVisitDeleted, newEvent: newDeletedEvent},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			event, err := test.newEvent(visit)
			if err != nil {
				t.Fatalf("create event: %v", err)
			}
			if event.AggregateType != outbox.AggregateTypeVisit ||
				event.AggregateID != visit.ID ||
				event.EventType != test.eventType {
				t.Fatalf("unexpected event metadata: %+v", event)
			}

			var payload struct {
				ID             string      `json:"id"`
				DoctorID       string      `json:"doctor_id"`
				PatientID      string      `json:"patient_id"`
				ClinicID       string      `json:"clinic_id"`
				Status         VisitStatus `json:"status"`
				VisitStartTime time.Time   `json:"visit_start_time"`
				VisitEndTime   time.Time   `json:"visit_end_time"`
				CreatedAt      time.Time   `json:"created_at"`
				UpdatedAt      time.Time   `json:"updated_at"`
			}
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			if payload.ID != visit.ID ||
				payload.DoctorID != visit.DoctorID ||
				payload.PatientID != visit.PatientID ||
				payload.ClinicID != visit.ClinicID ||
				payload.Status != visit.Status ||
				!payload.VisitStartTime.Equal(visit.VisitStartTime) ||
				!payload.VisitEndTime.Equal(visit.VisitEndTime) ||
				!payload.CreatedAt.Equal(visit.CreatedAt) ||
				!payload.UpdatedAt.Equal(visit.UpdatedAt) {
				t.Fatalf("expected payload to contain visit %+v, got %+v", visit, payload)
			}
		})
	}
}
