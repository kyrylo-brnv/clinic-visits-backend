package visit

import (
	"encoding/json"
	"testing"
	"time"
)

func TestVisitMarshalJSONUsesFixedMicrosecondTimestamps(t *testing.T) {
	t.Parallel()

	location := time.FixedZone("EEST", 3*60*60)
	visit := Visit{
		ID:             "44444444-4444-4444-8444-444444444444",
		DoctorID:       "11111111-1111-4111-8111-111111111111",
		PatientID:      "22222222-2222-4222-8222-222222222222",
		ClinicID:       "33333333-3333-4333-8333-333333333333",
		VisitStartTime: time.Date(2026, time.August, 4, 18, 37, 1, 4_990_000, location),
		VisitEndTime:   time.Date(2026, time.August, 4, 19, 37, 1, 120_000_000, location),
		CreatedAt:      time.Date(2026, time.August, 4, 12, 0, 0, 0, time.UTC),
		UpdatedAt:      time.Date(2026, time.August, 4, 12, 0, 1, 999_999_999, time.UTC),
	}

	payload, err := json.Marshal(visit)
	if err != nil {
		t.Fatalf("marshal visit: %v", err)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		t.Fatalf("decode visit JSON fields: %v", err)
	}

	expectedKeys := []string{
		"id",
		"doctor_id",
		"patient_id",
		"clinic_id",
		"visit_start_time",
		"visit_end_time",
		"created_at",
		"updated_at",
	}
	for _, key := range expectedKeys {
		if _, ok := fields[key]; !ok {
			t.Errorf("expected JSON key %q", key)
		}
	}
	if len(fields) != len(expectedKeys) {
		t.Fatalf("expected exactly %d JSON keys, got %d: %s", len(expectedKeys), len(fields), payload)
	}

	expectedTimestamps := map[string]string{
		"visit_start_time": "2026-08-04T18:37:01.004990+03:00",
		"visit_end_time":   "2026-08-04T19:37:01.120000+03:00",
		"created_at":       "2026-08-04T12:00:00.000000+00:00",
		"updated_at":       "2026-08-04T12:00:01.999999+00:00",
	}
	for key, expected := range expectedTimestamps {
		var actual string
		if err := json.Unmarshal(fields[key], &actual); err != nil {
			t.Fatalf("decode %s: %v", key, err)
		}
		if actual != expected {
			t.Errorf("expected %s %q, got %q", key, expected, actual)
		}
	}

	var decoded Visit
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode visit payload: %v", err)
	}
	if decoded.ID != visit.ID ||
		decoded.DoctorID != visit.DoctorID ||
		decoded.PatientID != visit.PatientID ||
		decoded.ClinicID != visit.ClinicID {
		t.Fatalf("non-time values changed after JSON round trip: %+v", decoded)
	}

	expectedTimes := []time.Time{
		visit.VisitStartTime.Truncate(time.Microsecond),
		visit.VisitEndTime.Truncate(time.Microsecond),
		visit.CreatedAt.Truncate(time.Microsecond),
		visit.UpdatedAt.Truncate(time.Microsecond),
	}
	actualTimes := []time.Time{
		decoded.VisitStartTime,
		decoded.VisitEndTime,
		decoded.CreatedAt,
		decoded.UpdatedAt,
	}
	for index := range expectedTimes {
		if !actualTimes[index].Equal(expectedTimes[index]) {
			t.Errorf("timestamp %d changed instant: expected %v, got %v", index, expectedTimes[index], actualTimes[index])
		}
	}
}
