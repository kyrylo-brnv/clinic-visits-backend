package elasticsearch

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDocumentJSON(t *testing.T) {
	createdAt := time.Date(2026, time.August, 5, 14, 30, 45, 123456000, time.FixedZone("EEST", 3*60*60))
	updatedAt := createdAt.Add(time.Hour)
	dateOfBirth := time.Date(1988, time.April, 12, 0, 0, 0, 0, time.UTC)
	isDeleted := false
	visit := VisitSummary{
		ID:             "visit-1",
		DoctorID:       "doctor-1",
		PatientID:      "patient-1",
		ClinicID:       "clinic-1",
		Status:         "SCHEDULED",
		VisitStartTime: createdAt.Add(24 * time.Hour),
		VisitEndTime:   createdAt.Add(25 * time.Hour),
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}

	tests := []struct {
		name     string
		document any
		wantJSON string
	}{
		{
			name: "doctor",
			document: DoctorDocument{
				ID: "doctor-1", SpecialtyID: "specialty-1", ClinicID: "clinic-1", FullName: "Dr. Ada Lovelace",
				CreatedAt: createdAt, UpdatedAt: updatedAt, Visits: []VisitSummary{visit},
			},
			wantJSON: `{"id":"doctor-1","specialty_id":"specialty-1","clinic_id":"clinic-1","full_name":"Dr. Ada Lovelace","created_at":"2026-08-05T14:30:45.123456+03:00","updated_at":"2026-08-05T15:30:45.123456+03:00","visits":[{"id":"visit-1","doctor_id":"doctor-1","patient_id":"patient-1","clinic_id":"clinic-1","status":"SCHEDULED","visit_start_time":"2026-08-06T14:30:45.123456+03:00","visit_end_time":"2026-08-06T15:30:45.123456+03:00","created_at":"2026-08-05T14:30:45.123456+03:00","updated_at":"2026-08-05T15:30:45.123456+03:00"}]}`,
		},
		{
			name: "patient",
			document: PatientDocument{
				ID: "patient-1", FirstName: "Grace", LastName: "Hopper", DateOfBirth: dateOfBirth, Gender: "FEMALE",
				IsDeleted: &isDeleted, CreatedAt: createdAt, UpdatedAt: updatedAt, Visits: []VisitSummary{visit},
			},
			wantJSON: `{"id":"patient-1","first_name":"Grace","last_name":"Hopper","date_of_birth":"1988-04-12T00:00:00Z","gender":"FEMALE","is_deleted":false,"created_at":"2026-08-05T14:30:45.123456+03:00","updated_at":"2026-08-05T15:30:45.123456+03:00","visits":[{"id":"visit-1","doctor_id":"doctor-1","patient_id":"patient-1","clinic_id":"clinic-1","status":"SCHEDULED","visit_start_time":"2026-08-06T14:30:45.123456+03:00","visit_end_time":"2026-08-06T15:30:45.123456+03:00","created_at":"2026-08-05T14:30:45.123456+03:00","updated_at":"2026-08-05T15:30:45.123456+03:00"}]}`,
		},
		{
			name: "clinic",
			document: ClinicDocument{
				ID: "clinic-1", Name: "Central Clinic", Address: "1 Main Street", TimeZone: "Europe/Kyiv",
				CreatedAt: createdAt, UpdatedAt: updatedAt, Visits: []VisitSummary{visit},
			},
			wantJSON: `{"id":"clinic-1","name":"Central Clinic","address":"1 Main Street","time_zone":"Europe/Kyiv","created_at":"2026-08-05T14:30:45.123456+03:00","updated_at":"2026-08-05T15:30:45.123456+03:00","visits":[{"id":"visit-1","doctor_id":"doctor-1","patient_id":"patient-1","clinic_id":"clinic-1","status":"SCHEDULED","visit_start_time":"2026-08-06T14:30:45.123456+03:00","visit_end_time":"2026-08-06T15:30:45.123456+03:00","created_at":"2026-08-05T14:30:45.123456+03:00","updated_at":"2026-08-05T15:30:45.123456+03:00"}]}`,
		},
		{
			name: "visit",
			document: VisitDocument{
				ID: "visit-1", DoctorID: "doctor-1", PatientID: "patient-1", ClinicID: "clinic-1", Status: "SCHEDULED",
				VisitStartTime: visit.VisitStartTime, VisitEndTime: visit.VisitEndTime, CreatedAt: createdAt, UpdatedAt: updatedAt,
				Doctor:  VisitDoctorData{ID: "doctor-1", SpecialtyID: "specialty-1", ClinicID: "clinic-1", FullName: "Dr. Ada Lovelace"},
				Patient: VisitPatientData{ID: "patient-1", FirstName: "Grace", LastName: "Hopper", DateOfBirth: dateOfBirth, Gender: "FEMALE", IsDeleted: &isDeleted},
				Clinic:  VisitClinicData{ID: "clinic-1", Name: "Central Clinic", Address: "1 Main Street", TimeZone: "Europe/Kyiv"},
			},
			wantJSON: `{"id":"visit-1","doctor_id":"doctor-1","patient_id":"patient-1","clinic_id":"clinic-1","status":"SCHEDULED","visit_start_time":"2026-08-06T14:30:45.123456+03:00","visit_end_time":"2026-08-06T15:30:45.123456+03:00","created_at":"2026-08-05T14:30:45.123456+03:00","updated_at":"2026-08-05T15:30:45.123456+03:00","doctor":{"id":"doctor-1","specialty_id":"specialty-1","clinic_id":"clinic-1","full_name":"Dr. Ada Lovelace"},"patient":{"id":"patient-1","first_name":"Grace","last_name":"Hopper","date_of_birth":"1988-04-12T00:00:00Z","gender":"FEMALE","is_deleted":false},"clinic":{"id":"clinic-1","name":"Central Clinic","address":"1 Main Street","time_zone":"Europe/Kyiv"}}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := json.Marshal(test.document)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}
			if string(got) != test.wantJSON {
				t.Errorf("json.Marshal() = %s, want %s", got, test.wantJSON)
			}
		})
	}
}
